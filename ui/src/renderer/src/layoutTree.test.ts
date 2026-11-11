// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { createLeaf, findLeaf, placeTab, type LayoutLeaf, type LayoutNode } from './layoutTree.ts'

type Tab = { id: string }

function leafOf(ids: string[], active = ids[0]): LayoutLeaf<Tab> {
  return { ...createLeaf(ids.map((id) => ({ id }))), activeTabId: active }
}

function idsOf(root: LayoutNode<Tab> | null | undefined, paneId: string): string[] {
  return findLeaf(root ?? null, paneId)?.tabs.map((t) => t.id) ?? []
}

function split(...children: LayoutLeaf<Tab>[]): LayoutNode<Tab> {
  return { type: 'split', id: 's', direction: 'row', children, sizes: children.map(() => 0.5) }
}

test('reorder moves a tab right, counting the index before the move', () => {
  const leaf = leafOf(['a', 'b', 'c', 'd'])
  const result = placeTab(leaf, leaf.id, 'a', leaf.id, 'center', false, 3)
  assert.deepEqual(idsOf(result?.root, leaf.id), ['b', 'c', 'a', 'd'])
  assert.equal(findLeaf(result!.root, leaf.id)?.activeTabId, 'a')
})

test('reorder moves a tab left and to the end', () => {
  const leaf = leafOf(['a', 'b', 'c'])
  assert.deepEqual(
    idsOf(placeTab(leaf, leaf.id, 'c', leaf.id, 'center', false, 0)?.root, leaf.id),
    ['c', 'a', 'b']
  )
  assert.deepEqual(
    idsOf(placeTab(leaf, leaf.id, 'a', leaf.id, 'center', false, 3)?.root, leaf.id),
    ['b', 'c', 'a']
  )
})

test('dropping a tab on its own slot is a no-op', () => {
  const leaf = leafOf(['a', 'b', 'c'])
  assert.equal(placeTab(leaf, leaf.id, 'b', leaf.id, 'center', false, 1), null)
  assert.equal(placeTab(leaf, leaf.id, 'b', leaf.id, 'center', false, 2), null)
})

test('a center drop on the own pane without an index stays a no-op', () => {
  const leaf = leafOf(['a', 'b'])
  assert.equal(placeTab(leaf, leaf.id, 'a', leaf.id, 'center', false), null)
})

test('moving to another pane inserts at the index', () => {
  const left = leafOf(['a', 'b'])
  const right = leafOf(['x', 'y'])
  const result = placeTab(split(left, right), left.id, 'a', right.id, 'center', false, 1)
  assert.deepEqual(idsOf(result?.root, left.id), ['b'])
  assert.deepEqual(idsOf(result?.root, right.id), ['x', 'a', 'y'])
  assert.equal(findLeaf(result!.root, right.id)?.activeTabId, 'a')
})

test('moving to another pane without an index appends', () => {
  const left = leafOf(['a', 'b'])
  const right = leafOf(['x', 'y'])
  const result = placeTab(split(left, right), left.id, 'a', right.id, 'center', false)
  assert.deepEqual(idsOf(result?.root, right.id), ['x', 'y', 'a'])
})

test('duplicating within a pane inserts a copy at the index', () => {
  const leaf = leafOf(['a', 'b'])
  const result = placeTab(leaf, leaf.id, 'b', leaf.id, 'center', true, 0)
  const ids = idsOf(result?.root, leaf.id)
  assert.equal(ids.length, 3)
  assert.equal(ids[0], result?.placed.id)
  assert.deepEqual(ids.slice(1), ['a', 'b'])
})
