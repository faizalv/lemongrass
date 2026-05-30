<template>
  <div>
    <div :style="row" @click="select">
      <button :style="chevronBtn" @click.stop="toggle">
        <svg
          v-if="node.children.length > 0"
          :style="{ transform: open ? 'rotate(90deg)' : 'rotate(0deg)', transition: 'transform 140ms ease', display: 'block' }"
          width="10" height="10" viewBox="0 0 24 24" fill="none"
          stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"
        >
          <polyline points="9 18 15 12 9 6" />
        </svg>
      </button>

      <svg width="13" height="13" viewBox="0 0 24 24" fill="none"
        :stroke="isSelected ? 'var(--color-amber)' : 'var(--color-gray-400)'"
        stroke-width="2" stroke-linecap="round" stroke-linejoin="round"
        style="flex-shrink:0"
      >
        <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
      </svg>

      <span :style="label">{{ node.name }}</span>
    </div>

    <div v-if="open && node.children.length > 0" :style="children">
      <FolderNode
        v-for="child in node.children"
        :key="child.path"
        :node="child"
        :selected-path="selectedPath"
        @select="$emit('select', $event)"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import type { FsNode } from '../types'

defineOptions({ name: 'FolderNode' })

const props = defineProps<{
  node: FsNode
  selectedPath: string
}>()

const emit = defineEmits<{ select: [path: string] }>()

const open = ref(false)
const isSelected = computed(() => props.selectedPath === props.node.path)

function toggle() {
  if (props.node.children.length > 0) open.value = !open.value
}

function select() {
  emit('select', props.node.path)
  if (props.node.children.length > 0) open.value = !open.value
}

const row = computed(() => ({
  display: 'flex', alignItems: 'center', gap: '6px',
  padding: '4px 8px 4px 0', borderRadius: '5px', cursor: 'pointer',
  background: isSelected.value ? 'rgba(245,197,24,0.10)' : 'transparent',
  border: isSelected.value ? '1px solid rgba(245,197,24,0.20)' : '1px solid transparent',
  transition: 'background 100ms',
  userSelect: 'none',
} as Record<string, any>))

const label = computed(() => ({
  flex: 1, fontSize: '13px', fontFamily: 'var(--font-body)',
  color: isSelected.value ? 'var(--color-amber)' : 'var(--color-gray-100)',
  fontWeight: isSelected.value ? 600 : 400,
  whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
}))

const chevronBtn = {
  background: 'transparent', border: 'none', cursor: 'pointer',
  padding: '0', width: '16px', height: '16px', flexShrink: 0,
  display: 'flex', alignItems: 'center', justifyContent: 'center',
  color: 'var(--color-gray-500)',
}

const children = { paddingLeft: '18px' }
</script>
