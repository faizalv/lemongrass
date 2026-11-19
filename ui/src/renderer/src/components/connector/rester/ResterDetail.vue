<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { onMounted, ref, toRef } from 'vue'
import { vaultSession, describeError, noteResterChange } from '../../../vaultSession'
import type { VaultRester } from '../../../../../preload/types'
import { AppButton, AppChip, AppEmptyState, AppIcon } from '@sankasten'
import EnvironmentsStrip from './EnvironmentsStrip.vue'
import AuthenticationSection from './AuthenticationSection.vue'
import UserCenter from './UserCenter.vue'
import { plainCopy } from './resterDraft'

const props = defineProps<{
  name: string
}>()

const emit = defineEmits<{
  close: []
}>()

const sessionPassphrase = toRef(vaultSession, 'passphrase')

const rester = ref<VaultRester | null>(null)

// Go encodes an empty slice as null, so the lists every child iterates are normalized once here.
function normalize(r: VaultRester): VaultRester {
  return { ...r, Environments: r.Environments ?? [], Users: r.Users ?? [] }
}
const loading = ref(true)
const loadError = ref('')

async function load(): Promise<void> {
  loading.value = true
  loadError.value = ''
  try {
    rester.value = normalize(await window.api.vault.getRester(sessionPassphrase.value, props.name))
  } catch (err) {
    loadError.value = describeError(err)
  } finally {
    loading.value = false
  }
}

// A rester is one encrypted record, so every edit rereads it, applies the change and writes the whole record.
async function commit(mutate: (r: VaultRester) => void): Promise<string> {
  try {
    const fresh = normalize(await window.api.vault.getRester(sessionPassphrase.value, props.name))
    mutate(fresh)
    const next = plainCopy(fresh)
    await window.api.vault.updateRester(sessionPassphrase.value, props.name, next)
    noteResterChange()
    rester.value = next
    return ''
  } catch (err) {
    return describeError(err)
  }
}

onMounted(load)
</script>

<template>
  <div class="section rester-detail">
    <header class="rester-header">
      <AppButton variant="text" size="sm" class="rester-back" @click="emit('close')">
        <AppIcon name="chevronLeft" :size="10" />
        Resters
      </AppButton>
      <div class="rester-heading">
        <h3 class="rester-title">{{ name }}</h3>
        <AppChip v-if="rester" tone="quiet">
          {{ rester.Environments.length }}
          {{ rester.Environments.length === 1 ? 'environment' : 'environments' }}
        </AppChip>
      </div>
    </header>

    <p v-if="loadError" class="error-text">{{ loadError }}</p>
    <AppEmptyState v-else-if="loading" layout="fill" loading title="Loading..." />

    <template v-else-if="rester">
      <EnvironmentsStrip :rester="rester" :commit="commit" />
      <AuthenticationSection :rester="rester" :commit="commit" />
      <UserCenter :rester="rester" :commit="commit" />
    </template>
  </div>
</template>

<style scoped>
.rester-header {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: var(--space-1);
}

/* The back link hangs left so its text lines up with the title below it. */
.rester-back {
  margin-left: calc(-1 * var(--space-3));
}

.rester-heading {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  min-width: 0;
}

.rester-title {
  min-width: 0;
  overflow: hidden;
  font-family: var(--font-display);
  font-size: var(--text-lg);
  font-weight: var(--weight-semibold);
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
