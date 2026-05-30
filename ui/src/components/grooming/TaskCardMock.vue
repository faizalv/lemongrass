<template>
  <div :style="cardStyle">
    <!-- Title row -->
    <div style="display:flex;align-items:flex-start;gap:12px;margin-bottom:14px">
      <div :style="idxBadge">{{ task.idx }}</div>
      <div style="flex:1">
        <div :style="titleStyle">{{ task.title }}</div>
      </div>
      <span v-if="decision === 'accept' && commitStatus === 'open'" :style="pill('var(--color-success)','rgba(74,222,128,0.10)')">Accepted</span>
      <span v-if="decision === 'reject'" :style="pill('var(--color-error)','rgba(248,113,113,0.10)')">Rejected</span>
      <span v-if="decision === 'correction'" :style="pill('var(--color-amber)','rgba(245,197,24,0.10)')">Correction</span>
      <span v-if="commitStatus === 'committing'" :style="committingPill">
        <Spinner :size="10" /> Making detail…
      </span>
      <span v-if="commitStatus === 'committed'" :style="pill('var(--color-gray-300)','rgba(255,255,255,0.05)')">Detailed</span>
    </div>

    <!-- PRD reference -->
    <div style="margin-bottom:14px;padding-left:12px;border-left:2px solid rgba(245,197,24,0.30)">
      <div :style="sectionLabel">PRD reference</div>
      <div :style="prdText">"{{ task.prdRef }}"</div>
    </div>

    <!-- How-to -->
    <div style="margin-bottom:14px">
      <div :style="sectionLabel">How-to</div>
      <div :style="howToText">{{ task.howTo }}</div>
    </div>

    <!-- Affected files -->
    <div style="margin-bottom:16px">
      <div :style="sectionLabel">Affected files</div>
      <div :style="filesBox">
        <div
          v-for="(f, i) in task.files"
          :key="i"
          :style="{ ...fileRow, borderBottom: i < task.files.length - 1 ? '1px solid rgba(255,255,255,0.04)' : 'none' }"
        >
          <AppIcon
            :name="f.range === 'new file' ? 'file-plus' : 'file-pen-line'"
            :size="11"
            :extra-style="{ color: f.range === 'new file' ? 'var(--color-success)' : 'var(--color-amber)', flexShrink: 0 }"
          />
          <span style="color:var(--color-gray-100);flex-shrink:0;white-space:nowrap">{{ f.path }}</span>
          <span style="color:var(--color-gray-500);white-space:nowrap;flex-shrink:0">{{ f.range }}</span>
          <span style="flex:1" />
          <span :style="fileNote">{{ f.note }}</span>
        </div>
      </div>
    </div>

    <!-- Correction textarea -->
    <div v-if="showCorrection && commitStatus !== 'committed'" style="margin-bottom:14px">
      <div :style="{ ...sectionLabel, color: 'var(--color-amber)' }">Your correction</div>
      <textarea
        :value="correction"
        :style="correctionInput"
        placeholder="e.g. The bucket should also key on API token when present, not just user_id."
        @input="$emit('correction-change', ($event.target as HTMLTextAreaElement).value)"
      />
    </div>

    <!-- Action row -->
    <div v-if="commitStatus !== 'committed' && commitStatus !== 'committing'" style="display:flex;align-items:center;gap:8px;flex-wrap:wrap">
      <button :style="thumbBtn('var(--color-success)', decision === 'accept')" @click="toggleDecide('accept')">
        <AppIcon name="thumbs-up" :size="12" />
        Accept
      </button>
      <button :style="thumbBtn('var(--color-error)', decision === 'reject')" @click="toggleDecide('reject')">
        <AppIcon name="thumbs-down" :size="12" />
        Reject
      </button>
      <button :style="thumbBtn('var(--color-amber)', decision === 'correction')" @click="toggleCorrection">
        <AppIcon name="message-square-warning" :size="12" />
        Correction
      </button>
      <span style="flex:1" />
      <button v-if="decision === 'correction' && (correction || '').trim().length > 3" :style="amendBtn" @click="$emit('amend')">
        <AppIcon name="pencil-line" :size="12" />
        Amend with this
      </button>
    </div>

    <!-- Committing state -->
    <div v-if="commitStatus === 'committing'" :style="committingBar">
      <Spinner :size="13" />
      Distilling implementation detail — line ranges, patch shape, token-budget…
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import type { Task, Decision, TaskCommitStatus } from '../../types'
import AppIcon from '../AppIcon.vue'
import Spinner from './Spinner.vue'

const props = defineProps<{
  task: Task & { idx: number }
  decision: Decision
  correction?: string
  commitStatus: TaskCommitStatus
}>()

const emit = defineEmits<{
  'decide': [d: Decision]
  'correction-change': [text: string]
  'amend': []
}>()

const showCorrection = ref(props.decision === 'correction' || !!(props.correction))

watch(() => props.decision, (d) => {
  if (d === 'correction') showCorrection.value = true
})

function toggleDecide(state: 'accept' | 'reject') {
  emit('decide', props.decision === state ? null : state)
  showCorrection.value = false
}

function toggleCorrection() {
  if (props.decision === 'correction') {
    emit('decide', null)
    showCorrection.value = false
  } else {
    emit('decide', 'correction')
    showCorrection.value = true
  }
}

const cardStyle = computed(() => ({
  background: props.commitStatus === 'committed' ? 'var(--color-surface-0)' : 'var(--color-surface-1)',
  border: `1px solid ${
    props.decision === 'accept' ? 'rgba(74,222,128,0.30)' :
    props.decision === 'reject' ? 'rgba(248,113,113,0.30)' :
    props.decision === 'correction' ? 'rgba(245,197,24,0.30)' :
    'rgba(255,255,255,0.07)'
  }`,
  borderRadius: '10px', padding: '18px 20px',
  opacity: props.commitStatus === 'committed' ? 0.55 : 1,
  transition: 'all 200ms ease',
}))

const pill = (color: string, bg: string) => ({
  display: 'inline-block', fontSize: '10px', fontWeight: 700,
  padding: '3px 9px', borderRadius: '999px', background: bg, color,
  fontFamily: "'DM Sans',sans-serif", letterSpacing: '0.04em', textTransform: 'uppercase',
})
const committingPill = computed(() => ({
  ...pill('var(--color-amber)', 'rgba(245,197,24,0.10)'),
  display: 'inline-flex', alignItems: 'center', gap: '6px',
}))
const thumbBtn = (color: string, active: boolean) => ({
  display: 'inline-flex', alignItems: 'center', gap: '6px',
  padding: '6px 11px', borderRadius: '999px',
  background: active ? `${color}15` : 'transparent',
  border: `1px solid ${active ? color : 'rgba(255,255,255,0.10)'}`,
  color: active ? color : 'var(--color-gray-300)',
  fontFamily: 'var(--font-body)', fontSize: '12px', fontWeight: 600,
  cursor: 'pointer', transition: 'all 120ms ease',
})
const idxBadge = { width: '22px', height: '22px', borderRadius: '5px', background: 'rgba(245,197,24,0.10)', color: 'var(--color-amber)', fontWeight: 700, fontFamily: 'var(--font-mono)', fontSize: '11px', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0, marginTop: '1px' }
const titleStyle = { fontSize: '15.5px', fontWeight: 600, color: 'var(--color-fg-primary)', fontFamily: 'var(--font-body)', letterSpacing: '-0.005em', lineHeight: 1.4 }
const sectionLabel = { fontSize: '10px', fontWeight: 700, letterSpacing: '0.10em', textTransform: 'uppercase', color: 'var(--color-gray-400)', fontFamily: 'var(--font-body)', marginBottom: '4px' }
const prdText = { fontSize: '12.5px', color: 'var(--color-gray-200)', lineHeight: 1.6, fontStyle: 'italic', fontFamily: 'var(--font-body)' }
const howToText = { fontSize: '13px', color: 'var(--color-gray-100)', lineHeight: 1.65, fontFamily: 'var(--font-body)' }
const filesBox = { background: 'var(--color-surface-0)', border: '1px solid rgba(255,255,255,0.05)', borderRadius: '6px', overflow: 'hidden' }
const fileRow = { display: 'flex', alignItems: 'center', gap: '10px', padding: '7px 12px', fontFamily: 'var(--font-mono)', fontSize: '11.5px' }
const fileNote = { color: 'var(--color-gray-400)', fontStyle: 'italic', fontFamily: 'var(--font-body)', fontSize: '11px', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', maxWidth: '40%' }
const correctionInput = { width: '100%', minHeight: '70px', padding: '10px 12px', background: 'var(--color-surface-0)', border: '1px solid rgba(245,197,24,0.25)', borderRadius: '6px', color: 'var(--color-gray-100)', fontFamily: 'var(--font-body)', fontSize: '13px', lineHeight: 1.6, outline: 'none', resize: 'vertical' } as Record<string, any>
const amendBtn = { display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '8px 14px', borderRadius: '6px', background: 'var(--color-amber)', color: 'var(--color-surface-0)', border: 'none', cursor: 'pointer', fontFamily: 'var(--font-body)', fontWeight: 700, fontSize: '12.5px' }
const committingBar = { display: 'flex', alignItems: 'center', gap: '10px', padding: '12px 14px', background: 'rgba(245,197,24,0.06)', border: '1px solid rgba(245,197,24,0.20)', borderRadius: '6px', fontFamily: 'var(--font-body)', fontSize: '12.5px', color: 'var(--color-amber)' }
</script>
