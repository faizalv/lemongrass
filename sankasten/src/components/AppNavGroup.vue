<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The Sankasten Authors -->

<script setup lang="ts">
// A parent AppNavItem with its child AppNavItems indented under it and joined by tree guides: a
// vertical line from under the parent's icon down to the last child, and an elbow into each
// child's left edge, the last ending in a rounded corner. The group owns the gap between the
// parent and the children, so the line meets the parent's bottom edge exactly. size must match
// the items' size, since it places the line under the parent's 13px icon.
withDefaults(defineProps<{ size?: 'sm' | 'md' }>(), { size: 'md' })
</script>

<template>
  <div class="app-nav-group" :class="`size-${size}`">
    <slot name="parent" />
    <div class="nav-children" role="group">
      <slot />
    </div>
  </div>
</template>

<style scoped>
/* --guide is the line's x: the item's left padding plus half the 13px icon. --indent is where a
   child card starts, so the elbow runs from the line to the card's edge. */
.app-nav-group {
  --gap: var(--space-1);
  --guide: calc(var(--space-3) + 6px);
  --indent: calc(var(--guide) + var(--space-3));
  --line: var(--color-border-default);
  display: flex;
  flex-direction: column;
  gap: var(--gap);
}

.size-sm {
  --guide: calc(var(--space-2) + 6px);
  --indent: calc(var(--guide) + 10px);
}

.nav-children {
  display: flex;
  flex-direction: column;
  gap: var(--gap);
  padding-left: var(--indent);
}

.nav-children > :deep(*) {
  position: relative;
}

/* Each child draws the stretch of line beside it, reaching up through the gap above it, so the
   segments join into one line from the parent down. */
.nav-children > :deep(*)::before {
  content: '';
  position: absolute;
  top: calc(-1 * var(--gap));
  bottom: 0;
  left: calc(var(--guide) - var(--indent));
  border-left: 1px solid var(--line);
  pointer-events: none;
}

.nav-children > :deep(*:not(:last-child))::before {
  bottom: calc(-1 * var(--gap));
}

.nav-children > :deep(*)::after {
  content: '';
  position: absolute;
  top: 50%;
  left: calc(var(--guide) - var(--indent));
  width: calc(var(--indent) - var(--guide));
  border-top: 1px solid var(--line);
  pointer-events: none;
}

/* The last child's line stops at its middle and turns into the elbow with a rounded corner. */
.nav-children > :deep(*:last-child)::before {
  bottom: 50%;
  width: calc(var(--indent) - var(--guide));
  border-bottom: 1px solid var(--line);
  border-bottom-left-radius: var(--radius-md);
}

.nav-children > :deep(*:last-child)::after {
  display: none;
}

/* The elbow into the active child is drawn strong, so the open entry reads as reached from its
   parent. */
.nav-children > :deep(.active)::after,
.nav-children > :deep(.active:last-child)::before {
  border-color: var(--color-border-strong);
}
</style>
