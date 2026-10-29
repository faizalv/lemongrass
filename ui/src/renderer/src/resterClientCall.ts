// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import type {
  ResterClientCall,
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

// Turns a saved request into what the vault sends. A JSON body leaves the content type empty, so the
// proxy sets application/json; a form body is urlencoded.
export function buildCall(request: ResterClientRequest): ResterClientCall {
  let body = ''
  let contentType = ''
  if (request.bodyKind === 'json') {
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
    headers: request.headers
      .filter((header) => header.name.trim() !== '')
      .map((header) => ({ name: header.name.trim(), value: header.value }))
  }
}
