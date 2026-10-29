// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

package restergate

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/faizalv/lemongrass/internal/vault"
)

// humanServer counts logins and answers /api/* with the bearer token it saw, so a test can tell a
// cached token from a fresh login.
func humanServer(t *testing.T, logins *atomic.Int32) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		n := logins.Add(1)
		json.NewEncoder(w).Encode(map[string]string{"access_token": "tok" + string(rune('0'+n))})
	})
	mux.HandleFunc("/api/echo", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"zeta":1,"alpha":"`+r.Method+" "+r.Header.Get("Authorization")+`"}`)
	})
	mux.HandleFunc("/api/big", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(strings.Repeat("a", maxLoginResponseBytes+10)))
	})
	mux.HandleFunc("/api/binary", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte{0xff, 0xfe, 0x00, 0x01})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func setUpHumanRester(t *testing.T, srv *httptest.Server) *testSvc {
	t.Helper()
	svc := newTestService(t)
	if err := svc.SetPassphrase("root-secret"); err != nil {
		t.Fatalf("SetPassphrase: %v", err)
	}
	domain := newEditableDomain(srv.URL, []vault.ResterUser{
		{Name: "Alice Admin", Fields: map[string]string{"u": "alice"}},
		{Name: "bob", Fields: map[string]string{"u": "bob"}},
	})
	if err := svc.PutRester("root-secret", "test-rester", domain.Rester()); err != nil {
		t.Fatalf("PutRester: %v", err)
	}
	return svc
}

func humanCall(user, method, path string) HumanCall {
	return HumanCall{Rester: "test-rester", Environment: vault.DefaultEnvironment, User: user, Method: method, Path: path}
}

func TestHumanRequestLogsInOnceAndKeepsRawBody(t *testing.T) {
	var logins atomic.Int32
	svc := setUpHumanRester(t, humanServer(t, &logins))

	// No channel exists and no scope applies, so any method goes through.
	for _, method := range []string{"GET", "DELETE"} {
		result, err := svc.HumanRequest("root-secret", humanCall("Alice Admin", method, "/api/echo"))
		if err != nil {
			t.Fatalf("HumanRequest %s: %v", method, err)
		}
		want := `{"zeta":1,"alpha":"` + method + ` Bearer tok1"}`
		if result.Status != 200 || result.Body != want {
			t.Errorf("%s: status %d body %q, want 200 %q", method, result.Status, result.Body, want)
		}
		if result.Size != len(want) || result.Truncated || result.Binary {
			t.Errorf("%s: size %d truncated %v binary %v, want %d false false", method, result.Size, result.Truncated, result.Binary, len(want))
		}
	}
	if n := logins.Load(); n != 1 {
		t.Errorf("logins = %d, want 1", n)
	}
}

func TestHumanTokensAreSeparateFromChannelsAndUsers(t *testing.T) {
	var logins atomic.Int32
	svc := setUpHumanRester(t, humanServer(t, &logins))
	c, err := svc.CreateHTTPChannel("root-secret", "ch", "test-rester", vault.DefaultEnvironment, vault.TypeAPI, vault.HTTPScope{Methods: []string{"GET"}}, nil, time.Hour)
	if err != nil {
		t.Fatalf("CreateHTTPChannel: %v", err)
	}

	if _, err := svc.HumanRequest("root-secret", humanCall("alice_admin", "GET", "/api/echo")); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RequestHTTP(c.ID, "alice_admin", "GET", "/api/echo", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.HumanRequest("root-secret", humanCall("bob", "GET", "/api/echo")); err != nil {
		t.Fatal(err)
	}
	if n := logins.Load(); n != 3 {
		t.Errorf("logins = %d, want 3: the human, the channel and bob each log in once", n)
	}
}

func TestFlushHumanTokensForcesLogin(t *testing.T) {
	var logins atomic.Int32
	svc := setUpHumanRester(t, humanServer(t, &logins))

	svc.HumanRequest("root-secret", humanCall("bob", "GET", "/api/echo"))
	svc.HumanRequest("root-secret", humanCall("Alice Admin", "GET", "/api/echo"))
	if n := svc.FlushHumanTokens("test-rester", vault.DefaultEnvironment, "Bob"); n != 1 {
		t.Errorf("flushed %d, want 1", n)
	}
	svc.HumanRequest("root-secret", humanCall("bob", "GET", "/api/echo"))
	svc.HumanRequest("root-secret", humanCall("Alice Admin", "GET", "/api/echo"))
	if n := logins.Load(); n != 3 {
		t.Errorf("logins = %d, want 3: only bob logs in again", n)
	}
	if n := svc.FlushHumanTokens("test-rester", vault.DefaultEnvironment, ""); n != 2 {
		t.Errorf("flush all: flushed %d, want 2", n)
	}
}

func TestUpdateResterDropsHumanTokens(t *testing.T) {
	var logins atomic.Int32
	srv := humanServer(t, &logins)
	svc := setUpHumanRester(t, srv)

	svc.HumanRequest("root-secret", humanCall("bob", "GET", "/api/echo"))
	edited := newEditableDomain(srv.URL, []vault.ResterUser{{Name: "bob", Fields: map[string]string{"u": "bob2"}}})
	if err := svc.UpdateRester("root-secret", "test-rester", edited.Rester()); err != nil {
		t.Fatalf("UpdateRester: %v", err)
	}
	svc.HumanRequest("root-secret", humanCall("bob", "GET", "/api/echo"))
	if n := logins.Load(); n != 2 {
		t.Errorf("logins = %d, want 2: the edit drops the cached token", n)
	}
}

func TestHumanRequestWrongPassphrase(t *testing.T) {
	var logins atomic.Int32
	svc := setUpHumanRester(t, humanServer(t, &logins))
	_, err := svc.HumanRequest("wrong", humanCall("bob", "GET", "/api/echo"))
	if !vault.IsAuthFailure(err) {
		t.Errorf("err = %v, want an auth failure", err)
	}
	if logins.Load() != 0 {
		t.Error("a wrong passphrase must not reach the login endpoint")
	}
}

func TestHumanRequestTruncatedAndBinaryBodies(t *testing.T) {
	var logins atomic.Int32
	svc := setUpHumanRester(t, humanServer(t, &logins))

	big, err := svc.HumanRequest("root-secret", humanCall("bob", "GET", "/api/big"))
	if err != nil {
		t.Fatal(err)
	}
	if !big.Truncated || big.Size != maxLoginResponseBytes || len(big.Body) != maxLoginResponseBytes {
		t.Errorf("big: truncated %v size %d body %d bytes, want true and %d", big.Truncated, big.Size, len(big.Body), maxLoginResponseBytes)
	}

	bin, err := svc.HumanRequest("root-secret", humanCall("bob", "GET", "/api/binary"))
	if err != nil {
		t.Fatal(err)
	}
	if !bin.Binary || bin.Body != "" || bin.Size != 4 {
		t.Errorf("binary: binary %v body %q size %d, want true, empty, 4", bin.Binary, bin.Body, bin.Size)
	}
}

func TestHumanDownloadStreamsBody(t *testing.T) {
	var logins atomic.Int32
	svc := setUpHumanRester(t, humanServer(t, &logins))

	dl, err := svc.HumanDownload("root-secret", humanCall("bob", "GET", "/api/big"))
	if err != nil {
		t.Fatal(err)
	}
	if dl.Body == nil || dl.Inline != nil {
		t.Fatalf("download = %+v, want an open body", dl)
	}
	defer dl.Body.Close()
	n, err := io.Copy(io.Discard, dl.Body)
	if err != nil || n != maxLoginResponseBytes+10 {
		t.Errorf("read %d bytes, err %v, want %d", n, err, maxLoginResponseBytes+10)
	}
	if dl.Name != "big" {
		t.Errorf("Name = %q, want big", dl.Name)
	}
}
