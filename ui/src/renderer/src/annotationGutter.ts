// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

import { createApp, watch, type App } from 'vue'
import {
  Facet,
  RangeSet,
  StateEffect,
  StateField,
  type EditorState,
  type Extension,
  type Range,
  type Transaction
} from '@codemirror/state'
import {
  BlockType,
  Decoration,
  EditorView,
  GutterMarker,
  ViewPlugin,
  WidgetType,
  gutterLineClass,
  type DecorationSet
} from '@codemirror/view'
import AnnotationCard from './components/review/AnnotationCard.vue'

// Line numbers here are 1-based, inclusive, and in the editor's current document.
export interface LineRange {
  fromLine: number
  toLine: number
}

// What a card shows under a sent comment: whoever answered it.
export interface CardReply {
  author: 'tab' | 'human'
  label: string
  body: string
  // The agent vendor of the tab that wrote it, for its icon; none for the human.
  vendor?: string | null
}

export interface AnnotationRange extends LineRange {
  id: string
  comment: string
  // A comment already sent: its card is read only and shows the replies, with a way to answer.
  sent?: boolean
  replies?: CardReply[]
  // A draft that answers a sent comment, drawn as a reply.
  reply?: boolean
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
  // Writing an answer under a sent comment; the caller queues it as a draft.
  onReply?: (id: string, comment: string) => void
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
      reply?: (comment: string) => void
    },
    readonly extra: Pick<AnnotationRange, 'sent' | 'replies' | 'reply'> = {}
  ) {
    super()
  }

  eq(other: CardWidget): boolean {
    return (
      other.key === this.key &&
      other.comment === this.comment &&
      other.draft === this.draft &&
      other.extra.sent === this.extra.sent &&
      other.extra.reply === this.extra.reply &&
      JSON.stringify(other.extra.replies ?? []) === JSON.stringify(this.extra.replies ?? [])
    )
  }

  toDOM(view: EditorView): HTMLElement {
    const dom = document.createElement('div')
    dom.className = 'cm-annotation-card'
    dom.contentEditable = 'false'
    const app = createApp(AnnotationCard, {
      comment: this.comment,
      draft: this.draft,
      sent: this.extra.sent === true,
      isReply: this.extra.reply === true,
      replies: this.extra.replies ?? [],
      onSave: (comment: string) => this.handlers.save(view, comment),
      onCancel: () => this.handlers.cancel(view),
      onRemove: () => this.handlers.remove(),
      onReply: (comment: string) => this.handlers.reply?.(comment)
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

// The comment controls live on the line numbers themselves, so commenting takes no column of its
// own: these classes reach every gutter's cell on the line, and the theme styles only the number's.
class LineClass extends GutterMarker {
  constructor(readonly elementClass: string) {
    super()
  }
  eq(other: LineClass): boolean {
    return other.elementClass === this.elementClass
  }
}

const commentedClass = new LineClass('cm-annotation-commented')
const hoverClass = new LineClass('cm-annotation-hover')
const targetClass = new LineClass('cm-annotation-target')

// The line whose number the pointer is over, which shows a "+" in place of the number.
const setHover = StateEffect.define<number | null>()

const hoverField = StateField.define<number | null>({
  create: () => null,
  update(line, tr) {
    for (const effect of tr.effects) if (effect.is(setHover)) return effect.value
    return tr.docChanged ? null : line
  }
})

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

const lineClasses = gutterLineClass.compute([infoField, hoverField], (state) => {
  const info = state.field(infoField)
  const hover = state.field(hoverField)
  const marks: Range<GutterMarker>[] = []
  const at = (line: number): number => state.doc.line(line).from
  for (const line of info.commentedLines) marks.push(commentedClass.range(at(line)))
  for (const line of info.targetStarts) marks.push(targetClass.range(at(line)))
  if (hover !== null && hover <= state.doc.lines) marks.push(hoverClass.range(at(hover)))
  return RangeSet.of(marks, true)
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
    remove: () => config.onRemove(id),
    reply: (comment) => config.onReply?.(id, comment)
  })

  function buildDecorations(state: EditorState): DecorationSet {
    const info = state.field(infoField)
    const draft = state.field(draftField)
    const decorations: Range<Decoration>[] = []
    for (const range of info.ranges) {
      for (let line = range.fromLine; line <= range.toLine; line++) {
        decorations.push(commentedLine.range(state.doc.line(line).from))
      }
      const card = new CardWidget(range.id, range.comment, false, handlers(range.id), {
        sent: range.sent,
        replies: range.replies,
        reply: range.reply
      })
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

  // Hovering a line number turns it into a "+" and pressing it opens a comment there. Gutters sit
  // outside the content DOM, where the editor's own event handlers do not reach, so the listeners
  // go on the whole editor and keep to the number cells.
  const numberPointer = ViewPlugin.define((view) => {
    const numberLine = (event: MouseEvent): number | null => {
      const cell = (event.target as HTMLElement | null)?.closest?.(
        '.cm-lineNumbers .cm-gutterElement'
      )
      if (!cell || !view.dom.contains(cell)) return null
      // A card under a line is a block of its own; its stretch of the gutter is not a number.
      const block = view.elementAtHeight(event.clientY - view.documentTop)
      if (block.type !== BlockType.Text) return null
      return view.state.doc.lineAt(block.from).number
    }
    const hover = (line: number | null): void => {
      if (view.state.field(hoverField) !== line) view.dispatch({ effects: setHover.of(line) })
    }
    const move = (event: MouseEvent): void => hover(numberLine(event))
    const leave = (): void => hover(null)
    const down = (event: MouseEvent): void => {
      if (event.button !== 0) return
      const line = numberLine(event)
      if (line === null) return
      event.preventDefault()
      startDraft(view, line)
    }
    view.dom.addEventListener('mousemove', move)
    view.dom.addEventListener('mouseleave', leave)
    view.dom.addEventListener('mousedown', down)
    return {
      destroy() {
        view.dom.removeEventListener('mousemove', move)
        view.dom.removeEventListener('mouseleave', leave)
        view.dom.removeEventListener('mousedown', down)
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
    hoverField,
    lineClasses,
    decorationField,
    numberPointer,
    EditorView.editorAttributes.of({ class: 'cm-annotatable' }),
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
      const before = view.dom.querySelector<HTMLElement>('.cm-gutters-before')
      const after = view.dom.querySelector<HTMLElement>('.cm-gutters-after')
      const left = before?.offsetWidth ?? 0
      const right = after?.offsetWidth ?? 0
      return { left, width: Math.max(view.scrollDOM.clientWidth - left - right, 0) }
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
  // Base theme rules reach every editor, so the number cells are styled only in an editor that
  // takes comments.
  '&.cm-annotatable .cm-lineNumbers .cm-gutterElement': {
    position: 'relative',
    cursor: 'pointer'
  },
  '&.cm-annotatable .cm-lineNumbers .cm-gutterElement.cm-annotation-commented': {
    color: 'var(--color-amber)',
    fontWeight: 'var(--weight-semibold)'
  },
  // The "+" covers the number in its own cell, so nothing beside it moves.
  '&.cm-annotatable .cm-lineNumbers .cm-gutterElement.cm-annotation-hover, &.cm-annotatable .cm-lineNumbers .cm-gutterElement.cm-annotation-target':
    { color: 'transparent' },
  '&.cm-annotatable .cm-lineNumbers .cm-gutterElement.cm-annotation-hover::after, &.cm-annotatable .cm-lineNumbers .cm-gutterElement.cm-annotation-target::after':
    {
      content: '"+"',
      position: 'absolute',
      top: '50%',
      left: '50%',
      transform: 'translate(-50%, -50%)',
      width: '14px',
      height: '14px',
      lineHeight: '14px',
      textAlign: 'center',
      borderRadius: 'var(--radius-sm, 3px)',
      background: 'var(--color-amber)',
      color: 'var(--color-on-fill)',
      fontWeight: 'var(--weight-medium)'
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
