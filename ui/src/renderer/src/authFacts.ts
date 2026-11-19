// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import type {
  VaultAppStep,
  VaultLoginStep,
  VaultResterAPI,
  VaultResterApp
} from '../../preload/types'

// A fact an authentication card shows. code marks a value typed or sent as written, shown as a
// code value that copies on click; prefix is plain text before it.
export interface AuthFact {
  label: string
  prefix?: string
  value: string
  code: boolean
}

// One step of a login, as a row: what it does, what it acts on and, for a fill, the login field it
// types.
export type AuthStepKind = 'goto' | 'fill' | 'click' | 'wait' | 'launch' | 'tap' | 'key'

export interface AuthStep {
  kind: AuthStepKind
  label: string
  target: string
  field?: string
}

// The stored API side as facts a person looks up, in the order they come up when calling it:
// where the login is, where the token is in the answer, how long it lasts, and where it goes.
export function apiFacts(api: VaultResterAPI): AuthFact[] {
  const facts: AuthFact[] = [
    api.LoginEndpoint
      ? { label: 'Login', value: api.LoginEndpoint, code: true }
      : { label: 'Login', value: 'None, every user brings a token', code: false }
  ]
  if (api.LoginEndpoint) {
    facts.push(
      api.TokenPath
        ? { label: 'Token path', value: api.TokenPath, code: true }
        : { label: 'Token path', value: 'Not set', code: false }
    )
    facts.push(tokenLifetime(api))
  }
  facts.push(tokenPlacement(api))
  const headers = (api.Headers ?? []).length
  if (headers > 0) facts.push({ label: 'Headers', value: `${headers} shared`, code: false })
  return facts
}

function tokenLifetime(api: VaultResterAPI): AuthFact {
  const label = 'Lifetime'
  if (api.TTLOrigin === 'jwt-exp') {
    return { label, value: 'Read from the token (JWT exp)', code: false }
  }
  if (api.TTLOrigin !== '') {
    return { label, prefix: 'From response field', value: api.TTLOrigin, code: true }
  }
  return { label, value: `Fixed, ${api.FixedTTLSeconds / 60} min`, code: false }
}

function tokenPlacement(api: VaultResterAPI): AuthFact {
  const label = 'Token goes in'
  const { Kind, Name, Prefix } = api.TokenPlacement
  if (Kind === 'cookie') return { label, prefix: 'Cookie', value: Name, code: true }
  if (Kind === 'query') return { label, prefix: 'Query parameter', value: Name, code: true }
  return { label, prefix: 'Header', value: `${Name}: ${Prefix}<token>`, code: true }
}

// The browser login steps in the order the browser runs them.
export function loginSteps(steps: VaultLoginStep[] | null): AuthStep[] {
  return (steps ?? []).map((s): AuthStep => {
    if (s.Action === 'goto') return { kind: 'goto', label: 'Go to', target: s.URL || '/' }
    if (s.Action === 'fill') {
      return { kind: 'fill', label: 'Fill', target: s.Selector, field: s.Field }
    }
    if (s.Action === 'click') return { kind: 'click', label: 'Click', target: s.Selector }
    return { kind: 'wait', label: 'Wait', target: s.Selector || s.URL }
  })
}

export function appFacts(app: VaultResterApp): AuthFact[] {
  return [
    app.Package
      ? { label: 'Package', value: app.Package, code: true }
      : { label: 'Package', value: 'Not set', code: false }
  ]
}

// The phone login steps in the order the phone runs them.
export function appSteps(steps: VaultAppStep[] | null): AuthStep[] {
  return (steps ?? []).map((s): AuthStep => {
    const target = s.Text || s.ID
    if (s.Action === 'launch') return { kind: 'launch', label: 'Launch', target: '' }
    if (s.Action === 'tap') return { kind: 'tap', label: 'Tap', target }
    if (s.Action === 'fill') return { kind: 'fill', label: 'Fill', target, field: s.Field }
    if (s.Action === 'key') return { kind: 'key', label: 'Press', target: s.Key }
    return { kind: 'wait', label: 'Wait', target }
  })
}
