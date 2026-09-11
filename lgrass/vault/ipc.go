package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"
)

const (
	opPutCredential       = "put_credential"
	opCreateChannel       = "create_channel"
	opActivate            = "activate"
	opQuery               = "query"
	opRevoke              = "revoke"
	opChannelScope        = "channel_scope"
	opListChannels        = "list_channels"
	opListConnections     = "list_connections"
	opDeleteConnection    = "delete_connection"
	opTestConnection      = "test_connection"
	opTestConnectionSaved = "test_connection_saved"
	opHasPassphrase       = "has_passphrase"
	opSetPassphrase       = "set_passphrase"
	opVerifyPassphrase    = "verify_passphrase"
	opResetVault          = "reset_vault"
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

type putCredentialPayload struct {
	RootSecret string `json:"root_secret"`
	DBName     string `json:"db_name"`
	Value      []byte `json:"value"`
}

type createChannelPayload struct {
	RootSecret string `json:"root_secret"`
	DBName     string `json:"db_name"`
	Scope      Scope  `json:"scope"`
	TTLSeconds int    `json:"ttl_seconds"`
}

type activatePayload struct {
	RootSecret string    `json:"root_secret"`
	ID         ChannelID `json:"id"`
	TTLSeconds int       `json:"ttl_seconds"`
}

type queryPayload struct {
	ID             ChannelID `json:"id"`
	DeclaredTables []string  `json:"declared_tables"`
	SQL            string    `json:"sql"`
}

type queryResultPayload struct {
	Result QueryResult `json:"result"`
}

type channelIDPayload struct {
	ID ChannelID `json:"id"`
}

type channelPayload struct {
	Channel Channel `json:"channel"`
}

type channelListPayload struct {
	Channels []Channel `json:"channels"`
}

type connectionListPayload struct {
	Names []string `json:"names"`
}

type dbNamePayload struct {
	DBName string `json:"db_name"`
}

type testConnectionPayload struct {
	ConnectionString string `json:"connection_string"`
}

type testConnectionSavedPayload struct {
	RootSecret string `json:"root_secret"`
	DBName     string `json:"db_name"`
}

type rootSecretPayload struct {
	RootSecret string `json:"root_secret"`
}

type hasPassphrasePayload struct {
	HasPassphrase bool `json:"has_passphrase"`
}

// Serve accepts connections on l and handles one request per connection.
// adminLimiter gates PutCredential, CreateChannel, and Activate -- the
// operations a request carries a root secret for.
func Serve(svc *Service, l net.Listener, adminLimiter *FailureLimiter) error {
	for {
		conn, err := l.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		go handleConn(svc, adminLimiter, conn)
	}
}

func handleConn(svc *Service, adminLimiter *FailureLimiter, conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(connDeadline))

	if err := verifyPeerUID(conn); err != nil {
		json.NewEncoder(conn).Encode(errResponse(err))
		return
	}

	var req request
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		json.NewEncoder(conn).Encode(errResponse(err))
		return
	}
	if requiresPeerBinaryCheck(req.Op) {
		if err := verifyPeerBinary(conn); err != nil {
			json.NewEncoder(conn).Encode(errResponse(err))
			return
		}
	}
	json.NewEncoder(conn).Encode(dispatch(svc, adminLimiter, req))
}

// requiresPeerBinaryCheck reports whether op is one the agent process (the lgrass binary)
// issues, as opposed to Electron's admin ops -- Electron's own binary path isn't fixed yet,
// so those ops stay at UID-only verification.
func requiresPeerBinaryCheck(op string) bool {
	return op == opQuery || op == opChannelScope
}

func dispatch(svc *Service, adminLimiter *FailureLimiter, req request) response {
	switch req.Op {
	case opPutCredential:
		var p putCredentialPayload
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return errResponse(err)
		}
		return adminOp(adminLimiter, func() (response, error) {
			return response{OK: true}, svc.PutCredential(p.RootSecret, p.DBName, p.Value)
		})

	case opCreateChannel:
		var p createChannelPayload
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return errResponse(err)
		}
		return adminOp(adminLimiter, func() (response, error) {
			c, err := svc.CreateChannel(p.RootSecret, p.DBName, p.Scope, time.Duration(p.TTLSeconds)*time.Second)
			return payloadResponse(channelPayload{Channel: c}), err
		})

	case opActivate:
		var p activatePayload
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return errResponse(err)
		}
		return adminOp(adminLimiter, func() (response, error) {
			c, err := svc.Activate(p.RootSecret, p.ID, time.Duration(p.TTLSeconds)*time.Second)
			return payloadResponse(channelPayload{Channel: c}), err
		})

	case opQuery:
		var p queryPayload
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return errResponse(err)
		}
		result, err := svc.Query(p.ID, p.DeclaredTables, p.SQL)
		if err != nil {
			return errResponse(err)
		}
		return payloadResponse(queryResultPayload{Result: result})

	case opRevoke:
		var p channelIDPayload
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return errResponse(err)
		}
		if err := svc.Revoke(p.ID); err != nil {
			return errResponse(err)
		}
		return response{OK: true}

	case opChannelScope:
		var p channelIDPayload
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return errResponse(err)
		}
		c, err := svc.ChannelScope(p.ID)
		if err != nil {
			return errResponse(err)
		}
		return payloadResponse(channelPayload{Channel: c})

	case opListChannels:
		channels, err := svc.ListChannels()
		if err != nil {
			return errResponse(err)
		}
		return payloadResponse(channelListPayload{Channels: channels})

	case opListConnections:
		names, err := svc.ListConnections()
		if err != nil {
			return errResponse(err)
		}
		return payloadResponse(connectionListPayload{Names: names})

	case opDeleteConnection:
		var p dbNamePayload
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return errResponse(err)
		}
		if err := svc.DeleteConnection(p.DBName); err != nil {
			return errResponse(err)
		}
		return response{OK: true}

	case opTestConnection:
		var p testConnectionPayload
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return errResponse(err)
		}
		if err := svc.TestConnection(p.ConnectionString); err != nil {
			return errResponse(err)
		}
		return response{OK: true}

	case opTestConnectionSaved:
		var p testConnectionSavedPayload
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return errResponse(err)
		}
		return adminOp(adminLimiter, func() (response, error) {
			return response{OK: true}, svc.TestConnectionSaved(p.RootSecret, p.DBName)
		})

	case opHasPassphrase:
		has, err := svc.HasPassphrase()
		if err != nil {
			return errResponse(err)
		}
		return payloadResponse(hasPassphrasePayload{HasPassphrase: has})

	case opSetPassphrase:
		var p rootSecretPayload
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return errResponse(err)
		}
		return adminOp(adminLimiter, func() (response, error) {
			return response{OK: true}, svc.SetPassphrase(p.RootSecret)
		})

	case opVerifyPassphrase:
		var p rootSecretPayload
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return errResponse(err)
		}
		return adminOp(adminLimiter, func() (response, error) {
			return response{OK: true}, svc.VerifyPassphrase(p.RootSecret)
		})

	case opResetVault:
		if err := svc.ResetVault(); err != nil {
			return errResponse(err)
		}
		return response{OK: true}

	default:
		return errResponse(fmt.Errorf("vault: unknown op %q", req.Op))
	}
}

// adminOp checks the limiter before a root-secret-bearing call and records the outcome after, so repeated wrong guesses eventually lock out rather than running unthrottled.
func adminOp(adminLimiter *FailureLimiter, fn func() (response, error)) response {
	if err := adminLimiter.Check(); err != nil {
		return errResponse(err)
	}
	resp, err := fn()
	if err != nil {
		adminLimiter.RecordFailure()
		return errResponse(err)
	}
	adminLimiter.RecordSuccess()
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

// Client talks to a running vault daemon over its unix socket.
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
		return fmt.Errorf("vault: dialing %s: %w", c.SocketPath, err)
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
		return fmt.Errorf("vault: sending request: %w", err)
	}

	var resp response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return fmt.Errorf("vault: reading response: %w", err)
	}
	if !resp.OK {
		return errors.New(resp.Error)
	}
	if out != nil && resp.Payload != nil {
		return json.Unmarshal(resp.Payload, out)
	}
	return nil
}

func (c *Client) PutCredential(rootSecret, dbName string, value []byte) error {
	return c.call(opPutCredential, putCredentialPayload{RootSecret: rootSecret, DBName: dbName, Value: value}, nil)
}

func (c *Client) CreateChannel(rootSecret, dbName string, scope Scope, ttl time.Duration) (Channel, error) {
	var out channelPayload
	err := c.call(opCreateChannel, createChannelPayload{RootSecret: rootSecret, DBName: dbName, Scope: scope, TTLSeconds: int(ttl.Seconds())}, &out)
	return out.Channel, err
}

func (c *Client) Activate(rootSecret string, id ChannelID, ttl time.Duration) (Channel, error) {
	var out channelPayload
	err := c.call(opActivate, activatePayload{RootSecret: rootSecret, ID: id, TTLSeconds: int(ttl.Seconds())}, &out)
	return out.Channel, err
}

func (c *Client) Query(id ChannelID, declaredTables []string, sqlText string) (QueryResult, error) {
	var out queryResultPayload
	err := c.call(opQuery, queryPayload{ID: id, DeclaredTables: declaredTables, SQL: sqlText}, &out)
	return out.Result, err
}

func (c *Client) Revoke(id ChannelID) error {
	return c.call(opRevoke, channelIDPayload{ID: id}, nil)
}

func (c *Client) ChannelScope(id ChannelID) (Channel, error) {
	var out channelPayload
	err := c.call(opChannelScope, channelIDPayload{ID: id}, &out)
	return out.Channel, err
}

func (c *Client) ListChannels() ([]Channel, error) {
	var out channelListPayload
	err := c.call(opListChannels, nil, &out)
	return out.Channels, err
}

func (c *Client) ListConnections() ([]string, error) {
	var out connectionListPayload
	err := c.call(opListConnections, nil, &out)
	return out.Names, err
}

func (c *Client) DeleteConnection(dbName string) error {
	return c.call(opDeleteConnection, dbNamePayload{DBName: dbName}, nil)
}

func (c *Client) TestConnection(connectionString string) error {
	return c.call(opTestConnection, testConnectionPayload{ConnectionString: connectionString}, nil)
}

func (c *Client) TestConnectionSaved(rootSecret, dbName string) error {
	return c.call(opTestConnectionSaved, testConnectionSavedPayload{RootSecret: rootSecret, DBName: dbName}, nil)
}

func (c *Client) HasPassphrase() (bool, error) {
	var out hasPassphrasePayload
	err := c.call(opHasPassphrase, nil, &out)
	return out.HasPassphrase, err
}

func (c *Client) SetPassphrase(rootSecret string) error {
	return c.call(opSetPassphrase, rootSecretPayload{RootSecret: rootSecret}, nil)
}

func (c *Client) VerifyPassphrase(rootSecret string) error {
	return c.call(opVerifyPassphrase, rootSecretPayload{RootSecret: rootSecret}, nil)
}

func (c *Client) ResetVault() error {
	return c.call(opResetVault, nil, nil)
}
