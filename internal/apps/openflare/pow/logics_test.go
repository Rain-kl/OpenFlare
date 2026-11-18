// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package pow

import (
	"context"
	"errors"
	"testing"

	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	openresty "github.com/Rain-kl/Wavelet/internal/openflare/openresty"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestIndependentPolicyLifecycle(t *testing.T) {
	conn, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.AutoMigrate(&model.ProxyRoute{}, &model.OpenFlarePoWPolicy{}, &model.ZoneDomain{}); err != nil {
		t.Fatal(err)
	}
	db.SetDB(conn)
	t.Cleanup(func() {
		db.SetDB(nil)
		sqlDB, err := conn.DB()
		if err == nil {
			if err := sqlDB.Close(); err != nil {
				t.Error(err)
			}
		}
	})
	for _, id := range []uint{1, 2} {
		if err := conn.Create(&model.ProxyRoute{ID: id, SiteName: "site", OriginURL: "http://127.0.0.1"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	config := openresty.DefaultPoWConfig()
	config.PathRules = &openresty.PoWPathRules{Mode: "blacklist", Blacklist: []openresty.PoWPathMatch{{Type: "prefix", Value: "/admin/"}}}
	input := Policy{Enabled: true, Config: config}
	if err := SavePolicy(ctx, 1, input); err != nil {
		t.Fatalf("SavePolicy(route 1) error = %v", err)
	}
	input.Enabled = false
	if err := SavePolicy(ctx, 2, input); err != nil {
		t.Fatal(err)
	}
	policies, err := ListPolicies(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(policies) != 2 {
		t.Fatalf("ListPolicies() = %+v, want 2 policies", policies)
	}
	if !policies[0].Enabled || policies[1].Enabled || policies[0].Config.PathRules.Blacklist[0].Value != "/admin/" {
		t.Errorf("ListPolicies() = %+v, want isolated enabled/disabled policies with persisted paths", policies)
	}
	input.Config.Difficulty = 0
	var validation *ValidationError
	if err := SavePolicy(ctx, 1, input); !errors.As(err, &validation) {
		t.Errorf("SavePolicy(invalid difficulty) error = %v, want ValidationError", err)
	}
	policies, err = ListPolicies(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !policies[0].Enabled {
		t.Error("SavePolicy(invalid) changed saved enabled policy")
	}
	if err := repository.DeleteProxyRouteAndUnbind(ctx, 1); err != nil {
		t.Fatal(err)
	}
	policies, err = ListPolicies(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(policies) != 1 || policies[0].RouteID != 2 {
		t.Errorf("ListPolicies(after site deletion) = %+v, want only route 2", policies)
	}
}
