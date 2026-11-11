package browser

var platformPaths = map[ID]paths{
	Chrome: {
		executables:  []string{"/usr/bin/google-chrome-stable", "/usr/bin/google-chrome", "/opt/google/chrome/google-chrome"},
		installRoots: []string{"/opt/google/chrome"},
		userDataDirs: []string{"~/.config/google-chrome", "~/.config/google-chrome-stable"},
	},
	ChromeBeta: {
		executables:  []string{"/usr/bin/google-chrome-beta", "/opt/google/chrome-beta/google-chrome"},
		installRoots: []string{"/opt/google/chrome-beta"},
		userDataDirs: []string{"~/.config/google-chrome-beta"},
	},
	ChromeDev: {
		executables:  []string{"/usr/bin/google-chrome-unstable", "/opt/google/chrome-unstable/google-chrome"},
		installRoots: []string{"/opt/google/chrome-unstable"},
		userDataDirs: []string{"~/.config/google-chrome-unstable"},
	},
	Edge: {
		executables:  []string{"/usr/bin/microsoft-edge-stable", "/usr/bin/microsoft-edge", "/opt/microsoft/msedge/msedge"},
		installRoots: []string{"/opt/microsoft/msedge"},
		userDataDirs: []string{"~/.config/microsoft-edge", "~/.config/microsoft-edge-stable"},
	},
	EdgeBeta: {
		executables:  []string{"/usr/bin/microsoft-edge-beta", "/opt/microsoft/msedge-beta/msedge"},
		installRoots: []string{"/opt/microsoft/msedge-beta"},
		userDataDirs: []string{"~/.config/microsoft-edge-beta"},
	},
	EdgeDev: {
		executables:  []string{"/usr/bin/microsoft-edge-dev", "/opt/microsoft/msedge-dev/msedge"},
		installRoots: []string{"/opt/microsoft/msedge-dev"},
		userDataDirs: []string{"~/.config/microsoft-edge-dev"},
	},
	Brave: {
		executables:  []string{"/usr/bin/brave-browser", "/usr/bin/brave", "/opt/brave.com/brave/brave"},
		installRoots: []string{"/opt/brave.com/brave", "/usr/lib/brave-browser"},
		userDataDirs: []string{"~/.config/BraveSoftware/Brave-Browser"},
	},
	Vivaldi: {
		executables:  []string{"/usr/bin/vivaldi-stable", "/usr/bin/vivaldi", "/opt/vivaldi/vivaldi"},
		installRoots: []string{"/opt/vivaldi", "/usr/lib/vivaldi"},
		userDataDirs: []string{"~/.config/vivaldi", "~/.config/vivaldi-stable"},
	},
	Opera: {
		executables:  []string{"/usr/bin/opera", "/usr/lib/x86_64-linux-gnu/opera/opera", "/usr/lib/opera/opera"},
		installRoots: []string{"/usr/lib/x86_64-linux-gnu/opera", "/usr/lib/opera"},
		userDataDirs: []string{"~/.config/opera"},
	},
	OperaGX: {
		executables:  []string{"/usr/bin/opera-gx", "/usr/lib/x86_64-linux-gnu/opera-gx/opera"},
		installRoots: []string{"/usr/lib/x86_64-linux-gnu/opera-gx"},
		userDataDirs: []string{"~/.config/opera-gx"},
	},
	Chromium: {
		executables:  []string{"/usr/bin/chromium", "/usr/bin/chromium-browser", "/snap/bin/chromium"},
		installRoots: []string{"/usr/lib/chromium", "/usr/lib/chromium-browser", "/snap/chromium/current/usr/lib/chromium-browser", "/opt/chromium"},
		userDataDirs: []string{"~/.config/chromium"},
	},
	UngoogledChromium: {
		executables:  []string{"/usr/bin/ungoogled-chromium", "/usr/lib/ungoogled-chromium/chrome", "/opt/ungoogled-chromium/chrome"},
		installRoots: []string{"/usr/lib/ungoogled-chromium", "/opt/ungoogled-chromium"},
		userDataDirs: []string{"~/.config/ungoogled-chromium"},
	},
}
