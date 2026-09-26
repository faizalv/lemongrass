package session

import (
	"testing"
	"time"
)

func TestMarksRecordListAndClear(t *testing.T) {
	store := openTestStore(t)

	if marks, _ := store.Marks(tabA); len(marks) != 0 {
		t.Fatalf("fresh tab has marks: %v", marks)
	}
	store.MarkReady(tabA, SkillMark("lgrass-copilot"))
	store.MarkReady(tabA, SkillMark("lgrass-copilot"))
	store.MarkReady(tabA, MarkListening)
	store.MarkReady(tabB, SkillMark("lgrass-copilot"))

	marks, _ := store.Marks(tabA)
	if len(marks) != 2 || !marks["skill:lgrass-copilot"] || !marks["listen"] {
		t.Errorf("marks = %v, want the skill and listen marks once each", marks)
	}
	store.ClearMarks(tabA)
	if marks, _ := store.Marks(tabA); len(marks) != 0 {
		t.Errorf("marks after clear = %v", marks)
	}
	if marks, _ := store.Marks(tabB); len(marks) != 1 {
		t.Errorf("clearing one tab touched another: %v", marks)
	}
}

func TestListenerLiveFollowsTheHeartbeatWindow(t *testing.T) {
	store := openTestStore(t)

	if live, _ := store.ListenerLive(tabA, time.Now()); live {
		t.Error("a tab with no heartbeat reads as live")
	}
	store.Heartbeat(tabA)
	if live, _ := store.ListenerLive(tabA, time.Now()); !live {
		t.Error("a fresh heartbeat reads as not live")
	}
	if live, _ := store.ListenerLive(tabA, time.Now().Add(ListenerLiveWindow+time.Second)); live {
		t.Error("a heartbeat past the window still reads as live")
	}
}

func TestForgetTabClearsMarksAndRegisterPrunesOrphanedOnes(t *testing.T) {
	store := openTestStore(t)
	store.RegisterTab(tabA, "claude")
	store.MarkReady(tabA, MarkListening)
	store.ForgetTab(tabA)
	if marks, _ := store.Marks(tabA); len(marks) != 0 {
		t.Errorf("marks survived ForgetTab: %v", marks)
	}

	old := time.Now().Add(-tabRecordTTL - time.Hour).UTC().Format(time.RFC3339Nano)
	store.db.Exec(`INSERT INTO lg_ready_marks (project_id, tab_id, kind, marked_at) VALUES ('p', 'orphan-tab', 'listen', ?)`, old)
	store.RegisterTab(tabB, "codex")
	store.MarkReady(tabB, MarkListening)
	store.db.Exec(`UPDATE lg_ready_marks SET marked_at = ? WHERE tab_id = ?`, old, tabB)
	store.RegisterTab(tabC, "codex")

	var count int
	store.db.QueryRow(`SELECT COUNT(*) FROM lg_ready_marks`).Scan(&count)
	if count != 1 {
		t.Errorf("marks after prune = %d, want only the old mark of a tab that still has a record", count)
	}
}

func TestMemberKeepsItsAssignmentAndSkills(t *testing.T) {
	store := openTestStore(t)
	g, err := store.CreateGroup("g", Member{TabID: tabA, Label: "lead", Vendor: "claude"}, []Member{
		{TabID: tabB, Label: "reviewer", Vendor: "claude", Prompt: "Review the diff.", Skills: []string{"lgrass-connector", "bibliothek"}},
		{TabID: tabC, Label: "tester", Vendor: "codex", Prompt: "Write tests."},
	})
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}

	group, member, err := store.LiveMembership(tabB)
	if err != nil || group.ID != g.ID || member.Role != RoleCopilot || member.Prompt != "Review the diff." {
		t.Fatalf("LiveMembership = %+v %+v %v", group, member, err)
	}
	if len(member.Skills) != 2 || member.Skills[0] != "lgrass-connector" || member.Skills[1] != "bibliothek" {
		t.Errorf("skills = %v", member.Skills)
	}
	if _, m, _ := store.LiveMembership(tabC); len(m.Skills) != 0 || m.Prompt != "Write tests." {
		t.Errorf("tester member = %+v, want no skills", m)
	}
	if _, m, _ := store.LiveMembership(tabA); m.Role != RolePilot {
		t.Errorf("pilot role = %q", m.Role)
	}
	store.DisbandGroup(g.ID)
	if _, _, err := store.LiveMembership(tabB); err != ErrNoSuchGroup {
		t.Errorf("LiveMembership after disband = %v, want ErrNoSuchGroup", err)
	}
}

func TestMigrationAddsMemberColumnsToAnOlderTable(t *testing.T) {
	store := openTestStore(t)
	dbPath := DBPath()
	store.db.Exec(`DROP TABLE lg_group_members`)
	store.db.Exec(`CREATE TABLE lg_group_members (group_id INTEGER NOT NULL, tab_id TEXT NOT NULL, role TEXT NOT NULL, label TEXT NOT NULL, vendor TEXT NOT NULL, PRIMARY KEY (group_id, tab_id))`)
	store.db.Exec(`INSERT INTO lg_group_members VALUES (1, 'old-tab', 'copilot', 'x', 'claude')`)

	reopened, err := Open(dbPath, testProjectID)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer reopened.Close()
	members, err := reopened.GroupMembers(1)
	if err != nil || len(members) != 1 || members[0].Prompt != "" || len(members[0].Skills) != 0 {
		t.Errorf("members after migration = %+v, %v, want the old row with empty new columns", members, err)
	}
	if _, err := Open(dbPath, testProjectID); err != nil {
		t.Errorf("a second open after migrating failed: %v", err)
	}
}
