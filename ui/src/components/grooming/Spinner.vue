<template>
  <svg
    :width="size"
    :height="size"
    viewBox="0 0 24 24"
    fill="none"
    :stroke="color"
    stroke-width="2.2"
    stroke-linecap="round"
    stroke-linejoin="round"
    class="spin"
    style="flex-shrink:0"
  >
    <path d="M21 12a9 9 0 1 1-6.219-8.56"/>
  </svg>
</template>

<script setup lang="ts">
withDefaults(defineProps<{ size?: number; color?: string }>(), {
  size: 14,
  color: 'var(--color-amber)',
})
</script>
