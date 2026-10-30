// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import {
  EditorState,
  RangeSetBuilder,
  StateEffect,
  StateField,
  type Extension,
  type Range,
  type Text,
  type Transaction,
  type TransactionSpec
} from '@codemirror/state'
import {
  Decoration,
  EditorView,
  WidgetType,
  drawSelection,
  keymap,
  lineNumbers,
  type DecorationSet
} from '@codemirror/view'
import { defaultKeymap, history, historyKeymap, invertedEffects } from '@codemirror/commands'
import type { GitLineRange } from '../../preload/types'
import type { MiddleChunk } from './conflictModel'

// A conflict's place in a side pane, which never changes since the side panes are read only.
export interface SideChunk {
  index: number
  range: GitLineRange
}

const setChunk = StateEffect.define<MiddleChunk>()
const setCurrent = StateEffect.define<number | null>()

// The middle's conflicts follow every edit. An edit that touches a conflict resolves it, and an
// empty conflict takes text typed at its position, since there is nowhere else for it to go.
export const middleChunks = StateField.define<MiddleChunk[]>({
  create: () => [],
  update(chunks, tr) {
    let next = chunks
    if (tr.docChanged) {
      next = chunks.map((chunk) => {
        let touched = false
        tr.changes.iterChangedRanges((fromA, toA) => {
          if (
            fromA === toA
              ? fromA >= chunk.from && fromA < chunk.to
              : fromA < chunk.to && toA > chunk.from
          ) {
            touched = true
          }
          if (chunk.from === chunk.to && fromA <= chunk.from && toA >= chunk.from) touched = true
        })
        const from = tr.changes.mapPos(chunk.from, -1)
        const to = tr.changes.mapPos(chunk.to, chunk.from === chunk.to ? 1 : -1)
        return { ...chunk, from, to: Math.max(from, to), resolved: chunk.resolved || touched }
      })
    }
    for (const effect of tr.effects) {
      if (effect.is(setChunk)) {
        next = next.map((chunk) => (chunk.index === effect.value.index ? effect.value : chunk))
      }
    }
    return next
  }
})

// Undoing a take puts the conflict back as it was, unresolved included.
const undoChunks = invertedEffects.of((tr) => {
  const before = tr.startState.field(middleChunks, false)
  if (!before) return []
  return tr.effects.flatMap((effect) => {
    if (!effect.is(setChunk)) return []
    const old = before.find((chunk) => chunk.index === effect.value.index)
    return old ? [setChunk.of(old)] : []
  })
})

const currentChunk = StateField.define<number | null>({
  create: () => null,
  update(value, tr) {
    for (const effect of tr.effects) if (effect.is(setCurrent)) value = effect.value
    return value
  }
})

const sideChunks = StateField.define<SideChunk[]>({
  create: () => [],
  update: (value) => value
})

class EmptyChunkWidget extends WidgetType {
  readonly resolved: boolean

  constructor(resolved: boolean) {
    super()
    this.resolved = resolved
  }

  eq(other: EmptyChunkWidget): boolean {
    return other.resolved === this.resolved
  }

  toDOM(): HTMLElement {
    const el = document.createElement('div')
    el.className = `cm-conflict-empty ${this.resolved ? 'resolved' : 'unresolved'}`
    el.textContent = this.resolved ? 'Resolved as empty' : 'No lines here. Take a side or type.'
    return el
  }
}

// Mounts the middle pane's bar for one conflict into dom and returns its unmount.
export type MountChunkBar = (dom: HTMLElement, bar: ChunkBarProps) => () => void

export interface ChunkBarProps {
  index: number
  label: string
  resolved: boolean
}

const BAR_HEIGHT = 30

const mountedBars = new WeakMap<HTMLElement, () => void>()

// A bar above each conflict in all three panes, at the same height, so the panes stay aligned. The
// middle's bar carries the take buttons, the side panes' bars only the label.
class ChunkBarWidget extends WidgetType {
  readonly bar: ChunkBarProps
  readonly mount: MountChunkBar | null

  constructor(bar: ChunkBarProps, mount: MountChunkBar | null) {
    super()
    this.bar = bar
    this.mount = mount
  }

  eq(other: ChunkBarWidget): boolean {
    return (
      other.bar.index === this.bar.index &&
      other.bar.label === this.bar.label &&
      other.bar.resolved === this.bar.resolved
    )
  }

  get estimatedHeight(): number {
    return BAR_HEIGHT
  }

  toDOM(): HTMLElement {
    const dom = document.createElement('div')
    dom.className = 'cm-conflict-bar'
    dom.contentEditable = 'false'
    if (this.mount) mountedBars.set(dom, this.mount(dom, this.bar))
    else dom.textContent = this.bar.label
    return dom
  }

  destroy(dom: HTMLElement): void {
    mountedBars.get(dom)?.()
    mountedBars.delete(dom)
  }

  // The bar's buttons handle their own clicks, which the editor must not take.
  ignoreEvent(): boolean {
    return true
  }
}

function barWidget(bar: ChunkBarProps, mount: MountChunkBar | null, at: number): Range<Decoration> {
  return Decoration.widget({ widget: new ChunkBarWidget(bar, mount), block: true, side: -2 }).range(
    at
  )
}

class HiddenLinesWidget extends WidgetType {
  readonly count: number

  constructor(count: number) {
    super()
    this.count = count
  }

  eq(other: HiddenLinesWidget): boolean {
    return other.count === this.count
  }

  toDOM(): HTMLElement {
    const dom = document.createElement('div')
    dom.className = 'cm-conflict-hidden'
    dom.textContent = `${this.count} lines hidden`
    return dom
  }
}

const setFocus = StateEffect.define<boolean>()

// Focus hides the clean lines far from any conflict. spans gives the lines to hide in a state, and
// is asked again on every edit and on every change for which recompute says yes.
export function focusLines(
  spans: (state: EditorState) => GitLineRange[],
  recompute: (tr: Transaction) => boolean
): Extension {
  const build = (state: EditorState): DecorationSet =>
    Decoration.set(
      spans(state)
        .filter((span) => span.start >= 1 && span.end <= state.doc.lines && span.end > span.start)
        .map((span) =>
          Decoration.replace({
            widget: new HiddenLinesWidget(span.end - span.start + 1),
            block: true
          }).range(state.doc.line(span.start).from, state.doc.line(span.end).to)
        )
    )
  return StateField.define<{ on: boolean; lines: DecorationSet }>({
    create: () => ({ on: false, lines: Decoration.none }),
    update(value, tr) {
      let on = value.on
      for (const effect of tr.effects) if (effect.is(setFocus)) on = effect.value
      if (!on) return value.on ? { on, lines: Decoration.none } : value
      if (on === value.on && !tr.docChanged && !recompute(tr)) return value
      return { on, lines: build(tr.state) }
    },
    provide: (field) => EditorView.decorations.from(field, (value) => value.lines)
  })
}

export function setFocusLines(view: EditorView, on: boolean): void {
  view.dispatch({ effects: setFocus.of(on) })
}

function lineClasses(
  doc: Text,
  ranges: { from: number; to: number; className: string }[]
): DecorationSet {
  const builder = new RangeSetBuilder<Decoration>()
  for (const range of ranges) {
    if (range.from === range.to) continue
    const first = doc.lineAt(range.from).number
    const last = doc.lineAt(range.to - 1).number
    for (let line = first; line <= last; line++) {
      builder.add(
        doc.line(line).from,
        doc.line(line).from,
        Decoration.line({ class: range.className })
      )
    }
  }
  return builder.finish()
}

function chunkClass(index: number, current: number | null, resolved: boolean): string {
  const state = resolved ? 'cm-conflict-resolved' : 'cm-conflict-unresolved'
  return index === current ? `${state} cm-conflict-current` : state
}

function middleDecorations(mount: MountChunkBar): Extension {
  return EditorView.decorations.compute(['doc', middleChunks, currentChunk], (state) => {
    const current = state.field(currentChunk)
    const chunks = state.field(middleChunks)
    const lines = lineClasses(
      state.doc,
      chunks.map((chunk) => ({
        from: chunk.from,
        to: chunk.to,
        className: chunkClass(chunk.index, current, chunk.resolved)
      }))
    )
    const widgets = chunks.flatMap((chunk, at) => {
      const bar = barWidget(
        { index: chunk.index, label: `Conflict ${at + 1}`, resolved: chunk.resolved },
        mount,
        chunk.from
      )
      if (chunk.from !== chunk.to) return [bar]
      const empty = Decoration.widget({
        widget: new EmptyChunkWidget(chunk.resolved),
        block: true,
        side: -1
      }).range(chunk.from)
      return [bar, empty]
    })
    return lines.update({ add: widgets, sort: true })
  })
}

const sideDecorations = EditorView.decorations.compute(
  ['doc', sideChunks, currentChunk],
  (state) => {
    const current = state.field(currentChunk)
    const doc = state.doc
    const chunks = state.field(sideChunks).filter(({ range }) => range.start <= doc.lines)
    const lines = lineClasses(
      doc,
      chunks.flatMap(({ index, range }) => {
        if (range.end < range.start) return []
        const last = doc.line(Math.min(range.end, doc.lines))
        return [
          {
            from: doc.line(range.start).from,
            to: Math.min(last.to + 1, doc.length),
            className:
              index === current ? 'cm-conflict-side cm-conflict-current' : 'cm-conflict-side'
          }
        ]
      })
    )
    // An empty side sits after the line before it, which is the next line's start.
    const bars = chunks.map(({ index, range }, at) =>
      barWidget(
        { index, label: `Conflict ${at + 1}`, resolved: false },
        null,
        range.start > doc.lines ? doc.length : doc.line(range.start).from
      )
    )
    return lines.update({ add: bars, sort: true })
  }
)

const theme = EditorView.theme({
  '&': {
    height: '100%',
    fontSize: 'var(--text-xs)',
    backgroundColor: 'var(--color-surface-0)',
    color: 'var(--color-fg-primary)'
  },
  '.cm-scroller': { fontFamily: 'var(--font-mono)', lineHeight: '1.55' },
  // Chromium paints the corner where both scrollbars meet white unless told otherwise.
  '.cm-scroller::-webkit-scrollbar-corner': { background: 'transparent' },
  '.cm-gutters': {
    backgroundColor: 'var(--color-surface-0)',
    color: 'var(--color-fg-disabled)',
    border: 'none'
  },
  '&.cm-focused': { outline: 'none' },
  '.cm-cursor': { borderLeftColor: 'var(--color-amber)' },
  '.cm-selectionBackground, &.cm-focused .cm-selectionBackground': {
    backgroundColor: 'var(--color-amber-muted) !important'
  },
  '.cm-conflict-unresolved': { backgroundColor: 'var(--color-error-muted)' },
  '.cm-conflict-resolved': { backgroundColor: 'var(--color-success-muted)' },
  '.cm-conflict-side': { backgroundColor: 'var(--color-amber-muted)' },
  '.cm-conflict-current': { boxShadow: 'inset 2px 0 0 var(--color-amber)' },
  '.cm-conflict-empty': {
    padding: '0 var(--space-2)',
    fontFamily: 'var(--font-body)',
    fontStyle: 'italic',
    color: 'var(--color-fg-muted)',
    boxShadow: 'inset 2px 0 0 var(--color-amber)'
  },
  '.cm-conflict-bar': {
    height: `${BAR_HEIGHT}px`,
    boxSizing: 'border-box',
    display: 'flex',
    alignItems: 'center',
    padding: '0 var(--space-2)',
    fontFamily: 'var(--font-body)',
    color: 'var(--color-fg-secondary)',
    borderTop: '1px solid var(--color-border-subtle)'
  },
  '.cm-conflict-hidden': {
    padding: '0 var(--space-2)',
    fontFamily: 'var(--font-body)',
    fontStyle: 'italic',
    color: 'var(--color-fg-muted)',
    backgroundColor: 'var(--color-surface-1)'
  },
  '.cm-conflict-empty.unresolved': { backgroundColor: 'var(--color-error-muted)' },
  '.cm-conflict-empty.resolved': { backgroundColor: 'var(--color-success-muted)' }
})

function base(extra: Extension[]): Extension[] {
  return [lineNumbers(), drawSelection(), currentChunk, theme, ...extra]
}

export function middleState(
  doc: string,
  chunks: MiddleChunk[],
  onUpdate: (state: EditorState, docChanged: boolean) => void,
  mountBar: MountChunkBar,
  extra: Extension[]
): EditorState {
  return EditorState.create({
    doc,
    extensions: base([
      middleChunks.init(() => chunks),
      undoChunks,
      middleDecorations(mountBar),
      history(),
      keymap.of([...defaultKeymap, ...historyKeymap]),
      EditorView.updateListener.of((update) => {
        if (update.docChanged || update.selectionSet || update.transactions.length) {
          onUpdate(update.state, update.docChanged)
        }
      }),
      extra
    ])
  })
}

export function sideState(doc: string, chunks: SideChunk[], extra: Extension[]): EditorState {
  return EditorState.create({
    doc,
    extensions: base([
      sideChunks.init(() => chunks),
      sideDecorations,
      // Read only but still editable as a view, so lines can be selected to comment on them.
      EditorState.readOnly.of(true),
      extra
    ])
  })
}

// Replaces a conflict in the middle with one side, both, or its text from before, as one undoable
// step that leaves the cursor at its start.
export function chunkReplacement(
  state: EditorState,
  index: number,
  text: string,
  resolved: boolean
): TransactionSpec | null {
  const chunk = state.field(middleChunks).find((c) => c.index === index)
  if (!chunk) return null
  return {
    changes: { from: chunk.from, to: chunk.to, insert: text },
    effects: [
      setChunk.of({ index, from: chunk.from, to: chunk.from + text.length, resolved }),
      EditorView.scrollIntoView(chunk.from, { y: 'center' })
    ],
    selection: { anchor: chunk.from }
  }
}

export function replaceChunk(
  view: EditorView,
  index: number,
  text: string,
  resolved: boolean
): void {
  const spec = chunkReplacement(view.state, index, text, resolved)
  if (spec) view.dispatch(spec)
}

export function showCurrent(view: EditorView, index: number | null): void {
  if (view.state.field(currentChunk) !== index) view.dispatch({ effects: setCurrent.of(index) })
}

// The conflict the cursor sits in, counting the position right after an empty one.
export function chunkAtCursor(state: EditorState): number | null {
  const head = state.selection.main.head
  const chunk = state
    .field(middleChunks)
    .find((c) => (c.from === c.to ? head === c.from : head >= c.from && head < c.to))
  return chunk ? chunk.index : null
}

// A middle chunk's lines, 1-based and inclusive, with an empty chunk as end = start - 1.
export function chunkLines(doc: Text, chunk: MiddleChunk): GitLineRange {
  const start = doc.lineAt(chunk.from).number
  return { start, end: chunk.from === chunk.to ? start - 1 : doc.lineAt(chunk.to - 1).number }
}
