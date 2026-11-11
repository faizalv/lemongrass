// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import type { IconShape } from '@sankasten'
import { deviceIcon, nodeIcon, resterClientIcon, vaultIcon } from './icons'
import { nodeStatus } from './nodeStatus'
import { vaultSession } from './vaultSession'
import { openDevice, openResterClient, openVault, type ProjectRef } from './workspace'

export type ToolId = 'vault' | 'device' | 'resterClient' | 'node'

export type ToolTone = 'neutral' | 'accent' | 'success' | 'danger'

// App-wide state only, shown as the tool button's dot and tooltip. Nothing per project, so a button
// reads the same in every project.
export interface ToolStatus {
  // A word or two, such as Locked or Shared.
  text?: string
  tone: ToolTone
}

export interface ToolKind {
  id: ToolId
  label: string
  icon: IconShape[]
  status: () => ToolStatus
  open: (project: ProjectRef) => void
}

const IDLE: ToolStatus = { tone: 'neutral' }

// Tools that open a tab or a modal and join the workspace, shown in this order in the bottom group
// of the side toolbar. A new tool is one entry here.
export const TOOL_KINDS: ToolKind[] = [
  {
    id: 'resterClient',
    label: 'Rester Client',
    icon: resterClientIcon,
    status: () => IDLE,
    open: openResterClient
  },
  {
    id: 'vault',
    label: 'Vault',
    icon: vaultIcon,
    status: () =>
      vaultSession.unlocked
        ? { text: 'Unlocked', tone: 'accent' }
        : { text: 'Locked', tone: 'neutral' },
    open: openVault
  },
  {
    id: 'device',
    label: 'Device',
    icon: deviceIcon,
    status: () => IDLE,
    open: openDevice
  },
  {
    id: 'node',
    label: 'Node',
    icon: nodeIcon,
    status: () => {
      switch (nodeStatus.phase) {
        case 'shared':
          return { text: 'Shared', tone: 'success' }
        case 'connecting':
          return { text: 'Connecting', tone: 'accent' }
        case 'checking':
          return { text: 'Checking', tone: 'neutral' }
        default:
          return { text: 'Off', tone: 'neutral' }
      }
    },
    open: () => {
      nodeStatus.modalOpen = true
    }
  }
]

// The tool button's tooltip, such as "Vault: Locked".
export function toolTitle(kind: ToolKind, status: ToolStatus): string {
  return status.text ? `${kind.label}: ${status.text}` : kind.label
}

// A neutral status draws no dot, so only a state worth noticing marks the button.
export function toolDot(status: ToolStatus): Exclude<ToolTone, 'neutral'> | undefined {
  return status.tone === 'neutral' ? undefined : status.tone
}
