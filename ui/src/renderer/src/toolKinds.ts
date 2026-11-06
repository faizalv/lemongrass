// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import type { IconShape } from '@sankasten'
import { deviceIcon, nodeIcon, resterClientIcon, sshIcon, vaultIcon } from './icons'
import { nodeStatus } from './nodeStatus'
import { vaultSession } from './vaultSession'
import { openDevice, openResterClient, openSSH, openVault, type ProjectRef } from './workspace'

export type ToolId = 'vault' | 'device' | 'resterClient' | 'ssh' | 'node'

export type ToolTone = 'neutral' | 'accent' | 'success' | 'danger'

// App-wide state only, shown on the tool's pad in the toolbox. Nothing per project: the dock is a
// set of stateless shortcuts and the pads must read the same in every project.
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
  // Always in the dock, ahead of the human's pins, and cannot be unpinned.
  alwaysPinned?: boolean
}

const IDLE: ToolStatus = { tone: 'neutral' }

// Tools that open a tab or a modal and join the workspace, shown in the header toolbox and dock.
// A new tool is one entry here.
export const TOOL_KINDS: ToolKind[] = [
  {
    id: 'vault',
    label: 'Vault',
    alwaysPinned: true,
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
    id: 'resterClient',
    label: 'Rester Client',
    alwaysPinned: true,
    icon: resterClientIcon,
    status: () => IDLE,
    open: openResterClient
  },
  {
    id: 'ssh',
    label: 'SSH',
    icon: sshIcon,
    status: () => IDLE,
    open: openSSH
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

export function toolKindById(id: string): ToolKind | undefined {
  return TOOL_KINDS.find((kind) => kind.id === id)
}

// The pad's hover title, such as "Vault: Locked".
export function toolTitle(kind: ToolKind, status: ToolStatus): string {
  return status.text ? `${kind.label}: ${status.text}` : kind.label
}
