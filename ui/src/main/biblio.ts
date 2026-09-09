import { ipcMain } from 'electron'
import { readdirSync, readFileSync, realpathSync, statSync } from 'fs'
import { join, relative, resolve, sep } from 'path'

export interface BiblioNode {
  name: string
  path: string
  type: 'dir' | 'file'
  children?: BiblioNode[]
}

export interface BiblioTree {
  // books/toc.md, pulled out of the normal books/ listing so it can be
  // shown in its own dedicated spot rather than buried inside a category.
  toc: BiblioNode | null
  children: BiblioNode[]
}

function extractToc(children: BiblioNode[]): BiblioNode | null {
  const books = children.find((n) => n.type === 'dir' && n.name === 'books')
  if (!books?.children) return null
  const index = books.children.findIndex((n) => n.type === 'file' && n.name === 'toc.md')
  if (index === -1) return null
  const [toc] = books.children.splice(index, 1)
  return toc
}

function walk(dir: string, relativeTo: string): BiblioNode[] {
  const entries = readdirSync(dir, { withFileTypes: true })
    .filter((e) => e.isDirectory() || e.name.endsWith('.md'))
    .sort((a, b) => {
      if (a.isDirectory() !== b.isDirectory()) return a.isDirectory() ? -1 : 1
      return a.name.localeCompare(b.name)
    })

  return entries.map((entry) => {
    const fullPath = join(dir, entry.name)
    const relPath = relative(relativeTo, fullPath)
    if (entry.isDirectory()) {
      return { name: entry.name, path: relPath, type: 'dir', children: walk(fullPath, relativeTo) }
    }
    return { name: entry.name, path: relPath, type: 'file' }
  })
}

// Resolves a biblio-relative path and checks it stays inside the biblio/
// root, so a crafted "../.." path from the renderer can't read outside it.
function resolveInBiblio(biblioRoot: string, relativePath: string): string | null {
  const resolved = resolve(biblioRoot, relativePath)
  if (resolved !== biblioRoot && !resolved.startsWith(biblioRoot + sep)) return null
  return resolved
}

export function registerBiblioHandlers(): void {
  ipcMain.handle('biblio:tree', (_event, projectPath: string): BiblioTree | null => {
    const biblioRoot = join(projectPath, 'biblio')
    try {
      if (!statSync(biblioRoot).isDirectory()) return null
      const root = realpathSync(biblioRoot)
      const children = walk(root, root)
      return { toc: extractToc(children), children }
    } catch {
      return null
    }
  })

  ipcMain.handle(
    'biblio:read',
    (_event, projectPath: string, relativePath: string): string | null => {
      const biblioRoot = join(projectPath, 'biblio')
      try {
        const root = realpathSync(biblioRoot)
        const target = resolveInBiblio(root, relativePath)
        if (!target || !target.endsWith('.md')) return null
        return readFileSync(target, 'utf-8')
      } catch {
        return null
      }
    }
  )
}
