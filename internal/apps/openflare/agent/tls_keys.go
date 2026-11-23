// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/credential"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/edgeconfig"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/internal/shared/httpx"
	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const maxTLSKeyFetchBatch = 1024

var (
	errTLSKeyNodeQuarantined = errors.New(errNodeQuarantined)
	errTLSKeyForbidden       = errors.New(errEdgeResourceOutsideRelease)
	errTLSKeyChanged         = errConfigResourceChanged
	errTLSKeyNoActiveConfig  = errors.New(errNoActiveConfig)
	errTLSKeyBadRequest      = errors.New(errTLSKeysRequestInvalid)
)

// FetchTLSKeysHandler returns memory-only private keys for certificates in the active release.
// @Summary 拉取边缘节点内存态 TLS 私钥
// @Description 仅返回当前活跃配置发布范围内、且公钥摘要匹配的 TLS 私钥；响应禁止缓存，并强制写入节点证书下发审计台账。
// @Tags openflare-agent
// @Accept json
// @Produce json
// @Security AgentTokenAuth
// @Param body body protocol.FetchTLSKeysRequest true "证书 ID 与公钥 SHA-256 请求列表"
// @Success 200 {object} response.Any{data=protocol.FetchTLSKeysResponse} "私钥集合"
// @Failure 400 {object} response.Any "参数错误"
// @Failure 401 {object} response.Any "Token 无效"
// @Failure 403 {object} response.Any "节点已隔离或证书不在发布范围内"
// @Failure 404 {object} response.Any "无活跃配置"
// @Failure 409 {object} response.Any "证书摘要已变更"
// @Failure 500 {object} response.Any "内部错误"
// @Router /api/v1/agent/tls/keys [post]
func FetchTLSKeysHandler(c *gin.Context) {
	node, ok := NodeFromContext(c)
	if !ok {
		response.AbortUnauthorized(c, errInvalidAgentToken)
		return
	}
	var req protocol.FetchTLSKeysRequest
	if !httpx.BindJSON(c, &req) {
		return
	}
	c.Header("Cache-Control", "private, no-store, no-cache, must-revalidate")
	c.Header("Pragma", "no-cache")

	resp, err := FetchNodeTLSKeys(c.Request.Context(), node, req)
	switch {
	case errors.Is(err, errTLSKeyNodeQuarantined):
		c.Header(agentQuarantinedHeader, agentQuarantinedHeaderTrue)
		response.AbortForbidden(c, errNodeQuarantined)
	case errors.Is(err, errTLSKeyForbidden):
		response.AbortForbidden(c, errEdgeResourceOutsideRelease)
	case errors.Is(err, errTLSKeyChanged):
		response.AbortConflict(c, errConfigResourceChanged.Error())
	case errors.Is(err, errTLSKeyNoActiveConfig):
		response.AbortNotFound(c, errNoActiveConfig)
	case errors.Is(err, errTLSKeyBadRequest):
		response.AbortBadRequest(c, errTLSKeysRequestInvalid)
	case err != nil:
		logger.ErrorF(c.Request.Context(), "fetch TLS keys for node %s: %v", node.NodeID, err)
		response.AbortInternal(c, "Could not load TLS private keys")
	default:
		c.JSON(http.StatusOK, response.OK(resp))
	}
}

// FetchNodeTLSKeys authorizes, audits, and returns private keys for an active edge node.
func FetchNodeTLSKeys(ctx context.Context, node *model.OpenFlareNode, req protocol.FetchTLSKeysRequest) (*protocol.FetchTLSKeysResponse, error) {
	if node == nil || node.NodeID == "" {
		return nil, errTLSKeyNodeQuarantined
	}
	ctx = db.WithPrimaryReads(ctx)
	freshNode, err := repository.GetOpenFlareNodeByID(ctx, node.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errTLSKeyNodeQuarantined
		}
		return nil, err
	}
	if freshNode.IsQuarantined() {
		return nil, errTLSKeyNodeQuarantined
	}

	requested, err := normalizeTLSKeyRequestItems(req.Certificates)
	if err != nil {
		return nil, err
	}

	version, err := repository.GetActivePingoraConfigVersion(ctx)
	if isActiveConfigNotFound(err) {
		return nil, errTLSKeyNoActiveConfig
	}
	if err != nil {
		return nil, err
	}
	if version.PayloadChecksum == "" {
		return nil, errTLSKeyNoActiveConfig
	}
	source, err := repository.GetConfigVersionSource(ctx, version)
	if err != nil {
		return nil, err
	}
	allowed, err := edgeconfig.ActiveCertificateDigests(source.SnapshotJSON, source.SupportFilesJSON)
	if err != nil {
		return nil, err
	}

	keys := make(map[string]string, len(requested))
	deliveries := make([]model.OpenFlareNodeCertDelivery, 0, len(requested))
	now := time.Now().UTC()
	for _, item := range requested {
		expectedDigest, ok := allowed[item.CertificateID]
		if !ok {
			return nil, errTLSKeyForbidden
		}
		if expectedDigest != item.CertSHA256 {
			return nil, errTLSKeyChanged
		}
		cert, certErr := repository.GetTLSCertificateByID(ctx, item.CertificateID)
		if certErr != nil {
			if errors.Is(certErr, gorm.ErrRecordNotFound) {
				return nil, errTLSKeyChanged
			}
			return nil, certErr
		}
		if edgeconfig.CertificatePEMDigest(cert.CertPEM) != item.CertSHA256 {
			return nil, errTLSKeyChanged
		}
		keyPEM, openErr := credential.Open(cert.KeyPEM)
		if openErr != nil {
			return nil, openErr
		}
		keyPEM = strings.TrimSpace(keyPEM)
		if keyPEM == "" {
			return nil, errors.New("certificate private key is empty")
		}
		keys[strconv.FormatUint(uint64(item.CertificateID), 10)] = keyPEM + "\n"
		deliveries = append(deliveries, model.OpenFlareNodeCertDelivery{
			NodeID:           freshNode.NodeID,
			CertificateID:    item.CertificateID,
			CertSHA256:       item.CertSHA256,
			ReleaseSequence:  version.ReleaseSequence,
			DeliveryCount:    1,
			FirstDeliveredAt: now,
			LastDeliveredAt:  now,
		})
	}

	// Fail closed: do not return private keys if the delivery audit ledger cannot be persisted.
	if err := repository.UpsertNodeCertDeliveries(ctx, deliveries); err != nil {
		return nil, err
	}

	return &protocol.FetchTLSKeysResponse{
		ReleaseSequence: version.ReleaseSequence,
		Keys:            keys,
	}, nil
}

func normalizeTLSKeyRequestItems(items []protocol.TLSKeyRequestItem) ([]protocol.TLSKeyRequestItem, error) {
	if len(items) == 0 || len(items) > maxTLSKeyFetchBatch {
		return nil, errTLSKeyBadRequest
	}
	seen := make(map[uint]string, len(items))
	normalized := make([]protocol.TLSKeyRequestItem, 0, len(items))
	for _, item := range items {
		if item.CertificateID == 0 {
			return nil, errTLSKeyBadRequest
		}
		digest := strings.ToLower(strings.TrimSpace(item.CertSHA256))
		raw, err := hex.DecodeString(digest)
		if err != nil || len(raw) != 32 {
			return nil, errTLSKeyBadRequest
		}
		if previous, exists := seen[item.CertificateID]; exists {
			if previous != digest {
				return nil, errTLSKeyBadRequest
			}
			continue
		}
		seen[item.CertificateID] = digest
		normalized = append(normalized, protocol.TLSKeyRequestItem{
			CertificateID: item.CertificateID,
			CertSHA256:    digest,
		})
	}
	return normalized, nil
}
