// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package main

import (
	"strings"
	"testing"

	"github.com/faizalv/lemongrass/internal/workgroup"
)

func TestDeclinedMessageSeparatesReasonNoReasonAndWithdrawal(t *testing.T) {
	cases := []struct {
		answer workgroup.Answer
		want   string
	}{
		{workgroup.Answer{Reason: "use haiku"}, "Their reason: use haiku"},
		{workgroup.Answer{}, "declined without a reason"},
		{workgroup.Answer{Withdrawn: true}, "withdrawn before the human answered"},
	}
	for _, c := range cases {
		got := declinedMessage(c.answer)
		if !strings.HasPrefix(got, "lgrass workgroup: ") || !strings.Contains(got, c.want) {
			t.Errorf("declinedMessage(%+v) = %q, want %q", c.answer, got, c.want)
		}
	}
}

func TestJoinMessageNamesEveryNewThinker(t *testing.T) {
	one := joinMessage([]workgroup.SpawnMember{{TabID: "aaaaaaaa-1111-4111-8111-111111111111", Label: "docs", Vendor: "codex"}})
	if one != "aaaaaaaa docs (codex) joins the group as a new thinker, approved by the human." {
		t.Errorf("one = %q", one)
	}
	two := joinMessage([]workgroup.SpawnMember{{TabID: "aaaaaaaa-1111-4111-8111-111111111111", Label: "docs", Vendor: "codex"}, {TabID: "bbbbbbbb-2222-4222-8222-222222222222", Label: "perf", Vendor: "claude"}})
	if !strings.Contains(two, "aaaaaaaa docs (codex), bbbbbbbb perf (claude) join the group") || strings.Contains(two, "!>>") {
		t.Errorf("two = %q", two)
	}
}

func TestCollaboratorJoinMessageNamesTheTabWithoutAMention(t *testing.T) {
	got := collaboratorJoinMessage("4341002a", "codex")
	if !strings.Contains(got, "4341002a (codex)") || !strings.Contains(got, "collaborator") || strings.Contains(got, "!>>") {
		t.Errorf("message = %q", got)
	}
}
