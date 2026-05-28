<template>
  <div :style="cardStyle">
    <!-- Title row -->
    <div style="display:flex;align-items:flex-start;gap:12px;margin-bottom:14px">
      <div :style="idxBadge">{{ task.idx }}</div>
      <div style="flex:1">
        <div :style="titleStyle">{{ task.title }}</div>
      </div>
    </div>

    <!-- Directives -->
    <div style="margin-bottom:16px">
      <div :style="sectionLabel">Directives</div>
      <div :style="filesBox">
        <div
          v-for="(item, i) in task.impl"
          :key="i"
          :style="{ ...implRow, borderBottom: i < task.impl.length - 1 ? '1px solid rgba(255,255,255,0.04)' : 'none' }"
        >{{ item }}</div>
      </div>
    </div>

    <!-- Action row -->
    <div style="display:flex;align-items:center;gap:8px">
      <button :style="thumbBtn('#4ADE80', decision?.approved === true)" @click="toggleApprove">
        <AppIcon name="thumbs-up" :size="12" />
        Approve
      </button>
      <button :style="thumbBtn('#F87171', decision?.approved === false)" @click="toggleReject">
        <AppIcon name="thumbs-down" :size="12" />
        Reject
      </button>
    </div>

    <!-- Feedback textarea -->
    <div v-if="decision?.approved === false" style="margin-top:10px">
      <textarea
        :value="decision.feedback"
        :style="correctionInput"
        placeholder="What should change? (required)"
        rows="2"
        @input="onFeedback"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { ApiTask } from '../../types'
import AppIcon from '../AppIcon.vue'

const props = defineProps<{
  task: ApiTask & { idx: number }
  decision: { approved: boolean; feedback: string } | null
}>()

const emit = defineEmits<{
  'decide': [d: { approved: boolean; feedback: string } | null]
}>()

function toggleApprove() {
  emit('decide', props.decision?.approved === true ? null : { approved: true, feedback: '' })
}

function toggleReject() {
  emit('decide', props.decision?.approved === false ? null : { approved: false, feedback: '' })
}

function onFeedback(e: Event) {
  emit('decide', { approved: false, feedback: (e.target as HTMLTextAreaElement).value })
}

const cardStyle = computed(() => ({
  background: '#141414',
  border: `1px solid ${
    props.decision?.approved === true  ? 'rgba(74,222,128,0.30)' :
    props.decision?.approved === false ? 'rgba(248,113,113,0.30)' :
    'rgba(255,255,255,0.07)'
  }`,
  borderRadius: '10px', padding: '18px 20px',
  transition: 'all 200ms ease',
}))

const thumbBtn = (color: string, active: boolean) => ({
  display: 'inline-flex', alignItems: 'center', gap: '6px',
  padding: '6px 11px', borderRadius: '999px',
  background: active ? `${color}15` : 'transparent',
  border: `1px solid ${active ? color : 'rgba(255,255,255,0.10)'}`,
  color: active ? color : '#9A9A9A',
  fontFamily: "'DM Sans',sans-serif", fontSize: '12px', fontWeight: 600,
  cursor: 'pointer', transition: 'all 120ms ease',
})
const idxBadge      = { width: '22px', height: '22px', borderRadius: '5px', background: 'rgba(245,197,24,0.10)', color: '#F5C518', fontWeight: 700, fontFamily: "'JetBrains Mono','Courier Prime',monospace", fontSize: '11px', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0, marginTop: '1px' }
const titleStyle    = { fontSize: '15.5px', fontWeight: 600, color: '#fff', fontFamily: "'DM Sans',sans-serif", letterSpacing: '-0.005em', lineHeight: 1.4 }
const sectionLabel  = { fontSize: '10px', fontWeight: 700, letterSpacing: '0.10em', textTransform: 'uppercase', color: '#717171', fontFamily: "'DM Sans',sans-serif", marginBottom: '4px' }
const filesBox      = { background: '#0A0A0A', border: '1px solid rgba(255,255,255,0.05)', borderRadius: '6px', overflow: 'hidden' }
const implRow       = { padding: '6px 12px', fontFamily: "'JetBrains Mono','Courier Prime',monospace", fontSize: '11.5px', color: '#9A9A9A', lineHeight: 1.6 }
const correctionInput = { width: '100%', minHeight: '70px', padding: '10px 12px', background: '#0A0A0A', border: '1px solid rgba(248,113,113,0.25)', borderRadius: '6px', color: '#E0E0E0', fontFamily: "'DM Sans',sans-serif", fontSize: '13px', lineHeight: 1.6, outline: 'none', resize: 'vertical' as const }
</script>
