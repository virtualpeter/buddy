import Foundation

/// Socket location shared with the Go client (`internal/buddy/agent`).
/// Keep both sides in step: ~/Library/Application Support/Buddy/agent.sock,
/// overridable with BUDDY_AGENT_SOCK.
enum AgentPaths {
	static var supportDirectory: URL {
		let home = FileManager.default.homeDirectoryForCurrentUser
		return home.appendingPathComponent("Library/Application Support/Buddy", isDirectory: true)
	}

	static var socketPath: String {
		if let override = ProcessInfo.processInfo.environment["BUDDY_AGENT_SOCK"], !override.isEmpty {
			return override
		}
		return supportDirectory.appendingPathComponent("agent.sock").path
	}

	/// Config lives next to the socket, so BUDDY_AGENT_SOCK isolates a dev or
	/// soak run from the real ~/Library/Application Support/Buddy store.
	static var configPath: URL {
		let dir = (socketPath as NSString).deletingLastPathComponent
		return URL(fileURLWithPath: dir).appendingPathComponent("agent-config.json")
	}

	/// `~/.config/buddy`, or `$XDG_CONFIG_HOME/buddy` when that is set. Same
	/// directory the CLI uses for config.yaml, the vault, and token caches.
	static var cliConfigDirectory: URL {
		let env = ProcessInfo.processInfo.environment["XDG_CONFIG_HOME"] ?? ""
		if !env.isEmpty {
			return URL(fileURLWithPath: env, isDirectory: true)
				.appendingPathComponent("buddy", isDirectory: true)
		}
		return FileManager.default.homeDirectoryForCurrentUser
			.appendingPathComponent(".config/buddy", isDirectory: true)
	}

	/// Removes the Agent support directory and the CLI config directory.
	/// A socket-path override only drops the Agent files in that directory.
	static func removeInstalledState() {
		let fm = FileManager.default
		removeTree(supportDirectory)
		let configDir = configPath.deletingLastPathComponent().standardizedFileURL
		if configDir != supportDirectory.standardizedFileURL {
			for name in [
				"agent-config.json",
				"agent-config.last-good.json",
				"agent-config.last-good.meta.json",
				"agent.sock",
				"update-check.json",
			] {
				try? fm.removeItem(at: configDir.appendingPathComponent(name))
			}
		}
		removeTree(cliConfigDirectory)
	}

	private static func removeTree(_ url: URL) {
		guard FileManager.default.fileExists(atPath: url.path) else { return }
		do {
			try FileManager.default.removeItem(at: url)
		} catch {
			AgentLog.shared.config("could not remove \(url.path): \(error)")
		}
	}

	/// Creates the parent directory as 0700 so a stray umask cannot widen it.
	static func prepareSocketDirectory() throws {
		let dir = (socketPath as NSString).deletingLastPathComponent
		try FileManager.default.createDirectory(
			atPath: dir,
			withIntermediateDirectories: true,
			attributes: [.posixPermissions: 0o700]
		)
		try? FileManager.default.setAttributes([.posixPermissions: 0o700], ofItemAtPath: dir)
	}
}
