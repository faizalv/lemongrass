<template>
  <div :style="wrap">
    <!-- Logo mark -->
    <div :style="logoMark">
      <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="#0A0A0A" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M11 20A7 7 0 0 1 9.8 6.1C15.5 5 17 4.48 19.2 2.2c1.7 6.6.2 13.1-5 17.7"/>
        <path d="M2 21c0-3 1.85-5.36 5.08-6"/>
      </svg>
    </div>

    <!-- Wordmark -->
    <div :style="wordmark">lemongrass</div>

    <!-- Tagline -->
    <div :style="tagline">Your local AI coding agent.</div>
    <div :style="desc">
      Groom requirements, plan patches, and run tests. All from your own machine.
      Add a project to get started.
    </div>

    <!-- Primary CTA -->
    <button :style="ctaBtn" @click="$emit('add')">
      <AppIcon name="folder-git-2" :size="15" />
      Add a project
      <AppIcon name="arrow-right" :size="14" />
    </button>

    <!-- Feature hints -->
    <div :style="hints">
      <div :style="hintItem">
        <AppIcon name="radar" :size="13" color="var(--color-amber)" />
        <span>Recon maps your codebase semantically</span>
      </div>
      <div :style="hintDot" />
      <div :style="hintItem">
        <AppIcon name="layers" :size="13" color="var(--color-amber)" />
        <span>Workspaces isolate each task</span>
      </div>
      <div :style="hintDot" />
      <div :style="hintItem">
        <AppIcon name="sparkles" :size="13" color="var(--color-amber)" />
        <span>Grooming proposes tasks, you decide</span>
      </div>
    </div>

    <!-- Footer -->
    <div :style="footer">
      Open source · MIT licensed ·
      <a href="#" style="color:var(--color-amber);text-decoration:none">github.com/faizalv/lemongrass</a>
      · v0.3.0-rc2
    </div>
  </div>
</template>

<script setup lang="ts">
import AppIcon from './AppIcon.vue'

defineEmits<{ 'add': [] }>()

const wrap = {
  flex: 1,
  display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center',
  padding: '40px 24px', textAlign: 'center', gap: '0',
  background: 'var(--color-surface-0)',
  userSelect: 'none' as const,
}
const logoMark = {
  width: '56px', height: '56px', borderRadius: '14px',
  background: 'var(--color-amber)',
  display: 'flex', alignItems: 'center', justifyContent: 'center',
  marginBottom: '20px',
  boxShadow: '0 0 0 8px rgba(245,197,24,0.08), 0 0 0 16px rgba(245,197,24,0.04)',
}
const wordmark = {
  fontFamily: 'var(--font-display)',
  fontSize: '32px', fontWeight: 700, color: 'var(--color-fg-primary)',
  letterSpacing: '-0.02em', marginBottom: '12px',
}
const tagline = {
  fontFamily: 'var(--font-body)',
  fontSize: '16px', fontWeight: 600, color: 'var(--color-gray-100)',
  marginBottom: '8px',
}
const desc = {
  fontFamily: 'var(--font-body)',
  fontSize: '13.5px', color: 'var(--color-gray-400)', lineHeight: 1.7,
  maxWidth: '380px', marginBottom: '28px',
}
const ctaBtn = {
  display: 'inline-flex', alignItems: 'center', gap: '9px',
  padding: '12px 22px', borderRadius: '8px',
  background: 'var(--color-amber)', color: 'var(--color-surface-0)',
  border: 'none', cursor: 'pointer',
  fontFamily: 'var(--font-body)', fontWeight: 700, fontSize: '14px',
  marginBottom: '32px',
  boxShadow: '0 0 0 0 rgba(245,197,24,0)',
  transition: 'box-shadow 150ms ease, transform 100ms ease',
}
const hints = {
  display: 'flex', alignItems: 'center', gap: '10px',
  flexWrap: 'wrap' as const, justifyContent: 'center',
  marginBottom: '28px',
}
const hintItem = {
  display: 'inline-flex', alignItems: 'center', gap: '6px',
  fontFamily: 'var(--font-body)', fontSize: '12px', color: 'var(--color-gray-400)',
}
const hintDot = {
  width: '3px', height: '3px', borderRadius: '50%', background: 'var(--color-gray-600)',
}
const footer = {
  fontFamily: 'var(--font-body)',
  fontSize: '11.5px', color: 'var(--color-gray-600)', lineHeight: 1.6,
}
</script>
