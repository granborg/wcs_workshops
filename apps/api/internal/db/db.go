// Package db opens the SQLite database and applies embedded SQL migrations.
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver, no C compiler needed
)

//go:embed migrations/*.sql
var migrations embed.FS

// Open opens (and creates when missing) the SQLite database at path.
// Pragmas are set through the DSN so they apply to every pooled connection.
func Open(path string) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}
	dsn := "file:" + filepath.ToSlash(path) +
		"?_pragma=foreign_keys(1)" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=busy_timeout(5000)" +
		"&_pragma=synchronous(NORMAL)"
	d, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	d.SetMaxOpenConns(4)
	d.SetConnMaxIdleTime(5 * time.Minute)
	if err := d.Ping(); err != nil {
		d.Close()
		return nil, err
	}
	return d, nil
}

// Migrate applies every migrations/*.sql file not yet recorded in schema_migrations, in name order.
//
// Each migration runs with foreign keys off, as the SQLite procedure for rebuilding a table requires
// (https://www.sqlite.org/lang_altertable.html#otheralter). Otherwise DROP TABLE on a rebuilt table would
// cascade deletes into the tables that reference it. PRAGMA foreign_key_check runs before commit, so a
// migration that breaks a reference is rolled back.
func Migrate(d *sql.DB) error {
	ctx := context.Background()
	// PRAGMA foreign_keys is per connection and has no effect inside a transaction: use one connection.
	conn, err := d.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
		return err
	}
	// The connection goes back to the pool afterwards, so turn foreign keys on again in every case.
	defer conn.ExecContext(ctx, `PRAGMA foreign_keys = ON`)
	return migrate(ctx, conn)
}

func migrate(ctx context.Context, d *sql.Conn) error {
	if _, err := d.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    TEXT PRIMARY KEY,
		applied_at INTEGER NOT NULL
	)`); err != nil {
		return err
	}

	entries, err := fs.ReadDir(migrations, "migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		var applied int
		if err := d.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, name).Scan(&applied); err != nil {
			return err
		}
		if applied > 0 {
			continue
		}
		body, err := migrations.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		tx, err := d.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if err := runMigration(ctx, tx, string(body)); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %s: %w", name, err)
		}
		if err := checkForeignKeys(ctx, tx); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`, name, time.Now().UnixMilli()); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// addColumnRE matches "ALTER TABLE <table> ADD [COLUMN] <column> ...".
var addColumnRE = regexp.MustCompile(`(?is)^ALTER\s+TABLE\s+["\x60]?(\w+)["\x60]?\s+ADD\s+(?:COLUMN\s+)?["\x60]?(\w+)`)

// runMigration runs the statements of one migration file in order. An ADD COLUMN for a column that already
// exists is skipped: another build of the app (another branch that was deployed) can have added the same
// column under a different migration name, and SQLite has no ADD COLUMN IF NOT EXISTS.
func runMigration(ctx context.Context, tx *sql.Tx, body string) error {
	for _, stmt := range splitStatements(body) {
		if m := addColumnRE.FindStringSubmatch(stmt); m != nil {
			var n int
			if err := tx.QueryRowContext(ctx,
				`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`, m[1], m[2]).Scan(&n); err != nil {
				return err
			}
			if n > 0 {
				continue
			}
		}
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

// splitStatements cuts SQL into statements at each semicolon outside quotes, and removes "--" comments.
// It does not handle trigger bodies (BEGIN ... END), which no migration uses.
func splitStatements(body string) []string {
	var out []string
	var cur strings.Builder
	flush := func() {
		if s := strings.TrimSpace(cur.String()); s != "" {
			out = append(out, s)
		}
		cur.Reset()
	}
	var quote rune // the open quote character, or 0
	runes := []rune(body)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case quote != 0:
			cur.WriteRune(r)
			if r == quote {
				quote = 0 // a doubled quote ('') closes and opens again, which keeps the text the same
			}
		case r == '\'' || r == '"' || r == '`':
			quote = r
			cur.WriteRune(r)
		case r == '-' && i+1 < len(runes) && runes[i+1] == '-':
			for i < len(runes) && runes[i] != '\n' {
				i++
			}
			cur.WriteRune('\n')
		case r == ';':
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return out
}

// checkForeignKeys returns an error when any row refers to a row that does not exist.
func checkForeignKeys(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		var table string
		var rowid sql.NullInt64
		var parent string
		var fkid int
		if err := rows.Scan(&table, &rowid, &parent, &fkid); err != nil {
			return err
		}
		return fmt.Errorf("foreign key violation: %s row %d refers to a missing %s row", table, rowid.Int64, parent)
	}
	return rows.Err()
}
