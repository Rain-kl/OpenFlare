// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package node

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"sync"
	"testing"
	"time"

	cf "github.com/Rain-kl/Wavelet/internal/apps/openflare/cloudflare"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/credential"
	oftls "github.com/Rain-kl/Wavelet/internal/apps/openflare/tls"
	"github.com/Rain-kl/Wavelet/internal/infra/config"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/infra/task"
	"github.com/Rain-kl/Wavelet/internal/listener"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/edgeconfig"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateSecurityTestCertPEM(t *testing.T, cn string) (string, string) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		DNSNames:     []string{cn},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	require.NoError(t, err)
	keyBytes, err := x509.MarshalECPrivateKey(priv)
	require.NoError(t, err)
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}))
	return certPEM, keyPEM
}

func TestQuarantineAndUnquarantineNode(t *testing.T) {
	conn := setupNodePageDB(t, 0)
	ctx := context.Background()

	previousSecret := config.Config.App.SessionSecret
	config.Config.App.SessionSecret = "test_session_secret_for_node_security"
	t.Cleanup(func() { config.Config.App.SessionSecret = previousSecret })

	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	previousClient := task.AsynqClient
	task.AsynqClient = asynq.NewClient(asynq.RedisClientOpt{Addr: mr.Addr()})
	task.RegisterTaskMeta(oftls.SSLSingleRenewMeta)
	t.Cleanup(func() {
		if task.AsynqClient != nil {
			_ = task.AsynqClient.Close()
		}
		task.AsynqClient = previousClient
	})

	var dispatchedCFGroups []uint
	restoreCFDispatch := cf.SetDispatchTaskForTest(func(_ context.Context, taskType string, payload []byte, _ string) (string, error) {
		if taskType == cf.TaskTypeSyncGroup {
			var p cf.SyncGroupPayload
			if err := json.Unmarshal(payload, &p); err == nil {
				dispatchedCFGroups = append(dispatchedCFGroups, p.GroupID)
			}
		}
		return "cf-task-1", nil
	})
	defer restoreCFDispatch()

	now := time.Now().UTC()
	oldToken := "old-node-token-1234567890abcdef01"
	primary := model.OpenFlareNode{
		NodeID:      "edge-primary-sec",
		Name:        "Primary Edge",
		AccessToken: oldToken,
		NodeType:    nodeTypeEdgeNode,
		Engine:      "pingora",
		Status:      nodeStatusOnline,
		IP:          "198.51.100.10",
		LastSeenAt:  &now,
	}
	backup := model.OpenFlareNode{
		NodeID:      "edge-backup-sec",
		Name:        "Backup Edge",
		AccessToken: "backup-token-1234567890abcdef012",
		NodeType:    nodeTypeEdgeNode,
		Engine:      "pingora",
		Status:      nodeStatusOnline,
		IP:          "198.51.100.20",
		LastSeenAt:  &now,
	}
	require.NoError(t, conn.Create(&primary).Error)
	require.NoError(t, conn.Create(&backup).Error)

	group := model.CFPointingGroup{
		Name:          "prod-dns-group",
		Enabled:       true,
		TargetMode:    model.CFPointingTargetModeNode,
		PrimaryNodeID: primary.ID,
		BackupNodeID:  &backup.ID,
		ActiveNodeID:  primary.ID,
	}
	require.NoError(t, conn.Create(&group).Error)

	acmeCertPEM, acmeKeyPEM := generateSecurityTestCertPEM(t, "acme.example.com")
	acmeSealed, err := credential.Seal(acmeKeyPEM)
	require.NoError(t, err)
	acmeCert := model.TLSCertificate{
		Name:          "ACME Exposed Cert",
		Provider:      "acme",
		PrimaryDomain: "acme.example.com",
		CertPEM:       acmeCertPEM,
		KeyPEM:        acmeSealed,
		ApplyStatus:   "ready",
	}
	require.NoError(t, conn.Create(&acmeCert).Error)

	uploadCertPEM, uploadKeyPEM := generateSecurityTestCertPEM(t, "upload.example.com")
	uploadSealed, err := credential.Seal(uploadKeyPEM)
	require.NoError(t, err)
	uploadCert := model.TLSCertificate{
		Name:          "Upload Exposed Cert",
		Provider:      "upload",
		PrimaryDomain: "upload.example.com",
		CertPEM:       uploadCertPEM,
		KeyPEM:        uploadSealed,
		ApplyStatus:   "ready",
	}
	require.NoError(t, conn.Create(&uploadCert).Error)

	require.NoError(t, repository.UpsertNodeCertDeliveries(ctx, []model.OpenFlareNodeCertDelivery{
		{
			NodeID:           primary.NodeID,
			CertificateID:    acmeCert.ID,
			CertSHA256:       edgeconfig.CertificatePEMDigest(acmeCertPEM),
			ReleaseSequence:  1,
			DeliveryCount:    1,
			FirstDeliveredAt: now,
			LastDeliveredAt:  now,
		},
		{
			NodeID:           primary.NodeID,
			CertificateID:    uploadCert.ID,
			CertSHA256:       edgeconfig.CertificatePEMDigest(uploadCertPEM),
			ReleaseSequence:  1,
			DeliveryCount:    1,
			FirstDeliveredAt: now,
			LastDeliveredAt:  now,
		},
	}))

	var mu sync.Mutex
	var revokedEvents []listener.NodeTokenRevoked
	listener.OnNodeTokenRevoked(func(_ context.Context, event listener.NodeTokenRevoked) {
		mu.Lock()
		defer mu.Unlock()
		if event.NodeID == primary.NodeID {
			revokedEvents = append(revokedEvents, event)
		}
	})

	result, err := QuarantineNode(ctx, primary.ID, QuarantineInput{
		Reason:          "ssh compromise",
		RotateACMECerts: true,
	})
	require.NoError(t, err)
	require.NotNil(t, result.Node)
	assert.Equal(t, nodeStatusQuarantined, result.Node.Status)
	assert.NotNil(t, result.Node.QuarantinedAt)
	assert.Equal(t, "ssh compromise", result.Node.QuarantineReason)
	assert.Empty(t, result.Node.AccessToken)

	storedPrimary, err := repository.GetOpenFlareNodeByID(ctx, primary.ID)
	require.NoError(t, err)
	assert.True(t, storedPrimary.IsQuarantined())
	assert.NotEqual(t, oldToken, storedPrimary.AccessToken)
	assert.Len(t, storedPrimary.AccessToken, 32)
	quarantineToken := storedPrimary.AccessToken

	mu.Lock()
	require.Len(t, revokedEvents, 1)
	assert.Equal(t, oldToken, revokedEvents[0].OldToken)
	assert.Equal(t, "ssh compromise", revokedEvents[0].Reason)
	mu.Unlock()

	assert.Equal(t, []string{"prod-dns-group"}, result.DNSSwitchedGroups)
	assert.Equal(t, []uint{group.ID}, dispatchedCFGroups)

	var updatedGroup model.CFPointingGroup
	require.NoError(t, conn.First(&updatedGroup, group.ID).Error)
	assert.Equal(t, backup.ID, updatedGroup.ActiveNodeID)

	assert.Equal(t, []uint{acmeCert.ID}, result.RotatedCertificateIDs)
	assert.Equal(t, []uint{uploadCert.ID}, result.ManualActionCertificateIDs)
	assert.Empty(t, result.RotationErrors)

	var executions []model.TaskExecution
	require.NoError(t, db.DB(ctx).Where("task_type = ?", oftls.SSLSingleRenewTask).Find(&executions).Error)
	require.Len(t, executions, 1)
	var taskPayload oftls.SSLSingleRenewPayload
	require.NoError(t, json.Unmarshal([]byte(executions[0].Payload), &taskPayload))
	assert.Equal(t, acmeCert.ID, taskPayload.ID)
	assert.True(t, taskPayload.RevokePrevious)
	assert.Equal(t, edgeconfig.CertificatePEMDigest(acmeCertPEM), edgeconfig.CertificatePEMDigest(taskPayload.PreviousCertPEM))

	// Unquarantine issues another fresh token and clears quarantine state.
	unquarantined, err := UnquarantineNode(ctx, primary.ID)
	require.NoError(t, err)
	assert.Equal(t, nodeStatusPending, unquarantined.Status)
	assert.Nil(t, unquarantined.QuarantinedAt)
	assert.Empty(t, unquarantined.QuarantineReason)
	assert.NotEmpty(t, unquarantined.AccessToken)
	assert.NotEqual(t, oldToken, unquarantined.AccessToken)
	assert.NotEqual(t, quarantineToken, unquarantined.AccessToken)

	mu.Lock()
	assert.Len(t, revokedEvents, 2)
	assert.Equal(t, quarantineToken, revokedEvents[1].OldToken)
	mu.Unlock()
}
