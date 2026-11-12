// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

// Sends a Plain request from the Rester Client: a full URL with no rester and no user, so no
// credential is involved and the vault is not asked. The answer takes the shape of a vault human
// call (restergate.HumanRequest and HumanDownload), so the response box and the history treat
// both kinds alike. Like gitMove.ts, this file has no value imports between app modules, so
// node --test can load it straight from source; the caller builds the body with callBody.

import { createWriteStream, unlinkSync } from 'node:fs'
import { Readable } from 'node:stream'
import { pipeline } from 'node:stream/promises'
import type { ResterClientCall, ResterClientDownload, ResterClientResult } from '../preload/types'
import type { BuiltBody } from './resterUpload'

// restergate.RequestTimeout: the whole inline call, or the wait for headers and then each silence
// between body reads of a download.
export const PLAIN_TIMEOUT_MS = 120_000
// restergate.maxLoginResponseBytes, the inline cap of a human call.
export const PLAIN_INLINE_BYTES = 1 << 20

export function plainURL(path: string): URL {
  let url: URL
  try {
    url = new URL(path.trim())
  } catch {
    throw new Error('lgrass: a Plain request needs a full URL, such as https://example.com/path')
  }
  if (url.protocol !== 'http:' && url.protocol !== 'https:') {
    throw new Error('lgrass: a Plain request takes an http or https URL')
  }
  return url
}

// The headers go out as written. A built body type (form, multipart, file) wins over a written
// Content-Type, since a multipart boundary must match the body; a JSON body defaults to
// application/json like the vault's proxy does.
function requestInit(
  call: ResterClientCall,
  { body, contentType }: BuiltBody,
  signal: AbortSignal
): RequestInit {
  const headers = new Headers()
  const written = call.headers.some((h) => h.name.toLowerCase() === 'content-type')
  if (contentType) headers.set('Content-Type', contentType)
  else if (body.length > 0 && !written) headers.set('Content-Type', 'application/json')
  for (const header of call.headers) {
    if (contentType && header.name.toLowerCase() === 'content-type') continue
    headers.append(header.name, header.value)
  }
  return {
    method: call.method,
    headers,
    body: body.length > 0 ? new Uint8Array(body) : undefined,
    redirect: 'follow',
    signal
  }
}

function describeFailure(err: unknown, url: URL): Error {
  const error = err as Error & { cause?: Error }
  if (error.name === 'TimeoutError' || error.name === 'AbortError') {
    return new Error(`lgrass: ${url.host} did not answer within ${PLAIN_TIMEOUT_MS / 1000} s`)
  }
  if (error.message?.startsWith('lgrass: ')) return error
  return new Error(`lgrass: calling ${url.host}: ${error.cause?.message ?? error.message}`)
}

// Header names in Go's canonical form, as a vault call reports them, so a run reads the same
// whichever side sent it.
function canonicalName(name: string): string {
  return name
    .split('-')
    .map((word) => (word ? word[0].toUpperCase() + word.slice(1).toLowerCase() : word))
    .join('-')
}

export function headerMap(headers: Headers): Record<string, string[]> {
  const out: Record<string, string[]> = {}
  for (const [name, value] of headers) {
    if (name === 'set-cookie') continue
    out[canonicalName(name)] = [value]
  }
  const cookies = headers.getSetCookie()
  if (cookies.length > 0) out['Set-Cookie'] = cookies
  return out
}

export async function readLimited(
  stream: ReadableStream<Uint8Array> | null,
  limit: number
): Promise<{ bytes: Buffer; truncated: boolean }> {
  if (!stream) return { bytes: Buffer.alloc(0), truncated: false }
  const reader = stream.getReader()
  const chunks: Uint8Array[] = []
  let total = 0
  for (;;) {
    const { done, value } = await reader.read()
    if (done) break
    chunks.push(value)
    total += value.length
    if (total > limit) {
      await reader.cancel()
      break
    }
  }
  const all = Buffer.concat(chunks)
  return all.length > limit
    ? { bytes: all.subarray(0, limit), truncated: true }
    : { bytes: all, truncated: false }
}

// A cut can split a multi-byte character, so only up to three tail bytes are forgiven, and only
// when the body was cut. ignoreBOM keeps a byte order mark, as the vault does.
export function decodeText(bytes: Buffer, truncated: boolean): { body: string; binary: boolean } {
  const decoder = new TextDecoder('utf-8', { fatal: true, ignoreBOM: true })
  let text = bytes
  for (let dropped = 0; ; dropped++) {
    try {
      return { body: decoder.decode(text), binary: false }
    } catch {
      if (!truncated || dropped >= 3 || text.length === 0) return { body: '', binary: true }
      text = text.subarray(0, text.length - 1)
    }
  }
}

async function inlineResult(
  response: Response,
  start: number,
  requested: URL
): Promise<ResterClientResult> {
  const { bytes, truncated } = await readLimited(response.body, PLAIN_INLINE_BYTES)
  return {
    status: response.status,
    headers: headerMap(response.headers),
    ...decodeText(bytes, truncated),
    size: bytes.length,
    truncated,
    durationMs: Date.now() - start,
    url: response.url || requested.href
  }
}

export async function plainRequest(
  call: ResterClientCall,
  body: BuiltBody
): Promise<ResterClientResult> {
  const url = plainURL(call.path)
  const start = Date.now()
  try {
    const init = requestInit(call, body, AbortSignal.timeout(PLAIN_TIMEOUT_MS))
    return await inlineResult(await fetch(url, init), start, url)
  } catch (err) {
    throw describeFailure(err, url)
  }
}

// restergate.baseFileName: a server-supplied name reduced to a plain file name, or '' when nothing
// safe is left.
function baseFileName(name: string): string {
  const base = name.replaceAll('\\', '/').split('/').filter(Boolean).pop() ?? ''
  // eslint-disable-next-line no-control-regex
  const clean = base.replace(/[\u0000-\u001f\u007f]/g, '').trim()
  return clean === '.' || clean === '..' ? '' : clean
}

// restergate.downloadName: Content-Disposition's file name, else the last segment of the final
// URL path.
export function downloadName(disposition: string | null, finalURL: string): string {
  const extended = disposition?.match(/filename\*\s*=\s*(?:UTF-8|ISO-8859-1)?'[^']*'([^;]+)/i)
  const quoted = disposition?.match(/filename\s*=\s*"((?:[^"\\]|\\.)*)"/i)
  const bare = disposition?.match(/filename\s*=\s*([^;"\s]+)/i)
  let fromHeader = ''
  try {
    if (extended) fromHeader = decodeURIComponent(extended[1].trim())
  } catch {
    // A malformed escape falls back to the plain parameter.
  }
  fromHeader ||= quoted ? quoted[1].replace(/\\(.)/g, '$1') : (bare?.[1] ?? '')
  const name = baseFileName(fromHeader)
  if (name) return name
  try {
    return baseFileName(decodeURIComponent(new URL(finalURL).pathname))
  } catch {
    return ''
  }
}

// Streams a 2xx body to the file chooseFile returns once the response has started, so it can offer
// the server's file name; null cancels. Any other status is read inline. The timeout is paused
// while the file is being chosen, since that wait is the human's.
export async function plainDownload(
  call: ResterClientCall,
  body: BuiltBody,
  chooseFile: (name: string) => Promise<string | null>
): Promise<ResterClientDownload> {
  const url = plainURL(call.path)
  const start = Date.now()
  const controller = new AbortController()
  let timer: ReturnType<typeof setTimeout> | undefined
  const arm = (): void => {
    clearTimeout(timer)
    timer = setTimeout(
      () => controller.abort(new DOMException('timed out', 'TimeoutError')),
      PLAIN_TIMEOUT_MS
    )
  }
  let filePath = ''
  try {
    const init = requestInit(call, body, controller.signal)
    arm()
    const response = await fetch(url, init)
    if (response.status < 200 || response.status >= 300) {
      return { kind: 'inline', result: await inlineResult(response, start, url) }
    }
    clearTimeout(timer)
    const picked = await chooseFile(
      downloadName(response.headers.get('content-disposition'), response.url || url.href)
    )
    if (!picked) {
      controller.abort()
      return { kind: 'cancelled' }
    }
    filePath = picked
    arm()
    let bytes = 0
    const reader = response.body?.getReader()
    async function* chunks(): AsyncGenerator<Uint8Array> {
      if (!reader) return
      for (;;) {
        const { done, value } = await reader.read()
        if (done) return
        bytes += value.length
        arm()
        yield value
      }
    }
    await pipeline(Readable.from(chunks()), createWriteStream(filePath))
    return {
      kind: 'saved',
      path: filePath,
      bytes,
      status: response.status,
      durationMs: Date.now() - start
    }
  } catch (err) {
    if (filePath) {
      try {
        unlinkSync(filePath)
      } catch {
        // Nothing was written yet.
      }
    }
    throw describeFailure(err, url)
  } finally {
    clearTimeout(timer)
  }
}
