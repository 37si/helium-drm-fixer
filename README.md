# hdf

A small command-line tool that adds Widevine support to Helium.

`hdf` finds an existing `WidevineCdm` installation from another Chromium-based browser or downloads the latest available build from Mozilla, then installs it where Helium expects it.

Written in Go. Single binary, no external dependencies, and no browser modifications required.

## Why?

Helium doesn't ship with Widevine, so DRM-protected content won't play on services that require it.

`hdf` takes care of the missing CDM by reusing one already installed on your system or downloading one from Mozilla. It then places it in Helium's installation directory so the browser can load it normally.

## Installation

Prebuilt binaries for Linux, macOS and Windows are available on the Releases page.

Alternatively, you can use the installation script. It automatically detects your operating system, downloads the appropriate binary and installs it to `~/.local/bin` on Linux and macOS, or `%LOCALAPPDATA%\hdf` on Windows.

**Linux / macOS**

```bash
curl -fsSL https://raw.githubusercontent.com/37si/helium-drm-fixer/main/scripts/install.sh | sh
```

**Windows (PowerShell)**

```powershell
irm https://raw.githubusercontent.com/37si/helium-drm-fixer/main/scripts/install.ps1 | iex
```

**Build from source**

Requires Go 1.26.7 or newer.

```bash
git clone https://github.com/37si/helium-drm-fixer
cd helium-drm-fixer
make build
```

The compiled binary will be available in `bin/hdf` (`bin/hdf.exe` on Windows).

## Uninstall

**Linux / macOS**

```bash
curl -fsSL https://raw.githubusercontent.com/37si/helium-drm-fixer/main/scripts/uninstall.sh | sh
```

**Windows (PowerShell)**

```powershell
irm https://raw.githubusercontent.com/37si/helium-drm-fixer/main/scripts/uninstall.ps1 | iex
```

This removes the `hdf` binary and, on Windows, the user PATH entry. Settings, logs and backups are left alone so you can reinstall later.

To remove those too:

```bash
curl -fsSL https://raw.githubusercontent.com/37si/helium-drm-fixer/main/scripts/uninstall.sh | sh -s -- --purge
```

```powershell
$env:HDF_PURGE = '1'
irm https://raw.githubusercontent.com/37si/helium-drm-fixer/main/scripts/uninstall.ps1 | iex
```

## Usage

Run `hdf` without any arguments to open the interactive interface. You can browse detected browsers, check available Widevine versions and choose a source to install from.

For headless use, `hdf` also supports command-line flags:

```bash
hdf                              # launch the interactive interface
hdf --list                       # list detected browsers and CDMs
hdf --status                     # show Helium's current Widevine status
hdf --doctor                     # check for issues and suggested fixes
hdf --browser brave --yes        # install from Brave without confirmation
hdf --fresh --yes                # download the latest build from Mozilla
hdf --source-path /opt/vivaldi --yes
hdf --watch --interval 1m        # restore Widevine after Helium updates
hdf --verify                     # open a DRM test page in Helium
hdf --backups                    # list available backups
hdf --clear-backups              # delete every backup, with two confirmations
hdf --rollback latest            # restore the latest backup
hdf --dry-run --yes              # preview changes without applying them
hdf --json --status              # output status as JSON
hdf --completion bash            # generate shell completions
```

Run `hdf --help` for the full list of available options.

Inside the interface, `--interval` also controls the watch screen, and the watch log scrolls with the arrow keys. Pressing `ctrl+c` during an install lets the copy finish first; press it again to quit immediately.

## Custom paths

If automatic detection doesn't find your browser or Helium installation, you can provide the paths yourself.

```bash
hdf --source-path /usr/lib/chromium/WidevineCdm --helium /opt/helium --yes
```

`--source-path` accepts a `WidevineCdm` directory, a browser installation directory, an app bundle or the browser executable itself. If the path points at an executable, `hdf` looks in the executable's folder and its parent for a CDM.

`--helium` accepts Helium's installation directory. On macOS you can point it at `Helium.app` or at a folder inside the bundle. If left unspecified, `hdf` will try to locate it automatically.

On Linux and macOS, `which helium` can help locate the browser executable. On Windows, the default per-user installation is usually:

```text
%LOCALAPPDATA%\imput\Helium\Application
```

## Limitations

Getting Widevine installed is only part of the picture. Whether a streaming service actually works depends on more than the presence of the CDM.

`hdf` doesn't modify Helium's Chromium build, browser identity, codecs or DRM capabilities. It simply makes an existing Widevine installation available to the browser.

Some services, such as Netflix and Amazon Prime Video, have additional requirements around browser certification, hardware-backed DRM, HDCP and Verified Media Path (VMP). Because of this, playback may work on some services but remain unavailable or limited on others.

These restrictions can't be fixed by copying a CDM. `hdf` cannot provide hardware-backed L1 DRM or change the certification status of the browser.

## Troubleshooting

**Helium isn't detected.**

Start with `hdf --doctor` to check for common issues. `hdf --list` shows the browsers and CDM paths it found. Add `--verbose` to log the detection details.

You can also specify the installation directory manually with `--helium`.

**Widevine installed successfully, but playback still doesn't work.**

Completely close Helium and launch it again. The browser loads Widevine at startup, so a restart is needed after installation.

To check what the browser actually reports, run `hdf --verify`. This opens DRM Sense, where you can check the available DRM capabilities.

**It installed an older version of Widevine.**

By default, sources are checked in browser order. Use `--newest` to select the highest available version, or `--fresh` to download the latest build from Mozilla.

**I want to restore my previous installation.**

A backup is created before every installation unless you use `--keep 0` or `--no-backup`. You can view available backups and restore one with:

```bash
hdf --backups
hdf --rollback latest
```

To remove every saved backup instead, run `hdf --clear-backups`. It asks twice before deleting anything. In a script, `--clear-backups --yes` uses the flag and `--yes` as the two confirmations.

**Where are the configuration files and logs?**

Run `hdf --print-config` to see the active configuration file.

Default locations:

- Linux: `~/.config/hdf/config.cfg`
- macOS: `~/Library/Application Support/hdf/`
- Windows: `%AppData%\hdf\`

Logs are stored in `~/.cache/hdf/hdf.log`. You can specify another location using `--log-file`. With `--verbose`, the extra debug lines are also written to the terminal.


## License

[License](/LICENSE.md)
