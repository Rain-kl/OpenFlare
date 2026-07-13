package task

import (
	"testing"

	"github.com/Rain-kl/Wavelet/internal/config"
	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
)

func TestNewRedisConnOptConfiguresMaintenanceNotifications(t *testing.T) {
	previous := config.Config.Redis
	t.Cleanup(func() { config.Config.Redis = previous })

	for _, test := range []struct {
		name    string
		enabled bool
		want    maintnotifications.Mode
	}{
		{name: "disabled by default", enabled: false, want: maintnotifications.ModeDisabled},
		{name: "auto when enabled", enabled: true, want: maintnotifications.ModeAuto},
	} {
		t.Run(test.name, func(t *testing.T) {
			config.Config.Redis.MaintNotifications = test.enabled

			t.Run("standalone", func(t *testing.T) {
				config.Config.Redis.ClusterMode = false
				config.Config.Redis.MasterName = ""
				config.Config.Redis.Addrs = []string{"127.0.0.1:6379"}

				client, ok := NewRedisConnOpt().MakeRedisClient().(*redis.Client)
				if !ok {
					t.Fatal("standalone option did not create *redis.Client")
				}
				defer func() { _ = client.Close() }()
				assertMaintenanceNotificationsMode(t, client.Options().MaintNotificationsConfig, test.want)
			})

			t.Run("cluster", func(t *testing.T) {
				config.Config.Redis.ClusterMode = true
				config.Config.Redis.MasterName = ""
				config.Config.Redis.Addrs = []string{"127.0.0.1:6379"}

				client, ok := NewRedisConnOpt().MakeRedisClient().(*redis.ClusterClient)
				if !ok {
					t.Fatal("cluster option did not create *redis.ClusterClient")
				}
				defer func() { _ = client.Close() }()
				assertMaintenanceNotificationsMode(t, client.Options().MaintNotificationsConfig, test.want)
			})

			t.Run("sentinel", func(t *testing.T) {
				config.Config.Redis.ClusterMode = false
				config.Config.Redis.MasterName = "openflare"
				config.Config.Redis.Addrs = []string{"127.0.0.1:26379"}

				client, ok := NewRedisConnOpt().MakeRedisClient().(*redis.Client)
				if !ok {
					t.Fatal("sentinel option did not create *redis.Client")
				}
				defer func() { _ = client.Close() }()
				assertMaintenanceNotificationsMode(t, client.Options().MaintNotificationsConfig, test.want)
			})
		})
	}
}

func assertMaintenanceNotificationsMode(t *testing.T, cfg *maintnotifications.Config, want maintnotifications.Mode) {
	t.Helper()
	if cfg == nil || cfg.Mode != want {
		t.Fatalf("maintenance notifications mode = %v, want %v", cfg, want)
	}
}
