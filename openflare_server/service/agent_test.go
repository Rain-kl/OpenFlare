package service

import "testing"

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

	if _, err := PublishConfigVersion("root"); err != nil {
		t.Fatalf("PublishConfigVersion failed: %v", err)
	}

	activeConfig, err := GetActiveConfigForAgent()
	if err != nil {
		t.Fatalf("GetActiveConfigForAgent failed: %v", err)
	}

	foundPowConfig := false
	for _, file := range activeConfig.SupportFiles {
		if file.Path != "pow_config.json" {
			continue
		}
		foundPowConfig = true
		if file.Content == "" {
			t.Fatal("expected pow_config.json content to be populated")
		}
	}
	if !foundPowConfig {
		t.Fatal("expected agent config to include pow_config.json support file")
	}
}
