// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

// Shared by main, which trims a request's history, and the renderer's setting, so both agree on
// the choices.

// How many runs a request's history may keep, picked in Settings > General.
export const RUN_LIMIT_CHOICES = [5, 10, 20, 50]
export const DEFAULT_RUN_LIMIT = 10

// The stored runs of one request, newest included, stay within this many bytes. It matches the
// inline response cap, so one response at the cap leaves no room for older runs.
export const HISTORY_BUDGET_BYTES = 10 << 20

export function validRunLimit(value: unknown): value is number {
  return typeof value === 'number' && RUN_LIMIT_CHOICES.includes(value)
}
