<script setup lang="ts">
import TerminalPane from './TerminalPane.vue'
import type { PaneTab } from '../../../preload'

// One leaf of the split-pane tree: a tab strip plus the terminals for
// its own tabs. Several of these can be on screen at once once a pane
// has been split -- see PaneLayout.vue for how they're arranged.

defineProps<{
  tabs: PaneTab[]
  activeTabId: string | null
  focused: boolean
}>()

const emit = defineEmits<{
  focus: []
  'select-tab': [tabId: string]
  'close-tab': [tabId: string]
  'add-tab': []
  split: [direction: 'row' | 'column']
  'close-pane': []
  exit: [tabId: string]
}>()
</script>

<template>
  <div class="terminal-group" :class="{ focused }" @mousedown="emit('focus')">
    <div class="tab-bar">
      <button
        v-for="tab in tabs"
        :key="tab.id"
        class="tab"
        :class="{ active: tab.id === activeTabId }"
        @click="emit('select-tab', tab.id)"
      >
        {{ tab.label }}
        <span class="tab-close" @click.stop="emit('close-tab', tab.id)">&times;</span>
      </button>
      <button class="tab-add" title="New shell" @click="emit('add-tab')">+</button>
      <div class="group-actions">
        <button class="group-action" title="Split right" @click="emit('split', 'row')">
          <svg width="12" height="12" viewBox="0 0 12 12">
            <rect x="0.5" y="0.5" width="11" height="11" fill="none" stroke="currentColor" stroke-width="1" />
            <line x1="6" y1="0.5" x2="6" y2="11.5" stroke="currentColor" stroke-width="1" />
          </svg>
        </button>
        <button class="group-action" title="Split down" @click="emit('split', 'column')">
          <svg width="12" height="12" viewBox="0 0 12 12">
            <rect x="0.5" y="0.5" width="11" height="11" fill="none" stroke="currentColor" stroke-width="1" />
            <line x1="0.5" y1="6" x2="11.5" y2="6" stroke="currentColor" stroke-width="1" />
          </svg>
        </button>
        <button class="group-action" title="Close pane" @click="emit('close-pane')">
          <svg width="12" height="12" viewBox="0 0 12 12">
            <line x1="1.5" y1="1.5" x2="10.5" y2="10.5" stroke="currentColor" stroke-width="1" />
            <line x1="10.5" y1="1.5" x2="1.5" y2="10.5" stroke="currentColor" stroke-width="1" />
          </svg>
        </button>
      </div>
    </div>
    <div class="panes">
      <p v-if="tabs.length === 0" class="empty">No shells open. Click + to start one.</p>
      <TerminalPane
        v-for="tab in tabs"
        v-show="tab.id === activeTabId"
        :key="tab.id"
        :command="tab.command"
        :cwd="tab.cwd"
        @exit="emit('exit', tab.id)"
      />
    </div>
  </div>
</template>

<style scoped>
.terminal-group {
  height: 100%;
  width: 100%;
  min-height: 0;
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.tab-bar {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-3) var(--space-4) var(--space-2);
}

.terminal-group.focused .tab-bar {
  box-shadow: inset 0 -2px 0 var(--color-amber);
}

.tab {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-4);
  background: transparent;
  border: none;
  border-radius: var(--radius-pill);
  color: var(--color-fg-secondary);
  font-family: var(--font-body);
  font-size: var(--text-sm);
  cursor: pointer;
  transition: background var(--duration-fast) var(--ease-out);
}

.tab:hover {
  background: var(--color-surface-1);
}

.tab.active {
  background: var(--color-surface-2);
  color: var(--color-fg-primary);
}

.tab-close {
  color: var(--color-fg-muted);
  border-radius: var(--radius-pill);
}

.tab-close:hover {
  color: var(--color-fg-primary);
}

.tab-add {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  background: transparent;
  border: none;
  border-radius: var(--radius-pill);
  color: var(--color-fg-accent);
  font-size: var(--text-md);
  cursor: pointer;
  transition: background var(--duration-fast) var(--ease-out);
}

.tab-add:hover {
  background: var(--color-surface-1);
}

.group-actions {
  display: flex;
  align-items: center;
  gap: var(--space-1);
  margin-left: auto;
}

.group-action {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  background: transparent;
  border: none;
  border-radius: var(--radius-md);
  color: var(--color-fg-muted);
  cursor: pointer;
  transition:
    background var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out);
}

.group-action:hover {
  background: var(--color-surface-1);
  color: var(--color-fg-primary);
}

.panes {
  flex: 1;
  min-height: 0;
  position: relative;
  padding: 0 var(--space-4) var(--space-4);
}

.panes > .terminal-card {
  height: 100%;
}

.empty {
  color: var(--color-fg-muted);
  font-size: var(--text-sm);
  padding: var(--space-4);
}
</style>
