<!-- SPDX-License-Identifier: MPL-2.0 -->
<!-- Copyright (C) 2026 The Lemongrass Authors -->

<script setup lang="ts">
import { ref } from 'vue'
import { AppIconButton } from '@sankasten'
import { isWindows } from '../../platform'
import { isMac } from '../../windowControls'

// How to remove Lemongrass with the system's own tools, for the system the app runs on. People uninstall from the
// OS, not from inside an app, so the app only explains the order. Keep it in step with the Uninstall section of
// docs/install.md. Linux is the rpm, the only Linux package released.
const cleanCommand = '~/.local/bin/lgrassconf uninstall'
const removeCommand = 'sudo dnf remove lemongrass'

const dataFolders = isWindows
  ? ['%USERPROFILE%\\.lemongrass', '%APPDATA%\\lemongrass']
  : isMac
    ? ['~/.lemongrass', '~/Library/Application Support/lemongrass']
    : ['~/.lemongrass', '~/.config/lemongrass']

const copied = ref<string | null>(null)

function copy(text: string): void {
  void navigator.clipboard.writeText(text)
  copied.value = text
  setTimeout(() => {
    if (copied.value === text) copied.value = null
  }, 1500)
}
</script>

<template>
  <section class="uninstall">
    <h3 class="settings-subtitle">Uninstall</h3>

    <ol v-if="isWindows" class="steps settings-text">
      <li>Close Lemongrass.</li>
      <li>
        Open Windows Settings, then Apps, then Installed apps. Find Lemongrass and choose Uninstall.
        Its uninstaller also removes the keeper, the hooks, the skills and the helper programs.
      </li>
    </ol>

    <ol v-else class="steps settings-text">
      <li>{{ isMac ? 'Quit Lemongrass.' : 'Close Lemongrass.' }}</li>
      <li>
        In a terminal, remove the keeper, the hooks, the skills and the helper programs from your
        account:
        <div class="command">
          <code>{{ cleanCommand }}</code>
          <AppIconButton
            size="sm"
            :icon="copied === cleanCommand ? 'check' : 'copy'"
            :label="copied === cleanCommand ? 'Copied' : 'Copy command'"
            @click="copy(cleanCommand)"
          />
        </div>
      </li>
      <li v-if="isMac">Move Lemongrass from Applications to the Trash.</li>
      <li v-else>
        Remove the package:
        <div class="command">
          <code>{{ removeCommand }}</code>
          <AppIconButton
            size="sm"
            :icon="copied === removeCommand ? 'check' : 'copy'"
            :label="copied === removeCommand ? 'Copied' : 'Copy command'"
            @click="copy(removeCommand)"
          />
        </div>
      </li>
    </ol>

    <p v-if="!isWindows" class="settings-text">
      Run the first command before removing the app. Removing the app does not reach your home
      folder, and the keeper puts the hooks back while it runs.
    </p>

    <p class="settings-text">
      Your data stays: the vault, your projects and the biblio history in
      <code>{{ dataFolders[0] }}</code
      >, and the app's own settings in <code>{{ dataFolders[1] }}</code
      >. Delete both to remove everything.
    </p>
  </section>
</template>

<style scoped>
.uninstall {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.steps {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  margin: 0;
  padding-left: var(--space-5);
}

.command {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  margin-top: var(--space-1);
  padding: var(--space-1) var(--space-1) var(--space-1) var(--space-3);
  background: var(--color-surface-0);
  border-radius: var(--radius-md);
}

.command code {
  flex: 1;
  min-width: 0;
  color: var(--color-fg-primary);
  font-family: var(--font-mono);
  font-size: var(--text-xs);
  word-break: break-all;
}

.settings-text code {
  font-family: var(--font-mono);
  font-size: var(--text-xs);
}
</style>
