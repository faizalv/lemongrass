// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2026 The Sankasten Authors

// The reference text for every component: what it is for, its props, events, slots and what
// falls through. Kept beside the live demos in the sections, and updated with a component's API.

export interface PropDoc {
  name: string
  type: string
  default?: string
  doc: string
}

export interface ComponentDoc {
  name: string
  summary: string
  props: PropDoc[]
  events?: PropDoc[]
  slots?: PropDoc[]
  notes?: string[]
}

const rowSize: PropDoc = {
  name: 'size',
  type: "'xs' | 'sm' | 'md'",
  default: "'md'",
  doc: 'Control height from --control-h-*. sm for dense surfaces; xs only for an action inside a list, tree or menu row.'
}

const size: PropDoc = {
  name: 'size',
  type: "'sm' | 'md'",
  default: "'md'",
  doc: 'Control height from --control-h-sm or --control-h-md. sm for dense surfaces.'
}

export const DOCS: Record<string, ComponentDoc> = {
  AppButton: {
    name: 'AppButton',
    summary:
      'Every text button. A native button: type, disabled and listeners fall through, and with no type it still submits a form.',
    props: [
      {
        name: 'variant',
        type: "'ghost' | 'primary' | 'danger' | 'text'",
        default: "'ghost'",
        doc: 'primary for the one or two main actions of a surface, ghost for the rest, danger for a destructive action, text for a frameless secondary action in a toolbar or row.'
      },
      rowSize,
      {
        name: 'tone',
        type: "'neutral' | 'accent' | 'info' | 'success' | 'danger'",
        default: "'neutral'",
        doc: 'Colors the label of a ghost or text button and keeps it on hover.'
      },
      {
        name: 'pressed',
        type: 'boolean',
        doc: 'A toggle that is on: aria-pressed and an amber-muted fill. Leave undefined on a plain action.'
      }
    ],
    slots: [{ name: 'default', type: '', doc: 'The label.' }],
    notes: ['Exposes focus(). Layout tweaks go in a call-site class, never its height or padding.']
  },
  AppIconButton: {
    name: 'AppIconButton',
    summary:
      'A square button holding only a glyph, for navigation, toggles and row actions. The label is required: it is the title and the aria-label.',
    props: [
      {
        name: 'icon',
        type: 'AppIconName',
        doc: 'An app-wide glyph. Without it the default slot draws the glyph.'
      },
      {
        name: 'label',
        type: 'string',
        doc: 'Required. Title and aria-label. A title attribute from the caller wins as the tooltip, for a disabled reason.'
      },
      rowSize,
      {
        name: 'variant',
        type: "'bare' | 'ghost' | 'tinted'",
        default: "'bare'",
        doc: 'bare for toolbars and rows, ghost adds a border, tinted fills in the tone.'
      },
      {
        name: 'tone',
        type: "'neutral' | 'accent' | 'info' | 'success' | 'danger'",
        default: "'neutral'",
        doc: 'On bare it colors the glyph on hover only (a window close is danger); on ghost and tinted always.'
      },
      {
        name: 'pressed',
        type: 'boolean',
        doc: 'A toggle that is on: aria-pressed, amber-muted fill and amber glyph.'
      },
      {
        name: 'busy',
        type: 'boolean',
        default: 'false',
        doc: 'The glyph becomes a spinner at full opacity and clicks are ignored.'
      },
      {
        name: 'confirm',
        type: 'boolean',
        default: 'false',
        doc: 'Click-again confirm: the first click arms it, the second emits confirm.'
      },
      {
        name: 'confirmLabel',
        type: 'string',
        default: "'Click again'",
        doc: 'Tooltip and label while armed.'
      }
    ],
    events: [
      { name: 'click', type: 'MouseEvent', doc: 'Not emitted while busy or with confirm.' },
      { name: 'confirm', type: '', doc: 'The armed second click.' }
    ],
    slots: [
      {
        name: 'default',
        type: '',
        doc: 'Any glyph: an icon component or a short text such as B or H1. Any svg is drawn at the size glyph height.'
      },
      {
        name: 'badge',
        type: '',
        doc: 'Pinned to the top right and not clipped, for an AppBadge count.'
      }
    ],
    notes: ['disabled and other attributes fall through to the native button. Exposes focus().']
  },
  AppConfirmButton: {
    name: 'AppConfirmButton',
    summary:
      'A destructive or committing action that confirms in place: the first click starts a one second wait, then a second click runs it.',
    props: [
      {
        name: 'variant',
        type: "'ghost' | 'primary' | 'danger'",
        doc: 'Renders through AppButton; without it the root is a bare button the caller styles.'
      },
      {
        name: 'tone',
        type: "'error' | 'jade'",
        default: "'error'",
        doc: 'error for a destructive action, jade for a go action.'
      },
      {
        name: 'confirmLabel',
        type: 'string',
        default: "'Click again'",
        doc: 'Title and aria-label while armed.'
      },
      { ...rowSize, doc: 'Passed to AppButton when a variant is set.' }
    ],
    events: [{ name: 'confirm', type: '', doc: 'The armed click.' }],
    slots: [
      { name: 'default', type: '{ state, armed }', doc: 'The label; can change with the state.' }
    ]
  },
  AppSplitButton: {
    name: 'AppSplitButton',
    summary:
      'A main action with its alternatives behind a chevron, two AppButtons sharing one pill.',
    props: [
      { name: 'items', type: 'MenuItem[]', doc: 'The menu behind the chevron.' },
      {
        name: 'variant',
        type: "'ghost' | 'primary' | 'danger'",
        default: "'ghost'",
        doc: 'Both parts.'
      },
      size,
      { name: 'disabled', type: 'boolean', default: 'false', doc: 'Both parts.' },
      { name: 'menuLabel', type: 'string', default: "'More actions'", doc: 'Title of the chevron.' }
    ],
    events: [
      { name: 'click', type: 'MouseEvent', doc: 'The main part.' },
      { name: 'select', type: 'string', doc: 'A menu item key.' },
      { name: 'menu', type: '', doc: 'The menu opened.' }
    ]
  },
  AppMenuButton: {
    name: 'AppMenuButton',
    summary: 'A bare icon that opens a menu of rarer actions, square at the control height.',
    props: [
      { name: 'items', type: 'MenuItem[]', doc: 'The menu.' },
      { name: 'label', type: 'string', doc: 'Title and aria-label.' },
      size,
      { name: 'disabled', type: 'boolean', default: 'false', doc: '' }
    ],
    events: [{ name: 'select', type: 'string', doc: 'A menu item key.' }],
    slots: [{ name: 'default', type: '', doc: 'The glyph.' }]
  },
  AppContextMenu: {
    name: 'AppContextMenu',
    summary:
      'The popup menu, placed from its measured size inside the window. At the pointer with x and y, or beside a control with anchor.',
    props: [
      {
        name: 'items',
        type: 'MenuItem[]',
        doc: '{ key, label, disabled?, title?, confirm?, icon? }. A confirm item is a click-again confirm; icon is stroke path data in a 14 by 14 box.'
      },
      { name: 'x, y', type: 'number', doc: 'Pointer position for a right-click menu.' },
      {
        name: 'anchor',
        type: 'MenuAnchor',
        doc: 'A control’s edges, so a flipped menu still lines up with it.'
      }
    ],
    events: [
      { name: 'select', type: 'string', doc: '' },
      { name: 'close', type: '', doc: 'Outside press or Escape.' }
    ]
  },
  AppInput: {
    name: 'AppInput',
    summary:
      'Every single-line text input. class and style land on the wrapper and set layout; every other attribute and listener lands on the input.',
    props: [
      { name: 'v-model', type: 'string | number', doc: 'With the .number and .trim modifiers.' },
      size,
      { name: 'mono', type: 'boolean', default: 'false', doc: 'Monospace value.' },
      {
        name: 'icon',
        type: "'search'",
        doc: 'A leading search glyph inside the frame, for a filter box.'
      }
    ],
    notes: [
      'type="number" draws its own step buttons. Exposes focus(), select() and el; type a template ref as AppInputHandle.'
    ]
  },
  AppTextarea: {
    name: 'AppTextarea',
    summary:
      'Multi-line text. The textarea is the root, so attributes and a layout class fall through. Multi-line, so not on the height scale.',
    props: [
      { name: 'v-model', type: 'string', doc: '' },
      {
        name: 'size',
        type: "'sm' | 'md'",
        default: "'md'",
        doc: 'Text size: --text-xs or --text-sm.'
      },
      { name: 'resize', type: "'vertical' | 'none'", default: "'vertical'", doc: '' },
      { name: 'mono', type: 'boolean', default: 'false', doc: '' }
    ]
  },
  AppSwitcher: {
    name: 'AppSwitcher',
    summary: 'The single-choice dropdown: a plain trigger for headers or framed for forms.',
    props: [
      {
        name: 'items',
        type: '{ id, label, title?, search?, disabled?, hint?, group? }[]',
        doc: 'disabled is the reason an item cannot be picked; group shows a heading above each run.'
      },
      { name: 'activeId', type: 'string | null', doc: '' },
      { name: 'placeholder', type: 'string', doc: 'Shown with no active item.' },
      { name: 'emptyText', type: 'string', doc: 'Shown with no items.' },
      { name: 'framed', type: 'boolean', default: 'false', doc: 'Reads as a field, for forms.' },
      size,
      {
        name: 'searchable',
        type: 'boolean',
        default: 'false',
        doc: 'A filter box at the top of the panel.'
      },
      { name: 'label', type: 'string', doc: 'Fixed trigger text instead of the active label.' },
      { name: 'addLabel', type: 'string', doc: 'An add row at the end of the list.' },
      { name: 'disabled', type: 'boolean', default: 'false', doc: '' },
      { name: 'above', type: 'boolean', doc: 'Prefer opening above.' }
    ],
    events: [
      { name: 'select', type: 'string', doc: 'The picked id.' },
      { name: 'add', type: '', doc: 'The add row.' }
    ]
  },
  AppMultiPicker: {
    name: 'AppMultiPicker',
    summary:
      'The multi-choice dropdown. Its panel opens in the form’s flow, with a filter box and grouped checkboxes.',
    props: [
      { name: 'groups', type: 'PickerGroup[]', doc: '{ label?, items: { value, label }[] }' },
      { name: 'v-model', type: 'string[]', doc: '' },
      { name: 'summary', type: 'string', doc: 'Trigger text, such as "3 tables".' },
      size,
      { name: 'filterPlaceholder', type: 'string', default: "'Filter...'", doc: '' },
      { name: 'disabled', type: 'boolean', default: 'false', doc: '' }
    ]
  },
  AppSegmented: {
    name: 'AppSegmented',
    summary: 'Picks one of a few modes. Name the group with aria-label on the component.',
    props: [
      { name: 'options', type: '{ value, label }[]', doc: '' },
      { name: 'v-model', type: 'string | number', doc: '' },
      size,
      {
        name: 'block',
        type: 'boolean',
        default: 'false',
        doc: 'Full width of its container with equal segments, for a switch that heads a panel.'
      }
    ]
  },
  AppCheckbox: {
    name: 'AppCheckbox',
    summary: 'A checkbox with its label. At least the control height; a long label may wrap.',
    props: [
      { name: 'checked', type: 'boolean', doc: 'Toggle it in your @click.' },
      {
        name: 'indeterminate',
        type: 'boolean',
        default: 'false',
        doc: 'The partly checked state of a select-all box; wins over checked.'
      },
      size
    ]
  },
  AppNavItem: {
    name: 'AppNavItem',
    summary:
      'A sidebar entry card: a full-width raised button that opens a view, a panel or a fold.',
    props: [
      { name: 'label', type: 'string', doc: 'Or the default slot.' },
      {
        name: 'state',
        type: 'string',
        doc: 'Secondary text after the label; takes the free width and ellipsizes.'
      },
      {
        name: 'active',
        type: 'boolean',
        default: 'false',
        doc: 'The view it opens is showing, or the thing is on: amber-muted.'
      },
      {
        name: 'tone',
        type: "'neutral' | 'danger'",
        default: "'neutral'",
        doc: 'danger for an entry that needs attention now; wins over active.'
      },
      { name: 'count', type: 'number', doc: 'An amber pill, hidden at 0.' },
      { name: 'countTitle', type: 'string', doc: 'Tooltip of the count.' },
      {
        name: 'alert',
        type: 'string',
        doc: 'A red "!" pill after the count; the text is its title and accessible name.'
      },
      {
        name: 'expanded',
        type: 'boolean',
        doc: 'For an entry that folds a section: trailing chevron and aria-expanded.'
      },
      size
    ],
    slots: [
      { name: 'icon', type: '', doc: 'The leading glyph, drawn 13px tall.' },
      {
        name: 'trailing',
        type: '',
        doc: 'After the state, before the count, such as a status dot.'
      }
    ]
  },
  AppDisclosure: {
    name: 'AppDisclosure',
    summary:
      'The header of a foldable group. A row heading, so its height comes from its text, not the control scale.',
    props: [
      {
        name: 'v-model:open',
        type: 'boolean',
        default: 'false',
        doc: 'Sets aria-expanded and turns the chevron.'
      },
      { name: 'label', type: 'string', doc: 'Or the default slot.' },
      { name: 'count', type: 'number', doc: '' },
      {
        name: 'countLook',
        type: "'text' | 'pill'",
        default: "'text'",
        doc: 'A muted number or an AppBadge.'
      },
      { name: 'countTone', type: 'AppBadge tone', default: "'info'", doc: 'For the pill.' },
      {
        name: 'block',
        type: 'boolean',
        default: 'false',
        doc: 'Full width with padding, for a group head in a list.'
      }
    ]
  },
  AppChip: {
    name: 'AppChip',
    summary:
      'A value in a pill. A value, not a control, so it has one 20px height. Set chips in a wrapping flex row.',
    props: [
      {
        name: 'tone',
        type: "'default' | 'quiet' | 'accent' | 'info' | 'success' | 'danger'",
        default: "'default'",
        doc: 'quiet for read-only values and global entries; a tone for a checked state.'
      },
      { name: 'mono', type: 'boolean', default: 'false', doc: '' },
      { name: 'struck', type: 'boolean', default: 'false', doc: 'Listed but switched off.' },
      { name: 'tag', type: 'string', doc: 'Muted text after the value.' },
      { name: 'actionLabel', type: 'string', doc: 'An amber inline action; emits action.' },
      {
        name: 'removable',
        type: 'boolean',
        default: 'false',
        doc: 'The remove glyph; emits remove.'
      },
      {
        name: 'removeLabel',
        type: 'string',
        default: "'Remove'",
        doc: 'Title and aria-label of the remove glyph.'
      }
    ],
    events: [
      { name: 'remove', type: '', doc: '' },
      { name: 'action', type: '', doc: '' }
    ],
    notes: [
      'Attributes fall through to the root, so a chip can take tabindex and hang an AppInfoPopover.'
    ]
  },
  AppBadge: {
    name: 'AppBadge',
    summary: 'A count or short tag in a pill. The caller decides when to show it.',
    props: [
      {
        name: 'tone',
        type: "'accent' | 'info' | 'success' | 'danger' | 'neutral'",
        default: "'accent'",
        doc: ''
      },
      { name: 'size', type: "'sm' | 'md'", default: "'md'", doc: '16px or 18px tall.' }
    ]
  },
  AppEmptyState: {
    name: 'AppEmptyState',
    summary: 'Every empty, loading and no-match message.',
    props: [
      {
        name: 'layout',
        type: "'inline' | 'fill'",
        default: "'inline'",
        doc: 'inline stands in a list’s place as a dashed card; fill centers itself in a view whose whole content is missing.'
      },
      { name: 'title', type: 'string', doc: 'Or the default slot.' },
      { name: 'detail', type: 'string', doc: 'Or the detail slot. A second sentence goes here.' },
      {
        name: 'loading',
        type: 'boolean',
        default: 'false',
        doc: 'A spinner before the title, role status.'
      },
      { name: 'icon', type: 'IconShape[]', doc: 'Above the title, fill only.' },
      { name: 'size', type: "'sm' | 'md'", default: "'md'", doc: 'sm tightens it for sidebars.' }
    ],
    slots: [{ name: 'action', type: '', doc: 'A button under the text.' }]
  },
  AppProgressLine: {
    name: 'AppProgressLine',
    summary:
      'A 3px progress line. Without a value it is indeterminate, for work of unknown length.',
    props: [
      { name: 'label', type: 'string', doc: 'Required, the progressbar name.' },
      { name: 'tone', type: "'jade' | 'amber'", default: "'jade'", doc: '' },
      {
        name: 'value',
        type: 'number | null',
        default: 'null',
        doc: '0 to 1 fills to that fraction.'
      }
    ],
    notes: ['Pair it with text that changes, such as an elapsed counter.']
  },
  AppInfoPopover: {
    name: 'AppInfoPopover',
    summary:
      'The hint card shown while its anchor is hovered or focused, in body so a modal never crops it.',
    props: [
      {
        name: 'anchor',
        type: 'HTMLElement | null',
        doc: 'Set on mouseenter or focus, clear on mouseleave or blur.'
      },
      { name: 'id', type: 'string', doc: 'Point the anchor’s aria-describedby at it.' }
    ]
  },
  AppModal: {
    name: 'AppModal',
    summary: 'A dialog over an overlay. Escape and an overlay click emit close.',
    props: [
      { name: 'title', type: 'string', doc: '' },
      { name: 'size', type: "'sm' | 'md' | 'lg'", default: "'sm'", doc: 'Dialog width.' },
      {
        name: 'scope',
        type: "'container' | 'app'",
        default: "'container'",
        doc: 'Covers the nearest positioned container or the whole window.'
      },
      { name: 'flush', type: 'boolean', default: 'false', doc: 'No body padding.' },
      { name: 'label', type: 'string', doc: 'aria-label when there is no title.' }
    ],
    events: [{ name: 'close', type: '', doc: '' }],
    slots: [
      { name: 'default', type: '', doc: 'The body.' },
      { name: 'footer', type: '', doc: 'Actions, md buttons.' }
    ]
  },
  AppIcon: {
    name: 'AppIcon',
    summary: 'The app-wide glyphs: a glyph that means the same on any screen belongs here.',
    props: [
      { name: 'name', type: 'AppIconName', doc: '' },
      { name: 'size', type: 'number', default: '13', doc: 'Rendered height in px.' }
    ]
  },
  IconFrame: {
    name: 'IconFrame',
    summary:
      'The svg every icon component draws into, so size, stroke and accessibility are set one way.',
    props: [
      { name: 'size', type: 'number', doc: 'Rendered height; the width follows the box aspect.' },
      { name: 'box', type: 'number | [w, h]', default: '14', doc: 'The viewBox.' },
      {
        name: 'strokeWidth',
        type: 'number | null',
        default: '1.2',
        doc: 'null draws a filled glyph.'
      },
      { name: 'fill', type: 'string', default: "'none'", doc: '' }
    ]
  }
}
