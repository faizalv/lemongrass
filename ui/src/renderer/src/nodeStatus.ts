// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { reactive } from 'vue'

export type NodePhase = 'checking' | 'connecting' | 'shared' | 'off'

// The node is app-wide, so one poll serves every place that shows it; the modal flag lives here
// so the toolbox and the dock open the same modal.
export const nodeStatus = reactive({ phase: 'checking' as NodePhase, modalOpen: false })

let timer: ReturnType<typeof setTimeout> | undefined
let started = false

// Polls quickly while ngrok is connecting, which takes seconds, so Connecting does not linger.
export async function refreshNodeStatus(): Promise<void> {
  clearTimeout(timer)
  try {
    const st = (await window.api.node.state()).status
    nodeStatus.phase = st.running ? 'shared' : st.connecting ? 'connecting' : 'off'
  } catch {
    nodeStatus.phase = 'off'
  }
  timer = setTimeout(refreshNodeStatus, nodeStatus.phase === 'connecting' ? 2_000 : 15_000)
}

export function startNodeStatus(): void {
  if (started) return
  started = true
  void refreshNodeStatus()
}

export function closeNodeModal(): void {
  nodeStatus.modalOpen = false
  void refreshNodeStatus()
}
