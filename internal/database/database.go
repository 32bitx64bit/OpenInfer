// Package database opens the application SQLite database and applies
// versioned migrations from the embedded migrations filesystem.
package database

import (
	"database/sql"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

// DB wraps *sql.DB.
type DB struct {
	*sql.DB
}

// Open opens (creating if needed) the database at dir/openinfer.db and
// applies any pending migrations inside transactions.
func Open(dir string, migrationsFS fs.FS) (*DB, error) {
	path := filepath.Join(dir, "openinfer.db")
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	db.SetMaxOpenConns(4)
	if err := Migrate(db, migrationsFS); err != nil {
		db.Close()
		return nil, err
	}
	return &DB{db}, nil
}

// Migrate applies all pending *.sql migrations in filename order. Each
// migration runs in its own transaction and is recorded in schema_migrations.
func Migrate(db *sql.DB, migrationsFS fs.FS) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
	)`); err != nil {
		return fmt.Errorf("creating schema_migrations: %w", err)
	}

	entries, err := fs.ReadDir(migrationsFS, ".")
	if err != nil {
		return fmt.Errorf("reading migrations: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		ver, err := migrationVersion(name)
		if err != nil {
			return err
		}
		var applied int
		if err = db.QueryRow(`SELECT COUNT(1) FROM schema_migrations WHERE version = ?`, ver).Scan(&applied); err != nil {
			return err
		}
		if applied > 0 {
			continue
		}
		body, err := fs.ReadFile(migrationsFS, name)
		if err != nil {
			return err
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(body)); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %s: %w", name, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version) VALUES (?)`, ver); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func migrationVersion(name string) (int, error) {
	base := strings.TrimSuffix(filepath.Base(name), ".sql")
	num, _, _ := strings.Cut(base, "_")
	v, err := strconv.Atoi(num)
	if err != nil {
		return 0, fmt.Errorf("invalid migration filename %q", name)
	}
	return v, nil
}
