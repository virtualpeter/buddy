import AppKit

/// Buddy Agent: a menu-bar-only process that owns Jamf integrations and mints
/// bearers for the buddy CLI over a unix-domain socket. Config is served while
/// locked; unlock gates minting. It never returns a secret value.
final class AgentAppDelegate: NSObject, NSApplicationDelegate, NSMenuDelegate {
	private let state: AgentState
	private let store: AgentStore
	private let server: AgentServer
	private let gate: UnlockGate
	private let windows: AgentWindowController
	private var statusItem: NSStatusItem?
	private var statusMenu: NSMenu?
	private var statusSummaryItem: NSMenuItem?
	private var lockMenuItem: NSMenuItem?
	private var openAtLoginItem: NSMenuItem?
	private var openBuddyItem: NSMenuItem?
	private var menuIsOpen = false
	private var pendingStatusRefresh = false
	private var showSettingsObserver: UnsafeMutableRawPointer?

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
	}

	func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
		false
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

		let menu = AgentStatusMenu.makeMenu(delegate: self)
		statusMenu = menu
		captureMenuItems(menu)
		item.menu = menu
		refreshStatusItem()
	}

	private func captureMenuItems(_ menu: NSMenu) {
		statusSummaryItem = menu.item(at: 0)
		lockMenuItem = menu.item(at: 1)
		openAtLoginItem = menu.item(at: 5)
		openBuddyItem = menu.item(at: 4)
	}

	func menuWillOpen(_ menu: NSMenu) {
		menuIsOpen = true
		updateMenuItems()
	}

	func menuDidClose(_ menu: NSMenu) {
		menuIsOpen = false
		if pendingStatusRefresh {
			applyStatusItemRefresh()
		}
	}

	func menuNeedsUpdate(_ menu: NSMenu) {
		updateMenuItems()
	}

	private func updateMenuItems() {
		openAtLoginItem?.state = AgentLoginItem.isEnabled ? .on : .off
		openAtLoginItem?.isEnabled = AgentLoginItem.isAvailable
		openBuddyItem?.isEnabled = AgentBranding.buddyAppURL() != nil
		let lockWord = state.unlocked ? "Unlocked" : "Locked"
		let plural = store.authCount == 1 ? "" : "s"
		statusSummaryItem?.title =
			"Buddy Agent — \(lockWord) (\(store.authCount) auth\(plural))"
		lockMenuItem?.title = state.unlocked ? "Lock Agent" : "Unlock Agent"
	}

	private func refreshStatusItem() {
		if menuIsOpen {
			pendingStatusRefresh = true
			updateMenuItems()
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
			"Version \(AgentBranding.marketingVersion), \(lockWord.lowercased()), socket at \(server.socketPath)")
		updateMenuItems()
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

	@objc func openSettings() {
		windows.show()
	}

	private func offerSetupIfNeeded() {
		let key = "BuddyAgentOfferedEmptySetup"
		guard store.isEmpty, !UserDefaults.standard.bool(forKey: key) else { return }
		UserDefaults.standard.set(true, forKey: key)
		windows.show()
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

	@objc func toggleOpenAtLogin(_ sender: NSMenuItem) {
		let enable = sender.state != .on
		if let message = AgentLoginItem.setEnabled(enable) {
			let alert = NSAlert()
			alert.messageText = "Open at Login"
			alert.informativeText = message
			alert.runModal()
		}
		sender.state = AgentLoginItem.isEnabled ? .on : .off
		updateMenuItems()
	}

	@objc func showHelp() {
		windows.show()
		AgentHelp.postShowHelpNotification()
	}

	@objc func showAbout() {
		NSApp.activate(ignoringOtherApps: true)
		NSApp.orderFrontStandardAboutPanel(options: [
			.applicationName: "Buddy Agent",
			.applicationVersion: AgentBranding.marketingVersion,
			.version: AgentBranding.buildNumber,
			.credits: AgentBranding.aboutCredits(),
		])
	}

	@objc func quit() {
		NSApp.terminate(nil)
	}

	private func presentFatal(_ message: String) {
		let alert = NSAlert()
		alert.messageText = "Buddy Agent"
		alert.informativeText = message
		alert.alertStyle = .critical
		alert.runModal()
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
