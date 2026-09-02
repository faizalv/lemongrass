<script setup lang="ts">
import { onMounted, onBeforeUnmount, ref } from 'vue'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'

// Renders one PTY session. Raw bytes only, in both directions -- this
// component never inspects or reacts to what's in the stream. PTY is
// display-only, never a control-signal source.

const props = defineProps<{
  /** The agent binary to spawn, e.g. "claude". Never a shell. */
  command: string
  args?: string[]
  cwd?: string
}>()

const emit = defineEmits<{
  exit: [exitCode: number]
}>()

const containerEl = ref<HTMLDivElement>()
let term: Terminal | undefined
let fitAddon: FitAddon | undefined
let shellId: string | undefined
let unsubscribeData: (() => void) | undefined
let unsubscribeExit: (() => void) | undefined
let resizeObserver: ResizeObserver | undefined

// xterm's theme needs literal color strings, not var() references --
// this reads a token's resolved value off :root, keeping style.css's
// token block as the single source of truth.
function cssVar(name: string): string {
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim()
}

onMounted(async () => {
  if (!containerEl.value) return

  term = new Terminal({
    // Canvas's ctx.font is parsed outside the CSS cascade, so it needs
    // a resolved font-family string -- an unresolved var() reference is
    // invalid and silently ignored, breaking the fixed-width cell grid.
    fontFamily: cssVar('--font-mono'),
    fontSize: 13,
    fontWeight: 400, // mirrors --weight-regular
    fontWeightBold: 600, // mirrors --weight-semibold
    cursorStyle: 'bar',
    cursorBlink: false,
    theme: {
      background: cssVar('--color-surface-1'),
      foreground: cssVar('--color-fg-primary'),
      cursor: cssVar('--color-amber'),
      cursorAccent: cssVar('--color-surface-1'),
      selectionBackground: cssVar('--color-amber-muted'),
      // ANSI 16 mapped to the brand accent set instead of xterm's default
      // VGA palette -- dim variants for normal, full-saturation for bright.
      black: cssVar('--color-gray-700'),
      red: cssVar('--color-coral-dim'),
      green: cssVar('--color-jade-dim'),
      yellow: cssVar('--color-amber-dim'),
      blue: cssVar('--color-sapphire-dim'),
      magenta: cssVar('--color-violet-dim'),
      cyan: cssVar('--color-teal-dim'),
      white: cssVar('--color-gray-100'),
      brightBlack: cssVar('--color-gray-500'),
      brightRed: cssVar('--color-coral'),
      brightGreen: cssVar('--color-jade'),
      brightYellow: cssVar('--color-amber'),
      brightBlue: cssVar('--color-sapphire'),
      brightMagenta: cssVar('--color-violet'),
      brightCyan: cssVar('--color-teal'),
      brightWhite: cssVar('--color-white')
    }
  })
  fitAddon = new FitAddon()
  term.loadAddon(fitAddon)
  term.open(containerEl.value)
  fitAddon.fit()

  const { id } = await window.api.pty.spawn({
    command: props.command,
    args: props.args,
    cwd: props.cwd,
    cols: term.cols,
    rows: term.rows
  })
  shellId = id

  unsubscribeData = window.api.pty.onData((payload) => {
    if (payload.id === shellId) term?.write(payload.data)
  })
  unsubscribeExit = window.api.pty.onExit((payload) => {
    if (payload.id === shellId) emit('exit', payload.exitCode)
  })

  term.onData((data) => {
    if (shellId) window.api.pty.write(shellId, data)
  })

  resizeObserver = new ResizeObserver(() => {
    fitAddon?.fit()
    if (term && shellId) window.api.pty.resize(shellId, term.cols, term.rows)
  })
  resizeObserver.observe(containerEl.value)
})

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  unsubscribeData?.()
  unsubscribeExit?.()
  if (shellId) window.api.pty.kill(shellId)
  term?.dispose()
})
</script>

<template>
  <div class="terminal-card fade-in">
    <div ref="containerEl" class="terminal-pane"></div>
  </div>
</template>

<style scoped>
/* FitAddon sizes the terminal off this element's own box, without
   accounting for any padding/border on it -- card chrome (padding,
   border, radius) has to live on the wrapper instead, or the computed
   rows/cols overshoot the actual space inside the inset. */
.terminal-card {
  height: 100%;
  width: 100%;
  padding: var(--space-3);
  background: var(--color-surface-1);
  border: 1px solid var(--color-border-subtle);
  border-radius: var(--radius-lg);
  overflow: hidden;
}

.terminal-pane {
  height: 100%;
  width: 100%;
}
</style>
