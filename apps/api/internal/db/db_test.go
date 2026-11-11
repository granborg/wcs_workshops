package db

import (
	"path/filepath"
	"slices"
	"testing"
)

// TestPlaceholderMigrationKeepsReferences applies 0001, adds a video with a share code and a grant,
// then applies the rest. Rebuilding the videos table must not cascade deletes into qr_codes or access_grants.
// The later migrations must also keep the existing video and fill in its date.
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
	if _, err := d.Exec(`INSERT INTO videos (id, owner_id, directory_id, title, status, created_at, updated_at)
		VALUES ('v2', 'u1', 'd1', 'Later', 'placeholder', 0, 0)`); err != nil {
		t.Errorf("placeholder status rejected: %v", err)
	}
	// 0004 gives existing videos the UTC date of their upload (created_at 0 is 1970-01-01).
	var date string
	if err := d.QueryRow(`SELECT video_date FROM videos WHERE id = 'v1'`).Scan(&date); err != nil || date != "1970-01-01" {
		t.Errorf("video_date = %q (%v), want 1970-01-01", date, err)
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

// Another deployed build added width and height under its own migration name. Migrating this build must keep
// those columns and their values, and add only what is missing.
func TestMigrateSkipsColumnsThatExist(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	for _, name := range []string{"0001_init.sql", "0002_video_placeholder.sql", "0003_admins_upload_links.sql", "0004_video_date.sql"} {
		body, err := migrations.ReadFile("migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := d.Exec(string(body)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	for _, q := range []string{
		`CREATE TABLE schema_migrations (version TEXT PRIMARY KEY, applied_at INTEGER NOT NULL)`,
		`INSERT INTO schema_migrations VALUES ('0001_init.sql', 0), ('0002_video_placeholder.sql', 0),
			('0003_admins_upload_links.sql', 0), ('0004_video_date.sql', 0), ('0005_other_build.sql', 0)`,
		`ALTER TABLE videos ADD COLUMN width INTEGER`,
		`ALTER TABLE videos ADD COLUMN height INTEGER`,
		`INSERT INTO users VALUES ('u1', 'a1', NULL, 'Creator', 'creator', 0, 0)`,
		`INSERT INTO directories VALUES ('d1', 'u1', NULL, 'Root', 0)`,
		`INSERT INTO videos (id, owner_id, directory_id, title, status, width, height, created_at, updated_at)
			VALUES ('v1', 'u1', 'd1', 'Tall', 'ready', 1080, 1920, 0, 0)`,
	} {
		if _, err := d.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}

	if err := Migrate(d); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	var w, h int
	var thumb *string
	if err := d.QueryRow(`SELECT width, height, thumbnail_url FROM videos WHERE id = 'v1'`).Scan(&w, &h, &thumb); err != nil {
		t.Fatal(err)
	}
	if w != 1080 || h != 1920 || thumb != nil {
		t.Fatalf("width=%d height=%d thumbnail=%v", w, h, thumb)
	}
}

func TestSplitStatements(t *testing.T) {
	got := splitStatements(`-- header; with a semicolon
ALTER TABLE t ADD COLUMN a TEXT; -- trailing; comment
INSERT INTO t VALUES ('x;y', 'it''s -- not a comment');

`)
	want := []string{"ALTER TABLE t ADD COLUMN a TEXT", "INSERT INTO t VALUES ('x;y', 'it''s -- not a comment')"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q", got)
	}
}
