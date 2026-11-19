<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { IconFrame, type IconShape } from '@sankasten'

// A sidebar entry's glyph, shared by its full row and its icon in the collapsed rail.
defineProps<{ shapes: IconShape[] }>()
</script>

<template>
  <IconFrame :size="14">
    <component :is="shape.tag" v-for="(shape, i) in shapes" :key="i" v-bind="shape.attrs" />
  </IconFrame>
</template>
