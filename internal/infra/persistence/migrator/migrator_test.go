// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package migrator

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"slices"
	"strings"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/infra/config"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/openflare/edgeconfig"
	"github.com/Rain-kl/Wavelet/internal/openflare/performance"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/pressly/goose/v3"
	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// expectedMigratedSystemConfigCount 为全新库执行全部迁移后 w_system_configs 的行数
// （初始系统配置 + 各期配置迁移/新增 seed：of_options 迁移、文件白名单、磁盘缓存、
// 登录会话 TTL、升级源、存储、FRPS Web UI、Pages、OpenResty 限流、单 IP 限频、
// 错误页、SW 离线、日志保留期、指标保留期等）；新增配置 seed 迁移时需同步更新本常量。
const expectedMigratedSystemConfigCount = 66

func TestTrustedProxyCIDRMigrationDefaultsMatchRenderer(t *testing.T) {
	want := edgeconfig.DefaultTrustedProxyCIDRs()
	if len(want) != 22 {
		t.Fatalf("renderer default contains %d proxy ranges, want 22", len(want))
	}
	for _, migrationPath := range []string{
		"goose/postgres/202610070005_add_openresty_trusted_proxy_cidrs.sql",
		"goose/sqlite/202610070005_add_openresty_trusted_proxy_cidrs.sql",
	} {
		migration, err := fs.ReadFile(migrationFS, migrationPath)
		if err != nil {
			t.Fatal(err)
		}
		seed, ok := strings.CutPrefix(string(migration), "-- +goose Up\nINSERT INTO w_system_configs (key, value, type, visibility, description, created_at, updated_at)\nVALUES ('openresty_trusted_proxy_cidrs', '")
		if !ok {
			t.Fatalf("%s does not contain the expected CIDR seed", migrationPath)
		}
		seed, _, ok = strings.Cut(seed, "', 'business'")
		if !ok {
			t.Fatalf("%s CIDR seed value is malformed", migrationPath)
		}
		var got []string
		if err := json.Unmarshal([]byte(seed), &got); err != nil {
			t.Fatalf("decode %s CIDR seed: %v", migrationPath, err)
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s CIDR seed differs from renderer default", migrationPath)
		}
	}
}

func TestGooseMigrationVersionsAreUniquePerDialect(t *testing.T) {
	for _, dir := range []string{"goose/postgres", "goose/sqlite"} {
		files, err := fs.Glob(migrationFS, dir+"/*.sql")
		if err != nil {
			t.Fatalf("glob %s migrations: %v", dir, err)
		}

		versions := make(map[string]string, len(files))
		for _, file := range files {
			name := strings.TrimPrefix(file, dir+"/")
			version, _, ok := strings.Cut(name, "_")
			if !ok {
				t.Errorf("migration %q does not include a version prefix", file)
				continue
			}
			if previous, exists := versions[version]; exists {
				t.Errorf("%s has duplicate migration version %s in %q and %q", dir, version, previous, name)
				continue
			}
			versions[version] = name
		}
	}
}

func TestMigrateInitializesSQLiteDatabase(t *testing.T) {
	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("gorm.Open(sqlite) error = %v", err)
	}

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() error = %v", err)
	}
	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
		MaintNotificationsConfig: &maintnotifications.Config{
			Mode: maintnotifications.ModeDisabled,
		},
	})

	previousDBEnabled := config.Config.Database.Enabled
	config.Config.Database.Enabled = false
	db.SetDB(sqliteDB)
	t.Cleanup(func() {
		config.Config.Database.Enabled = previousDBEnabled
		db.SetDB(nil)
		_ = redisClient.Close()
		mr.Close()
	})

	Migrate()
	assertScheduleLogDefaults(t)

	var systemConfigCount int64
	if err := sqliteDB.Table("w_system_configs").Count(&systemConfigCount).Error; err != nil {
		t.Fatalf("Migrate() count w_system_configs error = %v", err)
	}
	if systemConfigCount != expectedMigratedSystemConfigCount {
		t.Errorf("Migrate() w_system_configs count = %d, want %d", systemConfigCount, expectedMigratedSystemConfigCount)
	}
	ctx := context.Background()
	assertWAFWhitelistTriggers(t, ctx)
	policies, _, err := repository.ListPerformancePolicies(ctx)
	if err != nil || len(policies) != 4 {
		t.Fatalf("performance policy seeds: %v, %v", policies, err)
	}
	for index, policy := range policies[:3] {
		resolved, err := performance.Decode(policy.SettingsJSON)
		if err != nil || policy.IsDefault() || resolved["threads"] != []int{1, 2, 4}[index] {
			t.Fatalf("invalid preset %s: %v", policy.Name, err)
		}
	}
	defaultPolicy := policies[3]
	resolved, err := performance.Decode(defaultPolicy.SettingsJSON)
	if err != nil || !defaultPolicy.IsDefault() || resolved["threads_auto"] != true {
		t.Fatalf("invalid default performance policy: %v, %v", defaultPolicy, err)
	}
	for _, node := range []model.OpenFlareNode{
		{NodeID: "test-pingora", Name: "Pingora", Engine: "pingora"},
		{NodeID: "test-openresty", Name: "OpenResty", Engine: "openresty"},
	} {
		if err := repository.CreateOpenFlareNode(ctx, &node); err != nil {
			t.Fatal(err)
		}
	}
	id := policies[0].ID
	if err := repository.AssignPerformancePolicy(ctx, id, []string{"test-pingora"}); err != nil {
		t.Fatal(err)
	}
	if err := repository.AssignPerformancePolicy(ctx, id, []string{"test-openresty"}); !errors.Is(err, repository.ErrInvalidPerformancePolicyNodes) {
		t.Fatalf("expected invalid node assignment, got %v", err)
	}
	if _, err := repository.GetNodePerformancePolicy(ctx, "test-pingora"); err != nil {
		t.Fatalf("rejected bulk assignment removed a valid assignment: %v", err)
	}
	if err := repository.DeletePerformancePolicy(ctx, id); err != nil {
		t.Fatal(err)
	}
	settings, err := repository.GetNodePerformancePolicy(ctx, "test-pingora")
	if err != nil || settings != defaultPolicy.SettingsJSON {
		t.Fatalf("expected node to inherit the default policy, got %q, %v", settings, err)
	}
	if _, err := repository.GetNodePerformancePolicy(ctx, "test-openresty"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("non-Pingora node was eligible for a performance policy: %v", err)
	}

	var trustedProxyConfig model.SystemConfig
	if err := sqliteDB.Where("key = ?", model.ConfigKeyEdgeTrustedProxyCIDRs).First(&trustedProxyConfig).Error; err != nil {
		t.Fatalf("Migrate() trusted proxy CIDR config error = %v", err)
	}
	var trustedProxyCIDRs []string
	if err := json.Unmarshal([]byte(trustedProxyConfig.Value), &trustedProxyCIDRs); err != nil {
		t.Fatalf("decode trusted proxy CIDR config: %v", err)
	}
	if !slices.Equal(trustedProxyCIDRs, edgeconfig.DefaultTrustedProxyCIDRs()) {
		t.Errorf("Migrate() trusted proxy CIDRs = %#v, want default Cloudflare ranges", trustedProxyCIDRs)
	}

	var logMigrationConfig model.SystemConfig
	if err := sqliteDB.Where("key = ?", model.ConfigKeyLogDBMigration).First(&logMigrationConfig).Error; err != nil {
		t.Fatalf("Migrate() log migration config error = %v", err)
	}
	if logMigrationConfig.Value != "" || logMigrationConfig.Type != "system" || logMigrationConfig.Visibility != model.ConfigVisibilityHidden {
		t.Errorf("Migrate() log migration config = %+v, want an empty hidden system config", logMigrationConfig)
	}

	var adminCount int64
	if err := sqliteDB.Table("w_users").Where("username = ?", "admin").Count(&adminCount).Error; err != nil {
		t.Fatalf("Migrate() count admin user error = %v", err)
	}
	if adminCount != 1 {
		t.Errorf("Migrate() admin user count = %d, want %d", adminCount, 1)
	}

	var templateCount int64
	if err := sqliteDB.Table("w_templates").Count(&templateCount).Error; err != nil {
		t.Fatalf("Migrate() count templates error = %v", err)
	}
	if templateCount != 2 {
		t.Errorf("Migrate() templates count = %d, want %d", templateCount, 2)
	}

	if !sqliteDB.Migrator().HasTable("of_zones") {
		t.Error("Migrate() did not create of_zones")
	}
	if !sqliteDB.Migrator().HasTable("of_zone_domains") {
		t.Error("Migrate() did not create of_zone_domains")
	}
	for _, table := range []string{
		"of_pow_policies",
		"of_cf_connections",
		"of_cf_pointing_groups",
		"of_cf_pointing_members",
	} {
		if !sqliteDB.Migrator().HasTable(table) {
			t.Errorf("Migrate() did not create %s", table)
		}
	}
	if !sqliteDB.Migrator().HasColumn("of_cf_connections", "authorization") {
		t.Error("Migrate() did not create of_cf_connections.authorization")
	}
	if sqliteDB.Migrator().HasTable("of_managed_domains") {
		t.Error("Migrate() should drop of_managed_domains after phase-2 cleanup")
	}
	if sqliteDB.Migrator().HasColumn(&model.ProxyRoute{}, "domain") {
		t.Error("Migrate() should drop of_proxy_routes.domain after phase-2 cleanup")
	}
	if sqliteDB.Migrator().HasColumn(&model.ProxyRoute{}, "domains") {
		t.Error("Migrate() should drop of_proxy_routes.domains after phase-2 cleanup")
	}
	if sqliteDB.Migrator().HasColumn(&model.ProxyRoute{}, "cert_id") {
		t.Error("Migrate() should drop of_proxy_routes.cert_id after phase-2 cleanup")
	}
	if sqliteDB.Migrator().HasColumn(&model.ProxyRoute{}, "cert_ids") {
		t.Error("Migrate() should drop of_proxy_routes.cert_ids after phase-2 cleanup")
	}
	if sqliteDB.Migrator().HasColumn(&model.ProxyRoute{}, "domain_cert_ids") {
		t.Error("Migrate() should drop of_proxy_routes.domain_cert_ids after phase-2 cleanup")
	}

	zone := model.Zone{Domain: "example.com"}
	if err := sqliteDB.Create(&zone).Error; err != nil {
		t.Fatalf("Migrate() create Zone error = %v", err)
	}
	if err := sqliteDB.Create(&model.Zone{Domain: zone.Domain}).Error; err == nil {
		t.Error("Migrate() allowed duplicate of_zones.domain")
	}

	domain := model.ZoneDomain{ZoneID: zone.ID, Domain: "api.example.com"}
	if err := sqliteDB.Create(&domain).Error; err != nil {
		t.Fatalf("Migrate() create ZoneDomain error = %v", err)
	}
	if err := sqliteDB.Create(&model.ZoneDomain{ZoneID: zone.ID, Domain: domain.Domain}).Error; err == nil {
		t.Error("Migrate() allowed duplicate of_zone_domains.domain")
	}

	if err := sqliteDB.Exec(`INSERT INTO of_cf_pointing_members
		(group_id, zone_domain_id, proxied, cf_zone_id, cf_record_id, desired_ip, sync_status, last_error)
		VALUES (?, ?, ?, '', '', '', 'pending', '')`, 1, domain.ID, false).Error; err != nil {
		t.Fatalf("Migrate() insert Cloudflare member error = %v", err)
	}
	if err := sqliteDB.Exec(`INSERT INTO of_cf_pointing_members
		(group_id, zone_domain_id, proxied, cf_zone_id, cf_record_id, desired_ip, sync_status, last_error)
		VALUES (?, ?, ?, '', '', '', 'pending', '')`, 2, domain.ID, false).Error; err == nil {
		t.Error("Migrate() allowed duplicate of_cf_pointing_members.zone_domain_id")
	}
}

func assertScheduleLogDefaults(t *testing.T) {
	t.Helper()
	schedules, err := repository.ListSchedules(t.Context())
	require.NoError(t, err)
	var failureOnlyTypes []string
	for _, schedule := range schedules {
		if schedule.HideSuccessLog {
			failureOnlyTypes = append(failureOnlyTypes, schedule.TaskType)
		}
	}
	assert.ElementsMatch(t, []string{"of_waf_ip_group_sync", "of_uptime_kuma_sync"}, failureOnlyTypes)
}

func TestCloudflarePointingPostgresMigrationQuotesAuthorizationColumn(t *testing.T) {
	content, err := migrationFS.ReadFile("goose/postgres/202608040001_create_cloudflare_pointing.sql")
	if err != nil {
		t.Fatalf("read Cloudflare pointing migration: %v", err)
	}
	if !strings.Contains(string(content), `"authorization" TEXT NOT NULL DEFAULT ''`) {
		t.Error("Cloudflare pointing PostgreSQL migration must quote reserved column authorization")
	}
}

func TestMigrateClearsStaleSystemConfigCache(t *testing.T) {
	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("gorm.Open(sqlite) error = %v", err)
	}

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() error = %v", err)
	}
	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	previousDBEnabled := config.Config.Database.Enabled
	previousRedis := db.Redis
	config.Config.Database.Enabled = false
	db.SetDB(sqliteDB)
	db.Redis = redisClient
	t.Cleanup(func() {
		config.Config.Database.Enabled = previousDBEnabled
		db.SetDB(nil)
		db.Redis = previousRedis
		_ = redisClient.Close()
		mr.Close()
	})

	staleConfig := model.SystemConfig{
		Key:   model.ConfigKeyEmailLoginVerificationEnabled,
		Value: "true",
		Type:  "system",
	}
	if err := db.HSetJSON(context.Background(), repository.SystemConfigRedisHashKey, model.ConfigKeyEmailLoginVerificationEnabled, &staleConfig); err != nil {
		t.Fatalf("HSetJSON() error = %v", err)
	}

	Migrate()

	exists, err := db.Redis.Exists(context.Background(), db.PrefixedKey(repository.SystemConfigRedisHashKey)).Result()
	if err != nil {
		t.Fatalf("Redis.Exists() error = %v", err)
	}
	if exists != 0 {
		t.Fatalf("system config cache exists = %d, want 0", exists)
	}

	enabled, err := repository.GetBoolByKey(context.Background(), model.ConfigKeyEmailLoginVerificationEnabled)
	if err != nil {
		t.Fatalf("GetBoolByKey(%s) error = %v", model.ConfigKeyEmailLoginVerificationEnabled, err)
	}
	if enabled {
		t.Fatalf("GetBoolByKey(%s) = true, want false", model.ConfigKeyEmailLoginVerificationEnabled)
	}
}

func TestNodeQuarantineAndCertDeliveryMigrations(t *testing.T) {
	sqliteDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	sqlDB, err := sqliteDB.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() {
		db.SetDB(nil)
		_ = sqlDB.Close()
	})

	previousDBEnabled := config.Config.Database.Enabled
	config.Config.Database.Enabled = false
	db.SetDB(sqliteDB)
	t.Cleanup(func() { config.Config.Database.Enabled = previousDBEnabled })

	result := Migrate()
	require.True(t, result.Applied)

	assert.True(t, sqliteDB.Migrator().HasColumn(&model.OpenFlareNode{}, "quarantined_at"))
	assert.True(t, sqliteDB.Migrator().HasColumn(&model.OpenFlareNode{}, "quarantine_reason"))
	assert.True(t, sqliteDB.Migrator().HasIndex(&model.OpenFlareNode{}, "idx_of_nodes_quarantined_at"))
	assert.True(t, sqliteDB.Migrator().HasTable(&model.OpenFlareNodeCertDelivery{}))
	assert.True(t, sqliteDB.Migrator().HasIndex(&model.OpenFlareNodeCertDelivery{}, "idx_of_node_cert_deliveries_unique"))

	scrubDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	scrubSQLDB, err := scrubDB.DB()
	require.NoError(t, err)
	scrubSQLDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = scrubSQLDB.Close() })

	gooseFS, err := fs.Sub(migrationFS, "goose/sqlite")
	require.NoError(t, err)
	provider, err := goose.NewProvider(goose.DialectSQLite3, scrubSQLDB, gooseFS)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = provider.UpTo(ctx, 202611230001)
	require.NoError(t, err)

	var beforeRevision int64
	require.NoError(t, scrubDB.Raw("SELECT source_revision FROM of_edge_snapshot_state WHERE id = 1").Scan(&beforeRevision).Error)

	dirty := model.ConfigVersion{
		Version:          "legacy-v3",
		SnapshotJSON:     `{"snapshot":{}}`,
		SupportFilesJSON: `[{"path":"1.key","content":"-----BEGIN PRIVATE KEY-----\nsecret\n-----END PRIVATE KEY-----\n"}]`,
		Checksum:         "legacy-sum",
		PayloadChecksum:  "legacy-payload-sum",
	}
	require.NoError(t, scrubDB.Create(&dirty).Error)

	_, err = provider.Up(ctx)
	require.NoError(t, err)

	var scrubbed model.ConfigVersion
	require.NoError(t, scrubDB.Where("version = ?", dirty.Version).First(&scrubbed).Error)
	assert.Equal(t, "[]", scrubbed.SupportFilesJSON)
	assert.Empty(t, scrubbed.PayloadChecksum)

	var afterRevision int64
	require.NoError(t, scrubDB.Raw("SELECT source_revision FROM of_edge_snapshot_state WHERE id = 1").Scan(&afterRevision).Error)
	assert.Equal(t, beforeRevision+1, afterRevision)
}

func assertWAFWhitelistTriggers(t *testing.T, ctx context.Context) {
	t.Helper()
	before, err := repository.GetEdgeSnapshotState(ctx)
	require.NoError(t, err)
	entry := &model.OpenFlareWAFWhitelistEntry{Domain: "example.test", PathMatch: "prefix", Path: "/public/"}
	require.NoError(t, repository.SaveOpenFlareWAFWhitelistEntry(ctx, entry))
	entry.Path = "/webhooks/"
	require.NoError(t, repository.SaveOpenFlareWAFWhitelistEntry(ctx, entry))
	require.NoError(t, repository.DeleteOpenFlareWAFWhitelistEntry(ctx, entry.ID))
	after, err := repository.GetEdgeSnapshotState(ctx)
	require.NoError(t, err)
	require.Equal(t, before.SourceRevision+3, after.SourceRevision)
}
