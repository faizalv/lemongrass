// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { readFileSync, realpathSync } from 'node:fs'
import { homedir } from 'node:os'
import { basename, dirname, join, resolve } from 'node:path'

const LONG_CONTEXT_ARG = 'model_context_window=1000000'

function readConfig(path: string): string {
  try {
    return readFileSync(path, 'utf8')
  } catch (error) {
    if (['ENOENT', 'ENOTDIR'].includes((error as NodeJS.ErrnoException).code ?? '')) return ''
    throw error
  }
}

function unquote(value: string): string {
  return value.replace(/^("|')(.*)\1$/, '$2')
}

function configProvidesContext(key: string, value: string): boolean {
  return (
    key === 'model_context_window' ||
    key === 'model_catalog_json' ||
    (key === 'model_provider' && unquote(value.trim()) !== 'openai')
  )
}

function configEntries(text: string): [string, string][] {
  // Scan for declarations only. A matching declaration in an ambiguous TOML scope
  // takes precedence over injecting a CLI flag that would override the user's window.
  return Array.from(
    text.matchAll(/^[ \t]*([\w-]+|"[\w-]+"|'[\w-]+')[ \t]*=[ \t]*([^\r\n#]*)/gm),
    ([, key, value]) => [unquote(key), value.trim()]
  )
}

function projectConfigs(cwd: string): string[] {
  const dir = realpathSync(cwd)
  const paths = [join(dir, 'config.toml')]
  for (let current = dir; ; current = dirname(current)) {
    paths.push(join(current, '.codex', 'config.toml'))
    if (dirname(current) === current) return paths
  }
}

// A launch flag outranks config files. Apply it as a default only while the local
// config leaves the choice to Codex. Model selection and the cap stay with Codex.
export function codexContextArgs(
  args: readonly string[],
  cwd: string,
  codexHome = process.env.CODEX_HOME || join(homedir(), '.codex'),
  systemConfig = process.platform === 'win32'
    ? join(process.env.ProgramData || 'C:\\ProgramData', 'OpenAI', 'Codex', 'config.toml')
    : '/etc/codex/config.toml'
): string[] {
  let profile: string | undefined
  let directory = cwd
  for (let i = 0; i < args.length; i++) {
    const arg = args[i]
    if (arg === '--') break
    if (arg === '--oss') return []

    const option = /^(--(?:config|profile|cd))(?:=(.*))?$|^(-[cpC])(.*)$/s.exec(arg)
    if (!option) continue
    const flag = option[1] ?? option[3]
    const inline = option[2] ?? (option[4]?.replace(/^=/, '') || undefined)
    const value = inline ?? args[++i]
    if (value === undefined) continue

    if (flag === '-p' || flag === '--profile') profile = value
    else if (flag === '-C' || flag === '--cd') directory = resolve(cwd, value)
    else {
      const eq = value.indexOf('=')
      if (eq < 0) continue
      const key = unquote(value.slice(0, eq).trim())
      const setting = value.slice(eq + 1).trim()
      if (configProvidesContext(key, setting)) return []
      if (key === 'profile') profile = unquote(setting)
    }
  }

  try {
    const configs = [readConfig(systemConfig), readConfig(join(codexHome, 'config.toml'))]
    const settings = configs.flatMap(configEntries)
    profile ??= settings.findLast(([key]) => key === 'profile')?.[1]
    if (profile) {
      profile = unquote(profile)
      if (basename(profile) !== profile) return []
      settings.push(...configEntries(readConfig(join(codexHome, `${profile}.config.toml`))))
    }
    for (const path of projectConfigs(directory)) settings.push(...configEntries(readConfig(path)))
    if (settings.some(([key, value]) => configProvidesContext(key, value))) return []
  } catch {
    return []
  }

  return ['-c', LONG_CONTEXT_ARG]
}
