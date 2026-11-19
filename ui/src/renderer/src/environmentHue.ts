// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

// The hues an environment name can take. Cool ones only: amber and orange would read as the accent,
// green as success, and coral and rose as an error.
export const ENVIRONMENT_HUES = ['sky', 'violet', 'teal', 'fuchsia', 'sapphire', 'indigo'] as const

export type EnvironmentHue = (typeof ENVIRONMENT_HUES)[number]

// Environments take hues in their order within one rester, so two of them differ until the palette
// runs out, which a hash of the name does not guarantee.
export function environmentHue(index: number): EnvironmentHue {
  return ENVIRONMENT_HUES[index % ENVIRONMENT_HUES.length]
}

// The custom property `.env-name` in vault.css reads its color from.
export function environmentHueStyle(index: number): Record<string, string> {
  return { '--env-hue': `var(--color-${environmentHue(index)})` }
}
