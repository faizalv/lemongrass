// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import { createApp, watch, type App } from 'vue'
import {
  Facet,
  StateEffect,
  StateField,
  type EditorState,
  type Extension,
  type Range,
  type Transaction
} from '@codemirror/state'
import {
  Decoration,
  EditorView,
  GutterMarker,
  ViewPlugin,
  WidgetType,
  gutter,
  type DecorationSet
} from '@codemirror/view'
import AnnotationCard from './components/review/AnnotationCard.vue'

// Line numbers here are 1-based, inclusive, and in the editor's current document.
export interface LineRange {
  fromLine: number
  toLine: number
}

export interface AnnotationRange extends LineRange {
  id: string
  comment: string
}

// The annotator kind supplies everything that knows what a comment is anchored to, so this
// extension only ever sees line ranges of the editor it sits in.
export interface AnnotationGutterConfig {
  // Read inside a Vue watcher, so a store change redraws the markers.
  ranges: () => AnnotationRange[]
  // Ranges commented as a whole from a button on their first line, such as conflict chunks.
  targets: () => LineRange[]
  canAdd: (range: LineRange) => boolean
  onAdd: (range: LineRange, quote: string, comment: string) => void
  onEdit: (id: string, comment: string) => void
  onRemove: (id: string) => void
}

const configFacet = Facet.define<AnnotationGutterConfig, AnnotationGutterConfig | null>({
  combine: (values) => values[0] ?? null
})

const refreshEffect = StateEffect.define<null>()

// Redraws the comment markers, for a caller that replaced the document wholesale.
export function refreshAnnotations(view: EditorView): void {
  view.dispatch({ effects: refreshEffect.of(null) })
}

interface Draft {
  from: number
  to: number
}

const setDraft = StateEffect.define<Draft | null>()

interface Info {
  ranges: AnnotationRange[]
  targets: LineRange[]
  commentedLines: Set<number>
  targetStarts: Set<number>
}

function validRange(state: EditorState, range: LineRange): boolean {
  return range.fromLine >= 1 && range.fromLine <= range.toLine && range.toLine <= state.doc.lines
}

export function lineRangeQuote(state: EditorState, range: LineRange): string {
  const from = state.doc.line(range.fromLine).from
  const to = state.doc.line(range.toLine).to
  return state.sliceDoc(from, to)
}

// A selection that ends at the start of a line does not include that line.
export function selectionLines(state: EditorState, from: number, to: number): LineRange {
  const first = state.doc.lineAt(from)
  let last = state.doc.lineAt(to)
  if (to > from && to === last.from && last.number > first.number) {
    last = state.doc.line(last.number - 1)
  }
  return { fromLine: first.number, toLine: last.number }
}

const mountedCards = new WeakMap<HTMLElement, App>()

class CardWidget extends WidgetType {
  constructor(
    readonly key: string,
    readonly comment: string,
    readonly draft: boolean,
    readonly handlers: {
      save: (view: EditorView, comment: string) => void
      cancel: (view: EditorView) => void
      remove: () => void
    }
  ) {
    super()
  }

  eq(other: CardWidget): boolean {
    return other.key === this.key && other.comment === this.comment && other.draft === this.draft
  }

  toDOM(view: EditorView): HTMLElement {
    const dom = document.createElement('div')
    dom.className = 'cm-annotation-card'
    dom.contentEditable = 'false'
    const app = createApp(AnnotationCard, {
      comment: this.comment,
      draft: this.draft,
      onSave: (comment: string) => this.handlers.save(view, comment),
      onCancel: () => this.handlers.cancel(view),
      onRemove: () => this.handlers.remove()
    })
    app.mount(dom)
    mountedCards.set(dom, app)
    return dom
  }

  destroy(dom: HTMLElement): void {
    mountedCards.get(dom)?.unmount()
    mountedCards.delete(dom)
  }

  // The card's textarea and buttons handle their own input, which the editor must not take.
  ignoreEvent(): boolean {
    return true
  }
}

class CommentMarker extends GutterMarker {
  elementClass = 'cm-annotation-commented'
  toDOM(): Node {
    const dom = document.createElement('span')
    dom.className = 'cm-annotation-dot'
    dom.title = 'Commented'
    return dom
  }
}

class AddMarker extends GutterMarker {
  constructor(readonly target: boolean) {
    super()
  }
  eq(other: AddMarker): boolean {
    return other.target === this.target
  }
  toDOM(): Node {
    const dom = document.createElement('span')
    dom.className = this.target ? 'cm-annotation-add target' : 'cm-annotation-add'
    dom.textContent = '+'
    dom.title = this.target ? 'Comment on this chunk' : 'Comment on this line or the selection'
    return dom
  }
}

const commentMarker = new CommentMarker()
const addMarker = new AddMarker(false)
const targetMarker = new AddMarker(true)

function readInfo(state: EditorState): Info {
  const config = state.facet(configFacet)
  if (!config)
    return { ranges: [], targets: [], commentedLines: new Set(), targetStarts: new Set() }
  const ranges = config.ranges().filter((range) => validRange(state, range))
  const targets = config.targets().filter((range) => validRange(state, range))
  const commentedLines = new Set<number>()
  for (const range of ranges) {
    for (let line = range.fromLine; line <= range.toLine; line++) commentedLines.add(line)
  }
  return {
    ranges,
    targets,
    commentedLines,
    targetStarts: new Set(targets.map((t) => t.fromLine))
  }
}

const infoField = StateField.define<Info>({
  create: readInfo,
  update(info, tr) {
    if (tr.docChanged || tr.effects.some((e) => e.is(refreshEffect))) return readInfo(tr.state)
    return info
  }
})

const draftField = StateField.define<Draft | null>({
  create: () => null,
  update(draft, tr) {
    for (const effect of tr.effects) if (effect.is(setDraft)) return effect.value
    if (draft && tr.docChanged) {
      return { from: tr.changes.mapPos(draft.from, -1), to: tr.changes.mapPos(draft.to, 1) }
    }
    return draft
  }
})

// The commented lines and an open draft, for an extension that hides lines, such as a collapse
// of unchanged lines, which must keep these shown.
export function commentedLineRanges(state: EditorState): LineRange[] {
  const info = state.field(infoField, false)
  if (!info) return []
  const ranges: LineRange[] = info.ranges.map(({ fromLine, toLine }) => ({ fromLine, toLine }))
  const draft = state.field(draftField, false)
  if (draft) ranges.push(selectionLines(state, draft.from, draft.to))
  return ranges
}

// True when a transaction may have changed commentedLineRanges without changing the document.
export function isAnnotationChange(tr: Transaction): boolean {
  return tr.effects.some((e) => e.is(refreshEffect) || e.is(setDraft))
}

export function annotationGutter(config: AnnotationGutterConfig): Extension {
  const handlers = (id: string): CardWidget['handlers'] => ({
    save: (view, comment) => {
      if (id !== '') return config.onEdit(id, comment)
      const draft = view.state.field(draftField)
      if (!draft) return
      const range = selectionLines(view.state, draft.from, draft.to)
      config.onAdd(range, lineRangeQuote(view.state, range), comment)
      view.dispatch({ effects: setDraft.of(null) })
    },
    cancel: (view) => view.dispatch({ effects: setDraft.of(null) }),
    remove: () => config.onRemove(id)
  })

  function buildDecorations(state: EditorState): DecorationSet {
    const info = state.field(infoField)
    const draft = state.field(draftField)
    const decorations: Range<Decoration>[] = []
    for (const range of info.ranges) {
      for (let line = range.fromLine; line <= range.toLine; line++) {
        decorations.push(commentedLine.range(state.doc.line(line).from))
      }
      const card = new CardWidget(range.id, range.comment, false, handlers(range.id))
      decorations.push(
        Decoration.widget({ widget: card, block: true, side: 1 }).range(
          state.doc.line(range.toLine).to
        )
      )
    }
    if (draft) {
      const range = selectionLines(state, draft.from, draft.to)
      for (let line = range.fromLine; line <= range.toLine; line++) {
        decorations.push(draftLine.range(state.doc.line(line).from))
      }
      const card = new CardWidget('', '', true, handlers(''))
      decorations.push(
        Decoration.widget({ widget: card, block: true, side: 2 }).range(
          state.doc.line(range.toLine).to
        )
      )
    }
    return Decoration.set(decorations, true)
  }

  const decorationField = StateField.define<DecorationSet>({
    create: buildDecorations,
    update(decorations, tr) {
      const touched = tr.docChanged || tr.effects.some((e) => e.is(refreshEffect) || e.is(setDraft))
      return touched ? buildDecorations(tr.state) : decorations
    },
    provide: (field) => EditorView.decorations.from(field)
  })

  function startDraft(view: EditorView, lineNumber: number): void {
    const state = view.state
    const info = state.field(infoField)
    const selection = state.selection.main
    let range: LineRange = { fromLine: lineNumber, toLine: lineNumber }
    const selected = selection.empty ? null : selectionLines(state, selection.from, selection.to)
    if (selected && selected.fromLine <= lineNumber && lineNumber <= selected.toLine) {
      range = selected
    } else if (info.targetStarts.has(lineNumber)) {
      range = info.targets.find((target) => target.fromLine === lineNumber) ?? range
    }
    if (!config.canAdd(range)) return
    view.dispatch({
      effects: setDraft.of({
        from: state.doc.line(range.fromLine).from,
        to: state.doc.line(range.toLine).to
      })
    })
  }

  const markers = gutter({
    class: 'cm-annotation-gutter',
    lineMarker(view, line) {
      const info = view.state.field(infoField)
      const number = view.state.doc.lineAt(line.from).number
      if (info.targetStarts.has(number)) return targetMarker
      if (info.commentedLines.has(number)) return commentMarker
      return addMarker
    },
    lineMarkerChange: (update) =>
      update.startState.field(infoField) !== update.state.field(infoField),
    initialSpacer: () => addMarker,
    domEventHandlers: {
      mousedown(view, line) {
        startDraft(view, view.state.doc.lineAt(line.from).number)
        return true
      }
    }
  })

  // The store is reactive and lives outside the editor, so its changes reach the editor as a
  // refresh. Dispatching from inside an update is not allowed, hence the deferral.
  const storeWatcher = ViewPlugin.define((view) => {
    let alive = true
    const stop = watch(config.ranges, () => {
      setTimeout(() => {
        if (alive) refreshAnnotations(view)
      })
    })
    return {
      destroy() {
        alive = false
        stop()
      }
    }
  })

  return [
    configFacet.of(config),
    infoField,
    draftField,
    decorationField,
    markers,
    storeWatcher,
    cardWidth,
    theme
  ]
}

// A block widget is as wide as the longest line, so on a long line the card would run out of
// view. It sticks after the gutters instead, as wide as the visible text area.
const cardWidth = ViewPlugin.define((view) => {
  const measure = {
    read: () => {
      const gutters = view.dom.querySelector<HTMLElement>('.cm-gutters')
      const left = gutters?.offsetWidth ?? 0
      return { left, width: Math.max(view.scrollDOM.clientWidth - left, 0) }
    },
    write: ({ left, width }: { left: number; width: number }) => {
      view.dom.style.setProperty('--cm-annotation-left', `${left}px`)
      view.dom.style.setProperty('--cm-annotation-width', `${width}px`)
    }
  }
  view.requestMeasure(measure)
  return {
    update(update) {
      if (update.geometryChanged) view.requestMeasure(measure)
    }
  }
})

const commentedLine = Decoration.line({ class: 'cm-annotation-line' })
const draftLine = Decoration.line({ class: 'cm-annotation-line draft' })

const theme = EditorView.baseTheme({
  '.cm-annotation-gutter .cm-gutterElement': {
    width: '16px',
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    cursor: 'pointer'
  },
  '.cm-annotation-add': {
    visibility: 'hidden',
    width: '14px',
    height: '14px',
    lineHeight: '14px',
    textAlign: 'center',
    borderRadius: 'var(--radius-sm, 3px)',
    background: 'var(--color-amber)',
    color: 'var(--color-black)',
    fontWeight: 'var(--weight-medium)'
  },
  '.cm-annotation-add.target, .cm-gutterElement:hover .cm-annotation-add': {
    visibility: 'visible'
  },
  '.cm-annotation-dot': {
    width: '7px',
    height: '7px',
    borderRadius: '50%',
    background: 'var(--color-amber)'
  },
  '.cm-annotation-line': { background: 'var(--color-amber-muted)' },
  '.cm-annotation-line.draft': { outline: '1px dashed var(--color-amber)' },
  '.cm-annotation-card': {
    position: 'sticky',
    left: 'var(--cm-annotation-left, 0)',
    boxSizing: 'border-box',
    width: 'var(--cm-annotation-width, 100%)',
    cursor: 'auto',
    whiteSpace: 'normal'
  }
})
