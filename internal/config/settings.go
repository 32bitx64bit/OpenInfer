package config

import (
	"database/sql"
	"encoding/json"
	"time"
)

// Settings is a typed key-value store over the settings table.
type Settings struct {
	db *sql.DB
}

func NewSettings(db *sql.DB) *Settings { return &Settings{db: db} }

func (s *Settings) Get(key, def string) string {
	var v string
	if err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&v); err != nil {
		return def
	}
	return v
}

func (s *Settings) Set(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO settings(key,value,updated_at) VALUES (?,?,?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`,
		key, value, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Settings) GetJSON(key string, dst any) error {
	v := s.Get(key, "")
	if v == "" {
		return nil
	}
	return json.Unmarshal([]byte(v), dst)
}

func (s *Settings) SetJSON(key string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.Set(key, string(b))
}

// All returns every setting (used by the Settings page).
func (s *Settings) All() (map[string]string, error) {
	rows, err := s.db.Query(`SELECT key, value FROM settings ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if rows.Scan(&k, &v) == nil {
			out[k] = v
		}
	}
	return out, nil
}
