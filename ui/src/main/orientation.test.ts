// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import assert from 'node:assert/strict'
import { mkdirSync, mkdtempSync, readFileSync, statSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { test } from 'node:test'
import { orientationLaunch } from './orientation.ts'

function homeWith(...roles: string[]): string {
  const home = mkdtempSync(join(tmpdir(), 'orientation-'))
  const dir = join(home, '.lemongrass', 'orientation')
  mkdirSync(dir, { recursive: true })
  for (const role of roles) writeFileSync(join(dir, `${role}.md`), `${role} text\n`)
  return home
}

function projectWithLaws(summary: string): string {
  const cwd = mkdtempSync(join(tmpdir(), 'project-'))
  mkdirSync(join(cwd, 'biblio', 'laws'), { recursive: true })
  writeFileSync(join(cwd, 'biblio', 'laws', 'summary.md'), summary)
  return cwd
}

test('a plain Claude tab gets the tab orientation', () => {
  const home = homeWith('tab', 'thinker')
  assert.deepEqual(orientationLaunch('claude', 'tab', home), {
    args: ['--append-system-prompt-file', join(home, '.lemongrass', 'orientation', 'tab.md')],
    env: {}
  })
})

test('a Claude thinker gets the thinker orientation', () => {
  const home = homeWith('tab', 'thinker')
  assert.deepEqual(orientationLaunch('claude', 'thinker', home), {
    args: ['--append-system-prompt-file', join(home, '.lemongrass', 'orientation', 'thinker.md')],
    env: {}
  })
})

test('a project with laws gets the orientation followed by its laws summary', () => {
  const home = homeWith('tab')
  const cwd = projectWithLaws('# Laws summary\n\n- a rule\n')
  const launch = orientationLaunch('claude', 'tab', home, cwd)
  assert.equal(launch.args[0], '--append-system-prompt-file')
  assert.equal(readFileSync(launch.args[1], 'utf8'), 'tab text\n\n# Laws summary\n\n- a rule\n')
  assert.deepEqual(launch.env, { LGRASS_LAWS_IN_PROMPT: '1' })
})

test('two projects get separate prompt files and one project keeps its path', () => {
  const home = homeWith('tab')
  const first = projectWithLaws('# Laws summary\n\n- one\n')
  const second = projectWithLaws('# Laws summary\n\n- two\n')
  const a = orientationLaunch('claude', 'tab', home, first).args[1]
  const b = orientationLaunch('claude', 'tab', home, second).args[1]
  assert.notEqual(a, b)
  assert.equal(orientationLaunch('claude', 'tab', home, first).args[1], a)
})

test('a project without laws gets the plain orientation and no flag env', () => {
  const home = homeWith('tab')
  const cwd = mkdtempSync(join(tmpdir(), 'project-'))
  assert.deepEqual(orientationLaunch('claude', 'tab', home, cwd), {
    args: ['--append-system-prompt-file', join(home, '.lemongrass', 'orientation', 'tab.md')],
    env: {}
  })
})

test('other agents get no orientation', () => {
  const home = homeWith('tab', 'thinker', 'codex-tab')
  const cwd = projectWithLaws('# Laws summary\n')
  for (const command of ['agent', 'zsh']) {
    assert.deepEqual(orientationLaunch(command, 'tab', home, cwd), { args: [], env: {} })
  }
})

// A Codex home whose cache holds a listed default, a later listed model and a hidden one, each with its own base prompt.
function codexHome(config?: string): string {
  const dir = mkdtempSync(join(tmpdir(), 'codex-home-'))
  const model = (slug: string, priority: number, visibility: string): object => ({
    slug,
    priority,
    visibility,
    model_messages: { instructions_template: `base of ${slug}\n` }
  })
  writeFileSync(
    join(dir, 'models_cache.json'),
    JSON.stringify({
      models: [
        model('model-b', 2, 'list'),
        model('model-hidden', 0, 'hide'),
        model('model-a', 1, 'list')
      ]
    })
  )
  if (config !== undefined) writeFileSync(join(dir, 'config.toml'), config)
  return dir
}

function instructionsFile(launch: { args: string[] }): string {
  assert.equal(launch.args.length, 2)
  assert.equal(launch.args[0], '-c')
  const prefix = 'model_instructions_file='
  assert.ok(launch.args[1].startsWith(prefix))
  return JSON.parse(launch.args[1].slice(prefix.length)) as string
}

test('a Codex tab gets its model base prompt followed by the Codex orientation', () => {
  const home = homeWith('codex-tab', 'codex-thinker')
  const codex = codexHome('model = "model-b"\n[profiles.fast]\nmodel = "model-a"\n')
  const launch = orientationLaunch('codex', 'thinker', home, undefined, [], codex)
  assert.equal(
    readFileSync(instructionsFile(launch), 'utf8'),
    'base of model-b\n\ncodex-thinker text\n'
  )
  assert.deepEqual(launch.env, {})
})

test('the launch model wins over config, then a profile, then the top-level model', () => {
  const home = homeWith('codex-tab')
  const codex = codexHome(
    'model = "model-b"\nprofile = "fast"\n[profiles.fast]\nmodel = "model-a"\n'
  )
  const base = (args: string[]): string =>
    readFileSync(
      instructionsFile(orientationLaunch('codex', 'tab', home, undefined, args, codex)),
      'utf8'
    )
  assert.match(base([]), /^base of model-a\n/)
  assert.match(base(['-p', 'none']), /^base of model-b\n/)
  assert.match(base(['-m', 'model-hidden', 'a prompt']), /^base of model-hidden\n/)
  assert.match(base(['-c', 'model="model-b"', 'resume', 'session-1']), /^base of model-b\n/)
})

test('a Codex tab with no model anywhere gets the first listed model by priority', () => {
  const home = homeWith('codex-tab')
  const launch = orientationLaunch('codex', 'tab', home, undefined, [], codexHome())
  assert.match(
    readFileSync(instructionsFile(launch), 'utf8'),
    /^base of model-a\n\ncodex-tab text\n$/
  )
})

test('a Codex project with laws gets them after the orientation and keeps one stable file', () => {
  const home = homeWith('codex-tab')
  const codex = codexHome('model = "model-a"\n')
  const cwd = projectWithLaws('# Laws summary\n\n- a rule\n')
  const first = orientationLaunch('codex', 'tab', home, cwd, [], codex)
  const path = instructionsFile(first)
  assert.equal(
    readFileSync(path, 'utf8'),
    'base of model-a\n\ncodex-tab text\n\n# Laws summary\n\n- a rule\n'
  )
  assert.deepEqual(first.env, { LGRASS_LAWS_IN_PROMPT: '1' })
  const written = statSync(path).mtimeMs
  assert.equal(instructionsFile(orientationLaunch('codex', 'tab', home, cwd, [], codex)), path)
  assert.equal(statSync(path).mtimeMs, written)
})

test('a Codex tab whose model has no cached base prompt gets no instructions file', () => {
  const home = homeWith('codex-tab')
  const none = { args: [], env: {} }
  assert.deepEqual(
    orientationLaunch('codex', 'tab', home, undefined, ['-m', 'unknown'], codexHome()),
    none
  )
  const empty = mkdtempSync(join(tmpdir(), 'codex-home-'))
  assert.deepEqual(orientationLaunch('codex', 'tab', home, undefined, [], empty), none)
  assert.deepEqual(orientationLaunch('codex', 'thinker', home, undefined, [], codexHome()), none)
})

test('a missing orientation file adds nothing', () => {
  const cwd = projectWithLaws('# Laws summary\n')
  assert.deepEqual(orientationLaunch('claude', 'thinker', homeWith('tab'), cwd), {
    args: [],
    env: {}
  })
})
