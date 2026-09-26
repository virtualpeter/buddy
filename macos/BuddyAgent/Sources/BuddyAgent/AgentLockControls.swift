import AppKit
import SwiftUI

private func showAgentLockAlert(title: String, message: String) {
	let alert = NSAlert()
	alert.alertStyle = .warning
	alert.messageText = title
	alert.informativeText = message
	alert.runModal()
}

enum AgentLockUI {
	static func failureAlert(for outcome: UnlockOutcome) -> (title: String, message: String) {
		let title = "Buddy Agent is still locked"
		switch outcome {
		case .timedOut:
			return (title, "The unlock prompt went unanswered.")
		case .unavailable:
			return (
				title,
				"This Mac has no Touch ID or login password available to LocalAuthentication."
			)
		case .declined, .unlocked:
			return (title, "The unlock prompt was declined.")
		}
	}

	/// Runs `action` now when the Agent is unlocked (or sealing is not required).
	/// While locked, asks to unlock first and runs `action` only after that succeeds.
	static func perform(
		gate: UnlockGate,
		state: AgentState,
		reason: String = "save Buddy Agent config",
		action: @escaping () -> Void
	) {
		let needsUnlock = AgentStore.configPersistRequiresUnlock && !state.unlocked
		guard needsUnlock else {
			action()
			return
		}
		unlock(gate: gate, reason: reason) { outcome in
			if outcome == .unlocked {
				action()
			} else {
				let alert = failureAlert(for: outcome)
				showAgentLockAlert(title: alert.title, message: alert.message)
			}
		}
	}

	static func unlock(
		gate: UnlockGate,
		reason: String = "unlock the Buddy Agent",
		onComplete: @escaping (UnlockOutcome) -> Void = { _ in }
	) {
		DispatchQueue.global(qos: .userInitiated).async {
			let outcome = gate.authorize(reason: reason)
			DispatchQueue.main.async {
				if outcome == .unlocked {
					AgentWindowRefocus.bringSettingsForward()
				}
				onComplete(outcome)
			}
		}
	}
}

/// Toolbar lock toggle — plain icon, color reflects state (not a primary action button).
struct AgentLockButton: View {
	@ObservedObject var state: AgentState
	let gate: UnlockGate
	@State private var working = false

	private var unavailable: Bool {
		!state.unlocked && UnlockGate.mode == .unavailable
	}

	var body: some View {
		Button(action: toggle) {
			Image(systemName: state.unlocked ? "lock.open.fill" : "lock.fill")
				.symbolRenderingMode(.monochrome)
				.font(.title3)
				.foregroundStyle(state.unlocked ? Color.secondary : Color.orange)
		}
		.buttonStyle(.plain)
		.disabled(working || unavailable)
		.accessibilityLabel(state.unlocked ? "Unlocked — click to lock" : "Locked — click to unlock")
		.help(
			unavailable
				? "Unlock is not available on this Mac."
				: state.unlocked
					? "Minting is allowed. Click to lock."
					: "Minting is blocked. Click to unlock."
		)
	}

	private func toggle() {
		if state.unlocked {
			state.lockVault()
			return
		}
		working = true
		AgentLockUI.unlock(gate: gate) { outcome in
			working = false
			if outcome != .unlocked {
				let alert = AgentLockUI.failureAlert(for: outcome)
				showAgentLockAlert(title: alert.title, message: alert.message)
			}
		}
	}
}
