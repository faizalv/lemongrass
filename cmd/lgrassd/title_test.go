// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package main

import (
	"bytes"
	"testing"
)

func TestAgyTitlePrintsConversationTitle(t *testing.T) {
	var out bytes.Buffer
	code := runAgyTitle([]byte(`{"conversation_title":"  Fix login\nflow  ","agent_state":"idle"}`), &out)
	if code != 0 {
		t.Fatalf("code %d, want 0", code)
	}
	if out.String() != "Fix login flow\n" {
		t.Fatalf("stdout %q, want %q", out.String(), "Fix login flow\n")
	}
}

func TestAgyTitleEmptyOrInvalidPrintsNothing(t *testing.T) {
	for _, payload := range []string{
		`{}`,
		`{"conversation_title":"   \n\t "}`,
		`{"conversation_title":null}`,
		`not json`,
		``,
	} {
		var out bytes.Buffer
		if code := runAgyTitle([]byte(payload), &out); code != 0 || out.Len() != 0 {
			t.Errorf("payload %q -> code %d stdout %q, want 0 and empty", payload, code, out.String())
		}
	}
}
