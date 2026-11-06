// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

// @ts-ignore node:assert has no types in the node tsconfig
import assert from 'node:assert/strict'
import { mkdirSync, mkdtempSync, truncateSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { test } from 'node:test'
import {
  buildFileBody,
  buildMultipart,
  fileTypeFor,
  MAX_REQUEST_BODY_BYTES
  // @ts-ignore node --test imports the .ts source by its extension
} from './resterUpload.ts'

function part(over: Record<string, unknown>): Record<string, unknown> {
  return { name: '', kind: 'text', value: '', path: '', size: 0, contentType: '', ...over }
}

// Bytes that are not valid UTF-8 and hold CR LF, so any text decoding along the way would show.
function binaryFile(dir: string, name: string): { path: string; bytes: Buffer } {
  const bytes = Buffer.from([
    0x50, 0x4b, 0x03, 0x04, 0xff, 0xfe, 0x00, 0x0d, 0x0a, 0x80, 0x2d, 0x2d
  ])
  const path = join(dir, name)
  writeFileSync(path, bytes)
  return { path, bytes }
}

async function parse(body: Buffer, contentType: string): Promise<FormData> {
  return new Response(body, { headers: { 'content-type': contentType } }).formData()
}

test('a multipart body carries text fields and file bytes unchanged', async () => {
  const dir = mkdtempSync(join(tmpdir(), 'upload-'))
  const sheet = binaryFile(dir, 'report.xlsx')
  const built = await buildMultipart([
    part({ name: 'file', kind: 'file', path: sheet.path }),
    part({ name: 'note', value: 'quarter "one"' })
  ])
  assert.match(built.contentType, /^multipart\/form-data; boundary=[0-9a-f]{32}$/)
  const form = await parse(built.body, built.contentType)
  assert.equal(form.get('note'), 'quarter "one"')
  const file = form.get('file') as File
  assert.equal(file.name, 'report.xlsx')
  assert.equal(file.type, fileTypeFor('report.xlsx'))
  assert.deepEqual(Buffer.from(await file.arrayBuffer()), sheet.bytes)
})

test('text fields come before files, files keep row order', async () => {
  const dir = mkdtempSync(join(tmpdir(), 'upload-'))
  const a = binaryFile(dir, 'a.bin')
  const b = binaryFile(dir, 'b.bin')
  const built = await buildMultipart(
    [
      part({ name: 'second', kind: 'file', path: b.path }),
      part({ name: 'text', value: 'v' }),
      part({ name: 'first', kind: 'file', path: a.path, contentType: 'text/csv' })
    ],
    'XB'
  )
  const text = built.body.toString('latin1')
  assert.ok(text.indexOf('name="text"') < text.indexOf('name="second"'))
  assert.ok(text.indexOf('name="second"') < text.indexOf('name="first"'))
  assert.match(text, /name="first"; filename="a.bin"\r\nContent-Type: text\/csv\r\n/)
  assert.match(
    text,
    /name="second"; filename="b.bin"\r\nContent-Type: application\/octet-stream\r\n/
  )
  assert.ok(text.endsWith('--XB--\r\n'))
})

test('a file body is the file bytes with a type from the extension or the override', async () => {
  const dir = mkdtempSync(join(tmpdir(), 'upload-'))
  const data = binaryFile(dir, 'data.csv')
  const auto = await buildFileBody({ path: data.path, size: 0, contentType: '' })
  assert.deepEqual(auto.body, data.bytes)
  assert.equal(auto.contentType, 'text/csv')
  const set = await buildFileBody({ path: data.path, size: 0, contentType: 'application/x-thing' })
  assert.equal(set.contentType, 'application/x-thing')
})

test('a missing file, a directory and a relative path fail with the path named', async () => {
  const dir = mkdtempSync(join(tmpdir(), 'upload-'))
  const missing = join(dir, 'gone.pdf')
  await assert.rejects(buildFileBody({ path: missing, size: 0, contentType: '' }), (err: Error) =>
    err.message.includes(missing)
  )
  mkdirSync(join(dir, 'folder'))
  await assert.rejects(
    buildMultipart([part({ name: 'f', kind: 'file', path: join(dir, 'folder') })]),
    /is not a regular file/
  )
  await assert.rejects(
    buildFileBody({ path: 'docs/a.pdf', size: 0, contentType: '' }),
    /docs\/a.pdf is not an absolute path/
  )
  await assert.rejects(
    buildMultipart([part({ name: 'f', kind: 'file', path: '' })]),
    /choose a file for the field f/
  )
})

test('a body over the limit fails before any file is read', async () => {
  const dir = mkdtempSync(join(tmpdir(), 'upload-'))
  const big = join(dir, 'big.bin')
  writeFileSync(big, '')
  truncateSync(big, MAX_REQUEST_BODY_BYTES + 1)
  await assert.rejects(
    buildFileBody({ path: big, size: 0, contentType: '' }),
    /25 MiB request limit/
  )
  const half = join(dir, 'half.bin')
  writeFileSync(half, '')
  truncateSync(half, MAX_REQUEST_BODY_BYTES / 2 + 1)
  await assert.rejects(
    buildMultipart([
      part({ name: 'a', kind: 'file', path: half }),
      part({ name: 'b', kind: 'file', path: half })
    ]),
    /the multipart body is \d+ bytes, over the 25 MiB request limit/
  )
})

test('a type or field name with a line break is refused', async () => {
  const dir = mkdtempSync(join(tmpdir(), 'upload-'))
  const data = binaryFile(dir, 'x.bin')
  await assert.rejects(
    buildMultipart([
      part({ name: 'f', kind: 'file', path: data.path, contentType: 'text/plain\r\nX-Evil: 1' })
    ]),
    /invalid type/
  )
  await assert.rejects(buildMultipart([part({ name: 'a\r\nb', value: 'v' })]), /line break/)
})
