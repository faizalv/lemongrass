<template>
  <div :style="root">
    <button :style="bar" @click="expanded = !expanded">
      <span :style="dotWrap">
        <span :style="dot" />
        <span v-if="current" class="lg-pulse-ring" :style="pulseRing" />
      </span>
      <span :style="barLabel">Vectorization</span>
      <span v-if="!loaded" :style="badge('idle')">–</span>
      <span v-else-if="pending > 0" :style="badge('active')">{{ pending }} pending</span>
      <span v-else :style="badge('done')">complete</span>
      <span style="flex:1" />
      <AppIcon :name="expanded ? 'chevron-up' : 'chevron-down'" :size="11" :extra-style="{ flexShrink: 0, color: 'var(--color-gray-600)' }" />
    </button>

    <div v-if="expanded" :style="panel">
      <div :style="desc">
        e5-base embeds unexplored symbols in the background. Search includes them before annotation.
      </div>

      <div :style="progressWrap">
        <div :style="progressTrack">
          <div :style="progressFill" />
        </div>
        <span :style="progressLabel">{{ embedded }} / {{ total }} embedded</span>
      </div>

      <div v-if="current" :style="currentRow">
        <div class="lg-spin" :style="spinner" />
        <span :style="currentText">{{ current }}</span>
      </div>

      <div v-if="recentList.length" :style="recentWrap">
        <div v-for="sym in recentList" :key="sym" :style="recentItem">{{ sym }}</div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import AppIcon from './AppIcon.vue'

const props = defineProps<{ projectId: string }>()

const pending    = ref(0)
const total      = ref(0)
const current    = ref('')
const recentList = ref<string[]>([])
const loaded     = ref(false)
const expanded   = ref(false)
let   timer: ReturnType<typeof setInterval> | null = null

const embedded = computed(() => total.value - pending.value)
const fraction = computed(() => total.value > 0 ? embedded.value / total.value : 0)

async function load() {
  try {
    const r = await fetch(`/api/recon/projects/${props.projectId}/embed-status`)
    if (!r.ok) return
    const data = await r.json()
    pending.value    = data.pending  ?? 0
    total.value      = data.total    ?? 0
    current.value    = data.current  ?? ''
    recentList.value = [...(data.recent ?? [])].reverse().slice(0, 8)
    loaded.value     = true
  } catch { /* ignore */ }
}

onMounted(() => {
  load()
  timer = setInterval(load, 2000)
})
onUnmounted(() => { if (timer) clearInterval(timer) })

const root = {
  borderTop: '1px solid rgba(255,255,255,0.06)',
  flexShrink: 0,
  background: 'var(--color-surface-0)',
} as Record<string, any>

const bar = {
  width: '100%',
  display: 'flex',
  alignItems: 'center',
  gap: '8px',
  padding: '9px 16px',
  background: 'transparent',
  border: 'none',
  cursor: 'pointer',
  textAlign: 'left',
} as Record<string, any>

const barLabel = {
  fontFamily: 'var(--font-body)',
  fontSize: '11.5px',
  fontWeight: 600,
  color: 'var(--color-gray-400)',
}

const dotWrap = {
  position: 'relative',
  width: '8px',
  height: '8px',
  flexShrink: 0,
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
} as Record<string, any>

const dot = computed(() => ({
  width: '6px',
  height: '6px',
  borderRadius: '50%',
  flexShrink: 0,
  position: 'relative',
  zIndex: 1,
  background: current.value
    ? 'var(--color-amber)'
    : pending.value > 0
      ? 'rgba(251,191,36,0.5)'
      : loaded.value
        ? 'var(--color-success, #34d399)'
        : 'var(--color-gray-700)',
}) as Record<string, any>)

const pulseRing = {
  position: 'absolute',
  top: '50%',
  left: '50%',
  transform: 'translate(-50%, -50%)',
  width: '12px',
  height: '12px',
  borderRadius: '50%',
  border: '1.5px solid var(--color-amber)',
  animation: 'lg-pulse 1.4s ease-out infinite',
  zIndex: 0,
} as Record<string, any>

const badge = (state: 'idle' | 'active' | 'done') => ({
  fontFamily: 'var(--font-mono)',
  fontSize: '10.5px',
  padding: '1px 7px',
  borderRadius: '999px',
  background: state === 'active'
    ? 'rgba(251,191,36,0.10)'
    : state === 'done'
      ? 'rgba(52,211,153,0.10)'
      : 'rgba(255,255,255,0.05)',
  color: state === 'active'
    ? 'var(--color-amber)'
    : state === 'done'
      ? 'var(--color-success, #34d399)'
      : 'var(--color-gray-600)',
})

const panel = {
  padding: '0 16px 14px',
  display: 'flex',
  flexDirection: 'column',
  gap: '10px',
} as Record<string, any>

const desc = {
  fontFamily: 'var(--font-body)',
  fontSize: '11.5px',
  color: 'var(--color-gray-600)',
  lineHeight: '1.5',
}

const progressWrap = {
  display: 'flex',
  alignItems: 'center',
  gap: '10px',
}

const progressTrack = {
  flex: 1,
  height: '3px',
  borderRadius: '2px',
  background: 'rgba(255,255,255,0.06)',
  overflow: 'hidden',
}

const progressFill = computed(() => ({
  height: '100%',
  borderRadius: '2px',
  width: `${Math.round(fraction.value * 100)}%`,
  background: fraction.value >= 1
    ? 'var(--color-success, #34d399)'
    : 'var(--color-amber)',
  transition: 'width 0.8s ease',
}) as Record<string, any>)

const progressLabel = {
  fontFamily: 'var(--font-mono)',
  fontSize: '11px',
  color: 'var(--color-gray-500)',
  flexShrink: 0,
}

const currentRow = {
  display: 'flex',
  alignItems: 'center',
  gap: '8px',
}

const spinner = {
  width: '10px',
  height: '10px',
  borderRadius: '50%',
  border: '1.5px solid rgba(251,191,36,0.25)',
  borderTopColor: 'var(--color-amber)',
  flexShrink: 0,
  animation: 'lg-spin 0.7s linear infinite',
}

const currentText = {
  fontFamily: 'var(--font-mono)',
  fontSize: '11px',
  color: 'var(--color-amber)',
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap',
}

const recentWrap = {
  display: 'flex',
  flexDirection: 'column',
  gap: '2px',
} as Record<string, any>

const recentItem = {
  fontFamily: 'var(--font-mono)',
  fontSize: '10.5px',
  color: 'var(--color-gray-600)',
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap',
}
</script>

<style>
@keyframes lg-pulse {
  0%   { transform: translate(-50%, -50%) scale(0.8); opacity: 0.7; }
  70%  { transform: translate(-50%, -50%) scale(2);   opacity: 0;   }
  100% { transform: translate(-50%, -50%) scale(2);   opacity: 0;   }
}
@keyframes lg-spin {
  to { transform: rotate(360deg); }
}
</style>
