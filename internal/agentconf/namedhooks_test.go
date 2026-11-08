// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package agentconf

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveNamedHooksPreservesForeignEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hooks.json")
	for _, tc := range []struct {
		data    string
		remains bool
	}{
		{`{"lemongrass":{},"personal":{"enabled":false}}`, true},
		{`{"lemongrass":{}}`, false},
	} {
		if err := os.WriteFile(path, []byte(tc.data), 0o600); err != nil {
			t.Fatal(err)
		}
		if changed, err := RemoveNamedHookConfig(path, "lemongrass"); err != nil || !changed {
			t.Fatalf("remove = %v, %v", changed, err)
		}
		data, err := os.ReadFile(path)
		if !tc.remains {
			if !os.IsNotExist(err) {
				t.Fatalf("owned-only file remains: %v", err)
			}
			continue
		}
		var top map[string]json.RawMessage
		if err != nil || json.Unmarshal(data, &top) != nil || len(top) != 1 || top["personal"] == nil {
			t.Fatalf("foreign config = %s, %v", data, err)
		}
		info, _ := os.Stat(path)
		if info.Mode().Perm() != 0o600 {
			t.Fatal("file mode changed")
		}
	}
	if changed, err := RemoveNamedHookConfig(path, "lemongrass"); err != nil || changed {
		t.Fatalf("missing file remove = %v, %v", changed, err)
	}
}
