package service

import (
	"errors"
	"openflare/model"
	"strconv"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestGetActiveConfigForAgentIncludesPoWConfig(t *testing.T) {
	setupServiceTestDB(t)

	_, err := CreateProxyRoute(ProxyRouteInput{
		Domain:     "pow-agent.example.com",
		OriginURL:  "https://origin.internal",
		Enabled:    true,
		PoWEnabled: true,
		PoWConfig:  `{"difficulty":4,"algorithm":"fast","session_ttl":86400,"challenge_ttl":300,"whitelist":{"paths":["/.well-known/*","/favicon.ico","/robots.txt"],"user_agents":["Googlebot","bingbot","Baiduspider"]},"blacklist":{"ips":[],"ip_cidrs":[],"paths":[],"path_regexes":[],"user_agents":[]}}`,
	})
	if err != nil {
		t.Fatalf("CreateProxyRoute failed: %v", err)
	}

	if _, err := PublishConfigVersion("root", false); err != nil {
		t.Fatalf("PublishConfigVersion failed: %v", err)
	}

	activeConfig, err := GetActiveConfigForAgent()
	if err != nil {
		t.Fatalf("GetActiveConfigForAgent failed: %v", err)
	}

	for _, file := range activeConfig.SupportFiles {
		if file.Path == "pow_config.json" || file.Path == "waf_config.json" {
			t.Fatalf("agent config should not receive rendered runtime config file %s", file.Path)
		}
	}
	if !strings.Contains(activeConfig.SourceConfigJSON, `"pow_enabled":true`) {
		t.Fatal("expected agent config source json to include PoW source configuration")
	}
}

func TestGetActiveConfigForAgentIncludesWAFConfig(t *testing.T) {
	setupServiceTestDB(t)

	_, err := CreateProxyRoute(ProxyRouteInput{
		Domain:    "waf-agent.example.com",
		OriginURL: "https://origin.internal",
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("CreateProxyRoute failed: %v", err)
	}

	if _, err := PublishConfigVersion("root", false); err != nil {
		t.Fatalf("PublishConfigVersion failed: %v", err)
	}

	activeConfig, err := GetActiveConfigForAgent()
	if err != nil {
		t.Fatalf("GetActiveConfigForAgent failed: %v", err)
	}

	for _, file := range activeConfig.SupportFiles {
		if file.Path == "waf_config.json" {
			t.Fatal("agent config should not receive rendered waf_config.json")
		}
	}
	if !strings.Contains(activeConfig.SourceConfigJSON, `"waf"`) {
		t.Fatal("expected agent config source json to include WAF source configuration")
	}
}

func TestChangedWAFIPGroupsForAgentReturnsChecksumDelta(t *testing.T) {
	setupServiceTestDB(t)

	route, err := CreateProxyRoute(ProxyRouteInput{
		SiteName:  "agent-waf-ip-group",
		Domains:   []string{"agent-waf-ip-group.example.com"},
		OriginURL: "https://origin.internal",
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("CreateProxyRoute failed: %v", err)
	}
	ipGroup, err := CreateWAFIPGroup(WAFIPGroupInput{
		Name:    "agent runtime group",
		Type:    WAFIPGroupTypeManual,
		Enabled: true,
		IPList:  []string{"203.0.113.44"},
	})
	if err != nil {
		t.Fatalf("CreateWAFIPGroup failed: %v", err)
	}
	ruleGroup, err := CreateWAFRuleGroup(WAFRuleGroupInput{
		Name:              "agent refs",
		Enabled:           true,
		IPBlacklistGroups: []uint{ipGroup.ID},
	})
	if err != nil {
		t.Fatalf("CreateWAFRuleGroup failed: %v", err)
	}
	if _, err = ReplaceWAFSiteRuleGroups(route.ID, []uint{ruleGroup.ID}); err != nil {
		t.Fatalf("ReplaceWAFSiteRuleGroups failed: %v", err)
	}
	if _, err = PublishConfigVersion("root", false); err != nil {
		t.Fatalf("PublishConfigVersion failed: %v", err)
	}

	groups, err := ChangedWAFIPGroupsForAgent(nil, nil)
	if err != nil {
		t.Fatalf("ChangedWAFIPGroupsForAgent failed: %v", err)
	}
	if len(groups) != 1 || groups[0].ID != ipGroup.ID || groups[0].IPList[0] != "203.0.113.44" || groups[0].Checksum == "" {
		t.Fatalf("unexpected changed groups: %#v", groups)
	}
	groupKey := strconv.FormatUint(uint64(ipGroup.ID), 10)
	same, err := ChangedWAFIPGroupsForAgent(nil, map[string]string{groupKey: groups[0].Checksum})
	if err != nil {
		t.Fatalf("ChangedWAFIPGroupsForAgent with checksum failed: %v", err)
	}
	if len(same) != 0 {
		t.Fatalf("expected no delta for matching checksum, got %#v", same)
	}
	updated, err := UpdateWAFIPGroup(ipGroup.ID, WAFIPGroupInput{
		Name:    "agent runtime group",
		Type:    WAFIPGroupTypeManual,
		Enabled: true,
		IPList:  []string{"203.0.113.45"},
	})
	if err != nil {
		t.Fatalf("UpdateWAFIPGroup failed: %v", err)
	}
	delta, err := ChangedWAFIPGroupsForAgent(nil, map[string]string{groupKey: groups[0].Checksum})
	if err != nil {
		t.Fatalf("ChangedWAFIPGroupsForAgent after update failed: %v", err)
	}
	if len(delta) != 1 || delta[0].ID != updated.ID || delta[0].IPList[0] != "203.0.113.45" || delta[0].Checksum == groups[0].Checksum {
		t.Fatalf("expected updated group delta, got %#v", delta)
	}
}

func TestGetActiveConfigForAgentUsesTenMinutePoWSessionDefault(t *testing.T) {
	setupServiceTestDB(t)

	_, err := CreateProxyRoute(ProxyRouteInput{
		Domain:     "pow-default.example.com",
		OriginURL:  "https://origin.internal",
		Enabled:    true,
		PoWEnabled: true,
		PoWConfig:  `{}`,
	})
	if err != nil {
		t.Fatalf("CreateProxyRoute failed: %v", err)
	}

	if _, err := PublishConfigVersion("root", false); err != nil {
		t.Fatalf("PublishConfigVersion failed: %v", err)
	}

	activeConfig, err := GetActiveConfigForAgent()
	if err != nil {
		t.Fatalf("GetActiveConfigForAgent failed: %v", err)
	}

	for _, file := range activeConfig.SupportFiles {
		if file.Path == "pow_config.json" {
			t.Fatal("agent config should not receive rendered pow_config.json")
		}
	}
	if !strings.Contains(activeConfig.SourceConfigJSON, `"session_ttl":600`) {
		t.Fatalf("expected default PoW session TTL to be in source json, got %s", activeConfig.SourceConfigJSON)
	}
}

func TestRegisterNodeWithAgentToken(t *testing.T) {
	setupServiceTestDB(t)

	// 1. Success path
	latitude := 31.2304
	longitude := 121.4737
	node, err := CreateNode(NodeInput{
		Name:              "reserved-node-1",
		IP:                "192.168.1.10",
		GeoManualOverride: true,
		GeoName:           "Shanghai",
		GeoLatitude:       &latitude,
		GeoLongitude:      &longitude,
	})
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	stored, err := model.GetNodeByID(node.ID)
	if err != nil {
		t.Fatalf("failed to fetch stored node: %v", err)
	}

	payload := AgentNodePayload{
		Name:            "payload-name-should-be-ignored",
		IP:              "192.168.1.20",
		AgentVersion:    "v1.0.1",
		NginxVersion:    "1.27.1.3",
		OpenrestyStatus: "healthy",
	}

	resp, err := RegisterNodeWithAgentToken(stored, payload)
	if err != nil {
		t.Fatalf("RegisterNodeWithAgentToken failed: %v", err)
	}

	if resp.NodeID != stored.NodeID || resp.AgentToken != stored.AgentToken || resp.Name != "reserved-node-1" {
		t.Errorf("unexpected response: %+v", resp)
	}

	// Verify that the node was updated in the DB
	updated, err := model.GetNodeByID(node.ID)
	if err != nil {
		t.Fatalf("failed to fetch updated node: %v", err)
	}
	if updated.AgentVersion != "v1.0.1" || updated.NginxVersion != "1.27.1.3" || updated.OpenrestyStatus != "healthy" {
		t.Errorf("node attributes were not updated: %+v", updated)
	}
	// Name should be preserved since preserveName is true
	if updated.Name != "reserved-node-1" {
		t.Errorf("expected name to be preserved, got %s", updated.Name)
	}

	// 2. Fail path - Nil Node
	_, err = RegisterNodeWithAgentToken(nil, payload)
	if err == nil || !strings.Contains(err.Error(), "节点不存在") {
		t.Errorf("expected error '节点不存在', got %v", err)
	}

	// 3. Fail path - Invalid Payload (empty IP)
	badPayload := payload
	badPayload.IP = ""
	_, err = RegisterNodeWithAgentToken(stored, badPayload)
	if err == nil || !strings.Contains(err.Error(), "ip 不能为空") {
		t.Errorf("expected error 'ip 不能为空', got %v", err)
	}

	// 4. Name update if empty
	emptyNameNode := &model.Node{
		NodeID:     "node-empty-name",
		Name:       "",
		AgentToken: "empty-name-token",
	}
	if err := emptyNameNode.Insert(); err != nil {
		t.Fatalf("failed to insert emptyNameNode: %v", err)
	}
	payloadWithName := payload
	payloadWithName.Name = "filled-name"
	payloadWithName.IP = "192.168.1.30"
	_, err = RegisterNodeWithAgentToken(emptyNameNode, payloadWithName)
	if err != nil {
		t.Fatalf("RegisterNodeWithAgentToken empty name node failed: %v", err)
	}
	updatedEmptyName, err := model.GetNodeByNodeID("node-empty-name")
	if err != nil {
		t.Fatalf("failed to fetch updatedEmptyName: %v", err)
	}
	if updatedEmptyName.Name != "filled-name" {
		t.Errorf("expected name to be filled, got %s", updatedEmptyName.Name)
	}
}

func TestRegisterNodeWithDiscovery(t *testing.T) {
	setupServiceTestDB(t)

	// 1. Success path
	payload := AgentNodePayload{
		Name:            "discovery-node",
		IP:              "192.168.2.10",
		AgentVersion:    "v1.0.0",
		NginxVersion:    "1.27.1.3",
		OpenrestyStatus: "healthy",
	}

	resp, err := RegisterNodeWithDiscovery(payload)
	if err != nil {
		t.Fatalf("RegisterNodeWithDiscovery failed: %v", err)
	}

	if resp.NodeID == "" || resp.AgentToken == "" || resp.Name != "discovery-node" {
		t.Errorf("unexpected response: %+v", resp)
	}

	// Verify database persistence
	node, err := model.GetNodeByNodeID(resp.NodeID)
	if err != nil {
		t.Fatalf("failed to fetch node: %v", err)
	}
	if node.IP != "192.168.2.10" || node.AgentVersion != "v1.0.0" || node.Name != "discovery-node" {
		t.Errorf("unexpected stored node data: %+v", node)
	}

	// 2. Name fallback if payload name is empty
	payloadNoName := payload
	payloadNoName.Name = ""
	payloadNoName.IP = "192.168.2.20"
	respNoName, err := RegisterNodeWithDiscovery(payloadNoName)
	if err != nil {
		t.Fatalf("RegisterNodeWithDiscovery no name failed: %v", err)
	}
	nodeNoName, err := model.GetNodeByNodeID(respNoName.NodeID)
	if err != nil {
		t.Fatalf("failed to fetch no-name node: %v", err)
	}
	if nodeNoName.Name != respNoName.NodeID {
		t.Errorf("expected name fallback to NodeID, got %s", nodeNoName.Name)
	}

	// 3. Fail path - Invalid Payload (empty AgentVersion)
	badPayload := payload
	badPayload.AgentVersion = ""
	_, err = RegisterNodeWithDiscovery(badPayload)
	if err == nil || !strings.Contains(err.Error(), "agent_version 不能为空") {
		t.Errorf("expected error 'agent_version 不能为空', got %v", err)
	}
}

func TestReportApplyLog_Success(t *testing.T) {
	setupServiceTestDB(t)

	// Seed node
	node := &model.Node{
		NodeID:       "node-apply-1",
		Name:         "apply-edge",
		IP:           "192.168.3.10",
		AgentToken:   "apply-token",
		AgentVersion: "v1.0.0",
		Status:       NodeStatusOffline,
	}
	if err := node.Insert(); err != nil {
		t.Fatalf("failed to insert node: %v", err)
	}

	payload := ApplyLogPayload{
		NodeID:              "node-apply-1",
		Version:             "20260531-001",
		Result:              "success",
		Message:             "Configuration applied successfully",
		Checksum:            "chk-1",
		MainConfigChecksum:  "m-chk-1",
		RouteConfigChecksum: "r-chk-1",
		SupportFileCount:    3,
	}

	log, err := ReportApplyLog(payload)
	if err != nil {
		t.Fatalf("ReportApplyLog failed: %v", err)
	}

	if log.NodeID != "node-apply-1" || log.Result != "success" || log.Message != "Configuration applied successfully" {
		t.Errorf("unexpected returned log: %+v", log)
	}

	// Verify that the node status and current version are updated in the DB
	updatedNode, err := model.GetNodeByNodeID("node-apply-1")
	if err != nil {
		t.Fatalf("failed to reload node: %v", err)
	}
	if updatedNode.CurrentVersion != "20260531-001" || updatedNode.Status != NodeStatusOnline || updatedNode.LastError != "" {
		t.Errorf("node was not updated correctly: %+v", updatedNode)
	}

	// Verify apply log is stored
	storedLogs, err := model.ListApplyLogs(model.ApplyLogQuery{NodeID: "node-apply-1", PageNo: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListApplyLogs failed: %v", err)
	}
	if len(storedLogs) != 1 || storedLogs[0].Checksum != "chk-1" {
		t.Errorf("expected 1 log, got: %d", len(storedLogs))
	}
}

func TestReportApplyLog_WarningAndFailure(t *testing.T) {
	setupServiceTestDB(t)

	// Seed node
	node := &model.Node{
		NodeID:         "node-apply-2",
		Name:           "apply-edge-2",
		IP:             "192.168.3.20",
		AgentToken:     "apply-token-2",
		AgentVersion:   "v1.0.0",
		CurrentVersion: "20260531-001", // Old version
		Status:         NodeStatusOnline,
	}
	if err := node.Insert(); err != nil {
		t.Fatalf("failed to insert node: %v", err)
	}

	// 1. Report Failure
	failPayload := ApplyLogPayload{
		NodeID:  "node-apply-2",
		Version: "20260531-002", // Target failed version
		Result:  "failed",
		Message: "reload process exited with code 1",
	}

	_, err := ReportApplyLog(failPayload)
	if err != nil {
		t.Fatalf("ReportApplyLog failed: %v", err)
	}

	// Node CurrentVersion should NOT be updated. Node LastError should be updated.
	updatedNode, err := model.GetNodeByNodeID("node-apply-2")
	if err != nil {
		t.Fatalf("failed to reload node: %v", err)
	}
	if updatedNode.CurrentVersion != "20260531-001" {
		t.Errorf("expected CurrentVersion to remain unchanged, got %s", updatedNode.CurrentVersion)
	}
	if updatedNode.LastError != "reload process exited with code 1" {
		t.Errorf("expected LastError to be set, got %s", updatedNode.LastError)
	}

	// 2. Report Warning (e.g. rolled back to old version successfully)
	warningPayload := ApplyLogPayload{
		NodeID:  "node-apply-2",
		Version: "20260531-002",
		Result:  "warning",
		Message: "reload failed, rolled back to 20260531-001 successfully",
	}

	_, err = ReportApplyLog(warningPayload)
	if err != nil {
		t.Fatalf("ReportApplyLog warning failed: %v", err)
	}

	updatedNodeWarning, err := model.GetNodeByNodeID("node-apply-2")
	if err != nil {
		t.Fatalf("failed to reload node: %v", err)
	}
	// CurrentVersion remains 20260531-001. LastError is the warning message.
	if updatedNodeWarning.CurrentVersion != "20260531-001" {
		t.Errorf("expected CurrentVersion to remain unchanged, got %s", updatedNodeWarning.CurrentVersion)
	}
	if updatedNodeWarning.LastError != "reload failed, rolled back to 20260531-001 successfully" {
		t.Errorf("expected LastError to be warning message, got %s", updatedNodeWarning.LastError)
	}
}

func TestReportApplyLog_Failures(t *testing.T) {
	setupServiceTestDB(t)

	// Seed node
	node := &model.Node{
		NodeID:       "node-apply-3",
		Name:         "apply-edge-3",
		IP:           "192.168.3.30",
		AgentToken:   "apply-token-3",
		AgentVersion: "v1.0.0",
		Status:       NodeStatusOnline,
	}
	if err := node.Insert(); err != nil {
		t.Fatalf("failed to insert node: %v", err)
	}

	// 1. Missing NodeID
	_, err := ReportApplyLog(ApplyLogPayload{Version: "v1", Result: "success"})
	if err == nil || !strings.Contains(err.Error(), "node_id 不能为空") {
		t.Errorf("expected empty node_id error, got %v", err)
	}

	// 2. Missing Version
	_, err = ReportApplyLog(ApplyLogPayload{NodeID: "node-apply-3", Result: "success"})
	if err == nil || !strings.Contains(err.Error(), "version 不能为空") {
		t.Errorf("expected empty version error, got %v", err)
	}

	// 3. Invalid Result
	_, err = ReportApplyLog(ApplyLogPayload{NodeID: "node-apply-3", Version: "v1", Result: "corrupted"})
	if err == nil || !strings.Contains(err.Error(), "result 仅支持 success、warning 或 failed") {
		t.Errorf("expected invalid result error, got %v", err)
	}

	// 4. Non-existent NodeID
	_, err = ReportApplyLog(ApplyLogPayload{NodeID: "non-existent-node-xyz", Version: "v1", Result: "success"})
	if err == nil || !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("expected record not found error, got %v", err)
	}

	// 5. Truncate excessively long message
	veryLongMsg := strings.Repeat("A", 20000)
	log, err := ReportApplyLog(ApplyLogPayload{
		NodeID:  "node-apply-3",
		Version: "v1",
		Result:  "success",
		Message: veryLongMsg,
	})
	if err != nil {
		t.Fatalf("ReportApplyLog with very long message failed: %v", err)
	}
	if len(log.Message) != 16000 {
		t.Errorf("expected message to be truncated to 16000, got %d", len(log.Message))
	}
}

func TestListAndCleanupApplyLogs(t *testing.T) {
	setupServiceTestDB(t)

	// Seed node
	node := &model.Node{
		NodeID:     "node-logs",
		Name:       "logs-edge",
		IP:         "192.168.4.10",
		AgentToken: "logs-token",
		Status:     NodeStatusOnline,
	}
	if err := node.Insert(); err != nil {
		t.Fatalf("failed to insert node: %v", err)
	}

	now := time.Now()
	// Seed logs of different ages
	logs := []model.ApplyLog{
		{NodeID: "node-logs", Version: "v1", Result: "success", Message: "1", CreatedAt: now.Add(-10 * 24 * time.Hour)}, // 10 days ago
		{NodeID: "node-logs", Version: "v2", Result: "success", Message: "2", CreatedAt: now.Add(-5 * 24 * time.Hour)},  // 5 days ago
		{NodeID: "node-logs", Version: "v3", Result: "success", Message: "3", CreatedAt: now},                           // Now
	}
	for i := range logs {
		if err := model.DB.Create(&logs[i]).Error; err != nil {
			t.Fatalf("failed to seed log: %v", err)
		}
	}

	// 1. Test pagination using ListApplyLogsPage
	pageResult, err := ListApplyLogsPage(ApplyLogListQuery{
		NodeID:   "node-logs",
		PageNo:   1,
		PageSize: 2,
	})
	if err != nil {
		t.Fatalf("ListApplyLogsPage failed: %v", err)
	}
	if pageResult.Total != 3 || len(pageResult.Rows) != 2 || pageResult.TotalPage != 2 {
		t.Errorf("unexpected pagination result: %+v", pageResult)
	}

	// 2. Test Cleanup with RetentionDays = 7
	cleanupResult, err := CleanupApplyLogs(ApplyLogCleanupInput{
		DeleteAll:     false,
		RetentionDays: 7,
	})
	if err != nil {
		t.Fatalf("CleanupApplyLogs failed: %v", err)
	}
	if cleanupResult.DeletedCount != 1 {
		t.Errorf("expected 1 log to be deleted, got %d", cleanupResult.DeletedCount)
	}

	// Verify remaining logs: newer logs (v2 and v3) should still be in the DB
	remainingLogs, err := model.ListApplyLogs(model.ApplyLogQuery{NodeID: "node-logs", PageNo: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListApplyLogs failed: %v", err)
	}
	if len(remainingLogs) != 2 {
		t.Errorf("expected 2 remaining logs, got %d", len(remainingLogs))
	}

	// 3. Test Cleanup with DeleteAll = true
	cleanupAll, err := CleanupApplyLogs(ApplyLogCleanupInput{
		DeleteAll: true,
	})
	if err != nil {
		t.Fatalf("CleanupApplyLogs deleteAll failed: %v", err)
	}
	if cleanupAll.DeletedCount != 2 {
		t.Errorf("expected 2 remaining logs to be deleted, got %d", cleanupAll.DeletedCount)
	}

	// Verify DB is empty of apply logs
	finalLogs, err := model.ListApplyLogs(model.ApplyLogQuery{NodeID: "node-logs", PageNo: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListApplyLogs failed: %v", err)
	}
	if len(finalLogs) != 0 {
		t.Errorf("expected 0 remaining logs, got %d", len(finalLogs))
	}
}
