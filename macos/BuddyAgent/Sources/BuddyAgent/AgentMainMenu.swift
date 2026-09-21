import AppKit

/// Accessory apps have no menu bar by default; Paste/Cut/Copy only reach text
/// fields when the main menu defines the standard Edit key equivalents.
enum AgentMainMenu {
	static func install() {
		guard NSApp.mainMenu == nil else { return }
		let mainMenu = NSMenu()

		let appMenuItem = NSMenuItem()
		mainMenu.addItem(appMenuItem)
		let appMenu = NSMenu()
		appMenuItem.submenu = appMenu

		let appName = "Buddy Agent"
		appMenu.addItem(
			withTitle: "About \(appName)",
			action: nil,
			keyEquivalent: "")
		appMenu.addItem(.separator())
		appMenu.addItem(
			withTitle: "Hide \(appName)",
			action: #selector(NSApplication.hide(_:)),
			keyEquivalent: "h")
		let hideOthers = appMenu.addItem(
			withTitle: "Hide Others",
			action: #selector(NSApplication.hideOtherApplications(_:)),
			keyEquivalent: "h")
		hideOthers.keyEquivalentModifierMask = [.command, .option]
		appMenu.addItem(
			withTitle: "Show All",
			action: #selector(NSApplication.unhideAllApplications(_:)),
			keyEquivalent: "")
		appMenu.addItem(.separator())
		// About and Quit are wired in AgentAppDelegate (Quit distinguishes tray vs ⌘Q).
		appMenu.addItem(
			withTitle: "Quit \(appName)",
			action: nil,
			keyEquivalent: "q")

		let editMenuItem = NSMenuItem(title: "Edit", action: nil, keyEquivalent: "")
		mainMenu.addItem(editMenuItem)
		let editMenu = NSMenu(title: "Edit")
		editMenuItem.submenu = editMenu

		editMenu.addItem(
			withTitle: "Undo",
			action: Selector(("undo:")),
			keyEquivalent: "z")
		editMenu.addItem(
			withTitle: "Redo",
			action: Selector(("redo:")),
			keyEquivalent: "Z")
		editMenu.addItem(.separator())
		editMenu.addItem(
			withTitle: "Cut",
			action: #selector(NSText.cut(_:)),
			keyEquivalent: "x")
		editMenu.addItem(
			withTitle: "Copy",
			action: #selector(NSText.copy(_:)),
			keyEquivalent: "c")
		editMenu.addItem(
			withTitle: "Paste",
			action: #selector(NSText.paste(_:)),
			keyEquivalent: "v")
		editMenu.addItem(
			withTitle: "Select All",
			action: #selector(NSText.selectAll(_:)),
			keyEquivalent: "a")

		NSApp.mainMenu = mainMenu
	}

	static func wire(to target: AnyObject, about: Selector, quit: Selector) {
		guard let appMenu = NSApp.mainMenu?.item(at: 0)?.submenu else { return }
		for item in appMenu.items {
			if item.title.hasPrefix("About") {
				item.target = target
				item.action = about
			} else if item.keyEquivalent == "q", !item.isSeparatorItem {
				item.target = target
				item.action = quit
			}
		}
	}
}
