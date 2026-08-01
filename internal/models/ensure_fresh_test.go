package models

import (
	"database/sql"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestEnsureFreshSchemaBump(t *testing.T) {
	dir := t.TempDir()
	db := openTestDB(t)
	lib := NewLibrary(db, dir, nil, slog.Default())

	// Empty library, outdated schema → Scan runs (0 models).
	did, n, reason, err := lib.EnsureFresh("")
	if err != nil {
		t.Fatal(err)
	}
	if !did || n != 0 || reason != "metadata schema upgraded" {
		t.Fatalf("did=%v n=%d reason=%q", did, n, reason)
	}

	// Same schema, still empty → fresh.
	did, _, _, err = lib.EnsureFresh(MetadataSchemaVersion)
	if err != nil {
		t.Fatal(err)
	}
	if did {
		t.Fatal("expected no scan when schema matches and dirs empty")
	}
}

func TestEnsureFreshDetectsNewFile(t *testing.T) {
	dir := t.TempDir()
	db := openTestDB(t)
	lib := NewLibrary(db, dir, nil, slog.Default())

	// Seed schema as current with empty library.
	did, _, _, err := lib.EnsureFresh(MetadataSchemaVersion)
	if err != nil || did {
		t.Fatalf("warmup: did=%v err=%v", did, err)
	}

	// Drop a minimal GGUF-like file name; Scan will skip unreadable content,
	// but discoverPrimaries should still see it and mark stale.
	path := filepath.Join(dir, "toy-model.gguf")
	if err := os.WriteFile(path, []byte("not-a-gguf"), 0o644); err != nil {
		t.Fatal(err)
	}
	stale, why, err := lib.staleReason()
	if err != nil {
		t.Fatal(err)
	}
	if !stale {
		t.Fatal("expected stale after new gguf appeared")
	}
	if why != "model set changed" && why != "new model file" {
		// empty DB + 1 primary → len mismatch → "model set changed"
		t.Logf("stale reason: %s", why)
	}
}

func TestEnsureFreshMtime(t *testing.T) {
	dir := t.TempDir()
	db := openTestDB(t)
	lib := NewLibrary(db, dir, nil, slog.Default())

	path := filepath.Join(dir, "m.gguf")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	st, _ := os.Stat(path)
	past := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339Nano)
	_, err := db.Exec(`INSERT INTO models(id,alias,primary_path,projector_path,size_bytes,
		quantization,architecture,parameters,context_length,metadata_json,created_at,updated_at)
		VALUES ('id1','m',?,'',?,?,?,?,?,?,?,?)`,
		path, st.Size(), "", "", 0, 0, "{}", past, past)
	if err != nil {
		t.Fatal(err)
	}

	// Touch file so mtime is after updated_at.
	future := time.Now().UTC().Add(time.Minute)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}

	stale, why, err := lib.staleReason()
	if err != nil {
		t.Fatal(err)
	}
	if !stale || why != "model file newer than library" {
		t.Fatalf("stale=%v why=%q", stale, why)
	}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := "file:ensurefresh-" + t.Name() + "?mode=memory&cache=shared"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`CREATE TABLE models (
		id TEXT PRIMARY KEY, alias TEXT, favorite INTEGER DEFAULT 0, notes TEXT DEFAULT '',
		primary_path TEXT, projector_path TEXT DEFAULT '', size_bytes INTEGER,
		quantization TEXT DEFAULT '', architecture TEXT DEFAULT '', parameters INTEGER DEFAULT 0,
		context_length INTEGER DEFAULT 0, metadata_json TEXT DEFAULT '{}',
		source_repo TEXT DEFAULT '', pinned_runtime TEXT DEFAULT '', pinned_backend TEXT DEFAULT '',
		last_loaded_at TEXT DEFAULT '', last_runtime TEXT DEFAULT '', last_result TEXT DEFAULT '',
		created_at TEXT, updated_at TEXT
	)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE model_directories (
		id TEXT PRIMARY KEY, path TEXT UNIQUE, managed INTEGER DEFAULT 0, created_at TEXT
	)`)
	if err != nil {
		t.Fatal(err)
	}
	return db
}
