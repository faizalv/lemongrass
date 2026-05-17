<template>
  <div class="fade-in" :style="wrap">
    <div style="display:flex;align-items:center;gap:10px;margin-bottom:4px">
      <Spinner :size="16" />
      <div :style="titleStyle">{{ title }}</div>
    </div>
    <div v-if="subtitle" :style="subtitleStyle">{{ subtitle }}</div>

    <div :style="logBox">
      <div
        v-for="(step, i) in steps"
        :key="i"
        :style="{ display:'flex', alignItems:'flex-start', gap:'8px', color: stepColor(step.status), padding:'2px 0' }"
      >
        <span :style="{ width:'12px', color: stepMarkerColor(step.status), fontWeight:700, flexShrink:0, lineHeight:1.9 }">
          {{ step.status === 'pending' ? '·' : step.status === 'ok' ? '✓' : '!' }}
        </span>
        <span style="flex:1;min-width:0">
          {{ step.label }}
          <span v-if="step.detail" style="color:#555;margin-left:8px;font-style:italic">{{ step.detail }}</span>
        </span>
        <span v-if="step.status === 'pending' && i === steps.length - 1" style="padding-top:8px;flex-shrink:0">
          <TypingDots />
        </span>
      </div>
    </div>

    <slot name="footer" />
  </div>
</template>

<script setup lang="ts">
import type { ReconStep } from '../../types'
import Spinner from './Spinner.vue'
import TypingDots from './TypingDots.vue'

defineProps<{
  title: string
  subtitle?: string
  steps: ReconStep[]
}>()

function stepColor(status: string) {
  if (status === 'ok') return '#C4C4C4'
  if (status === 'miss') return '#F5C518'
  return '#717171'
}
function stepMarkerColor(status: string) {
  if (status === 'ok') return '#4ADE80'
  if (status === 'miss') return '#F5C518'
  return '#555'
}

const wrap = { maxWidth: '760px', margin: '32px auto 0', padding: '0 32px 40px' }
const titleStyle = {
  fontFamily: "'Comfortaa', sans-serif", fontSize: '22px', fontWeight: 700,
  color: '#fff', letterSpacing: '-0.02em',
}
const subtitleStyle = {
  fontSize: '13.5px', color: '#9A9A9A', marginBottom: '24px',
  fontFamily: "'DM Sans',sans-serif", lineHeight: 1.6,
}
const logBox = {
  background: '#0E0E0E', border: '1px solid rgba(255,255,255,0.06)',
  borderRadius: '8px', padding: '14px 18px',
  fontFamily: "'JetBrains Mono','Courier Prime',monospace",
  fontSize: '12.5px', lineHeight: 1.9,
}
</script>
