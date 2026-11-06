<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { AppButton, AppIconButton, AppInput } from '@sankasten'
import type { ResterClientPair } from '../../../../preload/types'

defineProps<{
  addLabel: string
  namePlaceholder: string
}>()

// Edited in place; the parent owns the array.
const pairs = defineModel<ResterClientPair[]>({ required: true })
</script>

<template>
  <div class="pairs">
    <div v-for="(pair, i) in pairs" :key="i" class="pair">
      <AppInput
        v-model="pair.name"
        size="sm"
        type="text"
        :placeholder="namePlaceholder"
        spellcheck="false"
      />
      <AppInput v-model="pair.value" size="sm" type="text" placeholder="value" spellcheck="false" />
      <AppIconButton icon="close" size="sm" label="Remove" @click="pairs.splice(i, 1)" />
    </div>
    <AppButton class="add" size="sm" @click="pairs.push({ name: '', value: '' })"
      >+ {{ addLabel }}</AppButton
    >
  </div>
</template>

<style scoped>
.pairs {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.pair {
  display: grid;
  grid-template-columns: minmax(0, 2fr) minmax(0, 3fr) auto;
  gap: var(--space-2);
  align-items: center;
}

.add {
  align-self: flex-start;
}
</style>
