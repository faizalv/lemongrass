<template>
  <div class="fade-in" :style="card">
    <div style="display:flex;align-items:center;gap:8px;margin-bottom:8px">
      <AppIcon name="key-round" :size="14" color="#F5C518" />
      <span :style="eyebrow">Permission needed</span>
    </div>
    <div :style="title">
      Run recon for <span style="color:#F5C518">{{ pathInfo.path }}</span>?
    </div>
    <div :style="desc">
      I need this module's semantic map to plan against it.
      <strong style="color:#fff">{{ pathInfo.fileCount }} files</strong>, est.
      <strong style="color:#fff">{{ pathInfo.estTokens }}</strong>. One-time cost — it'll cache and only re-index when files change.
    </div>

    <div :style="preview">
      <div v-for="(f, i) in pathInfo.preview" :key="i">
        <span style="color:#555">·</span> {{ f }}
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
  textTransform: 'uppercase', color: '#F5C518', fontFamily: "'DM Sans',sans-serif",
}
const title = {
  fontFamily: "'Comfortaa', sans-serif", fontSize: '17px', fontWeight: 700,
  color: '#fff', marginBottom: '6px', letterSpacing: '-0.01em',
}
const desc = {
  fontSize: '13px', color: '#B0B0B0', lineHeight: 1.6, marginBottom: '14px',
  fontFamily: "'DM Sans',sans-serif",
}
const preview = {
  background: '#0A0A0A', border: '1px solid rgba(255,255,255,0.06)',
  borderRadius: '6px', padding: '10px 12px', marginBottom: '14px',
  fontFamily: "'JetBrains Mono','Courier Prime',monospace",
  fontSize: '11.5px', color: '#9A9A9A', lineHeight: 1.8,
}
const btnPrimary = {
  display: 'inline-flex', alignItems: 'center', gap: '6px',
  padding: '8px 16px', borderRadius: '6px',
  background: '#F5C518', color: '#0A0A0A',
  border: 'none', cursor: 'pointer',
  fontFamily: "'DM Sans',sans-serif", fontWeight: 700, fontSize: '13px',
}
const btnGhost = {
  padding: '8px 16px', borderRadius: '6px',
  background: 'transparent', color: '#B0B0B0',
  border: '1px solid rgba(255,255,255,0.12)', cursor: 'pointer',
  fontFamily: "'DM Sans',sans-serif", fontWeight: 500, fontSize: '13px',
}
</script>
