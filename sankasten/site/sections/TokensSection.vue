<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The Sankasten Authors -->

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { AppButton, AppIconButton, AppInput, AppSegmented } from '../../src'

// Token values are read from the live stylesheet, so the page always shows what tokens.css holds.
const values = ref<Record<string, string>>({})

const colorGroups: { title: string; tokens: string[] }[] = [
  {
    title: 'Brand and accents',
    tokens: [
      'amber',
      'amber-dim',
      'amber-muted',
      'coral',
      'orange',
      'jade',
      'sapphire',
      'violet',
      'fuchsia',
      'teal'
    ].map((n) => `--color-${n}`)
  },
  {
    title: 'Surfaces',
    tokens: ['surface-0', 'surface-1', 'surface-2', 'surface-3'].map((n) => `--color-${n}`)
  },
  {
    title: 'Text',
    tokens: ['fg-primary', 'fg-secondary', 'fg-muted', 'fg-disabled', 'fg-accent'].map(
      (n) => `--color-${n}`
    )
  },
  {
    title: 'Borders',
    tokens: ['border-subtle', 'border-default', 'border-strong'].map((n) => `--color-${n}`)
  },
  {
    title: 'Status',
    tokens: ['success', 'success-muted', 'warning', 'error', 'error-muted', 'info', 'deleted'].map(
      (n) => `--color-${n}`
    )
  }
]

const layers = [
  { token: '--layer-page', use: 'The page.' },
  { token: '--layer-card', use: 'A card or list row on the page, with border-subtle.' },
  { token: '--layer-card-hover', use: 'Its hover.' },
  { token: '--layer-raised', use: 'A card on a card, a sidebar entry, a modal content card.' },
  { token: '--layer-raised-hover', use: 'Its hover.' },
  { token: '--layer-input', use: 'Every input: always the darkest, so it reads as cut in.' },
  { token: '--layer-float', use: 'Dropdowns, popovers, menus.' },
  { token: '--layer-float-hover', use: 'A menu row hover.' }
]

const textSizes = ['xs', 'sm', 'base', 'md', 'lg', 'xl', '2xl', '3xl'].map((n) => `--text-${n}`)
const fonts = ['--font-display', '--font-body', '--font-mono']
const weights = ['regular', 'medium', 'semibold', 'bold', 'black'].map((n) => `--weight-${n}`)
const spaces = [1, 2, 3, 4, 5, 6, 8, 10, 12, 16].map((n) => `--space-${n}`)
const radii = ['sm', 'md', 'lg', 'xl', '2xl', 'pill'].map((n) => `--radius-${n}`)
const shadows = ['sm', 'md', 'lg', 'xl', 'glow-amber'].map((n) => `--shadow-${n}`)
const motion = [
  '--duration-fast',
  '--duration-base',
  '--duration-slow',
  '--duration-slower',
  '--ease-default',
  '--ease-out',
  '--ease-in'
]
const controls = [
  '--control-h-xs',
  '--control-h-sm',
  '--control-h-md',
  '--control-icon-xs',
  '--control-icon-sm',
  '--control-icon-md'
]

const all = [
  ...colorGroups.flatMap((g) => g.tokens),
  ...layers.map((l) => l.token),
  ...textSizes,
  ...fonts,
  ...weights,
  ...spaces,
  ...radii,
  ...shadows,
  ...motion,
  ...controls
]

onMounted(() => {
  const style = getComputedStyle(document.documentElement)
  values.value = Object.fromEntries(all.map((t) => [t, style.getPropertyValue(t).trim()]))
})

const segment = ref('a')
</script>

<template>
  <section id="tokens" class="section">
    <h2>Tokens</h2>
    <p class="lead">
      Components read color, type, spacing, radius, motion and height only through these variables,
      never a raw value. Import <code>styles/index.css</code> once and every token is on
      <code>:root</code>; <code>data-theme="light"</code> on an ancestor switches the light values.
    </p>

    <h3 id="tokens-color">Color</h3>
    <div v-for="group in colorGroups" :key="group.title" class="token-group">
      <h4>{{ group.title }}</h4>
      <div class="swatches">
        <div v-for="t in group.tokens" :key="t" class="swatch">
          <span class="chip" :style="{ background: `var(${t})` }" />
          <code>{{ t }}</code>
          <span class="value">{{ values[t] }}</span>
        </div>
      </div>
    </div>

    <h3 id="tokens-layers">Layers</h3>
    <p>
      Backgrounds go through layer roles, never a surface step picked per screen, so the same
      element sits on the same layer everywhere. Chrome such as header bars and toolbars keeps its
      own surface.
    </p>
    <div class="layer-demo">
      <div class="layer-page">
        <span class="layer-tag">page</span>
        <div class="layer-card">
          <span class="layer-tag">card</span>
          <div class="layer-raised">
            <span class="layer-tag">raised</span>
            <AppInput model-value="input, darkest" size="sm" />
          </div>
        </div>
      </div>
      <table class="token-table">
        <tbody>
          <tr v-for="l in layers" :key="l.token">
            <td>
              <code>{{ l.token }}</code>
            </td>
            <td class="value">{{ values[l.token] }}</td>
            <td>{{ l.use }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <h3 id="tokens-type">Type</h3>
    <div class="type-list">
      <div v-for="f in fonts" :key="f" class="type-row">
        <code>{{ f }}</code>
        <span :style="{ fontFamily: `var(${f})`, fontSize: 'var(--text-lg)' }">
          Sankasten builds calm tools
        </span>
      </div>
      <div v-for="t in textSizes" :key="t" class="type-row">
        <code>{{ t }} {{ values[t] }}</code>
        <span :style="{ fontSize: `var(${t})` }">The quick amber fox</span>
      </div>
      <div class="type-row">
        <code>weights</code>
        <span>
          <span v-for="w in weights" :key="w" :style="{ fontWeight: `var(${w})` }" class="weight">
            {{ w.replace('--weight-', '') }}
          </span>
        </span>
      </div>
    </div>

    <h3 id="tokens-space">Spacing and radius</h3>
    <div class="space-list">
      <div v-for="s in spaces" :key="s" class="space-row">
        <code>{{ s }}</code>
        <span class="bar" :style="{ width: `var(${s})` }" />
        <span class="value">{{ values[s] }}</span>
      </div>
    </div>
    <div class="radius-list">
      <div v-for="r in radii" :key="r" class="radius">
        <span class="radius-box" :style="{ borderRadius: `var(${r})` }" />
        <code>{{ r }}</code>
      </div>
    </div>

    <h3 id="tokens-depth">Shadow and motion</h3>
    <div class="radius-list">
      <div v-for="s in shadows" :key="s" class="radius">
        <span class="radius-box shadow" :style="{ boxShadow: `var(${s})` }" />
        <code>{{ s }}</code>
      </div>
    </div>
    <table class="token-table">
      <tbody>
        <tr v-for="m in motion" :key="m">
          <td>
            <code>{{ m }}</code>
          </td>
          <td class="value">{{ values[m] }}</td>
        </tr>
      </tbody>
    </table>

    <h3 id="tokens-height">Control height</h3>
    <p>
      Every single-line control is exactly one of these tall, border included, so two controls of
      one size always line up. Height comes from the token and padding is horizontal only. A screen
      picks a size; it never sets a control's height or vertical padding.
    </p>
    <table class="token-table">
      <tbody>
        <tr v-for="c in controls" :key="c">
          <td>
            <code>{{ c }}</code>
          </td>
          <td class="value">{{ values[c] }}</td>
        </tr>
      </tbody>
    </table>
    <div class="height-demo">
      <div v-for="s in ['md', 'sm'] as const" :key="s" class="height-row">
        <span class="height-guide" :style="{ height: `var(--control-h-${s})` }" />
        <AppInput model-value="Field" :size="s" style="width: 140px" />
        <AppButton :size="s" variant="primary">Primary</AppButton>
        <AppButton :size="s">Ghost</AppButton>
        <AppSegmented
          v-model="segment"
          :size="s"
          aria-label="Mode"
          :options="[
            { value: 'a', label: 'One' },
            { value: 'b', label: 'Two' }
          ]"
        />
        <AppIconButton icon="plus" label="Add" :size="s" variant="ghost" />
        <code>{{ s }}</code>
      </div>
    </div>
  </section>
</template>

<style scoped>
h3 {
  margin: var(--space-8) 0 var(--space-3);
  font-size: var(--text-lg);
  scroll-margin-top: var(--space-6);
}

h4 {
  margin: var(--space-4) 0 var(--space-2);
  color: var(--color-fg-secondary);
  font-family: var(--font-body);
  font-size: var(--text-xs);
  font-weight: var(--weight-semibold);
  letter-spacing: var(--tracking-wide);
  text-transform: uppercase;
}

.swatches {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(190px, 1fr));
  gap: var(--space-2);
}

.swatch {
  display: grid;
  grid-template-columns: 32px 1fr;
  grid-template-rows: auto auto;
  column-gap: var(--space-2);
  align-items: center;
  padding: var(--space-2);
  background: var(--layer-card);
  border: 1px solid var(--color-border-subtle);
  border-radius: var(--radius-md);
}

.swatch .chip {
  grid-row: span 2;
  width: 32px;
  height: 32px;
  border: 1px solid var(--color-border-default);
  border-radius: var(--radius-sm);
}

.swatch code {
  font-size: var(--text-xs);
}

.value {
  color: var(--color-fg-muted);
  font-family: var(--font-mono);
  font-size: var(--text-xs);
}

.layer-demo {
  display: grid;
  grid-template-columns: minmax(260px, 1fr) 2fr;
  gap: var(--space-4);
  align-items: start;
}

.layer-page,
.layer-card,
.layer-raised {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  padding: var(--space-3);
  border: 1px solid var(--color-border-subtle);
  border-radius: var(--radius-md);
}

.layer-page {
  background: var(--layer-page);
}

.layer-card {
  background: var(--layer-card);
}

.layer-raised {
  background: var(--layer-raised);
}

.layer-tag {
  color: var(--color-fg-muted);
  font-size: var(--text-xs);
}

.token-table {
  width: 100%;
  border-collapse: collapse;
  font-size: var(--text-xs);
}

.token-table td {
  padding: var(--space-1) var(--space-2);
  border-bottom: 1px solid var(--color-border-subtle);
  color: var(--color-fg-secondary);
}

.type-list,
.space-list {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.type-row,
.space-row {
  display: grid;
  grid-template-columns: 200px 1fr;
  align-items: baseline;
  gap: var(--space-4);
}

.space-row {
  grid-template-columns: 120px auto 1fr;
  align-items: center;
}

.weight + .weight {
  margin-left: var(--space-3);
}

.bar {
  height: 10px;
  background: var(--color-amber);
  border-radius: 2px;
}

.radius-list {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-4);
  margin: var(--space-4) 0;
}

.radius {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-2);
}

.radius-box {
  width: 64px;
  height: 48px;
  background: var(--layer-raised);
  border: 1px solid var(--color-border-default);
}

.radius-box.shadow {
  border-radius: var(--radius-md);
}

.height-demo {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  margin-top: var(--space-4);
  padding: var(--space-4);
  background: var(--layer-card);
  border-radius: var(--radius-md);
}

.height-row {
  position: relative;
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

/* A band the exact token height behind the row, so the alignment is visible. */
.height-guide {
  position: absolute;
  left: -8px;
  right: 0;
  border-top: 1px dashed var(--color-amber-dim);
  border-bottom: 1px dashed var(--color-amber-dim);
  pointer-events: none;
}
</style>
