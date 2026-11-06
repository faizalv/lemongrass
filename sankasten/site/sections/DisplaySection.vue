<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The Sankasten Authors -->

<script setup lang="ts">
import { ref } from 'vue'
import {
  APP_ICONS,
  AppBadge,
  AppButton,
  AppChip,
  AppDisclosure,
  AppEmptyState,
  AppIcon,
  AppIconButton,
  AppInfoPopover,
  AppModal,
  AppNavGroup,
  AppNavItem,
  AppPad,
  AppProgressLine,
  IconFrame,
  type AppIconName,
  type IconShape
} from '../../src'
import DemoRow from '../parts/DemoRow.vue'
import DocBlock from '../parts/DocBlock.vue'

const badgeTones = ['accent', 'info', 'success', 'danger', 'neutral'] as const
const chipTones = ['default', 'quiet', 'accent', 'info', 'success', 'danger'] as const
const iconNames = Object.keys(APP_ICONS) as AppIconName[]

const open = ref(true)
const stashOpen = ref(false)
const folded = ref(true)
const pad = ref('one')
const pinned = ref(false)
const account = ref('second')
const chips = ref(['orders', 'users', 'invoices'])
const excluded = ref(false)
const modal = ref(false)
const anchor = ref<HTMLElement | null>(null)

const folder: IconShape[] = [{ tag: 'path', attrs: { d: 'M1.5 4h4l1.5 1.5h5.5v6h-11z' } }]
</script>

<template>
  <section id="display" class="section">
    <h2>Display and navigation</h2>

    <DocBlock name="AppNavItem">
      <div class="sidebar">
        <AppNavItem label="Contents" size="sm" active>
          <template #icon><AppIcon name="info" /></template>
        </AppNavItem>
        <AppNavItem
          label="Vault"
          state="Locked"
          size="sm"
          :count="2"
          count-title="2 decisions wait"
        >
          <template #icon><AppIcon name="copy" /></template>
        </AppNavItem>
        <AppNavItem
          label="Source"
          state="main"
          size="sm"
          :count="4"
          :expanded="!folded"
          :active="!folded"
          @click="folded = !folded"
        >
          <template #icon><AppIcon name="chevronRight" /></template>
        </AppNavItem>
        <AppNavItem label="Merge" state="conflict" size="sm" tone="danger" :count="3" />
        <AppNavItem label="Groups" size="sm" :count="1" alert="A group waits for your approval">
          <template #icon><AppIcon name="plus" /></template>
        </AppNavItem>
        <AppNavItem label="Node" state="Shared" size="sm">
          <template #trailing><span class="dot" /></template>
        </AppNavItem>
      </div>
    </DocBlock>

    <DocBlock name="AppNavGroup">
      <div class="groups">
        <div v-for="s in ['md', 'sm'] as const" :key="s" class="sidebar">
          <AppNavItem label="General" :size="s">
            <template #icon><AppIcon name="info" /></template>
          </AppNavItem>
          <AppNavGroup :size="s">
            <template #parent>
              <AppNavItem label="Accounts" :size="s" @click="account = 'first'">
                <template #icon><AppIcon name="copy" /></template>
              </AppNavItem>
            </template>
            <AppNavItem
              v-for="a in ['first', 'second', 'third']"
              :key="a"
              :label="a"
              :size="s"
              :active="account === a"
              @click="account = a"
            >
              <template #icon><AppIcon name="check" /></template>
            </AppNavItem>
          </AppNavGroup>
          <AppNavItem label="Updates" :size="s">
            <template #icon><AppIcon name="search" /></template>
          </AppNavItem>
        </div>
      </div>
    </DocBlock>

    <DocBlock name="AppDisclosure">
      <div class="sidebar">
        <AppDisclosure v-model:open="open" label="Changes" :count="12" block />
        <div class="row">
          <AppDisclosure v-model:open="stashOpen" label="Stashes" :count="2" count-look="pill" />
          <span class="spacer" />
          <AppIconButton icon="plus" label="New stash" size="sm" tone="accent" />
        </div>
      </div>
    </DocBlock>

    <DocBlock name="AppChip">
      <DemoRow label="tones">
        <AppChip v-for="t in chipTones" :key="t" :tone="t">{{ t }}</AppChip>
      </DemoRow>
      <DemoRow label="remove">
        <AppChip
          v-for="(c, i) in chips"
          :key="c"
          mono
          removable
          :remove-label="`Remove ${c}`"
          @remove="chips.splice(i, 1)"
        >
          {{ c }}
        </AppChip>
      </DemoRow>
      <DemoRow label="tag, action">
        <AppChip
          tone="quiet"
          tag="global"
          :struck="excluded"
          :action-label="excluded ? 'Include' : 'Exclude'"
          @action="excluded = !excluded"
        >
          /usr/bin/git
        </AppChip>
        <AppChip
          tone="danger"
          mono
          removable
          tabindex="0"
          aria-describedby="missing-hint"
          @mouseenter="anchor = $event.currentTarget as HTMLElement"
          @mouseleave="anchor = null"
          @focus="anchor = $event.currentTarget as HTMLElement"
          @blur="anchor = null"
        >
          payments
        </AppChip>
        <AppInfoPopover id="missing-hint" :anchor="anchor">
          This table does not exist on the server.
        </AppInfoPopover>
      </DemoRow>
    </DocBlock>

    <DocBlock name="AppBadge">
      <DemoRow v-for="s in ['md', 'sm'] as const" :key="s" :label="s">
        <AppBadge v-for="t in badgeTones" :key="t" :tone="t" :size="s">{{
          s === 'md' ? 12 : 3
        }}</AppBadge>
      </DemoRow>
      <DemoRow label="on an icon button; a status dot and a trailing mark instead of a count">
        <AppIconButton icon="info" label="Notifications" size="sm">
          <template #badge><AppBadge size="sm">5</AppBadge></template>
        </AppIconButton>
        <AppIconButton icon="copy" label="Changes waiting" size="sm" dot="accent" />
        <AppIconButton icon="copy" label="Changes waiting, 2 unsent" size="sm" dot="accent">
          <template #trailing>
            <svg
              class="demo-ahead"
              viewBox="0 0 10 10"
              fill="none"
              stroke="currentColor"
              stroke-width="1.5"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <path d="M5 8.5v-7M2 4.5l3-3 3 3" />
            </svg>
          </template>
        </AppIconButton>
        <AppIconButton icon="close" label="Stopped" size="sm" dot="danger" pressed />
      </DemoRow>
    </DocBlock>

    <DocBlock name="AppPad">
      <div class="pad-grid">
        <AppPad
          v-for="p in ['one', 'two', 'three']"
          :key="p"
          :label="`Agent ${p}`"
          :detail="`run-${p}`"
          mono
          :selected="pad === p"
          @click="pad = p"
        >
          <template #icon><AppIcon name="copy" /></template>
        </AppPad>
        <AppPad label="Tunnel" detail="Shared" tone="success" size="sm">
          <template #icon><AppIcon name="search" /></template>
          <template #corner>
            <AppIconButton
              icon="plus"
              size="xs"
              :label="pinned ? 'Unpin' : 'Pin'"
              :pressed="pinned"
              @click="pinned = !pinned"
            />
          </template>
        </AppPad>
      </div>
    </DocBlock>

    <DocBlock name="AppEmptyState">
      <AppEmptyState title="No stashes." />
      <AppEmptyState title="Nothing matches." detail="Try a shorter filter." size="sm" />
      <AppEmptyState loading title="Loading history" />
      <div class="fill-box">
        <AppEmptyState
          layout="fill"
          :icon="folder"
          title="Nothing open."
          detail="Pick a file in the tree."
        >
          <template #action><AppButton variant="primary">Open a file</AppButton></template>
        </AppEmptyState>
      </div>
    </DocBlock>

    <DocBlock name="AppProgressLine">
      <AppProgressLine label="Running" />
      <AppProgressLine label="Uploading" tone="amber" :value="0.4" />
    </DocBlock>

    <DocBlock name="AppInfoPopover">
      <p class="out">Shown on the danger chip under AppChip: hover or focus it.</p>
    </DocBlock>

    <DocBlock name="AppModal">
      <div class="modal-box">
        <AppButton @click="modal = true">Open a modal</AppButton>
        <AppModal v-if="modal" title="Rename branch" @close="modal = false">
          <p class="out">The modal covers its nearest positioned container with scope container.</p>
          <template #footer>
            <AppButton @click="modal = false">Cancel</AppButton>
            <AppButton variant="primary" @click="modal = false">Rename</AppButton>
          </template>
        </AppModal>
      </div>
    </DocBlock>

    <DocBlock name="AppIcon">
      <div class="icons">
        <div v-for="n in iconNames" :key="n" class="icon-cell">
          <AppIcon :name="n" :size="16" />
          <code>{{ n }}</code>
        </div>
      </div>
    </DocBlock>

    <DocBlock name="IconFrame">
      <DemoRow label="custom glyph">
        <IconFrame :size="20">
          <path d="M2 7h10M7 2v10" />
        </IconFrame>
        <IconFrame :size="20" :box="[10, 14]" :stroke-width="null" fill="currentColor">
          <circle cx="5" cy="7" r="3" />
        </IconFrame>
      </DemoRow>
    </DocBlock>
  </section>
</template>

<style scoped>
.sidebar {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
  width: 260px;
  padding: var(--space-2);
  background: var(--color-surface-1);
  border-radius: var(--radius-md);
}

.groups {
  display: flex;
  gap: var(--space-4);
}

.row {
  display: flex;
  align-items: center;
  padding: 0 var(--space-2);
}

.spacer {
  flex: 1;
}

.dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--color-success);
}

.fill-box {
  display: flex;
  flex-direction: column;
  height: 220px;
  border: 1px solid var(--color-border-subtle);
  border-radius: var(--radius-md);
}

.modal-box {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  height: 260px;
  border: 1px solid var(--color-border-subtle);
  border-radius: var(--radius-md);
}

.icons {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));
  gap: var(--space-2);
}

.icon-cell {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2);
  color: var(--color-fg-primary);
}

.icon-cell code {
  font-size: var(--text-xs);
}

.out {
  color: var(--color-fg-muted);
  font-size: var(--text-xs);
}

.pad-grid {
  display: grid;
  grid-template-columns: repeat(4, 120px);
  gap: var(--space-2);
}

.demo-ahead {
  color: var(--color-success);
}
</style>
