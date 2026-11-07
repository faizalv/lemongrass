// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { languageIdFor } from './languageFor.ts'

test('an extension picks its language, case-insensitive', () => {
  assert.equal(languageIdFor('cmd/lgrass/main.go'), 'go')
  assert.equal(languageIdFor('ui/src/App.vue'), 'vue')
  assert.equal(languageIdFor('ui/scripts/afterPack.mjs'), 'javascript')
  assert.equal(languageIdFor('src/View.TSX'), 'tsx')
  assert.equal(languageIdFor('.github/workflows/release.yml'), 'yaml')
  assert.equal(languageIdFor('scripts/dev.sh'), 'shell')
})

test('a whole name wins over an extension and covers Dockerfile variants', () => {
  assert.equal(languageIdFor('build/Containerfile'), 'dockerfile')
  assert.equal(languageIdFor('Dockerfile'), 'dockerfile')
  assert.equal(languageIdFor('docker/Dockerfile.dev'), 'dockerfile')
})

test('an unknown extension, a dotfile or no extension is plain text', () => {
  assert.equal(languageIdFor('go.sum'), null)
  assert.equal(languageIdFor('.gitignore'), null)
  assert.equal(languageIdFor('Makefile'), null)
  assert.equal(languageIdFor('LICENSE'), null)
  assert.equal(languageIdFor('notes.txt'), null)
})

test('a name that matches an object prototype key is plain text', () => {
  assert.equal(languageIdFor('a.constructor'), null)
  assert.equal(languageIdFor('toString'), null)
})

test('env files, ini-style config and systemd units are key and value config', () => {
  assert.equal(languageIdFor('/srv/app/.env'), 'properties')
  assert.equal(languageIdFor('/srv/app/.env.production'), 'properties')
  assert.equal(languageIdFor('/srv/app/settings.ENV'), 'properties')
  assert.equal(languageIdFor('/etc/php/8.3/php.ini'), 'properties')
  assert.equal(languageIdFor('/etc/sysctl.conf'), 'properties')
  assert.equal(languageIdFor('/etc/systemd/system/app.service'), 'properties')
  assert.equal(languageIdFor('/etc/systemd/system/backup.timer'), 'properties')
})

test('nginx.conf and any .conf under an nginx folder are nginx config', () => {
  assert.equal(languageIdFor('/etc/nginx/nginx.conf'), 'nginx')
  assert.equal(languageIdFor('/etc/nginx/conf.d/default.conf'), 'nginx')
  assert.equal(languageIdFor('/etc/nginx/sites-enabled/app.conf'), 'nginx')
  assert.equal(languageIdFor('/srv/nginx.conf'), 'nginx')
})

test('shell startup dotfiles are shell', () => {
  assert.equal(languageIdFor('/home/deploy/.bashrc'), 'shell')
  assert.equal(languageIdFor('/root/.profile'), 'shell')
  assert.equal(languageIdFor('.zshrc'), 'shell')
})
