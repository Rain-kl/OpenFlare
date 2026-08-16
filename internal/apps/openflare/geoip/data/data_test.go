// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package data

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/oschwald/maxminddb-golang"
)

func TestEmbeddedCountryDatabaseIsValid(t *testing.T) {
	raw, err := fs.ReadFile(FS, DefaultMMDBName)
	if err != nil {
		t.Fatalf("read embedded Country database: %v", err)
	}
	reader, err := maxminddb.FromBytes(raw)
	if err != nil {
		t.Fatalf("open embedded Country database: %v", err)
	}
	defer reader.Close()

	if !strings.Contains(reader.Metadata.DatabaseType, "Country") {
		t.Fatalf("unexpected database type %q", reader.Metadata.DatabaseType)
	}
}
