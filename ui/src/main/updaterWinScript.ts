// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

// Must match uninstallDisplayName in electron-builder.yml, which names the uninstaller beside the app's exe.
export const WIN_UNINSTALLER = 'Uninstall Lemongrass.exe'

// Only a copy the per-user setup.exe installed can update itself: its folder holds the uninstaller, and the next
// setup.exe installs into the same place.
export function winUnsupportedReason(arch: string, hasUninstaller: boolean): string | null {
  if (arch !== 'x64') return 'Updates are published for Windows x64 only.'
  if (!hasUninstaller)
    return 'This copy was not installed from the setup, so it cannot update itself.'
  return null
}
