package geoipdata

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/oschwald/maxminddb-golang"
)

func TestEmbeddedDatabasesAreValid(t *testing.T) {
	tests := []struct {
		name             string
		filename         string
		databaseTypePart string
	}{
		{name: "Country", filename: DefaultMMDBName, databaseTypePart: "Country"},
		{name: "City", filename: DefaultCityMMDBName, databaseTypePart: "City"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, err := fs.ReadFile(FS, test.filename)
			if err != nil {
				t.Fatalf("read embedded database: %v", err)
			}
			reader, err := maxminddb.FromBytes(data)
			if err != nil {
				t.Fatalf("open embedded database: %v", err)
			}
			defer reader.Close()

			if !strings.Contains(reader.Metadata.DatabaseType, test.databaseTypePart) {
				t.Fatalf("unexpected database type %q", reader.Metadata.DatabaseType)
			}
		})
	}
}
