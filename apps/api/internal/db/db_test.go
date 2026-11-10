package db

import (
	"path/filepath"
	"testing"
)

// TestPlaceholderMigrationKeepsReferences applies 0001, adds a video with a share code and a grant,
// then applies the rest. Rebuilding the videos table must not cascade deletes into qr_codes or access_grants.
func TestPlaceholderMigrationKeepsReferences(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	body, err := migrations.ReadFile("migrations/0001_init.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`CREATE TABLE schema_migrations (version TEXT PRIMARY KEY, applied_at INTEGER NOT NULL)`,
		string(body),
		`INSERT INTO schema_migrations VALUES ('0001_init.sql', 0)`,
		`INSERT INTO users VALUES ('u1', 'a1', NULL, 'Creator', 'creator', 0, 0), ('u2', 'a2', NULL, 'Viewer', 'viewer', 0, 0)`,
		`INSERT INTO directories VALUES ('d1', 'u1', NULL, 'Root', 0)`,
		`INSERT INTO videos VALUES ('v1', 'u1', 'd1', 'Week 1', '123', NULL, 'ready', 60, 0, 0)`,
		`INSERT INTO qr_codes VALUES ('q1', 'code', 'u1', NULL, 'v1', NULL, NULL, NULL, 0)`,
		`INSERT INTO access_grants VALUES ('g1', 'u2', 'u1', NULL, 'v1', 'q1', NULL, 0, NULL)`,
	} {
		if _, err := d.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}

	if err := Migrate(d); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	for table, want := range map[string]int{"videos": 1, "qr_codes": 1, "access_grants": 1} {
		var n int
		if err := d.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != want {
			t.Errorf("%s rows = %d, want %d", table, n, want)
		}
	}
	if _, err := d.Exec(`INSERT INTO videos (id, owner_id, directory_id, title, vimeo_id, vimeo_hash, status, duration_sec, created_at, updated_at) VALUES ('v2', 'u1', 'd1', 'Later', NULL, NULL, 'placeholder', NULL, 0, 0)`); err != nil {
		t.Errorf("placeholder status rejected: %v", err)
	}

	// Foreign keys must be on again for every pooled connection, including the one Migrate used.
	d.SetMaxOpenConns(1)
	var fk int
	if err := d.QueryRow(`PRAGMA foreign_keys`).Scan(&fk); err != nil || fk != 1 {
		t.Errorf("foreign_keys = %d (%v), want 1", fk, err)
	}
	if _, err := d.Exec(`DELETE FROM videos WHERE id = 'v1'`); err != nil {
		t.Fatal(err)
	}
	var grants int
	d.QueryRow(`SELECT COUNT(*) FROM access_grants`).Scan(&grants)
	if grants != 0 {
		t.Errorf("grant not deleted with its video: foreign keys are off")
	}
}
