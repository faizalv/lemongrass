<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { IconFrame } from '@sankasten'
import { CONFLICT_ICONS, type ConflictIconName } from '../../conflictIcons'

withDefaults(defineProps<{ name: ConflictIconName; size?: number }>(), { size: 13 })
</script>

<template>
  <IconFrame class="conflict-icon" :size="size" :stroke-width="1.3">
    <path v-for="d in CONFLICT_ICONS[name]" :key="d" :d="d" />
  </IconFrame>
</template>
