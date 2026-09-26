package workgroup

import (
	"encoding/json"
	"net"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func fakeApp(t *testing.T, answer func(Request) Response) (string, chan Request) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "app.sock")
	l, err := net.Listen("unix", path)
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	t.Cleanup(func() { l.Close() })
	seen := make(chan Request, 4)
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			var req Request
			json.NewDecoder(conn).Decode(&req)
			seen <- req
			json.NewEncoder(conn).Encode(answer(req))
			conn.Close()
		}
	}()
	return path, seen
}

func TestProposeReturnsATokenAndSpawnPresentsOnlyIt(t *testing.T) {
	path, seen := fakeApp(t, func(r Request) Response {
		return Response{OK: true, Approved: r.Op == opPropose, Token: "tok-1"}
	})
	req := Request{ProjectPath: "/p", PilotTabID: "pilot-tab", PilotLabel: "lead", GroupName: "g", Members: []SpawnMember{{TabID: "t1", Label: "a", Vendor: "claude", Prompt: "go"}}}

	token, approved, err := Propose(path, req)
	if err != nil || !approved || token != "tok-1" {
		t.Fatalf("Propose = %q, %v, %v, want an approval with a token", token, approved, err)
	}
	if got := <-seen; got.Op != "propose" || got.PilotTabID != "pilot-tab" || len(got.Members) != 1 || got.Members[0].Prompt != "go" {
		t.Errorf("propose request = %+v", got)
	}
	if err := Spawn(path, token); err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	if got := <-seen; got.Op != "spawn" || got.Token != "tok-1" || len(got.Members) != 0 || got.PilotTabID != "" {
		t.Errorf("spawn request = %+v, want only the op and token", got)
	}
}

func TestProposeDeclinedIsNotAnError(t *testing.T) {
	path, _ := fakeApp(t, func(Request) Response { return Response{OK: true, Approved: false} })
	token, approved, err := Propose(path, Request{})
	if err != nil || approved || token != "" {
		t.Errorf("Propose = %q, %v, %v, want a plain decline", token, approved, err)
	}
}

func TestAppErrorsAndAnUnreachableAppCarryThePackagePrefix(t *testing.T) {
	path, _ := fakeApp(t, func(Request) Response { return Response{Error: "pilot tab not found"} })
	err := Spawn(path, "tok")
	if err == nil || err.Error() != "workgroup: pilot tab not found" {
		t.Errorf("Spawn error = %v", err)
	}
	if _, _, err := Propose(filepath.Join(t.TempDir(), "none.sock"), Request{}); err == nil || !strings.HasPrefix(err.Error(), "workgroup: the lemongrass app is not reachable") {
		t.Errorf("unreachable error = %v", err)
	}
}

func TestNewTabIDIsAVersion4UUID(t *testing.T) {
	re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		id := NewTabID()
		if !re.MatchString(id) || seen[id] {
			t.Fatalf("tab id %q is malformed or repeated", id)
		}
		seen[id] = true
	}
}

func TestComposePromptNamesTheRoleThreadAndAssignment(t *testing.T) {
	claude := ComposePrompt("claude", "reviewer", "schema-review", "lead", []string{"lgrass-copilot", "lgrass-connector"}, "Review the diff.")
	for _, want := range []string{`copilot "reviewer"`, `workgroup "schema-review"`, `pilot "lead"`, "lgrass-copilot, lgrass-connector", "lgrass workgroup thread", "Review the diff."} {
		if !strings.Contains(claude, want) {
			t.Errorf("claude prompt missing %q", want)
		}
	}
	if strings.Contains(claude, "lgrass listen") {
		t.Error("a claude prompt asks for a listener")
	}
	if !strings.Contains(ComposePrompt("codex", "t", "g", "p", []string{"lgrass-copilot"}, "x"), "lgrass listen") {
		t.Error("a codex prompt does not ask for a listener")
	}
}
