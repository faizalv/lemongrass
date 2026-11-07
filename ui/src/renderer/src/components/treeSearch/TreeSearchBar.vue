<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { AppInput } from '@sankasten'
import type { SearchField } from '../../useSearchField'

// The default slot holds a view's own search options under the field, such as Biblio's scope.
defineProps<{
  search: SearchField
  status: string
  placeholder: string
}>()
</script>

<template>
  <div class="tree-search-bar">
    <AppInput
      :ref="search.bindInput"
      :model-value="search.query.value"
      class="field"
      size="sm"
      icon="search"
      :placeholder="placeholder"
      aria-label="Search the tree"
      @update:model-value="search.setQuery"
      @keydown="search.onInputKeydown"
      @blur="search.onInputBlur"
    />
    <slot />
    <span v-if="status" class="status">{{ status }}</span>
  </div>
</template>

<style scoped>
.tree-search-bar {
  display: flex;
  flex-direction: column;
  gap: 2px;
  margin-bottom: var(--space-2);
}

.tree-search-bar .field {
  width: 100%;
}

.status {
  padding-left: var(--space-1);
  color: var(--color-fg-muted);
  font-size: var(--text-xs);
}
</style>
