package browser

var platformPaths = map[ID]paths{
	Chrome: {
		executables:  []string{"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"},
		installRoots: []string{"/Applications/Google Chrome.app"},
		userDataDirs: []string{"~/Library/Application Support/Google/Chrome"},
		appBundles:   []string{"/Applications/Google Chrome.app"},
	},
	ChromeBeta: {
		executables:  []string{"/Applications/Google Chrome Beta.app/Contents/MacOS/Google Chrome Beta"},
		installRoots: []string{"/Applications/Google Chrome Beta.app"},
		userDataDirs: []string{"~/Library/Application Support/Google/Chrome Beta"},
		appBundles:   []string{"/Applications/Google Chrome Beta.app"},
	},
	ChromeDev: {
		executables:  []string{"/Applications/Google Chrome Dev.app/Contents/MacOS/Google Chrome Dev"},
		installRoots: []string{"/Applications/Google Chrome Dev.app"},
		userDataDirs: []string{"~/Library/Application Support/Google/Chrome Dev"},
		appBundles:   []string{"/Applications/Google Chrome Dev.app"},
	},
	ChromeCanary: {
		executables:  []string{"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary"},
		installRoots: []string{"/Applications/Google Chrome Canary.app"},
		userDataDirs: []string{"~/Library/Application Support/Google/Chrome Canary"},
		appBundles:   []string{"/Applications/Google Chrome Canary.app"},
	},
	Edge: {
		executables:  []string{"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge"},
		installRoots: []string{"/Applications/Microsoft Edge.app"},
		userDataDirs: []string{"~/Library/Application Support/Microsoft Edge"},
		appBundles:   []string{"/Applications/Microsoft Edge.app"},
	},
	EdgeBeta: {
		executables:  []string{"/Applications/Microsoft Edge Beta.app/Contents/MacOS/Microsoft Edge Beta"},
		installRoots: []string{"/Applications/Microsoft Edge Beta.app"},
		userDataDirs: []string{"~/Library/Application Support/Microsoft Edge Beta"},
		appBundles:   []string{"/Applications/Microsoft Edge Beta.app"},
	},
	EdgeDev: {
		executables:  []string{"/Applications/Microsoft Edge Dev.app/Contents/MacOS/Microsoft Edge Dev"},
		installRoots: []string{"/Applications/Microsoft Edge Dev.app"},
		userDataDirs: []string{"~/Library/Application Support/Microsoft Edge Dev"},
		appBundles:   []string{"/Applications/Microsoft Edge Dev.app"},
	},
	EdgeCanary: {
		executables:  []string{"/Applications/Microsoft Edge Canary.app/Contents/MacOS/Microsoft Edge Canary"},
		installRoots: []string{"/Applications/Microsoft Edge Canary.app"},
		userDataDirs: []string{"~/Library/Application Support/Microsoft Edge Canary"},
		appBundles:   []string{"/Applications/Microsoft Edge Canary.app"},
	},
	Brave: {
		executables:  []string{"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser"},
		installRoots: []string{"/Applications/Brave Browser.app"},
		userDataDirs: []string{"~/Library/Application Support/BraveSoftware/Brave-Browser"},
		appBundles:   []string{"/Applications/Brave Browser.app"},
	},
	Vivaldi: {
		executables:  []string{"/Applications/Vivaldi.app/Contents/MacOS/Vivaldi"},
		installRoots: []string{"/Applications/Vivaldi.app"},
		userDataDirs: []string{"~/Library/Application Support/Vivaldi"},
		appBundles:   []string{"/Applications/Vivaldi.app"},
	},
	Opera: {
		executables:  []string{"/Applications/Opera.app/Contents/MacOS/Opera"},
		installRoots: []string{"/Applications/Opera.app"},
		userDataDirs: []string{"~/Library/Application Support/com.operasoftware.Opera"},
		appBundles:   []string{"/Applications/Opera.app"},
	},
	OperaGX: {
		executables:  []string{"/Applications/Opera GX.app/Contents/MacOS/Opera"},
		installRoots: []string{"/Applications/Opera GX.app"},
		userDataDirs: []string{"~/Library/Application Support/com.operasoftware.OperaGX"},
		appBundles:   []string{"/Applications/Opera GX.app"},
	},
	Chromium: {
		executables:  []string{"/Applications/Chromium.app/Contents/MacOS/Chromium"},
		installRoots: []string{"/Applications/Chromium.app"},
		userDataDirs: []string{"~/Library/Application Support/Chromium"},
		appBundles:   []string{"/Applications/Chromium.app"},
	},
	UngoogledChromium: {
		executables:  []string{"/Applications/ungoogled-chromium.app/Contents/MacOS/Chromium"},
		installRoots: []string{"/Applications/ungoogled-chromium.app"},
		userDataDirs: []string{"~/Library/Application Support/ungoogled-chromium"},
		appBundles:   []string{"/Applications/ungoogled-chromium.app"},
	},
}
