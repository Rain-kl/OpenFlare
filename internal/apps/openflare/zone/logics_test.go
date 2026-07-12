// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package zone

import (
	"context"
	"testing"

	"github.com/Rain-kl/Wavelet/internal/db"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupZoneDB(t *testing.T) context.Context {
	t.Helper()
	conn, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	require.NoError(t, err)
	require.NoError(t, conn.AutoMigrate(&model.Zone{}, &model.ZoneDomain{}, &model.TLSCertificate{}))
	db.SetDB(conn)
	t.Cleanup(func() { db.SetDB(nil) })
	return context.Background()
}

func TestCreateZoneDomainRejectsWildcard(t *testing.T) {
	ctx := setupZoneDB(t)
	zone, err := Create(ctx, Input{Domain: "example.com"})
	require.NoError(t, err)
	_, err = CreateDomain(ctx, zone.ID, DomainInput{Domain: "*.example.com"})
	require.EqualError(t, err, errDomainWildcardUnsupported)
}

func TestLegacyImportUsesEffectiveTLDPlusOne(t *testing.T) {
	root, err := zoneRoot("api.example.co.uk")
	require.NoError(t, err)
	require.Equal(t, "example.co.uk", root)
}
