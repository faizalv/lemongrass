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
</style>
