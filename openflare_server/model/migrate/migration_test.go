package migrate

import "testing"

func TestMigrationsAreContinuousFromBaseVersion(t *testing.T) {
	migrations := Migrations()
	if len(migrations) == 0 {
		t.Fatal("expected at least one registered migration")
	}
	expectedFrom := BaseDatabaseSchemaVersion
	for _, migration := range migrations {
		if migration.FromVersion != expectedFrom {
			t.Fatalf("expected migration from v%d, got v%d -> v%d", expectedFrom, migration.FromVersion, migration.ToVersion)
		}
		if migration.ToVersion != migration.FromVersion+1 {
			t.Fatalf("expected one-step migration, got v%d -> v%d", migration.FromVersion, migration.ToVersion)
		}
		expectedFrom = migration.ToVersion
	}
	if CurrentVersion() != expectedFrom {
		t.Fatalf("unexpected current version: got %d want %d", CurrentVersion(), expectedFrom)
	}
}
