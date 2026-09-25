package agent

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/faizalv/lemongrass/vault"
)

func TestServiceFlushHTTPTokens(t *testing.T) {
	var logins atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		logins.Add(1)
		json.NewEncoder(w).Encode(map[string]string{"access_token": "tok"})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	vaultClient := startTestVault(t)
	domain := vault.Domain{
		BaseURL:         srv.URL,
		LoginEndpoint:   "/login",
		TokenPath:       "access_token",
		TokenPlacement:  vault.TokenPlacement{Kind: vault.PlacementHeader, Name: "Authorization", Prefix: "Bearer "},
		FixedTTLSeconds: 3600,
		Users:           []vault.DomainUser{{Name: "alice", Fields: map[string]string{"u": "alice"}}},
	}
	if err := vaultClient.PutDomain(testRootSecret, "staging", domain); err != nil {
		t.Fatalf("PutDomain: %v", err)
	}
	c, err := vaultClient.CreateHTTPChannel(testRootSecret, "test-channel", "staging", fullHTTPScope(), 5*time.Minute)
	if err != nil {
		t.Fatalf("CreateHTTPChannel: %v", err)
	}
	svc := NewService(vaultClient)
	shortID, err := svc.RegisterChannel(c.ID)
	if err != nil {
		t.Fatalf("RegisterChannel: %v", err)
	}

	for i := 0; i < 2; i++ {
		if _, err := svc.RequestHTTP(shortID, "", "GET", "/api/x", nil); err != nil {
			t.Fatalf("RequestHTTP: %v", err)
		}
	}
	if logins.Load() != 1 {
		t.Fatalf("logins = %d, want 1 before the flush", logins.Load())
	}

	flushed, err := svc.FlushHTTPTokens(shortID, "")
	if err != nil || flushed != 1 {
		t.Fatalf("FlushHTTPTokens = %d, %v, want 1", flushed, err)
	}
	if _, err := svc.RequestHTTP(shortID, "", "GET", "/api/x", nil); err != nil {
		t.Fatalf("RequestHTTP after flush: %v", err)
	}
	if logins.Load() != 2 {
		t.Errorf("logins = %d, want 2 after the flush", logins.Load())
	}

	if _, err := svc.FlushHTTPTokens("BOGUS1", ""); !errors.Is(err, ErrNoSuchChannel) {
		t.Errorf("FlushHTTPTokens with an unregistered short id: got %v, want ErrNoSuchChannel", err)
	}
}
