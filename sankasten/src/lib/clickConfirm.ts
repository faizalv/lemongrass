// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 The Sankasten Authors

export type ConfirmState = 'idle' | 'pending' | 'armed'

// Long enough to swallow a habitual double click as well as a mouse that fires twice, so the
// second click of a destructive action is always a deliberate one.
export const CONFIRM_DELAY_MS = 1000

// What a press did: `started` the wait, came `early` during it (ignored, but worth explaining to
// the user), or `confirmed`, in which case the action should run.
export type PressResult = 'started' | 'early' | 'confirmed'

export interface ClickConfirm {
  state(): ConfirmState
  press(): PressResult
  reset(): void
}

export function createClickConfirm(
  onChange: (state: ConfirmState) => void,
  delay = CONFIRM_DELAY_MS
): ClickConfirm {
  let state: ConfirmState = 'idle'
  let timer: ReturnType<typeof setTimeout> | undefined

  function set(next: ConfirmState): void {
    if (state === next) return
    state = next
    onChange(next)
  }

  function reset(): void {
    clearTimeout(timer)
    timer = undefined
    set('idle')
  }

  function press(): PressResult {
    if (state === 'armed') {
      reset()
      return 'confirmed'
    }
    if (state === 'pending') return 'early'
    set('pending')
    timer = setTimeout(() => {
      timer = undefined
      set('armed')
    }, delay)
    return 'started'
  }

  return { state: () => state, press, reset }
}
