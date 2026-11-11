package main

import (
	"fmt"
	"strings"

	"hdf/internal/browser"
)

var flagNames = []string{
	"--browser", "--source-path", "--helium", "--log-file", "--repo", "--rollback",
	"--completion", "--keep", "--interval", "--list", "--check", "--status", "--doctor",
	"--history", "--backups", "--clear-backups", "--watch", "--verify", "--upgrade", "--json", "--online",
	"--newest", "--fresh", "--gmp-version", "--no-backup", "--dry-run", "--yes",
	"--external", "--no-external", "--no-anim", "--no-mouse",
	"--terminal", "--verbose", "--quiet", "--config",
	"--print-config", "--version", "--help",
}

func printCompletion(shell string) error {
	switch strings.ToLower(shell) {
	case "bash":
		fmt.Print(bashCompletion)
	case "zsh":
		fmt.Print(zshCompletion)
	case "fish":
		fmt.Print(fishCompletion)
	case "powershell", "pwsh":
		fmt.Print(powerShellCompletion)
	default:
		return fmt.Errorf("unknown shell %q: use bash, zsh, fish or powershell", shell)
	}

	return nil
}

func browserIDs() []string {
	ids := make([]string, 0, len(browser.All)+1)
	for _, b := range browser.All {
		ids = append(ids, string(b.ID))
	}

	return append(ids, string(browser.Fresh))
}

func browserNames() string { return strings.Join(browserIDs(), " ") }

var bashCompletion = `_hdf() {
  local current previous
  current="${COMP_WORDS[COMP_CWORD]}"
  previous="${COMP_WORDS[COMP_CWORD-1]}"

  case "${previous}" in
    --browser|-b)
      COMPREPLY=($(compgen -W "` + browserNames() + `" -- "${current}"))
      return
      ;;
    --completion)
      COMPREPLY=($(compgen -W "bash zsh fish powershell" -- "${current}"))
      return
      ;;
    --rollback)
      COMPREPLY=($(compgen -W "latest" -- "${current}"))
      return
      ;;
    --interval)
      COMPREPLY=($(compgen -W "15s 30s 1m 5m" -- "${current}"))
      return
      ;;
    --source-path|--helium|--log-file)
      COMPREPLY=($(compgen -f -- "${current}"))
      return
      ;;
  esac

  COMPREPLY=($(compgen -W "` + strings.Join(flagNames, " ") + `" -- "${current}"))
}

complete -F _hdf hdf
`

var zshCompletion = `#compdef hdf

_hdf() {
  local -a flags browsers

  flags=(
    '--browser[only use this browser as the source]:browser:(` + browserNames() + `)'
    '--source-path[WidevineCdm directory or browser install]:path:_files'
    '--helium[Helium install directory]:path:_files -/'
    '--log-file[write logs here]:path:_files'
    '--status[show what Helium has and whether an update exists]'
    '--list[list detected browsers]'
    '--check[resolve source and target]'
    '--doctor[run health checks]'
    '--history[list previous installs]'
    '--backups[list saved copies]'
    '--clear-backups[delete every saved backup]'
    '--rollback[restore a backup]:id:(latest)'
    '--watch[reinstall when Helium updates]'
    '--interval[poll interval]:duration:(15s 30s 1m 5m)'
    '--verify[open a Widevine test stream]'
    '--upgrade[install the newest release]'
    '--repo[GitHub repository]:repo:_users'
    '--newest[prefer the highest CDM version]'
    '--fresh[download the newest WidevineCdm from Mozilla]'
    '--gmp-version[Firefox version whose update endpoint to ask]:version:'
    '--no-backup[do not keep a copy]'
    '--keep[how many backups to keep]:count:'
    '--dry-run[report without writing]'
    '--yes[run without the interface]'
    '--json[machine readable output]'
    '--online[allow network checks]'
    '--no-anim[turn off interface animations]'
    '--no-mouse[ignore the mouse]'
    '--external[always open a separate window]'
    '--no-external[never leave this terminal]'
    '--terminal[terminal emulator to open]:app:'
    '--completion[print a completion script]:shell:(bash zsh fish powershell)'
    '--verbose[log every lookup]'
    '--quiet[no progress output]'
    '--config[settings file]:path:_files'
    '--print-config[show the settings in effect]'
    '--version[print the version]'
    '--help[show the help]'
  )

  _arguments -s "${flags[@]}"
}

_hdf "$@"
`

var fishCompletion = `complete -c hdf -f
complete -c hdf -l browser -s b -x -a "` + browserNames() + `" -d 'Source browser'
complete -c hdf -l source-path -r -d 'WidevineCdm directory or browser install'
complete -c hdf -l helium -r -d 'Helium install directory'
complete -c hdf -l log-file -r -d 'Log file'
complete -c hdf -l status -s s -d 'Show install status'
complete -c hdf -l list -s l -d 'List detected browsers'
complete -c hdf -l check -d 'Resolve source and target'
complete -c hdf -l doctor -s d -d 'Run health checks'
complete -c hdf -l history -d 'List previous installs'
complete -c hdf -l backups -d 'List saved copies'
complete -c hdf -l clear-backups -d 'Delete every saved backup'
complete -c hdf -l rollback -x -a latest -d 'Restore a backup'
complete -c hdf -l watch -d 'Reinstall when Helium updates'
complete -c hdf -l interval -x -a "15s 30s 1m 5m" -d 'Poll interval'
complete -c hdf -l verify -d 'Open a Widevine test stream'
complete -c hdf -l upgrade -d 'Install the newest release'
complete -c hdf -l repo -x -d 'GitHub repository'
complete -c hdf -l newest -d 'Prefer the highest CDM version'
complete -c hdf -l fresh -d 'Download the newest WidevineCdm from Mozilla'
complete -c hdf -l gmp-version -x -d 'Firefox version whose update endpoint to ask'
complete -c hdf -l no-backup -d 'Do not keep a copy'
complete -c hdf -l keep -x -d 'How many backups to keep'
complete -c hdf -l dry-run -s n -d 'Report without writing'
complete -c hdf -l yes -s y -d 'Run without the interface'
complete -c hdf -l json -d 'Machine readable output'
complete -c hdf -l online -d 'Allow network checks'
complete -c hdf -l no-anim -d 'Turn off interface animations'
complete -c hdf -l no-mouse -d 'Ignore the mouse'
complete -c hdf -l external -d 'Always open a separate window'
complete -c hdf -l no-external -d 'Never leave this terminal'
complete -c hdf -l terminal -r -d 'Terminal emulator to open'
complete -c hdf -l completion -x -a "bash zsh fish powershell" -d 'Print a completion script'
complete -c hdf -l verbose -s v -d 'Log every lookup'
complete -c hdf -l quiet -s q -d 'No progress output'
complete -c hdf -l config -r -d 'Settings file'
complete -c hdf -l print-config -d 'Show the settings in effect'
complete -c hdf -l version -d 'Print the version'
complete -c hdf -l help -s h -d 'Show the help'
`

var powerShellCompletion = `Register-ArgumentCompleter -Native -CommandName hdf -ScriptBlock {
    param($wordToComplete, $commandAst, $cursorPosition)

    $flags = @(
        '--browser', '--source-path', '--helium', '--log-file', '--repo', '--rollback',
        '--completion', '--keep', '--interval', '--list', '--check', '--status', '--doctor',
        '--history', '--backups', '--clear-backups', '--watch', '--verify', '--upgrade', '--json', '--online',
        '--newest', '--fresh', '--gmp-version', '--no-backup', '--dry-run', '--yes',
        '--external', '--no-external', '--no-anim', '--no-mouse',
        '--terminal', '--verbose', '--quiet', '--config',
        '--print-config', '--version', '--help'
    )

    $browsers = @('` + strings.Join(browserIDs(), "', '") + `')

    $previous = $commandAst.CommandElements[-2].Value
    switch ($previous) {
        '--browser' { $browsers | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
            [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
        }; return }
        '--completion' { @('bash', 'zsh', 'fish', 'powershell') | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
            [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
        }; return }
        '--rollback' { @('latest') | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
            [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
        }; return }
    }

    $flags | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
        [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterName', $_)
    }
}
`
