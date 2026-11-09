# Build

```
make build-linux
```

Cross-compiles `lgrass`, `lgrassd` and `lgrassconf` for `linux/amd64` into `dist/`, then builds and packages the Electron app for Linux. This is the one entrypoint above both toolchains. `ui/`'s own `npm run build:linux` builds only the Electron half and expects `dist/` to exist already, so use the root `make` target. `make verify` compiles every Go package without writing a binary into the source tree. Every Go target first checks out the bibliothek submodule, and `make bibliothek-update` moves its pin to bibliothek's latest commit. It builds AppImage, snap, deb and rpm, so it needs `dpkg` and `snapcraft` on the host.

```
make build-rpm
```

Same cross-compile, then builds only the rpm (needs `rpmbuild`), for Fedora and other rpm distributions. The package installs the app under `/opt/Lemongrass` and registers no service. On each launch the app copies `lgrass` and `lgrassconf` into `~/.local/bin` and `lgrassd` into `~/.lemongrass/bin`, then runs `lgrassconf install`, which writes the keeper's systemd user unit. Removing the package prints a reminder to run `lgrassconf uninstall`, described under [Uninstall](../install.md#uninstall).

```
make build-mac
```

Cross-compiles `lgrass`, `lgrassd` and `lgrassconf` for `darwin/amd64` and `darwin/arm64`, then packages a macOS `.app`, `.dmg` and `.zip` via electron-builder. On first launch the app copies the matching arch binaries into `~/.local/bin` and runs `lgrassconf install` (LaunchAgent). The app is signed ad hoc and not notarized, since both need an Apple Developer ID.

```
make build-windows
```

Cross-compiles the three binaries for `windows/amd64` as `.exe`, then builds the per-user NSIS `setup.exe` on Linux. electron-builder runs the installer once under the system wine to extract the uninstaller, so the host needs wine with 32-bit support. On first launch the app copies `lgrass` and `lgrassconf` into `%USERPROFILE%\.local\bin`, adds that folder to the user's `Path`, and runs `lgrassconf install`, which registers the keeper's logon task. The `setup.exe` is unsigned.
