import { ipcMain } from 'electron'
import { execFile } from 'child_process'
import type { WorkgroupInfo } from '../preload/types'

const LGRASS_TIMEOUT_MS = 5_000

function runLgrass(lgrassPath: string, projectPath: string, args: string[]): Promise<string> {
  return new Promise((resolve, reject) => {
    execFile(
      lgrassPath,
      ['workgroup', ...args],
      { cwd: projectPath, encoding: 'utf8', timeout: LGRASS_TIMEOUT_MS },
      (failure, stdout, stderr) =>
        failure ? reject(new Error(stderr.trim() || failure.message)) : resolve(stdout)
    )
  })
}

export function registerWorkgroupHandlers(lgrassPath: string | null): void {
  ipcMain.handle(
    'workgroups:list',
    async (_event, projectPath: string): Promise<WorkgroupInfo[]> => {
      if (!lgrassPath) return []
      try {
        return JSON.parse(await runLgrass(lgrassPath, projectPath, ['list', '--json']))
      } catch (err) {
        console.error('lgrass workgroup list failed:', err)
        return []
      }
    }
  )

  ipcMain.handle(
    'workgroups:disband',
    async (
      _event,
      { projectPath, groupId }: { projectPath: string; groupId: number }
    ): Promise<{ ok: boolean; error?: string }> => {
      if (!lgrassPath) return { ok: false, error: 'lgrass is not installed' }
      try {
        await runLgrass(lgrassPath, projectPath, ['disband', String(groupId)])
        return { ok: true }
      } catch (err) {
        return { ok: false, error: err instanceof Error ? err.message : String(err) }
      }
    }
  )
}
