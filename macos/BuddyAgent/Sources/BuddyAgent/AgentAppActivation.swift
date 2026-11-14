import AppKit

/// Activation for a menu-bar minter that also owns a real window.
///
/// Launch stays accessory (no Dock tile). Opening Settings or About adopts the
/// regular policy and leaves it there until the last of those windows closes.
/// The status panel is not a window we key on show: it stays non-activating so
/// its glass surface is not asked to `makeKey`.
enum AgentAppActivation {
	/// True while the status-item panel is on screen.
	private(set) static var statusMenuIsOpen = false

	private static var usingRegularPolicy = false
	private static var aboutHold = 0
	private static var aboutWatch: [ObjectIdentifier: NSObjectProtocol] = [:]

	static func setStatusMenuOpen(_ open: Bool) {
		statusMenuIsOpen = open
	}

	/// Show the Settings window and keep the regular policy until it closes.
	static func present(_ window: NSWindow?) {
		guard let window else { return }
		let raise = {
			adoptRegularPolicy()
			NSApp.activate(ignoringOtherApps: true)
			if window.canBecomeKey {
				window.makeKeyAndOrderFront(nil)
			} else {
				window.orderFrontRegardless()
			}
		}
		if Thread.isMainThread {
			raise()
		} else {
			DispatchQueue.main.async(execute: raise)
		}
	}

	/// Standard About panel. Holds the regular policy until that panel closes.
	static func presentAbout(options: [NSApplication.AboutPanelOptionKey: Any]) {
		let work = { presentAboutOnMain(options: options) }
		if Thread.isMainThread {
			work()
		} else {
			DispatchQueue.main.async(execute: work)
		}
	}

	static func settingsWindowDidClose() {
		endRegularSessionIfIdle()
	}

	static func orderForward(_ window: NSWindow?) {
		window?.orderFrontRegardless()
	}

	/// For the system Touch ID sheet. Does not adopt the regular policy: a tray
	/// unlock has no window, and the Dock tile should stay hidden.
	static func activateIfAllowed() {
		NSApp.activate(ignoringOtherApps: true)
	}

	private static func adoptRegularPolicy() {
		if NSApp.activationPolicy() != .regular {
			NSApp.setActivationPolicy(.regular)
		}
		usingRegularPolicy = true
	}

	private static func presentAboutOnMain(options: [NSApplication.AboutPanelOptionKey: Any]) {
		adoptRegularPolicy()
		let before = Set(NSApp.windows.map(ObjectIdentifier.init))
		NSApp.activate(ignoringOtherApps: true)
		NSApp.orderFrontStandardAboutPanel(options: options)
		if let candidate = aboutCandidate(before: before) {
			watchAbout(candidate)
			return
		}
		DispatchQueue.main.async {
			if let candidate = aboutCandidate(before: before) {
				watchAbout(candidate)
			} else {
				endRegularSessionIfIdle()
			}
		}
	}

	private static func aboutCandidate(before: Set<ObjectIdentifier>) -> NSWindow? {
		let fresh = NSApp.windows.filter {
			!before.contains(ObjectIdentifier($0)) && $0.isVisible && isUserFacing($0)
		}
		if let window = fresh.first { return window }
		// orderFrontStandardAboutPanel makes the About panel key. Settings and the
		// status panel are excluded by isUserFacing.
		if let key = NSApp.keyWindow, isUserFacing(key) { return key }
		return NSApp.windows.first { $0.isVisible && isAboutPanel($0) && isUserFacing($0) }
	}

	private static func isUserFacing(_ window: NSWindow) -> Bool {
		if window === AgentWindowController.settingsWindow { return false }
		if window.level == .popUpMenu || window.level == .statusBar { return false }
		return true
	}

	private static func isAboutPanel(_ window: NSWindow) -> Bool {
		window.className.localizedCaseInsensitiveContains("About")
			|| window.title.localizedCaseInsensitiveContains("About")
	}

	private static func watchAbout(_ window: NSWindow) {
		let id = ObjectIdentifier(window)
		guard aboutWatch[id] == nil else { return }
		aboutHold += 1
		aboutWatch[id] = NotificationCenter.default.addObserver(
			forName: NSWindow.willCloseNotification,
			object: window,
			queue: .main
		) { _ in
			if let token = aboutWatch.removeValue(forKey: id) {
				NotificationCenter.default.removeObserver(token)
			}
			aboutHold = max(0, aboutHold - 1)
			endRegularSessionIfIdle()
		}
	}

	private static func endRegularSessionIfIdle() {
		guard usingRegularPolicy else { return }
		// Nil only after the settings window actually closes, including while minimized.
		if AgentWindowController.settingsWindow != nil { return }
		if aboutHold > 0 { return }
		usingRegularPolicy = false
		if NSApp.activationPolicy() != .accessory {
			NSApp.setActivationPolicy(.accessory)
		}
	}
}
