# Real terminals, controlled by structured events

Panes are `xterm.js` in the renderer over `node-pty` in the main process, the stack behind VS Code's integrated terminal, not a chat UI. Control decisions such as nudges come from each agent's own hook events and never from parsing rendered terminal text. The app also provides tabs, split layouts that persist across restarts, session restore, and a Git panel with diff, commit and push.
