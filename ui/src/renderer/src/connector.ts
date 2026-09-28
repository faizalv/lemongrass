import { reactive } from 'vue'

const AUTO_LOCK_MS = 5 * 60 * 1000

export const connectorSession = reactive({
  unlocked: false,
  passphrase: '',
  shortIdByChannel: {} as Record<string, string>,
  shortIdByHTTPChannel: {} as Record<string, string>
})

const knownErrors: Record<string, string> = {
  'vault: wrong passphrase': 'Wrong passphrase.'
}

// Electron wraps a failed ipcRenderer.invoke as "Error invoking remote method '...': Error: <message>".
export function describeError(err: unknown): string {
  const raw = err instanceof Error ? err.message : String(err)
  const stripped = raw.replace(/^Error invoking remote method '[^']*': (Error: )?/, '')
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
  connectorSession.shortIdByChannel = {}
  connectorSession.shortIdByHTTPChannel = {}
}

export function lockConnector(): void {
  connectorSession.unlocked = false
  connectorSession.passphrase = ''
  clearIdleTimer()
}

export function unlockConnector(passphrase: string): void {
  connectorSession.passphrase = passphrase
  connectorSession.unlocked = true
  noteConnectorActivity()
}

export function noteConnectorActivity(): void {
  if (!connectorSession.unlocked) return
  clearIdleTimer()
  idleTimer = setTimeout(lockConnector, AUTO_LOCK_MS)
}
