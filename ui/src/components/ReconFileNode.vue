<template>
  <div>
    <div :style="row" @click="onRowClick">

      <!-- Chevron — identical SVG and animation to FolderNode -->
      <button :style="chevronBtn" @click.stop="toggle">
        <svg
          v-if="node.isDir && node.children.length > 0"
          :style="{ transform: isOpen ? 'rotate(90deg)' : 'rotate(0deg)', transition: 'transform 140ms ease', display: 'block' }"
          width="10" height="10" viewBox="0 0 24 24" fill="none"
          stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"
        >
          <polyline points="9 18 15 12 9 6" />
        </svg>
      </button>

      <!-- Icon: folder SVG for dirs (same path as FolderNode), coverage dot for files -->
      <svg
        v-if="node.isDir"
        width="13" height="13" viewBox="0 0 24 24" fill="none"
        stroke="#717171" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"
        style="flex-shrink:0"
      >
        <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
      </svg>
      <span v-else :style="fileDot" />

      <span :style="nameStyle">{{ node.name }}</span>

      <!-- Dir coverage fraction -->
      <span v-if="node.isDir && node.total > 0" :style="covStyle">{{ node.explored }}/{{ node.total }}</span>
    </div>

    <div v-if="isOpen && node.isDir && node.children.length > 0" :style="childWrap">
      <ReconFileNode
        v-for="child in node.children"
        :key="child.path"
        :node="child"
        :selected-file="selectedFile"
        :force-open="forceOpen"
        @select="$emit('select', $event)"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import type { ReconTreeNode } from '../types'

defineOptions({ name: 'ReconFileNode' })

const props = defineProps<{
  node:         ReconTreeNode
  selectedFile: string
  forceOpen?:   boolean
  defaultOpen?: boolean
}>()

const emit = defineEmits<{ select: [path: string] }>()

const openInternal = ref(props.defaultOpen ?? false)
const isOpen       = computed(() => props.forceOpen || openInternal.value)
const isSelected   = computed(() => !props.node.isDir && props.selectedFile === props.node.path)

function toggle() {
  if (props.node.isDir && props.node.children.length > 0) openInternal.value = !openInternal.value
}

function onRowClick() {
  if (props.node.isDir) toggle()
  else emit('select', props.node.path)
}

// ── Styles — mirror FolderNode exactly ───────────────────────────────────────

const row = computed(() => ({
  display: 'flex', alignItems: 'center', gap: '6px',
  padding: '4px 8px 4px 0', borderRadius: '5px', cursor: 'pointer',
  background: isSelected.value ? 'rgba(245,197,24,0.10)' : 'transparent',
  border:     isSelected.value ? '1px solid rgba(245,197,24,0.20)' : '1px solid transparent',
  transition: 'background 100ms',
  userSelect: 'none' as const,
}))

const nameStyle = computed(() => ({
  flex: 1,
  fontFamily: "'DM Sans',sans-serif",
  fontSize: '13px',
  color:      isSelected.value ? '#F5C518' : props.node.isDir ? '#9A9A9A' : '#D4D4D4',
  fontWeight: isSelected.value || props.node.isDir ? 600 : 400,
  whiteSpace: 'nowrap' as const, overflow: 'hidden', textOverflow: 'ellipsis',
}))

const fileDot = computed(() => {
  const { explored, total } = props.node
  const color = total === 0         ? '#2A2A2A'
    : explored === total            ? '#4ADE80'
    : explored > 0                  ? '#F5C518'
    :                                 '#2A2A2A'
  return { width: '6px', height: '6px', borderRadius: '50%', background: color, flexShrink: 0 }
})

const covStyle = computed(() => {
  const pct   = props.node.total > 0 ? props.node.explored / props.node.total : 0
  const color = pct === 1 ? '#4ADE80' : pct > 0 ? '#F5C518' : '#333'
  return { marginLeft: 'auto', flexShrink: 0, paddingRight: '4px', fontFamily: "'JetBrains Mono',monospace", fontSize: '10px', color }
})

const chevronBtn = {
  background: 'transparent', border: 'none', cursor: 'pointer',
  padding: '0', width: '16px', height: '16px', flexShrink: 0,
  display: 'flex', alignItems: 'center', justifyContent: 'center',
  color: '#555',
}

const childWrap = { paddingLeft: '18px' }
</script>
