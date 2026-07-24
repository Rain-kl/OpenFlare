package config

import "testing"

func TestApplyEnvOverridesRedisMaintNotifications(t *testing.T) {
	t.Setenv("REDIS_MAINT_NOTIFICATIONS", "true")

	cfg := &configModel{}
	applyEnvOverrides(cfg)

	if !cfg.Redis.MaintNotifications {
		t.Fatal("REDIS_MAINT_NOTIFICATIONS=true was not applied")
	}
}
