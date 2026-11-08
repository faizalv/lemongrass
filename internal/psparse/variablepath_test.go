// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package psparse

import "testing"

func TestVariablePath(t *testing.T) {
	cases := []struct {
		path, unqualified, drive string
		global, variable, drv    bool
	}{
		{"x", "x", "", false, true, false},
		{"global:x", "x", "", true, true, false},
		{"GLOBAL:x", "x", "", true, true, false},
		{"env:PATH", "PATH", "env", false, false, true},
		{"variable:global:x", "global:x", "", false, true, false},
		{"glob:x", "x", "glob", false, false, true},
		{"globalx", "globalx", "", false, true, false},
		{":x", "x", "", false, true, false},
		{"C:\\file.txt", "\\file.txt", "C", false, false, true},
	}
	for _, c := range cases {
		v := NewVariablePath(c.path)
		if v.UnqualifiedPath() != c.unqualified || v.DriveName() != c.drive || v.IsGlobal() != c.global ||
			v.IsVariable() != c.variable || v.IsDriveQualified() != c.drv {
			t.Errorf("%q: unqualified %q drive %q global %v variable %v drive-qualified %v", c.path,
				v.UnqualifiedPath(), v.DriveName(), v.IsGlobal(), v.IsVariable(), v.IsDriveQualified())
		}
	}
}
