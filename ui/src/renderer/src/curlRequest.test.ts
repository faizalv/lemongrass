// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { looksLikeCurl, matchBaseURL, parseCurl, shellWords } from './curlRequest.ts'

test('shell words follow bash quoting', () => {
  assert.deepEqual(shellWords(`curl 'a b' "c \\"d\\"" $'e\\nf' g\\ h \\\n  i`), [
    'curl',
    'a b',
    'c "d"',
    'e\nf',
    'g h',
    'i'
  ])
})

test('a browser copy as cURL keeps method, url, headers and json body, and drops auth', () => {
  const parsed = parseCurl(
    `curl 'https://api.example.test/v1/orders?page=2' \\
  -X 'PATCH' \\
  -H 'accept: application/json' \\
  -H 'authorization: Bearer abc' \\
  -H 'content-type: application/json' \\
  -H 'cookie: sid=1' \\
  --data-raw '{"status":"paid"}' \\
  --compressed`
  )
  assert.equal(parsed.method, 'PATCH')
  assert.equal(parsed.url, 'https://api.example.test/v1/orders?page=2')
  assert.deepEqual(parsed.headers, [{ name: 'accept', value: 'application/json' }])
  assert.equal(parsed.bodyKind, 'json')
  assert.equal(parsed.json, '{\n  "status": "paid"\n}')
  assert.deepEqual(parsed.dropped, ['authorization', 'cookie'])
})

test('data without a json content type becomes form fields and implies POST', () => {
  const parsed = parseCurl(
    `curl https://x.test/login -d 'name=a%20b&tag=1' --data-urlencode 'q=c&d'`
  )
  assert.equal(parsed.method, 'POST')
  assert.equal(parsed.bodyKind, 'form')
  assert.deepEqual(parsed.form, [
    { name: 'name', value: 'a b' },
    { name: 'tag', value: '1' },
    { name: 'q', value: 'c&d' }
  ])
})

test('raw json data is detected without a content type', () => {
  const parsed = parseCurl(`curl -XPUT https://x.test/a -d '[1,2]'`)
  assert.equal(parsed.method, 'PUT')
  assert.equal(parsed.bodyKind, 'json')
})

test('-G moves data to the query string', () => {
  const parsed = parseCurl(`curl -sG https://x.test/search -d q=lemon`)
  assert.equal(parsed.method, 'GET')
  assert.equal(parsed.url, 'https://x.test/search?q=lemon')
  assert.equal(parsed.bodyKind, 'none')
})

test('the rester token header and --user are left out, and -F fills a multipart body', () => {
  const parsed = parseCurl(
    `curl https://x.test/up -H 'X-Api-Key: k' -u me:pw -F 'title=hi' -F 'file=@/tmp/a.png;type=image/webp;filename=b.png'`,
    'x-api-key'
  )
  assert.deepEqual(parsed.dropped, ['X-Api-Key', '--user'])
  assert.equal(parsed.bodyKind, 'multipart')
  assert.equal(parsed.method, 'POST')
  assert.deepEqual(parsed.multipart, [
    { name: 'title', kind: 'text', value: 'hi', path: '', size: 0, contentType: '' },
    {
      name: 'file',
      kind: 'file',
      value: '',
      path: '/tmp/a.png',
      size: 0,
      contentType: 'image/webp'
    }
  ])
  assert.deepEqual(parsed.notes, [])
})

test('--form-string stays text, a < field is left out, and a relative file gets a note', () => {
  const parsed = parseCurl(
    `curl https://x.test/up --form-string 'at=@home' -F 'body=<notes.txt' -F 'doc=@docs/a.pdf'`
  )
  assert.deepEqual(
    parsed.multipart.map((part) => [part.name, part.kind, part.value, part.path]),
    [
      ['at', 'text', '@home', ''],
      ['doc', 'file', '', 'docs/a.pdf']
    ]
  )
  assert.deepEqual(parsed.notes, [
    'The field body reads its value from a file and was left out.',
    'The file docs/a.pdf is a relative path. Choose it again.'
  ])
})

test('--data-binary @file fills a file body with the pasted content type', () => {
  const parsed = parseCurl(
    `curl -X PUT https://x.test/import -H 'Content-Type: text/csv' --data-binary @/srv/rows.csv`
  )
  assert.equal(parsed.bodyKind, 'file')
  assert.equal(parsed.method, 'PUT')
  assert.deepEqual(parsed.file, { path: '/srv/rows.csv', size: 0, contentType: 'text/csv' })
  assert.deepEqual(parsed.headers, [])
  const plain = parseCurl(`curl https://x.test/import -d @rows.csv`)
  assert.equal(plain.bodyKind, 'none')
  assert.deepEqual(plain.notes, ['The body file rows.csv was not read.'])
})

test('looksLikeCurl only matches a curl command', () => {
  assert.ok(looksLikeCurl('  curl https://x.test'))
  assert.ok(!looksLikeCurl('/curl/path'))
})

test('matchBaseURL picks the longest base on a segment boundary', () => {
  const targets = [
    { rester: 'shop', environment: 'staging', baseURL: 'https://api.example.test' },
    { rester: 'shop', environment: 'v2', baseURL: 'https://api.example.test/v2/' },
    { rester: 'other', environment: 'prod', baseURL: 'https://api.example.testing' }
  ]
  assert.deepEqual(matchBaseURL('https://api.example.test/v2/orders?x=1', targets), {
    target: targets[1],
    path: '/orders?x=1'
  })
  assert.equal(
    matchBaseURL('https://api.example.test/v20/a', targets)?.target.environment,
    'staging'
  )
  assert.equal(matchBaseURL('https://elsewhere.test/a', targets), null)
})
