<template>
  <div :style="stickyWrap">
    <div :style="barStyle">
      <!-- Progress + counts -->
      <div style="flex:1;min-width:0">
        <div style="display:flex;align-items:center;gap:10px;margin-bottom:8px;font-family:'DM Sans',sans-serif">
          <span style="font-size:12.5px;font-weight:600;color:var(--color-gray-100)">Review progress</span>
          <span style="font-size:11px;color:var(--color-gray-400);font-family:'JetBrains Mono','Courier Prime',monospace">{{ reviewedCount }}/{{ total }}</span>
          <span style="flex:1" />
          <span v-if="acceptedCount > 0" :style="countChip('var(--color-success)')">▲ {{ acceptedCount }}</span>
          <span v-if="rejectedCount > 0" :style="countChip('var(--color-error)')">▼ {{ rejectedCount }}</span>
        </div>

        <!-- Progress bar -->
        <div style="height:4px;border-radius:99px;background:var(--color-gray-800);display:flex;overflow:hidden">
          <div :style="{ width: `${(acceptedCount/total)*100}%`, background:'var(--color-success)', transition:'width 200ms ease' }" />
          <div :style="{ width: `${(rejectedCount/total)*100}%`, background:'var(--color-error)', transition:'width 200ms ease' }" />
        </div>

        <!-- Hint / blocker -->
        <div :style="hintStyle">
          <span v-if="blocker">ⓘ</span>
          {{ blocker || (canGenerate ? 'Ready to distill implementation details.' : '') }}
        </div>
      </div>

      <!-- Generate button -->
      <button
        :disabled="!canGenerate"
        :style="genBtn"
        @click="$emit('generate')"
      >
        <Spinner v-if="inBatch" :size="13" />
        <span v-else style="font-size:13px">✦</span>
        {{ cta }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import Spinner from './Spinner.vue'

const props = defineProps<{
  total: number
  reviewedCount: number
  acceptedCount: number
  rejectedCount: number
  blocker: string | null
  canGenerate: boolean
  batchMode: boolean
  committing: string | null
  committedCount: number
}>()

defineEmits<{ 'generate': [] }>()

const inBatch = computed(() => props.batchMode || !!props.committing)

const cta = computed(() => {
  if (inBatch.value) return `Distilling ${props.committedCount + (props.committing ? 1 : 0)}/${props.acceptedCount}`
  if (props.acceptedCount === 0 && props.reviewedCount === props.total && props.total > 0) return 'Finish: nothing to plan'
  if (props.acceptedCount > 0) return `Make implementation detail${props.acceptedCount !== 1 ? 's' : ''} (${props.acceptedCount})`
  return 'Decide on each task first'
})

const stickyWrap = {
  position: 'sticky', bottom: 0,
  marginTop: '-100px', marginLeft: '-32px', marginRight: '-32px',
  padding: '14px 32px 20px',
  background: 'linear-gradient(180deg, rgba(10,10,10,0) 0%, rgba(10,10,10,0.95) 25%, var(--color-surface-0) 60%)',
  zIndex: 5,
} as Record<string, any>
const barStyle = computed(() => ({
  background: 'var(--color-gray-900)',
  border: `1px solid ${props.canGenerate ? 'rgba(245,197,24,0.30)' : 'rgba(255,255,255,0.08)'}`,
  borderRadius: '10px', padding: '14px 18px',
  display: 'flex', alignItems: 'center', gap: '16px',
  boxShadow: props.canGenerate ? '0 0 0 4px rgba(245,197,24,0.06), 0 8px 20px rgba(0,0,0,0.4)' : '0 6px 18px rgba(0,0,0,0.45)',
  transition: 'all 180ms ease',
}))
const genBtn = computed(() => ({
  display: 'inline-flex', alignItems: 'center', gap: '8px',
  padding: '11px 18px', borderRadius: '8px',
  background: inBatch.value ? 'rgba(245,197,24,0.10)' : props.canGenerate ? 'var(--color-amber)' : 'var(--color-gray-800)',
  color: inBatch.value ? 'var(--color-amber)' : props.canGenerate ? 'var(--color-surface-0)' : 'var(--color-gray-500)',
  border: inBatch.value ? '1px solid rgba(245,197,24,0.25)' : 'none',
  cursor: props.canGenerate ? 'pointer' : 'not-allowed',
  fontFamily: 'var(--font-body)', fontWeight: 700, fontSize: '13px',
  whiteSpace: 'nowrap', flexShrink: 0, transition: 'all 150ms ease',
}))

const hintStyle = computed(() => ({
  marginTop: '8px', fontSize: '11.5px',
  color: props.blocker ? 'var(--color-amber)' : 'var(--color-gray-600)',
  fontFamily: 'var(--font-body)',
  lineHeight: 1.4, display: 'flex', alignItems: 'center', gap: '6px',
}))

const countChip = (color: string) => ({
  display: 'inline-flex', alignItems: 'center', gap: '4px',
  padding: '2px 8px', borderRadius: '999px',
  background: `${color}15`, color,
  fontSize: '11px', fontWeight: 700, fontFamily: "'DM Sans',sans-serif",
})
</script>
