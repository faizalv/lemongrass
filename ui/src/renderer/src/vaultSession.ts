// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { reactive } from 'vue'

export const AUTO_LOCK_CHOICES = [1, 5, 15, 30]
const DEFAULT_AUTO_LOCK_MINUTES = 5

let autoLockMinutes = DEFAULT_AUTO_LOCK_MINUTES

function validMinutes(value: unknown): value is number {
  return typeof value === 'number' && AUTO_LOCK_CHOICES.includes(value)
}

export async function loadVaultAutoLock(): Promise<number> {
  const saved = (await window.api.uiState.load()).vaultAutoLockMinutes
  autoLockMinutes = validMinutes(saved) ? saved : DEFAULT_AUTO_LOCK_MINUTES
  return autoLockMinutes
}

// Saves the choice and re-arms the idle timer of an unlocked vault with it.
export function setVaultAutoLock(minutes: number): void {
  if (!validMinutes(minutes)) return
  autoLockMinutes = minutes
  window.api.uiState.save({ vaultAutoLockMinutes: minutes })
  noteVaultActivity()
}

export const vaultSession = reactive({
  unlocked: false,
  passphrase: '',
  shortIdByChannel: {} as Record<string, string>,
  shortIdByHTTPChannel: {} as Record<string, string>,
  // Bumped after a rester is created, edited or deleted, so views that keep a copy of the resters,
  // such as the keep-alive Rester Client, reload it at once.
  resterVersion: 0
})

export function noteResterChange(): void {
  vaultSession.resterVersion++
}

const knownErrors: Record<string, string> = {
  'vault: wrong passphrase': 'Wrong passphrase.'
}

// Electron wraps a failed ipcRenderer.invoke as "Error invoking remote method '...': Error: <message>".
export function describeError(err: unknown): string {
  const raw = err instanceof Error ? err.message : String(err)
  const stripped = raw.replace(/^Error invoking remote method '[^']*': (Error: )?/, '')
  const lockout = stripped.match(
    /^vault: too many failed attempts, locked out(?:, try again in (.+))?$/
  )
  if (lockout) {
    return `Too many wrong passphrase attempts. Try again in ${lockout[1] ?? 'a few minutes'}.`
  }
  return knownErrors[stripped] ?? stripped
}

let idleTimer: ReturnType<typeof setTimeout> | null = null

function clearIdleTimer(): void {
  if (idleTimer) {
    clearTimeout(idleTimer)
    idleTimer = null
  }
}

export function clearShortIds(): void {
  vaultSession.shortIdByChannel = {}
  vaultSession.shortIdByHTTPChannel = {}
}

export function lockVaultSession(): void {
  vaultSession.unlocked = false
  vaultSession.passphrase = ''
  clearIdleTimer()
}

// Every unlock also moves a policy sealed by an earlier version into the readable policy file, which is a no-op once done.
export function unlockVaultSession(passphrase: string): void {
  vaultSession.passphrase = passphrase
  vaultSession.unlocked = true
  noteVaultActivity()
  // The saved choice arrives after the first timer is armed, so the timer is armed again with it.
  void loadVaultAutoLock().then(noteVaultActivity)
  window.api.vault.migratePolicy(passphrase).catch(() => undefined)
}

export function noteVaultActivity(): void {
  if (!vaultSession.unlocked) return
  clearIdleTimer()
  idleTimer = setTimeout(
    () => {
      lockVaultSession()
      void window.api.notifications.add({ event: 'vault-locked', key: 'vault-locked' })
    },
    autoLockMinutes * 60 * 1000
  )
}
