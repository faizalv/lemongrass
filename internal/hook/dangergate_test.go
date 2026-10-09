package hook

import (
	"encoding/json"
	"testing"

	"github.com/faizalv/lemongrass/internal/guard"
)

func bashPayload(t *testing.T, command string) hookEvent {
	t.Helper()
	input, err := json.Marshal(map[string]string{"command": command})
	if err != nil {
		t.Fatal(err)
	}
	return hookEvent{ToolName: "Bash", ToolInput: input, TabID: "tab", Cwd: t.TempDir()}
}

func TestDangerDecisionToolInputs(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cases := []struct {
		name  string
		tool  string
		input string
		want  string
	}{
		{"read normal file", "Read", `{"file_path":"/tmp/a.txt"}`, ""},
		{"read ssh key", "Read", `{"file_path":"~/.ssh/id_rsa"}`, "secret-path"},
		{"grep in aws dir", "Grep", `{"pattern":"key","path":"~/.aws"}`, "secret-path"},
		{"grep pattern only", "Grep", `{"pattern":".lemongrass","path":"src"}`, ""},
		{"glob into gnupg", "Glob", `{"pattern":"~/.gnupg/*"}`, "secret-path"},
		{"write settings", "Write", `{"file_path":"~/.claude/settings.json"}`, "guard-config"},
		{"edit project settings", "Edit", `{"file_path":".claude/settings.local.json"}`, "guard-config"},
		{"read settings", "Read", `{"file_path":"~/.claude/settings.json"}`, ""},
		{"patch into lgrass binary", "apply_patch", `{"command":"*** Update File: ~/.local/bin/lgrass\n"}`, "guard-config"},
		{"bash rm", "Bash", `{"command":"rm -rf build"}`, "rm-force"},
		{"bash ls", "Bash", `{"command":"ls"}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := hookEvent{ToolName: tc.tool, ToolInput: json.RawMessage(tc.input), TabID: "tab"}
			verdict := dangerDecision(payload)
			got := ""
			if verdict != nil {
				got = verdict.RuleID
			}
			if got != tc.want {
				t.Errorf("rule = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDangerGateOnlyForLemongrassTabs(t *testing.T) {
	store := openHookTestStore(t)
	payload := bashPayload(t, "rm -rf build")
	payload.TabID = ""
	if result := hookPreToolUse(store, payload, t.TempDir()); result.PermissionDecision == "deny" {
		t.Errorf("a session outside a lemongrass tab was denied: %s", result.PermissionDecisionReason)
	}
	payload.TabID = "tab"
	result := hookPreToolUse(store, payload, t.TempDir())
	if result.PermissionDecision != "deny" {
		t.Errorf("a lemongrass tab was not denied for rm -rf")
	}
}

func TestDangerVerdictPrefersDenyOverApproval(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	verdict := dangerDecision(bashPayload(t, "rm -rf build; git add ."))
	if verdict == nil || verdict.Mode != guard.ModeDeny {
		t.Errorf("verdict = %+v, want a deny", verdict)
	}
}
