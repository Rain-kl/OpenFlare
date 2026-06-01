package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"openflare/model"
	"strings"
	"testing"
	"time"
)

func TestWAFRuleGroupValidationAndNormalization(t *testing.T) {
	setupServiceTestDB(t)

	group, err := CreateWAFRuleGroup(WAFRuleGroupInput{
		Name:             "edge guard",
		Enabled:          true,
		BlockStatusCode:  451,
		IPWhitelist:      []string{" 192.0.2.1 ", "192.0.2.1", "198.51.100.0/24"},
		IPBlacklist:      []string{"203.0.113.10"},
		CountryBlacklist: []string{" cn ", "CN", "us"},
	})
	if err != nil {
		t.Fatalf("CreateWAFRuleGroup failed: %v", err)
	}
	if len(group.IPWhitelist) != 2 || group.IPWhitelist[0] != "192.0.2.1" || group.IPWhitelist[1] != "198.51.100.0/24" {
		t.Fatalf("unexpected normalized ip whitelist: %#v", group.IPWhitelist)
	}
	if len(group.CountryBlacklist) != 2 || group.CountryBlacklist[0] != "CN" || group.CountryBlacklist[1] != "US" {
		t.Fatalf("unexpected normalized countries: %#v", group.CountryBlacklist)
	}

	if _, err = CreateWAFRuleGroup(WAFRuleGroupInput{
		Name:        "bad ip",
		Enabled:     true,
		IPBlacklist: []string{"not-an-ip"},
	}); err == nil {
		t.Fatal("expected invalid IP to be rejected")
	}
}

func TestWAFGlobalGroupAndBindings(t *testing.T) {
	setupServiceTestDB(t)

	groups, err := ListWAFRuleGroups()
	if err != nil {
		t.Fatalf("ListWAFRuleGroups failed: %v", err)
	}
	if len(groups) == 0 || !groups[0].IsGlobal {
		t.Fatalf("expected default global WAF rule group, got %#v", groups)
	}
	if err = DeleteWAFRuleGroup(groups[0].ID); err == nil {
		t.Fatal("expected global WAF rule group delete to be rejected")
	}

	route, err := CreateProxyRoute(ProxyRouteInput{
		SiteName:  "waf-site",
		Domains:   []string{"waf.example.com"},
		OriginURL: "https://origin.internal",
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("CreateProxyRoute failed: %v", err)
	}
	custom, err := CreateWAFRuleGroup(WAFRuleGroupInput{
		Name:            "custom",
		Enabled:         true,
		BlockStatusCode: 418,
		IPBlacklist:     []string{"203.0.113.10"},
	})
	if err != nil {
		t.Fatalf("CreateWAFRuleGroup failed: %v", err)
	}
	if _, err = ReplaceWAFRuleGroupSites(custom.ID, []uint{route.ID}); err != nil {
		t.Fatalf("ReplaceWAFRuleGroupSites failed: %v", err)
	}
	siteGroups, err := GetWAFSiteRuleGroups(route.ID)
	if err != nil {
		t.Fatalf("GetWAFSiteRuleGroups failed: %v", err)
	}
	if len(siteGroups.AppliedIDs) != 1 || siteGroups.AppliedIDs[0] != custom.ID {
		t.Fatalf("unexpected site WAF bindings: %#v", siteGroups.AppliedIDs)
	}
}

func TestPublishConfigVersionIncludesWAFSnapshotAndRuntimeConfig(t *testing.T) {
	setupServiceTestDB(t)

	route, err := CreateProxyRoute(ProxyRouteInput{
		SiteName:  "waf-publish",
		Domains:   []string{"waf-publish.example.com"},
		OriginURL: "https://origin.internal",
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("CreateProxyRoute failed: %v", err)
	}
	group, err := CreateWAFRuleGroup(WAFRuleGroupInput{
		Name:            "publish group",
		Enabled:         true,
		BlockStatusCode: 451,
		IPBlacklist:     []string{"203.0.113.0/24"},
	})
	if err != nil {
		t.Fatalf("CreateWAFRuleGroup failed: %v", err)
	}
	if _, err = ReplaceWAFSiteRuleGroups(route.ID, []uint{group.ID}); err != nil {
		t.Fatalf("ReplaceWAFSiteRuleGroups failed: %v", err)
	}
	result, err := PublishConfigVersion("root", false)
	if err != nil {
		t.Fatalf("PublishConfigVersion failed: %v", err)
	}
	if !strings.Contains(result.Version.RenderedConfig, "access_by_lua_file __OPENFLARE_LUA_DIR__/waf/check.lua;") {
		t.Fatal("expected route config to include WAF lua access hook")
	}
	if !strings.Contains(result.Version.SnapshotJSON, `"waf"`) {
		t.Fatal("expected snapshot to include waf document")
	}
	var files []SupportFile
	if err = json.Unmarshal([]byte(result.Version.SupportFilesJSON), &files); err != nil {
		t.Fatalf("decode support files failed: %v", err)
	}
	found := false
	for _, file := range files {
		if file.Path == "waf_config.json" && strings.Contains(file.Content, "203.0.113.0/24") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected waf_config.json support file, got %#v", files)
	}
}

func TestWAFIPGroupCRUDAndRuleGroupReference(t *testing.T) {
	setupServiceTestDB(t)

	ipGroup, err := CreateWAFIPGroup(WAFIPGroupInput{
		Name:    "bad actors",
		Type:    WAFIPGroupTypeManual,
		Enabled: true,
		IPList:  []string{"203.0.113.10", "203.0.113.10", "198.51.100.0/24"},
	})
	if err != nil {
		t.Fatalf("CreateWAFIPGroup failed: %v", err)
	}
	if len(ipGroup.IPList) != 2 {
		t.Fatalf("unexpected normalized IP group list: %#v", ipGroup.IPList)
	}

	group, err := CreateWAFRuleGroup(WAFRuleGroupInput{
		Name:              "referenced",
		Enabled:           true,
		BlockStatusCode:   403,
		IPBlacklistGroups: []uint{ipGroup.ID},
	})
	if err != nil {
		t.Fatalf("CreateWAFRuleGroup failed: %v", err)
	}
	if len(group.IPBlacklistGroups) != 1 || group.IPBlacklistGroups[0] != ipGroup.ID {
		t.Fatalf("unexpected blacklist group refs: %#v", group.IPBlacklistGroups)
	}
	if err = DeleteWAFIPGroup(ipGroup.ID); err == nil {
		t.Fatal("expected referenced IP group delete to be rejected")
	}
}

func TestWAFIPGroupSubscriptionParsers(t *testing.T) {
	textItems, err := parseWAFIPGroupSubscription([]byte("# comment\n203.0.113.10\n\n198.51.100.0/24\n"), "text", "")
	if err != nil {
		t.Fatalf("parse text subscription failed: %v", err)
	}
	if len(textItems) != 2 || textItems[0] != "198.51.100.0/24" || textItems[1] != "203.0.113.10" {
		t.Fatalf("unexpected text subscription items: %#v", textItems)
	}

	jsonItems, err := parseWAFIPGroupSubscription([]byte(`{"data":{"items":[{"ip":"203.0.113.11"},{"ip":"203.0.113.12"}]}}`), "json", "data.items[].ip")
	if err != nil {
		t.Fatalf("parse json subscription failed: %v", err)
	}
	if len(jsonItems) != 2 || jsonItems[0] != "203.0.113.11" || jsonItems[1] != "203.0.113.12" {
		t.Fatalf("unexpected json subscription items: %#v", jsonItems)
	}
}

func TestSyncWAFIPGroupDownloadsSubscription(t *testing.T) {
	setupServiceTestDB(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("203.0.113.20\n"))
	}))
	defer server.Close()

	group, err := CreateWAFIPGroup(WAFIPGroupInput{
		Name:                "subscription",
		Type:                WAFIPGroupTypeSubscription,
		Enabled:             true,
		SubscriptionURL:     server.URL,
		SubscriptionFormat:  WAFIPGroupSubscriptionFormatText,
		SyncIntervalMinutes: 10,
	})
	if err != nil {
		t.Fatalf("CreateWAFIPGroup failed: %v", err)
	}
	result, err := SyncWAFIPGroup(group.ID)
	if err != nil {
		t.Fatalf("SyncWAFIPGroup failed: %v", err)
	}
	if result.IPCount != 1 || result.Group.IPList[0] != "203.0.113.20" {
		t.Fatalf("unexpected sync result: %+v", result)
	}
}

func TestSyncWAFIPGroupAutomaticExprRules(t *testing.T) {
	setupServiceTestDB(t)

	now := time.Now().UTC()
	seedWAFNodeAccessLogs(t, now, "203.0.113.10", "app.example.com", 101, 81)
	seedWAFNodeAccessLogs(t, now, "203.0.113.11", "198.51.100.10", 60, 0)
	seedWAFNodeAccessLogs(t, now, "203.0.113.12", "app.example.com", 120, 10)

	group, err := CreateWAFIPGroup(WAFIPGroupInput{
		Name:    "auto blacklist",
		Type:    WAFIPGroupTypeAutomatic,
		Enabled: true,
		AutoConfig: json.RawMessage(`{
			"lookback_minutes": 60,
			"rules": [
				{"name":"单 IP 404 高频扫描","expr":"request_count > 100 && status_404_ratio >= 0.8"},
				{"name":"单 IP 直连访问异常","expr":"ip_host_count > 50 && ip_host_ratio > 0.5"}
			]
		}`),
	})
	if err != nil {
		t.Fatalf("CreateWAFIPGroup failed: %v", err)
	}
	result, err := SyncWAFIPGroup(group.ID)
	if err != nil {
		t.Fatalf("SyncWAFIPGroup failed: %v", err)
	}
	if result.IPCount != 2 {
		t.Fatalf("expected two matched IPs, got %#v", result)
	}
	want := map[string]bool{"203.0.113.10": true, "203.0.113.11": true}
	for _, item := range result.Group.IPList {
		if !want[item] {
			t.Fatalf("unexpected matched IP %s in %#v", item, result.Group.IPList)
		}
		delete(want, item)
	}
	if len(want) != 0 {
		t.Fatalf("missing matched IPs: %#v", want)
	}
}

func TestWAFIPGroupAutoConfigReturnsMatchedIPs(t *testing.T) {
	setupServiceTestDB(t)

	now := time.Now().UTC()
	seedWAFNodeAccessLogs(t, now, "203.0.113.10", "app.example.com", 101, 81)
	seedWAFNodeAccessLogs(t, now, "203.0.113.11", "198.51.100.10", 60, 0)
	seedWAFNodeAccessLogs(t, now, "203.0.113.12", "app.example.com", 120, 10)

	result, err := TestWAFIPGroupAutoConfig(WAFIPGroupAutoTestInput{
		AutoConfig: json.RawMessage(`{
			"lookback_minutes": 60,
			"rules": [
				{"name":"单 IP 404 高频扫描","expr":"request_count > 100 && status_404_ratio >= 0.8"},
				{"name":"单 IP 直连访问异常","expr":"ip_host_count > 50 && ip_host_ratio > 0.5"}
			]
		}`),
	})
	if err != nil {
		t.Fatalf("TestWAFIPGroupAutoConfig failed: %v", err)
	}
	if result.MatchedCount != 2 || result.RuleCount != 2 || result.LookbackMinutes != 60 {
		t.Fatalf("unexpected test result: %+v", result)
	}
	want := map[string]bool{"203.0.113.10": true, "203.0.113.11": true}
	for _, item := range result.MatchedIPs {
		if !want[item] {
			t.Fatalf("unexpected matched IP %s in %#v", item, result.MatchedIPs)
		}
		delete(want, item)
	}
	if len(want) != 0 {
		t.Fatalf("missing matched IPs: %#v", want)
	}
}

func TestWAFIPGroupAutomaticRejectsInvalidExpr(t *testing.T) {
	setupServiceTestDB(t)

	if _, err := CreateWAFIPGroup(WAFIPGroupInput{
		Name:    "bad auto",
		Type:    WAFIPGroupTypeAutomatic,
		Enabled: true,
		AutoConfig: json.RawMessage(`{
			"rules": [{"name":"bad","expr":"request_count > "}]
		}`),
	}); err == nil {
		t.Fatal("expected invalid Expr to be rejected")
	}
}

func TestPublishConfigVersionKeepsWAFIPGroupReferences(t *testing.T) {
	setupServiceTestDB(t)

	route, err := CreateProxyRoute(ProxyRouteInput{
		SiteName:  "waf-ip-groups",
		Domains:   []string{"waf-ip-groups.example.com"},
		OriginURL: "https://origin.internal",
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("CreateProxyRoute failed: %v", err)
	}
	ipGroup, err := CreateWAFIPGroup(WAFIPGroupInput{
		Name:    "publish refs",
		Type:    WAFIPGroupTypeManual,
		Enabled: true,
		IPList:  []string{"203.0.113.30"},
	})
	if err != nil {
		t.Fatalf("CreateWAFIPGroup failed: %v", err)
	}
	ruleGroup, err := CreateWAFRuleGroup(WAFRuleGroupInput{
		Name:              "publish group refs",
		Enabled:           true,
		BlockStatusCode:   451,
		IPBlacklistGroups: []uint{ipGroup.ID},
	})
	if err != nil {
		t.Fatalf("CreateWAFRuleGroup failed: %v", err)
	}
	if _, err = ReplaceWAFSiteRuleGroups(route.ID, []uint{ruleGroup.ID}); err != nil {
		t.Fatalf("ReplaceWAFSiteRuleGroups failed: %v", err)
	}
	result, err := PublishConfigVersion("root", false)
	if err != nil {
		t.Fatalf("PublishConfigVersion failed: %v", err)
	}
	if !strings.Contains(result.Version.SnapshotJSON, `"ip_groups"`) {
		t.Fatal("expected snapshot to include waf ip groups")
	}
	if strings.Contains(result.Version.SnapshotJSON, "203.0.113.30") {
		t.Fatal("expected snapshot to avoid embedding waf ip group members")
	}
	var files []SupportFile
	if err = json.Unmarshal([]byte(result.Version.SupportFilesJSON), &files); err != nil {
		t.Fatalf("decode support files failed: %v", err)
	}
	foundReference := false
	for _, file := range files {
		if file.Path == "waf_config.json" {
			if strings.Contains(file.Content, "203.0.113.30") {
				t.Fatalf("expected waf_config.json to avoid expanded IP group members, got %s", file.Content)
			}
			if strings.Contains(file.Content, `"ip_blacklist_group_ids":[`) {
				foundReference = true
			}
		}
	}
	if !foundReference {
		t.Fatalf("expected IP group reference in waf_config.json, got %#v", files)
	}
}

func TestWAFIPGroupAutomaticTTLExpiration(t *testing.T) {
	setupServiceTestDB(t)

	now := time.Now().UTC()
	// Seed access logs at now
	seedWAFNodeAccessLogs(t, now, "203.0.113.10", "app.example.com", 120, 100)

	group, err := CreateWAFIPGroup(WAFIPGroupInput{
		Name:    "auto ttl blacklist",
		Type:    WAFIPGroupTypeAutomatic,
		Enabled: true,
		AutoConfig: json.RawMessage(`{
			"lookback_minutes": 60,
			"ttl": 10,
			"rules": [
				{"name":"404 Scan","expr":"request_count > 100 && status_404_ratio >= 0.8"}
			]
		}`),
	})
	if err != nil {
		t.Fatalf("CreateWAFIPGroup failed: %v", err)
	}

	groupModel, err := model.GetWAFIPGroupByID(group.ID)
	if err != nil {
		t.Fatalf("GetWAFIPGroupByID failed: %v", err)
	}

	// First Sync (at now): should match 203.0.113.10
	res1, err := syncWAFIPGroup(groupModel, now)
	if err != nil {
		t.Fatalf("First Sync failed: %v", err)
	}
	if res1.IPCount != 1 || res1.Group.IPList[0] != "203.0.113.10" {
		t.Fatalf("expected 203.0.113.10 to be blacklisted, got: %#v", res1.Group.IPList)
	}
	if len(res1.Group.ExtIPs) != 1 || res1.Group.ExtIPs[0].IP != "203.0.113.10" {
		t.Fatalf("expected 203.0.113.10 to be in ExtIPs, got: %#v", res1.Group.ExtIPs)
	}

	// Second Sync (65 minutes later):
	// Since 65 minutes is outside the 60 minutes lookback window, the original logs won't match.
	// And since 65 minutes > 10s TTL, it should be expired and removed!
	futureTime := now.Add(65 * time.Minute)
	res2, err := syncWAFIPGroup(groupModel, futureTime)
	if err != nil {
		t.Fatalf("Second Sync failed: %v", err)
	}
	if res2.IPCount != 0 {
		t.Fatalf("expected IP to be expired and removed, got: %#v", res2.Group.IPList)
	}
	if len(res2.Group.ExtIPs) != 0 {
		t.Fatalf("expected ExtIPs to be empty after expiration, got: %#v", res2.Group.ExtIPs)
	}

	// Third Sync: test lease refresh / extension!
	// Re-run sync at now to get it captured again first
	_, err = syncWAFIPGroup(groupModel, now)
	if err != nil {
		t.Fatalf("Re-sync at now failed: %v", err)
	}

	// Now run sync at now + 5 seconds (5s < 10s TTL, so not expired, but matched again!):
	// Since it matches again, it should keep the IP active and extend CapturedAt to now + 5s!
	futureTime2 := now.Add(5 * time.Second)
	res3, err := syncWAFIPGroup(groupModel, futureTime2)
	if err != nil {
		t.Fatalf("Third Sync failed: %v", err)
	}
	if res3.IPCount != 1 || res3.Group.IPList[0] != "203.0.113.10" {
		t.Fatalf("expected IP to remain active, got: %#v", res3.Group.IPList)
	}
	if len(res3.Group.ExtIPs) != 1 || res3.Group.ExtIPs[0].CapturedAt != futureTime2.Format(time.RFC3339) {
		t.Fatalf("expected CapturedAt to be updated to %v, got %v", futureTime2.Format(time.RFC3339), res3.Group.ExtIPs[0].CapturedAt)
	}
}

func seedWAFNodeAccessLogs(t *testing.T, loggedAt time.Time, remoteAddr string, host string, total int, notFound int) {
	t.Helper()
	for i := 0; i < total; i++ {
		statusCode := http.StatusOK
		if i < notFound {
			statusCode = http.StatusNotFound
		}
		if err := model.DB.Create(&model.NodeAccessLog{
			NodeID:     "node-waf-auto",
			LoggedAt:   loggedAt.Add(-time.Duration(i%30) * time.Second),
			RemoteAddr: remoteAddr,
			Host:       host,
			Path:       "/probe",
			StatusCode: statusCode,
		}).Error; err != nil {
			t.Fatalf("failed to seed access log: %v", err)
		}
	}
}
