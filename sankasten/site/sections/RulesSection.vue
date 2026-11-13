<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The Sankasten Authors -->

<script setup lang="ts">
import { AppButton, AppIconButton, AppInput, AppMenuButton, AppIcon } from '../../src'

const more = [
  { key: 'rename', label: 'Rename' },
  { key: 'delete', label: 'Delete', confirm: true }
]
</script>

<template>
  <section id="rules" class="section">
    <h2>Rules</h2>

    <article id="rules-weight" class="rule">
      <h3>Weight by importance</h3>
      <p>
        Every control on a surface gets a weight. A framed button is for the one or two primary
        actions. Navigation and toggles are bare icons. Status is plain text. Rarer or riskier
        actions move behind a menu. Never add one more button of the same look beside the rest.
      </p>
      <div class="example">
        <AppIconButton icon="search" label="Search" size="sm" />
        <AppIconButton icon="copy" label="Copy path" size="sm" />
        <span class="status">3 changes</span>
        <span class="spacer" />
        <AppMenuButton :items="more" label="More actions" size="sm">
          <AppIcon name="dragHandle" />
        </AppMenuButton>
        <AppButton variant="primary" size="sm">Commit</AppButton>
      </div>
      <p class="hint">
        A control drawn over text, in an editor or a reader, always has a solid fill, so the text
        never shows through it.
      </p>
    </article>

    <article id="rules-sizes" class="rule">
      <h3>One row, one size</h3>
      <p>
        <code>sm</code> is for dense surfaces: box and panel headers, toolbars, sidebars, popovers,
        any strip of 38px or less. <code>md</code> is for form fields, modal bodies and footers,
        empty-state actions and a full view's main action. <code>xs</code> is only for an action
        inside a list, tree or menu row, so the row keeps the height of the plain rows around it.
        Never mix sizes side by side.
      </p>
      <div class="example">
        <AppInput model-value="Filter" size="sm" icon="search" style="width: 160px" />
        <AppButton size="sm">Refresh</AppButton>
        <AppIconButton icon="close" label="Clear" size="sm" />
      </div>
    </article>

    <article id="rules-labels" class="rule">
      <h3>Every icon button has a label</h3>
      <p>
        A glyph alone has no name for a screen reader and no tooltip for a new user. AppIconButton
        makes
        <code>label</code> required and puts it in <code>title</code> and <code>aria-label</code>. A
        disabled button's <code>title</code> says why it is disabled.
      </p>
    </article>

    <article id="rules-copy" class="rule">
      <h3>No dashes in UI copy</h3>
      <p>
        Text shown in the interface never uses an em dash or a double hyphen. Split the thought into
        two sentences instead. A message of two sentences is a title and a detail.
      </p>
    </article>

    <article id="rules-tokens" class="rule">
      <h3>Tokens and layers, never raw values</h3>
      <p>
        Colors and type go through tokens, backgrounds through layer roles. Inputs sit on the
        darkest layer wherever they are, so a field always reads as cut into its container.
      </p>
    </article>

    <article id="rules-color-steps" class="rule">
      <h3>Text steps by role</h3>
      <p>
        <code>fg-primary</code> is the thing itself: names, values, titles.
        <code>fg-secondary</code> is supporting text, such as a detail or a description.
        <code>fg-muted</code> is only for metadata a reader can skip, such as a path, a count, a
        timestamp or a hint. <code>fg-disabled</code> is only for a disabled control. Nothing a
        reader must read to act is muted.
      </p>
      <div class="example">
        <span class="step-primary">inventory.js</span>
        <span class="step-secondary">Changed by the last merge.</span>
        <span class="spacer" />
        <span class="status">src · 2 min ago</span>
      </div>
    </article>

    <article id="rules-color-surface" class="rule">
      <h3>Contrast against the real surface</h3>
      <p>
        Primary and secondary text keep at least 4.5:1, muted hints at least 4:1. Measure against
        what is drawn under the text, not the layer the screen sits on. A row's hover, a selection
        or a tint is part of that surface.
      </p>
    </article>

    <article id="rules-color-tint" class="rule">
      <h3>A tint is a surface</h3>
      <p>
        A muted fill, such as an active row, a pressed toggle, a banner or a selection, lowers the
        contrast of everything on it. <code>fg-muted</code> drops from 4.2:1 on
        <code>surface-2</code> to 3.1:1 on <code>amber-muted</code> over it. On a tint the lowest
        text step is <code>fg-secondary</code>. A component that draws a tint redefines
        <code>fg-muted</code> as <code>fg-secondary</code> inside itself, so content written for a
        plain layer stays readable without knowing where it sits.
      </p>
      <div class="example tint-example">
        <div class="tint-row">
          <span class="tint-name">merge/a-to-staging</span>
          <span class="status">current</span>
        </div>
        <div class="tint-row tint-fixed">
          <span class="tint-name">merge/a-to-staging</span>
          <span class="status">current</span>
        </div>
      </div>
      <p class="hint">
        The first row leaves the hint muted on the tint. The second redefines muted inside the row.
      </p>
    </article>

    <article id="rules-color-jobs" class="rule">
      <h3>Every color has a job</h3>
      <p>
        Amber marks focus, the active or selected thing and the one primary action. Status colors
        mark state: success, warning, error, info and deleted. The named hues tell kinds of things
        apart, such as file types, syntax, monograms and terminal colors, and carry no state. Jade
        is the one exception: it marks running work and a go action. Never pick a hue because it
        looks right.
      </p>
    </article>

    <article id="rules-color-alone" class="rule">
      <h3>Color never works alone</h3>
      <p>
        A state shown by color also has an icon, a word or a shape. Dense screens and color
        blindness both lose a difference that only a hue carries.
      </p>
    </article>

    <article id="rules-color-fix" class="rule">
      <h3>Fix color where it is defined</h3>
      <p>
        A color that reads badly is fixed in its token or in the component that draws the surface,
        and every screen gets the fix. A screen never overrides a color to patch its own contrast.
      </p>
    </article>
  </section>
</template>

<style scoped>
.rule {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  padding: var(--space-6);
  background: var(--layer-card);
  border: 1px solid var(--color-border-subtle);
  border-radius: var(--radius-lg);
  scroll-margin-top: var(--space-6);
}

.rule h3 {
  font-size: var(--text-lg);
}

.rule p {
  max-width: 80ch;
  color: var(--color-fg-secondary);
}

.example {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-3);
  background: var(--color-surface-1);
  border: 1px solid var(--color-border-default);
  border-radius: var(--radius-md);
}

.status {
  color: var(--color-fg-muted);
  font-size: var(--text-xs);
}

.spacer {
  flex: 1;
}

.hint {
  font-size: var(--text-xs);
}

.step-primary {
  color: var(--color-fg-primary);
  font-size: var(--text-sm);
}

.step-secondary {
  color: var(--color-fg-secondary);
  font-size: var(--text-xs);
}

.tint-example {
  flex-direction: column;
  align-items: stretch;
  background: var(--layer-raised);
}

.tint-row {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-1) var(--space-2);
  border-radius: var(--radius-sm);
  background: var(--color-amber-muted);
}

.tint-fixed {
  --color-fg-muted: var(--color-fg-secondary);
}

.tint-name {
  color: var(--color-fg-accent);
  font-size: var(--text-sm);
}
</style>
