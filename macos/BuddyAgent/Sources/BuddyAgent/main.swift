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
	private var titleItem: NSMenuItem?
	private var headerItem: NSMenuItem?
	private var openAtLoginItem: NSMenuItem?
	private var openBuddyItem: NSMenuItem?

	init(state: AgentState, store: AgentStore, server: AgentServer, gate: UnlockGate) {
		self.state = state
		self.store = store
		self.server = server
		self.gate = gate
		self.windows = AgentWindowController(
			store: store, state: state, gate: gate, socketPath: { server.socketPath })
	}

	func applicationDidFinishLaunching(_ notification: Notification) {
		state.onChange = { [weak self] in self?.refreshStatusItem() }
		// Network servers must not die when a peer closes before we finish writing
		// (agentReachable Dial+Close, unsigned probes, etc.).
		signal(SIGPIPE, SIG_IGN)
		// Drop pre-publish login-keychain copies without reading them (a read
		// would prompt once per older code signature). ACL items are untouched.
		AgentKeychain.purgeLoginLeftovers()
		do {
			try server.start()
		} catch {
			presentFatal("Could not listen on \(AgentPaths.socketPath): \(error)")
			return
		}
		buildStatusItem()
		if store.integrations.isEmpty {
			windows.show()
		}
	}

	func applicationWillTerminate(_ notification: Notification) {
		state.lockVault()
		server.stop()
	}

	private func buildStatusItem() {
		let item = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
		let menu = NSMenu()
		menu.delegate = self

		let title = NSMenuItem(title: "Buddy Agent", action: nil, keyEquivalent: "")
		title.isEnabled = false
		title.image = AgentBranding.menuIcon(size: 16)
		menu.addItem(title)
		titleItem = title

		let header = NSMenuItem(title: "Locked", action: nil, keyEquivalent: "")
		header.isEnabled = false
		menu.addItem(header)
		headerItem = header

		menu.addItem(.separator())
		menu.addItem(withTitle: "Unlock", action: #selector(unlock), keyEquivalent: "u").target = self
		menu.addItem(withTitle: "Lock", action: #selector(lockVault), keyEquivalent: "l").target = self
		menu.addItem(.separator())
		menu.addItem(withTitle: "Open Buddy Agent…", action: #selector(openWindow), keyEquivalent: "o")
			.target = self

		let openBuddy = NSMenuItem(
			title: "Open Buddy…", action: #selector(openBuddyApp), keyEquivalent: "")
		openBuddy.target = self
		menu.addItem(openBuddy)
		openBuddyItem = openBuddy

		let login = NSMenuItem(
			title: "Open at Login", action: #selector(toggleOpenAtLogin(_:)), keyEquivalent: "")
		login.target = self
		menu.addItem(login)
		openAtLoginItem = login

		menu.addItem(.separator())
		menu.addItem(withTitle: "About Buddy Agent…", action: #selector(showAbout), keyEquivalent: "")
			.target = self
		menu.addItem(withTitle: "Copy Socket Path", action: #selector(copySocketPath), keyEquivalent: "")
			.target = self
		menu.addItem(.separator())
		menu.addItem(withTitle: "Quit Buddy Agent", action: #selector(quit), keyEquivalent: "q").target =
			self
		item.menu = menu
		statusItem = item
		refreshStatusItem()
	}

	func menuNeedsUpdate(_ menu: NSMenu) {
		openAtLoginItem?.state = AgentLoginItem.isEnabled ? .on : .off
		openAtLoginItem?.isEnabled = AgentLoginItem.isAvailable
		openBuddyItem?.isEnabled = AgentBranding.buddyAppURL() != nil
		titleItem?.image = AgentBranding.menuIcon(size: 16)
	}

	private func refreshStatusItem() {
		let symbol = state.unlocked ? "lock.open.fill" : "lock.fill"
		let image = NSImage(systemSymbolName: symbol, accessibilityDescription: "Buddy Agent")
		image?.isTemplate = true  // follows menu-bar appearance
		statusItem?.button?.image = image
		statusItem?.button?.title = ""
		let count = store.integrations.count
		let plural = count == 1 ? "" : "s"
		let lockWord = state.unlocked ? "Unlocked" : "Locked"
		headerItem?.title = "\(lockWord) — \(count) integration\(plural)"
		statusItem?.button?.toolTip =
			"Buddy Agent \(AgentBranding.marketingVersion) — \(lockWord.lowercased())\n\(server.socketPath)"
	}

	@objc private func unlock() {
		// Same gate a token request goes through, so there is one presence path.
		// Off the main queue: authorize blocks until the prompt is answered.
		DispatchQueue.global(qos: .userInitiated).async { [gate, weak self] in
			let outcome = gate.authorize(reason: "unlock the Buddy Agent")
			guard outcome != .unlocked else { return }
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

	@objc private func openWindow() {
		windows.show()
	}

	@objc private func openBuddyApp() {
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

	@objc private func toggleOpenAtLogin(_ sender: NSMenuItem) {
		let enable = sender.state != .on
		if let message = AgentLoginItem.setEnabled(enable) {
			let alert = NSAlert()
			alert.messageText = "Open at Login"
			alert.informativeText = message
			alert.runModal()
		}
		sender.state = AgentLoginItem.isEnabled ? .on : .off
	}

	@objc private func showAbout() {
		NSApp.activate(ignoringOtherApps: true)
		let credits = NSAttributedString(
			string: AgentBranding.copyright
				+ "\nIndependent open-source software — not affiliated with or endorsed by Jamf.",
			attributes: [
				.font: NSFont.systemFont(ofSize: NSFont.smallSystemFontSize),
				.foregroundColor: NSColor.secondaryLabelColor,
			])
		NSApp.orderFrontStandardAboutPanel(options: [
			.applicationName: "Buddy Agent",
			.applicationVersion: AgentBranding.marketingVersion,
			.version: AgentBranding.buildNumber,
			.credits: credits,
		])
	}

	@objc private func copySocketPath() {
		NSPasteboard.general.clearContents()
		NSPasteboard.general.setString(server.socketPath, forType: .string)
	}

	@objc private func quit() {
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
