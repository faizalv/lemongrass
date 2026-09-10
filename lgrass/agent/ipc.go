package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/faizalv/lemongrass/vault"
)

const (
	opRegisterChannel = "register_channel"
	opQuery           = "query"
	opForget          = "forget"
)

const connDeadline = 10 * time.Second

type request struct {
	Op      string          `json:"op"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type response struct {
	OK      bool            `json:"ok"`
	Error   string          `json:"error,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type registerChannelPayload struct {
	RealID vault.ChannelID `json:"real_id"`
}

type shortIDPayload struct {
	ShortID string `json:"short_id"`
}

type queryPayload struct {
	ShortID   string `json:"short_id"`
	Table     string `json:"table"`
	Operation string `json:"operation"`
}

type queryResultPayload struct {
	Value []byte `json:"value"`
}

// Serve accepts connections on l and handles one request per connection, with queryLimiter gating repeated wrong short-id guesses at Query.
func Serve(svc *Service, l net.Listener, queryLimiter *vault.FailureLimiter) error {
	for {
		conn, err := l.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		go handleConn(svc, queryLimiter, conn)
	}
}

func handleConn(svc *Service, queryLimiter *vault.FailureLimiter, conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(connDeadline))

	var req request
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		json.NewEncoder(conn).Encode(errResponse(err))
		return
	}
	json.NewEncoder(conn).Encode(dispatch(svc, queryLimiter, req))
}

func dispatch(svc *Service, queryLimiter *vault.FailureLimiter, req request) response {
	switch req.Op {
	case opRegisterChannel:
		var p registerChannelPayload
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return errResponse(err)
		}
		shortID, err := svc.RegisterChannel(p.RealID)
		if err != nil {
			return errResponse(err)
		}
		return payloadResponse(shortIDPayload{ShortID: shortID})

	case opQuery:
		var p queryPayload
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return errResponse(err)
		}
		return queryOp(queryLimiter, func() (response, error) {
			value, err := svc.Query(p.ShortID, p.Table, p.Operation)
			return payloadResponse(queryResultPayload{Value: value}), err
		})

	case opForget:
		var p shortIDPayload
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return errResponse(err)
		}
		svc.Forget(p.ShortID)
		return response{OK: true}

	default:
		return errResponse(fmt.Errorf("agent: unknown op %q", req.Op))
	}
}

// queryOp checks the limiter before a short-id lookup and records the outcome after, so
// repeated wrong guesses eventually lock out rather than running unthrottled.
func queryOp(queryLimiter *vault.FailureLimiter, fn func() (response, error)) response {
	if err := queryLimiter.Check(); err != nil {
		return errResponse(err)
	}
	resp, err := fn()
	if err != nil {
		queryLimiter.RecordFailure()
		return errResponse(err)
	}
	queryLimiter.RecordSuccess()
	return resp
}

func errResponse(err error) response {
	return response{OK: false, Error: err.Error()}
}

func payloadResponse(v interface{}) response {
	b, err := json.Marshal(v)
	if err != nil {
		return errResponse(err)
	}
	return response{OK: true, Payload: b}
}

// Client talks to a running agent daemon over its unix socket.
type Client struct {
	SocketPath string
	Timeout    time.Duration
}

func (c *Client) timeout() time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return connDeadline
}

func (c *Client) call(op string, payload, out interface{}) error {
	conn, err := net.DialTimeout("unix", c.SocketPath, c.timeout())
	if err != nil {
		return fmt.Errorf("agent: dialing %s: %w", c.SocketPath, err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(c.timeout()))

	var payloadBytes json.RawMessage
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		payloadBytes = b
	}
	if err := json.NewEncoder(conn).Encode(request{Op: op, Payload: payloadBytes}); err != nil {
		return fmt.Errorf("agent: sending request: %w", err)
	}

	var resp response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return fmt.Errorf("agent: reading response: %w", err)
	}
	if !resp.OK {
		return errors.New(resp.Error)
	}
	if out != nil && resp.Payload != nil {
		return json.Unmarshal(resp.Payload, out)
	}
	return nil
}

func (c *Client) RegisterChannel(realID vault.ChannelID) (string, error) {
	var out shortIDPayload
	err := c.call(opRegisterChannel, registerChannelPayload{RealID: realID}, &out)
	return out.ShortID, err
}

func (c *Client) Query(shortID, table, operation string) ([]byte, error) {
	var out queryResultPayload
	err := c.call(opQuery, queryPayload{ShortID: shortID, Table: table, Operation: operation}, &out)
	return out.Value, err
}

func (c *Client) Forget(shortID string) error {
	return c.call(opForget, shortIDPayload{ShortID: shortID}, nil)
}
