// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 The Sankasten Authors

import type { InjectionKey } from 'vue'

// The selector of the enclosing AppModal's actions row, provided by AppModal for AppModalActions.
export const modalActionsKey: InjectionKey<string> = Symbol('sk-modal-actions')
