<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The Sankasten Authors -->

<template>
  <section id="overview" class="section">
    <h2>Sankasten</h2>
    <p class="lead">
      A dark, calm design system for tools people keep open all day: design tokens, the shared
      control styles, an icon set and the Vue 3 components built on them. This site is the reference
      for using the components and for making new ones.
    </p>
    <pre class="usage"><code>import 'sankasten/styles/index.css'
import { AppButton, AppIconButton, AppInput } from 'sankasten'</code></pre>
    <p class="lead">
      Import the styles once at startup, then every component from the package entry. The fonts the
      type tokens name (Comfortaa Variable, DM Sans Variable, JetBrains Mono Variable) are bundled
      by the app, for example from the <code>@fontsource-variable</code> packages, never fetched at
      run time.
    </p>
    <h3>Making a component</h3>
    <ul class="list">
      <li>
        Read every color, size, space, radius and duration from a token. No raw hex in a component.
      </li>
      <li>
        A single-line control takes <code>size?: 'sm' | 'md'</code> with <code>md</code> the
        default, sets <code>height</code> from <code>--control-h-*</code> with
        <code>box-sizing: border-box</code>, centers its content with flex, and keeps padding
        horizontal. Give every variant a 1px border, transparent where it is not drawn, so variants
        of one size match.
      </li>
      <li>
        Let attributes and listeners fall through to the native element, so <code>disabled</code>,
        <code>title</code>, <code>@click</code> and <code>aria-*</code> work without props.
      </li>
      <li>
        Draw a glyph from <code>AppIcon</code> or a component on <code>IconFrame</code>, never an
        inline svg for a glyph that means the same on any screen.
      </li>
      <li>
        Shared look across components goes in the global <code>sk-</code> classes of
        <code>controls.css</code>; everything else is scoped.
      </li>
      <li>Nothing app-specific: no app names, stores or platform APIs in the package.</li>
    </ul>
  </section>
</template>

<style scoped>
h3 {
  font-size: var(--text-lg);
}

.usage {
  padding: var(--space-4);
  background: var(--layer-card);
  border: 1px solid var(--color-border-subtle);
  border-radius: var(--radius-md);
  font-family: var(--font-mono);
  font-size: var(--text-xs);
}

.list {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  max-width: 80ch;
  padding-left: var(--space-5);
  color: var(--color-fg-secondary);
}
</style>
