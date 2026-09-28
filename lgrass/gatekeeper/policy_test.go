package gatekeeper

import (
	"strings"
	"testing"

	"github.com/faizalv/lemongrass/guard"
)

func TestIPCPolicyRoundTrip(t *testing.T) {
	client := startTestServer(t)
	if err := client.SetPassphrase(testRootSecret); err != nil {
		t.Fatalf("SetPassphrase: %v", err)
	}
	if _, active, err := client.CurrentPolicy(); err != nil || active {
		t.Fatalf("CurrentPolicy before any policy = active %v, %v", active, err)
	}

	policy := guard.Policy{Rules: map[string]string{"rm-plain": "allow"}, Binaries: []string{"make"}, Paths: []string{"~/private"}}
	if err := client.PutPolicy(testRootSecret, policy); err != nil {
		t.Fatalf("PutPolicy: %v", err)
	}
	current, active, err := client.CurrentPolicy()
	if err != nil || !active {
		t.Fatalf("CurrentPolicy = active %v, %v", active, err)
	}
	if current.Rules["rm-plain"] != "allow" || current.Binaries[0] != "make" || current.Paths[0] != "~/private" {
		t.Errorf("CurrentPolicy = %+v", current)
	}
	stored, err := client.GetPolicy(testRootSecret)
	if err != nil || stored.Binaries[0] != "make" {
		t.Errorf("GetPolicy = %+v, %v", stored, err)
	}
	if err := client.ActivatePolicy(testRootSecret); err != nil {
		t.Errorf("ActivatePolicy: %v", err)
	}
	rules, err := client.PolicyCatalog()
	if err != nil || len(rules) == 0 {
		t.Errorf("PolicyCatalog = %d rules, %v", len(rules), err)
	}
}

func TestIPCPolicyRejectsWhatTheGuardForbids(t *testing.T) {
	client := startTestServer(t)
	if err := client.SetPassphrase(testRootSecret); err != nil {
		t.Fatal(err)
	}
	err := client.PutPolicy(testRootSecret, guard.Policy{Binaries: []string{"lgrass"}})
	if err == nil || !strings.Contains(err.Error(), "lgrass cannot be blocked") {
		t.Errorf("blocking lgrass = %v", err)
	}
	err = client.PutPolicy(testRootSecret, guard.Policy{Rules: map[string]string{"secret-path": "allow"}})
	if err == nil || !strings.Contains(err.Error(), "cannot be changed") {
		t.Errorf("changing a locked rule = %v", err)
	}
	if _, active, _ := client.CurrentPolicy(); active {
		t.Error("a rejected policy became active")
	}
}

func TestIPCPolicyWrongPassphrase(t *testing.T) {
	client := startTestServer(t)
	if err := client.SetPassphrase(testRootSecret); err != nil {
		t.Fatal(err)
	}
	if err := client.PutPolicy("not the passphrase", guard.Policy{}); err == nil {
		t.Error("PutPolicy accepted a wrong passphrase")
	}
	if _, err := client.GetPolicy("not the passphrase"); err == nil {
		t.Error("GetPolicy accepted a wrong passphrase")
	}
}
