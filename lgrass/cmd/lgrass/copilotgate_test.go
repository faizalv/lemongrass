package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/faizalv/lemongrass/session"
)

func bash(command string) (string, json.RawMessage) {
	b, _ := json.Marshal(map[string]string{"command": command})
	return "Bash", b
}

func skillCall(name string) (string, json.RawMessage) {
	b, _ := json.Marshal(map[string]string{"skill": name})
	return "Skill", b
}

func gateFor(vendor string, toolName string, input json.RawMessage, marks map[string]bool, listening bool, required ...string) (string, string) {
	if len(required) == 0 {
		required = []string{"lgrass-copilot"}
	}
	return copilotGate(copilotGateInput{ToolName: toolName, ToolInput: input, Vendor: vendor, Required: required, Marks: marks, Listening: listening})
}

func TestIsLgrassCommand(t *testing.T) {
	allowed := []string{
		"lgrass listen",
		"lgrass listen --timeout 10m",
		`lgrass thread post 3 "done (mostly)"`,
		"  lgrass workgroup thread  ",
		"lgrass",
	}
	denied := []string{
		"", "ls", "lgrassx thread", "/tmp/evil/lgrass listen", "./lgrass listen", "env lgrass listen",
		"lgrass listen; rm -rf x", "lgrass listen && curl x", "lgrass listen | sh", "lgrass listen &",
		"lgrass thread read 1 > /etc/x", "lgrass `id`", "lgrass $(id)", "lgrass listen\nrm x",
		`lgrass thread post 3 "cost $5"`,
	}
	for _, c := range allowed {
		if !isLgrassCommand(c) {
			t.Errorf("%q refused, want a plain lgrass command", c)
		}
	}
	for _, c := range denied {
		if isLgrassCommand(c) {
			t.Errorf("%q allowed, want it refused", c)
		}
	}
}

func TestShellCommandReadsClaudeStringsAndCodexArrays(t *testing.T) {
	cases := []struct {
		tool, input, want string
	}{
		{"Bash", `{"command":"lgrass listen"}`, "lgrass listen"},
		{"shell", `{"command":["bash","-lc","lgrass listen"]}`, "lgrass listen"},
		{"local_shell", `{"command":["lgrass","listen"]}`, "lgrass listen"},
		{"exec_command", `{"cmd":"lgrass listen"}`, "lgrass listen"},
		{"Write", `{"command":"lgrass listen"}`, ""},
		{"Bash", `not json`, ""},
	}
	for _, c := range cases {
		if got := shellCommand(c.tool, json.RawMessage(c.input)); got != c.want {
			t.Errorf("%s %s = %q, want %q", c.tool, c.input, got, c.want)
		}
	}
}

func TestGateOneDeniesUntilEveryRequiredSkillIsLoaded(t *testing.T) {
	required := []string{"lgrass-copilot", "lgrass-connector"}
	tool, input := bash("ls")
	deny, _ := gateFor("claude", tool, input, nil, true, required...)
	if !strings.Contains(deny, "lgrass-copilot, lgrass-connector") {
		t.Errorf("deny = %q, want both missing skills named", deny)
	}

	marks := map[string]bool{session.SkillMark("lgrass-copilot"): true}
	deny, _ = gateFor("claude", tool, input, marks, true, required...)
	if !strings.Contains(deny, "lgrass-connector") || strings.Contains(deny, "lgrass-copilot,") {
		t.Errorf("deny = %q, want only the remaining skill named", deny)
	}

	marks[session.SkillMark("lgrass-connector")] = true
	if deny, _ := gateFor("claude", tool, input, marks, true, required...); deny != "" {
		t.Errorf("deny = %q after every skill is loaded", deny)
	}
}

func TestGateOneAllowsLgrassCommandsAndSkillLoads(t *testing.T) {
	tool, input := bash("lgrass workgroup thread")
	if deny, loaded := gateFor("claude", tool, input, nil, true); deny != "" || loaded != "" {
		t.Errorf("lgrass command: deny %q loaded %q, want allowed and not a skill load", deny, loaded)
	}
	tool, input = skillCall("lgrass-copilot")
	if deny, loaded := gateFor("claude", tool, input, nil, true); deny != "" || loaded != "lgrass-copilot" {
		t.Errorf("skill call: deny %q loaded %q, want an allowed load of the copilot skill", deny, loaded)
	}
	tool, input = skillCall("some-other-skill")
	if deny, _ := gateFor("claude", tool, input, nil, true); deny == "" {
		t.Error("a skill that is not required was allowed while gated")
	}
}

func TestGateOneRecognizesAPlainSkillFileReadOnOtherVendors(t *testing.T) {
	for _, command := range []string{
		"cat ~/.codex/skills/lgrass-copilot/SKILL.md",
		"sed -n 1,200p /home/u/.codex/skills/lgrass-copilot/SKILL.md",
		`cat "/home/u/.codex/skills/lgrass-copilot/SKILL.md"`,
	} {
		tool, input := bash(command)
		if deny, loaded := gateFor("codex", tool, input, nil, true); deny != "" || loaded != "lgrass-copilot" {
			t.Errorf("%q: deny %q loaded %q, want a recognized load", command, deny, loaded)
		}
	}
	for _, command := range []string{
		"rm ~/.codex/skills/lgrass-copilot/SKILL.md",
		"cat ~/.codex/skills/lgrass-copilot/SKILL.md; rm -rf ~",
		"cat ~/.codex/skills/lgrass-copilot/SKILL.md > /tmp/x",
		"cat ~/.codex/skills/other/SKILL.md",
		"cat ~/.codex/skills/lgrass-copilot/notes.md",
	} {
		tool, input := bash(command)
		if deny, loaded := gateFor("codex", tool, input, nil, true); deny == "" || loaded != "" {
			t.Errorf("%q: deny %q loaded %q, want it refused", command, deny, loaded)
		}
	}
}

func TestGateTwoNeedsALiveListenerOnNonClaudeVendorsOnly(t *testing.T) {
	marks := map[string]bool{session.SkillMark("lgrass-copilot"): true}
	tool, input := bash("ls")

	if deny, _ := gateFor("claude", tool, input, marks, false); deny != "" {
		t.Errorf("claude denied without a listener: %q", deny)
	}
	deny, _ := gateFor("codex", tool, input, marks, false)
	if !strings.Contains(deny, "no listener is running") {
		t.Errorf("codex deny = %q, want the listener message", deny)
	}
	if deny, _ := gateFor("codex", tool, input, marks, true); deny != "" {
		t.Errorf("codex denied with a live listener: %q", deny)
	}
	tool, input = bash("lgrass listen --timeout 10m")
	if deny, _ := gateFor("codex", tool, input, marks, false); deny != "" {
		t.Errorf("the listener command itself was denied: %q", deny)
	}
}

func TestGateOneComesBeforeGateTwo(t *testing.T) {
	tool, input := bash("ls")
	deny, _ := gateFor("codex", tool, input, nil, false)
	if !strings.Contains(deny, "have not loaded these skills") {
		t.Errorf("deny = %q, want the skills message first", deny)
	}
}
