<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { computed } from 'vue'
import type { BlockContext, BlockHold } from '../../conflictDecisions'
import type { ProjectRef } from '../../workspace'
import type { ConflictCommentPane, GitConflictChunk } from '../../../../preload/types'
import BlockCode from './BlockCode.vue'
import ConflictIcon from './ConflictIcon.vue'

// A block in its place: both sides side by side, each with a few clean lines around the block, and
// the result under them with more. Read only and colored by the file's grammar, with the block's
// lines tinted by what became of it. Between the sides and the result an arrow under each side
// says what became of it: faintly green and pointing down at the result for a side that was
// taken, a faint red cross for one that was not. A taken side also has a green line along its
// bottom edge, joining it to its arrow. A result that equals no side is
// violet, the model's color, and has no arrows.
const props = defineProps<{
  // The conflicted file's path, whose extension picks the grammar of every box.
  path: string
  // The project and git block the boxes' comments are scoped to.
  project: ProjectRef
  block: number
  sides: Extract<GitConflictChunk, { kind: 'conflict' }>
  hold: BlockHold
  leftRef: string
  rightRef: string
  leftContext: BlockContext
  rightContext: BlockContext
  resultContext: BlockContext
  // Hides the result, for a block whose result is not on disk yet.
  noResult?: boolean
}>()

type Mark = 'taken' | 'rejected' | 'open' | 'edited'

interface Pane {
  key: ConflictCommentPane
  title: string
  lines: string[]
  from: number
  to: number
  start: number | null
  mark: Mark
  empty: string
}

function paneOf(
  key: ConflictCommentPane,
  title: string,
  block: string[],
  context: BlockContext,
  mark: Mark,
  empty: string
): Pane {
  return {
    key,
    title,
    lines: [...context.before, ...block, ...context.after],
    from: context.before.length,
    to: context.before.length + block.length,
    start: context.start,
    mark,
    empty
  }
}

// Written text that equals one side or both marks them taken and the rest rejected; text that
// equals neither marks both sides neutral; markers left mark both open.
function sideMark(side: 'left' | 'right'): Mark {
  if (props.hold.kind !== 'written') return 'open'
  if (!props.hold.sides) return 'edited'
  return props.hold.sides.includes(side) ? 'taken' : 'rejected'
}

const panes = computed((): Pane[] => [
  paneOf(
    'left',
    props.leftRef,
    props.sides.left,
    props.leftContext,
    sideMark('left'),
    'This side has no lines here'
  ),
  paneOf(
    'right',
    props.rightRef,
    props.sides.right,
    props.rightContext,
    sideMark('right'),
    'This side has no lines here'
  )
])

const result = computed((): Pane | null => {
  if (props.hold.kind !== 'written' || props.noResult) return null
  return paneOf(
    'result',
    'Result',
    props.hold.lines,
    props.resultContext,
    props.hold.sides ? 'taken' : 'edited',
    'The block is removed'
  )
})

const arrowTitle: Partial<Record<Mark, string>> = {
  taken: 'Taken into the result',
  rejected: 'Not taken'
}

// The arrows row is there only when a decision gives the sides something to say.
const decided = computed(() => panes.value.some((p) => p.mark === 'taken' || p.mark === 'rejected'))
</script>

<template>
  <div class="block-sides">
    <div class="sides">
      <section
        v-for="pane in panes"
        :key="pane.key"
        class="box"
        :class="{ selected: pane.mark === 'taken' }"
      >
        <header class="box-head">
          <span class="box-title" :title="pane.title">
            <span class="from-word">From</span> <span class="from-ref">{{ pane.title }}</span>
          </span>
        </header>
        <BlockCode
          :path="path"
          :lines="pane.lines"
          :from="pane.from"
          :to="pane.to"
          :mark="pane.mark"
          :start="pane.start"
          :comment="{ project, pane: pane.key, block }"
        />
        <p v-if="pane.from === pane.to" class="empty" :class="pane.mark">{{ pane.empty }}</p>
      </section>
    </div>

    <div v-if="decided && result" class="arrows">
      <span
        v-for="pane in panes"
        :key="pane.key"
        class="arrow"
        :class="pane.mark"
        :title="arrowTitle[pane.mark]"
      >
        <ConflictIcon
          v-if="pane.mark === 'taken' || pane.mark === 'rejected'"
          :name="pane.mark === 'taken' ? 'arrowTaken' : 'crossRejected'"
          :size="28"
        />
      </span>
    </div>

    <section v-if="result" class="box">
      <header class="box-head">
        <span class="box-title">{{ result.title }}</span>
      </header>
      <BlockCode
        :path="path"
        :lines="result.lines"
        :from="result.from"
        :to="result.to"
        :mark="result.mark"
        :start="result.start"
        :comment="{ project, pane: result.key, block }"
      />
      <p v-if="result.from === result.to" class="empty" :class="result.mark">{{ result.empty }}</p>
    </section>
  </div>
</template>

<style scoped>
.block-sides {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  min-width: 0;
}

.sides,
.arrows {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: var(--space-3);
  min-width: 0;
}

.box {
  min-width: 0;
  overflow: hidden;
  border: 1px solid var(--color-border-subtle);
  border-radius: var(--radius-md);
  background: var(--color-surface-0);
}

.box-head {
  padding: var(--space-1) var(--space-2);
  border-bottom: 1px solid var(--color-border-subtle);
  font-size: var(--text-xs);
  color: var(--color-fg-secondary);
}

.box-title {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.box.selected {
  border-bottom: 2px solid var(--color-jade);
}

.from-word {
  color: var(--color-fg-primary);
}

.from-ref {
  font-weight: var(--weight-semibold);
  color: var(--color-fg-accent);
}

.arrow {
  display: flex;
  justify-content: center;
  min-height: 28px;
  opacity: 0.6;
}

.arrow.taken {
  color: var(--color-jade);
}

.arrow.rejected {
  color: var(--color-error);
}

.empty {
  margin: 0;
  padding: var(--space-1) var(--space-2);
  font-size: var(--text-xs);
  font-style: italic;
  color: var(--color-fg-secondary);
}

.empty.taken {
  background: var(--color-jade-muted);
}

.empty.rejected {
  background: var(--color-error-muted);
}

.empty.open {
  background: var(--color-sapphire-muted);
}

.empty.edited {
  background: var(--color-violet-muted);
}
</style>
