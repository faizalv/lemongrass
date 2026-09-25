package vault

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newSlowServer(delay time.Duration) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"access_token": "tok"})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})
	return httptest.NewServer(mux)
}

func setTimeouts(t *testing.T, login, request time.Duration) {
	t.Helper()
	oldLogin, oldRequest := httpClient.Timeout, RequestTimeout
	httpClient.Timeout, RequestTimeout = login, request
	t.Cleanup(func() { httpClient.Timeout, RequestTimeout = oldLogin, oldRequest })
}

func TestRequestHTTPUpstreamIsNotBoundByTheLoginTimeout(t *testing.T) {
	srv := newSlowServer(400 * time.Millisecond)
	defer srv.Close()
	svc, id := setUpHTTPChannel(t, srv, []DomainUser{{Name: "alice", Fields: map[string]string{"u": "alice"}}}, []string{"POST"})
	setTimeouts(t, 100*time.Millisecond, 5*time.Second)

	result, err := svc.RequestHTTP(id, "alice", "POST", "/import", []byte("x"), "text/csv")
	if err != nil || result.Status != 200 {
		t.Errorf("RequestHTTP = %+v, %v, want a slow upstream to complete under RequestTimeout", result, err)
	}
}

func TestRequestHTTPGivesUpAfterRequestTimeout(t *testing.T) {
	srv := newSlowServer(time.Second)
	defer srv.Close()
	svc, id := setUpHTTPChannel(t, srv, []DomainUser{{Name: "alice", Fields: map[string]string{"u": "alice"}}}, []string{"POST"})
	setTimeouts(t, 5*time.Second, 200*time.Millisecond)

	_, err := svc.RequestHTTP(id, "alice", "POST", "/import", []byte("x"), "text/csv")
	if err == nil || !strings.Contains(err.Error(), "request failed") {
		t.Errorf("err = %v, want the request to fail once RequestTimeout passes", err)
	}
}

func serveTestService(t *testing.T, svc *Service) *Client {
	t.Helper()
	sockPath := filepath.Join(t.TempDir(), "vault.sock")
	l, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	go Serve(svc, l, NewFailureLimiter(1000, time.Minute, time.Hour))
	t.Cleanup(func() { l.Close() })
	return &Client{SocketPath: sockPath}
}

func TestIPCRequestHTTPOutlivesConnDeadline(t *testing.T) {
	srv := newSlowServer(800 * time.Millisecond)
	defer srv.Close()
	svc, id := setUpHTTPChannel(t, srv, []DomainUser{{Name: "alice", Fields: map[string]string{"u": "alice"}}}, []string{"POST"})

	oldDeadline := connDeadline
	connDeadline = 300 * time.Millisecond
	t.Cleanup(func() { connDeadline = oldDeadline })
	setTimeouts(t, 5*time.Second, 5*time.Second)
	client := serveTestService(t, svc)

	result, err := client.RequestHTTP(id, "alice", "POST", "/import", []byte("x"), "text/csv")
	if err != nil || result.Status != 200 {
		t.Errorf("RequestHTTP = %+v, %v, want a call slower than connDeadline to complete", result, err)
	}
}
