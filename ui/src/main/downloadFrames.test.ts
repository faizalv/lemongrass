// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

// @ts-ignore
import assert from 'node:assert/strict'
import { test } from 'node:test'
// @ts-ignore
import { DownloadDecoder, type DownloadEvent } from './downloadFrames.ts'

function frame(data: Buffer): Buffer {
  const head = Buffer.alloc(4)
  head.writeUInt32BE(data.length, 0)
  return Buffer.concat([head, data])
}

const header = Buffer.from(
  JSON.stringify({ ok: true, payload: { stream: { status: 200 } } }) + '\n'
)
const end = Buffer.alloc(4)

function decodeBytewise(input: Buffer): DownloadEvent[] {
  const decoder = new DownloadDecoder()
  const events: DownloadEvent[] = []
  for (const byte of input) events.push(...decoder.push(Buffer.from([byte])))
  return events
}

function joined(events: DownloadEvent[]): string {
  return Buffer.concat(
    events.flatMap((event) => (event.kind === 'data' ? [event.data] : []))
  ).toString()
}

test('decodes a header, frames and the end marker fed one byte at a time', () => {
  const events = decodeBytewise(
    Buffer.concat([header, frame(Buffer.from('hello ')), frame(Buffer.from('world')), end])
  )
  assert.deepEqual(events[0], {
    kind: 'header',
    line: { ok: true, payload: { stream: { status: 200 } } }
  })
  assert.equal(joined(events), 'hello world')
  assert.deepEqual(events.at(-1), { kind: 'end' })
})

test('decodes everything at once and ignores bytes after the end', () => {
  const decoder = new DownloadDecoder()
  const events = decoder.push(
    Buffer.concat([header, frame(Buffer.from('abc')), end, Buffer.from('junk')])
  )
  assert.equal(joined(events), 'abc')
  assert.equal(events.filter((event) => event.kind === 'end').length, 1)
})

test('reports an abort frame with its message', () => {
  const abort = Buffer.alloc(6)
  abort.writeUInt32BE(0xffffffff, 0)
  abort.writeUInt16BE(4, 4)
  const events = decodeBytewise(
    Buffer.concat([header, frame(Buffer.from('x')), abort, Buffer.from('boom')])
  )
  assert.deepEqual(events.at(-1), { kind: 'abort', message: 'boom' })
})

test('a header-only answer yields just the header', () => {
  const decoder = new DownloadDecoder()
  const events = decoder.push(Buffer.from('{"ok":false,"error":"vault: wrong passphrase"}\n'))
  assert.deepEqual(events, [
    { kind: 'header', line: { ok: false, error: 'vault: wrong passphrase' } }
  ])
})

test('refuses an oversized frame', () => {
  const big = Buffer.alloc(4)
  big.writeUInt32BE((1 << 20) + 1, 0)
  assert.throws(() => new DownloadDecoder().push(Buffer.concat([header, big])))
})
