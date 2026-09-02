Lemongrass v2. Built fresh as of 2026-09-02, replacing an abandoned prior codebase (RECON/grooming/semantic-map) now at `/mnt/data/Projects/lemongrass-legacy`. No code here inherits from it -- only the Sankasten design language does (see below).

Read before proposing or building anything:
- `context/scratchpad/vision/README.md` -- why, and the product shape
- `context/scratchpad/phase1-electron-shell/PRD.md` -- current phase's concrete scope
- `context/handover/phase1-electron-shell.md` -- current status, what's actually built vs planned

## Non-negotiable constraints

- **PTY is display-only.** Never derive control signals (nudges, questionnaires, knowledge triggers) by parsing terminal output. Control comes from each agent CLI's own structured hook/event stream, running parallel to the PTY.
- **Not a generic shell.** A lemongrass shell spawns an agent binary directly (e.g. `claude`), not bash/zsh -- no `cd`, no arbitrary commands as a first-class surface.
- **No legacy logic reuse.** PTY runner, UAC hook engine, `#lg.*` protocol -- none of it carries over. What carries over is the Sankasten design language (tokens, useful components/patterns) from `lemongrass-legacy/context/sankasten_design_system/` and legacy's already-tokenized `ui/src/style.css`. Colors/type via `var(--color-*)`, `var(--font-*)`, never raw hex.
- **Stack:** Electron + Vue. `kencana-nfc/ui` (`/home/faizal/Projects/ascendiz/kencana-nfc/ui`) is the scaffolding precedent -- electron-vite layout (`src/main`, `src/renderer/src`), electron-builder for packaging.

`context/` is gitignored, same convention as kencana-backend/kencana-nfc -- personal knowledge-keeping, not shipped with the repo.
