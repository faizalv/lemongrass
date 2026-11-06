<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
// The fields of a new key, shared by Create key and the inline key step of Add server. The parent
// owns the buttons and calls create().
import { computed, nextTick, onMounted, ref } from 'vue'
import { AppInput, type AppInputHandle } from '@sankasten'
import { describeError } from '../../vaultSession'
import type { SSHKey } from '../../../../preload/types'

const props = defineProps<{
  existing: string[]
  // A starting name, such as the server the key is made for.
  suggested?: string
}>()

const emit = defineEmits<{
  created: [key: SSHKey]
}>()

const NAME_PATTERN = /^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/

function freeName(): string {
  const base = (props.suggested ?? '').replace(/[^A-Za-z0-9._-]/g, '-') || 'id_ed25519'
  if (!props.existing.includes(base)) return base
  for (let n = 2; ; n++) {
    const name = `${base}_${n}`
    if (!props.existing.includes(name)) return name
  }
}

const name = ref(freeName())
const comment = ref('')
const passphrase = ref('')
const repeat = ref('')
const busy = ref(false)
const error = ref('')
const nameRef = ref<AppInputHandle | null>(null)

const problem = computed((): string => {
  const n = name.value.trim()
  if (!n) return ''
  if (!NAME_PATTERN.test(n) || n.endsWith('.pub')) {
    return 'Use letters, digits, dots, dashes and underscores, starting with a letter or digit.'
  }
  if (props.existing.includes(n)) return 'A key with the same name already exists.'
  if (repeat.value && repeat.value !== passphrase.value) return 'The passphrases do not match.'
  return ''
})

const canCreate = computed(
  () => !!name.value.trim() && !problem.value && repeat.value === passphrase.value && !busy.value
)

onMounted(async () => {
  await nextTick()
  nameRef.value?.focus()
  nameRef.value?.select()
})

async function create(): Promise<void> {
  if (!canCreate.value) return
  busy.value = true
  error.value = ''
  try {
    const key = await window.api.ssh.createKey({
      name: name.value.trim(),
      comment: comment.value.trim(),
      passphrase: passphrase.value
    })
    emit('created', key)
  } catch (err) {
    error.value = describeError(err)
  } finally {
    busy.value = false
  }
}

defineExpose({ create, canCreate, busy })
</script>

<template>
  <div class="key-form">
    <div class="settings-fields">
      <label class="settings-field">
        <span>Name</span>
        <AppInput
          ref="nameRef"
          v-model="name"
          aria-label="Key name"
          :disabled="busy"
          @keydown.enter="create"
        />
      </label>
      <label class="settings-field">
        <span>Label (optional)</span>
        <AppInput
          v-model="comment"
          placeholder="you@laptop"
          aria-label="Key label"
          :disabled="busy"
          @keydown.enter="create"
        />
      </label>
    </div>
    <p class="key-form-hint">
      The label shows next to the key on servers and on GitHub, so you can tell your keys apart.
    </p>
    <div class="settings-fields">
      <label class="settings-field">
        <span>Passphrase (optional)</span>
        <AppInput
          v-model="passphrase"
          type="password"
          autocomplete="new-password"
          aria-label="Passphrase"
          :disabled="busy"
          @keydown.enter="create"
        />
      </label>
      <label class="settings-field">
        <span>Repeat passphrase</span>
        <AppInput
          v-model="repeat"
          type="password"
          autocomplete="new-password"
          aria-label="Repeat passphrase"
          :disabled="busy"
          @keydown.enter="create"
        />
      </label>
    </div>
    <p class="key-form-hint">
      With a passphrase, a copied key file is useless on its own. ssh asks for it when the key is
      used.
    </p>
    <p v-if="problem" class="settings-error">{{ problem }}</p>
    <p v-if="error" class="settings-error">{{ error }}</p>
  </div>
</template>

<style scoped>
.key-form {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.key-form-hint {
  margin-top: calc(-1 * var(--space-1));
  color: var(--color-fg-muted);
  font-size: var(--text-xs);
  line-height: var(--leading-relaxed);
}
</style>
