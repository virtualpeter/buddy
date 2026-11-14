import AppKit
import SwiftUI

/// Brings the Settings window back after LocalAuthentication.
enum AgentWindowRefocus {
	/// One `makeKeyAndOrderFront` on the next turn, once the system sheet is gone.
	static func bringSettingsForward() {
		DispatchQueue.main.async {
			AgentAppActivation.present(AgentWindowController.settingsWindow)
		}
	}
}

/// Holds the single Agent window. The process launches as an accessory; while this
/// window is open the app adopts the regular policy (Dock tile) and can become key.
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

	func closeSettings() {
		window?.close()
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
			AgentAppActivation.present(window)
		}
	}

	func windowWillClose(_ notification: Notification) {
		if (notification.object as? NSWindow) === window {
			Self.settingsWindow = nil
			AgentAppActivation.settingsWindowDidClose()
		}
	}
}
