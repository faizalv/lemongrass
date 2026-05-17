<template>
  <svg
    :width="size"
    :height="size"
    viewBox="0 0 24 24"
    fill="none"
    :stroke="color || 'currentColor'"
    stroke-width="2"
    stroke-linecap="round"
    stroke-linejoin="round"
    :style="svgStyle"
    aria-hidden="true"
  ><g v-html="paths" /></svg>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { ICON_PATHS } from '../data/icons'

const props = defineProps<{
  name: string
  size?: number
  color?: string
  extraStyle?: Record<string, string | number>
}>()

const paths = computed(() => ICON_PATHS[props.name] ?? '')

const svgStyle = computed(() => ({
  display: 'inline-block',
  verticalAlign: 'middle',
  flexShrink: 0,
  ...props.extraStyle,
}))
</script>
