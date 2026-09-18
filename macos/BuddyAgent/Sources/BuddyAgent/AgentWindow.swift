import AppKit
import SwiftUI

/// Holds the single Agent window. The app is an accessory (no Dock tile), so
/// the window is created lazily from the menu and the app is activated
/// explicitly to bring it forward.
final class AgentWindowController {
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
	}

	func show() {
		if window == nil {
			let root = AgentRootView(
				store: store, state: state, gate: gate, socketPath: socketPath())
			let window = NSWindow(contentViewController: NSHostingController(rootView: root))
			window.title = "Buddy Agent"
			window.styleMask = [.titled, .closable, .miniaturizable, .resizable]
			window.setContentSize(NSSize(width: 680, height: 470))
			window.center()
			window.isReleasedWhenClosed = false
			self.window = window
		}
		NSApp.activate(ignoringOtherApps: true)
		window?.makeKeyAndOrderFront(nil)
	}
}
