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
          <span v-if="step.detail" style="color:var(--color-gray-500);margin-left:8px;font-style:italic">{{ step.detail }}</span>
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
  if (status === 'ok') return 'var(--color-gray-100)'
  if (status === 'miss') return 'var(--color-amber)'
  return 'var(--color-gray-400)'
}
function stepMarkerColor(status: string) {
  if (status === 'ok') return 'var(--color-success)'
  if (status === 'miss') return 'var(--color-amber)'
  return 'var(--color-gray-500)'
}

const wrap = { maxWidth: '760px', margin: '32px auto 0', padding: '0 32px 40px' }
const titleStyle = {
  fontFamily: 'var(--font-display)', fontSize: '22px', fontWeight: 700,
  color: 'var(--color-fg-primary)', letterSpacing: '-0.02em',
}
const subtitleStyle = {
  fontSize: '13.5px', color: 'var(--color-gray-300)', marginBottom: '24px',
  fontFamily: 'var(--font-body)', lineHeight: 1.6,
}
const logBox = {
  background: 'var(--color-surface-0)', border: '1px solid rgba(255,255,255,0.06)',
  borderRadius: '8px', padding: '14px 18px',
  fontFamily: 'var(--font-mono)',
  fontSize: '12.5px', lineHeight: 1.9,
}
</script>
