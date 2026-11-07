// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/faizalv/lemongrass/internal/session"
)

const (
	gateTabLeader  = "aaaaaaaa-1111-4111-8111-111111111111"
	gateTabClaude  = "bbbbbbbb-2222-4222-8222-222222222222"
	gateTabCodex   = "cccccccc-3333-4333-8333-333333333333"
	gateTabOutside = "dddddddd-4444-4444-8444-444444444444"
)

func storeWithGroup(t *testing.T, claudeSkills []string) (*session.Store, session.Group) {
	t.Helper()
	return storeWithClaudeThinker(t, session.Member{Skills: claudeSkills})
}

// The Claude thinker takes its skills and commands from claude; the rest of its row is fixed.
func storeWithClaudeThinker(t *testing.T, claude session.Member) (*session.Store, session.Group) {
	t.Helper()
	store := openHookTestStore(t)
	group, err := store.CreateGroup("review", session.Member{TabID: gateTabLeader, Label: "lead", Vendor: "claude"}, []session.Member{
		{TabID: gateTabClaude, Label: "reviewer", Vendor: "claude", Skills: claude.Skills, Commands: claude.Commands},
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

func TestOnlyThinkersAreGated(t *testing.T) {
	store, _ := storeWithGroup(t, nil)
	dir := t.TempDir()

	for _, tab := range []string{gateTabLeader, gateTabOutside, ""} {
		result := hookPreToolUse(store, preTool(tab, "Bash", map[string]string{"command": "ls"}), dir)
		if result.PermissionDecision == "deny" {
			t.Errorf("tab %q was gated: %s", tab, result.PermissionDecisionReason)
		}
	}
	result := hookPreToolUse(store, preTool(gateTabClaude, "Bash", map[string]string{"command": "ls"}), dir)
	if result.PermissionDecision != "deny" || !strings.Contains(result.PermissionDecisionReason, "lgrass how thinker start") {
		t.Errorf("thinker was not gated: %+v", result)
	}
}

func TestThinkerStartLiftsTheClaudeGateAndSurvivesToLaterCalls(t *testing.T) {
	store, _ := storeWithGroup(t, nil)
	dir := t.TempDir()

	load := hookPreToolUse(store, preTool(gateTabClaude, "Bash", map[string]string{"command": "lgrass how thinker start"}), dir)
	if load.PermissionDecision == "deny" {
		t.Fatalf("the thinker start read was denied: %s", load.PermissionDecisionReason)
	}
	for _, command := range []string{"ls", "go test ./..."} {
		result := hookPreToolUse(store, preTool(gateTabClaude, "Bash", map[string]string{"command": command}), dir)
		if result.PermissionDecision == "deny" {
			t.Errorf("%q denied after the role was read: %s", command, result.PermissionDecisionReason)
		}
	}
}

func TestTheRetiredThinkerSkillDoesNotOpenTheClaudeGate(t *testing.T) {
	store, _ := storeWithGroup(t, nil)
	dir := t.TempDir()
	hookPreToolUse(store, preTool(gateTabClaude, "Skill", map[string]string{"skill": "lgrass-howtobe-thinker"}), dir)
	if result := hookPreToolUse(store, preTool(gateTabClaude, "Bash", map[string]string{"command": "ls"}), dir); result.PermissionDecision != "deny" {
		t.Error("a Claude thinker passed the gate with the skill instead of lgrass how thinker start")
	}
}

func TestAThinkerWithARequiredSkillListMustLoadThemAll(t *testing.T) {
	store, _ := storeWithGroup(t, []string{"lgrass-connector"})
	dir := t.TempDir()

	hookPreToolUse(store, preTool(gateTabClaude, "Bash", map[string]string{"command": "lgrass how thinker start"}), dir)
	result := hookPreToolUse(store, preTool(gateTabClaude, "Bash", map[string]string{"command": "ls"}), dir)
	if result.PermissionDecision != "deny" || !strings.Contains(result.PermissionDecisionReason, "lgrass-connector") || strings.Contains(result.PermissionDecisionReason, "thinker start") {
		t.Fatalf("the second required skill was not enforced: %+v", result)
	}
	hookPreToolUse(store, preTool(gateTabClaude, "Skill", map[string]string{"skill": "lgrass-connector"}), dir)
	if result := hookPreToolUse(store, preTool(gateTabClaude, "Bash", map[string]string{"command": "ls"}), dir); result.PermissionDecision == "deny" {
		t.Errorf("still denied after both skills: %s", result.PermissionDecisionReason)
	}
}

// storeWithCodexThinker holds one Codex thinker, gateTabCodex, with the given row fields.
func storeWithCodexThinker(t *testing.T, codex session.Member) *session.Store {
	t.Helper()
	store := openHookTestStore(t)
	_, err := store.CreateGroup("review", session.Member{TabID: gateTabLeader, Label: "lead", Vendor: "claude"}, []session.Member{
		{TabID: gateTabCodex, Label: "tester", Vendor: "codex", Skills: codex.Skills, Commands: codex.Commands},
	})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	return store
}

// codexShell sends a shell call the way Codex 0.167 does: tool Bash, the script as a plain string.
func codexShell(store *session.Store, dir, script string) hookResult {
	return hookPreToolUse(store, preTool(gateTabCodex, "Bash", map[string]string{"command": script}), dir)
}

func TestCodexThinkerStartsWithHowThenLoadsItsListedSkills(t *testing.T) {
	store := storeWithCodexThinker(t, session.Member{Skills: []string{"schema-check", "bibliothek"}})
	dir := projectWithBiblio(t)

	result := codexShell(store, dir, "ls")
	if result.PermissionDecision != "deny" || !strings.Contains(result.PermissionDecisionReason, "run `lgrass how thinker start`, then load these skills: schema-check (") {
		t.Fatalf("deny = %+v, want the thinker start then the listed skill, bibliothek left out", result)
	}
	if strings.Contains(result.PermissionDecisionReason, session.ThinkerSkill) {
		t.Errorf("a Codex thinker was told to load the retired thinker skill: %s", result.PermissionDecisionReason)
	}
	codexShell(store, dir, "cat ~/.codex/skills/lgrass-howtobe-thinker/SKILL.md")
	codexShell(store, dir, "lgrass how thinker start")
	if result := codexShell(store, dir, "ls"); result.PermissionDecision != "deny" || strings.Contains(result.PermissionDecisionReason, "thinker start") {
		t.Fatalf("deny = %+v, want only the listed skill left", result)
	}
	if load := codexShell(store, dir, "cat ~/.codex/skills/schema-check/SKILL.md"); load.PermissionDecision == "deny" {
		t.Fatalf("the skill read was denied: %s", load.PermissionDecisionReason)
	}
	if result := codexShell(store, dir, "ls"); result.PermissionDecision == "deny" {
		t.Errorf("still denied after the start and the skill: %s", result.PermissionDecisionReason)
	}
}

func TestDisbandingFreesTheThinkers(t *testing.T) {
	store, group := storeWithGroup(t, nil)
	store.DisbandGroup(group.ID)

	result := hookPreToolUse(store, preTool(gateTabCodex, "Bash", map[string]string{"command": "ls"}), t.TempDir())
	if result.PermissionDecision == "deny" {
		t.Errorf("a disbanded group's thinker is still gated: %s", result.PermissionDecisionReason)
	}
}

func TestBibliothekIsNeverRequiredOfAClaudeThinker(t *testing.T) {
	store, _ := storeWithGroup(t, []string{"bibliothek"})
	dir := projectWithBiblio(t)
	store.MarkReady(gateTabClaude, session.ThinkerStartMark)
	if result := hookPreToolUse(store, preTool(gateTabClaude, "Bash", map[string]string{"command": "ls"}), dir); result.PermissionDecision == "deny" {
		t.Errorf("a Claude thinker was held to load bibliothek: %s", result.PermissionDecisionReason)
	}
}

func TestPlainTabNeedsNoBibliothekSkill(t *testing.T) {
	store, _ := storeWithGroup(t, nil)
	dir := projectWithBiblio(t)
	if result := hookPreToolUse(store, preTool(gateTabOutside, "Bash", map[string]string{"command": "ls"}), dir); result.PermissionDecision == "deny" {
		t.Errorf("a plain tab was denied: %s", result.PermissionDecisionReason)
	}
}

func TestFoundationalRulesStillApplyToThinkers(t *testing.T) {
	store, _ := storeWithGroup(t, nil)
	dir := projectWithBiblio(t)
	store.MarkReady(gateTabClaude, session.ThinkerStartMark)

	result := hookPreToolUse(store, preTool(gateTabClaude, "EnterPlanMode", map[string]string{}), dir)
	if result.PermissionDecision != "deny" || !strings.Contains(result.PermissionDecisionReason, "EnterPlanMode") {
		t.Errorf("a thinker was allowed into plan mode: %+v", result)
	}
}

func TestSessionStartSendsLawsAndTheStartLineAndResetsMarks(t *testing.T) {
	t.Setenv(lawsInPromptEnv, "")
	store, _ := storeWithGroup(t, []string{"lgrass-connector"})
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "biblio", "laws"), 0o755)
	os.WriteFile(filepath.Join(dir, "biblio", "laws", "summary.md"), []byte("# Laws summary\n- a law"), 0o600)
	store.MarkReady(gateTabClaude, session.ThinkerStartMark)

	result := hookSessionStart(store, hookEvent{SessionID: "s1", TabID: gateTabClaude}, dir)
	for _, want := range []string{"a law", `thinker "reviewer"`, "run `lgrass how thinker start`, then load these skills: lgrass-connector", "lgrass workgroup thread"} {
		if !strings.Contains(result.AdditionalContext, want) {
			t.Errorf("start context missing %q:\n%s", want, result.AdditionalContext)
		}
	}
	if strings.Contains(result.AdditionalContext, "lgrass listen") {
		t.Error("a Claude thinker was told to run a listener")
	}
	if marks, _ := store.Marks(gateTabClaude); len(marks) != 0 {
		t.Errorf("marks survived SessionStart: %v", marks)
	}

	codex := hookSessionStart(store, hookEvent{SessionID: "s2", TabID: gateTabCodex}, dir)
	if strings.Contains(codex.AdditionalContext, "lgrass listen") {
		t.Error("a Codex thinker was told to keep a listener running")
	}
	for _, want := range []string{`thinker "tester"`, "run `lgrass how thinker start`", "lgrass workgroup thread"} {
		if !strings.Contains(codex.AdditionalContext, want) {
			t.Errorf("Codex start context missing %q", want)
		}
	}
	if strings.Contains(codex.AdditionalContext, session.ThinkerSkill) {
		t.Error("a Codex thinker was told to load the retired thinker skill")
	}
	leader := hookSessionStart(store, hookEvent{SessionID: "s3", TabID: gateTabLeader}, dir)
	if strings.Contains(leader.AdditionalContext, "thinker") || !strings.Contains(leader.AdditionalContext, "a law") {
		t.Errorf("the leader's start context = %q, want laws only", leader.AdditionalContext)
	}
}

func TestAThinkerMustRunEveryListedCommand(t *testing.T) {
	store, _ := storeWithClaudeThinker(t, session.Member{Commands: []string{"lgrass how work task export-cli"}})
	dir := t.TempDir()
	run := func(command string) hookResult {
		return hookPreToolUse(store, preTool(gateTabClaude, "Bash", map[string]string{"command": command}), dir)
	}

	result := run("ls")
	if result.PermissionDecision != "deny" || !strings.Contains(result.PermissionDecisionReason, "run `lgrass how thinker start`, then run `lgrass how work task export-cli`") {
		t.Fatalf("deny = %+v, want both commands named", result)
	}
	run("lgrass how thinker start")
	if result := run("ls"); result.PermissionDecision != "deny" || strings.Contains(result.PermissionDecisionReason, "thinker start") {
		t.Fatalf("deny = %+v, want only the listed command left", result)
	}
	run("lgrass how work task export-cli > /dev/null")
	if result := run("ls"); result.PermissionDecision != "deny" {
		t.Fatal("a redirected run of the command opened the gate")
	}
	run(`lgrass  how work task "export-cli"`)
	if result := run("ls"); result.PermissionDecision == "deny" {
		t.Errorf("still denied after every command ran: %s", result.PermissionDecisionReason)
	}
}

func TestCodexThinkerRunsTheThinkerStartAndItsListedCommands(t *testing.T) {
	store := storeWithCodexThinker(t, session.Member{Commands: []string{"lgrass how book write"}})
	dir := t.TempDir()
	if result := codexShell(store, dir, "ls"); result.PermissionDecision != "deny" || !strings.Contains(result.PermissionDecisionReason, "run `lgrass how thinker start`, then run `lgrass how book write`") {
		t.Fatalf("deny = %+v, want both commands named", result)
	}
	codexShell(store, dir, "lgrass how thinker start")
	codexShell(store, dir, "lgrass how book write")
	if result := codexShell(store, dir, "ls"); result.PermissionDecision == "deny" {
		t.Errorf("still denied: %s", result.PermissionDecisionReason)
	}
}
