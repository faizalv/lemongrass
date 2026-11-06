<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { IconFrame } from '@sankasten'
import type { IconShape } from '../tabKinds'

withDefaults(defineProps<{ shapes: IconShape[]; size?: number }>(), { size: 13 })
</script>

<template>
  <IconFrame class="tab-icon" :size="size">
    <component :is="shape.tag" v-for="(shape, index) in shapes" :key="index" v-bind="shape.attrs" />
  </IconFrame>
</template>
