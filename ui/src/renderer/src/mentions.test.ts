import { test } from 'node:test'
import assert from 'node:assert/strict'
import { bodyParts } from './mentions.ts'

const members = [
  { tabId: '35546ed4-1111-4222-8333-444455556666', label: 'lead' },
  { tabId: 'e7f244ea-d54c-4226-9156-b5429737b257', label: 'checker' }
]

test('a short mention shows the member label', () => {
  const parts = bodyParts('!>>35546ed4<<! checker here', members)
  assert.deepEqual(parts[0], { text: '@lead', mention: true })
  assert.equal(parts[1].text, ' checker here')
})

test('a full tab id mention shows the member label', () => {
  const parts = bodyParts('hi !>>E7F244EA-D54C-4226-9156-B5429737B257<<!', members)
  assert.deepEqual(parts[1], { text: '@checker', mention: true })
})

test('an unknown mention stays as written', () => {
  const parts = bodyParts('!>>deadbeef<<!', members)
  assert.deepEqual(parts[0], { text: '!>>deadbeef<<!', mention: false })
})

test('a mention missing its closing bang stays as written', () => {
  const parts = bodyParts('!>>35546ed4<< checker here', members)
  assert.deepEqual(parts[0], { text: '!>>35546ed4<< checker here', mention: false })
})
