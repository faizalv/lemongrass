// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

// Plain words for a Rester Client call that failed at the network level. The raw text comes from
// Go (restergate, for rester calls) or Node (plainRequest, for Plain calls), so each kind lists
// the wording of both; the text is matched rather than a code because history keeps only the text.

export interface ResterErrorText {
  title: string
  hint: string
}

interface Kind extends ResterErrorText {
  pattern: RegExp
}

const SERVER_SIDE = 'This came from the server or a proxy in front of it, not from lemongrass.'

// Checked in order. A timeout comes first, since a read that timed out is also wrapped as
// "reading response"; any other failure under that wrapping is a body cut off after the status
// arrived. Certificates come before the TLS handshake, whose wording a certificate error contains.
const KINDS: Kind[] = [
  {
    pattern:
      /did not answer within|deadline exceeded|Client\.Timeout|i\/o timeout|ETIMEDOUT|UND_ERR_.*TIMEOUT/i,
    title: 'The server took too long to answer.',
    hint: 'The server may be busy or the request too heavy. Send it again, or check whether the server is up.'
  },
  {
    pattern:
      /stream error|reading response|connection reset|ECONNRESET|unexpected EOF|other side closed|socket hang up|terminated/i,
    title: 'The server cut off its response partway through.',
    hint: `${SERVER_SIDE} Check the server's logs around this time, or send the request again.`
  },
  {
    pattern: /no such host|ENOTFOUND|EAI_AGAIN|server misbehaving/i,
    title: "The server's address could not be found.",
    hint: 'Check the host in the URL or the environment base URL. A private address may need a VPN.'
  },
  {
    pattern: /connection refused|ECONNREFUSED/i,
    title: 'Nothing is answering at that address.',
    hint: 'Check that the server is running and that the port in the URL is right.'
  },
  {
    pattern: /network is unreachable|no route to host|ENETUNREACH|EHOSTUNREACH/i,
    title: 'The network could not reach the server.',
    hint: 'Check your connection. A private address may need a VPN.'
  },
  {
    pattern: /x509|certificate|self[- ]signed|UNABLE_TO_VERIFY|CERT_|altnames/i,
    title: "The server's certificate is not trusted.",
    hint: 'It may be expired, self-signed or issued for another host. Check the address, or the certificate on the server.'
  },
  {
    pattern:
      /tls:|handshake|EPROTO|SSL routines|wrong version number|HTTP response to HTTPS client/i,
    title: 'The secure connection could not be set up.',
    hint: 'Check whether the server expects http or https at this address.'
  }
]

// The plain words for raw, or null when the failure is not one of the known kinds, which then
// shows as it is.
export function resterErrorText(raw: string): ResterErrorText | null {
  const kind = KINDS.find((k) => k.pattern.test(raw))
  return kind ? { title: kind.title, hint: kind.hint } : null
}
