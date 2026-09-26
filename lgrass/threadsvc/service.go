package threadsvc

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"path/filepath"
	"sync"
	"time"

	"github.com/faizalv/lemongrass/config"
	"github.com/faizalv/lemongrass/session"
)

const (
	opWake = "wake"
	opWait = "wait"

	maxWaitSeconds = 30
	dialTimeout    = 300 * time.Millisecond
)

func SocketPath() string {
	return filepath.Join(config.Dir(), "threads.sock")
}

type request struct {
	Op      string `json:"op"`
	TabID   string `json:"tab_id,omitempty"`
	Seconds int    `json:"seconds,omitempty"`
}

type response struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	Woke  bool   `json:"woke,omitempty"`
}

// A wake carries no data, so the ledger stays the only source of what is owed and a forged wake cannot inject a nudge.
type Service struct {
	store   *session.Store
	deliver *Deliverer
	mu      sync.Mutex
	waiters map[string][]chan struct{}
	now     func() time.Time
}

func NewService(store *session.Store) *Service {
	return &Service{
		store:   store,
		deliver: NewDeliverer(store),
		waiters: map[string][]chan struct{}{},
		now:     time.Now,
	}
}

func (s *Service) Serve(l net.Listener) error {
	for {
		conn, err := l.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		go s.handleConn(conn)
	}
}

func (s *Service) handleConn(conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	var req request
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		json.NewEncoder(conn).Encode(response{Error: "threadsvc: " + err.Error()})
		return
	}
	switch req.Op {
	case opWake:
		s.Wake()
		json.NewEncoder(conn).Encode(response{OK: true})
	case opWait:
		seconds := req.Seconds
		if seconds < 1 || seconds > maxWaitSeconds {
			seconds = maxWaitSeconds
		}
		conn.SetDeadline(time.Now().Add(time.Duration(seconds+5) * time.Second))
		woke := s.wait(req.TabID, time.Duration(seconds)*time.Second)
		json.NewEncoder(conn).Encode(response{OK: true, Woke: woke})
	default:
		json.NewEncoder(conn).Encode(response{Error: "threadsvc: unknown op " + req.Op})
	}
}

// Delivers to every tab that has pending rows: a push for Claude tabs, a signal to blocked listeners for the rest.
func (s *Service) Wake() {
	tabs, err := s.store.PendingTabs()
	if err != nil {
		return
	}
	for _, tab := range tabs {
		vendor, err := s.store.VendorForTab(tab)
		if err != nil {
			continue
		}
		if vendor == vendorClaude {
			s.deliver.DeliverTab(tab)
			continue
		}
		s.signal(tab)
	}
}

func (s *Service) signal(tabID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ch := range s.waiters[tabID] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (s *Service) wait(tabID string, timeout time.Duration) bool {
	ch := make(chan struct{}, 1)
	s.mu.Lock()
	s.waiters[tabID] = append(s.waiters[tabID], ch)
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		list := s.waiters[tabID]
		for i, c := range list {
			if c == ch {
				s.waiters[tabID] = append(list[:i], list[i+1:]...)
				break
			}
		}
		if len(s.waiters[tabID]) == 0 {
			delete(s.waiters, tabID)
		}
	}()

	if pending, err := s.store.PendingForTab(tabID); err == nil && len(pending) > 0 {
		return true
	}
	select {
	case <-ch:
		return true
	case <-time.After(timeout):
		return false
	}
}

// Scans for pending rows at start, then retries due pushes and prunes the ledger until ctx ends.
func (s *Service) Run(ctx context.Context, retryEvery, pruneEvery time.Duration) {
	s.Wake()
	retry := time.NewTicker(retryEvery)
	prune := time.NewTicker(pruneEvery)
	defer retry.Stop()
	defer prune.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-retry.C:
			s.retryDue()
		case <-prune.C:
			s.store.PruneNotifications(s.now())
		}
	}
}

func (s *Service) retryDue() {
	tabs, err := s.store.TabsDueForRetry(MaxAttempts, Backoff, s.now())
	if err != nil {
		return
	}
	for _, tab := range tabs {
		s.deliver.DeliverTab(tab)
	}
}

// Best effort: an error means the service is not reachable.
func Wake(socketPath string) error {
	resp, err := call(socketPath, request{Op: opWake}, 2*time.Second)
	if err != nil {
		return err
	}
	if !resp.OK {
		return errors.New(resp.Error)
	}
	return nil
}

// Blocks until the tab has pending rows or the seconds elapse; an error means the service is not reachable.
func Wait(socketPath, tabID string, seconds int) (bool, error) {
	resp, err := call(socketPath, request{Op: opWait, TabID: tabID, Seconds: seconds}, time.Duration(seconds+5)*time.Second)
	if err != nil {
		return false, err
	}
	if !resp.OK {
		return false, errors.New(resp.Error)
	}
	return resp.Woke, nil
}

func call(socketPath string, req request, deadline time.Duration) (response, error) {
	conn, err := net.DialTimeout("unix", socketPath, dialTimeout)
	if err != nil {
		return response{}, err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(deadline))
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return response{}, err
	}
	var resp response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return response{}, err
	}
	return resp, nil
}
