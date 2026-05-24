<template>
  <div v-if="!task" :style="empty">
    <AppIcon name="file-text" :size="24" color="#2A2A2A" :extra-style="{ marginBottom: '10px' }" />
    Pick a detail on the left.
  </div>
  <div v-else class="fade-in" style="flex:1;overflow:auto;padding:24px 32px 40px">
    <div :style="heading">{{ task.title }}</div>
    <div :style="metaRow">
      <span :style="detailReadyPill">Detail ready</span>
      <span style="color:#2A2A2A">·</span>
      <span style="white-space:nowrap">~{{ task.estTokens }} tokens budgeted</span>
      <span style="color:#2A2A2A">·</span>
      <span style="white-space:nowrap">task #{{ task.idx }}</span>
    </div>

    <div v-for="(s, i) in sections" :key="i" style="margin-bottom:24px">
      <div :style="sectionTitle">{{ s.h }}</div>
      <div v-if="s.body" :style="bodyText">{{ s.body }}</div>
      <div v-if="s.isFiles" :style="filesBox">
        <div
          v-for="(f, j) in task.files"
          :key="j"
          :style="{ ...fileRow, borderBottom: j < task.files.length - 1 ? '1px solid rgba(255,255,255,0.04)' : 'none' }"
        >
          <AppIcon :name="f.range === 'new file' ? 'file-plus' : 'file-pen-line'" :size="12" :extra-style="{ color: f.range === 'new file' ? '#4ADE80' : '#F5C518', flexShrink:0, marginTop:'2px' }" />
          <div style="flex:1;min-width:0">
            <div style="color:#E0E0E0">{{ f.path }}</div>
            <div style="color:#555;font-size:10.5px;margin-top:2px">{{ f.range }} · <span style="font-family:'DM Sans',sans-serif;font-style:italic">{{ f.note }}</span></div>
          </div>
        </div>
      </div>
      <ul v-if="s.list" style="padding-left:0;list-style:none;display:flex;flex-direction:column;gap:7px">
        <li v-for="(it, j) in s.list" :key="j" :style="listItem">
          <span style="color:#F5C518;margin-top:8px;width:4px;height:4px;border-radius:50%;background:#F5C518;flex-shrink:0;display:inline-block"></span>
          {{ it }}
        </li>
      </ul>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Task } from '../../types'
import AppIcon from '../AppIcon.vue'

const props = defineProps<{ task: (Task & { idx: number }) | null }>()

const sections = computed(() => {
  if (!props.task) return []
  return [
    { h: 'Summary', body: props.task.howTo },
    { h: 'Patch surface', isFiles: true },
    {
      h: 'Acceptance', list: [
        'Requests exceeding the bucket return 429 within ~5ms of the decision.',
        'Headers reflect post-decrement state, never pre-state.',
        'Redis outage degrades to fail-open with a structured warning log.',
        'Unit tests cover boundary (limit-1, limit, limit+1) for both anon & authed buckets.',
      ],
    },
    {
      h: 'Dependencies', list: [
        `Recon entry: internal/middleware/* (indexed, branch=feat/ratelimit)`,
        `Recon entry: internal/transport/response.go (indexed)`,
        `Redis client already wired through pkg/redis — no new dep.`,
      ],
    },
  ]
})

const empty = { flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', color: '#3D3D3D', fontFamily: "'DM Sans',sans-serif", fontSize: '13px', padding: '40px', textAlign: 'center' }
const heading = { fontFamily: "'Comfortaa', sans-serif", fontSize: '22px', fontWeight: 700, color: '#fff', letterSpacing: '-0.02em', marginBottom: '6px' }
const metaRow = { display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '22px', fontFamily: "'JetBrains Mono','Courier Prime',monospace", fontSize: '11px', color: '#717171', flexWrap: 'wrap' }
const detailReadyPill = { display: 'inline-block', fontSize: '10px', fontWeight: 700, padding: '3px 9px', borderRadius: '999px', background: 'rgba(74,222,128,0.10)', color: '#4ADE80', fontFamily: "'DM Sans',sans-serif", letterSpacing: 0, textTransform: 'none' }
const sectionTitle = { fontSize: '10px', fontWeight: 700, letterSpacing: '0.12em', textTransform: 'uppercase', color: '#F5C518', fontFamily: "'DM Sans',sans-serif", marginBottom: '8px' }
const bodyText = { fontSize: '13.5px', color: '#D4D4D4', lineHeight: 1.7, fontFamily: "'DM Sans',sans-serif" }
const filesBox = { background: '#0A0A0A', border: '1px solid rgba(255,255,255,0.05)', borderRadius: '6px', overflow: 'hidden' }
const fileRow = { display: 'flex', alignItems: 'flex-start', gap: '10px', padding: '10px 14px', fontFamily: "'JetBrains Mono','Courier Prime',monospace", fontSize: '12px' }
const listItem = { display: 'flex', alignItems: 'flex-start', gap: '8px', fontSize: '13px', color: '#B0B0B0', lineHeight: 1.6, fontFamily: "'DM Sans',sans-serif" }
</script>
