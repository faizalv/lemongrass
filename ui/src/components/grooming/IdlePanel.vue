<template>
  <div class="fade-in" :style="wrap">
    <div>
      <div :style="heading">What do you want built?</div>
      <div :style="sub">
        Drop a requirement, a PRD, a Linear ticket — anything. Grooming will read the recon map,
        propose a task breakdown, and wait for your sign-off before anyone touches code.
      </div>
    </div>

    <div
      :style="{ ...inputBox, borderColor: drag ? 'rgba(245,197,24,0.45)' : 'rgba(255,255,255,0.08)' }"
      @dragover.prevent="drag = true"
      @dragleave="drag = false"
      @drop.prevent="drag = false; $emit('attach')"
    >
      <textarea
        :value="modelValue"
        :style="textarea"
        :placeholder="placeholder"
        @input="$emit('update:modelValue', ($event.target as HTMLTextAreaElement).value)"
      />
      <div :style="toolbar">
        <button :style="toolBtn" @click="$emit('attach')">
          <AppIcon name="paperclip" :size="12" />
          Attach file
        </button>
        <button :style="toolBtn" @click="$emit('use-sample')">
          <AppIcon name="wand-sparkles" :size="12" />
          Use sample
        </button>
        <span style="flex:1" />
        <span :style="charCount">{{ modelValue.length }} chars</span>
        <button
          :disabled="modelValue.trim().length < 10"
          :style="startBtn(modelValue.trim().length >= 10)"
          @click="$emit('start')"
        >
          Start grooming
          <AppIcon name="arrow-right" :size="13" />
        </button>
      </div>
    </div>

    <div v-if="attachments.length" style="display:flex;flex-wrap:wrap;gap:8px">
      <div v-for="(a, i) in attachments" :key="i" :style="attachPill">
        <AppIcon :name="a.icon" :size="11" :extra-style="{ color: '#F5C518' }" />
        {{ a.name }}
      </div>
    </div>

    <div :style="infoBox">
      <AppIcon name="info" :size="14" color="#60A5FA" :extra-style="{ flexShrink: 0, marginTop: '2px' }" />
      <div :style="infoText">
        Grooming reads your recon map first. If parts of your codebase aren't indexed yet,
        it'll ask before running recon. No code is touched in this phase — only task proposals.
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import AppIcon from '../AppIcon.vue'

defineProps<{
  modelValue: string
  attachments: { name: string; icon: string }[]
}>()

defineEmits<{
  'update:modelValue': [v: string]
  'start': []
  'attach': []
  'use-sample': []
}>()

const drag = ref(false)

const placeholder = `Paste your requirement here…\n\ne.g. "Add per-user rate limiting to the public REST API. 60 req/min for anonymous, 300 for authenticated…"`

const wrap = {
  maxWidth: '760px', margin: '40px auto 0', padding: '0 32px 40px',
  display: 'flex', flexDirection: 'column', gap: '18px',
}
const heading = {
  fontFamily: "'Comfortaa', sans-serif", fontSize: '26px', fontWeight: 700,
  color: '#fff', letterSpacing: '-0.02em', marginBottom: '8px',
}
const sub = { fontSize: '14px', color: '#9A9A9A', fontFamily: "'DM Sans',sans-serif", lineHeight: 1.6 }
const inputBox = {
  background: '#111', border: '1px solid', borderRadius: '10px',
  overflow: 'hidden', transition: 'border-color 150ms ease',
}
const textarea = {
  width: '100%', minHeight: '200px', padding: '18px 20px',
  background: 'transparent', border: 'none', outline: 'none', resize: 'vertical',
  color: '#E0E0E0', fontFamily: "'DM Sans',sans-serif", fontSize: '14px', lineHeight: 1.7,
}
const toolbar = {
  display: 'flex', alignItems: 'center', gap: '10px',
  padding: '10px 14px', borderTop: '1px solid rgba(255,255,255,0.06)', background: '#0E0E0E',
}
const toolBtn = {
  display: 'inline-flex', alignItems: 'center', gap: '6px',
  padding: '6px 10px', background: 'transparent',
  border: '1px solid rgba(255,255,255,0.10)', borderRadius: '5px',
  color: '#B0B0B0', fontFamily: "'DM Sans',sans-serif", fontSize: '12px', fontWeight: 500,
  cursor: 'pointer',
}
const charCount = {
  fontSize: '11px', color: '#555', fontFamily: "'JetBrains Mono','Courier Prime',monospace",
  whiteSpace: 'nowrap',
}
const startBtn = (enabled: boolean) => ({
  display: 'inline-flex', alignItems: 'center', gap: '7px',
  padding: '8px 16px', borderRadius: '6px',
  background: enabled ? '#F5C518' : '#2A2A2A',
  color: enabled ? '#0A0A0A' : '#555',
  border: 'none', cursor: enabled ? 'pointer' : 'not-allowed',
  fontFamily: "'DM Sans',sans-serif", fontWeight: 700, fontSize: '13px',
})
const attachPill = {
  display: 'inline-flex', alignItems: 'center', gap: '6px',
  padding: '5px 10px', background: '#1A1A1A',
  border: '1px solid rgba(255,255,255,0.07)', borderRadius: '4px',
  fontSize: '11px', color: '#B0B0B0',
  fontFamily: "'JetBrains Mono','Courier Prime',monospace",
}
const infoBox = {
  marginTop: '4px', padding: '12px 16px',
  background: 'rgba(96,165,250,0.05)', border: '1px solid rgba(96,165,250,0.18)',
  borderRadius: '8px', display: 'flex', gap: '10px',
}
const infoText = { fontSize: '12.5px', color: '#9A9A9A', lineHeight: 1.6, fontFamily: "'DM Sans',sans-serif" }
</script>
