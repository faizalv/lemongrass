package vault

import (
	"strings"
	"testing"
)

func TestUserHandle(t *testing.T) {
	cases := map[string]string{
		"John Doe":       "john_doe",
		"  john   DOE ":  "john_doe",
		"john_doe":       "john_doe",
		"Alice":          "alice",
		"tab\tseparated": "tab_separated",
		"":               "",
	}
	for in, want := range cases {
		if got := UserHandle(in); got != want {
			t.Errorf("UserHandle(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPutDomainRejectsUsersWithTheSameHandle(t *testing.T) {
	svc, err := NewService(t.TempDir())
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	d := Domain{BaseURL: "https://api.example.test", Users: []DomainUser{{Name: "John Doe"}, {Name: "john_doe"}}}

	err = svc.PutDomain("root-secret", "staging", d)
	if err == nil || !strings.Contains(err.Error(), "John Doe") || !strings.Contains(err.Error(), "john_doe") {
		t.Fatalf("PutDomain err = %v, want a collision naming both users", err)
	}
	if names, _ := svc.ListDomains(); len(names) != 0 {
		t.Errorf("ListDomains() = %v, want nothing stored after a refused save", names)
	}

	d.Users[1].Name = "John DOE"
	if err := svc.PutDomain("root-secret", "staging", d); err == nil {
		t.Error("PutDomain accepted two users that differ only in case")
	}

	d.Users[1].Name = "jane doe"
	if err := svc.PutDomain("root-secret", "staging", d); err != nil {
		t.Errorf("PutDomain with distinct handles: %v", err)
	}
}

func TestUpdateDomainRejectsUsersWithTheSameHandle(t *testing.T) {
	svc, err := NewService(t.TempDir())
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	d := Domain{BaseURL: "https://api.example.test", Users: []DomainUser{{Name: "John Doe"}}}
	if err := svc.PutDomain("root-secret", "staging", d); err != nil {
		t.Fatalf("PutDomain: %v", err)
	}

	d.Users = append(d.Users, DomainUser{Name: "john_doe"})
	if err := svc.UpdateDomain("root-secret", "staging", d); err == nil {
		t.Fatal("UpdateDomain accepted a colliding user")
	}
	got, err := svc.getDomain("root-secret", "staging")
	if err != nil || len(got.Users) != 1 {
		t.Errorf("stored domain = %+v, %v, want the original single user kept", got, err)
	}
}
