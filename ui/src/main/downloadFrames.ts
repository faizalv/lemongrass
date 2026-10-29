// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

// Decodes a vault download response, the format gatekeeper/download.go writes: one JSON line, then
// for a streamed body frames of a 4-byte big-endian length and that many bytes, ending with a zero
// length, or an abort marker followed by a 2-byte length and an error message.

const MAX_FRAME = 1 << 20
const ABORT_FRAME = 0xffffffff

export type DownloadEvent =
  | { kind: 'header'; line: unknown }
  | { kind: 'data'; data: Buffer }
  | { kind: 'end' }
  | { kind: 'abort'; message: string }

export class DownloadDecoder {
  private buf: Buffer = Buffer.alloc(0)
  private headerRead = false
  private left = 0
  private done = false

  // Returns the events the bytes so far complete. Throws on a malformed header or an oversized frame.
  push(chunk: Buffer): DownloadEvent[] {
    this.buf = this.buf.length ? Buffer.concat([this.buf, chunk]) : chunk
    const events: DownloadEvent[] = []
    while (!this.done) {
      if (!this.headerRead) {
        const newline = this.buf.indexOf(0x0a)
        if (newline < 0) break
        events.push({
          kind: 'header',
          line: JSON.parse(this.buf.subarray(0, newline).toString('utf-8'))
        })
        this.buf = this.buf.subarray(newline + 1)
        this.headerRead = true
        continue
      }
      if (this.left > 0) {
        if (this.buf.length === 0) break
        const n = Math.min(this.left, this.buf.length)
        events.push({ kind: 'data', data: this.buf.subarray(0, n) })
        this.buf = this.buf.subarray(n)
        this.left -= n
        continue
      }
      if (this.buf.length < 4) break
      const size = this.buf.readUInt32BE(0)
      if (size === 0) {
        events.push({ kind: 'end' })
        this.done = true
      } else if (size === ABORT_FRAME) {
        if (this.buf.length < 6) break
        const length = this.buf.readUInt16BE(4)
        if (this.buf.length < 6 + length) break
        events.push({ kind: 'abort', message: this.buf.subarray(6, 6 + length).toString('utf-8') })
        this.done = true
      } else if (size > MAX_FRAME) {
        throw new Error(
          `lgrass: download frame of ${size} bytes is over the ${MAX_FRAME} byte limit`
        )
      } else {
        this.left = size
        this.buf = this.buf.subarray(4)
      }
    }
    return events
  }
}
