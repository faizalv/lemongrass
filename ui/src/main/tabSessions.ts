import { ipcMain } from 'electron'
import { execFile } from 'child_process'

const LGRASS_TIMEOUT_MS = 5_000

function runLgrass(lgrassPath: string, projectPath: string, args: string[]): Promise<string> {
  return new Promise((resolve, reject) => {
    execFile(
      lgrassPath,
      ['session', 'tabs', ...args],
      { cwd: projectPath, encoding: 'utf8', timeout: LGRASS_TIMEOUT_MS },
      (failure, stdout) => (failure ? reject(failure) : resolve(stdout))
    )
  })
}

export function registerTabSessionHandlers(lgrassPath: string | null): void {
  ipcMain.handle(
    'tabSessions:list',
    async (_event, projectPath: string): Promise<Record<string, string>> => {
      if (!lgrassPath) return {}
      try {
        return JSON.parse(await runLgrass(lgrassPath, projectPath, ['list']))
      } catch (err) {
        console.error('lgrass session tabs list failed:', err)
        return {}
      }
    }
  )

  ipcMain.handle(
    'tabSessions:forget',
    async (_event, { projectPath, tabId }: { projectPath: string; tabId: string }) => {
      if (!lgrassPath) return
      try {
        await runLgrass(lgrassPath, projectPath, ['forget', tabId])
      } catch (err) {
        console.error('lgrass session tabs forget failed:', err)
      }
    }
  )
}
