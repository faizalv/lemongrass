<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { computed, watch } from 'vue'
import { gitStateOf } from '../../git'
import { useConflictThread } from '../../conflictThread'
import { conflictMode, modelHolds, setConflictMode } from '../../conflictVerdicts'
import { setConflictPath, type ProjectRef } from '../../workspace'
import ConflictFilePicker from './ConflictFilePicker.vue'
import ConflictFileView from './ConflictFileView.vue'
import ConflictModeSwitch from './ConflictModeSwitch.vue'
import DecisionsView from './DecisionsView.vue'
import { AppEmptyState } from '@sankasten'

// The conflict tab: one for the whole conflict session. While a model holds the conflict it shows
// Decisions, every block of every file the human has to decide; the switch shows one file in
// Manual, the three panes, and back. Without a model it is always Manual.
const props = defineProps<{
  project: ProjectRef
  path: string
  tabId: string
}>()

// Read once here for every view and the picker, since each reader refreshes the thread on every
// git status change.
const thread = useConflictThread(() => props.project)
const holds = computed(() => modelHolds(props.project.id))
const mode = computed(() => conflictMode(props.project.id))

const status = computed(() => gitStateOf(props.project.id).status)
const conflicted = computed(() =>
  (status.value?.files ?? []).filter((file) => file.group === 'conflicted').map((file) => file.path)
)
// Shown in place of a file only once git status has loaded and lists no conflicted file.
const noneLeft = computed(() => !!status.value && conflicted.value.length === 0)

// The tab lasts the whole session, so it never rests on a file that is no longer conflicted,
// whether it was applied here, resolved by the model or settled outside the app: it moves to the
// file that followed it in the list as last seen, or the first one.
let lastSeen: string[] = []
watch(
  [conflicted, () => props.path],
  ([paths, path]) => {
    if (!status.value) return
    if (paths.length > 0 && !paths.includes(path)) {
      const at = lastSeen.indexOf(path)
      const next = at === -1 ? undefined : lastSeen.slice(at + 1).find((p) => paths.includes(p))
      setConflictPath(props.project, props.tabId, next ?? paths[0])
    }
    lastSeen = paths
  },
  { immediate: true }
)

// A card's file opens in Manual: the tab moves to it first. Without a path the switch shows the
// file the tab last showed.
function openInManual(path: string | null): void {
  if (path) setConflictPath(props.project, props.tabId, path)
  setConflictMode(props.project.id, 'manual')
}
</script>

<template>
  <div v-if="noneLeft" class="none-left">
    <AppEmptyState
      layout="fill"
      title="No conflicts left in this project."
      detail="You can close this tab."
    />
  </div>
  <DecisionsView
    v-else-if="holds && mode === 'decisions'"
    :project="project"
    :thread="thread"
    :files="conflicted"
    @manual="openInManual"
  />
  <!-- Each file the tab showed stays cached, keyed by path, so switching files keeps its unsaved
       middle and its undo history, as switching tabs does. -->
  <KeepAlive v-else>
    <ConflictFileView :key="path" :project="project" :path="path" :thread="thread">
      <template #picker>
        <!-- First, as in Decisions, so it stays where it is whatever the file name's width. -->
        <ConflictModeSwitch
          v-if="holds"
          mode="manual"
          @switch="setConflictMode(project.id, 'decisions')"
        />
        <ConflictFilePicker
          :project="project"
          :path="path"
          :thread="thread"
          @select="setConflictPath(project, tabId, $event)"
        />
      </template>
    </ConflictFileView>
  </KeepAlive>
</template>

<style scoped>
.none-left {
  display: flex;
  height: 100%;
  background: var(--layer-page);
}
</style>
