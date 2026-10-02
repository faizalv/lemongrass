package main

import (
	"strings"
	"testing"

	"github.com/faizalv/lemongrass/workgroup"
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
