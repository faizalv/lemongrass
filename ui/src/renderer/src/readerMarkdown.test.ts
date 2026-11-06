// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  biblioImageUrl,
  classifyReaderLink,
  headingAnchor,
  projectImageUrl,
  renderReaderMarkdown,
  resolveDocRelative
} from './readerMarkdown.ts'

const DOC = 'scratchpad/archive/some-task/prd.md'

function render(source: string): string {
  return renderReaderMarkdown(source, DOC, biblioImageUrl('/home/someone/project'))
}

test('tables render with their alignment', () => {
  const html = render('| a | b |\n|:-|-:|\n| 1 | 2 |')
  assert.match(html, /<table>/)
  assert.match(html, /<th align="left">a<\/th>/)
  assert.match(html, /<td align="right">2<\/td>/)
})

test('task items draw a checkbox and never a native input', () => {
  const html = render('- [ ] open\n- [x] done\n\n1. [ ] loose\n\n   second paragraph')
  assert.doesNotMatch(html, /<input/)
  assert.equal(html.match(/class="md-check"/g)?.length, 2)
  assert.equal(html.match(/class="md-check checked"/g)?.length, 1)
})

test('relative images load through biblio-image against the doc folder', () => {
  const html = render('![shot](images/a%20b.png "t")')
  const src = /src="([^"]+)"/.exec(html)?.[1] ?? ''
  const url = new URL(src.replace(/&amp;/g, '&'))
  assert.equal(url.protocol, 'biblio-image:')
  assert.equal(url.searchParams.get('f'), 'scratchpad/archive/some-task/images/a b.png')
  assert.equal(url.searchParams.get('p'), '/home/someone/project')
  assert.match(html, /alt="shot"/)
  assert.match(html, /title="t"/)
})

test('an image climbing above biblio renders as text', () => {
  const html = render('![gone](../../../../outside.png)')
  assert.doesNotMatch(html, /<img/)
  assert.match(html, /md-image-missing">gone</)
})

test('an image with a scheme keeps its src', () => {
  assert.match(
    render('![x](https://example.invalid/x.png)'),
    /src="https:\/\/example.invalid\/x.png"/
  )
})

test('headings carry unique GitHub-style anchors', () => {
  const html = render('## Plans & phases\n\n## Plans & phases\n\n#### `code` Title')
  assert.match(html, /<h2 data-anchor="plans--phases">/)
  assert.match(html, /<h2 data-anchor="plans--phases-1">/)
  assert.match(html, /<h4 data-anchor="code-title">/)
  assert.equal(headingAnchor('Design decisions'), 'design-decisions')
})

test('resolveDocRelative walks up and refuses to leave biblio', () => {
  assert.equal(resolveDocRelative('books/a/b.md', '../c/d.md'), 'books/c/d.md')
  assert.equal(resolveDocRelative('books/a/b.md', './e.md'), 'books/a/e.md')
  assert.equal(resolveDocRelative('books/a/b.md', '../../../x.md'), null)
})

test('reader links never navigate the app', () => {
  assert.deepEqual(classifyReaderLink(DOC, 'https://example.invalid/a'), {
    kind: 'external',
    url: 'https://example.invalid/a'
  })
  assert.deepEqual(classifyReaderLink(DOC, 'mailto:a@example.invalid'), {
    kind: 'external',
    url: 'mailto:a@example.invalid'
  })
  assert.deepEqual(classifyReaderLink(DOC, '#plans'), { kind: 'anchor', anchor: 'plans' })
  assert.deepEqual(classifyReaderLink(DOC, 'notes.md#idea'), {
    kind: 'doc',
    path: 'scratchpad/archive/some-task/notes.md',
    anchor: 'idea'
  })
  assert.deepEqual(classifyReaderLink(DOC, 'javascript:alert(1)'), { kind: 'none' })
  assert.deepEqual(classifyReaderLink(DOC, 'file:///etc/passwd'), { kind: 'none' })
  assert.deepEqual(classifyReaderLink(DOC, '/abs/path.md'), { kind: 'none' })
  assert.deepEqual(classifyReaderLink(DOC, '../../../../../x.md'), { kind: 'none' })
})

test('a project file resolves images through project-file against its own folder', () => {
  const html = renderReaderMarkdown(
    '![logo](../assets/logo.png)',
    'docs/guide/README.md',
    projectImageUrl('/home/someone/project')
  )
  const src = /src="([^"]+)"/.exec(html)?.[1] ?? ''
  const url = new URL(src.replace(/&amp;/g, '&'))
  assert.equal(url.protocol, 'project-file:')
  assert.equal(url.searchParams.get('f'), 'docs/assets/logo.png')
  assert.deepEqual(classifyReaderLink('docs/guide/README.md', '../../CONTRIBUTING.md'), {
    kind: 'doc',
    path: 'CONTRIBUTING.md',
    anchor: ''
  })
})
