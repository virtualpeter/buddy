import AppKit
import SwiftUI

/// Brings the Settings window back after LocalAuthentication on an accessory app.
enum AgentWindowRefocus {
	/// Order the settings window forward on the next run-loop turn. Avoids
	/// reentrant layout and does not call makeKey (SwiftUI popovers use glass
	/// windows that cannot become key).
	static func bringSettingsForward() {
		DispatchQueue.main.async {
			NSApp.activate(ignoringOtherApps: true)
			AgentWindowController.settingsWindow?.orderFrontRegardless()
		}
	}
}

/// Holds the single Agent window. The app is an accessory (no Dock tile), so
/// the window is created lazily from the menu and the app is activated
/// explicitly to bring it forward.
final class AgentWindowController: NSObject, NSWindowDelegate {
	/// Settings window for refocus after LocalAuthentication (avoid scanning NSApp.windows).
	static weak var settingsWindow: NSWindow?

	private var window: NSWindow?
	private let store: AgentStore
	private let state: AgentState
	private let gate: UnlockGate
	private let socketPath: () -> String

	init(
		store: AgentStore, state: AgentState, gate: UnlockGate,
		socketPath: @escaping () -> String
	) {
		self.store = store
		self.state = state
		self.gate = gate
		self.socketPath = socketPath
		super.init()
	}

	func show() {
		if window == nil {
			let root = AgentRootView(
				store: store, state: state, log: AgentLog.shared, gate: gate,
				socketPath: socketPath(),
				onImportFinished: { [weak self] in self?.show() })
			let window = NSWindow(contentViewController: NSHostingController(rootView: root))
			window.title = "Buddy Agent"
			window.styleMask = [.titled, .closable, .miniaturizable, .resizable]
			window.minSize = NSSize(width: 720, height: 520)
			window.setContentSize(NSSize(width: 760, height: 540))
			window.center()
			window.isReleasedWhenClosed = false
			// Stay visible while Touch ID / open panels deactivate the accessory app.
			window.hidesOnDeactivate = false
			window.collectionBehavior = [.moveToActiveSpace, .fullScreenAuxiliary]
			window.delegate = self
			self.window = window
			Self.settingsWindow = window
		}
		guard let window else { return }
		DispatchQueue.main.async {
			window.orderFrontRegardless()
			NSApp.activate(ignoringOtherApps: true)
			window.makeKey()
		}
	}

	func windowWillClose(_ notification: Notification) {
		if (notification.object as? NSWindow) === window {
			Self.settingsWindow = nil
		}
	}
}
