// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { reactive } from 'vue'
import type { EditorState, Text } from '@codemirror/state'
import type { SSHReadResult, SSHWriteResult } from '../../../../../preload/types'
import { serverRequest } from '../files/filesStore'

// The files a server tab has open in its editor live here, keyed by the server tab, not in the
// component: moving the tab to another pane remounts its body, and unsaved edits with their undo
// history must survive that. They end with the tab, in forgetServerEditor. The live EditorState
// and the saved text stay out of Vue's reactivity, since a proxy around a CodeMirror object
// breaks its identity checks and costs a walk of a 2 MB document.

export type RemoteFileStatus =
  'loading' | 'ready' | 'binary' | 'tooLarge' | 'isDir' | 'notLoggedIn' | 'error'

export interface RemoteFile {
  path: string
  status: RemoteFileStatus
  error: string
  // The text last read, handed to the editor; revision rises on every read so the editor
  // replaces its document even when the text is the same.
  content: string
  revision: number
  mtime: number
  dirty: boolean
  // The server's copy moved since the read; the banner offers Reload and Overwrite.
  changed: boolean
  saving: boolean
  // A save that failed for another reason, shown in the banner.
  notice: string
}

export interface ServerEditor {
  alias: string
  files: RemoteFile[]
  active: string | null
  // Folded into a bookmark on the Files layer, the terminal in front; files stay open as they were.
  minimized: boolean
}

interface Buffer {
  saved: Text | null
  state: EditorState | null
}

const editors = reactive<Record<string, ServerEditor>>({})
const buffers = new Map<string, Buffer>()

// The helper prefixes its errors with its package name, which says nothing to the human.
function plain(error: string): string {
  return error.replace(/^sshsession: /, '')
}

function bufferKey(serverTabId: string, path: string): string {
  return `${serverTabId}\n${path}`
}

export function serverEditorOf(serverTabId: string, alias: string): ServerEditor {
  if (!editors[serverTabId]) {
    editors[serverTabId] = { alias, files: [], active: null, minimized: false }
  }
  return editors[serverTabId]
}

function fileOf(serverTabId: string, path: string): RemoteFile | undefined {
  return editors[serverTabId]?.files.find((f) => f.path === path)
}

function bufferOf(serverTabId: string, path: string): Buffer {
  const key = bufferKey(serverTabId, path)
  let buffer = buffers.get(key)
  if (!buffer) {
    buffer = { saved: null, state: null }
    buffers.set(key, buffer)
  }
  return buffer
}

export function serverHasDirtyFiles(serverTabId: string): boolean {
  return editors[serverTabId]?.files.some((f) => f.dirty) ?? false
}

export function editorStateOf(serverTabId: string, path: string): EditorState | undefined {
  return buffers.get(bufferKey(serverTabId, path))?.state ?? undefined
}

// Shows the file in the server tab's editor, reading it the first time it opens.
export function openServerFile(serverTabId: string, alias: string, path: string): void {
  const editor = serverEditorOf(serverTabId, alias)
  editor.active = path
  editor.minimized = false
  if (fileOf(serverTabId, path)) return
  editor.files.push({
    path,
    status: 'loading',
    error: '',
    content: '',
    revision: 0,
    mtime: 0,
    dirty: false,
    changed: false,
    saving: false,
    notice: ''
  })
  void readServerFile(serverTabId, path)
}

export function showServerFile(serverTabId: string, path: string): void {
  const editor = editors[serverTabId]
  if (editor && fileOf(serverTabId, path)) editor.active = path
}

export function minimizeEditor(serverTabId: string): void {
  const editor = editors[serverTabId]
  if (editor?.files.length) editor.minimized = true
}

export function expandEditor(serverTabId: string): void {
  const editor = editors[serverTabId]
  if (editor) editor.minimized = false
}

// Closing the last file removes the editor and its bookmark; closing the shown one shows its
// neighbour.
export function closeServerFile(serverTabId: string, path: string): void {
  const editor = editors[serverTabId]
  if (!editor) return
  const at = editor.files.findIndex((f) => f.path === path)
  if (at < 0) return
  editor.files.splice(at, 1)
  buffers.delete(bufferKey(serverTabId, path))
  if (!editor.files.length) {
    editor.active = null
    editor.minimized = false
  } else if (editor.active === path) {
    editor.active = editor.files[Math.min(at, editor.files.length - 1)].path
  }
}

// Reads the file, replacing whatever the editor holds.
export async function readServerFile(serverTabId: string, path: string): Promise<void> {
  const editor = editors[serverTabId]
  const file = fileOf(serverTabId, path)
  if (!editor || !file) return
  file.status = 'loading'
  const reply = await serverRequest<SSHReadResult>(serverTabId, editor.alias, { op: 'read', path })
  if (!reply.ok) {
    const code = reply.code
    file.status =
      code === 'binary' || code === 'tooLarge' || code === 'isDir' || code === 'notLoggedIn'
        ? code
        : 'error'
    file.error = plain(reply.error)
    return
  }
  const buffer = bufferOf(serverTabId, path)
  buffer.saved = null
  buffer.state = null
  Object.assign(file, {
    status: 'ready',
    error: '',
    content: reply.result.content,
    revision: file.revision + 1,
    mtime: reply.result.mtime,
    dirty: false,
    changed: false,
    notice: ''
  })
}

// The editor reports every state; the first one after a read is the saved text.
export function editorChanged(serverTabId: string, path: string, state: EditorState): void {
  const file = fileOf(serverTabId, path)
  if (!file) return
  const buffer = bufferOf(serverTabId, path)
  buffer.state = state
  if (!buffer.saved) buffer.saved = state.doc
  file.dirty = !state.doc.eq(buffer.saved)
}

export async function saveServerFile(
  serverTabId: string,
  path: string,
  force = false
): Promise<void> {
  const editor = editors[serverTabId]
  const file = fileOf(serverTabId, path)
  const state = buffers.get(bufferKey(serverTabId, path))?.state
  if (!editor || !file || !state || file.saving || file.status !== 'ready') return
  if (!file.dirty && !force) return
  file.saving = true
  try {
    const doc = state.doc
    const reply = await serverRequest<SSHWriteResult>(serverTabId, editor.alias, {
      op: 'write',
      path,
      content: doc.toString(),
      mtime: file.mtime,
      force
    })
    if (reply.ok) {
      bufferOf(serverTabId, path).saved = doc
      file.mtime = reply.result.mtime
      file.changed = false
      file.notice = ''
      const current = buffers.get(bufferKey(serverTabId, path))?.state
      file.dirty = !!current && !current.doc.eq(doc)
    } else if (reply.code === 'changed') {
      file.changed = true
    } else if (reply.code === 'notLoggedIn') {
      file.notice = `Log in in the ${editor.alias} terminal first, then save again.`
    } else {
      file.notice = plain(reply.error)
    }
  } finally {
    file.saving = false
  }
}

export function forgetServerEditor(serverTabId: string): void {
  for (const file of editors[serverTabId]?.files ?? []) {
    buffers.delete(bufferKey(serverTabId, file.path))
  }
  delete editors[serverTabId]
}
