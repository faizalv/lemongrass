// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

// Builds the Rester Client's file bodies. The renderer cannot read files and the vault only carries
// bytes, so main reads the picked files here, under the same rules as `lgrass rester --file` and
// `--body-file` in cmd/lgrass/resterbody.go.

import { randomBytes } from 'node:crypto'
import { readFile, stat } from 'node:fs/promises'
import { homedir } from 'node:os'
import { basename, extname, isAbsolute, join } from 'node:path'
import type { ResterClientFileBody, ResterClientPart } from '../preload/types'

// restergate.MaxRequestBodyBytes; the vault enforces it again as the authority.
export const MAX_REQUEST_BODY_BYTES = 25 * 1024 * 1024

const OCTET_STREAM = 'application/octet-stream'

// The CLI's table, since a part type must not depend on which side built the body.
const EXTENSION_TYPES: Record<string, string> = {
  '.xlsx': 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  '.xls': 'application/vnd.ms-excel',
  '.csv': 'text/csv',
  '.json': 'application/json',
  '.pdf': 'application/pdf',
  '.zip': 'application/zip',
  '.txt': 'text/plain',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg'
}

export interface BuiltBody {
  body: Buffer
  contentType: string
}

export function fileTypeFor(path: string): string {
  return EXTENSION_TYPES[extname(path).toLowerCase()] ?? OCTET_STREAM
}

// The shape restergate.CheckContentType accepts. A type is written into the body's part headers,
// so a line break would let it add headers of its own.
function checkType(type: string, what: string): string {
  const trimmed = type.trim()
  if (!/^[\w!#$&^.+-]+\/[\w!#$&^.+-]+(\s*;[^\r\n]*)?$/.test(trimmed)) {
    throw new Error(`lgrass: ${what} has an invalid type "${type}"`)
  }
  return trimmed
}

function resolvePath(path: string): string {
  const expanded = path.startsWith('~/') ? join(homedir(), path.slice(2)) : path
  if (!isAbsolute(expanded)) {
    throw new Error(`lgrass: ${path} is not an absolute path, choose the file again`)
  }
  return expanded
}

function limitError(what: string, size: number): Error {
  return new Error(
    `lgrass: ${what} is ${size} bytes, over the ${MAX_REQUEST_BODY_BYTES >> 20} MiB request limit`
  )
}

async function fileSize(path: string): Promise<number> {
  let info
  try {
    info = await stat(path)
  } catch (err) {
    throw new Error(`lgrass: cannot read ${path}: ${(err as Error).message}`)
  }
  if (!info.isFile()) throw new Error(`lgrass: ${path} is not a regular file`)
  return info.size
}

async function readChecked(path: string): Promise<Buffer> {
  try {
    return await readFile(path)
  } catch (err) {
    throw new Error(`lgrass: cannot read ${path}: ${(err as Error).message}`)
  }
}

// Go's mime/multipart quotes names the same way.
function quote(value: string): string {
  return value.replace(/\\/g, '\\\\').replace(/"/g, '\\"')
}

function checkName(name: string): string {
  if (/[\r\n]/.test(name)) throw new Error(`lgrass: the field name "${name}" has a line break`)
  return name
}

export async function buildFileBody(file: ResterClientFileBody): Promise<BuiltBody> {
  if (!file.path) throw new Error('lgrass: choose a file for the body')
  const path = resolvePath(file.path)
  const size = await fileSize(path)
  if (size > MAX_REQUEST_BODY_BYTES) throw limitError(path, size)
  const contentType = file.contentType.trim()
    ? checkType(file.contentType, 'the file body')
    : fileTypeFor(path)
  return { body: await readChecked(path), contentType }
}

// Text fields first, then files in row order, as the CLI writes them. Every file is checked and
// its size counted before any is read, so an over-limit body fails without reading anything.
export async function buildMultipart(
  parts: ResterClientPart[],
  boundary = randomBytes(16).toString('hex')
): Promise<BuiltBody> {
  const texts = parts.filter((part) => part.kind === 'text')
  const files = parts.filter((part) => part.kind === 'file')
  const chunks: Buffer[] = []
  const head = (lines: string[]): void => {
    chunks.push(Buffer.from(`--${boundary}\r\n${lines.join('\r\n')}\r\n\r\n`))
  }
  for (const part of texts) {
    head([`Content-Disposition: form-data; name="${quote(checkName(part.name))}"`])
    chunks.push(Buffer.from(part.value), Buffer.from('\r\n'))
  }
  const planned: { part: ResterClientPart; path: string; type: string }[] = []
  let total = chunks.reduce((sum, chunk) => sum + chunk.length, 0)
  for (const part of files) {
    if (!part.path) throw new Error(`lgrass: choose a file for the field ${part.name}`)
    const path = resolvePath(part.path)
    const type = part.contentType.trim()
      ? checkType(part.contentType, `the field ${part.name}`)
      : fileTypeFor(path)
    total += await fileSize(path)
    if (total > MAX_REQUEST_BODY_BYTES) throw limitError('the multipart body', total)
    planned.push({ part, path, type })
  }
  for (const { part, path, type } of planned) {
    head([
      `Content-Disposition: form-data; name="${quote(checkName(part.name))}"; filename="${quote(basename(path))}"`,
      `Content-Type: ${type}`
    ])
    chunks.push(await readChecked(path), Buffer.from('\r\n'))
  }
  chunks.push(Buffer.from(`--${boundary}--\r\n`))
  const body = Buffer.concat(chunks)
  if (body.length > MAX_REQUEST_BODY_BYTES) throw limitError('the multipart body', body.length)
  return { body, contentType: `multipart/form-data; boundary=${boundary}` }
}
