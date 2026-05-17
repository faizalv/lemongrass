<template>
  <button :style="itemStyle" @click="$emit('select')">
    <div style="display:flex;align-items:center;gap:8px">
      <span :style="idxBadge">{{ task.idx }}</span>
      <span :style="{ fontSize:'12.5px', fontWeight:600, color: active ? '#F5C518' : '#E0E0E0', flex:1, lineHeight:1.4 }">
        {{ task.title }}
      </span>
    </div>
    <div :style="meta">
      <AppIcon name="files" :size="9" />
      <span>{{ task.files.length }} file{{ task.files.length !== 1 ? 's' : '' }}</span>
      <span style="color:#2A2A2A">·</span>
      <AppIcon name="check-circle-2" :size="9" :extra-style="{ color: '#4ADE80' }" />
      <span>Ready for planning</span>
    </div>
  </button>
</template>

<script setup lang="ts">
import type { Task } from '../../types'
import AppIcon from '../AppIcon.vue'

const props = defineProps<{ task: Task & { idx: number }; active: boolean }>()
defineEmits<{ 'select': [] }>()

const itemStyle = {
  width: '100%', textAlign: 'left', border: 'none', cursor: 'pointer',
  background: props.active ? 'rgba(245,197,24,0.08)' : '#111',
  borderLeft: props.active ? '2px solid #F5C518' : '2px solid transparent',
  padding: '12px 14px', display: 'flex', flexDirection: 'column', gap: '6px',
  transition: 'background 120ms ease', fontFamily: "'DM Sans',sans-serif",
}
const idxBadge = { width: '18px', height: '18px', borderRadius: '4px', background: 'rgba(74,222,128,0.10)', color: '#4ADE80', fontWeight: 700, fontFamily: "'JetBrains Mono','Courier Prime',monospace", fontSize: '10px', display: 'flex', alignItems: 'center', justifyContent: 'center', flexShrink: 0 }
const meta = { fontFamily: "'JetBrains Mono','Courier Prime',monospace", fontSize: '10px', color: '#555', display: 'flex', alignItems: 'center', gap: '6px' }
</script>
