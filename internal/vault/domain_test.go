package vault

import "testing"

func TestPutListDeleteDomain(t *testing.T) {
	dir := t.TempDir()
	svc, err := NewService(dir)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	d := Domain{BaseURL: "https://api.example.test", LoginEndpoint: "/login", TokenPath: "access_token"}
	if err := svc.PutDomain("root-secret", "staging", d); err != nil {
		t.Fatalf("PutDomain: %v", err)
	}
	if err := svc.PutCredential("root-secret", "some-db", []byte("postgres://x")); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}

	domains, err := svc.ListDomains()
	if err != nil {
		t.Fatalf("ListDomains: %v", err)
	}
	if len(domains) != 1 || domains[0] != "staging" {
		t.Errorf("ListDomains() = %v, want [\"staging\"]", domains)
	}

	conns, err := svc.ListConnections()
	if err != nil {
		t.Fatalf("ListConnections: %v", err)
	}
	if len(conns) != 1 || conns[0] != "some-db" {
		t.Errorf("ListConnections() = %v, want [\"some-db\"] (domains excluded)", conns)
	}

	got, err := svc.getDomain("root-secret", "staging")
	if err != nil {
		t.Fatalf("getDomain: %v", err)
	}
	if got.BaseURL != d.BaseURL || got.LoginEndpoint != d.LoginEndpoint {
		t.Errorf("getDomain() = %+v, want %+v", got, d)
	}

	if err := svc.DeleteDomain("staging"); err != nil {
		t.Fatalf("DeleteDomain: %v", err)
	}
	domains, err = svc.ListDomains()
	if err != nil {
		t.Fatalf("ListDomains after delete: %v", err)
	}
	if len(domains) != 0 {
		t.Errorf("ListDomains() after delete = %v, want empty", domains)
	}
}
