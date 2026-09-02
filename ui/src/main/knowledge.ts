import { ipcMain } from 'electron'
import { existsSync, readdirSync, readFileSync } from 'fs'
import { join, resolve, sep } from 'path'

// Knowledge browsing is read-only, straight off each project's own
// context/ folder -- no centralizing, no writing.

export interface TreeNode {
  name: string
  path: string
  type: 'file' | 'dir'
  children?: TreeNode[]
}

function buildTree(dir: string): TreeNode[] {
  const entries = readdirSync(dir, { withFileTypes: true })
  return entries
    .filter((e) => !e.name.startsWith('.'))
    .map((e): TreeNode => {
      const fullPath = join(dir, e.name)
      return e.isDirectory()
        ? { name: e.name, path: fullPath, type: 'dir', children: buildTree(fullPath) }
        : { name: e.name, path: fullPath, type: 'file' }
    })
    .sort((a, b) => {
      if (a.type !== b.type) return a.type === 'dir' ? -1 : 1
      return a.name.localeCompare(b.name)
    })
}

export function registerKnowledgeHandlers(): void {
  ipcMain.handle('knowledge:tree', (_event, projectPath: string): TreeNode[] => {
    const contextDir = join(projectPath, 'context')
    if (!existsSync(contextDir)) return []
    return buildTree(contextDir)
  })

  ipcMain.handle(
    'knowledge:read',
    (_event, { projectPath, filePath }: { projectPath: string; filePath: string }): string => {
      const contextDir = resolve(join(projectPath, 'context'))
      const target = resolve(filePath)
      // Guard against path traversal -- only ever read inside this
      // project's own context/ directory.
      if (target !== contextDir && !target.startsWith(contextDir + sep)) {
        throw new Error('path outside project context/ directory')
      }
      return readFileSync(target, 'utf-8')
    }
  )
}
