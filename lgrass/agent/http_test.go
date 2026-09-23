package agent

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/faizalv/lemongrass/vault"
)

func fullHTTPScope() vault.HTTPScope {
	return vault.HTTPScope{Methods: []string{"GET"}}
}

func TestServiceRegisterThenRequestHTTP(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"access_token": "tok"})
	})
	mux.HandleFunc("/api/orders", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
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
	if len(shortID) != shortIDLength {
		t.Errorf("RegisterChannel returned id of length %d, want %d", len(shortID), shortIDLength)
	}

	result, err := svc.RequestHTTP(shortID, "alice", "GET", "/api/orders", nil)
	if err != nil {
		t.Fatalf("RequestHTTP: %v", err)
	}
	if result.Status != 200 {
		t.Errorf("Status = %d, want 200", result.Status)
	}
}

func TestServiceRequestHTTPUnknownShortIDReturnsErrNoSuchChannel(t *testing.T) {
	svc := NewService(startTestVault(t))
	if _, err := svc.RequestHTTP("BOGUS1", "alice", "GET", "/api/orders", nil); !errors.Is(err, ErrNoSuchChannel) {
		t.Errorf("RequestHTTP with an unregistered short id: got %v, want ErrNoSuchChannel", err)
	}
}
