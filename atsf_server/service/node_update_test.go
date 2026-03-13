package service

import (
	"atsflare/common"
	"atsflare/model"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestRequestNodeAgentPreviewUpdate(t *testing.T) {
	setupServiceTestDB(t)

	node, err := CreateNode(NodeInput{Name: "preview-edge-1"})
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	originalClient := UpdateHTTPClientForTest()
	SetUpdateHTTPClientForTest(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://api.github.com/repos/"+common.AgentUpdateRepo+"/releases/tags/v0.5.0-rc.1" {
				t.Fatalf("unexpected request url: %s", req.URL.String())
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"tag_name":"v0.5.0-rc.1","prerelease":true}`)),
			}, nil
		}),
	})
	t.Cleanup(func() {
		SetUpdateHTTPClientForTest(originalClient)
	})

	updated, err := RequestNodeAgentUpdate(node.ID, NodeAgentUpdateInput{
		Channel: "preview",
		TagName: "v0.5.0-rc.1",
	})
	if err != nil {
		t.Fatalf("expected preview update request to succeed: %v", err)
	}
	if !updated.UpdateRequested {
		t.Fatal("expected update_requested to be true")
	}
	if updated.UpdateChannel != "preview" {
		t.Fatalf("unexpected update channel: %s", updated.UpdateChannel)
	}
	if updated.UpdateTag != "v0.5.0-rc.1" {
		t.Fatalf("unexpected update tag: %s", updated.UpdateTag)
	}
}

func TestHeartbeatNodeReturnsPreviewUpdateSettings(t *testing.T) {
	setupServiceTestDB(t)

	node := &model.Node{
		NodeID:                    "node-preview-1",
		Name:                      "preview-edge-1",
		IP:                        "10.0.0.8",
		AgentToken:                "agent-token",
		AgentVersion:              "v0.4.0",
		NginxVersion:              "1.27.1.2",
		Status:                    NodeStatusOnline,
		UpdateRequested:           true,
		UpdateChannel:             "preview",
		UpdateTag:                 "v0.5.0-rc.1",
		RestartOpenrestyRequested: true,
		AutoUpdateEnabled:         false,
	}
	if err := node.Insert(); err != nil {
		t.Fatalf("failed to seed node: %v", err)
	}
	if err := model.DB.Create(&model.ConfigVersion{
		Version:        "20260313-001",
		SnapshotJSON:   "{}",
		MainConfig:     "worker_processes auto;",
		RenderedConfig: "server { listen 80; }",
		Checksum:       "checksum-active-1",
		IsActive:       true,
		CreatedBy:      "root",
	}).Error; err != nil {
		t.Fatalf("failed to seed active config version: %v", err)
	}

	resp, err := HeartbeatNode(node, AgentNodePayload{
		NodeID:           node.NodeID,
		Name:             node.Name,
		IP:               node.IP,
		AgentVersion:     node.AgentVersion,
		NginxVersion:     node.NginxVersion,
		OpenrestyStatus:  OpenrestyStatusUnhealthy,
		OpenrestyMessage: "port 80 already allocated",
	})
	if err != nil {
		t.Fatalf("expected heartbeat to succeed: %v", err)
	}
	if resp.AgentSettings == nil {
		t.Fatal("expected agent settings in heartbeat response")
	}
	if resp.ActiveConfig == nil {
		t.Fatal("expected active config summary in heartbeat response")
	}
	if resp.ActiveConfig.Version == "" || resp.ActiveConfig.Checksum == "" {
		t.Fatal("expected active config summary to include version and checksum")
	}
	if !resp.AgentSettings.UpdateNow {
		t.Fatal("expected update_now to be true")
	}
	if resp.AgentSettings.UpdateChannel != "preview" {
		t.Fatalf("unexpected update channel: %s", resp.AgentSettings.UpdateChannel)
	}
	if resp.AgentSettings.UpdateTag != "v0.5.0-rc.1" {
		t.Fatalf("unexpected update tag: %s", resp.AgentSettings.UpdateTag)
	}
	if !resp.AgentSettings.RestartOpenrestyNow {
		t.Fatal("expected restart_openresty_now to be true")
	}
	if resp.Node.OpenrestyStatus != OpenrestyStatusUnhealthy {
		t.Fatalf("expected unhealthy openresty status, got %s", resp.Node.OpenrestyStatus)
	}
	if resp.Node.OpenrestyMessage != "port 80 already allocated" {
		t.Fatalf("unexpected openresty message: %s", resp.Node.OpenrestyMessage)
	}

	storedNode, err := model.GetNodeByID(node.ID)
	if err != nil {
		t.Fatalf("failed to reload node: %v", err)
	}
	if storedNode.UpdateRequested {
		t.Fatal("expected update_requested to be reset after heartbeat")
	}
	if storedNode.UpdateChannel != "stable" {
		t.Fatalf("expected update channel to reset to stable, got %s", storedNode.UpdateChannel)
	}
	if storedNode.UpdateTag != "" {
		t.Fatalf("expected update tag to be cleared, got %s", storedNode.UpdateTag)
	}
	if storedNode.RestartOpenrestyRequested {
		t.Fatal("expected restart_openresty_requested to be reset after heartbeat")
	}
}

func TestRequestNodeOpenrestyRestart(t *testing.T) {
	setupServiceTestDB(t)

	node, err := CreateNode(NodeInput{Name: "restart-edge-1"})
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	updated, err := RequestNodeOpenrestyRestart(node.ID)
	if err != nil {
		t.Fatalf("expected openresty restart request to succeed: %v", err)
	}
	if !updated.RestartOpenrestyRequested {
		t.Fatal("expected restart_openresty_requested to be true")
	}
}
