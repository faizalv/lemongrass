<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
export interface Tile {
  title: string
  // Trusted HTML from the home page's own copy, so inline code can be marked up.
  body?: string
  items?: string[]
  link?: string
  // Columns out of six, and rows, on wide screens. Narrow screens stack every tile full width.
  cols: 2 | 3 | 4 | 6
  rows?: 1 | 2
  // A corner glow in one of the logo's colours, for the tiles that carry the pitch.
  glow?: 'amber' | 'rose' | 'violet' | 'indigo'
}

defineProps<{ tiles: Tile[] }>()
</script>

<template>
  <div class="tiles">
    <component
      :is="t.link ? 'a' : 'div'"
      v-for="t in tiles"
      :key="t.title"
      class="tile"
      :class="[`c${t.cols}`, `r${t.rows ?? 1}`, t.glow && `glow ${t.glow}`, { linked: !!t.link }]"
      :href="t.link"
    >
      <h3 class="title">{{ t.title }}</h3>
      <p v-if="t.body" class="body" v-html="t.body" />
      <ul v-if="t.items" class="items">
        <li v-for="i in t.items" :key="i" v-html="i" />
      </ul>
      <span v-if="t.link" class="more">Read more</span>
    </component>
  </div>
</template>

<style scoped>
.tiles {
  display: grid;
  grid-template-columns: repeat(6, 1fr);
  grid-auto-rows: minmax(0, auto);
  gap: var(--space-4);
}

.c2 { grid-column: span 2; }
.c3 { grid-column: span 3; }
.c4 { grid-column: span 4; }
.c6 { grid-column: span 6; }
.r2 { grid-row: span 2; }

.tile {
  display: flex;
  flex-direction: column;
  padding: var(--space-6);
  background: var(--layer-card);
  border: 1px solid var(--color-border-default);
  border-radius: var(--radius-xl);
  color: inherit;
  text-decoration: none;
  transition:
    border-color var(--duration-fast) var(--ease-default),
    background var(--duration-fast) var(--ease-default);
}

.tile.glow {
  background:
    radial-gradient(120% 140% at 0% 0%, var(--tile-glow), transparent 60%),
    var(--layer-card);
}

/* Mixed from the full colour rather than the -muted tokens: at their 14% the cool hues vanish on the dark card. */
.amber { --tile-glow: color-mix(in oklch, var(--color-amber) 20%, transparent); }
.rose { --tile-glow: color-mix(in oklch, var(--color-rose) 26%, transparent); }
.violet { --tile-glow: color-mix(in oklch, var(--color-violet) 26%, transparent); }
.indigo { --tile-glow: color-mix(in oklch, var(--color-indigo) 30%, transparent); }

.tile.linked:hover {
  border-color: var(--color-border-strong);
}

.tile.linked:not(.glow):hover {
  background-color: var(--layer-card-hover);
}

.tile.linked:focus-visible {
  outline: 2px solid var(--color-amber);
  outline-offset: 2px;
}

.title {
  margin: 0 0 var(--space-3);
  font-size: var(--text-lg);
  font-weight: var(--weight-semibold);
  line-height: var(--leading-snug);
  color: var(--color-fg-primary);
}

.c4 .title,
.c6 .title,
.r2 .title {
  font-size: var(--text-xl);
}

.body,
.items {
  margin: 0;
  font-size: var(--text-sm);
  line-height: var(--leading-relaxed);
  color: var(--color-fg-secondary);
}

.items {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(14rem, 1fr));
  gap: var(--space-2) var(--space-6);
  margin-top: var(--space-3);
  padding-left: 1.1em;
}

.tile :deep(code) {
  font-family: var(--font-mono);
  font-size: 0.9em;
  padding: 0.1em 0.35em;
  border-radius: var(--radius-sm);
  background: var(--layer-raised);
  color: var(--color-fg-primary);
}

.more {
  margin-top: auto;
  padding-top: var(--space-4);
  font-size: var(--text-sm);
  font-weight: var(--weight-medium);
  color: var(--color-amber);
}

@media (max-width: 768px) {
  .tiles {
    grid-template-columns: 1fr;
  }

  .tile {
    grid-column: auto;
    grid-row: auto;
  }
}
</style>
