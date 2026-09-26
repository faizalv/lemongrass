package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/faizalv/lemongrass/session"
)

const (
	gateTabPilot   = "aaaaaaaa-1111-4111-8111-111111111111"
	gateTabClaude  = "bbbbbbbb-2222-4222-8222-222222222222"
	gateTabCodex   = "cccccccc-3333-4333-8333-333333333333"
	gateTabOutside = "dddddddd-4444-4444-8444-444444444444"
)

func storeWithGroup(t *testing.T, claudeSkills []string) (*session.Store, session.Group) {
	t.Helper()
	store := openHookTestStore(t)
	group, err := store.CreateGroup("review", session.Member{TabID: gateTabPilot, Label: "lead", Vendor: "claude"}, []session.Member{
		{TabID: gateTabClaude, Label: "reviewer", Vendor: "claude", Skills: claudeSkills},
		{TabID: gateTabCodex, Label: "tester", Vendor: "codex"},
	})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	return store, group
}

func preTool(tab, tool string, input any) hookEvent {
	raw, _ := json.Marshal(input)
	return hookEvent{SessionID: "s-" + strings.TrimSpace(tab), ToolName: tool, ToolInput: raw, TabID: tab, EnforceBibliothek: true}
}

func projectWithBiblio(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "biblio"), 0o755)
	return dir
}

func TestOnlyCopilotsAreGated(t *testing.T) {
	store, _ := storeWithGroup(t, nil)
	dir := t.TempDir()

	for _, tab := range []string{gateTabPilot, gateTabOutside, ""} {
		result := hookPreToolUse(store, preTool(tab, "Bash", map[string]string{"command": "ls"}), dir)
		if result.PermissionDecision == "deny" {
			t.Errorf("tab %q was gated: %s", tab, result.PermissionDecisionReason)
		}
	}
	result := hookPreToolUse(store, preTool(gateTabClaude, "Bash", map[string]string{"command": "ls"}), dir)
	if result.PermissionDecision != "deny" || !strings.Contains(result.PermissionDecisionReason, "lgrass-copilot") {
		t.Errorf("copilot was not gated: %+v", result)
	}
}

func TestLoadingTheSkillLiftsGateOneAndSurvivesToLaterCalls(t *testing.T) {
	store, _ := storeWithGroup(t, nil)
	dir := t.TempDir()

	load := hookPreToolUse(store, preTool(gateTabClaude, "Skill", map[string]string{"skill": "lgrass-copilot"}), dir)
	if load.PermissionDecision == "deny" {
		t.Fatalf("the skill load was denied: %s", load.PermissionDecisionReason)
	}
	for _, command := range []string{"ls", "go test ./..."} {
		result := hookPreToolUse(store, preTool(gateTabClaude, "Bash", map[string]string{"command": command}), dir)
		if result.PermissionDecision == "deny" {
			t.Errorf("%q denied after the skill was loaded: %s", command, result.PermissionDecisionReason)
		}
	}
}

func TestACopilotWithARequiredSkillListMustLoadThemAll(t *testing.T) {
	store, _ := storeWithGroup(t, []string{"lgrass-connector"})
	dir := t.TempDir()

	hookPreToolUse(store, preTool(gateTabClaude, "Skill", map[string]string{"skill": "lgrass-copilot"}), dir)
	result := hookPreToolUse(store, preTool(gateTabClaude, "Bash", map[string]string{"command": "ls"}), dir)
	if result.PermissionDecision != "deny" || !strings.Contains(result.PermissionDecisionReason, "lgrass-connector") {
		t.Fatalf("the second required skill was not enforced: %+v", result)
	}
	hookPreToolUse(store, preTool(gateTabClaude, "Skill", map[string]string{"skill": "lgrass-connector"}), dir)
	if result := hookPreToolUse(store, preTool(gateTabClaude, "Bash", map[string]string{"command": "ls"}), dir); result.PermissionDecision == "deny" {
		t.Errorf("still denied after both skills: %s", result.PermissionDecisionReason)
	}
}

func TestCodexCopilotNeedsALiveListenerAfterItsSkills(t *testing.T) {
	store, _ := storeWithGroup(t, nil)
	dir := t.TempDir()
	read := map[string]any{"command": []string{"bash", "-lc", "cat ~/.codex/skills/lgrass-copilot/SKILL.md"}}

	if load := hookPreToolUse(store, preTool(gateTabCodex, "shell", read), dir); load.PermissionDecision == "deny" {
		t.Fatalf("the skill read was denied: %s", load.PermissionDecisionReason)
	}
	ls := map[string]any{"command": []string{"bash", "-lc", "ls"}}
	result := hookPreToolUse(store, preTool(gateTabCodex, "shell", ls), dir)
	if result.PermissionDecision != "deny" || !strings.Contains(result.PermissionDecisionReason, "no listener") {
		t.Fatalf("gate two did not hold: %+v", result)
	}
	store.Heartbeat(gateTabCodex)
	if result := hookPreToolUse(store, preTool(gateTabCodex, "shell", ls), dir); result.PermissionDecision == "deny" {
		t.Errorf("denied with a live listener: %s", result.PermissionDecisionReason)
	}
}

func TestDisbandingFreesTheCopilots(t *testing.T) {
	store, group := storeWithGroup(t, nil)
	store.DisbandGroup(group.ID)

	result := hookPreToolUse(store, preTool(gateTabCodex, "Bash", map[string]string{"command": "ls"}), t.TempDir())
	if result.PermissionDecision == "deny" {
		t.Errorf("a disbanded group's copilot is still gated: %s", result.PermissionDecisionReason)
	}
}

func TestBibliothekGateIsOptionalForACopilotUnlessRequired(t *testing.T) {
	store, _ := storeWithGroup(t, []string{"bibliothek"})
	dir := projectWithBiblio(t)
	store.MarkReady(gateTabClaude, session.SkillMark("lgrass-copilot"))
	store.MarkReady(gateTabClaude, session.SkillMark("bibliothek"))

	// The required copilot has not signed the bibliothek gate, so it is denied for that.
	result := hookPreToolUse(store, preTool(gateTabClaude, "Bash", map[string]string{"command": "ls"}), dir)
	if result.PermissionDecision != "deny" || !strings.Contains(result.PermissionDecisionReason, "bibliothek hasn't been invoked") {
		t.Errorf("a copilot that must use bibliothek was not held to the gate: %+v", result)
	}

	// The tester requires no bibliothek, so the same call passes the bibliothek gate.
	store.MarkReady(gateTabCodex, session.SkillMark("lgrass-copilot"))
	store.Heartbeat(gateTabCodex)
	event := preTool(gateTabCodex, "Bash", map[string]string{"command": "ls"})
	event.EnforceBibliothek = true
	if result := hookPreToolUse(store, event, dir); result.PermissionDecision == "deny" {
		t.Errorf("a copilot without the bibliothek requirement was held to its gate: %s", result.PermissionDecisionReason)
	}
	// A plain tab is still held to it.
	if result := hookPreToolUse(store, preTool(gateTabOutside, "Bash", map[string]string{"command": "ls"}), dir); result.PermissionDecision != "deny" {
		t.Error("a plain tab escaped the bibliothek gate")
	}
}

func TestFoundationalRulesStillApplyToCopilots(t *testing.T) {
	store, _ := storeWithGroup(t, nil)
	dir := projectWithBiblio(t)
	store.MarkReady(gateTabClaude, session.SkillMark("lgrass-copilot"))

	result := hookPreToolUse(store, preTool(gateTabClaude, "EnterPlanMode", map[string]string{}), dir)
	if result.PermissionDecision != "deny" || !strings.Contains(result.PermissionDecisionReason, "EnterPlanMode") {
		t.Errorf("a copilot was allowed into plan mode: %+v", result)
	}
}

func TestSessionStartSendsLawsAndTheStartLineAndResetsMarks(t *testing.T) {
	store, _ := storeWithGroup(t, []string{"lgrass-connector"})
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "biblio", "laws"), 0o755)
	os.WriteFile(filepath.Join(dir, "biblio", "laws", "summary.md"), []byte("# Laws summary\n- a law"), 0o600)
	store.MarkReady(gateTabClaude, session.SkillMark("lgrass-copilot"))

	result := hookSessionStart(store, hookEvent{SessionID: "s1", TabID: gateTabClaude}, dir)
	for _, want := range []string{"a law", `copilot "reviewer"`, "lgrass-copilot, lgrass-connector", "lgrass workgroup thread"} {
		if !strings.Contains(result.AdditionalContext, want) {
			t.Errorf("start context missing %q:\n%s", want, result.AdditionalContext)
		}
	}
	if strings.Contains(result.AdditionalContext, "lgrass listen") {
		t.Error("a Claude copilot was told to run a listener")
	}
	if marks, _ := store.Marks(gateTabClaude); len(marks) != 0 {
		t.Errorf("marks survived SessionStart: %v", marks)
	}

	codex := hookSessionStart(store, hookEvent{SessionID: "s2", TabID: gateTabCodex}, dir)
	if !strings.Contains(codex.AdditionalContext, "lgrass listen") {
		t.Error("a Codex copilot was not told to keep a listener running")
	}
	pilot := hookSessionStart(store, hookEvent{SessionID: "s3", TabID: gateTabPilot}, dir)
	if strings.Contains(pilot.AdditionalContext, "copilot") || !strings.Contains(pilot.AdditionalContext, "a law") {
		t.Errorf("the pilot's start context = %q, want laws only", pilot.AdditionalContext)
	}
}
