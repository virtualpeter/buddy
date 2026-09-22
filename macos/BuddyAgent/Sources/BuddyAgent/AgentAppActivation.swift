import AppKit

/// Brings user-facing windows forward without calling makeKey on non-keyable
/// panels (status menus use AppKit glass windows that reject key on macOS 26+).
enum AgentAppActivation {
	/// True while the status-item menu panel is on screen.
	private(set) static var statusMenuIsOpen = false

	private static var usingRegularPolicyForSettings = false

	static func setStatusMenuOpen(_ open: Bool) {
		statusMenuIsOpen = open
	}

	/// Show the Settings window. Briefly uses `.regular` so AppKit will key and
	/// stack the window above other apps; reverts in `settingsWindowDidClose()`.
	static func present(_ window: NSWindow?) {
		guard let window else { return }
		let raise = {
			if NSApp.activationPolicy() != .regular {
				NSApp.setActivationPolicy(.regular)
				usingRegularPolicyForSettings = true
			}
			NSApp.activate(ignoringOtherApps: true)
			window.orderFrontRegardless()
			if window.canBecomeKey {
				window.makeKeyAndOrderFront(nil)
			}
		}
		let work = {
			raise()
			for delay in [0.0, 0.05] {
				DispatchQueue.main.asyncAfter(deadline: .now() + delay) {
					raise()
				}
			}
		}
		if Thread.isMainThread {
			work()
		} else {
			DispatchQueue.main.async(execute: work)
		}
	}

	static func settingsWindowDidClose() {
		guard usingRegularPolicyForSettings else { return }
		usingRegularPolicyForSettings = false
		NSApp.setActivationPolicy(.accessory)
	}

	static func orderForward(_ window: NSWindow?) {
		window?.orderFrontRegardless()
	}

	/// For system sheets (Touch ID, About) that need app activation.
	static func activateIfAllowed() {
		NSApp.activate(ignoringOtherApps: true)
	}
}
