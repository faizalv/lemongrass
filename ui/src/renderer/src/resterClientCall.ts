// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import type {
  ResterClientCall,
  ResterClientFileBody,
  ResterClientPart,
  ResterClientRequest,
  VaultRester,
  VaultResterUser
} from '../../preload/types'

// A user is a token user when it carries a token anywhere, mirroring vault.ResterUser.IsBYOT.
function isTokenUser(user: VaultResterUser): boolean {
  return !!user.Token || Object.values(user.Overrides ?? {}).some((override) => !!override.Token)
}

// The users the vault lets call environment's api side, the api half of vault.Rester.CanUse.
export function apiUsers(rester: VaultRester, environment: string): VaultResterUser[] {
  const env = rester.Environments.find((e) => e.Name === environment)
  if (!rester.API || !env?.BaseURL) return []
  return rester.Users.filter(
    (user) =>
      (user.Environments ?? []).includes(environment) &&
      (isTokenUser(user) || rester.API!.LoginEndpoint !== '')
  )
}

// Requests saved before multipart and file bodies existed load without those fields.
export function withBodyDefaults(request: ResterClientRequest): ResterClientRequest {
  request.multipart ??= []
  request.file ??= { path: '', size: 0, contentType: '' }
  return request
}

// Turns a saved request into what the vault sends. A JSON body leaves the content type empty, so the
// proxy sets application/json; a form body is urlencoded. A multipart or file body travels as its
// parts or path, because only main can read the files.
export function buildCall(request: ResterClientRequest): ResterClientCall {
  let body = ''
  let contentType = ''
  let multipart: ResterClientPart[] | undefined
  let file: ResterClientFileBody | undefined
  if (request.bodyKind === 'multipart') {
    const parts = (request.multipart ?? [])
      .filter((part) => part.name.trim() !== '')
      .map((part) => ({ ...part, name: part.name.trim() }))
    if (parts.length > 0) multipart = parts
  } else if (request.bodyKind === 'file' && request.file) {
    file = { ...request.file }
  } else if (request.bodyKind === 'json') {
    body = request.json.trim()
  } else if (request.bodyKind === 'form') {
    const params = new URLSearchParams()
    for (const field of request.form) {
      if (field.name.trim() !== '') params.append(field.name.trim(), field.value)
    }
    body = params.toString()
    contentType = 'application/x-www-form-urlencoded'
  }
  return {
    rester: request.rester,
    environment: request.environment,
    user: request.user,
    method: request.method,
    path: request.url.trim(),
    body,
    contentType: body ? contentType : '',
    ...(multipart ? { multipart } : {}),
    ...(file ? { file } : {}),
    headers: request.headers
      .filter((header) => header.name.trim() !== '')
      .map((header) => ({ name: header.name.trim(), value: header.value }))
  }
}

// The status capsule's color class in resterClient.css.
export function statusClass(status: number): string {
  if (status >= 200 && status < 300) return 'ok'
  if (status >= 300 && status < 400) return 'redirect'
  if (status >= 400 && status < 500) return 'client'
  return 'server'
}
