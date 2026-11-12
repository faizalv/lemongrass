// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { rmSync } from 'node:fs'
import { mkdtemp, readdir, rm, statfs } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import type { RelayPorts } from './sshRelay'

// A relay stages each copy in its own folder in the system temp directory, named with the app's
// pid so a later start can tell a crashed run's leftovers from a second running instance's.
const PREFIX = 'lemongrass-relay-'
const STAGE_NAME = /^lemongrass-relay-(\d+)-/

export const relayStagePorts: Pick<RelayPorts, 'stage' | 'freeBytes' | 'removeStage' | 'join'> = {
  stage: () => mkdtemp(join(tmpdir(), `${PREFIX}${process.pid}-`)),
  freeBytes: async (dir) => {
    const s = await statfs(dir)
    return s.bavail * s.bsize
  },
  removeStage: (dir) => rm(dir, { recursive: true, force: true }),
  join
}

function alive(pid: number): boolean {
  try {
    process.kill(pid, 0)
    return true
  } catch (err) {
    // EPERM means the pid exists under another user, so its folder is not ours to judge.
    return (err as NodeJS.ErrnoException).code === 'EPERM'
  }
}

// Removes staging folders whose app process is gone. /tmp is RAM on many Linux systems, so a
// crash mid-copy must not hold the staged bytes until reboot.
export async function sweepStaleStages(): Promise<void> {
  const dir = tmpdir()
  const names = await readdir(dir).catch(() => [] as string[])
  for (const name of names) {
    const pid = Number(STAGE_NAME.exec(name)?.[1])
    if (!pid || pid === process.pid || alive(pid)) continue
    await rm(join(dir, name), { recursive: true, force: true }).catch(() => undefined)
  }
}

// Quit cannot wait on promises, so the folders still in use go synchronously.
export function removeStagesNow(dirs: string[]): void {
  for (const dir of dirs) {
    try {
      rmSync(dir, { recursive: true, force: true })
    } catch {
      // A folder that cannot be removed now is swept at the next start.
    }
  }
}
