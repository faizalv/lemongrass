<template>
  <div :style="overlay" @mousedown.self="$emit('close')">
    <div :style="panel">
      <!-- Header -->
      <div :style="header">
        <div :style="iconWrap">
          <AppIcon name="gauge" :size="16" :extra-style="{ color: 'var(--color-amber)' }" />
        </div>
        <div style="flex:1">
          <div :style="titleStyle">Usage</div>
          <div :style="subStyle">Claude Code · current limits</div>
        </div>
        <button :style="closeBtn" @click="$emit('close')">
          <AppIcon name="x" :size="18" />
        </button>
      </div>

      <!-- Body -->
      <div :style="body">
        <div v-if="loading" :style="loadingState">
          <Spinner :size="16" />
          <span>Fetching usage…</span>
        </div>

        <template v-else>
          <!-- Session -->
          <div :style="block">
            <div style="display:flex;align-items:baseline;justify-content:space-between;margin-bottom:8px">
              <div :style="blockLabel">Current session</div>
              <div :style="pctLabel">{{ data.session_pct }}%</div>
            </div>
            <div :style="barTrack">
              <div :style="barFill(data.session_pct)" />
            </div>
            <div v-if="data.session_resets" :style="resetLabel">Resets {{ localizeResetTime(data.session_resets) }}</div>
          </div>

          <!-- Week -->
          <div :style="block">
            <div style="display:flex;align-items:baseline;justify-content:space-between;margin-bottom:8px">
              <div :style="blockLabel">Current week <span :style="allModels">(all models)</span></div>
              <div :style="pctLabel">{{ data.week_pct }}%</div>
            </div>
            <div :style="barTrack">
              <div :style="barFill(data.week_pct)" />
            </div>
            <div v-if="data.week_resets" :style="resetLabel">Resets {{ localizeResetTime(data.week_resets) }}</div>
          </div>
        </template>

        <!-- Footer -->
        <div :style="footerBox">
          <strong style="color:var(--color-gray-300)">Lemongrass · v0.0.1</strong><br/>
          AGPL-3.0 licensed. &nbsp;
          <a href="#" style="color:var(--color-amber);text-decoration:none">github.com/faizalv/lemongrass</a>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import AppIcon from '../AppIcon.vue'
import Spinner from '../grooming/Spinner.vue'

defineEmits<{ 'close': [] }>()

interface UsageData {
  session_pct: number
  session_resets: string
  week_pct: number
  week_resets: string
}

const loading = ref(true)
const data = ref<UsageData>({ session_pct: 0, session_resets: '', week_pct: 0, week_resets: '' })
async function fetchUsage() {
  try {
    const r = await fetch('/api/lg/usage')
    if (r.ok) data.value = await r.json()
  } catch { /* ignore */ } finally {
    loading.value = false
  }
}

function localizeResetTime(s: string): string {
  if (!s || !s.toLowerCase().includes('utc')) return s
  const clean = s.replace(/\s*\(UTC\)/i, '').trim()
  const now = new Date()
  let utcDate: Date | null = null

  const withDate = clean.match(/^([A-Za-z]+)\s+(\d+),?\s+(.+)/)
  if (withDate) {
    utcDate = new Date(`${withDate[1]} ${withDate[2]} ${now.getFullYear()} ${withDate[3]} UTC`)
  } else {
    utcDate = new Date(`${now.toISOString().slice(0, 10)}T${to24h(clean)}:00Z`)
    if (utcDate <= now) utcDate.setDate(utcDate.getDate() + 1)
  }

  if (!utcDate || isNaN(utcDate.getTime())) return s

  const opts: Intl.DateTimeFormatOptions = {
    hour: 'numeric', minute: '2-digit', timeZoneName: 'short',
    ...(withDate ? { month: 'short', day: 'numeric' } : {}),
  }
  return utcDate.toLocaleString([], opts)
}

function to24h(t: string): string {
  const m = t.match(/^(\d+)(?::(\d+))?\s*(am|pm)$/i)
  if (!m) return '00:00'
  let h = parseInt(m[1])
  const min = m[2] ?? '00'
  const ap = m[3].toLowerCase()
  if (ap === 'pm' && h !== 12) h += 12
  if (ap === 'am' && h === 12) h = 0
  return `${String(h).padStart(2, '0')}:${min}`
}

onMounted(() => { fetchUsage() })

function barFill(pct: number) {
  const color = pct >= 80 ? 'var(--color-error)' : pct >= 60 ? 'var(--color-amber)' : 'var(--color-info)'
  return {
    height: '100%',
    width: pct + '%',
    background: color,
    borderRadius: '3px',
    transition: 'width 400ms ease',
  }
}

const overlay = {
  position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.7)',
  backdropFilter: 'blur(6px)', zIndex: 300,
  display: 'flex', alignItems: 'center', justifyContent: 'center',
  padding: '24px', animation: 'lgFadeIn 160ms ease',
} as Record<string, any>
const panel = {
  background: 'var(--color-gray-900)', border: '1px solid rgba(255,255,255,0.10)',
  borderRadius: '12px', width: '100%', maxWidth: '480px',
  boxShadow: '0 24px 64px rgba(0,0,0,0.7)',
  display: 'flex', flexDirection: 'column',
} as Record<string, any>
const header = {
  padding: '22px 26px 18px', borderBottom: '1px solid rgba(255,255,255,0.07)',
  display: 'flex', alignItems: 'center', gap: '12px',
}
const iconWrap = {
  width: '32px', height: '32px', borderRadius: '8px',
  background: 'rgba(245,197,24,0.10)',
  display: 'flex', alignItems: 'center', justifyContent: 'center',
}
const titleStyle = {
  fontFamily: 'var(--font-display)', fontSize: '20px', fontWeight: 700,
  color: 'var(--color-fg-primary)', letterSpacing: '-0.02em',
}
const subStyle   = { fontSize: '12px', color: 'var(--color-gray-400)', fontFamily: 'var(--font-body)' }
const closeBtn   = { background: 'transparent', border: 'none', color: 'var(--color-gray-400)', cursor: 'pointer', padding: '6px' }
const body       = { padding: '20px 26px 18px' }
const loadingState = { display: 'flex', alignItems: 'center', gap: '10px', fontSize: '13px', color: 'var(--color-gray-500)', fontFamily: 'var(--font-body)', padding: '24px 0' }

const block      = { marginBottom: '24px' }
const blockLabel = { fontSize: '13px', fontWeight: 600, color: 'var(--color-gray-200)', fontFamily: 'var(--font-body)' }
const allModels  = { fontSize: '11px', color: 'var(--color-gray-500)', fontWeight: 400 }
const pctLabel   = { fontFamily: 'var(--font-mono)', fontSize: '13px', fontWeight: 700, color: 'var(--color-fg-primary)' }
const barTrack   = { height: '6px', background: 'rgba(255,255,255,0.08)', borderRadius: '3px', overflow: 'hidden' }
const resetLabel = { fontSize: '11px', color: 'var(--color-gray-600)', fontFamily: 'var(--font-body)', marginTop: '6px' }
const footerBox  = {
  marginTop: '8px', padding: '12px 14px',
  background: 'rgba(255,255,255,0.02)', border: '1px solid rgba(255,255,255,0.05)',
  borderRadius: '8px', fontSize: '11.5px', color: 'var(--color-gray-400)', lineHeight: 1.6,
  fontFamily: 'var(--font-body)',
}
</script>
