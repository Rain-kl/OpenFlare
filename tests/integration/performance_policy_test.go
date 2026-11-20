// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	ofnode "github.com/Rain-kl/Wavelet/internal/apps/openflare/node"
	"github.com/Rain-kl/Wavelet/internal/openflare/edgeconfig"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/stretchr/testify/require"
)

func TestPerformancePolicyAssignmentAndResourceSync(t *testing.T) {
	engine, admin, cleanup := setupCoreChainTest(t)
	t.Cleanup(cleanup)
	ctx := context.Background()
	first, err := ofnode.CreateNode(ctx, ofnode.Input{Name: "first-edge"})
	require.NoError(t, err)
	second, err := ofnode.CreateNode(ctx, ofnode.Input{Name: "second-edge"})
	require.NoError(t, err)
	third, err := ofnode.CreateNode(ctx, ofnode.Input{Name: "third-edge"})
	require.NoError(t, err)
	create := apiPath("/performance-policies/")
	newPolicy := performJSONRequest(t, engine, http.MethodPost, create,
		map[string]any{"name": "High performance", "settings": map[string]any{"threads": 4}}, adminAuthHeaders(admin.Token))
	require.Equal(t, http.StatusOK, newPolicy.Code, newPolicy.Body.String())
	var policy struct{ ID uint }
	unmarshalAPIData(t, requireAPIOK(t, newPolicy).Data, &policy)
	require.NotZero(t, policy.ID)

	assign := apiPath(fmt.Sprintf("/performance-policies/%d/assign", policy.ID))
	assigned := performJSONRequest(t, engine, http.MethodPost, assign,
		map[string]any{"node_ids": []string{first.NodeID, second.NodeID}}, adminAuthHeaders(admin.Token))
	require.Equal(t, http.StatusOK, assigned.Code, assigned.Body.String())

	heartbeat := func(token string) protocol.ConfigResource {
		t.Helper()
		record := performJSONRequest(t, engine, http.MethodPost, "/api/v1/agent/nodes/heartbeat", protocol.NodePayload{
			Engine: "pingora", ContractVersion: edgeconfig.ContractVersion,
			ConfigSyncVersion: protocol.EdgeConfigSyncVersion, IP: "192.0.2.1", Version: "test",
		}, map[string]string{"X-Agent-Token": token})
		require.Equal(t, http.StatusOK, record.Code, record.Body.String())
		var response struct{ Config protocol.ConfigManifest }
		unmarshalAPIData(t, requireAPIOK(t, record).Data, &response)
		require.Equal(t, protocol.EdgeConfigSyncVersion, response.Config.Version)
		resource := response.Config.Resources["performance.v1"]
		digest := sha256.Sum256([]byte(resource.InlineJSON))
		require.Equal(t, hex.EncodeToString(digest[:]), resource.SHA256)
		return resource
	}
	target := heartbeat(first.AccessToken)
	require.Contains(t, target.InlineJSON, `"threads":4`)
	require.Equal(t, target.SHA256, heartbeat(second.AccessToken).SHA256)
	unassigned := heartbeat(third.AccessToken)
	require.JSONEq(t, `{"settings":null}`, unassigned.InlineJSON)
	pull := "/api/v1/agent/config/pull?" + url.Values{
		"key": {"performance.v1"}, "sha256": {target.SHA256},
	}.Encode()
	ok := performJSONRequest(t, engine, http.MethodGet, pull, nil, map[string]string{"X-Agent-Token": first.AccessToken})
	require.Equal(t, http.StatusOK, ok.Code)
	require.Equal(t, target.InlineJSON, ok.Body.String())
	stale := performJSONRequest(t, engine, http.MethodGet, pull, nil, map[string]string{"X-Agent-Token": third.AccessToken})
	require.Equal(t, http.StatusConflict, stale.Code)

	cleared := performJSONRequest(t, engine, http.MethodPost, assign,
		map[string]any{"node_ids": []string{}}, adminAuthHeaders(admin.Token))
	require.Equal(t, http.StatusOK, cleared.Code, cleared.Body.String())
	require.Equal(t, unassigned.SHA256, heartbeat(first.AccessToken).SHA256)
}
