package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeChecklists(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".lgrass"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".lgrass", "checklists.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestResolveSignableBuiltins(t *testing.T) {
	for _, id := range []string{BibliothekChecklistID, MemoryWriteChecklistID} {
		got, ok, err := ResolveSignable(t.TempDir(), id)
		if err != nil || !ok {
			t.Fatalf("ResolveSignable(%q) = ok %v, err %v, want a registered word", id, ok, err)
		}
		if got.Pledge == "" {
			t.Errorf("ResolveSignable(%q) has no pledge", id)
		}
	}
}

func TestResolveSignableRejectsUnknownWords(t *testing.T) {
	dir := t.TempDir()
	for _, id := range []string{"--help", "memory-feedback-law", "", "I-WRITE-MEMORY"} {
		if _, ok, err := ResolveSignable(dir, id); ok || err != nil {
			t.Errorf("ResolveSignable(%q) = ok %v, err %v, want unregistered", id, ok, err)
		}
	}
}

func TestResolveSignableChecklistPledge(t *testing.T) {
	dir := writeChecklists(t, `[
		{"id": "with-pledge", "content": "the terms", "pledge": "I accept."},
		{"id": "content-only", "content": "the terms"}
	]`)
	got, ok, err := ResolveSignable(dir, "with-pledge")
	if err != nil || !ok || got.Pledge != "I accept." {
		t.Errorf("with-pledge = %+v, ok %v, err %v", got, ok, err)
	}
	got, ok, err = ResolveSignable(dir, "content-only")
	if err != nil || !ok || got.Pledge != "the terms" {
		t.Errorf("content-only = %+v, ok %v, err %v, want the content as pledge", got, ok, err)
	}
}

func TestResolveSignableInvalidChecklistsFile(t *testing.T) {
	dir := writeChecklists(t, `not json`)
	if _, ok, err := ResolveSignable(dir, "anything"); ok || err == nil {
		t.Errorf("ResolveSignable with an invalid checklists file = ok %v, err %v, want an error", ok, err)
	}
}

func TestResolveSignableUsesEveryRegisteredSource(t *testing.T) {
	saved := signSources
	defer func() { signSources = saved }()
	signSources = append(append([]SignSource{}, saved...), func(string) ([]Signable, error) {
		return []Signable{{ID: "user-set", Pledge: "mine"}}, nil
	})
	got, ok, err := ResolveSignable(t.TempDir(), "user-set")
	if err != nil || !ok || got.Pledge != "mine" {
		t.Errorf("added source = %+v, ok %v, err %v", got, ok, err)
	}
}

func TestDeniesStayShort(t *testing.T) {
	for name, msg := range map[string]string{
		"memory":     FormatMemoryFeedbackDeny(),
		"bibliothek": FormatBibliothekDeny(),
		"checklist":  FormatChecklistDeny(Checklist{ID: "review"}),
	} {
		if len(strings.Fields(msg)) > 60 {
			t.Errorf("%s deny has %d words, want at most 60", name, len(strings.Fields(msg)))
		}
	}
}
