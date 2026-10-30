// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import {
  EditorState,
  RangeSetBuilder,
  StateEffect,
  StateField,
  type Extension,
  type Text,
  type Transaction,
  type TransactionSpec
} from '@codemirror/state'
import {
  Decoration,
  EditorView,
  GutterMarker,
  WidgetType,
  gutter,
  drawSelection,
  keymap,
  lineNumbers,
  type DecorationSet
} from '@codemirror/view'
import { defaultKeymap, history, historyKeymap, invertedEffects } from '@codemirror/commands'
import type { GitLineRange } from '../../preload/types'
import type { ConflictSide, MiddleChunk, SideDecision } from './conflictModel'

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

const setDecisions = StateEffect.define<Record<number, SideDecision>>()

// What the middle decided for this pane's side of each conflict, pushed in from the middle.
const sideDecisions = StateField.define<Record<number, SideDecision>>({
  create: () => ({}),
  update(value, tr) {
    for (const effect of tr.effects) if (effect.is(setDecisions)) value = effect.value
    return value
  }
})

export type OnDecide = (index: number, decision: 'taken' | 'rejected') => void

const CHEVRONS = {
  left: 'M3 3.5l3.5 3.5L3 10.5M7.5 3.5L11 7l-3.5 3.5',
  right: 'M11 3.5L7.5 7l3.5 3.5M6.5 3.5L3 7l3.5 3.5'
}
const CROSS = 'M4 4l6 6M10 4l-6 6'

function iconButton(className: string, path: string, label: string, action: string): HTMLElement {
  const button = document.createElement('button')
  button.className = `cm-decide-button ${className}`
  button.title = label
  button.setAttribute('aria-label', label)
  button.dataset.decide = action
  button.innerHTML = `<svg width="12" height="12" viewBox="0 0 14 14" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"><path d="${path}"/></svg>`
  return button
}

// The chevron takes this pane's side into the middle and the cross rejects it. The decided one
// stays lit.
class DecideMarker extends GutterMarker {
  readonly side: ConflictSide
  readonly decision: SideDecision

  constructor(side: ConflictSide, decision: SideDecision) {
    super()
    this.side = side
    this.decision = decision
  }

  eq(other: DecideMarker): boolean {
    return other.side === this.side && other.decision === this.decision
  }

  toDOM(): Node {
    const dom = document.createElement('span')
    dom.className = 'cm-decide'
    const lit = (own: SideDecision): string =>
      this.decision === 'open' ? '' : this.decision === own ? ' on' : ' off'
    const take = iconButton(
      `take${lit('taken')}`,
      CHEVRONS[this.side],
      `Take the ${this.side} side`,
      'taken'
    )
    const reject = iconButton(
      `reject${lit('rejected')}`,
      CROSS,
      `Reject the ${this.side} side`,
      'rejected'
    )
    // The chevron points into the middle, so it sits on the middle's side of the cross.
    if (this.side === 'left') dom.append(reject, take)
    else dom.append(take, reject)
    return dom
  }
}

// The conflict whose first line is this one, counting an empty side at the line after it.
function sideChunkAt(state: EditorState, lineNumber: number): SideChunk | undefined {
  return state
    .field(sideChunks)
    .find(({ range }) => Math.min(range.start, state.doc.lines) === lineNumber)
}

// The decision gutter sits next to the middle: after the text in the left pane, before it in the
// right pane, so the two side panes mirror each other.
function decisionGutter(side: ConflictSide, onDecide: OnDecide): Extension {
  return gutter({
    class: 'cm-conflict-decide',
    side: side === 'left' ? 'after' : 'before',
    lineMarker(view, line) {
      const chunk = sideChunkAt(view.state, view.state.doc.lineAt(line.from).number)
      if (!chunk) return null
      return new DecideMarker(side, view.state.field(sideDecisions)[chunk.index] ?? 'open')
    },
    lineMarkerChange: (update) =>
      update.startState.field(sideDecisions) !== update.state.field(sideDecisions),
    initialSpacer: () => new DecideMarker(side, 'open'),
    domEventHandlers: {
      mousedown(view, line, event) {
        const action = (event.target as HTMLElement).closest('[data-decide]')
        if (!action) return false
        const chunk = sideChunkAt(view.state, view.state.doc.lineAt(line.from).number)
        if (chunk) onDecide(chunk.index, action.getAttribute('data-decide') as 'taken' | 'rejected')
        event.preventDefault()
        return true
      }
    }
  })
}

class LineNumberMarker extends GutterMarker {
  readonly number: number

  constructor(number: number) {
    super()
    this.number = number
  }

  eq(other: LineNumberMarker): boolean {
    return other.number === this.number
  }

  toDOM(): Node {
    return document.createTextNode(String(this.number))
  }
}

// lineNumbers() can only sit before the text, and the left pane shows its numbers after it, next
// to the middle. The cm-lineNumbers class gives it the same look.
const lineNumbersAfter = gutter({
  class: 'cm-lineNumbers cm-lineNumbers-after',
  side: 'after',
  lineMarker: (view, line) => new LineNumberMarker(view.state.doc.lineAt(line.from).number),
  lineMarkerChange: (update) => update.docChanged,
  initialSpacer: (view) => new LineNumberMarker(view.state.doc.lines)
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
    el.textContent = this.resolved
      ? 'Resolved as empty'
      : 'No lines here. Decide each side or type.'
    return el
  }
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

const middleDecorations = EditorView.decorations.compute(
  ['doc', middleChunks, currentChunk],
  (state) => {
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
    const empty = chunks
      .filter((chunk) => chunk.from === chunk.to)
      .map((chunk) =>
        Decoration.widget({
          widget: new EmptyChunkWidget(chunk.resolved),
          block: true,
          side: -1
        }).range(chunk.from)
      )
    return empty.length ? lines.update({ add: empty, sort: true }) : lines
  }
)

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
    return lines
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
  // The base theme draws a light border between the text and a gutter after it.
  '.cm-gutters.cm-gutters-after': { border: 'none !important' },
  '&.cm-focused': { outline: 'none' },
  '.cm-cursor': { borderLeftColor: 'var(--color-amber)' },
  '.cm-selectionBackground, &.cm-focused .cm-selectionBackground': {
    backgroundColor: 'var(--color-amber-muted) !important'
  },
  // Conflicts are sapphire in the sides and jade in the middle, kept faint so syntax colors can
  // come later, and never amber, which marks comments.
  '.cm-conflict-unresolved': { backgroundColor: 'var(--color-jade-muted)' },
  '.cm-conflict-resolved': { boxShadow: 'inset 1px 0 0 var(--color-jade-dim)' },
  '.cm-conflict-side': { backgroundColor: 'var(--color-sapphire-muted)' },
  '.cm-conflict-side.cm-conflict-current': { boxShadow: 'inset 2px 0 0 var(--color-sapphire)' },
  '.cm-conflict-current': { boxShadow: 'inset 2px 0 0 var(--color-jade)' },
  '.cm-conflict-empty': {
    padding: '0 var(--space-2)',
    fontFamily: 'var(--font-body)',
    fontStyle: 'italic',
    color: 'var(--color-fg-muted)',
    boxShadow: 'inset 1px 0 0 var(--color-jade-dim)'
  },
  '.cm-conflict-decide .cm-gutterElement': { display: 'flex', alignItems: 'center' },
  '.cm-decide': {
    display: 'flex',
    alignItems: 'center',
    gap: '2px',
    padding: '0 2px'
  },
  '.cm-decide-button': {
    display: 'flex',
    padding: '1px',
    border: 'none',
    borderRadius: 'var(--radius-sm)',
    background: 'none',
    cursor: 'pointer'
  },
  // Both controls are drawn in full until a side is decided, then the one not chosen dims.
  '.cm-decide-button.take': { color: 'var(--color-jade)' },
  '.cm-decide-button.reject': { color: 'var(--color-error)' },
  '.cm-decide-button.on': { backgroundColor: 'var(--color-surface-2)' },
  '.cm-decide-button.off': { color: 'var(--color-fg-disabled)' },
  '.cm-decide-button.take.off:hover': { color: 'var(--color-jade)' },
  '.cm-decide-button.reject.off:hover': { color: 'var(--color-error)' },
  '.cm-lineNumbers-after .cm-gutterElement': { textAlign: 'left', padding: '0 5px 0 3px' },
  '.cm-conflict-hidden': {
    padding: '0 var(--space-2)',
    fontFamily: 'var(--font-body)',
    fontStyle: 'italic',
    color: 'var(--color-fg-muted)',
    backgroundColor: 'var(--color-surface-1)'
  },
  '.cm-conflict-empty.unresolved': { backgroundColor: 'var(--color-jade-muted)' }
})

function base(extra: Extension[]): Extension[] {
  return [drawSelection(), currentChunk, theme, ...extra]
}

export function middleState(
  doc: string,
  chunks: MiddleChunk[],
  onUpdate: (state: EditorState, docChanged: boolean) => void,
  extra: Extension[]
): EditorState {
  return EditorState.create({
    doc,
    extensions: base([
      lineNumbers(),
      middleChunks.init(() => chunks),
      undoChunks,
      middleDecorations,
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

export function sideState(
  doc: string,
  chunks: SideChunk[],
  side: ConflictSide,
  onDecide: OnDecide,
  extra: Extension[]
): EditorState {
  return EditorState.create({
    doc,
    extensions: base([
      sideChunks.init(() => chunks),
      sideDecisions,
      sideDecorations,
      // Read only but still editable as a view, so lines can be selected to comment on them.
      EditorState.readOnly.of(true),
      // Gutters on one side line up in extension order, so the decision gutter comes nearest the
      // middle and the extras' gutters, such as the comment one, end up on the outer edge.
      side === 'left'
        ? [extra, lineNumbersAfter, decisionGutter(side, onDecide)]
        : [decisionGutter(side, onDecide), lineNumbers(), extra]
    ])
  })
}

export function showDecisions(view: EditorView, decisions: Record<number, SideDecision>): void {
  view.dispatch({ effects: setDecisions.of(decisions) })
}

// Rewrites conflicts in the middle after a decision, as one undoable step. Each update carries the
// chunk's new decisions and its new text; positions are worked out here.
export function chunkReplacements(
  state: EditorState,
  updates: { chunk: MiddleChunk; text: string }[]
): TransactionSpec | null {
  const current = state.field(middleChunks)
  const sorted = updates
    .map((update) => ({ ...update, old: current.find((c) => c.index === update.chunk.index) }))
    .filter((update): update is typeof update & { old: MiddleChunk } => update.old !== undefined)
    .sort((a, b) => a.old.from - b.old.from)
  if (sorted.length === 0) return null
  let shift = 0
  const effects = sorted.map(({ chunk, text, old }) => {
    const from = old.from + shift
    shift += text.length - (old.to - old.from)
    return setChunk.of({ ...chunk, from, to: from + text.length })
  })
  const first = sorted[0].old.from
  return {
    changes: sorted.map(({ text, old }) => ({ from: old.from, to: old.to, insert: text })),
    effects:
      sorted.length === 1
        ? [...effects, EditorView.scrollIntoView(first, { y: 'center' })]
        : effects,
    selection: sorted.length === 1 ? { anchor: first } : undefined
  }
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
