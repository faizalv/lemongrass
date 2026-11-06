// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

//go:build !windows

package agentconf

import "strings"

func pathTextHas(text, path string) bool {
	return strings.Contains(text, path)
}
