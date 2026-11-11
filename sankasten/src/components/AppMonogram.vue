<!-- SPDX-License-Identifier: Apache-2.0 -->
<!-- Copyright (C) 2026 The Sankasten Authors -->

<script setup lang="ts">
import { computed } from 'vue'
import { capsuleHue } from '../lib/capsuleHue'
import { monogram } from '../lib/monogram'

// A name's initials on a solid square in the name's hue, the same hue its capsule dot takes, so
// one name keeps one color everywhere. Decorative: the name itself is always shown beside it.
const props = withDefaults(defineProps<{ name: string; size?: 'sm' | 'md' }>(), { size: 'md' })

const initials = computed(() => monogram(props.name))
const hue = computed(() => capsuleHue(props.name))
</script>

<template>
  <span class="app-monogram" :class="[`size-${size}`, `hue-${hue}`]" aria-hidden="true">
    {{ initials }}
  </span>
</template>

<style scoped>
.app-monogram {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--radius-sm);
  background: var(--monogram-hue);
  color: var(--color-on-fill);
  font-family: var(--font-body);
  font-weight: var(--weight-bold);
  line-height: 1;
  letter-spacing: var(--tracking-snug);
}

.size-md {
  width: 20px;
  height: 20px;
  font-size: 10px;
}

.size-sm {
  width: 16px;
  height: 16px;
  font-size: 8px;
}

.hue-amber {
  --monogram-hue: var(--color-amber);
}

.hue-coral {
  --monogram-hue: var(--color-coral);
}

.hue-fuchsia {
  --monogram-hue: var(--color-fuchsia);
}

.hue-jade {
  --monogram-hue: var(--color-jade);
}

.hue-sapphire {
  --monogram-hue: var(--color-sapphire);
}

.hue-teal {
  --monogram-hue: var(--color-teal);
}

.hue-violet {
  --monogram-hue: var(--color-violet);
}
</style>
