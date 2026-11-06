<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
// Add server and Edit server. A login key can be created without leaving the form. With no key,
// ssh tries the default keys and then asks for the server's password, which suits a bare VM.
import { computed, nextTick, onMounted, ref } from 'vue'
import {
  AppButton,
  AppCheckbox,
  AppDisclosure,
  AppInput,
  type AppInputHandle,
  AppModal,
  AppSwitcher
} from '@sankasten'
import SSHKeyForm from './SSHKeyForm.vue'
import { describeError } from '../../vaultSession'
import { tildePath } from './sshView'
import type { SSHHostEntry, SSHHostFields, SSHHostList, SSHKey } from '../../../../preload/types'

const props = defineProps<{
  // The server being edited, or null for a new one.
  host: SSHHostEntry | null
  hosts: SSHHostList
  keys: SSHKey[]
  // A key to start with, such as one just created.
  presetKey?: SSHKey | null
}>()

const emit = defineEmits<{
  close: []
  keyCreated: [key: SSHKey]
  saved: [alias: string]
}>()

const NONE = ''
// Never a key path, which is always absolute.
const NEW_KEY = 'new-key'

const start: SSHHostFields = props.host?.fields ?? {
  alias: '',
  hostName: '',
  user: '',
  port: '',
  identityFile: props.presetKey?.privatePath ?? '',
  proxyJump: '',
  identitiesOnly: true
}

const alias = ref(start.alias)
const hostName = ref(start.hostName)
const user = ref(start.user)
const port = ref(start.port)
const identityFile = ref(start.identityFile)
const proxyJump = ref(start.proxyJump)
const identitiesOnly = ref(start.identitiesOnly)
const more = ref(!!(start.port || start.proxyJump))
const creatingKey = ref(false)
const madeKeys = ref<SSHKey[]>([])
const busy = ref(false)
const error = ref('')
const aliasRef = ref<AppInputHandle | null>(null)
const keyForm = ref<InstanceType<typeof SSHKeyForm> | null>(null)

const allKeys = computed(() => [...props.keys, ...madeKeys.value])

const keyItems = computed(() => [
  ...allKeys.value
    .filter((key) => key.hasPrivate)
    .map((key) => ({ id: key.privatePath, label: key.name, hint: key.comment || undefined })),
  { id: NEW_KEY, label: 'Create a new key' },
  { id: NONE, label: 'None', hint: 'Default keys, then the server password' }
])

const jumpItems = computed(() => [
  { id: NONE, label: 'None', hint: 'Connect straight to the server' },
  ...props.hosts.hosts
    .filter((h) => h.alias !== props.host?.alias)
    .map((h) => ({ id: h.alias, label: h.alias, hint: h.resolved?.hostname }))
])

// The first server saved while ssh does not read lemongrass's file links it, after the human sees
// the line.
const linking = computed(() => !props.host && props.hosts.include !== 'included')

const taken = computed(() =>
  props.hosts.hosts.some((h) => h.alias === alias.value.trim() && h.alias !== props.host?.alias)
)

const canSave = computed(
  () => !!alias.value.trim() && !!hostName.value.trim() && !taken.value && !busy.value
)

onMounted(async () => {
  await nextTick()
  aliasRef.value?.focus()
})

function pickKey(id: string): void {
  if (id === NEW_KEY) creatingKey.value = true
  else identityFile.value = id
}

function keyCreated(key: SSHKey): void {
  madeKeys.value.push(key)
  identityFile.value = key.privatePath
  creatingKey.value = false
  emit('keyCreated', key)
}

async function save(): Promise<void> {
  if (!canSave.value) return
  busy.value = true
  error.value = ''
  const name = alias.value.trim()
  try {
    await window.api.ssh.saveHost(props.host?.alias ?? null, {
      alias: name,
      hostName: hostName.value.trim(),
      user: user.value.trim(),
      port: String(port.value).trim(),
      identityFile: identityFile.value,
      proxyJump: proxyJump.value,
      identitiesOnly: !!identityFile.value && identitiesOnly.value
    })
    if (linking.value) await window.api.ssh.addInclude()
    emit('saved', name)
  } catch (err) {
    error.value = describeError(err)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <AppModal
    :title="creatingKey ? 'Create a login key' : host ? `Edit ${host.alias}` : 'Add server'"
    size="md"
    @close="emit('close')"
  >
    <SSHKeyForm
      v-if="creatingKey"
      ref="keyForm"
      :existing="allKeys.map((k) => k.name)"
      :suggested="alias.trim()"
      @created="keyCreated"
    />

    <template v-else>
      <div class="settings-fields">
        <label class="settings-field">
          <span>Name</span>
          <AppInput
            ref="aliasRef"
            v-model="alias"
            placeholder="web-staging"
            aria-label="Server name, used as ssh web-staging"
            :disabled="busy"
            @keydown.enter="save"
          />
        </label>
        <label class="settings-field">
          <span>Address</span>
          <AppInput
            v-model="hostName"
            placeholder="203.0.113.10 or server.example.com"
            aria-label="Server address"
            :disabled="busy"
            @keydown.enter="save"
          />
        </label>
      </div>
      <div class="settings-fields">
        <label class="settings-field">
          <span>Username</span>
          <AppInput
            v-model="user"
            placeholder="Same as on the computer"
            aria-label="Username on the server"
            :disabled="busy"
            @keydown.enter="save"
          />
        </label>
        <div class="settings-field">
          <span>Login key</span>
          <AppSwitcher
            framed
            :items="keyItems"
            :active-id="identityFile"
            placeholder="None"
            empty-text="No keys."
            :disabled="busy"
            @select="pickKey"
          />
        </div>
      </div>

      <AppDisclosure v-model:open="more" label="More options" />
      <template v-if="more">
        <div class="settings-fields">
          <label class="settings-field settings-field--narrow">
            <span>Port</span>
            <AppInput
              v-model="port"
              placeholder="22"
              aria-label="Port"
              :disabled="busy"
              @keydown.enter="save"
            />
          </label>
          <div class="settings-field">
            <span>Jump host</span>
            <AppSwitcher
              framed
              :items="jumpItems"
              :active-id="proxyJump"
              :label="proxyJump || undefined"
              placeholder="None"
              empty-text="No other servers."
              :disabled="busy"
              @select="proxyJump = $event"
            />
          </div>
        </div>
        <p class="server-hint">
          A jump host is a server ssh connects through first, for servers reachable only from inside
          its network.
        </p>
        <AppCheckbox
          v-if="identityFile"
          :checked="identitiesOnly"
          size="sm"
          @click="identitiesOnly = !identitiesOnly"
        >
          Offer only the login key, so a server that limits login attempts does not lock you out
        </AppCheckbox>
      </template>

      <div v-if="linking" class="server-link">
        <p class="server-hint">
          Lemongrass saves servers in
          <code>{{ tildePath(hosts.managedFile, hosts.home) }}</code> and adds one line to the top
          of <code>{{ tildePath(hosts.configFile, hosts.home) }}</code
          >, so the server also works in any terminal. Nothing else in the file changes.
        </p>
        <pre class="server-line">{{ hosts.includeLine }}</pre>
      </div>

      <p v-if="taken" class="settings-error">A server with the same name already exists.</p>
      <p v-if="error" class="settings-error">{{ error }}</p>
    </template>

    <template #footer>
      <template v-if="creatingKey">
        <AppButton @click="creatingKey = false">Back</AppButton>
        <AppButton variant="primary" :disabled="!keyForm?.canCreate" @click="keyForm?.create()">
          Create key
        </AppButton>
      </template>
      <template v-else>
        <AppButton @click="emit('close')">Cancel</AppButton>
        <AppButton variant="primary" :disabled="!canSave" @click="save">
          {{ host ? 'Save' : 'Add server' }}
        </AppButton>
      </template>
    </template>
  </AppModal>
</template>

<style scoped>
.server-hint {
  color: var(--color-fg-muted);
  font-size: var(--text-xs);
  line-height: var(--leading-relaxed);
}

.server-hint code {
  color: var(--color-fg-secondary);
  font-family: inherit;
}

.server-link {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.server-line {
  margin: 0;
  padding: var(--space-2) var(--space-3);
  border-radius: var(--radius-sm);
  background: var(--layer-input);
  color: var(--color-fg-primary);
  font-family: var(--font-mono);
  font-size: var(--text-xs);
}
</style>
