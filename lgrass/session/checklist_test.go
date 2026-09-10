package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestChecklistMatches(t *testing.T) {
	c := Checklist{Tool: "Write|Edit", PathContains: "payments/"}

	cases := []struct {
		tool, path string
		want       bool
	}{
		{"Write", "payments/charge.go", true},
		{"Edit", "payments/charge.go", true},
		{"Read", "payments/charge.go", false},  // tool not in the pattern
		{"Write", "billing/invoice.go", false}, // path doesn't contain the substring
	}
	for _, c2 := range cases {
		if got := c.Matches(c2.tool, c2.path); got != c2.want {
			t.Errorf("Matches(%q, %q) = %v, want %v", c2.tool, c2.path, got, c2.want)
		}
	}
}

func TestChecklistMatchesAnyToolOrPathWhenUnset(t *testing.T) {
	c := Checklist{}
	if !c.Matches("Bash", "anything") {
		t.Error("empty Checklist should match any tool and any path")
	}
}

func TestChecklistTTLDefaultsWhenUnset(t *testing.T) {
	c := Checklist{}
	if c.TTL() != defaultChecklistTTL {
		t.Errorf("TTL() = %v, want default %v", c.TTL(), defaultChecklistTTL)
	}
	c.TTLMinutes = 10
	if c.TTL() != 10*time.Minute {
		t.Errorf("TTL() = %v, want 10m", c.TTL())
	}
}

func TestLoadChecklistsMissingFileIsNotAnError(t *testing.T) {
	list, err := LoadChecklists(t.TempDir())
	if err != nil {
		t.Fatalf("LoadChecklists on a project with no .lgrass/checklists.json: %v", err)
	}
	if list != nil {
		t.Errorf("list = %v, want nil", list)
	}
}

func TestLoadChecklistsParsesConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".lgrass"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	const config = `[{"id": "tos", "tool": "Write|Edit", "content": "read the terms first", "ttl_minutes": 5}]`
	if err := os.WriteFile(filepath.Join(dir, ".lgrass", "checklists.json"), []byte(config), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	list, err := LoadChecklists(dir)
	if err != nil {
		t.Fatalf("LoadChecklists: %v", err)
	}
	if len(list) != 1 || list[0].ID != "tos" || list[0].Content != "read the terms first" {
		t.Fatalf("list = %+v, want one checklist %q", list, "tos")
	}
}
