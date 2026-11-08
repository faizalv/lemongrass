// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package agentusage

import (
	"bytes"
	"math"
	"os"
	"testing"
	"time"
)

func TestAgyStatuslineFixture(t *testing.T) {
	payload, err := os.ReadFile("testdata/agy-statusline.json")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(payload, []byte("email")) {
		t.Fatal("fixture contains account identity")
	}
	home := t.TempDir()
	now := time.Date(2026, 11, 8, 5, 0, 0, 0, time.UTC)
	if err := RecordAgy(home, payload, now); err != nil {
		t.Fatal(err)
	}
	if err := RecordContext(home, "agy-tab", payload, now); err != nil {
		t.Fatal(err)
	}
	rec, ok, err := ReadAgy(home)
	if err != nil || !ok {
		t.Fatalf("read: ok=%v err=%v", ok, err)
	}
	if rec.Gemini == nil || math.Abs(rec.Gemini.UsedPercent-8.36264) > 0.00001 || rec.ThirdParty == nil || rec.ThirdParty.UsedPercent != 0 {
		t.Fatalf("quota: %+v", rec)
	}
	if rec.Gemini.ResetsAt != time.Date(2026, 11, 15, 4, 17, 18, 0, time.UTC).Unix() {
		t.Fatal("incorrect reset")
	}
	contexts, err := ReadContexts(home)
	if err != nil || contexts["agy-tab"].UsedTokens != 7206 || contexts["agy-tab"].WindowTokens != 1048576 {
		t.Fatalf("contexts: %v %v", contexts, err)
	}
	before, _ := os.ReadFile(agyPath(home))
	if err := RecordAgy(home, payload, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(agyPath(home))
	if !bytes.Equal(before, after) {
		t.Fatal("unchanged quota rewritten")
	}
	report := Collect(home, t.TempDir(), nil, now)
	if report.Agy == nil || len(report.Agy.Windows) != 2 || report.Agy.Windows[0].Label != "Gemini" || report.Agy.Windows[1].Label != "Third-party" {
		t.Fatalf("report: %+v", report.Agy)
	}
	report = Collect(home, t.TempDir(), nil, now.Add(8*24*time.Hour))
	if !report.Agy.Windows[0].Reset || report.Agy.Windows[0].UsedPercent != 0 {
		t.Fatal("reset not applied")
	}
}

func TestAgyPartialAndChangedQuota(t *testing.T) {
	home := t.TempDir()
	now := time.Unix(1000, 0)
	first := []byte(`{"quota":{"gemini-weekly":{"remaining_fraction":0.8,"reset_time":"2030-01-01T00:00:00Z"},"3p-weekly":{"remaining_fraction":1,"reset_time":"2030-01-01T00:00:00Z"}}}`)
	if err := RecordAgy(home, first, now); err != nil {
		t.Fatal(err)
	}
	if err := RecordAgy(home, []byte(`{"quota":{"gemini-weekly":{"remaining_fraction":0.5,"reset_time":"2030-01-01T00:00:00Z"}}}`), now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	rec, _, _ := ReadAgy(home)
	if rec.Gemini.UsedPercent != 50 || rec.ThirdParty == nil || rec.SeenAt != 1001 {
		t.Fatalf("record: %+v", rec)
	}
}

func TestAgyEmptyAndMalformedPayload(t *testing.T) {
	home := t.TempDir()
	for _, p := range []string{`{}`, `{"quota":{}}`, `{"quota":{"gemini-weekly":{}}}`} {
		if err := RecordAgy(home, []byte(p), time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	if _, ok, err := ReadAgy(home); err != nil || ok {
		t.Fatalf("empty record: %v %v", ok, err)
	}
	if err := RecordAgy(home, []byte(`{`), time.Now()); err == nil {
		t.Fatal("malformed payload accepted")
	}
}
