import AppKit
import ServiceManagement

/// Open-at-login for Buddy Agent via SMAppService (macOS 13+).
///
/// Apple's guidance: use `SMAppService.mainApp` when this process *is* the app
/// that should launch at login. That covers both a standalone BuddyAgent.app
/// and the nested LoginItems copy — Bundle.main is always BuddyAgent.
/// (`loginItem(identifier:)` is for a *host* app registering a helper in its
/// own Contents/Library/LoginItems; the Agent is the helper, so mainApp is
/// the correct call site.)
enum AgentLoginItem {
	static var service: SMAppService { .mainApp }

	static var isEnabled: Bool {
		service.status == .enabled
	}

	/// .requiresApproval means the user must flip the switch in System Settings.
	static var needsApproval: Bool {
		service.status == .requiresApproval
	}

	static var isAvailable: Bool {
		switch service.status {
		case .notFound: return false
		default: return true
		}
	}

	@discardableResult
	static func setEnabled(_ on: Bool) -> String? {
		do {
			if on {
				if service.status == .enabled { return nil }
				try service.register()
			} else if service.status == .enabled || service.status == .requiresApproval {
				try service.unregister()
			}
			if service.status == .requiresApproval {
				SMAppService.openSystemSettingsLoginItems()
				return "Turn on Buddy Agent in System Settings → General → Login Items."
			}
			return nil
		} catch {
			if service.status == .requiresApproval {
				SMAppService.openSystemSettingsLoginItems()
				return "Turn on Buddy Agent in System Settings → General → Login Items."
			}
			return error.localizedDescription
		}
	}
}

enum AgentBranding {
	static var marketingVersion: String {
		Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String
			?? agentVersion
	}

	static var buildNumber: String {
		Bundle.main.object(forInfoDictionaryKey: "CFBundleVersion") as? String ?? "1"
	}

	static var copyright: String {
		Bundle.main.object(forInfoDictionaryKey: "NSHumanReadableCopyright") as? String
			?? "Copyright © 2026 Peter Viertel. MIT License."
	}

	/// Buddy.app next to a nested Agent, or the usual Applications locations.
	static func buddyAppURL() -> URL? {
		let agent = Bundle.main.bundleURL
		// …/Buddy.app/Contents/Library/LoginItems/BuddyAgent.app
		let nested = agent
			.deletingLastPathComponent()
			.deletingLastPathComponent()
			.deletingLastPathComponent()
			.deletingLastPathComponent()
		if nested.lastPathComponent == "Buddy.app",
			FileManager.default.isExecutableFile(atPath: nested.appendingPathComponent("Contents/MacOS/Buddy").path)
		{
			return nested
		}
		let home = FileManager.default.homeDirectoryForCurrentUser
		for candidate in [
			URL(fileURLWithPath: "/Applications/Buddy.app"),
			home.appendingPathComponent("Applications/Buddy.app"),
		] {
			if FileManager.default.isExecutableFile(
				atPath: candidate.appendingPathComponent("Contents/MacOS/Buddy").path)
			{
				return candidate
			}
		}
		return nil
	}

	static func menuIcon(size: CGFloat = 16) -> NSImage? {
		guard let base = NSImage(named: "AppIcon") ?? bundleAppIcon() else { return nil }
		let image = NSImage(size: NSSize(width: size, height: size))
		image.lockFocus()
		base.draw(
			in: NSRect(x: 0, y: 0, width: size, height: size),
			from: .zero,
			operation: .copy,
			fraction: 1)
		image.unlockFocus()
		image.isTemplate = false
		return image
	}

	private static func bundleAppIcon() -> NSImage? {
		guard let url = Bundle.main.url(forResource: "AppIcon", withExtension: "icns") else {
			return nil
		}
		return NSImage(contentsOf: url)
	}
}
