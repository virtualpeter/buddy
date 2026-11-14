import AppKit

/// Buddy Agent: a menu-bar minter that owns Jamf integrations and mints bearers
/// for the buddy CLI over a unix-domain socket. The tray is a satellite; the
/// Settings window is a normal key window while it is open. Config is served
/// while locked; unlock gates minting. It never returns a secret value.
final class AgentAppDelegate: NSObject, NSApplicationDelegate {
	private let state: AgentState
	private let store: AgentStore
	private let server: AgentServer
	private let gate: UnlockGate
	private let windows: AgentWindowController
	private let statusPanel = AgentStatusPanelController()
	private var statusItem: NSStatusItem?
	private var menuIsOpen = false
	private var pendingStatusRefresh = false
	private var pendingOpenSettings = false
	private var pendingShowHelp = false
	private var pendingShowAbout = false
	private var pendingRevealAuth: String?
	private var pendingBearerAuth: String?
	private var showSettingsObserver: UnsafeMutableRawPointer?
	private var userRequestedQuit = false

	init(state: AgentState, store: AgentStore, server: AgentServer, gate: UnlockGate) {
		self.state = state
		self.store = store
		self.server = server
		self.gate = gate
		self.windows = AgentWindowController(
			store: store, state: state, gate: gate, socketPath: { server.socketPath })
		super.init()
	}

	func applicationDidFinishLaunching(_ notification: Notification) {
		AgentMainMenu.install()
		AgentMainMenu.wire(
			to: self, about: #selector(showAbout), quit: #selector(quitFromApplicationMenu))
		state.onChange = { [weak self] in self?.refreshStatusItem() }
		signal(SIGPIPE, SIG_IGN)
		AgentKeychain.purgeLoginLeftovers()
		do {
			try server.start()
		} catch {
			presentFatal("Could not listen on \(AgentPaths.socketPath): \(error)")
			return
		}
		AgentKeychainRecovery.start(
			prepareForReplacement: { [weak self] in self?.server.handoffSocket() },
			resumeListening: { [weak self] in
				guard let self else { return }
				do {
					try self.server.start()
				} catch {
					self.presentFatal("Could not listen on \(AgentPaths.socketPath): \(error)")
				}
			},
			quit: { [weak self] in self?.quit() })
		buildStatusItem()
		refreshUpdateCheck()
		store.startConfigFileMonitor { [weak state] in state?.unlocked ?? false }
		offerSetupIfNeeded()
		registerShowSettingsObserver()
		if store.isEmpty {
			AgentIdleLock.applyEmptyConfigDefault()
			state.refreshIdleLockTimer()
		}
	}

	func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
		false
	}

	func applicationShouldTerminate(_ sender: NSApplication) -> NSApplication.TerminateReply {
		if userRequestedQuit { return .terminateNow }
		// Dock Quit while a window is open (regular policy). ⌘Q is wired to
		// close the window and does not come through here.
		if NSApp.activationPolicy() == .regular {
			userRequestedQuit = true
			return .terminateNow
		}
		return .terminateCancel
	}

	func applicationWillTerminate(_ notification: Notification) {
		if let showSettingsObserver {
			CFNotificationCenterRemoveObserver(
				CFNotificationCenterGetDarwinNotifyCenter(),
				showSettingsObserver,
				AgentInterprocess.showSettingsNotification,
				nil)
		}
		store.stopConfigFileMonitor()
		state.lockVault()
		server.stop()
	}

	private func registerShowSettingsObserver() {
		let obs = Unmanaged.passUnretained(self).toOpaque()
		showSettingsObserver = obs
		CFNotificationCenterAddObserver(
			CFNotificationCenterGetDarwinNotifyCenter(),
			obs,
			{ _, observer, _, _, _ in
				guard let observer else { return }
				let delegate = Unmanaged<AgentAppDelegate>.fromOpaque(observer).takeUnretainedValue()
				DispatchQueue.main.async { delegate.openSettings() }
			},
			AgentInterprocess.showSettingsNotificationName,
			nil,
			.deliverImmediately)
	}

	private func buildStatusItem() {
		let item = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
		item.autosaveName = "BuddyAgentStatusItem"
		statusItem = item
		item.button?.target = self
		item.button?.action = #selector(toggleStatusPanel(_:))
		refreshStatusItem()
	}

	private func refreshUpdateCheck() {
		let enabled = store.managedPrefs.checkUpdates ?? (store.features.checkUpdates != false)
		let revision = AgentBranding.revision
		MainActor.assumeIsolated {
			AgentUpdateCheck.shared.refresh(enabled: enabled, current: revision)
		}
	}

	@objc private func toggleStatusPanel(_ sender: Any?) {
		guard let button = statusItem?.button else { return }
		refreshUpdateCheck()
		statusPanel.toggle(
			relativeTo: button, store: store, state: state, gate: gate, delegate: self)
	}

	func statusPanelWillOpen() {
		menuIsOpen = true
		AgentAppActivation.setStatusMenuOpen(true)
	}

	func statusPanelDidClose() {
		menuIsOpen = false
		AgentAppActivation.setStatusMenuOpen(false)
		if pendingStatusRefresh {
			applyStatusItemRefresh()
		}
		let openSettings = pendingOpenSettings
		let showHelp = pendingShowHelp
		let revealAuth = pendingRevealAuth
		let bearerAuth = pendingBearerAuth
		let showAbout = pendingShowAbout
		pendingOpenSettings = false
		pendingShowHelp = false
		pendingRevealAuth = nil
		pendingBearerAuth = nil
		pendingShowAbout = false
		guard openSettings || showAbout else { return }
		// Let the panel finish ordering out before the window becomes key.
		DispatchQueue.main.async { [weak self] in
			guard let self else { return }
			if openSettings {
				self.windows.show()
				if showHelp {
					DispatchQueue.main.async { AgentHelp.postShowHelpNotification() }
				}
				if let revealAuth {
					DispatchQueue.main.async { AgentRevealAuth.post(revealAuth) }
				}
				if let bearerAuth {
					DispatchQueue.main.async { AgentRevealAuth.postBearer(bearerAuth) }
				}
			}
			if showAbout {
				self.showAboutNow()
			}
		}
	}

	private func refreshStatusItem() {
		if menuIsOpen {
			pendingStatusRefresh = true
			return
		}
		applyStatusItemRefresh()
	}

	private func applyStatusItemRefresh() {
		pendingStatusRefresh = false
		let symbol = state.unlocked ? "lock.open.fill" : "lock.fill"
		let image = NSImage(systemSymbolName: symbol, accessibilityDescription: "Buddy Agent")
		image?.isTemplate = true
		statusItem?.button?.image = image
		statusItem?.button?.title = ""
		let lockWord = state.unlocked ? "Unlocked" : "Locked"
		statusItem?.button?.setAccessibilityLabel("Buddy Agent")
		statusItem?.button?.setAccessibilityHelp(
			"Version \(AgentBranding.revision), \(lockWord.lowercased()), socket at \(server.socketPath)")
	}

	@objc func toggleLockFromMenu() {
		if state.unlocked {
			lockVault()
		} else {
			unlock()
		}
	}

	@objc private func unlock() {
		AgentLockUI.unlock(gate: gate) { [weak self] outcome in
			guard let self else { return }
			if outcome == .unlocked {
				refreshStatusItem()
			} else {
				reportUnlock(outcome)
			}
		}
	}

	private func reportUnlock(_ outcome: UnlockOutcome) {
		let copy = AgentLockUI.failureAlert(for: outcome)
		let alert = NSAlert()
		alert.messageText = copy.title
		alert.informativeText = copy.message
		alert.runModal()
	}

	@objc private func lockVault() {
		state.lockVault()
	}

	@objc func openSettings() {
		presentSettingsWindow()
	}

	func openAuthFromPanel(_ name: String) {
		presentSettingsWindow(revealAuth: name)
	}

	func openBearerFromPanel(_ name: String) {
		presentSettingsWindow(bearerAuth: name)
	}

	private func offerSetupIfNeeded() {
		let key = "BuddyAgentOfferedEmptySetup"
		guard store.isEmpty, !UserDefaults.standard.bool(forKey: key) else { return }
		UserDefaults.standard.set(true, forKey: key)
		DispatchQueue.main.async { [weak self] in
			self?.presentSettingsWindow()
		}
	}

	@objc func openBuddyApp() {
		guard let url = AgentBranding.buddyAppURL() else {
			let alert = NSAlert()
			alert.messageText = "Buddy.app not found"
			alert.informativeText =
				"Install Buddy.app in /Applications or ~/Applications, or run the Agent from inside Buddy.app."
			alert.runModal()
			return
		}
		NSWorkspace.shared.open(url)
	}

	@objc func toggleOpenAtLoginFromPanel() {
		let enable = !AgentLoginItem.isEnabled
		if let message = AgentLoginItem.setEnabled(enable) {
			let alert = NSAlert()
			alert.messageText = "Open at Login"
			alert.informativeText = message
			alert.runModal()
		}
	}

	@objc func showHelp() {
		presentSettingsWindow(showHelpAfter: true)
	}

	private func presentSettingsWindow(
		showHelpAfter: Bool = false, revealAuth: String? = nil, bearerAuth: String? = nil
	) {
		if menuIsOpen {
			pendingOpenSettings = true
			pendingShowHelp = showHelpAfter
			pendingRevealAuth = revealAuth
			pendingBearerAuth = bearerAuth
			statusPanel.dismiss()
			return
		}
		windows.show()
		if showHelpAfter {
			DispatchQueue.main.async { AgentHelp.postShowHelpNotification() }
		}
		if let revealAuth {
			DispatchQueue.main.async { AgentRevealAuth.post(revealAuth) }
		}
		if let bearerAuth {
			DispatchQueue.main.async { AgentRevealAuth.postBearer(bearerAuth) }
		}
	}

	@objc func showAbout() {
		if menuIsOpen {
			pendingShowAbout = true
			statusPanel.dismiss()
			return
		}
		showAboutNow()
	}

	private func showAboutNow() {
		AgentAppActivation.presentAbout(options: [
			.applicationName: "Buddy Agent",
			.applicationVersion: AgentBranding.revision,
			.version: AgentBranding.buildTime,
			.credits: AgentBranding.aboutCredits(),
		])
	}

	@objc func quitFromApplicationMenu() {
		if AgentWindowController.settingsWindow != nil {
			windows.closeSettings()
		}
	}

	@objc func quit() {
		userRequestedQuit = true
		NSApp.terminate(nil)
	}

	private func presentFatal(_ message: String) {
		let alert = NSAlert()
		alert.messageText = "Buddy Agent"
		alert.informativeText = message
		alert.alertStyle = .critical
		alert.runModal()
		userRequestedQuit = true
		NSApp.terminate(nil)
	}
}

let agentState = AgentState()
let agentStore = AgentStore()
let agentGate = UnlockGate(state: agentState)
agentStore.configSealPresence = {
	guard agentState.unlocked else { return nil }
	return agentGate.presenceContext
}
agentStore.runConfigSeal = { work in
	agentGate.withAuthenticatedContext(work)
}
agentGate.onSessionOpened = { presence in
	agentStore.resealConfig(presence: presence)
}
let agentServer = AgentServer(state: agentState, store: agentStore, gate: agentGate)
let application = NSApplication.shared
let agentDelegate = AgentAppDelegate(
	state: agentState, store: agentStore, server: agentServer, gate: agentGate)
application.setActivationPolicy(.accessory)
application.delegate = agentDelegate
application.run()
