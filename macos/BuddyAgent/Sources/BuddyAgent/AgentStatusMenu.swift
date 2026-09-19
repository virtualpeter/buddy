import AppKit

/// Standard `NSMenu` for the status item (all macOS versions; system-drawn on macOS 27+).
enum AgentStatusMenu {
	static func makeMenu(delegate: AgentAppDelegate) -> NSMenu {
		let menu = NSMenu()
		menu.delegate = delegate

		let summary = NSMenuItem(title: "Buddy Agent", action: nil, keyEquivalent: "")
		summary.isEnabled = false
		menu.addItem(summary)

		let lock = NSMenuItem(
			title: "Unlock Agent", action: #selector(AgentAppDelegate.toggleLockFromMenu),
			keyEquivalent: "l")
		lock.target = delegate
		menu.addItem(lock)

		menu.addItem(.separator())
		menu.addItem(
			withTitle: "Settings…", action: #selector(AgentAppDelegate.openSettings), keyEquivalent: ",")
			.target = delegate
		let openBuddy = NSMenuItem(
			title: "Open Buddy…", action: #selector(AgentAppDelegate.openBuddyApp), keyEquivalent: "")
		openBuddy.target = delegate
		menu.addItem(openBuddy)
		let login = NSMenuItem(
			title: "Open at Login", action: #selector(AgentAppDelegate.toggleOpenAtLogin(_:)),
			keyEquivalent: "")
		login.target = delegate
		menu.addItem(login)

		menu.addItem(.separator())
		let help = NSMenuItem(
			title: "Help", action: #selector(AgentAppDelegate.showHelp), keyEquivalent: "?")
		help.target = delegate
		help.keyEquivalentModifierMask = .command
		menu.addItem(help)
		menu.addItem(
			withTitle: "About Buddy Agent…", action: #selector(AgentAppDelegate.showAbout),
			keyEquivalent: "")
			.target = delegate
		menu.addItem(.separator())
		menu.addItem(
			withTitle: "Quit Buddy Agent", action: #selector(AgentAppDelegate.quit), keyEquivalent: "q")
			.target = delegate
		return menu
	}
}
