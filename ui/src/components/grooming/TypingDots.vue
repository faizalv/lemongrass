<template>
  <span style="display:inline-flex;gap:3px">
    <span :style="dot(0)" />
    <span :style="dot(0.2)" />
    <span :style="dot(0.4)" />
  </span>
</template>

<script setup lang="ts">
function dot(delay: number) {
  return {
    width: '4px', height: '4px', borderRadius: '50%', background: '#F5C518',
    animation: `dots 1.2s ease-in-out ${delay}s infinite`,
    display: 'inline-block',
  }
}
</script>
