import AppKit

/// Buddy Agent: a menu-bar-only process that owns Jamf integrations and mints
/// bearers for the buddy CLI over a unix-domain socket. Config is served while
/// locked; unlock gates minting. It never returns a secret value.
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
	private var pendingShowAbout = false
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
		buildStatusItem()
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
		userRequestedQuit ? .terminateNow : .terminateCancel
	}

	func applicationWillTerminate(_ notification: Notification) {
		if let showSettingsObserver {
			CFNotificationCenterRemoveObserver(
				CFNotificationCenterGetDarwinNotifyCenter(),
				showSettingsObserver,
				AgentInterprocess.showSettingsNotification,
				nil)
		}
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

	@objc private func toggleStatusPanel(_ sender: Any?) {
		guard let button = statusItem?.button else { return }
		statusPanel.toggle(
			relativeTo: button,
			snapshot: AgentStatusMenu.snapshot(unlocked: state.unlocked, authCount: store.authCount),
			delegate: self)
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
		if pendingOpenSettings {
			pendingOpenSettings = false
			windows.show()
		}
		if pendingShowAbout {
			pendingShowAbout = false
			showAboutNow()
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
		DispatchQueue.global(qos: .userInitiated).async { [gate, weak self] in
			let outcome = gate.authorize(reason: "unlock the Buddy Agent")
			guard outcome != .unlocked else {
				DispatchQueue.main.async { self?.refreshStatusItem() }
				return
			}
			DispatchQueue.main.async { self?.reportUnlock(outcome) }
		}
	}

	private func reportUnlock(_ outcome: UnlockOutcome) {
		let alert = NSAlert()
		alert.messageText = "Buddy Agent is still locked"
		switch outcome {
		case .timedOut:
			alert.informativeText = "The unlock prompt went unanswered."
		case .unavailable:
			alert.informativeText =
				"This Mac has no Touch ID or login password available to LocalAuthentication."
		default:
			alert.informativeText = "The unlock prompt was declined."
		}
		alert.runModal()
	}

	@objc private func lockVault() {
		state.lockVault()
	}

	private func presentSettingsWindow() {
		if menuIsOpen {
			pendingOpenSettings = true
			statusPanel.dismiss()
			return
		}
		windows.show()
	}

	@objc func openSettings() {
		presentSettingsWindow()
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
		presentSettingsWindow()
		AgentHelp.postShowHelpNotification()
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
		AgentAppActivation.activateIfAllowed()
		NSApp.orderFrontStandardAboutPanel(options: [
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
let agentServer = AgentServer(state: agentState, store: agentStore, gate: agentGate)
let application = NSApplication.shared
let agentDelegate = AgentAppDelegate(
	state: agentState, store: agentStore, server: agentServer, gate: agentGate)
application.setActivationPolicy(.accessory)
application.delegate = agentDelegate
application.run()
