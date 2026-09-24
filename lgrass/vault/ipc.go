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
	opListTables          = "list_tables"
	opHasPassphrase       = "has_passphrase"
	opSetPassphrase       = "set_passphrase"
	opVerifyPassphrase    = "verify_passphrase"
	opResetVault          = "reset_vault"

	opPutDomain         = "put_domain"
	opListDomains       = "list_domains"
	opDeleteDomain      = "delete_domain"
	opTestDomainLogin   = "test_domain_login"
	opCreateHTTPChannel = "create_http_channel"
	opActivateHTTP      = "activate_http"
	opRequestHTTP       = "request_http"
	opRevokeHTTP        = "revoke_http"
	opHTTPChannelScope  = "http_channel_scope"
	opListHTTPChannels  = "list_http_channels"
	opHTTPChannelInfo   = "http_channel_info"
	opHTTPChannelUsers  = "http_channel_users"
	opGetDomain         = "get_domain"
	opUpdateDomain      = "update_domain"
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
	Name       string `json:"name"`
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

type tablesPayload struct {
	Tables []string `json:"tables"`
}

type hasPassphrasePayload struct {
	HasPassphrase bool `json:"has_passphrase"`
}

type putDomainPayload struct {
	RootSecret string `json:"root_secret"`
	Name       string `json:"name"`
	Domain     Domain `json:"domain"`
}

type domainNamePayload struct {
	Name string `json:"name"`
}

type domainListPayload struct {
	Names []string `json:"names"`
}

type testDomainLoginPayload struct {
	Domain Domain     `json:"domain"`
	User   DomainUser `json:"user"`
}

type createHTTPChannelPayload struct {
	RootSecret string    `json:"root_secret"`
	Name       string    `json:"name"`
	DomainName string    `json:"domain_name"`
	Scope      HTTPScope `json:"scope"`
	TTLSeconds int       `json:"ttl_seconds"`
}

type activateHTTPPayload struct {
	RootSecret string    `json:"root_secret"`
	ID         ChannelID `json:"id"`
	TTLSeconds int       `json:"ttl_seconds"`
}

type requestHTTPPayload struct {
	ID     ChannelID `json:"id"`
	User   string    `json:"user"`
	Method string    `json:"method"`
	Path   string    `json:"path"`
	Body   []byte    `json:"body"`
}

type httpResultPayload struct {
	Result HTTPResult `json:"result"`
}

type httpChannelPayload struct {
	Channel HTTPChannel `json:"channel"`
}

type httpChannelInfoPayload struct {
	Info HTTPChannelInfo `json:"info"`
}

type httpChannelUsersPayload struct {
	Users []HTTPUserInfo `json:"users"`
}

type getDomainPayload struct {
	RootSecret string `json:"root_secret"`
	Name       string `json:"name"`
}

type domainPayload struct {
	Domain Domain `json:"domain"`
}

type httpChannelListPayload struct {
	Channels []HTTPChannel `json:"channels"`
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
	return op == opQuery || op == opChannelScope || op == opRequestHTTP || op == opHTTPChannelScope || op == opHTTPChannelInfo || op == opHTTPChannelUsers
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
			c, err := svc.CreateChannel(p.RootSecret, p.Name, p.DBName, p.Scope, time.Duration(p.TTLSeconds)*time.Second)
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

	case opListTables:
		var p testConnectionSavedPayload
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return errResponse(err)
		}
		return adminOp(adminLimiter, func() (response, error) {
			tables, err := svc.ListTables(p.RootSecret, p.DBName)
			return payloadResponse(tablesPayload{Tables: tables}), err
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

	case opPutDomain:
		var p putDomainPayload
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return errResponse(err)
		}
		return adminOp(adminLimiter, func() (response, error) {
			return response{OK: true}, svc.PutDomain(p.RootSecret, p.Name, p.Domain)
		})

	case opListDomains:
		names, err := svc.ListDomains()
		if err != nil {
			return errResponse(err)
		}
		return payloadResponse(domainListPayload{Names: names})

	case opDeleteDomain:
		var p domainNamePayload
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return errResponse(err)
		}
		if err := svc.DeleteDomain(p.Name); err != nil {
			return errResponse(err)
		}
		return response{OK: true}

	case opTestDomainLogin:
		var p testDomainLoginPayload
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return errResponse(err)
		}
		if err := svc.TestDomainLogin(p.Domain, p.User); err != nil {
			return errResponse(err)
		}
		return response{OK: true}

	case opCreateHTTPChannel:
		var p createHTTPChannelPayload
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return errResponse(err)
		}
		return adminOp(adminLimiter, func() (response, error) {
			c, err := svc.CreateHTTPChannel(p.RootSecret, p.Name, p.DomainName, p.Scope, time.Duration(p.TTLSeconds)*time.Second)
			return payloadResponse(httpChannelPayload{Channel: c}), err
		})

	case opActivateHTTP:
		var p activateHTTPPayload
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return errResponse(err)
		}
		return adminOp(adminLimiter, func() (response, error) {
			c, err := svc.ActivateHTTP(p.RootSecret, p.ID, time.Duration(p.TTLSeconds)*time.Second)
			return payloadResponse(httpChannelPayload{Channel: c}), err
		})

	case opRequestHTTP:
		var p requestHTTPPayload
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return errResponse(err)
		}
		result, err := svc.RequestHTTP(p.ID, p.User, p.Method, p.Path, p.Body)
		if err != nil {
			return errResponse(err)
		}
		return payloadResponse(httpResultPayload{Result: result})

	case opRevokeHTTP:
		var p channelIDPayload
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return errResponse(err)
		}
		if err := svc.RevokeHTTP(p.ID); err != nil {
			return errResponse(err)
		}
		return response{OK: true}

	case opHTTPChannelScope:
		var p channelIDPayload
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return errResponse(err)
		}
		c, err := svc.HTTPChannelScope(p.ID)
		if err != nil {
			return errResponse(err)
		}
		return payloadResponse(httpChannelPayload{Channel: c})

	case opHTTPChannelInfo:
		var p channelIDPayload
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return errResponse(err)
		}
		info, err := svc.HTTPChannelInfo(p.ID)
		if err != nil {
			return errResponse(err)
		}
		return payloadResponse(httpChannelInfoPayload{Info: info})

	case opHTTPChannelUsers:
		var p channelIDPayload
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return errResponse(err)
		}
		users, err := svc.HTTPChannelUsers(p.ID)
		if err != nil {
			return errResponse(err)
		}
		return payloadResponse(httpChannelUsersPayload{Users: users})

	case opGetDomain:
		var p getDomainPayload
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return errResponse(err)
		}
		return adminOp(adminLimiter, func() (response, error) {
			d, err := svc.GetDomain(p.RootSecret, p.Name)
			return payloadResponse(domainPayload{Domain: d}), err
		})

	case opUpdateDomain:
		var p putDomainPayload
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return errResponse(err)
		}
		return adminOp(adminLimiter, func() (response, error) {
			return response{OK: true}, svc.UpdateDomain(p.RootSecret, p.Name, p.Domain)
		})

	case opListHTTPChannels:
		channels, err := svc.ListHTTPChannels()
		if err != nil {
			return errResponse(err)
		}
		return payloadResponse(httpChannelListPayload{Channels: channels})

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

func (c *Client) CreateChannel(rootSecret, name, dbName string, scope Scope, ttl time.Duration) (Channel, error) {
	var out channelPayload
	err := c.call(opCreateChannel, createChannelPayload{RootSecret: rootSecret, Name: name, DBName: dbName, Scope: scope, TTLSeconds: int(ttl.Seconds())}, &out)
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

func (c *Client) ListTables(rootSecret, dbName string) ([]string, error) {
	var out tablesPayload
	err := c.call(opListTables, testConnectionSavedPayload{RootSecret: rootSecret, DBName: dbName}, &out)
	return out.Tables, err
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

func (c *Client) PutDomain(rootSecret, name string, d Domain) error {
	return c.call(opPutDomain, putDomainPayload{RootSecret: rootSecret, Name: name, Domain: d}, nil)
}

func (c *Client) ListDomains() ([]string, error) {
	var out domainListPayload
	err := c.call(opListDomains, nil, &out)
	return out.Names, err
}

func (c *Client) DeleteDomain(name string) error {
	return c.call(opDeleteDomain, domainNamePayload{Name: name}, nil)
}

func (c *Client) TestDomainLogin(d Domain, u DomainUser) error {
	return c.call(opTestDomainLogin, testDomainLoginPayload{Domain: d, User: u}, nil)
}

func (c *Client) CreateHTTPChannel(rootSecret, name, domainName string, scope HTTPScope, ttl time.Duration) (HTTPChannel, error) {
	var out httpChannelPayload
	err := c.call(opCreateHTTPChannel, createHTTPChannelPayload{RootSecret: rootSecret, Name: name, DomainName: domainName, Scope: scope, TTLSeconds: int(ttl.Seconds())}, &out)
	return out.Channel, err
}

func (c *Client) ActivateHTTP(rootSecret string, id ChannelID, ttl time.Duration) (HTTPChannel, error) {
	var out httpChannelPayload
	err := c.call(opActivateHTTP, activateHTTPPayload{RootSecret: rootSecret, ID: id, TTLSeconds: int(ttl.Seconds())}, &out)
	return out.Channel, err
}

func (c *Client) RequestHTTP(id ChannelID, user, method, path string, body []byte) (HTTPResult, error) {
	var out httpResultPayload
	err := c.call(opRequestHTTP, requestHTTPPayload{ID: id, User: user, Method: method, Path: path, Body: body}, &out)
	return out.Result, err
}

func (c *Client) RevokeHTTP(id ChannelID) error {
	return c.call(opRevokeHTTP, channelIDPayload{ID: id}, nil)
}

func (c *Client) HTTPChannelScope(id ChannelID) (HTTPChannel, error) {
	var out httpChannelPayload
	err := c.call(opHTTPChannelScope, channelIDPayload{ID: id}, &out)
	return out.Channel, err
}

func (c *Client) HTTPChannelInfo(id ChannelID) (HTTPChannelInfo, error) {
	var out httpChannelInfoPayload
	err := c.call(opHTTPChannelInfo, channelIDPayload{ID: id}, &out)
	return out.Info, err
}

func (c *Client) HTTPChannelUsers(id ChannelID) ([]HTTPUserInfo, error) {
	var out httpChannelUsersPayload
	err := c.call(opHTTPChannelUsers, channelIDPayload{ID: id}, &out)
	return out.Users, err
}

func (c *Client) GetDomain(rootSecret, name string) (Domain, error) {
	var out domainPayload
	err := c.call(opGetDomain, getDomainPayload{RootSecret: rootSecret, Name: name}, &out)
	return out.Domain, err
}

func (c *Client) UpdateDomain(rootSecret, name string, d Domain) error {
	return c.call(opUpdateDomain, putDomainPayload{RootSecret: rootSecret, Name: name, Domain: d}, nil)
}

func (c *Client) ListHTTPChannels() ([]HTTPChannel, error) {
	var out httpChannelListPayload
	err := c.call(opListHTTPChannels, nil, &out)
	return out.Channels, err
}
