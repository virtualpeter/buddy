import AppKit
import SwiftUI

private let agentStatusPanelWidth: CGFloat = 240

struct AgentStatusMenuSnapshot {
	let summaryTitle: String
	let lockTitle: String
	let openBuddyEnabled: Bool
	let loginEnabled: Bool
	let loginChecked: Bool
}

/// Status menu as a non-activating panel — avoids AppKit's `NSMenu` glass window,
/// which triggers a spurious `makeKeyWindow` / `canBecomeKeyWindow` warning on macOS 26+.
final class AgentStatusPanelController: NSObject {
	private var panel: NSPanel?
	private var outsideClickMonitor: Any?
	private weak var delegate: AgentAppDelegate?

	func toggle(relativeTo button: NSStatusBarButton, snapshot: AgentStatusMenuSnapshot, delegate: AgentAppDelegate) {
		self.delegate = delegate
		if panel?.isVisible == true {
			dismiss()
			return
		}
		show(relativeTo: button, snapshot: snapshot)
	}

	func dismiss() {
		guard panel?.isVisible == true else { return }
		removeOutsideClickMonitor()
		panel?.orderOut(nil)
		delegate?.statusPanelDidClose()
	}

	private func show(relativeTo button: NSStatusBarButton, snapshot: AgentStatusMenuSnapshot) {
		let content = AgentStatusMenuContent(snapshot: snapshot, controller: self)
		let hosting = NSHostingController(rootView: content)
		// Measure before attaching to the panel (avoids layout recursion on show).
		let fit = hosting.sizeThatFits(in: NSSize(width: agentStatusPanelWidth, height: 800))
		let size = NSSize(width: agentStatusPanelWidth, height: max(120, ceil(fit.height)))

		let panel: NSPanel
		if let existing = self.panel {
			panel = existing
		} else {
			panel = NSPanel(
				contentRect: NSRect(origin: .zero, size: size),
				styleMask: [.borderless, .nonactivatingPanel],
				backing: .buffered,
				defer: false)
			panel.isFloatingPanel = true
			panel.level = .popUpMenu
			panel.hasShadow = true
			panel.isOpaque = false
			panel.backgroundColor = .clear
			panel.becomesKeyOnlyIfNeeded = false
			panel.hidesOnDeactivate = false
			self.panel = panel
		}

		// Size before attaching the hosting controller to avoid reentrant AppKit layout.
		panel.setContentSize(size)
		panel.contentViewController = hosting
		position(panel, below: button, size: size)
		delegate?.statusPanelWillOpen()
		panel.orderFront(nil)
		installOutsideClickMonitor(for: panel)
	}

	private func position(_ panel: NSPanel, below button: NSStatusBarButton, size: NSSize) {
		guard let window = button.window else { return }
		let buttonOnScreen = window.convertToScreen(button.convert(button.bounds, to: nil))
		let origin = NSPoint(
			x: buttonOnScreen.midX - size.width / 2,
			y: buttonOnScreen.minY - size.height - 4)
		panel.setFrameOrigin(origin)
	}

	private func installOutsideClickMonitor(for panel: NSPanel) {
		removeOutsideClickMonitor()
		outsideClickMonitor = NSEvent.addGlobalMonitorForEvents(matching: [.leftMouseDown, .rightMouseDown]) {
			[weak self] _ in
			guard let self, panel.isVisible else { return }
			let click = NSEvent.mouseLocation
			if !panel.frame.contains(click) {
				DispatchQueue.main.async { self.dismiss() }
			}
		}
	}

	private func removeOutsideClickMonitor() {
		if let outsideClickMonitor {
			NSEvent.removeMonitor(outsideClickMonitor)
			self.outsideClickMonitor = nil
		}
	}

	fileprivate func perform(_ action: @escaping (AgentAppDelegate) -> Void) {
		dismiss()
		guard let delegate else { return }
		DispatchQueue.main.async { action(delegate) }
	}

	/// Tray row that dismisses via `presentSettingsWindow` / pending flags (Settings, Help, About).
	fileprivate func runMenuAction(_ action: @escaping (AgentAppDelegate) -> Void) {
		guard let delegate else { return }
		action(delegate)
	}

	fileprivate func toggleLockFromPanel(_ delegate: AgentAppDelegate) {
		DispatchQueue.main.async { delegate.toggleLockFromMenu() }
	}
}

private struct AgentStatusMenuContent: View {
	let snapshot: AgentStatusMenuSnapshot
	let controller: AgentStatusPanelController

	var body: some View {
		VStack(alignment: .leading, spacing: 0) {
			Text(snapshot.summaryTitle)
				.font(.callout)
				.foregroundStyle(.secondary)
				.padding(.horizontal, 14)
				.padding(.vertical, 8)
				.frame(maxWidth: .infinity, alignment: .leading)
			Divider()
			menuRow(snapshot.lockTitle) { controller.toggleLockFromPanel($0) }
			Divider()
			menuRow("Settings…", dismissFirst: false) { $0.openSettings() }
			menuRow("Open Buddy…", enabled: snapshot.openBuddyEnabled) { $0.openBuddyApp() }
			menuRow(
				loginTitle,
				enabled: snapshot.loginEnabled
			) { $0.toggleOpenAtLoginFromPanel() }
			Divider()
			menuRow("Help", dismissFirst: false) { $0.showHelp() }
			menuRow("About Buddy Agent…", dismissFirst: false) { $0.showAbout() }
			Divider()
			menuRow("Quit Buddy Agent") { $0.quit() }
		}
		.frame(width: agentStatusPanelWidth, alignment: .topLeading)
		.background(.regularMaterial, in: RoundedRectangle(cornerRadius: 8))
	}

	private var loginTitle: String {
		snapshot.loginChecked ? "✓ Open at Login" : "Open at Login"
	}

	private func menuRow(
		_ title: String,
		enabled: Bool = true,
		dismissFirst: Bool = true,
		action: @escaping (AgentAppDelegate) -> Void
	) -> some View {
		Button {
			if dismissFirst {
				controller.perform(action)
			} else {
				controller.runMenuAction(action)
			}
		} label: {
			Text(title)
				.frame(maxWidth: .infinity, alignment: .leading)
				.padding(.horizontal, 14)
				.padding(.vertical, 6)
		}
		.buttonStyle(.plain)
		.disabled(!enabled)
	}
}
