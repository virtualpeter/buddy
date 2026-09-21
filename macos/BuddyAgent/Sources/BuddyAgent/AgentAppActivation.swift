import AppKit

/// Brings user-facing windows forward without calling makeKey on non-keyable
/// panels (status menus use AppKit glass windows that reject key on macOS 26+).
enum AgentAppActivation {
	/// True while the status-item menu panel is on screen.
	private(set) static var statusMenuIsOpen = false

	static func setStatusMenuOpen(_ open: Bool) {
		statusMenuIsOpen = open
	}

	/// Show the Settings window. Stays `.accessory` (no Dock tile — Dock Quit would
	/// feel like closing Settings only).
	static func present(_ window: NSWindow?) {
		guard let window, window.canBecomeKey else { return }
		window.orderFrontRegardless()
		window.makeKey()
	}

	static func orderForward(_ window: NSWindow?) {
		window?.orderFrontRegardless()
	}

	/// For system sheets (Touch ID, About) that need app activation. Skipped while
	/// the status menu is open so AppKit does not try to key the menu panel.
	static func activateIfAllowed() {
		guard !statusMenuIsOpen else { return }
		NSApp.activate(ignoringOtherApps: true)
	}
}
