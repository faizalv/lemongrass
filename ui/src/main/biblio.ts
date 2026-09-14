import { ipcMain, WebContents } from 'electron'
import {
  existsSync,
  mkdirSync,
  readdirSync,
  readFileSync,
  realpathSync,
  statSync,
  watch,
  writeFileSync,
  type FSWatcher
} from 'fs'
import { dirname, join, relative, resolve, sep } from 'path'

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

// Guarantees a "scratchpad" entry always exists in the top-level listing,
// even before it's ever been created on disk -- otherwise the "add
// scratchpad" affordance would have no row to attach to for a biblio/ that
// hasn't grown one yet.
function ensureScratchpad(children: BiblioNode[]): BiblioNode[] {
  if (children.some((n) => n.type === 'dir' && n.name === 'scratchpad')) return children
  const scratchpad: BiblioNode = {
    name: 'scratchpad',
    path: 'scratchpad',
    type: 'dir',
    children: []
  }
  return [...children, scratchpad].sort((a, b) => a.name.localeCompare(b.name))
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

function slugify(title: string): string {
  const slug = title
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
  return slug || 'untitled'
}

// A tool call on the destroyed sender's WebContents (window closed mid-debounce) throws.
function send(sender: WebContents | undefined, channel: string, payload: unknown): void {
  if (sender && !sender.isDestroyed()) sender.send(channel, payload)
}

const watchDebounce = 400
const watchers = new Map<string, FSWatcher>()
const debounceTimers = new Map<string, ReturnType<typeof setTimeout>>()

// Starts a recursive watch on a project's biblio/ root the first time its
// tree is fetched, so a file added by anything other than this app's own
// IPC handlers (another lgrass pane, a direct filesystem write) still
// reaches the renderer instead of waiting for the next project switch.
function watchBiblio(
  biblioRoot: string,
  projectPath: string,
  getSender: () => WebContents | undefined
): void {
  if (watchers.has(projectPath)) return
  try {
    const watcher = watch(biblioRoot, { recursive: true }, () => {
      clearTimeout(debounceTimers.get(projectPath))
      debounceTimers.set(
        projectPath,
        setTimeout(() => send(getSender(), 'biblio:changed', projectPath), watchDebounce)
      )
    })
    watcher.on('error', () => {
      watcher.close()
      watchers.delete(projectPath)
    })
    watchers.set(projectPath, watcher)
  } catch {
    // Recursive fs.watch isn't available on every platform; the tree stays
    // on-demand-only for this project instead of failing the app.
  }
}

export function closeBiblioWatchers(): void {
  for (const watcher of watchers.values()) watcher.close()
  watchers.clear()
  for (const timer of debounceTimers.values()) clearTimeout(timer)
  debounceTimers.clear()
}

export function registerBiblioHandlers(getSender: () => WebContents | undefined): void {
  ipcMain.handle('biblio:tree', (_event, projectPath: string): BiblioTree | null => {
    const biblioRoot = join(projectPath, 'biblio')
    try {
      if (!statSync(biblioRoot).isDirectory()) return null
      const root = realpathSync(biblioRoot)
      watchBiblio(root, projectPath, getSender)
      const children = ensureScratchpad(walk(root, root))
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

  // Writes are scoped to scratchpad/ only -- that's the one tier bibliothek
  // itself calls user-editable; laws/handover/books aren't writable here.
  ipcMain.handle(
    'biblio:write',
    (_event, projectPath: string, relativePath: string, content: string): boolean => {
      if (!relativePath.startsWith(`scratchpad${sep}`) && relativePath !== 'scratchpad')
        return false
      const biblioRoot = join(projectPath, 'biblio')
      try {
        const root = realpathSync(biblioRoot)
        const target = resolveInBiblio(root, relativePath)
        if (!target || !target.endsWith('.md')) return false
        mkdirSync(dirname(target), { recursive: true })
        writeFileSync(target, content, 'utf-8')
        return true
      } catch {
        return false
      }
    }
  )

  ipcMain.handle(
    'biblio:createScratchpad',
    (_event, projectPath: string, title: string, content: string): string | null => {
      const biblioRoot = join(projectPath, 'biblio')
      try {
        mkdirSync(biblioRoot, { recursive: true })
        const root = realpathSync(biblioRoot)
        const scratchpadRoot = join(root, 'scratchpad')
        mkdirSync(scratchpadRoot, { recursive: true })

        const base = slugify(title)
        let slug = base
        let attempt = 2
        while (existsSync(join(scratchpadRoot, slug))) {
          slug = `${base}-${attempt}`
          attempt += 1
        }

        const taskDir = join(scratchpadRoot, slug)
        mkdirSync(taskDir, { recursive: true })
        writeFileSync(join(taskDir, 'notes.md'), content, 'utf-8')
        return `scratchpad${sep}${slug}${sep}notes.md`
      } catch {
        return null
      }
    }
  )

  // Writes a single file into an existing folder under scratchpad/.
  ipcMain.handle(
    'biblio:createScratchpadFile',
    (
      _event,
      projectPath: string,
      folderRelativePath: string,
      title: string,
      content: string
    ): string | null => {
      if (folderRelativePath !== 'scratchpad' && !folderRelativePath.startsWith(`scratchpad${sep}`))
        return null
      const biblioRoot = join(projectPath, 'biblio')
      try {
        const root = realpathSync(biblioRoot)
        const targetDir = resolveInBiblio(root, folderRelativePath)
        if (!targetDir || !statSync(targetDir).isDirectory()) return null

        const base = slugify(title)
        let filename = `${base}.md`
        let attempt = 2
        while (existsSync(join(targetDir, filename))) {
          filename = `${base}-${attempt}.md`
          attempt += 1
        }

        writeFileSync(join(targetDir, filename), content, 'utf-8')
        return `${folderRelativePath}${sep}${filename}`
      } catch {
        return null
      }
    }
  )
}
