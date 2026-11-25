// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { resterErrorText } from './resterErrorText.ts'

function title(raw: string): string | undefined {
  return resterErrorText(raw)?.title
}

const cases: [string, string[]][] = [
  [
    'The server took too long to answer.',
    [
      'restergate: Get "https://api.example.test/items": context deadline exceeded (Client.Timeout exceeded while awaiting headers)',
      'restergate: reading response: context deadline exceeded (Client.Timeout or context cancellation while reading body)',
      'lgrass: api.example.test did not answer within 120 s',
      'lgrass: calling api.example.test: connect ETIMEDOUT 10.0.0.5:443'
    ]
  ],
  [
    'The server cut off its response partway through.',
    [
      'restergate: reading response: stream error: stream ID 7; INTERNAL_ERROR; received from peer',
      'restergate: reading response: unexpected EOF',
      'restergate: Get "https://api.example.test": read tcp 10.0.0.2:50312->10.0.0.5:443: read: connection reset by peer',
      'lgrass: calling api.example.test: other side closed',
      'lgrass: calling api.example.test: read ECONNRESET',
      'terminated'
    ]
  ],
  [
    "The server's address could not be found.",
    [
      'restergate: Get "https://api.example.test": dial tcp: lookup api.example.test: no such host',
      'lgrass: calling api.example.test: getaddrinfo ENOTFOUND api.example.test',
      'lgrass: calling api.example.test: getaddrinfo EAI_AGAIN api.example.test'
    ]
  ],
  [
    'Nothing is answering at that address.',
    [
      'restergate: Get "http://127.0.0.1:8080": dial tcp 127.0.0.1:8080: connect: connection refused',
      'lgrass: calling 127.0.0.1:8080: connect ECONNREFUSED 127.0.0.1:8080'
    ]
  ],
  [
    'The network could not reach the server.',
    [
      'restergate: Get "https://10.1.2.3": dial tcp 10.1.2.3:443: connect: no route to host',
      'lgrass: calling 10.1.2.3: connect ENETUNREACH 10.1.2.3:443'
    ]
  ],
  [
    "The server's certificate is not trusted.",
    [
      'restergate: Get "https://api.example.test": tls: failed to verify certificate: x509: certificate signed by unknown authority',
      'lgrass: calling api.example.test: self-signed certificate',
      "lgrass: calling api.example.test: Hostname/IP does not match certificate's altnames"
    ]
  ],
  [
    'The secure connection could not be set up.',
    [
      'restergate: Get "https://api.example.test:8080": http: server gave HTTP response to HTTPS client',
      'restergate: Get "https://api.example.test": remote error: tls: handshake failure',
      'lgrass: calling api.example.test: write EPROTO 0080E2B1:error:0A00010B:SSL routines:ssl3_get_record:wrong version number'
    ]
  ]
]

test('each known failure gets its plain words, from Go and Node alike', () => {
  for (const [expected, raws] of cases) {
    for (const raw of raws) assert.equal(title(raw), expected, raw)
  }
})

test('an unknown failure is left as it is', () => {
  assert.equal(resterErrorText('restergate: scope: POST /admin is not allowed'), null)
  assert.equal(resterErrorText(''), null)
})
