<template>
  <div :style="overlay" @mousedown.self="$emit('close')">
    <div :style="panel">
      <!-- Header -->
      <div :style="header">
        <div :style="iconWrap">
          <AppIcon name="settings" :size="16" :extra-style="{ color: '#F5C518' }" />
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

        <SettingsRow label="Auto-run recon on attach" sub="When a project is freshly attached, recon its top-level modules without asking.">
          <Toggle :on="autoRecon" @click="autoRecon = !autoRecon" />
        </SettingsRow>

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
            <AppIcon name="check-circle-2" :size="11" :extra-style="{ color: '#4ADE80' }" />
            Connected
          </span>
        </SettingsRow>

        <SettingsRow label="Token ceiling per session" sub="Hard cap before a session hands off. Lower = more handovers, leaner context.">
          <span :style="tokenVal">48k</span>
        </SettingsRow>

        <div :style="footerBox">
          <strong style="color:#9A9A9A">Lemongrass · v0.3.0-rc2</strong><br/>
          MIT licensed.
          <a href="#" style="color:#F5C518;text-decoration:none">github.com/faizalv/lemongrass</a>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, defineComponent, h } from 'vue'
import AppIcon from '../AppIcon.vue'

defineEmits<{ 'close': [] }>()

const autoRecon = ref(true)
const parallelism = ref(4)
const theme = ref('dark')

const overlay = {
  position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.7)',
  backdropFilter: 'blur(6px)', zIndex: 300,
  display: 'flex', alignItems: 'center', justifyContent: 'center',
  padding: '24px', animation: 'lgFadeIn 160ms ease',
}
const panel = {
  background: '#111', border: '1px solid rgba(255,255,255,0.10)',
  borderRadius: '12px', width: '100%', maxWidth: '540px',
  boxShadow: '0 24px 64px rgba(0,0,0,0.7)',
  display: 'flex', flexDirection: 'column', maxHeight: '85vh',
}
const header = {
  padding: '22px 26px 18px', borderBottom: '1px solid rgba(255,255,255,0.07)',
  display: 'flex', alignItems: 'center', gap: '12px',
}
const iconWrap = {
  width: '32px', height: '32px', borderRadius: '8px',
  background: 'rgba(245,197,24,0.10)', color: '#F5C518',
  display: 'flex', alignItems: 'center', justifyContent: 'center',
}
const titleStyle = {
  fontFamily: "'Comfortaa',sans-serif", fontSize: '20px', fontWeight: 700,
  color: '#fff', letterSpacing: '-0.02em',
}
const subStyle = { fontSize: '12px', color: '#717171', fontFamily: "'DM Sans',sans-serif" }
const closeBtn = {
  background: 'transparent', border: 'none', color: '#717171', cursor: 'pointer', padding: '6px',
}
const body = { padding: '4px 26px 18px', overflow: 'auto' }
const sectionLabel = {
  fontSize: '10px', fontWeight: 700, letterSpacing: '0.12em',
  textTransform: 'uppercase', color: '#555', fontFamily: "'DM Sans',sans-serif",
  marginTop: '14px', marginBottom: '2px',
}
const segBtn = (active: boolean) => ({
  width: '32px', height: '28px', borderRadius: '5px',
  background: active ? '#F5C518' : 'transparent',
  color: active ? '#0A0A0A' : '#9A9A9A',
  border: `1px solid ${active ? '#F5C518' : 'rgba(255,255,255,0.10)'}`,
  fontFamily: "'JetBrains Mono','Courier Prime',monospace",
  fontSize: '12px', fontWeight: 700, cursor: 'pointer',
})
const themeBtn = (active: boolean) => ({
  padding: '5px 11px', borderRadius: '5px',
  background: active ? '#F5C518' : 'transparent',
  color: active ? '#0A0A0A' : '#9A9A9A',
  border: `1px solid ${active ? '#F5C518' : 'rgba(255,255,255,0.10)'}`,
  fontFamily: "'DM Sans',sans-serif", fontSize: '12px', fontWeight: 600,
  cursor: 'pointer', textTransform: 'capitalize',
})
const connectedPill = {
  display: 'inline-flex', alignItems: 'center', gap: '5px',
  padding: '4px 10px', borderRadius: '999px',
  background: 'rgba(74,222,128,0.10)', color: '#4ADE80',
  fontSize: '11px', fontWeight: 700, fontFamily: "'DM Sans',sans-serif",
}
const tokenVal = {
  fontFamily: "'JetBrains Mono','Courier Prime',monospace",
  fontSize: '13px', color: '#fff', fontWeight: 600,
}
const footerBox = {
  marginTop: '24px', padding: '12px 14px',
  background: 'rgba(255,255,255,0.02)', border: '1px solid rgba(255,255,255,0.05)',
  borderRadius: '8px', fontSize: '11.5px', color: '#717171', lineHeight: 1.6,
  fontFamily: "'DM Sans',sans-serif",
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
        h('div', { style: { fontSize: '13.5px', fontWeight: 600, color: '#E0E0E0', fontFamily: "'DM Sans',sans-serif" } }, props.label),
        props.sub ? h('div', { style: { fontSize: '12px', color: '#717171', marginTop: '2px', lineHeight: 1.5, fontFamily: "'DM Sans',sans-serif" } }, props.sub) : null,
      ]),
      slots.default?.(),
    ])
  },
})

const Toggle = defineComponent({
  props: ['on'],
  emits: ['click'],
  setup(props, { emit }) {
    return () => h('button', {
      onClick: () => emit('click'),
      style: {
        position: 'relative', width: '34px', height: '19px', borderRadius: '999px',
        background: props.on ? '#F5C518' : '#2A2A2A',
        border: 'none', cursor: 'pointer', flexShrink: 0,
        transition: 'background 150ms ease',
      },
    }, [
      h('span', {
        style: {
          position: 'absolute', top: '2.5px', left: props.on ? '17.5px' : '2.5px',
          width: '14px', height: '14px', borderRadius: '50%',
          background: props.on ? '#0A0A0A' : '#B0B0B0',
          transition: 'left 160ms cubic-bezier(0.4,0,0.2,1)',
        },
      }),
    ])
  },
})
</script>
