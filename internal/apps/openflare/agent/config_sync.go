// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strconv"
	"strings"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/performance"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"gorm.io/gorm"
)

const (
	snapshotKey            = "sites.v1"
	performanceKey         = "performance.v1"
	ipGroupPrefix          = "waf.ip-group.v1/"
	maxInlineConfigBytes   = 8 << 10
	maxManifestInlineBytes = 32 << 10
)

var (
	errConfigResourceNotFound = errors.New("configuration resource not found")
	errConfigResourceChanged  = errors.New("configuration resource changed")
)

type configResource struct {
	protocol.ConfigResource
	pull func(context.Context) (string, error)
	body string
}

type configProvider struct {
	key      string
	prefix   bool
	describe func(context.Context, *model.OpenFlareNode) (map[string]configResource, error)
}

// Providers own resource discovery and scoping. The transport applies the same
// inline limit, digest checks, and authorization rules to all resource types.
var configProviders = []configProvider{
	{key: performanceKey, describe: describePerformancePolicy},
	{key: snapshotKey, describe: describeSiteSnapshot},
	{key: ipGroupPrefix, prefix: true, describe: describeIPGroups},
}

func configManifest(ctx context.Context, node *model.OpenFlareNode, latestSiteRevision string) *protocol.ConfigManifest {
	ctx = db.WithPrimaryReads(ctx)
	manifest := &protocol.ConfigManifest{Version: protocol.EdgeConfigSyncVersion, Resources: make(map[string]protocol.ConfigResource)}
	remaining := maxManifestInlineBytes
	for _, provider := range configProviders {
		resources, err := provider.describe(ctx, node)
		if err != nil {
			logger.ErrorF(ctx, "describe node %s configuration resource %s: %v", node.NodeID, provider.key, err)
			continue
		}
		for _, key := range slices.Sorted(maps.Keys(resources)) {
			resource := resources[key].ConfigResource
			if key == snapshotKey && latestSiteRevision != "" && resource.Revision == latestSiteRevision {
				resource.Metadata = nil
			}
			if len(resource.InlineJSON) > remaining {
				resource.InlineJSON = ""
			}
			remaining -= len(resource.InlineJSON)
			manifest.Resources[key] = resource
		}
	}
	return manifest
}

func pullConfigResource(ctx context.Context, node *model.OpenFlareNode, key, digest string) (string, error) {
	ctx = db.WithPrimaryReads(ctx)
	for _, provider := range configProviders {
		if provider.key != key && (!provider.prefix || !strings.HasPrefix(key, provider.key)) {
			continue
		}
		resources, err := provider.describe(ctx, node)
		if err != nil {
			return "", err
		}
		resource, exists := resources[key]
		if !exists {
			return "", errConfigResourceNotFound
		}
		if resource.SHA256 != digest {
			return "", errConfigResourceChanged
		}
		body := resource.body
		if resource.pull != nil {
			body, err = resource.pull(ctx)
			if err != nil {
				return "", err
			}
		}
		if resourceDigest(body) != digest {
			return "", errConfigResourceChanged
		}
		return body, nil
	}
	return "", errConfigResourceNotFound
}

func inlineResource(data any, revision string) (configResource, error) {
	encoded, err := json.Marshal(data)
	if err != nil {
		return configResource{}, err
	}
	body := string(encoded)
	ref := protocol.ConfigResource{SHA256: resourceDigest(body), Revision: revision}
	if len(body) <= maxInlineConfigBytes {
		ref.InlineJSON = body
	}
	return configResource{ConfigResource: ref, body: body}, nil
}

func resourceDigest(body string) string {
	digest := sha256.Sum256([]byte(body))
	return hex.EncodeToString(digest[:])
}

func describeSiteSnapshot(ctx context.Context, _ *model.OpenFlareNode) (map[string]configResource, error) {
	meta, err := getActiveConfigMeta(ctx)
	if isActiveConfigNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]configResource{
		snapshotKey: {
			ConfigResource: protocol.ConfigResource{SHA256: meta.PayloadChecksum, Revision: meta.Version, Metadata: meta},
			pull: func(ctx context.Context) (string, error) {
				active, err := GetActivePingoraConfig(ctx)
				if err != nil {
					return "", err
				}
				if active.ReleaseSequence != meta.ReleaseSequence {
					return "", errConfigResourceChanged
				}
				return active.PayloadJSON, nil
			},
		},
	}, nil
}

func describePerformancePolicy(ctx context.Context, node *model.OpenFlareNode) (map[string]configResource, error) {
	var policy performance.NodePolicy
	encoded, err := repository.GetNodePerformancePolicy(ctx, node.NodeID)
	if err == nil {
		policy.Settings, err = performance.Decode(encoded)
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		err = nil
	}
	if err != nil {
		return nil, err
	}
	resource, err := inlineResource(policy, "")
	return map[string]configResource{performanceKey: resource}, err
}

func describeIPGroups(ctx context.Context, _ *model.OpenFlareNode) (map[string]configResource, error) {
	groups, err := validatedAgentWAFIPGroups(ctx, nil, true, enginePingora)
	if err != nil {
		return nil, err
	}
	resources := make(map[string]configResource, len(groups))
	for _, group := range groups {
		resource, err := inlineResource(group, group.Checksum)
		if err != nil {
			return nil, err
		}
		resources[ipGroupPrefix+strconv.FormatUint(uint64(group.ID), 10)] = resource
	}
	return resources, nil
}
