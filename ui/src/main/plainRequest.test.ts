// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

// @ts-ignore node:assert has no types in the node tsconfig
import assert from 'node:assert/strict'
import { mkdtempSync, readFileSync } from 'node:fs'
import { createServer, type IncomingMessage, type ServerResponse } from 'node:http'
import type { AddressInfo } from 'node:net'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { after, test } from 'node:test'
import type { ResterClientDownload, ResterClientResult } from '../preload/types'
import {
  decodeText,
  downloadName,
  plainDownload,
  plainRequest,
  plainURL,
  PLAIN_INLINE_BYTES
  // @ts-ignore node --test imports the .ts source by its extension
} from './plainRequest.ts'

interface Seen {
  method: string
  url: string
  headers: IncomingMessage['headers']
  body: string
}

const seen: Seen[] = []

function respond(req: IncomingMessage, res: ServerResponse, body: string): void {
  seen.push({ method: req.method ?? '', url: req.url ?? '', headers: req.headers, body })
  if (req.url === '/big') {
    res.end(Buffer.concat([Buffer.alloc(PLAIN_INLINE_BYTES - 1, 'a'), Buffer.from('é')]))
  } else if (req.url === '/binary') {
    res.end(Buffer.from([0xff, 0xfe, 0x00]))
  } else if (req.url === '/redirect') {
    res.writeHead(302, { Location: '/echo' }).end()
  } else if (req.url === '/file') {
    res.writeHead(200, { 'Content-Disposition': 'attachment; filename="report 1.csv"' })
    res.end('a,b\n1,2\n')
  } else if (req.url === '/missing') {
    res.writeHead(404, { 'Content-Type': 'application/json' }).end('{"error":"nope"}')
  } else {
    res.setHeader('Set-Cookie', ['a=1', 'b=2'])
    res.setHeader('X-Trace-Id', 't1')
    res.end(JSON.stringify({ ok: true }))
  }
}

const server = createServer((req, res) => {
  const chunks: Buffer[] = []
  req.on('data', (chunk: Buffer) => chunks.push(chunk))
  req.on('end', () => respond(req, res, Buffer.concat(chunks).toString('utf-8')))
})
await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve))
const base = `http://127.0.0.1:${(server.address() as AddressInfo).port}`
after(() => server.close())

// What callBody builds for a text body.
function built(c: Record<string, unknown>): { body: Buffer; contentType: string } {
  return { body: Buffer.from(c.body as string), contentType: c.contentType as string }
}

function send(c: Record<string, unknown>): Promise<ResterClientResult> {
  return plainRequest(c, built(c))
}

function download(
  c: Record<string, unknown>,
  choose: (name: string) => Promise<string | null>
): Promise<ResterClientDownload> {
  return plainDownload(c, built(c), choose)
}

function call(path: string, over: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    rester: '',
    environment: '',
    user: '',
    method: 'GET',
    path,
    body: '',
    contentType: '',
    headers: [],
    ...over
  }
}

test('a Plain request needs a full http or https URL', () => {
  assert.throws(() => plainURL('/api/login'), /needs a full URL/)
  assert.throws(() => plainURL('ftp://x.test/a'), /http or https/)
  assert.equal(plainURL(' https://x.test/a ').href, 'https://x.test/a')
})

test('headers go out as written and a JSON body defaults its type', async () => {
  const result = await send(
    call(`${base}/echo`, {
      method: 'POST',
      body: '{"name":"a"}',
      headers: [
        { name: 'Authorization', value: 'Bearer abc' },
        { name: 'Cookie', value: 'sid=1' }
      ]
    })
  )
  const last = seen.at(-1)!
  assert.equal(last.method, 'POST')
  assert.equal(last.body, '{"name":"a"}')
  assert.equal(last.headers['content-type'], 'application/json')
  assert.equal(last.headers['authorization'], 'Bearer abc')
  assert.equal(last.headers['cookie'], 'sid=1')
  assert.equal(result.status, 200)
  assert.equal(result.body, '{"ok":true}')
  assert.deepEqual(result.headers['Set-Cookie'], ['a=1', 'b=2'])
  assert.deepEqual(result.headers['X-Trace-Id'], ['t1'])
  assert.equal(result.binary, false)
})

test('a written Content-Type wins over the JSON default but not over a form body', async () => {
  await send(
    call(`${base}/echo`, {
      method: 'POST',
      body: 'x',
      headers: [{ name: 'Content-Type', value: 'text/plain' }]
    })
  )
  assert.equal(seen.at(-1)!.headers['content-type'], 'text/plain')
  await send(
    call(`${base}/echo`, {
      method: 'POST',
      body: 'a=1',
      contentType: 'application/x-www-form-urlencoded',
      headers: [{ name: 'Content-Type', value: 'text/plain' }]
    })
  )
  assert.equal(seen.at(-1)!.headers['content-type'], 'application/x-www-form-urlencoded')
})

test('redirects are followed and the final URL is reported', async () => {
  const result = await send(call(`${base}/redirect`))
  assert.equal(result.status, 200)
  assert.equal(result.url, `${base}/echo`)
})

test('a body past the inline cap is cut, forgiving a split character', async () => {
  const result = await send(call(`${base}/big`))
  assert.equal(result.truncated, true)
  assert.equal(result.size, PLAIN_INLINE_BYTES)
  assert.equal(result.binary, false)
  assert.equal(result.body.length, PLAIN_INLINE_BYTES - 1)
})

test('a body that is not UTF-8 is left out as binary', async () => {
  const result = await send(call(`${base}/binary`))
  assert.equal(result.binary, true)
  assert.equal(result.body, '')
  assert.equal(decodeText(Buffer.from([0x61, 0xc3]), false).binary, true)
  assert.deepEqual(decodeText(Buffer.from([0x61, 0xc3]), true), { body: 'a', binary: false })
})

test('an unreachable host fails with an lgrass error', async () => {
  await assert.rejects(send(call('http://127.0.0.1:1/x')), /^Error: lgrass: calling/)
})

test('a download saves a 2xx body under the server name and shows any other status inline', async () => {
  const dir = mkdtempSync(join(tmpdir(), 'plain-download-'))
  let offered = ''
  const saved = await download(call(`${base}/file`), async (name: string) => {
    offered = name
    return join(dir, name)
  })
  assert.equal(offered, 'report 1.csv')
  assert.equal(saved.kind, 'saved')
  assert.equal(readFileSync(join(dir, 'report 1.csv'), 'utf-8'), 'a,b\n1,2\n')

  const cancelled = await download(call(`${base}/file`), async () => null)
  assert.deepEqual(cancelled, { kind: 'cancelled' })

  const inline = await download(call(`${base}/missing`), async () => {
    throw new Error('no dialog for a 404')
  })
  assert.equal(inline.kind, 'inline')
  assert.equal(inline.kind === 'inline' ? inline.result.status : 0, 404)
})

test('the download name comes from Content-Disposition, then the URL path', () => {
  assert.equal(downloadName("attachment; filename*=UTF-8''r%C3%A9sum%C3%A9.pdf", 'x'), 'résumé.pdf')
  assert.equal(downloadName('attachment; filename="../../etc/passwd"', 'x'), 'passwd')
  assert.equal(downloadName('attachment; filename=a.txt', 'x'), 'a.txt')
  assert.equal(downloadName(null, 'https://x.test/files/q%201.xlsx?v=2'), 'q 1.xlsx')
  assert.equal(downloadName(null, 'https://x.test/'), '')
})
