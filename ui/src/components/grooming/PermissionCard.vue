<template>
  <div class="fade-in" :style="card">
    <div style="display:flex;align-items:center;gap:8px;margin-bottom:8px">
      <AppIcon name="key-round" :size="14" color="var(--color-amber)" />
      <span :style="eyebrow">Permission needed</span>
    </div>
    <div :style="title">
      Run recon for <span style="color:var(--color-amber)">{{ pathInfo.path }}</span>?
    </div>
    <div :style="desc">
      I need this module's semantic map to plan against it.
      <strong style="color:var(--color-fg-primary)">{{ pathInfo.fileCount }} files</strong>, est.
      <strong style="color:var(--color-fg-primary)">{{ pathInfo.estTokens }}</strong>. One-time cost. It'll cache and only re-index when files change.
    </div>

    <div :style="preview">
      <div v-for="(f, i) in pathInfo.preview" :key="i">
        <span style="color:var(--color-gray-500)">·</span> {{ f }}
      </div>
    </div>

    <div style="display:flex;gap:8px">
      <button :style="btnPrimary" @click="$emit('approve')">
        <AppIcon name="check" :size="13" />
        Run recon
      </button>
      <button :style="btnGhost" @click="$emit('skip')">Skip this module</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import AppIcon from '../AppIcon.vue'

defineProps<{
  pathInfo: { path: string; fileCount: number; estTokens: string; preview: string[] }
}>()
defineEmits<{ 'approve': []; 'skip': [] }>()

const card = {
  marginTop: '16px', padding: '18px 20px',
  background: 'rgba(245,197,24,0.05)', border: '1px solid rgba(245,197,24,0.30)',
  borderRadius: '10px',
}
const eyebrow = {
  fontSize: '10px', fontWeight: 700, letterSpacing: '0.12em',
  textTransform: 'uppercase', color: 'var(--color-amber)', fontFamily: 'var(--font-body)',
}
const title = {
  fontFamily: 'var(--font-display)', fontSize: '17px', fontWeight: 700,
  color: 'var(--color-fg-primary)', marginBottom: '6px', letterSpacing: '-0.01em',
}
const desc = {
  fontSize: '13px', color: 'var(--color-gray-200)', lineHeight: 1.6, marginBottom: '14px',
  fontFamily: 'var(--font-body)',
}
const preview = {
  background: 'var(--color-surface-0)', border: '1px solid rgba(255,255,255,0.06)',
  borderRadius: '6px', padding: '10px 12px', marginBottom: '14px',
  fontFamily: 'var(--font-mono)',
  fontSize: '11.5px', color: 'var(--color-gray-300)', lineHeight: 1.8,
}
const btnPrimary = {
  display: 'inline-flex', alignItems: 'center', gap: '6px',
  padding: '8px 16px', borderRadius: '6px',
  background: 'var(--color-amber)', color: 'var(--color-surface-0)',
  border: 'none', cursor: 'pointer',
  fontFamily: 'var(--font-body)', fontWeight: 700, fontSize: '13px',
}
const btnGhost = {
  padding: '8px 16px', borderRadius: '6px',
  background: 'transparent', color: 'var(--color-gray-200)',
  border: '1px solid rgba(255,255,255,0.12)', cursor: 'pointer',
  fontFamily: 'var(--font-body)', fontWeight: 500, fontSize: '13px',
}
</script>
