<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  attachShell,
  closeFailedShell,
  continueOnPrimary,
  copyShellSelection,
  focusShell,
  fitShell,
  pasteIntoShell,
  pasteTextIntoShell,
  setShellActive,
  shellAccountGone,
  shellHasSelection,
  shellRestoreFailures,
  startFreshShell,
  terminalFontReady,
  type ShellSpec
} from '../shellRegistry'
import { AppButton, AppContextMenu, AppEmptyState, type MenuItem } from '@sankasten'
import { heldThinkers } from '../thinkerHold'
import { terminalScheme } from '../appearance'
import { PATH_DRAG_MIME, isPathDrag, quotePath } from '../pathDrag'

const props = defineProps<{
  spec: ShellSpec
  // False while the tab sits behind its pane's active tab.
  active: boolean
}>()

const containerEl = ref<HTMLDivElement>()
let resizeObserver: ResizeObserver | undefined
const menu = ref<{ x: number; y: number; items: MenuItem[] } | null>(null)

function openMenu(event: MouseEvent): void {
  menu.value = {
    x: event.clientX,
    y: event.clientY,
    items: [
      { key: 'copy', label: 'Copy', disabled: !shellHasSelection(props.spec.id) },
      { key: 'paste', label: 'Paste' }
    ]
  }
}

function onMenuSelect(key: string): void {
  menu.value = null
  if (key === 'copy') void copyShellSelection(props.spec.id)
  else void pasteIntoShell(props.spec.id)
  focusShell(props.spec.id)
}

// A sidebar tree row held over the terminal: dropping it types its path, followed by a space so a
// second drop stays a separate word. Tab drags are not path drags and keep the pane's handling.
const pathOver = ref(false)

function onPathDragOver(event: DragEvent): void {
  if (!isPathDrag(event)) return
  event.preventDefault()
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'copy'
  pathOver.value = true
}

function onPathDragLeave(event: DragEvent): void {
  const card = event.currentTarget as HTMLElement
  if (!card.contains(event.relatedTarget as Node | null)) pathOver.value = false
}

function onPathDrop(event: DragEvent): void {
  pathOver.value = false
  const path = event.dataTransfer?.getData(PATH_DRAG_MIME)
  if (!path) return
  event.preventDefault()
  pasteTextIntoShell(props.spec.id, `${quotePath(path)} `)
  focusShell(props.spec.id)
}

let unmounted = false
let attached = false
// A held thinker has no process yet, and attaching would start one, so the terminal waits for the release.
const held = computed(() => heldThinkers[props.spec.id] === true)

async function attach(): Promise<void> {
  await terminalFontReady()
  if (unmounted || attached || !containerEl.value) return
  attached = true
  attachShell(props.spec, containerEl.value)
  setShellActive(props.spec.id, props.active)
  resizeObserver = new ResizeObserver(() => fitShell(props.spec.id))
  resizeObserver.observe(containerEl.value)
}

onMounted(() => {
  if (!held.value) void attach()
})

watch(
  held,
  (isHeld) => {
    if (!isHeld) void attach()
  },
  { flush: 'post' }
)

// Post flush, so the tab is already displayed when the fit reads its size.
watch(
  () => props.active,
  (active) => {
    if (attached) setShellActive(props.spec.id, active)
  },
  { flush: 'post' }
)

onBeforeUnmount(() => {
  unmounted = true
  resizeObserver?.disconnect()
})
</script>

<template>
  <AppEmptyState
    v-if="held"
    layout="fill"
    loading
    title="Thinker is preparing"
    detail="Another thinker in this group starts first. This one starts right after, so it can reuse the prompt the first one cached."
  />
  <div
    v-else
    class="terminal-card fade-in"
    :class="{ 'path-drop': pathOver }"
    :data-theme="terminalScheme"
    @dragover="onPathDragOver"
    @dragleave="onPathDragLeave"
    @drop="onPathDrop"
  >
    <div ref="containerEl" class="terminal-pane" @contextmenu.prevent="openMenu"></div>
    <div v-if="shellAccountGone[spec.id]" class="restore-failure">
      <span class="restore-message">
        The account {{ shellAccountGone[spec.id] }} was deleted, so this tab can't continue under
        it. Continuing on the primary account bills the primary and sends this conversation to it.
      </span>
      <AppButton class="close-tab" @click="closeFailedShell(spec.id)">Close tab</AppButton>
      <AppButton variant="primary" @click="continueOnPrimary(spec.id)">
        Continue on primary
      </AppButton>
    </div>
    <div v-else-if="shellRestoreFailures[spec.id]" class="restore-failure">
      <span class="restore-message">
        This tab could not resume its previous session. The agent's message is shown above.
      </span>
      <AppButton class="close-tab" @click="closeFailedShell(spec.id)">Close tab</AppButton>
      <AppButton variant="primary" @click="startFreshShell(spec.id)">Start a new session</AppButton>
    </div>
    <!-- The card's fade-in leaves a transform on it, which would clip a fixed menu to the card. -->
    <Teleport to="body">
      <AppContextMenu
        v-if="menu"
        :x="menu.x"
        :y="menu.y"
        :items="menu.items"
        @select="onMenuSelect"
        @close="menu = null"
      />
    </Teleport>
  </div>
</template>

<style scoped>
/* FitAddon sizes the terminal off the inner element's own box, without
   accounting for any padding or border on it, so the card chrome lives on
   this wrapper instead. */
.terminal-card {
  /* data-theme scopes the card to the terminal's scheme, which may stay dark in a light app. */
  flex: 1;
  min-height: 0;
  width: 100%;
  padding: var(--space-3);
  background: var(--color-surface-1);
  color: var(--color-fg-primary);
  border: 1px solid var(--color-border-subtle);
  border-radius: var(--radius-lg);
  overflow: hidden;
  position: relative;
}

.terminal-card.path-drop {
  border-color: var(--color-amber);
  box-shadow: inset 0 0 0 1px var(--color-amber);
}

.terminal-pane {
  height: 100%;
  width: 100%;
  /* xterm's layers carry their own z-index (the WebGL link canvas sits at 2), so they are kept
     inside the pane and the bars drawn over the terminal stay above them for clicks. */
  isolation: isolate;
}

.restore-failure {
  position: absolute;
  left: var(--space-3);
  right: var(--space-3);
  bottom: var(--space-3);
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-3);
  background: var(--color-surface-2);
  border: 1px solid var(--color-border-default);
  border-radius: var(--radius-lg);
}

.restore-message {
  flex: 1;
  color: var(--color-fg-secondary);
  font-family: var(--font-body);
  font-size: var(--text-xs);
}

/* The failure bar sits on surface-2, so the ghost hover needs one step brighter to show. */
.restore-failure .close-tab:hover:not(:disabled) {
  background: var(--color-surface-3);
}
</style>
