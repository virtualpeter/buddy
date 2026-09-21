import AppKit

/// Labels and actions for the status-item panel (see `AgentStatusPanel`).
enum AgentStatusMenu {
	static func snapshot(unlocked: Bool, authCount: Int) -> AgentStatusMenuSnapshot {
		let lockWord = unlocked ? "Unlocked" : "Locked"
		let plural = authCount == 1 ? "" : "s"
		return AgentStatusMenuSnapshot(
			summaryTitle: "Buddy Agent — \(lockWord) (\(authCount) auth\(plural))",
			lockTitle: unlocked ? "Lock Agent" : "Unlock Agent",
			openBuddyEnabled: AgentBranding.buddyAppURL() != nil,
			loginEnabled: AgentLoginItem.isAvailable,
			loginChecked: AgentLoginItem.isEnabled)
	}
}
