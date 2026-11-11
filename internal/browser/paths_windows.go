package browser

var platformPaths = map[ID]paths{
	Chrome: {
		executables: []string{
			`%PROGRAMFILES%\Google\Chrome\Application\chrome.exe`,
			`%PROGRAMFILES(X86)%\Google\Chrome\Application\chrome.exe`,
			`%LOCALAPPDATA%\Google\Chrome\Application\chrome.exe`,
		},
		installRoots: []string{
			`%PROGRAMFILES%\Google\Chrome\Application`,
			`%PROGRAMFILES(X86)%\Google\Chrome\Application`,
			`%LOCALAPPDATA%\Google\Chrome\Application`,
		},
		userDataDirs: []string{`%LOCALAPPDATA%\Google\Chrome\User Data`},
	},
	ChromeBeta: {
		executables: []string{
			`%PROGRAMFILES%\Google\Chrome Beta\Application\chrome.exe`,
			`%LOCALAPPDATA%\Google\Chrome Beta\Application\chrome.exe`,
		},
		installRoots: []string{
			`%PROGRAMFILES%\Google\Chrome Beta\Application`,
			`%LOCALAPPDATA%\Google\Chrome Beta\Application`,
		},
		userDataDirs: []string{`%LOCALAPPDATA%\Google\Chrome Beta\User Data`},
	},
	ChromeDev: {
		executables: []string{
			`%PROGRAMFILES%\Google\Chrome Dev\Application\chrome.exe`,
			`%LOCALAPPDATA%\Google\Chrome Dev\Application\chrome.exe`,
		},
		installRoots: []string{
			`%PROGRAMFILES%\Google\Chrome Dev\Application`,
			`%LOCALAPPDATA%\Google\Chrome Dev\Application`,
		},
		userDataDirs: []string{`%LOCALAPPDATA%\Google\Chrome Dev\User Data`},
	},
	ChromeCanary: {
		executables: []string{
			`%LOCALAPPDATA%\Google\Chrome SxS\Application\chrome.exe`,
			`%PROGRAMFILES%\Google\Chrome SxS\Application\chrome.exe`,
		},
		installRoots: []string{
			`%LOCALAPPDATA%\Google\Chrome SxS\Application`,
			`%PROGRAMFILES%\Google\Chrome SxS\Application`,
		},
		userDataDirs: []string{`%LOCALAPPDATA%\Google\Chrome SxS\User Data`},
	},
	Edge: {
		executables: []string{
			`%PROGRAMFILES(X86)%\Microsoft\Edge\Application\msedge.exe`,
			`%PROGRAMFILES%\Microsoft\Edge\Application\msedge.exe`,
			`%LOCALAPPDATA%\Microsoft\Edge\Application\msedge.exe`,
		},
		installRoots: []string{
			`%PROGRAMFILES(X86)%\Microsoft\Edge\Application`,
			`%PROGRAMFILES%\Microsoft\Edge\Application`,
			`%LOCALAPPDATA%\Microsoft\Edge\Application`,
		},
		userDataDirs: []string{`%LOCALAPPDATA%\Microsoft\Edge\User Data`},
	},
	EdgeBeta: {
		executables: []string{
			`%PROGRAMFILES(X86)%\Microsoft\Edge Beta\Application\msedge.exe`,
			`%PROGRAMFILES%\Microsoft\Edge Beta\Application\msedge.exe`,
		},
		installRoots: []string{
			`%PROGRAMFILES(X86)%\Microsoft\Edge Beta\Application`,
			`%PROGRAMFILES%\Microsoft\Edge Beta\Application`,
		},
		userDataDirs: []string{`%LOCALAPPDATA%\Microsoft\Edge Beta\User Data`},
	},
	EdgeDev: {
		executables: []string{
			`%PROGRAMFILES(X86)%\Microsoft\Edge Dev\Application\msedge.exe`,
			`%PROGRAMFILES%\Microsoft\Edge Dev\Application\msedge.exe`,
		},
		installRoots: []string{
			`%PROGRAMFILES(X86)%\Microsoft\Edge Dev\Application`,
			`%PROGRAMFILES%\Microsoft\Edge Dev\Application`,
		},
		userDataDirs: []string{`%LOCALAPPDATA%\Microsoft\Edge Dev\User Data`},
	},
	EdgeCanary: {
		executables: []string{
			`%LOCALAPPDATA%\Microsoft\Edge SxS\Application\msedge.exe`,
			`%PROGRAMFILES%\Microsoft\Edge SxS\Application\msedge.exe`,
		},
		installRoots: []string{
			`%LOCALAPPDATA%\Microsoft\Edge SxS\Application`,
			`%PROGRAMFILES%\Microsoft\Edge SxS\Application`,
		},
		userDataDirs: []string{`%LOCALAPPDATA%\Microsoft\Edge SxS\User Data`},
	},
	Brave: {
		executables: []string{
			`%PROGRAMFILES%\BraveSoftware\Brave-Browser\Application\brave.exe`,
			`%PROGRAMFILES(X86)%\BraveSoftware\Brave-Browser\Application\brave.exe`,
			`%LOCALAPPDATA%\BraveSoftware\Brave-Browser\Application\brave.exe`,
		},
		installRoots: []string{
			`%PROGRAMFILES%\BraveSoftware\Brave-Browser\Application`,
			`%PROGRAMFILES(X86)%\BraveSoftware\Brave-Browser\Application`,
			`%LOCALAPPDATA%\BraveSoftware\Brave-Browser\Application`,
		},
		userDataDirs: []string{`%LOCALAPPDATA%\BraveSoftware\Brave-Browser\User Data`},
	},
	Vivaldi: {
		executables: []string{
			`%LOCALAPPDATA%\Vivaldi\Application\vivaldi.exe`,
			`%PROGRAMFILES%\Vivaldi\Application\vivaldi.exe`,
		},
		installRoots: []string{
			`%LOCALAPPDATA%\Vivaldi\Application`,
			`%PROGRAMFILES%\Vivaldi\Application`,
		},
		userDataDirs: []string{`%LOCALAPPDATA%\Vivaldi\User Data`},
	},
	Opera: {
		executables: []string{
			`%LOCALAPPDATA%\Programs\Opera\opera.exe`,
			`%PROGRAMFILES%\Opera\opera.exe`,
			`%PROGRAMFILES(X86)%\Opera\opera.exe`,
		},
		installRoots: []string{
			`%LOCALAPPDATA%\Programs\Opera`,
			`%PROGRAMFILES%\Opera`,
			`%PROGRAMFILES(X86)%\Opera`,
		},
		userDataDirs: []string{
			`%APPDATA%\Opera Software\Opera Stable`,
			`%LOCALAPPDATA%\Opera Software\Opera Stable`,
		},
	},
	OperaGX: {
		executables: []string{
			`%LOCALAPPDATA%\Programs\Opera GX\opera.exe`,
			`%PROGRAMFILES%\Opera GX\opera.exe`,
			`%PROGRAMFILES(X86)%\Opera GX\opera.exe`,
		},
		installRoots: []string{
			`%LOCALAPPDATA%\Programs\Opera GX`,
			`%PROGRAMFILES%\Opera GX`,
			`%PROGRAMFILES(X86)%\Opera GX`,
		},
		userDataDirs: []string{
			`%APPDATA%\Opera Software\Opera GX Stable`,
			`%LOCALAPPDATA%\Opera Software\Opera GX Stable`,
		},
	},
	Chromium: {
		executables: []string{
			`%LOCALAPPDATA%\Chromium\Application\chrome.exe`,
			`%PROGRAMFILES%\Chromium\Application\chrome.exe`,
		},
		installRoots: []string{
			`%LOCALAPPDATA%\Chromium\Application`,
			`%PROGRAMFILES%\Chromium\Application`,
		},
		userDataDirs: []string{`%LOCALAPPDATA%\Chromium\User Data`},
	},
	UngoogledChromium: {
		executables: []string{
			`%PROGRAMFILES%\ungoogled-chromium\chrome.exe`,
			`%LOCALAPPDATA%\ungoogled-chromium\Application\chrome.exe`,
			`%USERPROFILE%\ungoogled-chromium\chrome.exe`,
		},
		installRoots: []string{
			`%PROGRAMFILES%\ungoogled-chromium`,
			`%LOCALAPPDATA%\ungoogled-chromium\Application`,
			`%USERPROFILE%\ungoogled-chromium`,
		},
		userDataDirs: []string{
			`%LOCALAPPDATA%\ungoogled-chromium\User Data`,
			`%USERPROFILE%\ungoogled-chromium\User Data`,
		},
	},
}
