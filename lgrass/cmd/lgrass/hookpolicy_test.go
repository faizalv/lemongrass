package main

import (
	"path/filepath"
	"testing"

	"github.com/faizalv/lemongrass/guard"
)

func TestHookPolicyReadsTheVaultsActivePolicy(t *testing.T) {
	client, _ := startStack(t)
	if got := hookPolicyFrom(client.SocketPath); len(got.Binaries) != 0 {
		t.Errorf("policy before the vault has one = %+v", got)
	}
	if err := client.SetPassphrase("pw"); err != nil {
		t.Fatal(err)
	}
	if err := client.PutPolicy("pw", guard.Policy{Binaries: []string{"make"}, Rules: map[string]string{"rm-plain": "allow"}}); err != nil {
		t.Fatal(err)
	}
	policy := hookPolicyFrom(client.SocketPath)
	if len(policy.Binaries) != 1 || policy.Binaries[0] != "make" || policy.Rules["rm-plain"] != "allow" {
		t.Fatalf("policy = %+v", policy)
	}
	verdict := guard.Decide(guard.Input{Tool: "Bash", Command: "make build", Cwd: t.TempDir(), Home: t.TempDir()}, policy)
	if verdict == nil || verdict.RuleID != "custom-binary" {
		t.Errorf("verdict = %+v, want custom-binary", verdict)
	}
}

func TestHookPolicyFallsBackToDefaultsWhenTheVaultIsUnreachable(t *testing.T) {
	got := hookPolicyFrom(filepath.Join(t.TempDir(), "missing.sock"))
	if len(got.Binaries)+len(got.Paths)+len(got.Rules) != 0 {
		t.Errorf("policy = %+v, want empty", got)
	}
}
