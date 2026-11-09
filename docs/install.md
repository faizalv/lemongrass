# Install and uninstall

## Linux

On Fedora and other rpm distributions, install the rpm from the [latest release](https://github.com/faizalv/lemongrass/releases). The app updates itself from then on.

## macOS

The macOS app is free and needs no Apple account. It is signed ad hoc, not with a paid Apple Developer ID, and is not notarized by Apple, so macOS warns about it once when it comes from a browser download. Either way below gets you past that.

**Option 1: one command (recommended).** Open Terminal and run:

```
curl -fsSL https://raw.githubusercontent.com/faizalv/lemongrass/main/scripts/install-mac.sh | bash
```

It downloads the dmg for your Mac's chip from the newest release, checks it against the release's `SHA256SUMS`, quits a running Lemongrass, puts the app in `/Applications` and opens it, with no warning. Updates come from the app itself, see below. Running the script again reinstalls the newest release, for example to repair an install. `LEMONGRASS_VERSION=v0.4.0-rc.1` before `bash` installs a given tag.

**Option 2: the dmg from the release page.**

1. Find your chip under Apple menu > About This Mac. Download `lemongrass-<version>-arm64.dmg` for Apple silicon (M1 and later) or `lemongrass-<version>-x64.dmg` for Intel from the [latest release](https://github.com/faizalv/lemongrass/releases/latest).
2. Open the dmg and drag Lemongrass onto Applications.
3. Open Lemongrass from Applications. macOS says "Lemongrass" Not Opened, Apple could not verify it is free of malware. Click **Done**. Do not click Move to Trash, which deletes the app.
4. Open System Settings > Privacy & Security and scroll down to Security. Next to "Lemongrass was blocked to protect your Mac", click **Open Anyway**.
5. Confirm with your password or Touch ID, then click **Open Anyway** once more. Lemongrass opens, and opens normally from then on.

Instead of steps 4 and 5 you can run `xattr -dr com.apple.quarantine /Applications/Lemongrass.app` in Terminal and open the app again. The dmg window shows the same steps and carries a text file with the command to copy.

If something goes wrong:

- There is no Open Anyway button: it appears only after you try to open the app (step 3), and for about an hour. Open Lemongrass again, click Done, and go back to Privacy & Security.
- You clicked Move to Trash: open the Trash, right-click Lemongrass and choose Put Back, or drag it to Applications again from the dmg, and start over at step 3.
- macOS says the app "is damaged and can't be opened": run the `xattr` command above. The download is not damaged, macOS shows this message for some apps that are not notarized.
- The warning comes back with the next version: a dmg downloaded with a browser needs these steps once per version. Updating from inside the app (below) or with the one command does not.

From 0.4.0 the app updates itself from Settings, Updates, the same as the rpm, as long as it runs from a folder such as `/Applications` and not from the dmg.

## Windows

On Windows 10 or 11 (x64), download `lemongrass-<version>-setup.exe` from the [latest release](https://github.com/faizalv/lemongrass/releases) and run it. It installs for your user only, into `%LOCALAPPDATA%\Programs\lemongrass`, and needs no admin rights. The app updates itself from Settings, Updates.

The `setup.exe` is not code signed, so Windows opens it with "Windows protected your PC". This is expected. Click More info, then Run anyway. Windows asks once per downloaded file, and an update the app installs itself does not ask.

## Verify a download

To check that a download is the one this repository built, use the [GitHub CLI](https://cli.github.com):

```
gh attestation verify lemongrass-<version>-setup.exe --repo faizalv/lemongrass
```

It confirms the file was built by this repository's release workflow from the tagged commit. The same works for the rpm and the dmgs. Without the GitHub CLI, compare the file's SHA-256 (`Get-FileHash <file>` in PowerShell) with its line in the release's `SHA256SUMS`.

## Release candidates

To be offered release candidates as well as stable releases, start the app with `LEMONGRASS_UPDATE_PRERELEASE=1` in its environment, on macOS with `LEMONGRASS_UPDATE_PRERELEASE=1 /Applications/Lemongrass.app/Contents/MacOS/Lemongrass`.

## Uninstall

Uninstall with your system's own tools. Settings, Updates in the app shows the same steps for the system it runs on.

On Windows, close Lemongrass, then open Settings, Apps, Installed apps, find Lemongrass and choose Uninstall. Its uninstaller runs `lgrassconf uninstall` itself, so nothing else is needed.

With the rpm or on macOS, close Lemongrass and first run as your own user:

```
~/.local/bin/lgrassconf uninstall
```

It stops the keeper service, takes the Lemongrass hooks and status line out of your Claude and Codex settings (putting back a status line you had before), and removes the Lemongrass skills and the `lgrass`, `lgrassconf` and `lgrassd` binaries. Then remove the package with `sudo dnf remove lemongrass`, or on macOS move Lemongrass from Applications to the Trash. Run `lgrassconf uninstall` first: removing the package does not reach the setup in your home folder, and the keeper keeps putting the hooks back while it runs. Launching Lemongrass again sets everything up again.

`~/.lemongrass` stays, with your projects, vault, threads and history, and so do your Claude account folders (`~/.claude-<name>`) and the projects you trusted in Claude and Codex. The app's own settings stay in Electron's data folder: `~/.config/lemongrass`, `%APPDATA%\lemongrass` on Windows, `~/Library/Application Support/lemongrass` on macOS. Delete `~/.lemongrass` (`%USERPROFILE%\.lemongrass` on Windows) and that folder yourself to remove that data too.
