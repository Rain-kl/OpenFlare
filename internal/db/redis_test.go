package db

import (
	"testing"

	"github.com/redis/go-redis/v9/maintnotifications"
)

func TestRedisMaintNotificationsConfig(t *testing.T) {
	for _, test := range []struct {
		name    string
		enabled bool
		want    maintnotifications.Mode
	}{
		{name: "disabled by default", enabled: false, want: maintnotifications.ModeDisabled},
		{name: "auto when enabled", enabled: true, want: maintnotifications.ModeAuto},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := redisMaintNotificationsConfig(test.enabled)
			if cfg.Mode != test.want {
				t.Fatalf("maintenance notifications mode = %v, want %v", cfg.Mode, test.want)
			}
		})
	}
}
