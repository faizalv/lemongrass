<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { computed, ref } from 'vue'
import type { VaultEnvironment, VaultRester } from '../../../../../preload/types'
import { AppButton, AppCapsule, AppConfirmButton, AppIcon, AppMosaic } from '@sankasten'
import { environmentHue } from '../../../environmentHue'
import EnvironmentModal from './EnvironmentModal.vue'
import type { CommitRester } from './resterDraft'

const props = defineProps<{
  rester: VaultRester
  commit: CommitRester
}>()

type ModalMode =
  { kind: 'add' } | { kind: 'edit'; source: string } | { kind: 'duplicate'; source: string }

const mode = ref<ModalMode | null>(null)
const saving = ref(false)
const modalError = ref('')
const stripError = ref('')
const deleting = ref<string | null>(null)
const expanded = ref<string | null>(null)

const names = computed(() => props.rester.Environments.map((e) => e.Name))

const modalTitle = computed(() => {
  const m = mode.value
  if (!m || m.kind === 'add') return 'New environment'
  return m.kind === 'edit' ? `Edit ${m.source}` : `New environment from ${m.source}`
})

const modalInitial = computed<VaultEnvironment>(() => {
  const m = mode.value
  if (!m || m.kind === 'add') return { Name: '', BaseURL: '', Origin: '', Headers: null }
  const source = props.rester.Environments.find((e) => e.Name === m.source)
  const base = source ?? { Name: m.source, BaseURL: '', Origin: '', Headers: null }
  return { ...base, Name: m.kind === 'duplicate' ? `${m.source} copy` : base.Name }
})

function open(next: ModalMode): void {
  modalError.value = ''
  mode.value = next
}

function assignedCount(env: string): number {
  return props.rester.Users.filter((u) => (u.Environments ?? []).includes(env)).length
}

async function save(environment: VaultEnvironment): Promise<void> {
  const m = mode.value
  if (!m) return
  saving.value = true
  modalError.value = ''
  modalError.value = await props.commit((r) => {
    if (m.kind === 'edit') {
      const index = r.Environments.findIndex((e) => e.Name === m.source)
      if (index !== -1) r.Environments[index] = environment
      return
    }
    r.Environments.push(environment)
    if (m.kind !== 'duplicate') return
    // Duplicate also copies who is assigned to the source and their overrides there.
    for (const user of r.Users) {
      if (!(user.Environments ?? []).includes(m.source)) continue
      user.Environments = [...(user.Environments ?? []), environment.Name]
      const override = user.Overrides?.[m.source]
      if (override) user.Overrides = { ...user.Overrides, [environment.Name]: { ...override } }
    }
  })
  saving.value = false
  if (modalError.value === '') mode.value = null
}

async function remove(name: string): Promise<void> {
  deleting.value = name
  stripError.value = await props.commit((r) => {
    r.Environments = r.Environments.filter((e) => e.Name !== name)
    for (const user of r.Users) {
      user.Environments = (user.Environments ?? []).filter((e) => e !== name)
      if (user.Overrides && name in user.Overrides) {
        const rest = { ...user.Overrides }
        delete rest[name]
        user.Overrides = Object.keys(rest).length > 0 ? rest : null
      }
    }
  })
  deleting.value = null
}
</script>

<template>
  <div class="env-strip">
    <div class="users-toolbar">
      <span class="field-label">Environments</span>
      <span class="toolbar-fill"></span>
      <AppButton size="sm" @click="open({ kind: 'add' })">+ Environment</AppButton>
    </div>

    <AppMosaic v-model:expanded="expanded" :fill="false">
      <AppCapsule
        v-for="(env, index) in rester.Environments"
        :key="env.Name"
        :label="env.Name"
        :hue="environmentHue(index)"
        :count="assignedCount(env.Name)"
        :count-title="`${assignedCount(env.Name)} users assigned`"
      >
        <dl class="vault-facts">
          <dt>API</dt>
          <dd :class="{ missing: !env.BaseURL }">{{ env.BaseURL || 'no base URL' }}</dd>
          <dt>Web</dt>
          <dd :class="{ missing: !env.Origin }">{{ env.Origin || 'no origin' }}</dd>
          <dt>Users</dt>
          <dd>{{ assignedCount(env.Name) }} assigned</dd>
          <dt>Headers</dt>
          <dd>{{ (env.Headers ?? []).length }} extra</dd>
        </dl>
        <div class="env-capsule-actions">
          <AppConfirmButton
            variant="danger"
            size="sm"
            :disabled="rester.Environments.length < 2 || deleting === env.Name"
            :title="rester.Environments.length < 2 ? 'A rester keeps at least one environment' : ''"
            @confirm="remove(env.Name)"
          >
            <AppIcon name="trash" :size="12" />
            {{ deleting === env.Name ? 'Deleting...' : 'Delete' }}
          </AppConfirmButton>
          <span class="toolbar-fill"></span>
          <AppButton size="sm" @click="open({ kind: 'duplicate', source: env.Name })">
            <AppIcon name="copy" :size="12" />
            Duplicate
          </AppButton>
          <AppButton size="sm" variant="primary" @click="open({ kind: 'edit', source: env.Name })">
            <AppIcon name="edit" :size="12" />
            Edit
          </AppButton>
        </div>
      </AppCapsule>
    </AppMosaic>

    <p v-if="stripError" class="error-text">{{ stripError }}</p>

    <EnvironmentModal
      v-if="mode"
      :key="modalTitle"
      :title="modalTitle"
      :initial="modalInitial"
      :lock-name="mode.kind === 'edit'"
      :taken="names"
      :saving="saving"
      :error="modalError"
      @save="save"
      @close="mode = null"
    />
  </div>
</template>

<style scoped>
.env-strip {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.toolbar-fill {
  flex: 1;
}

.env-strip .users-toolbar {
  margin-bottom: 0;
}
</style>
