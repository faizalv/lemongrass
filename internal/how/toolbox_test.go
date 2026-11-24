// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package how

import (
	"strings"
	"testing"
)

func TestToolboxManuals(t *testing.T) {
	root := biblioRoot(t)
	wantAll(t, render(t, root, "toolbox"), "add", "update", "remove", "list")
	for action, want := range map[string]string{
		"add":    "lgrass toolbox add <slug> --about",
		"update": "--drop <file>",
		"remove": "lgrass toolbox remove <slug>",
		"list":   "lgrass toolbox list",
	} {
		wantAll(t, render(t, root, "toolbox", action), want, "at most 40 items", "the hooks refuse any direct write")
	}
	if _, err := Render(t.TempDir(), []string{"toolbox", "add"}); err == nil || !strings.Contains(err.Error(), "lgrass biblio init") {
		t.Errorf("toolbox add without biblio/: err %v", err)
	}
}
