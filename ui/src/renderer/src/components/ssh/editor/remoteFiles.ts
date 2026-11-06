// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { reactive } from 'vue'
import type { EditorState, Text } from '@codemirror/state'
import type { SSHReadResult, SSHWriteResult } from '../../../../../preload/types'
import { serverRequest } from '../files/filesStore'

// A remote file tab's state lives here, keyed by the tab, not in the component: moving the tab to
// another pane remounts its body, and unsaved edits with their undo history must survive that.
// The live EditorState and the saved text stay out of Vue's reactivity, since a proxy around a
// CodeMirror object breaks its identity checks and costs a walk of a 2 MB document.

export type RemoteFileStatus =
  'loading' | 'ready' | 'binary' | 'tooLarge' | 'isDir' | 'notLoggedIn' | 'error'

export interface RemoteFile {
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

interface Buffer {
  saved: Text | null
  state: EditorState | null
}

const files = reactive<Record<string, RemoteFile>>({})
const buffers = new Map<string, Buffer>()

// The helper prefixes its errors with its package name, which says nothing to the human.
function plain(error: string): string {
  return error.replace(/^sshsession: /, '')
}

export function remoteFileOf(tabId: string): RemoteFile {
  if (!files[tabId]) {
    files[tabId] = {
      status: 'loading',
      error: '',
      content: '',
      revision: 0,
      mtime: 0,
      dirty: false,
      changed: false,
      saving: false,
      notice: ''
    }
  }
  return files[tabId]
}

function bufferOf(tabId: string): Buffer {
  let buffer = buffers.get(tabId)
  if (!buffer) {
    buffer = { saved: null, state: null }
    buffers.set(tabId, buffer)
  }
  return buffer
}

export function isRemoteFileDirty(tabId: string): boolean {
  return files[tabId]?.dirty ?? false
}

export function editorStateOf(tabId: string): EditorState | undefined {
  return buffers.get(tabId)?.state ?? undefined
}

export interface RemoteTarget {
  serverTabId: string
  alias: string
  path: string
}

// Reads the file, replacing whatever the editor holds.
export async function readRemoteFile(tabId: string, target: RemoteTarget): Promise<void> {
  const file = remoteFileOf(tabId)
  file.status = 'loading'
  const reply = await serverRequest<SSHReadResult>(target.serverTabId, target.alias, {
    op: 'read',
    path: target.path
  })
  if (!reply.ok) {
    const code = reply.code
    file.status =
      code === 'binary' || code === 'tooLarge' || code === 'isDir' || code === 'notLoggedIn'
        ? code
        : 'error'
    file.error = plain(reply.error)
    return
  }
  const buffer = bufferOf(tabId)
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
export function editorChanged(tabId: string, state: EditorState): void {
  const file = files[tabId]
  if (!file) return
  const buffer = bufferOf(tabId)
  buffer.state = state
  if (!buffer.saved) buffer.saved = state.doc
  file.dirty = !state.doc.eq(buffer.saved)
}

export async function saveRemoteFile(
  tabId: string,
  target: RemoteTarget,
  force = false
): Promise<void> {
  const file = files[tabId]
  const state = buffers.get(tabId)?.state
  if (!file || !state || file.saving || file.status !== 'ready') return
  if (!file.dirty && !force) return
  file.saving = true
  try {
    const doc = state.doc
    const reply = await serverRequest<SSHWriteResult>(target.serverTabId, target.alias, {
      op: 'write',
      path: target.path,
      content: doc.toString(),
      mtime: file.mtime,
      force
    })
    if (reply.ok) {
      bufferOf(tabId).saved = doc
      file.mtime = reply.result.mtime
      file.changed = false
      file.notice = ''
      const current = buffers.get(tabId)?.state
      file.dirty = !!current && !current.doc.eq(doc)
    } else if (reply.code === 'changed') {
      file.changed = true
    } else if (reply.code === 'notLoggedIn') {
      file.notice = `Log in in the ${target.alias} terminal first, then save again.`
    } else {
      file.notice = plain(reply.error)
    }
  } finally {
    file.saving = false
  }
}

export function forgetRemoteFile(tabId: string): void {
  buffers.delete(tabId)
  delete files[tabId]
}
