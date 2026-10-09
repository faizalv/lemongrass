package vault

import (
	"bytes"
	"errors"
	"testing"
)

func newPolicyService(t *testing.T) *Service {
	t.Helper()
	svc, err := NewService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SetPassphrase("correct horse"); err != nil {
		t.Fatal(err)
	}
	return svc
}

func TestPolicyRoundTripAndActivation(t *testing.T) {
	svc := newPolicyService(t)
	if _, ok := svc.ActivePolicy(); ok {
		t.Fatal("a policy was active before any was stored")
	}
	policy := []byte(`{"binaries":["make"]}`)
	if err := svc.PutPolicy("correct horse", policy); err != nil {
		t.Fatalf("PutPolicy: %v", err)
	}
	active, ok := svc.ActivePolicy()
	if !ok || !bytes.Equal(active, policy) {
		t.Errorf("active policy = %q, %v", active, ok)
	}
	got, err := svc.GetPolicy("correct horse")
	if err != nil || !bytes.Equal(got, policy) {
		t.Errorf("GetPolicy = %q, %v", got, err)
	}

	reopened, err := NewService(svc.policies.dir + "/..")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reopened.ActivePolicy(); ok {
		t.Error("a restarted vault served a policy before unlock")
	}
	if err := reopened.ActivatePolicy("correct horse"); err != nil {
		t.Fatalf("ActivatePolicy: %v", err)
	}
	if active, ok := reopened.ActivePolicy(); !ok || !bytes.Equal(active, policy) {
		t.Errorf("reactivated policy = %q, %v", active, ok)
	}
}

func TestPolicyWrongPassphrase(t *testing.T) {
	svc := newPolicyService(t)
	if err := svc.PutPolicy("wrong", []byte(`{}`)); !errors.Is(err, ErrWrongPassphrase) {
		t.Errorf("PutPolicy with a wrong passphrase = %v", err)
	}
	if err := svc.PutPolicy("correct horse", []byte(`{"paths":["~/x"]}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetPolicy("wrong"); !errors.Is(err, ErrWrongPassphrase) {
		t.Errorf("GetPolicy with a wrong passphrase = %v", err)
	}
	if err := svc.ActivatePolicy("wrong"); err == nil {
		t.Error("ActivatePolicy accepted a wrong passphrase")
	}
}

func TestPolicyNeedsPassphraseAndEmptyActivates(t *testing.T) {
	svc, err := NewService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.PutPolicy("x", []byte(`{}`)); !errors.Is(err, ErrNoPassphrase) {
		t.Errorf("PutPolicy with no passphrase set = %v", err)
	}
	if err := svc.SetPassphrase("pw"); err != nil {
		t.Fatal(err)
	}
	if got, err := svc.GetPolicy("pw"); err != nil || got != nil {
		t.Errorf("GetPolicy with nothing stored = %q, %v", got, err)
	}
	if err := svc.ActivatePolicy("pw"); err != nil {
		t.Fatal(err)
	}
	if active, ok := svc.ActivePolicy(); !ok || len(active) != 0 {
		t.Errorf("empty activation = %q, %v", active, ok)
	}
}

func TestResetVaultDropsPolicy(t *testing.T) {
	svc := newPolicyService(t)
	if err := svc.PutPolicy("correct horse", []byte(`{"binaries":["make"]}`)); err != nil {
		t.Fatal(err)
	}
	if err := svc.ResetVault(); err != nil {
		t.Fatal(err)
	}
	if _, ok := svc.ActivePolicy(); ok {
		t.Error("policy still active after reset")
	}
	names, _ := svc.policies.List()
	if len(names) != 0 {
		t.Errorf("stored policy survived reset: %v", names)
	}
}
