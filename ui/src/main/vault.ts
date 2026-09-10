import { ipcMain } from 'electron'
import * as net from 'net'
import { homedir } from 'os'
import { join } from 'path'

function vaultSocketPath(): string {
  return join(homedir(), '.lemongrass', 'vault.sock')
}

function agentSocketPath(): string {
  return join(homedir(), '.lemongrass', 'agent.sock')
}

interface RPCResponse<T> {
  ok: boolean
  error?: string
  payload?: T
}

// One request per connection, mirroring the Go side (vault/ipc.go, agent/ipc.go): write one
// JSON line, then read until the peer closes -- both daemons answer exactly once per connection.
function connectOnce<T>(socketPath: string, op: string, payload?: unknown): Promise<T> {
  return new Promise((resolve, reject) => {
    const socket = net.createConnection(socketPath)
    const chunks: Buffer[] = []
    socket.on('connect', () => {
      socket.end(JSON.stringify({ op, payload }) + '\n')
    })
    socket.on('data', (chunk) => chunks.push(chunk))
    socket.on('error', reject)
    socket.on('close', () => {
      if (chunks.length === 0) {
        reject(new Error(`lgrass: no response from ${op}`))
        return
      }
      try {
        const resp = JSON.parse(Buffer.concat(chunks).toString('utf-8')) as RPCResponse<T>
        if (!resp.ok) {
          reject(new Error(resp.error || `lgrass: ${op} failed`))
          return
        }
        resolve(resp.payload as T)
      } catch (err) {
        reject(err)
      }
    })
  })
}

function isConnectionRace(err: unknown): boolean {
  const code = (err as NodeJS.ErrnoException)?.code
  return code === 'ENOENT' || code === 'ECONNREFUSED'
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

// Retries only the connection-race window right after daemons.ts spawns the vault/agent
// daemon on this session's own launch -- the socket file may not exist for the first call
// or two yet. Any other failure, including an op-level error the daemon itself returned,
// surfaces immediately rather than retrying.
async function call<T>(
  socketPath: string,
  op: string,
  payload?: unknown,
  deadline = Date.now() + 2000
): Promise<T> {
  try {
    return await connectOnce<T>(socketPath, op, payload)
  } catch (err) {
    if (!isConnectionRace(err) || Date.now() >= deadline) throw err
    await sleep(100)
    return call<T>(socketPath, op, payload, deadline)
  }
}

function vaultCall<T>(op: string, payload?: unknown): Promise<T> {
  return call<T>(vaultSocketPath(), op, payload)
}

function agentCall<T>(op: string, payload?: unknown): Promise<T> {
  return call<T>(agentSocketPath(), op, payload)
}

export interface Scope {
  Tables: string[]
  Operations: string[]
}

export interface Channel {
  ID: string
  DBName: string
  Scope: Scope
  CreatedAt: string
  ExpiresAt: string
}

export interface ChannelWithShortId {
  channel: Channel
  shortId: string
}

async function listChannels(): Promise<Channel[]> {
  const { channels } = await vaultCall<{ channels: Channel[] }>('list_channels')
  return channels ?? []
}

async function listConnections(): Promise<string[]> {
  const { names } = await vaultCall<{ names: string[] }>('list_connections')
  return names ?? []
}

// Writes a connection's credential (engine folded into the connection string itself, e.g.
// `mysql://user:pass@host:port/db`) under the master passphrase. Never read back -- the vault
// has no op that returns a decrypted credential value to a caller.
async function putCredential(
  passphrase: string,
  name: string,
  connectionString: string
): Promise<void> {
  await vaultCall<void>('put_credential', {
    root_secret: passphrase,
    db_name: name,
    value: Buffer.from(connectionString, 'utf-8').toString('base64')
  })
}

// Registers with the agent right after, so the human has a short id to relay to the model --
// the root secret stops here, it never transits the agent (book chapter: "authority lives in
// the vault, not the agent").
async function createChannel(
  passphrase: string,
  dbName: string,
  scope: Scope,
  ttlSeconds: number
): Promise<ChannelWithShortId> {
  const { channel } = await vaultCall<{ channel: Channel }>('create_channel', {
    root_secret: passphrase,
    db_name: dbName,
    scope,
    ttl_seconds: ttlSeconds
  })
  const { short_id: shortId } = await agentCall<{ short_id: string }>('register_channel', {
    real_id: channel.ID
  })
  return { channel, shortId }
}

async function activateChannel(
  passphrase: string,
  id: string,
  ttlSeconds: number
): Promise<ChannelWithShortId> {
  const { channel } = await vaultCall<{ channel: Channel }>('activate', {
    root_secret: passphrase,
    id,
    ttl_seconds: ttlSeconds
  })
  const { short_id: shortId } = await agentCall<{ short_id: string }>('register_channel', {
    real_id: channel.ID
  })
  return { channel, shortId }
}

// Revoke only writes through to the vault. The vault is authoritative on validity regardless
// of what the agent's own short-id mapping still thinks it knows (book chapter: "Authority
// lives in the vault, not the agent") -- a stale agent-side mapping to a revoked channel just
// fails at the next Query, so there's nothing this call needs from the agent to be correct.
async function revokeChannel(id: string): Promise<void> {
  await vaultCall<void>('revoke', { id })
}

export function registerVaultHandlers(): void {
  ipcMain.handle('vault:list', () => listChannels())
  ipcMain.handle(
    'vault:create',
    (_event, passphrase: string, dbName: string, scope: Scope, ttlSeconds: number) =>
      createChannel(passphrase, dbName, scope, ttlSeconds)
  )
  ipcMain.handle('vault:activate', (_event, passphrase: string, id: string, ttlSeconds: number) =>
    activateChannel(passphrase, id, ttlSeconds)
  )
  ipcMain.handle('vault:revoke', (_event, id: string) => revokeChannel(id))
  ipcMain.handle('vault:listConnections', () => listConnections())
  ipcMain.handle(
    'vault:putCredential',
    (_event, passphrase: string, name: string, connectionString: string) =>
      putCredential(passphrase, name, connectionString)
  )
}
