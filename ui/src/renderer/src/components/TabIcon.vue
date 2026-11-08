<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { IconFrame } from '@sankasten'
import type { IconShape } from '../tabKinds'
import type { VendorMark } from '../vendorMarks'

// A vendor mark, when given, replaces the shapes. It fills with currentColor, so it follows the tab's text color as
// the stroked glyphs do.
withDefaults(defineProps<{ shapes: IconShape[]; mark?: VendorMark; size?: number }>(), {
  mark: undefined,
  size: 13
})
</script>

<template>
  <IconFrame
    v-if="mark"
    class="tab-icon"
    :size="size"
    :box="mark.box"
    :stroke-width="null"
    fill="currentColor"
  >
    <path :d="mark.d" />
  </IconFrame>
  <IconFrame v-else class="tab-icon" :size="size">
    <component :is="shape.tag" v-for="(shape, index) in shapes" :key="index" v-bind="shape.attrs" />
  </IconFrame>
</template>
