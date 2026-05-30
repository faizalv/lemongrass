<template>
  <div v-if="!task" :style="empty">
    <AppIcon name="file-text" :size="24" color="var(--color-gray-700)" :extra-style="{ marginBottom: '10px' }" />
    Pick a detail on the left.
  </div>
  <div v-else class="fade-in" style="flex:1;overflow:auto;padding:24px 32px 40px">
    <div :style="heading">{{ task.title }}</div>
    <div :style="metaRow">
      <span :style="detailReadyPill">Detail ready</span>
      <span style="color:var(--color-gray-700)">·</span>
      <span style="white-space:nowrap">~{{ task.estTokens }} tokens budgeted</span>
      <span style="color:var(--color-gray-700)">·</span>
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
          <AppIcon :name="f.range === 'new file' ? 'file-plus' : 'file-pen-line'" :size="12" :extra-style="{ color: f.range === 'new file' ? 'var(--color-success)' : 'var(--color-amber)', flexShrink:0, marginTop:'2px' }" />
          <div style="flex:1;min-width:0">
            <div style="color:var(--color-gray-100)">{{ f.path }}</div>
            <div style="color:var(--color-gray-500);font-size:10.5px;margin-top:2px">{{ f.range }} · <span style="font-family:'DM Sans',sans-serif;font-style:italic">{{ f.note }}</span></div>
          </div>
        </div>
      </div>
      <ul v-if="s.list" style="padding-left:0;list-style:none;display:flex;flex-direction:column;gap:7px">
        <li v-for="(it, j) in s.list" :key="j" :style="listItem">
          <span style="color:var(--color-amber);margin-top:8px;width:4px;height:4px;border-radius:50%;background:var(--color-amber);flex-shrink:0;display:inline-block"></span>
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

const empty = { flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', color: 'var(--color-gray-600)', fontFamily: 'var(--font-body)', fontSize: '13px', padding: '40px', textAlign: 'center' }
const heading = { fontFamily: 'var(--font-display)', fontSize: '22px', fontWeight: 700, color: 'var(--color-fg-primary)', letterSpacing: '-0.02em', marginBottom: '6px' }
const metaRow = { display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '22px', fontFamily: 'var(--font-mono)', fontSize: '11px', color: 'var(--color-gray-400)', flexWrap: 'wrap' }
const detailReadyPill = { display: 'inline-block', fontSize: '10px', fontWeight: 700, padding: '3px 9px', borderRadius: '999px', background: 'rgba(74,222,128,0.10)', color: 'var(--color-success)', fontFamily: 'var(--font-body)', letterSpacing: 0, textTransform: 'none' }
const sectionTitle = { fontSize: '10px', fontWeight: 700, letterSpacing: '0.12em', textTransform: 'uppercase', color: 'var(--color-amber)', fontFamily: 'var(--font-body)', marginBottom: '8px' }
const bodyText = { fontSize: '13.5px', color: 'var(--color-gray-100)', lineHeight: 1.7, fontFamily: 'var(--font-body)' }
const filesBox = { background: 'var(--color-surface-0)', border: '1px solid rgba(255,255,255,0.05)', borderRadius: '6px', overflow: 'hidden' }
const fileRow = { display: 'flex', alignItems: 'flex-start', gap: '10px', padding: '10px 14px', fontFamily: 'var(--font-mono)', fontSize: '12px' }
const listItem = { display: 'flex', alignItems: 'flex-start', gap: '8px', fontSize: '13px', color: 'var(--color-gray-200)', lineHeight: 1.6, fontFamily: 'var(--font-body)' }
</script>
