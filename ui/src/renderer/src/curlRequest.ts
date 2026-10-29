// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import type { ResterClientBodyKind, ResterClientPair } from '../../preload/types'

export interface ParsedCurl {
  method: string
  url: string
  headers: ResterClientPair[]
  bodyKind: ResterClientBodyKind
  json: string
  form: ResterClientPair[]
  // Credential headers and options removed because the chosen rester user supplies auth.
  dropped: string[]
  notes: string[]
}

// The rester user supplies auth, so a pasted credential is never kept in a saved request.
const CREDENTIAL_HEADERS = ['authorization', 'proxy-authorization', 'cookie']

// The proxy sets these itself and refuses them as call headers.
const PROXY_HEADERS = ['host', 'content-length', 'transfer-encoding', 'connection']

const SHORT_WITH_VALUE = new Set([
  'X',
  'H',
  'd',
  'F',
  'u',
  'b',
  'A',
  'e',
  'o',
  'm',
  'x',
  'w',
  'T',
  'E',
  'c',
  'r',
  'y',
  'Y',
  'z',
  'K'
])

const LONG_WITH_VALUE = new Set([
  'request',
  'header',
  'data',
  'data-raw',
  'data-binary',
  'data-ascii',
  'data-urlencode',
  'json',
  'form',
  'form-string',
  'user',
  'cookie',
  'user-agent',
  'referer',
  'url',
  'output',
  'max-time',
  'connect-timeout',
  'proxy',
  'write-out',
  'cacert',
  'cert',
  'key',
  'retry',
  'resolve',
  'cookie-jar',
  'upload-file',
  'oauth2-bearer'
])

// Splits a shell command line the way bash would for the quoting curl exports use: single quotes,
// double quotes with backslash escapes, ANSI-C $'...' quotes and backslash line continuations.
export function shellWords(input: string): string[] {
  const words: string[] = []
  let word = ''
  let inWord = false
  let i = 0
  while (i < input.length) {
    const c = input[i]
    if (c === '\\' && (input[i + 1] === '\n' || input.startsWith('\r\n', i + 1))) {
      i += input[i + 1] === '\n' ? 2 : 3
      continue
    }
    if (/\s/.test(c)) {
      if (inWord) words.push(word)
      word = ''
      inWord = false
      i++
      continue
    }
    inWord = true
    if (c === "'") {
      const end = input.indexOf("'", i + 1)
      const stop = end < 0 ? input.length : end
      word += input.slice(i + 1, stop)
      i = stop + 1
    } else if (c === '$' && input[i + 1] === "'") {
      i += 2
      while (i < input.length && input[i] !== "'") {
        if (input[i] === '\\' && i + 1 < input.length) {
          const [text, used] = ansiEscape(input, i + 1)
          word += text
          i += 1 + used
        } else {
          word += input[i++]
        }
      }
      i++
    } else if (c === '"') {
      i++
      while (i < input.length && input[i] !== '"') {
        if (input[i] === '\\' && '"\\$`\n'.includes(input[i + 1] ?? '')) {
          if (input[i + 1] !== '\n') word += input[i + 1]
          i += 2
        } else {
          word += input[i++]
        }
      }
      i++
    } else if (c === '\\') {
      word += input[i + 1] ?? ''
      i += 2
    } else {
      word += c
      i++
    }
  }
  if (inWord) words.push(word)
  return words
}

function ansiEscape(input: string, at: number): [string, number] {
  const c = input[at]
  const simple: Record<string, string> = {
    n: '\n',
    t: '\t',
    r: '\r',
    '\\': '\\',
    "'": "'",
    '"': '"',
    '0': '\0'
  }
  if (c in simple) return [simple[c], 1]
  if (c === 'x') {
    const hex = input.slice(at + 1, at + 3).match(/^[0-9a-fA-F]{1,2}/)?.[0] ?? ''
    return hex ? [String.fromCharCode(parseInt(hex, 16)), 1 + hex.length] : ['\\x', 1]
  }
  if (c === 'u') {
    const hex = input.slice(at + 1, at + 5).match(/^[0-9a-fA-F]{1,4}/)?.[0] ?? ''
    return hex ? [String.fromCharCode(parseInt(hex, 16)), 1 + hex.length] : ['\\u', 1]
  }
  return ['\\' + c, 1]
}

export function looksLikeCurl(text: string): boolean {
  return /^\s*curl(\.exe)?\s/.test(text)
}

// Parses a curl command into a request. tokenHeader is the rester's own token header name, dropped
// like the standard credential headers.
export function parseCurl(command: string, tokenHeader = ''): ParsedCurl {
  const words = shellWords(command.trim())
  const result: ParsedCurl = {
    method: '',
    url: '',
    headers: [],
    bodyKind: 'none',
    json: '',
    form: [],
    dropped: [],
    notes: []
  }
  const data: string[] = []
  let jsonBody: string | null = null
  let getMode = false
  const credentialNames = new Set(
    [...CREDENTIAL_HEADERS, tokenHeader.toLowerCase()].filter(Boolean)
  )

  const addHeader = (raw: string): void => {
    const colon = raw.indexOf(':')
    if (colon <= 0) return
    const name = raw.slice(0, colon).trim()
    const value = raw.slice(colon + 1).trim()
    const lower = name.toLowerCase()
    if (credentialNames.has(lower)) {
      result.dropped.push(name)
      return
    }
    if (PROXY_HEADERS.includes(lower)) return
    result.headers.push({ name, value })
  }

  for (let i = 1; i < words.length; i++) {
    const word = words[i]
    let option = ''
    let value: string | undefined
    if (word.startsWith('--')) {
      const eq = word.indexOf('=')
      option = eq > 0 ? word.slice(2, eq) : word.slice(2)
      if (eq > 0) value = word.slice(eq + 1)
      else if (LONG_WITH_VALUE.has(option)) value = words[++i] ?? ''
    } else if (word.startsWith('-') && word.length > 1) {
      const flag = word[1]
      if (SHORT_WITH_VALUE.has(flag)) {
        option = flag
        value = word.length > 2 ? word.slice(2) : (words[++i] ?? '')
      } else {
        // A cluster of value-less short flags such as -sSL; only -G and -I change the request.
        if (word.includes('G')) getMode = true
        if (word.includes('I')) result.method = 'HEAD'
        continue
      }
    } else {
      if (!result.url) result.url = word
      continue
    }

    switch (option) {
      case 'X':
      case 'request':
        result.method = (value ?? '').toUpperCase()
        break
      case 'H':
      case 'header':
        addHeader(value ?? '')
        break
      case 'd':
      case 'data':
      case 'data-raw':
      case 'data-binary':
      case 'data-ascii':
        if (value?.startsWith('@') && option !== 'data-raw') {
          result.notes.push(`The body file ${value.slice(1)} was not read.`)
        } else {
          data.push(value ?? '')
        }
        break
      case 'data-urlencode':
        data.push(urlencodeData(value ?? ''))
        break
      case 'json':
        jsonBody = (jsonBody ?? '') + (value ?? '')
        break
      case 'F':
      case 'form':
      case 'form-string': {
        const eq = (value ?? '').indexOf('=')
        const name = eq > 0 ? value!.slice(0, eq) : (value ?? '')
        const fieldValue = eq > 0 ? value!.slice(eq + 1) : ''
        if (fieldValue.startsWith('@') || fieldValue.startsWith('<')) {
          result.notes.push(`The file field ${name} was left out.`)
        } else {
          result.form.push({ name, value: fieldValue })
        }
        break
      }
      case 'u':
      case 'user':
        result.dropped.push('--user')
        break
      case 'b':
      case 'cookie':
        result.dropped.push('--cookie')
        break
      case 'oauth2-bearer':
        result.dropped.push('--oauth2-bearer')
        break
      case 'A':
      case 'user-agent':
        result.headers.push({ name: 'User-Agent', value: value ?? '' })
        break
      case 'e':
      case 'referer':
        result.headers.push({ name: 'Referer', value: value ?? '' })
        break
      case 'url':
        result.url = value ?? ''
        break
      case 'get':
        getMode = true
        break
      case 'head':
        result.method = 'HEAD'
        break
    }
  }

  const contentType = takeContentType(result.headers)
  if (getMode && data.length > 0) {
    const query = data.join('&')
    result.url += (result.url.includes('?') ? '&' : '?') + query
  } else if (jsonBody !== null) {
    result.bodyKind = 'json'
    result.json = jsonBody
  } else if (data.length > 0) {
    const raw = data.join('&')
    if (/json/i.test(contentType) || isJsonText(raw)) {
      result.bodyKind = 'json'
      result.json = raw
    } else {
      result.bodyKind = 'form'
      result.form = formPairs(raw)
    }
  } else if (result.form.length > 0) {
    result.bodyKind = 'form'
    result.notes.push('The multipart form is sent as a urlencoded form.')
  }
  if (result.bodyKind === 'json' && isJsonText(result.json)) {
    result.json = JSON.stringify(JSON.parse(result.json), null, 2)
  }

  if (!result.method) result.method = result.bodyKind === 'none' ? 'GET' : 'POST'
  return result
}

// Removes and returns the Content-Type header, since the body kind decides it when the request is sent.
function takeContentType(headers: ResterClientPair[]): string {
  const index = headers.findIndex((header) => header.name.toLowerCase() === 'content-type')
  if (index < 0) return ''
  const [header] = headers.splice(index, 1)
  return header.value
}

function isJsonText(text: string): boolean {
  const trimmed = text.trim()
  if (!trimmed.startsWith('{') && !trimmed.startsWith('[')) return false
  try {
    JSON.parse(trimmed)
    return true
  } catch {
    return false
  }
}

function urlencodeData(value: string): string {
  const eq = value.indexOf('=')
  if (eq < 0) return encodeURIComponent(value)
  return value.slice(0, eq + 1) + encodeURIComponent(value.slice(eq + 1))
}

function formPairs(raw: string): ResterClientPair[] {
  return raw
    .split('&')
    .filter((part) => part !== '')
    .map((part) => {
      const eq = part.indexOf('=')
      const decode = (s: string): string => {
        try {
          return decodeURIComponent(s.replace(/\+/g, ' '))
        } catch {
          return s
        }
      }
      return eq < 0
        ? { name: decode(part), value: '' }
        : { name: decode(part.slice(0, eq)), value: decode(part.slice(eq + 1)) }
    })
}

export interface BaseURLTarget {
  rester: string
  environment: string
  baseURL: string
}

// Picks the environment whose base URL is the longest prefix of url, on a path segment boundary,
// and returns it with url made relative to it. A url under no base URL returns null.
export function matchBaseURL(
  url: string,
  targets: BaseURLTarget[]
): { target: BaseURLTarget; path: string } | null {
  let best: { target: BaseURLTarget; path: string } | null = null
  for (const target of targets) {
    const base = target.baseURL.replace(/\/+$/, '')
    if (!base || !url.toLowerCase().startsWith(base.toLowerCase())) continue
    const rest = url.slice(base.length)
    if (rest !== '' && !'/?#'.includes(rest[0])) continue
    if (best && best.target.baseURL.length >= target.baseURL.length) continue
    best = { target, path: rest.startsWith('/') ? rest : '/' + rest }
  }
  return best
}
