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
