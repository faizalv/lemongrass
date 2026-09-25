package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/faizalv/lemongrass/dbgate"
	"github.com/faizalv/lemongrass/gatekeeper"
	"github.com/faizalv/lemongrass/restergate"
	"github.com/faizalv/lemongrass/vault"
)

const (
	opRegisterChannel     = "register_channel"
	opQuery               = "query"
	opRequestHTTP         = "request_http"
	opRequestHTTPDownload = "request_http_download"
	opHTTPChannelInfo     = "http_channel_info"
	opHTTPUsers           = "http_users"
	opHTTPFlush           = "http_flush"
	opForget              = "forget"
)

var connDeadline = 10 * time.Second

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
	ShortID        string   `json:"short_id"`
	DeclaredTables []string `json:"declared_tables"`
	SQL            string   `json:"sql"`
}

type queryResultPayload struct {
	Result dbgate.QueryResult `json:"result"`
}

type requestHTTPPayload struct {
	ShortID     string `json:"short_id"`
	User        string `json:"user"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	Body        []byte `json:"body"`
	ContentType string `json:"content_type"`
}

type httpResultPayload struct {
	Result restergate.HTTPResult `json:"result"`
}

type httpChannelInfoPayload struct {
	Info restergate.HTTPChannelInfo `json:"info"`
}

type httpUsersPayload struct {
	Users []restergate.HTTPUserInfo `json:"users"`
}

type httpFlushPayload struct {
	ShortID string `json:"short_id"`
	User    string `json:"user"`
}

type httpFlushedPayload struct {
	Flushed int `json:"flushed"`
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
	if req.Op == opRequestHTTP || req.Op == opRequestHTTPDownload {
		conn.SetDeadline(time.Now().Add(restergate.RequestTimeout))
	}
	if req.Op == opRequestHTTPDownload {
		serveDownload(svc, conn, req)
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
			result, err := svc.Query(p.ShortID, p.DeclaredTables, p.SQL)
			return payloadResponse(queryResultPayload{Result: result}), err
		})

	case opRequestHTTP:
		var p requestHTTPPayload
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return errResponse(err)
		}
		return queryOp(queryLimiter, func() (response, error) {
			result, err := svc.RequestHTTP(p.ShortID, p.User, p.Method, p.Path, p.Body, p.ContentType)
			return payloadResponse(httpResultPayload{Result: result}), err
		})

	case opHTTPChannelInfo:
		var p shortIDPayload
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return errResponse(err)
		}
		return queryOp(queryLimiter, func() (response, error) {
			info, err := svc.HTTPChannelInfo(p.ShortID)
			return payloadResponse(httpChannelInfoPayload{Info: info}), err
		})

	case opHTTPUsers:
		var p shortIDPayload
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return errResponse(err)
		}
		return queryOp(queryLimiter, func() (response, error) {
			users, err := svc.HTTPUsers(p.ShortID)
			return payloadResponse(httpUsersPayload{Users: users}), err
		})

	case opHTTPFlush:
		var p httpFlushPayload
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return errResponse(err)
		}
		return queryOp(queryLimiter, func() (response, error) {
			flushed, err := svc.FlushHTTPTokens(p.ShortID, p.User)
			return payloadResponse(httpFlushedPayload{Flushed: flushed}), err
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

func serveDownload(svc *Service, conn net.Conn, req request) {
	var p requestHTTPPayload
	if err := json.Unmarshal(req.Payload, &p); err != nil {
		gatekeeper.WriteDownloadError(conn, err)
		return
	}
	res, err := svc.RequestHTTPDownload(p.ShortID, p.User, p.Method, p.Path, p.Body, p.ContentType)
	if err != nil {
		gatekeeper.WriteDownloadError(conn, err)
		return
	}
	realID, _ := svc.lookup(p.ShortID)
	gatekeeper.WriteDownload(conn, res, restergate.RequestTimeout, func(err error) error { return redactID(err, realID, p.ShortID) })
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
	return c.callWithin(c.timeout(), op, payload, out)
}

func (c *Client) callWithin(timeout time.Duration, op string, payload, out interface{}) error {
	conn, err := net.DialTimeout("unix", c.SocketPath, timeout)
	if err != nil {
		return fmt.Errorf("agent: dialing %s: %w", c.SocketPath, err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(timeout))

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

// RequestHTTPDownload asks the agent daemon for a download and returns its header, and for a 2xx response the open body, which the caller must close.
func (c *Client) RequestHTTPDownload(shortID, user, method, path string, body []byte, contentType string) (gatekeeper.DownloadResult, error) {
	timeout := max(c.Timeout, restergate.RequestTimeout)
	conn, err := net.DialTimeout("unix", c.SocketPath, timeout)
	if err != nil {
		return gatekeeper.DownloadResult{}, fmt.Errorf("agent: dialing %s: %w", c.SocketPath, err)
	}
	conn.SetDeadline(time.Now().Add(timeout))

	payload, err := json.Marshal(requestHTTPPayload{ShortID: shortID, User: user, Method: method, Path: path, Body: body, ContentType: contentType})
	if err == nil {
		err = json.NewEncoder(conn).Encode(request{Op: opRequestHTTPDownload, Payload: payload})
	}
	if err != nil {
		conn.Close()
		return gatekeeper.DownloadResult{}, fmt.Errorf("agent: sending request: %w", err)
	}

	res, err := gatekeeper.ReadDownload(conn, restergate.RequestTimeout)
	if err != nil || res.Body == nil {
		conn.Close()
	}
	var remote gatekeeper.RemoteError
	if err != nil && !errors.As(err, &remote) {
		err = fmt.Errorf("agent: %w", err)
	}
	return res, err
}

func (c *Client) RegisterChannel(realID vault.ChannelID) (string, error) {
	var out shortIDPayload
	err := c.call(opRegisterChannel, registerChannelPayload{RealID: realID}, &out)
	return out.ShortID, err
}

func (c *Client) Query(shortID string, declaredTables []string, sqlText string) (dbgate.QueryResult, error) {
	var out queryResultPayload
	err := c.call(opQuery, queryPayload{ShortID: shortID, DeclaredTables: declaredTables, SQL: sqlText}, &out)
	return out.Result, err
}

func (c *Client) RequestHTTP(shortID, user, method, path string, body []byte, contentType string) (restergate.HTTPResult, error) {
	var out httpResultPayload
	err := c.callWithin(max(c.Timeout, restergate.RequestTimeout), opRequestHTTP, requestHTTPPayload{ShortID: shortID, User: user, Method: method, Path: path, Body: body, ContentType: contentType}, &out)
	return out.Result, err
}

func (c *Client) HTTPChannelInfo(shortID string) (restergate.HTTPChannelInfo, error) {
	var out httpChannelInfoPayload
	err := c.call(opHTTPChannelInfo, shortIDPayload{ShortID: shortID}, &out)
	return out.Info, err
}

func (c *Client) HTTPUsers(shortID string) ([]restergate.HTTPUserInfo, error) {
	var out httpUsersPayload
	err := c.call(opHTTPUsers, shortIDPayload{ShortID: shortID}, &out)
	return out.Users, err
}

func (c *Client) FlushHTTPTokens(shortID, user string) (int, error) {
	var out httpFlushedPayload
	err := c.call(opHTTPFlush, httpFlushPayload{ShortID: shortID, User: user}, &out)
	return out.Flushed, err
}

func (c *Client) Forget(shortID string) error {
	return c.call(opForget, shortIDPayload{ShortID: shortID}, nil)
}
