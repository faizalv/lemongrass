package vault

import (
	"testing"
	"time"
)

func TestListChannelsNewestFirst(t *testing.T) {
	svc := openTestService(t)
	if err := svc.PutCredential(testRootSecret, "app-backend", []byte(unreachableConnString)); err != nil {
		t.Fatalf("PutCredential: %v", err)
	}

	var created []ChannelID
	for i := 0; i < 12; i++ {
		c, err := svc.CreateChannel(testRootSecret, "ch", "app-backend", fullScope(), 5*time.Minute)
		if err != nil {
			t.Fatalf("CreateChannel %d: %v", i, err)
		}
		created = append(created, c.ID)
		time.Sleep(2 * time.Millisecond)
	}

	got, err := svc.ListChannels()
	if err != nil {
		t.Fatalf("ListChannels: %v", err)
	}
	if len(got) != len(created) {
		t.Fatalf("len(ListChannels()) = %d, want %d", len(got), len(created))
	}
	for i, c := range got {
		want := created[len(created)-1-i]
		if c.ID != want {
			t.Fatalf("ListChannels()[%d] = %s, want %s (newest first)", i, c.ID, want)
		}
	}
}

func TestListHTTPChannelsNewestFirst(t *testing.T) {
	svc := openTestService(t)
	d := Domain{BaseURL: "https://api.example.test", LoginEndpoint: "/login", TokenPath: "access_token"}
	if err := svc.PutDomain(testRootSecret, "staging", d); err != nil {
		t.Fatalf("PutDomain: %v", err)
	}

	var created []ChannelID
	for i := 0; i < 12; i++ {
		c, err := svc.CreateHTTPChannel(testRootSecret, "ch", "staging", HTTPScope{Methods: []string{"GET"}}, 5*time.Minute)
		if err != nil {
			t.Fatalf("CreateHTTPChannel %d: %v", i, err)
		}
		created = append(created, c.ID)
		time.Sleep(2 * time.Millisecond)
	}

	got, err := svc.ListHTTPChannels()
	if err != nil {
		t.Fatalf("ListHTTPChannels: %v", err)
	}
	if len(got) != len(created) {
		t.Fatalf("len(ListHTTPChannels()) = %d, want %d", len(got), len(created))
	}
	for i, c := range got {
		want := created[len(created)-1-i]
		if c.ID != want {
			t.Fatalf("ListHTTPChannels()[%d] = %s, want %s (newest first)", i, c.ID, want)
		}
	}
}
