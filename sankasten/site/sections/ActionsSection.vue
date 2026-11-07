<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The Sankasten Authors -->

<script setup lang="ts">
import { ref } from 'vue'
import {
  AppButton,
  AppConfirmButton,
  AppContextMenu,
  AppIcon,
  AppIconButton,
  AppMenuButton,
  AppEdgeTab,
  AppSplitButton,
  AppTab,
  AppTabClose,
  type MenuItem
} from '../../src'
import DemoRow from '../parts/DemoRow.vue'
import DocBlock from '../parts/DocBlock.vue'

const sizes = ['md', 'sm'] as const
const tones = ['neutral', 'accent', 'info', 'success', 'danger'] as const

const bold = ref(true)
const busy = ref(false)
const last = ref('Nothing yet.')

const items: MenuItem[] = [
  { key: 'rename', label: 'Rename' },
  { key: 'duplicate', label: 'Duplicate', icon: ['M4.5 4.5h7v7h-7z', 'M2.5 9.5v-7h7'] },
  { key: 'locked', label: 'Move', disabled: true, title: 'Not while it is open' },
  { key: 'delete', label: 'Delete', confirm: true }
]

const menuAt = ref<{ x: number; y: number } | null>(null)

const tabs = ref([
  { name: 'notes.txt', dirty: false },
  { name: 'server.conf', dirty: true },
  { name: 'a-rather-long-file-name.yaml', dirty: false }
])
const activeTab = ref('notes.txt')
const parked = ref(0)

function closeTab(name: string): void {
  tabs.value = tabs.value.filter((t) => t.name !== name)
  last.value = `Closed ${name}.`
}

function runBusy(): void {
  busy.value = true
  setTimeout(() => (busy.value = false), 1500)
}

function openMenu(event: MouseEvent): void {
  menuAt.value = { x: event.clientX, y: event.clientY }
}
</script>

<template>
  <section id="actions" class="section">
    <h2>Actions</h2>

    <DocBlock name="AppButton">
      <DemoRow v-for="s in sizes" :key="s" :label="`variants, ${s}`">
        <AppButton variant="primary" :size="s">Primary</AppButton>
        <AppButton :size="s">Ghost</AppButton>
        <AppButton variant="danger" :size="s">Danger</AppButton>
        <AppButton variant="text" :size="s">Text</AppButton>
      </DemoRow>
      <DemoRow label="xs, in a row">
        <div class="list-row">
          <span>stash@{0} WIP on main</span>
          <AppButton size="xs">Apply</AppButton>
          <AppConfirmButton variant="danger" size="xs">Drop</AppConfirmButton>
        </div>
      </DemoRow>
      <DemoRow label="tones on text">
        <AppButton v-for="t in tones" :key="t" variant="text" size="sm" :tone="t">{{
          t
        }}</AppButton>
      </DemoRow>
      <DemoRow label="states">
        <AppButton size="sm" :pressed="bold" variant="text" @click="bold = !bold"
          >Pressed</AppButton
        >
        <AppButton size="sm" disabled>Disabled</AppButton>
        <AppButton size="sm" variant="primary" disabled>Disabled</AppButton>
      </DemoRow>
    </DocBlock>

    <DocBlock name="AppIconButton">
      <DemoRow v-for="s in sizes" :key="s" :label="`bare, ${s}`">
        <AppIconButton icon="plus" label="Add" :size="s" />
        <AppIconButton icon="copy" label="Copy" :size="s" />
        <AppIconButton icon="search" label="Search" :size="s" />
        <AppIconButton icon="close" label="Close" tone="danger" :size="s" />
      </DemoRow>
      <DemoRow label="ghost and tinted">
        <AppIconButton icon="plus" label="Add" variant="ghost" size="sm" />
        <AppIconButton
          v-for="t in tones"
          :key="t"
          icon="check"
          :label="t"
          variant="tinted"
          :tone="t"
          size="sm"
        />
      </DemoRow>
      <DemoRow label="slot glyphs">
        <AppIconButton label="Bold" size="sm" :pressed="bold" @click="bold = !bold"
          >B</AppIconButton
        >
        <AppIconButton label="Heading 1" size="sm">H1</AppIconButton>
        <AppIconButton label="Drag" size="sm"><AppIcon name="dragHandle" /></AppIconButton>
      </DemoRow>
      <DemoRow label="xs, in a row">
        <div class="list-row">
          <span>docs/guide.md</span>
          <AppIconButton icon="plus" label="Add" size="xs" tone="accent" />
          <AppIconButton icon="trash" label="Delete" size="xs" tone="danger" confirm />
        </div>
      </DemoRow>
      <DemoRow label="busy, confirm">
        <AppIconButton
          icon="plus"
          label="Run"
          size="sm"
          variant="tinted"
          tone="info"
          :busy="busy"
          @click="runBusy"
        />
        <AppIconButton
          icon="trash"
          label="Delete"
          size="sm"
          tone="danger"
          confirm
          @confirm="last = 'Deleted.'"
        />
        <AppIconButton
          icon="trash"
          label="Delete"
          title="Open files cannot be deleted"
          size="sm"
          disabled
        />
        <span class="out">{{ last }}</span>
      </DemoRow>
    </DocBlock>

    <DocBlock name="AppConfirmButton">
      <DemoRow label="click twice">
        <AppConfirmButton variant="danger" size="sm" @confirm="last = 'Dropped.'"
          >Drop</AppConfirmButton
        >
        <AppConfirmButton variant="ghost" tone="jade" size="sm" @confirm="last = 'Started.'">
          Start
        </AppConfirmButton>
        <span class="out">{{ last }}</span>
      </DemoRow>
    </DocBlock>

    <DocBlock name="AppTab">
      <DemoRow label="a strip, one dirty">
        <div class="tab-strip" role="tablist" aria-label="Open files">
          <AppTab
            v-for="t in tabs"
            :key="t.name"
            :label="t.name"
            :title="`/srv/app/${t.name}`"
            :active="activeTab === t.name"
            :dirty="t.dirty"
            :close-label="`Close ${t.name}`"
            @select="activeTab = t.name"
            @close="closeTab(t.name)"
          >
            <template #icon><AppIcon name="copy" /></template>
          </AppTab>
        </div>
        <span class="out">{{ last }}</span>
      </DemoRow>
    </DocBlock>

    <DocBlock name="AppTabClose">
      <DemoRow v-for="s in sizes" :key="s" :label="`clean and dirty, ${s}`">
        <AppTabClose :size="s" label="Close" @close="last = 'Closed.'" />
        <AppTabClose :size="s" label="Close" dirty @close="last = 'Closed without saving.'" />
      </DemoRow>
    </DocBlock>

    <DocBlock name="AppEdgeTab">
      <DemoRow label="against a panel edge, left and right">
        <div class="edge-demo">
          <AppEdgeTab
            class="edge-left"
            :label="`Show the parked view, ${parked} open`"
            :count="parked"
            :dot="parked > 1"
            @click="parked++"
          >
            <template #icon><AppIcon name="copy" /></template>
          </AppEdgeTab>
          <div class="edge-panel">Panel</div>
          <AppEdgeTab class="edge-right" side="right" label="Show the other side">
            <template #icon><AppIcon name="chevronRight" /></template>
          </AppEdgeTab>
        </div>
      </DemoRow>
    </DocBlock>

    <DocBlock name="AppSplitButton">
      <DemoRow v-for="s in sizes" :key="s" :label="s">
        <AppSplitButton :items="items" :size="s" variant="primary" @select="(k) => (last = k)">
          Commit
        </AppSplitButton>
        <AppSplitButton :items="items" :size="s" @select="(k) => (last = k)">Merge</AppSplitButton>
      </DemoRow>
    </DocBlock>

    <DocBlock name="AppMenuButton">
      <DemoRow v-for="s in sizes" :key="s" :label="s">
        <AppMenuButton :items="items" label="More actions" :size="s" @select="(k) => (last = k)">
          <AppIcon name="dragHandle" />
        </AppMenuButton>
        <span class="out">{{ last }}</span>
      </DemoRow>
    </DocBlock>

    <DocBlock name="AppContextMenu">
      <div class="target" @contextmenu.prevent="openMenu">Right-click here</div>
      <Teleport to="body">
        <AppContextMenu
          v-if="menuAt"
          :x="menuAt.x"
          :y="menuAt.y"
          :items="items"
          @select="(k) => ((last = k), (menuAt = null))"
          @close="menuAt = null"
        />
      </Teleport>
    </DocBlock>
  </section>
</template>

<style scoped>
.out {
  color: var(--color-fg-muted);
  font-size: var(--text-xs);
}

/* A plain list row, 22px like the rows around it, holding xs actions. */
.list-row {
  display: flex;
  align-items: center;
  gap: var(--space-1);
  width: 320px;
  min-height: 22px;
  padding: 0 var(--space-2);
  background: var(--layer-card);
  border-radius: var(--radius-sm);
  color: var(--color-fg-secondary);
  font-size: var(--text-xs);
}

.list-row > span {
  flex: 1;
}

.tab-strip {
  display: flex;
  align-items: stretch;
  gap: 2px;
  height: 32px;
  width: 420px;
  padding: 0 var(--space-2);
  border-bottom: 1px solid var(--color-border-subtle);
  background: var(--color-surface-1);
}

/* The tabs touch the panel's sides; the panel is the layer they stick out from. */
.edge-demo {
  display: flex;
  align-items: flex-start;
}

.edge-demo .edge-left,
.edge-demo .edge-right {
  margin-top: var(--space-4);
}

.edge-panel {
  width: 160px;
  height: 96px;
  padding: var(--space-3);
  border: 1px solid var(--color-border-default);
  background: var(--color-surface-1);
  color: var(--color-fg-muted);
  font-size: var(--text-xs);
}

.target {
  padding: var(--space-6);
  border: 1px dashed var(--color-border-strong);
  border-radius: var(--radius-md);
  color: var(--color-fg-muted);
  text-align: center;
}
</style>
