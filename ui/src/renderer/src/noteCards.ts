// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (C) 2026 The Lemongrass Authors

import {
  Facet,
  StateEffect,
  StateField,
  type EditorState,
  type Extension,
  type Transaction
} from '@codemirror/state'
import { Decoration, EditorView, WidgetType, type DecorationSet } from '@codemirror/view'

// A model's note on lines of the editor it sits in, already mapped to that editor's lines.
export interface NoteMark {
  id: number
  fromLine: number
  toLine: number
  author: string
  body: string
}

type NotesOf = (state: EditorState) => NoteMark[]

// The notes live in the conflict thread outside the editor, so they are read through a function
// of the state being drawn, and redrawn when refreshNotes is dispatched or the document changes.
const notesConfig = Facet.define<NotesOf, NotesOf>({
  combine: (values) => values[0] ?? (() => [])
})

const refresh = StateEffect.define<null>()

function linesLabel(mark: NoteMark): string {
  return mark.fromLine === mark.toLine
    ? `line ${mark.fromLine}`
    : `lines ${mark.fromLine} to ${mark.toLine}`
}

// Read only: the human answers a note in the conflict thread, never by editing it. Model text is
// set as text, never as HTML.
class NoteWidget extends WidgetType {
  readonly mark: NoteMark

  constructor(mark: NoteMark) {
    super()
    this.mark = mark
  }

  eq(other: NoteWidget): boolean {
    const a = this.mark
    const b = other.mark
    return (
      a.id === b.id &&
      a.fromLine === b.fromLine &&
      a.toLine === b.toLine &&
      a.author === b.author &&
      a.body === b.body
    )
  }

  toDOM(): HTMLElement {
    const dom = document.createElement('div')
    dom.className = 'cm-note-card'
    const head = document.createElement('div')
    head.className = 'cm-note-head'
    const author = document.createElement('span')
    author.className = 'cm-note-author'
    author.textContent = this.mark.author
    const where = document.createElement('span')
    where.className = 'cm-note-where'
    where.textContent = `note on ${linesLabel(this.mark)}`
    head.append(author, where)
    const body = document.createElement('div')
    body.className = 'cm-note-body'
    body.textContent = this.mark.body
    dom.append(head, body)
    return dom
  }

  ignoreEvent(): boolean {
    return true
  }
}

function visibleMarks(state: EditorState): NoteMark[] {
  return state
    .facet(notesConfig)(state)
    .filter((mark) => mark.fromLine >= 1 && mark.fromLine <= state.doc.lines)
    .map((mark) => ({
      ...mark,
      toLine: Math.min(Math.max(mark.toLine, mark.fromLine), state.doc.lines)
    }))
}

function build(state: EditorState): DecorationSet {
  const ranges = visibleMarks(state)
    .sort((a, b) => a.toLine - b.toLine || a.id - b.id)
    .map((mark) =>
      Decoration.widget({ widget: new NoteWidget(mark), block: true, side: 1 }).range(
        state.doc.line(mark.toLine).to
      )
    )
  return Decoration.set(ranges, true)
}

const notesField = StateField.define<DecorationSet>({
  create: build,
  update(value, tr) {
    if (tr.docChanged || isNoteChange(tr)) return build(tr.state)
    return value
  },
  provide: (field) => EditorView.decorations.from(field)
})

// Same width and sticky placement as the comment cards, measured by the comment gutter. Violet,
// the model's color in the resolver; amber stays with the human's comments.
const theme = EditorView.theme({
  '.cm-note-card': {
    position: 'sticky',
    left: 'var(--cm-annotation-left, 0)',
    boxSizing: 'border-box',
    width: 'var(--cm-annotation-width, 100%)',
    margin: '2px 0 4px',
    padding: '4px 8px',
    borderLeft: '3px solid var(--color-violet)',
    borderRadius: 'var(--radius-sm)',
    background: 'var(--color-surface-1)',
    fontFamily: 'var(--font-body)',
    whiteSpace: 'normal',
    cursor: 'auto'
  },
  '.cm-note-head': { display: 'flex', gap: '6px', color: 'var(--color-fg-muted)' },
  '.cm-note-author': { color: 'var(--color-violet)', fontWeight: 'var(--weight-medium)' },
  '.cm-note-body': {
    color: 'var(--color-fg-primary)',
    whiteSpace: 'pre-wrap',
    wordBreak: 'break-word'
  }
})

export function noteCards(notes: NotesOf): Extension {
  return [notesConfig.of(notes), notesField, theme]
}

// Redraws the cards after the notes changed outside the editor.
export function refreshNotes(view: EditorView): void {
  view.dispatch({ effects: refresh.of(null) })
}

export function isNoteChange(tr: Transaction): boolean {
  return tr.effects.some((effect) => effect.is(refresh))
}

// Lines with a note, which Focus keeps visible.
export function notedLineRanges(state: EditorState): { fromLine: number; toLine: number }[] {
  return visibleMarks(state).map(({ fromLine, toLine }) => ({ fromLine, toLine }))
}
