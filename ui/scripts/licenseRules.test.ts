// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { detectLicense, licenseAccepted } from './licenseRules.ts'

test('a single accepted id passes and an unknown one fails', () => {
  assert.equal(licenseAccepted('MIT'), true)
  assert.equal(licenseAccepted('OFL-1.1'), true)
  assert.equal(licenseAccepted('GPL-2.0-only'), false)
  assert.equal(licenseAccepted('UNLICENSED'), false)
  assert.equal(licenseAccepted(''), false)
})

test('OR passes when any branch passes, AND only when every operand does', () => {
  assert.equal(licenseAccepted('(MIT OR CC0-1.0)'), true)
  assert.equal(licenseAccepted('SSPL-1.0 OR MIT'), true)
  assert.equal(licenseAccepted('MIT AND BSD-3-Clause'), true)
  assert.equal(licenseAccepted('MIT AND GPL-3.0-only'), false)
  assert.equal(licenseAccepted('GPL-3.0-only OR (MIT AND Apache-2.0)'), true)
})

test('WITH is judged by the license it modifies', () => {
  assert.equal(licenseAccepted('Apache-2.0 WITH LLVM-exception'), true)
  assert.equal(licenseAccepted('GPL-2.0-only WITH Classpath-exception-2.0'), false)
})

test('a malformed expression fails', () => {
  assert.equal(licenseAccepted('(MIT OR'), false)
  assert.equal(licenseAccepted('MIT OR OR ISC'), false)
  assert.equal(licenseAccepted('MIT ISC'), false)
})

test('license texts are recognised by their signature', () => {
  assert.equal(
    detectLicense('Apache License\n  Version 2.0, January 2004\n http://www.apache.org/licenses/'),
    'Apache-2.0'
  )
  assert.equal(detectLicense('Mozilla Public License Version 2.0\n=========='), 'MPL-2.0')
  assert.equal(
    detectLicense('Permission is hereby granted, free of charge, to any person obtaining a copy'),
    'MIT'
  )
  assert.equal(
    detectLicense(
      'Permission to use, copy, modify, and/or distribute this software for any\npurpose with or without fee'
    ),
    'ISC'
  )
  const bsd2 = 'Redistribution and use in source and binary forms, with or without modification'
  assert.equal(detectLicense(bsd2), 'BSD-2-Clause')
  assert.equal(
    detectLicense(
      `${bsd2}\n* Neither the name of Google Inc. nor the names of its contributors may be used to endorse`
    ),
    'BSD-3-Clause'
  )
  assert.equal(detectLicense('GNU GENERAL PUBLIC LICENSE Version 3'), null)
})
