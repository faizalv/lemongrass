// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { createHash } from 'node:crypto'
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'

export type OrientationRole = 'tab' | 'thinker'

export interface OrientationLaunch {
  args: string[]
  env: Record<string, string>
}

// The SessionStart hook skips the laws summary for a session started with this set.
export const LAWS_IN_PROMPT_ENV = 'LGRASS_LAWS_IN_PROMPT'

const NO_ORIENTATION: OrientationLaunch = { args: [], env: {} }

function readIfExists(path: string): string | undefined {
  return existsSync(path) ? readFileSync(path, 'utf8') : undefined
}

function projectKey(cwd: string): string {
  return createHash('sha256').update(cwd).digest('hex').slice(0, 16)
}

// Rewritten only when the bytes differ, so tabs that share a file keep sharing one prompt cache entry.
function writePromptFile(home: string, name: string, content: string): string {
  const dir = join(home, '.lemongrass', 'prompt')
  const path = join(dir, name)
  if (readIfExists(path) !== content) {
    mkdirSync(dir, { recursive: true })
    writeFileSync(path, content)
  }
  return path
}

function withLaws(orientation: string, laws: string): string {
  return `${orientation.trimEnd()}\n\n${laws.trim()}\n`
}

function projectLaws(cwd?: string): string | undefined {
  const laws = cwd ? readIfExists(join(cwd, 'biblio', 'laws', 'summary.md')) : undefined
  return laws === undefined || laws.trim() === '' ? undefined : laws
}

// lgrassconf installs one orientation per role; a Claude tab spawned before it exists starts without one.
function claudeLaunch(role: OrientationRole, home: string, cwd?: string): OrientationLaunch {
  const orientationPath = join(home, '.lemongrass', 'orientation', `${role}.md`)
  const orientation = readIfExists(orientationPath)
  if (orientation === undefined) return NO_ORIENTATION
  const laws = projectLaws(cwd)
  if (laws === undefined || cwd === undefined) {
    return { args: ['--append-system-prompt-file', orientationPath], env: {} }
  }
  // A project's laws summary follows the orientation, so tabs of one project share both in the prompt cache.
  const path = writePromptFile(home, `${projectKey(cwd)}-${role}.md`, withLaws(orientation, laws))
  return { args: ['--append-system-prompt-file', path], env: { [LAWS_IN_PROMPT_ENV]: '1' } }
}

// The value of a TOML string or bare word, as Codex reads it in config.toml and in a -c override.
function tomlString(value: string): string {
  const trimmed = value.trim()
  const basic = /^"((?:[^"\\]|\\.)*)"/.exec(trimmed)
  if (basic) return JSON.parse(`"${basic[1]}"`) as string
  const literal = /^'([^']*)'/.exec(trimmed)
  if (literal) return literal[1]
  return trimmed.split(/\s+#/)[0].trim()
}

interface CodexModelChoice {
  model?: string
  profile?: string
}

// The model and profile named on the launch's own command line; a later flag wins, as in Codex.
function choiceFromArgs(args: string[]): CodexModelChoice {
  const choice: CodexModelChoice = {}
  for (let i = 0; i < args.length; i++) {
    const arg = args[i]
    const next = args[i + 1]
    if ((arg === '-m' || arg === '--model') && next !== undefined) choice.model = next
    else if (arg.startsWith('--model=')) choice.model = arg.slice('--model='.length)
    else if ((arg === '-p' || arg === '--profile') && next !== undefined) choice.profile = next
    else if (arg.startsWith('--profile=')) choice.profile = arg.slice('--profile='.length)
    else if ((arg === '-c' || arg === '--config') && next !== undefined) {
      const eq = next.indexOf('=')
      if (eq < 0) continue
      const key = next.slice(0, eq).trim()
      if (key === 'model') choice.model = tomlString(next.slice(eq + 1))
      else if (key === 'profile') choice.profile = tomlString(next.slice(eq + 1))
    }
  }
  return choice
}

// The model config.toml picks: the active profile's model when it names one, else the top-level model.
function modelFromConfig(codexHome: string, profile?: string): string | undefined {
  const text = readIfExists(join(codexHome, 'config.toml'))
  if (text === undefined) return undefined
  let table = ''
  let rootModel: string | undefined
  let rootProfile: string | undefined
  const profileModels: Record<string, string> = {}
  for (const line of text.split(/\r?\n/)) {
    const header = /^\s*\[([^\]]+)\]\s*(#.*)?$/.exec(line)
    if (header) {
      table = header[1].trim()
      continue
    }
    const entry = /^\s*([A-Za-z0-9_-]+)\s*=(.*)$/.exec(line)
    if (!entry) continue
    const [, key, value] = entry
    if (table === '' && key === 'model') rootModel = tomlString(value)
    else if (table === '' && key === 'profile') rootProfile = tomlString(value)
    else if (table.startsWith('profiles.') && key === 'model')
      profileModels[tomlString(table.slice('profiles.'.length))] = tomlString(value)
  }
  const active = profile ?? rootProfile
  return (active !== undefined ? profileModels[active] : undefined) ?? rootModel
}

interface CachedModel {
  slug?: unknown
  priority?: unknown
  visibility?: unknown
  model_messages?: { instructions_template?: unknown }
}

// Codex's base prompt for a model is its instructions_template in models_cache.json, the same bytes a session
// records as base_instructions. With no model chosen, the first listed model by priority stands in for Codex's
// own default.
function codexBasePrompt(
  codexHome: string,
  model: string | undefined
): { model: string; text: string } | undefined {
  const raw = readIfExists(join(codexHome, 'models_cache.json'))
  if (raw === undefined) return undefined
  let models: CachedModel[]
  try {
    const parsed = JSON.parse(raw) as { models?: unknown }
    if (!Array.isArray(parsed.models)) return undefined
    models = parsed.models as CachedModel[]
  } catch {
    return undefined
  }
  const priority = (entry: CachedModel): number =>
    typeof entry.priority === 'number' ? entry.priority : Infinity
  const pick =
    model !== undefined
      ? models.find((entry) => entry.slug === model)
      : models
          .filter((entry) => entry.visibility === 'list' && typeof entry.slug === 'string')
          .sort((a, b) => priority(a) - priority(b))[0]
  const text = pick?.model_messages?.instructions_template
  if (typeof pick?.slug !== 'string' || typeof text !== 'string' || text.trim() === '')
    return undefined
  return { model: pick.slug, text }
}

// Codex has no additive instruction file, so the tab's model_instructions_file is Codex's own base prompt for the
// model followed by the orientation. A tab whose model has no cached base prompt opens on Codex's own prompt with
// no orientation, never on another model's base prompt. The file holds nothing per session, so tabs of one model,
// role and project share it.
function codexLaunch(
  role: OrientationRole,
  home: string,
  cwd: string | undefined,
  args: string[],
  codexHome: string
): OrientationLaunch {
  const orientation = readIfExists(join(home, '.lemongrass', 'orientation', `codex-${role}.md`))
  if (orientation === undefined) return NO_ORIENTATION
  const choice = choiceFromArgs(args)
  const base = codexBasePrompt(
    codexHome,
    choice.model ?? modelFromConfig(codexHome, choice.profile)
  )
  if (base === undefined) return NO_ORIENTATION
  const prompt = `${base.text}${base.text.endsWith('\n') ? '\n' : '\n\n'}${orientation.trimEnd()}\n`
  const model = base.model.replace(/[^A-Za-z0-9._-]/g, '_')
  const laws = projectLaws(cwd)
  const withProject = laws !== undefined && cwd !== undefined
  const path = writePromptFile(
    home,
    `codex-${withProject ? `${projectKey(cwd)}-` : ''}${model}-${role}.md`,
    withProject ? withLaws(prompt, laws) : prompt
  )
  return {
    args: ['-c', `model_instructions_file=${JSON.stringify(path)}`],
    env: withProject ? { [LAWS_IN_PROMPT_ENV]: '1' } : {}
  }
}

// args are the tab's own launch arguments, read for the model a Codex tab runs.
export function orientationLaunch(
  command: string,
  role: OrientationRole,
  home: string,
  cwd?: string,
  args: string[] = [],
  codexHome: string = process.env.CODEX_HOME || join(home, '.codex')
): OrientationLaunch {
  if (command === 'claude') return claudeLaunch(role, home, cwd)
  if (command === 'codex') return codexLaunch(role, home, cwd, args, codexHome)
  return NO_ORIENTATION
}
