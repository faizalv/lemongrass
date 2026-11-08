// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package agentusage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/faizalv/lemongrass/internal/agentconf"
)

type AgyRecord struct {
	Gemini     *Window `json:"gemini,omitempty"`
	ThirdParty *Window `json:"third_party,omitempty"`
	SeenAt     int64   `json:"seen_at"`
}

type agyBucket struct {
	Remaining *float64  `json:"remaining_fraction"`
	ResetTime time.Time `json:"reset_time"`
}

func (b *agyBucket) window() *Window {
	if b == nil || b.Remaining == nil {
		return nil
	}
	return &Window{UsedPercent: (1 - *b.Remaining) * 100, ResetsAt: b.ResetTime.Unix(), WindowMinutes: 10080}
}

func agyPath(home string) string { return filepath.Join(Dir(home), "agy.json") }

// RecordAgy saves only the quota figures. Other payload fields, including account identity, are never persisted.
func RecordAgy(home string, payload []byte, now time.Time) error {
	var p struct {
		Quota struct {
			Gemini     *agyBucket `json:"gemini-weekly"`
			ThirdParty *agyBucket `json:"3p-weekly"`
		} `json:"quota"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("agentusage: parsing Antigravity statusline payload: %w", err)
	}
	rec := AgyRecord{Gemini: p.Quota.Gemini.window(), ThirdParty: p.Quota.ThirdParty.window(), SeenAt: now.Unix()}
	if rec.Gemini == nil && rec.ThirdParty == nil {
		return nil
	}
	if old, ok, err := ReadAgy(home); err == nil && ok {
		rec.Gemini = keepUntilReset(rec.Gemini, old.Gemini, now)
		rec.ThirdParty = keepUntilReset(rec.ThirdParty, old.ThirdParty, now)
		if sameWindow(rec.Gemini, old.Gemini) && sameWindow(rec.ThirdParty, old.ThirdParty) {
			return nil
		}
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return agentconf.WriteAtomic(agyPath(home), append(data, '\n'), 0o600)
}

func ReadAgy(home string) (AgyRecord, bool, error) {
	var rec AgyRecord
	data, err := os.ReadFile(agyPath(home))
	if errors.Is(err, fs.ErrNotExist) {
		return rec, false, nil
	}
	if err != nil {
		return rec, false, err
	}
	if err := json.Unmarshal(data, &rec); err != nil {
		return rec, false, fmt.Errorf("agentusage: parsing Antigravity quota record: %w", err)
	}
	return rec, true, nil
}
