// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 The Sankasten Authors

// What a parent can call on an AppInput through a template ref.
export interface AppInputHandle {
  focus: () => void
  select: () => void
}
