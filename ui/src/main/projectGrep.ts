// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { gitBinary, readEnv } from './gitRun'
import { grepProject as grepWith } from './projectGrepCore'
import type { ProjectTextResult } from '../preload/types'

const grepEnv = { ...readEnv, GIT_LITERAL_PATHSPECS: '0' }

export function grepProject(root: string, query: string): Promise<ProjectTextResult> {
  return grepWith(root, query, { binary: gitBinary(), env: grepEnv })
}
