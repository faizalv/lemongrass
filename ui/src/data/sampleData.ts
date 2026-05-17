import type { Project, Workspace, Task } from '../types'

export const DEFAULT_PROJECTS: Project[] = [
  {
    id: 'lemongrass-api',
    name: 'lemongrass-api-server',
    branch: 'feat/ratelimit',
    shortPath: '~/projects/lemongrass-api-server',
  },
  {
    id: 'kestrel',
    name: 'kestrel-monolith-with-a-very-long-name-that-truncates',
    branch: 'main',
    shortPath: '~/work/kestrel',
  },
  {
    id: 'wren',
    name: 'wren-cli',
    branch: 'develop',
    shortPath: '~/projects/wren-cli',
  },
]

export const DEFAULT_WORKSPACES: Record<string, Workspace[]> = {
  'lemongrass-api': [
    { id: 'reconnaissance', name: 'Reconnaissance', icon: 'radar' },
    { id: 'ws-ratelimit', name: 'Add per-user rate limiting', icon: 'gauge', status: 'grooming' },
    { id: 'ws-auth', name: 'Fix auth refresh window edge case', icon: 'shield-half', status: 'idle' },
    { id: 'ws-cache', name: 'Introduce cache layer for /search endpoint', icon: 'database', status: 'idle' },
    { id: 'ws-migrate', name: 'Migrate user_events table to pgvector', icon: 'database-zap', status: 'idle' },
  ],
  'kestrel': [
    { id: 'reconnaissance', name: 'Reconnaissance', icon: 'radar' },
    { id: 'ws-billing', name: 'Quarterly billing reconciliation', icon: 'receipt', status: 'idle' },
  ],
  'wren': [
    { id: 'reconnaissance', name: 'Reconnaissance', icon: 'radar' },
  ],
}

export const PROPOSED_TASKS: Task[] = [
  {
    id: 't1',
    title: 'Introduce a Redis-backed rate-limit middleware',
    prdRef: '60 req/min for anon, 300 req/min for authenticated. Counters in Redis, keyed by IP or user_id.',
    howTo: 'Add a chi middleware that resolves the bucket key (IP vs. user_id), increments a sliding-window counter in Redis via INCRBY/EXPIRE, and short-circuits with 429 when over limit.',
    files: [
      { path: 'internal/middleware/ratelimit.go', range: 'new file', note: 'middleware + bucket helpers' },
      { path: 'internal/middleware/ratelimit_test.go', range: 'new file', note: 'unit tests w/ miniredis' },
      { path: 'cmd/server/main.go', range: 'L114–L128', note: 'mount middleware on /api/v1' },
    ],
    estTokens: '~2.1k',
  },
  {
    id: 't2',
    title: 'Emit X-RateLimit-* headers on every response',
    prdRef: 'Surface X-RateLimit-Remaining (etc.) on every response.',
    howTo: 'Have the middleware stash remaining/limit/reset in the request context; a response wrapper reads them and writes the three standard headers before the handler flushes.',
    files: [
      { path: 'internal/middleware/ratelimit.go', range: 'L82–L118', note: 'header writer hook' },
      { path: 'internal/transport/response.go', range: 'L41–L62', note: 'context lookup + header set' },
    ],
    estTokens: '~1.4k',
  },
  {
    id: 't3',
    title: 'Return 429 with Retry-After when over the bucket',
    prdRef: 'Return 429 with a Retry-After header.',
    howTo: 'When the middleware decides the request is over quota, compute seconds-until-reset from the Redis TTL and write a structured error body with Retry-After set.',
    files: [
      { path: 'internal/middleware/ratelimit.go', range: 'L120–L156', note: 'short-circuit branch' },
      { path: 'internal/errors/errors.go', range: 'L88–L95', note: 'add ErrRateLimited' },
    ],
    estTokens: '~0.9k',
  },
  {
    id: 't4',
    title: "Admin endpoint to inspect a user's quota",
    prdRef: "Admin endpoint to inspect a user's current quota.",
    howTo: 'Add GET /api/v1/admin/users/{id}/quota returning current count, limit, and seconds until reset. Gate it behind the existing admin auth middleware.',
    files: [
      { path: 'internal/handlers/admin/quota.go', range: 'new file', note: 'handler + DTO' },
      { path: 'internal/handlers/admin/router.go', range: 'L24–L31', note: 'register route' },
    ],
    estTokens: '~1.2k',
  },
]

export const RECON_PATH_INFO = {
  path: 'internal/transport/',
  fileCount: 14,
  estTokens: '~6.4k',
  preview: [
    'internal/transport/response.go',
    'internal/transport/request.go',
    'internal/transport/middleware/auth.go',
    'internal/transport/middleware/log.go',
    '… +10 more',
  ],
}
