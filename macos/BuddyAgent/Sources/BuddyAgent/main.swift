import AppKit

/// Buddy Agent (spike): a menu-bar-only process that owns Jamf integrations and
/// mints bearers for the buddy CLI over a unix-domain socket. Config is served
/// while locked; unlock gates minting. It never returns a secret value.
final class AgentAppDelegate: NSObject, NSApplicationDelegate {
	private let state: AgentState
	private let store: AgentStore
	private let server: AgentServer
	private let windows: AgentWindowController
	private var statusItem: NSStatusItem?
	private var headerItem: NSMenuItem?

	init(state: AgentState, store: AgentStore, server: AgentServer) {
		self.state = state
		self.store = store
		self.server = server
		self.windows = AgentWindowController(
			store: store, state: state, socketPath: { server.socketPath })
	}

	func applicationDidFinishLaunching(_ notification: Notification) {
		state.onChange = { [weak self] in self?.refreshStatusItem() }
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
		let header = NSMenuItem(title: "Locked", action: nil, keyEquivalent: "")
		header.isEnabled = false
		menu.addItem(header)
		headerItem = header
		menu.addItem(.separator())
		menu.addItem(withTitle: "Unlock", action: #selector(unlock), keyEquivalent: "u").target = self
		menu.addItem(withTitle: "Lock", action: #selector(lockVault), keyEquivalent: "l").target = self
		menu.addItem(.separator())
		menu.addItem(withTitle: "Open Buddy Agent...", action: #selector(openWindow), keyEquivalent: "o")
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

	private func refreshStatusItem() {
		let symbol = state.unlocked ? "lock.open.fill" : "lock.fill"
		let image = NSImage(systemSymbolName: symbol, accessibilityDescription: "Buddy Agent")
		image?.isTemplate = true  // follows menu-bar appearance
		statusItem?.button?.image = image
		statusItem?.button?.title = ""
		let count = store.integrations.count
		let plural = count == 1 ? "" : "s"
		let lockWord = state.unlocked ? "Unlocked" : "Locked"
		headerItem?.title = "\(lockWord) - \(count) integration\(plural)"
		statusItem?.button?.toolTip =
			"Buddy Agent \(agentVersion) - \(lockWord.lowercased())\n\(server.socketPath)"
	}

	@objc private func unlock() {
		// Spike: the menu click is the gate. LocalAuthentication / Keychain
		// userPresence lands here once the Agent is a signed app.
		state.unlock()
	}

	@objc private func lockVault() {
		state.lockVault()
	}

	@objc private func openWindow() {
		windows.show()
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
let agentServer = AgentServer(state: agentState, store: agentStore)
let application = NSApplication.shared
let agentDelegate = AgentAppDelegate(state: agentState, store: agentStore, server: agentServer)
application.setActivationPolicy(.accessory)
application.delegate = agentDelegate
application.run()
