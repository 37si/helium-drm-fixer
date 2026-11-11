# hdf

Helium doesn't come with Widevine, so DRM-protected videos may not play.

hdf installs Widevine for Helium. It can use a copy from another Chromium-based browser already installed on your system, or download one from Mozilla.

## Keyboard shortcuts

| Key | Action |
| --- | --- |
| `↑` / `↓` | Move through the list |
| `/` | Filter sources |
| `f` | Download from Mozilla |
| `enter` | View the installation plan / confirm |
| `tab` / `shift+tab` | Switch between fields |
| `esc` | Go back |
| `r` | Rescan browsers |
| `?` | Show help |
| `q` | Quit |
| `ctrl+z` | Suspend and return to the shell |

Use `fg` in your shell to resume hdf.

## Main screen

On startup, hdf scans your installed browsers for Widevine.

`●` means a usable CDM was found. `○` means the browser was found, but Widevine wasn't.

Select a source and press `enter` to see the installation details. Check the source and destination, then press `enter` again to proceed.

No changes are made before confirmation.

## Other screens

The following shortcuts are available from the main screen:

| Key | Screen | Description |
| --- | --- | --- |
| `s` | Status | Current Widevine installation and version |
| `d` | Doctor | System checks and possible fixes |
| `w` | Watch | Monitor Helium for updates |
| `v` | Verify | Open a DRM test page |
| `u` | Undo | Restore the previous installation |
| `c` | Clear backups | Delete every saved backup (from Status) |

Inside these screens:

- `esc` returns to the main screen.
- `r` refreshes the current screen.
- `↑` / `↓` scroll through doctor results.
- `space` pauses the watcher.
- `+` / `-` change the watch interval.

## Helium is already running

If Helium is open, hdf displays `● helium running` in the header.

Installing Widevine while Helium is running won't make it available immediately. Close and reopen the browser to load the new CDM.

The status screen (`s`) also shows the running process ID.

On Windows, Helium runs as `chrome.exe`. hdf distinguishes it from other Chromium browsers using its installation path.

## Mozilla download

Press `f` to fetch Widevine directly from Mozilla.

The download is checked against the SHA-512 checksum provided by Mozilla before extraction. No browser installation is needed.

If the download fails, hdf tries an existing browser source instead.

## Mouse

Mouse input is supported throughout the interface.

- Click a row to select it.
- Click the selected row again to open it.
- Use the scroll wheel to navigate lists and scrollable screens.
- The key hints at the bottom of the screen are clickable too.

Use `--no-mouse` to turn mouse support off.

## Command-line usage

All operations are also available without opening the interface.

```bash
hdf --list
hdf --status
hdf --doctor
hdf --fresh --yes
hdf --browser brave --yes
hdf --source-path /opt/vivaldi --yes
hdf --watch --interval 1m
hdf --verify
hdf --backups
hdf --clear-backups
hdf --rollback latest
hdf --history
hdf --upgrade
```

`--yes` skips confirmation prompts. `--dry-run` shows what would happen without making changes.

`--clear-backups` asks for two confirmations. With `--yes`, the flag and `--yes` together are the two confirmations, so it can run in a script.

For scripts, `--json` is supported by:

`--list`, `--check`, `--status`, `--doctor`, `--history` and `--backups`.

## Configuration

Options can be saved in `config.cfg`:

```ini
browser  = brave
interval = 1m
keep     = 5
newest   = true
```

Each setting uses the `key = value` format. You don't need to put every option in the file; only add the ones you want to change.

Command-line options take precedence over saved settings.

## Shell completion

Generate completions for your shell with:

```bash
hdf --completion bash > /etc/bash_completion.d/hdf
hdf --completion zsh  > "${fpath[1]}/_hdf"
hdf --completion fish > ~/.config/fish/completions/hdf.fish
```

For PowerShell:

```powershell
hdf --completion powershell | Out-File -Append $PROFILE
```

## Limitations

Widevine being installed doesn't mean every streaming service will accept it.

Some services only support specific browsers or require additional DRM capabilities. Netflix and Prime Video, for example, may check browser certification, HDCP or Verified Media Path (VMP).

hdf doesn't change any of these. It only installs the CDM; playback and supported quality still depend on what Helium can provide.
