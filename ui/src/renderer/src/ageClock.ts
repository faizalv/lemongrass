// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { ref } from 'vue'

const TICK_MS = 30 * 1000

// One clock for every age label, so "a moment ago" turns into "a minute ago" without a refresh.
export const ageNow = ref(Date.now())

setInterval(() => (ageNow.value = Date.now()), TICK_MS)
