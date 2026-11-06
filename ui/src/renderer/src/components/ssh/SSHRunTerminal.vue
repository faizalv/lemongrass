<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
// Runs ssh-add in a terminal the human types into, since it asks for a passphrase
// itself and the app never handles one.
import { onBeforeUnmount, ref } from 'vue'
import AppTerminal from '../AppTerminal.vue'
import { describeError } from '../../vaultSession'
import type { SSHRunRequest } from '../../../../preload/types'

const props = defineProps<{
  request: SSHRunRequest
}>()

const emit = defineEmits<{
  exit: [code: number]
  failed: [message: string]
}>()

const terminal = ref<InstanceType<typeof AppTerminal> | null>(null)
let runId: string | null = null
let started = false
// Output can arrive before runStart resolves with the id it belongs to.
const early = new Map<string, string[]>()

const offData = window.api.ssh.onRunData(({ runId: id, data }) => {
  if (id === runId) terminal.value?.write(data)
  else early.set(id, [...(early.get(id) ?? []), data])
})

const offExit = window.api.ssh.onRunExit(({ runId: id, exitCode }) => {
  if (id !== runId) return
  runId = null
  emit('exit', exitCode)
})

// The command starts once the terminal has measured itself, so it gets its real size.
async function launch(cols: number, rows: number): Promise<void> {
  try {
    const id = await window.api.ssh.runStart({ ...props.request }, cols, rows)
    runId = id
    for (const chunk of early.get(id) ?? []) terminal.value?.write(chunk)
    early.clear()
    terminal.value?.focus()
  } catch (err) {
    emit('failed', describeError(err))
  }
}

function onResize(size: { rows: number; cols: number }): void {
  if (runId) window.api.ssh.runResize(runId, size.cols, size.rows)
  else if (!started) {
    started = true
    void launch(size.cols, size.rows)
  }
}

function onInput(data: string): void {
  if (runId) window.api.ssh.runInput(runId, data)
}

onBeforeUnmount(() => {
  offData()
  offExit()
  if (runId) window.api.ssh.runKill(runId)
})
</script>

<template>
  <AppTerminal ref="terminal" label="Command terminal" @data="onInput" @resize="onResize" />
</template>
