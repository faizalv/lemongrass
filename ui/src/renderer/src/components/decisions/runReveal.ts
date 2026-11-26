// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { onBeforeUnmount, ref, watch, type Ref } from 'vue'
import type { Ask } from '../../../../preload/types'

// A command that ends within this time goes from Execute straight to its result, so its running view
// never flashes in between.
export const RUN_REVEAL_MS = 600

// True once the ask's command has been running for RUN_REVEAL_MS, counted from the daemon's start time
// so a command reopened long after it started shows as running at once. Stays true until it stops running.
export function useRunReveal(ask: () => Ask | null | undefined): Ref<boolean> {
  const revealed = ref(false)
  let timer: ReturnType<typeof setTimeout> | undefined

  watch(
    () => {
      const a = ask()
      return a?.state === 'running' ? (a.result?.started_at ?? '') : null
    },
    (startedAt) => {
      clearTimeout(timer)
      if (startedAt === null) {
        revealed.value = false
        return
      }
      if (revealed.value) return
      const since = startedAt ? Date.now() - new Date(startedAt).getTime() : 0
      const wait = RUN_REVEAL_MS - (Number.isNaN(since) ? 0 : since)
      if (wait <= 0) {
        revealed.value = true
        return
      }
      timer = setTimeout(() => (revealed.value = true), wait)
    },
    { immediate: true }
  )

  onBeforeUnmount(() => clearTimeout(timer))
  return revealed
}
