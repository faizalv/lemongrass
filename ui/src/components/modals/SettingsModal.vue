<template>
  <div :style="overlay" @mousedown.self="$emit('close')">
    <div :style="panel">
      <!-- Header -->
      <div :style="header">
        <div :style="iconWrap">
          <AppIcon name="settings" :size="16" :extra-style="{ color: 'var(--color-amber)' }" />
        </div>
        <div style="flex:1">
          <div :style="titleStyle">Settings</div>
          <div :style="subStyle">~/.lemongrass/config.toml</div>
        </div>
        <button :style="closeBtn" @click="$emit('close')">
          <AppIcon name="x" :size="18" />
        </button>
      </div>

      <!-- Body -->
      <div :style="body">
        <div :style="sectionLabel">Workspace</div>

        <SettingsRow label="Worker parallelism" sub="Max number of file-workers the planner can run simultaneously.">
          <div style="display:flex;align-items:center;gap:6px">
            <button
              v-for="n in [1,2,4,8]"
              :key="n"
              :style="segBtn(parallelism === n)"
              @click="parallelism = n"
            >{{ n }}</button>
          </div>
        </SettingsRow>

        <SettingsRow label="Theme" sub="Dark recommended — the editor is calibrated for it.">
          <div style="display:flex;align-items:center;gap:6px">
            <button
              v-for="t in ['dark','light']"
              :key="t"
              :style="themeBtn(theme === t)"
              @click="theme = t"
            >{{ t }}</button>
          </div>
        </SettingsRow>

        <div :style="sectionLabel" style="margin-top:20px">Claude</div>

        <SettingsRow label="Authentication" sub="Riding on your existing subscription. 5-hour refresh window.">
          <span :style="connectedPill">
            <AppIcon name="check-circle-2" :size="11" :extra-style="{ color: 'var(--color-success)' }" />
            Connected
          </span>
        </SettingsRow>

        <SettingsRow label="Token ceiling per session" sub="Hard cap before a session hands off. Lower = more handovers, leaner context.">
          <span :style="tokenVal">48k</span>
        </SettingsRow>

        <div :style="footerBox">
          <strong style="color:var(--color-gray-300)">Lemongrass · v0.3.0-rc2</strong><br/>
          MIT licensed.
          <a href="#" style="color:var(--color-amber);text-decoration:none">github.com/faizalv/lemongrass</a>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, defineComponent, h } from 'vue'
import AppIcon from '../AppIcon.vue'

defineEmits<{ 'close': [] }>()

const parallelism = ref(4)
const theme = ref('dark')

const overlay = {
  position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.7)',
  backdropFilter: 'blur(6px)', zIndex: 300,
  display: 'flex', alignItems: 'center', justifyContent: 'center',
  padding: '24px', animation: 'lgFadeIn 160ms ease',
} as Record<string, any>
const panel = {
  background: 'var(--color-gray-900)', border: '1px solid rgba(255,255,255,0.10)',
  borderRadius: '12px', width: '100%', maxWidth: '540px',
  boxShadow: '0 24px 64px rgba(0,0,0,0.7)',
  display: 'flex', flexDirection: 'column', maxHeight: '85vh',
} as Record<string, any>
const header = {
  padding: '22px 26px 18px', borderBottom: '1px solid rgba(255,255,255,0.07)',
  display: 'flex', alignItems: 'center', gap: '12px',
}
const iconWrap = {
  width: '32px', height: '32px', borderRadius: '8px',
  background: 'rgba(245,197,24,0.10)', color: 'var(--color-amber)',
  display: 'flex', alignItems: 'center', justifyContent: 'center',
}
const titleStyle = {
  fontFamily: 'var(--font-display)', fontSize: '20px', fontWeight: 700,
  color: 'var(--color-fg-primary)', letterSpacing: '-0.02em',
}
const subStyle = { fontSize: '12px', color: 'var(--color-gray-400)', fontFamily: 'var(--font-body)' }
const closeBtn = {
  background: 'transparent', border: 'none', color: 'var(--color-gray-400)', cursor: 'pointer', padding: '6px',
}
const body = { padding: '4px 26px 18px', overflow: 'auto' }
const sectionLabel = {
  fontSize: '10px', fontWeight: 700, letterSpacing: '0.12em',
  textTransform: 'uppercase', color: 'var(--color-gray-500)', fontFamily: 'var(--font-body)',
  marginTop: '14px', marginBottom: '2px',
}
const segBtn = (active: boolean) => ({
  width: '32px', height: '28px', borderRadius: '5px',
  background: active ? 'var(--color-amber)' : 'transparent',
  color: active ? 'var(--color-surface-0)' : 'var(--color-gray-300)',
  border: `1px solid ${active ? 'var(--color-amber)' : 'rgba(255,255,255,0.10)'}`,
  fontFamily: 'var(--font-mono)',
  fontSize: '12px', fontWeight: 700, cursor: 'pointer',
})
const themeBtn = (active: boolean) => ({
  padding: '5px 11px', borderRadius: '5px',
  background: active ? 'var(--color-amber)' : 'transparent',
  color: active ? 'var(--color-surface-0)' : 'var(--color-gray-300)',
  border: `1px solid ${active ? 'var(--color-amber)' : 'rgba(255,255,255,0.10)'}`,
  fontFamily: 'var(--font-body)', fontSize: '12px', fontWeight: 600,
  cursor: 'pointer', textTransform: 'capitalize',
})
const connectedPill = {
  display: 'inline-flex', alignItems: 'center', gap: '5px',
  padding: '4px 10px', borderRadius: '999px',
  background: 'rgba(74,222,128,0.10)', color: 'var(--color-success)',
  fontSize: '11px', fontWeight: 700, fontFamily: 'var(--font-body)',
}
const tokenVal = {
  fontFamily: 'var(--font-mono)',
  fontSize: '13px', color: 'var(--color-fg-primary)', fontWeight: 600,
}
const footerBox = {
  marginTop: '24px', padding: '12px 14px',
  background: 'rgba(255,255,255,0.02)', border: '1px solid rgba(255,255,255,0.05)',
  borderRadius: '8px', fontSize: '11.5px', color: 'var(--color-gray-400)', lineHeight: 1.6,
  fontFamily: 'var(--font-body)',
}

const SettingsRow = defineComponent({
  props: ['label', 'sub'],
  setup(props, { slots }) {
    return () => h('div', {
      style: {
        display: 'flex', alignItems: 'center', gap: '16px',
        padding: '14px 0', borderBottom: '1px solid rgba(255,255,255,0.05)',
      },
    }, [
      h('div', { style: { flex: 1, minWidth: 0 } }, [
        h('div', { style: { fontSize: '13.5px', fontWeight: 600, color: 'var(--color-gray-100)', fontFamily: 'var(--font-body)' } }, props.label),
        props.sub ? h('div', { style: { fontSize: '12px', color: 'var(--color-gray-400)', marginTop: '2px', lineHeight: 1.5, fontFamily: 'var(--font-body)' } }, props.sub) : null,
      ]),
      slots.default?.(),
    ])
  },
})

</script>
