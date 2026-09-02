import type { PaneLayoutNode, PaneTab } from '../../preload'

// Pure tree operations for the split-pane layout -- a leaf holds a tab
// strip (today's terminal group), a split holds two or more children
// side by side with resizable proportions. Kept dependency-free from
// Vue so the tree shape and its mutations stay easy to reason about
// on their own.

export function createTab(command: string, cwd: string | undefined, label: string): PaneTab {
  return { id: crypto.randomUUID(), label, command, cwd }
}

export function createLeaf(tabs: PaneTab[] = []): PaneLayoutNode {
  return { type: 'leaf', id: crypto.randomUUID(), tabs, activeTabId: tabs[0]?.id ?? null }
}

function replaceNode(
  root: PaneLayoutNode,
  targetId: string,
  replace: (node: PaneLayoutNode) => PaneLayoutNode | null
): PaneLayoutNode | null {
  if (root.id === targetId) return replace(root)
  if (root.type !== 'split') return root

  const children = root.children
    .map((child) => replaceNode(child, targetId, replace))
    .filter((child): child is PaneLayoutNode => child !== null)

  if (children.length === 0) return null
  if (children.length === 1) return children[0] // a lone survivor absorbs the split

  const sizes =
    children.length === root.children.length ? root.sizes : children.map(() => 1 / children.length)
  return { ...root, children, sizes }
}

export function findLeaf(
  root: PaneLayoutNode | null,
  paneId: string
): Extract<PaneLayoutNode, { type: 'leaf' }> | null {
  if (!root) return null
  if (root.type === 'leaf') return root.id === paneId ? root : null
  for (const child of root.children) {
    const found = findLeaf(child, paneId)
    if (found) return found
  }
  return null
}

export function firstLeafId(root: PaneLayoutNode | null): string | null {
  if (!root) return null
  return root.type === 'leaf' ? root.id : firstLeafId(root.children[0])
}

export function countTabs(root: PaneLayoutNode | null): number {
  if (!root) return 0
  if (root.type === 'leaf') return root.tabs.length
  return root.children.reduce((sum, child) => sum + countTabs(child), 0)
}

export function splitPane(
  root: PaneLayoutNode,
  paneId: string,
  direction: 'row' | 'column',
  newLeaf: PaneLayoutNode
): PaneLayoutNode {
  return (
    replaceNode(root, paneId, (leaf) => ({
      type: 'split',
      id: crypto.randomUUID(),
      direction,
      children: [leaf, newLeaf],
      sizes: [0.5, 0.5]
    })) ?? root
  )
}

export function closePane(root: PaneLayoutNode, paneId: string): PaneLayoutNode | null {
  return replaceNode(root, paneId, () => null)
}

export function closeTab(root: PaneLayoutNode, paneId: string, tabId: string): PaneLayoutNode | null {
  return replaceNode(root, paneId, (leaf) => {
    if (leaf.type !== 'leaf') return leaf
    const tabs = leaf.tabs.filter((tab) => tab.id !== tabId)
    if (tabs.length === 0) return null // the pane closes with its last tab
    const activeTabId = leaf.activeTabId === tabId ? tabs[tabs.length - 1].id : leaf.activeTabId
    return { ...leaf, tabs, activeTabId }
  })
}

export function addTabToPane(root: PaneLayoutNode, paneId: string, tab: PaneTab): PaneLayoutNode {
  return (
    replaceNode(root, paneId, (leaf) =>
      leaf.type === 'leaf' ? { ...leaf, tabs: [...leaf.tabs, tab], activeTabId: tab.id } : leaf
    ) ?? root
  )
}

export function setActiveTab(root: PaneLayoutNode, paneId: string, tabId: string): PaneLayoutNode {
  return (
    replaceNode(root, paneId, (leaf) =>
      leaf.type === 'leaf' ? { ...leaf, activeTabId: tabId } : leaf
    ) ?? root
  )
}

export function setSplitSizes(root: PaneLayoutNode, splitId: string, sizes: number[]): PaneLayoutNode {
  return (
    replaceNode(root, splitId, (node) => (node.type === 'split' ? { ...node, sizes } : node)) ?? root
  )
}
