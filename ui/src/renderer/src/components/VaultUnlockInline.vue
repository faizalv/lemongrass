<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
// A passphrase field that unlocks the vault in place, for a surface where a full gate or a modal
// would break the flow, such as a dropdown that offers more once unlocked. It only unlocks; a vault
// with no passphrase yet is set up in the Vault tab.
import { AppButton, AppInput, type AppInputHandle } from '@sankasten'
import { onMounted, ref } from 'vue'
import { describeError, unlockVaultSession } from '../vaultSession'

const props = withDefaults(defineProps<{ purpose?: string }>(), { purpose: 'use it' })

const input = ref('')
const error = ref('')
const unlocking = ref(false)
const hasPassphrase = ref(true)
const inputRef = ref<AppInputHandle | null>(null)

onMounted(async () => {
  // Focused a frame later, once a dropdown holding it has been placed and shown.
  requestAnimationFrame(() => inputRef.value?.focus())
  try {
    hasPassphrase.value = await window.api.vault.hasPassphrase()
  } catch (err) {
    error.value = describeError(err)
  }
})

async function unlock(): Promise<void> {
  if (!input.value || unlocking.value) return
  unlocking.value = true
  error.value = ''
  const passphrase = input.value
  try {
    await window.api.vault.verifyPassphrase(passphrase)
    input.value = ''
    unlockVaultSession(passphrase)
  } catch (err) {
    error.value = describeError(err)
  } finally {
    unlocking.value = false
  }
}
</script>

<template>
  <div class="vault-unlock-inline">
    <div v-if="hasPassphrase" class="unlock-row">
      <AppInput
        ref="inputRef"
        v-model="input"
        size="sm"
        type="password"
        class="unlock-input"
        placeholder="Vault passphrase"
        aria-label="Vault passphrase"
        @keydown.enter="unlock"
      />
      <AppButton size="sm" variant="primary" :disabled="!input || unlocking" @click="unlock">
        {{ unlocking ? 'Checking...' : 'Unlock' }}
      </AppButton>
    </div>
    <p v-else class="hint">Set a vault passphrase in the Vault tab to {{ props.purpose }}.</p>
    <p v-if="error" class="error">{{ error }}</p>
  </div>
</template>

<style scoped>
.vault-unlock-inline {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
}

.unlock-row {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.unlock-input {
  flex: 1;
  min-width: 0;
}

.hint,
.error {
  margin: 0;
  font-size: var(--text-xs);
}

.hint {
  color: var(--color-fg-muted);
}

.error {
  color: var(--color-error);
}
</style>
