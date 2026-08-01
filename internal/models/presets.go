package models

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// Preset is a named load configuration for a model.
type Preset struct {
	ID        string          `json:"id"`
	ModelID   string          `json:"model_id"`
	Name      string          `json:"name"`
	IsDefault bool            `json:"is_default"`
	Settings  json.RawMessage `json:"settings"`
	UpdatedAt string          `json:"updated_at"`
}

// ListPresets returns all presets of a model (default first).
func (l *Library) ListPresets(modelID string) ([]Preset, error) {
	rows, err := l.db.Query(`SELECT id,model_id,name,is_default,settings,updated_at
		FROM model_presets WHERE model_id = ? ORDER BY is_default DESC, name`, modelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Preset
	for rows.Next() {
		var p Preset
		var def int
		var settings string
		if err := rows.Scan(&p.ID, &p.ModelID, &p.Name, &def, &settings, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.IsDefault = def == 1
		p.Settings = json.RawMessage(settings)
		out = append(out, p)
	}
	return out, nil
}

// SavePreset creates or updates a preset. is_default clears other defaults.
func (l *Library) SavePreset(modelID, presetID, name string, settings json.RawMessage, isDefault bool) (string, error) {
	if name == "" {
		return "", fmt.Errorf("preset name is required")
	}
	var v any
	if err := json.Unmarshal(settings, &v); err != nil {
		return "", fmt.Errorf("invalid settings JSON: %w", err)
	}
	tx, err := l.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if isDefault {
		if _, err := tx.Exec(`UPDATE model_presets SET is_default = 0 WHERE model_id = ?`, modelID); err != nil {
			return "", err
		}
	}
	def := 0
	if isDefault {
		def = 1
	}
	if presetID == "" {
		presetID = uuid.NewString()
		if _, err := tx.Exec(`INSERT INTO model_presets(id,model_id,name,is_default,settings,created_at,updated_at)
			VALUES (?,?,?,?,?,?,?)`, presetID, modelID, name, def, string(settings), now(), now()); err != nil {
			return "", err
		}
	} else {
		res, err := tx.Exec(`UPDATE model_presets SET name=?, is_default=?, settings=?, updated_at=? WHERE id=? AND model_id=?`,
			name, def, string(settings), now(), presetID, modelID)
		if err != nil {
			return "", err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return "", fmt.Errorf("preset %s not found", presetID)
		}
	}
	return presetID, tx.Commit()
}

// DeletePreset removes a preset.
func (l *Library) DeletePreset(modelID, presetID string) error {
	_, err := l.db.Exec(`DELETE FROM model_presets WHERE id = ? AND model_id = ?`, presetID, modelID)
	return err
}

// LastGoodPresetName is the reserved preset written when a load with
// save_on_success reaches ready. Users see it as "Last known good".
const LastGoodPresetName = "Last known good"

// SaveLastGood upserts the reserved last-known-good preset for a model.
func (l *Library) SaveLastGood(modelID string, settings json.RawMessage) error {
	_, err := l.db.Exec(`INSERT INTO model_presets(id,model_id,name,is_default,settings,created_at,updated_at)
		VALUES (?,?,?,0,?,?,?)
		ON CONFLICT(model_id,name) DO UPDATE SET settings=excluded.settings, updated_at=excluded.updated_at`,
		uuid.NewString(), modelID, LastGoodPresetName, string(settings), now(), now())
	return err
}

// LastGood returns the last-known-good settings for a model, if any.
func (l *Library) LastGood(modelID string) (json.RawMessage, bool) {
	var s string
	if err := l.db.QueryRow(`SELECT settings FROM model_presets WHERE model_id = ? AND name = ?`,
		modelID, LastGoodPresetName).Scan(&s); err != nil {
		return nil, false
	}
	return json.RawMessage(s), true
}
