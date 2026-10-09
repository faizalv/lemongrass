package vault

import (
	"encoding/json"
	"testing"
)

func TestDomainUserTagsRoundTrip(t *testing.T) {
	svc := openTestService(t)
	if err := svc.SetPassphrase("root-secret"); err != nil {
		t.Fatal(err)
	}
	d := Domain{BaseURL: "http://x", Users: []DomainUser{
		{Name: "alice", Tags: []string{"tenant x", "low level"}},
		{Name: "bob"},
	}}
	if err := svc.PutDomain("root-secret", "d", d); err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetDomain("root-secret", "d")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Users[0].Tags) != 2 || got.Users[0].Tags[0] != "tenant x" || len(got.Users[1].Tags) != 0 {
		t.Errorf("users = %+v, want alice tagged and bob untagged", got.Users)
	}

	var legacy Domain
	if err := json.Unmarshal([]byte(`{"BaseURL":"http://x","Users":[{"Name":"carol","Fields":{"u":"c"},"Token":""}]}`), &legacy); err != nil {
		t.Fatalf("a domain stored before tags existed no longer decodes: %v", err)
	}
	if len(legacy.Users) != 1 || legacy.Users[0].Tags != nil {
		t.Errorf("legacy = %+v", legacy)
	}
}
