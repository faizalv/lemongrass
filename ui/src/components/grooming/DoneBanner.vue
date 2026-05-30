<template>
  <div class="fade-in" :style="banner">
    <div :style="iconCircle">
      <AppIcon name="check-check" :size="18" :extra-style="{ color: 'var(--color-success)' }" />
    </div>
    <div style="flex:1">
      <div :style="title">Grooming complete</div>
      <div :style="sub">Every accepted task has an implementation detail. The plan tab can take it from here.</div>
    </div>
    <button :style="btnGhost" @click="$emit('reset')">Start over</button>
    <button :style="btnPrimary" @click="$emit('continue')">
      Continue to planning
      <AppIcon name="arrow-right" :size="13" />
    </button>
  </div>
</template>

<script setup lang="ts">
import AppIcon from '../AppIcon.vue'

defineEmits<{ 'reset': []; 'continue': [] }>()

const banner = { margin: '0 32px 24px', padding: '18px 22px', background: 'linear-gradient(180deg, rgba(74,222,128,0.06), rgba(74,222,128,0.02))', border: '1px solid rgba(74,222,128,0.25)', borderRadius: '10px', display: 'flex', alignItems: 'center', gap: '16px' }
const iconCircle = { width: '36px', height: '36px', borderRadius: '50%', background: 'rgba(74,222,128,0.12)', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }
const title = { fontFamily: 'var(--font-display)', fontSize: '16px', fontWeight: 700, color: 'var(--color-fg-primary)', letterSpacing: '-0.01em', marginBottom: '2px' }
const sub = { fontSize: '12.5px', color: 'var(--color-gray-300)', fontFamily: 'var(--font-body)', lineHeight: 1.5 }
const btnGhost = { padding: '8px 14px', borderRadius: '6px', background: 'transparent', color: 'var(--color-gray-300)', border: '1px solid rgba(255,255,255,0.10)', cursor: 'pointer', fontFamily: 'var(--font-body)', fontWeight: 500, fontSize: '12.5px' }
const btnPrimary = { display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '8px 16px', borderRadius: '6px', background: 'var(--color-amber)', color: 'var(--color-surface-0)', border: 'none', cursor: 'pointer', fontFamily: 'var(--font-body)', fontWeight: 700, fontSize: '13px' }
</script>
