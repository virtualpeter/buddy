import Foundation

enum BundlePaths {
	/// Signed buddy CLI at Contents/Helpers/buddy.
	static func cliHelperURL() -> URL? {
		let helpers = Bundle.main.bundleURL
			.appendingPathComponent("Contents/Helpers/buddy")
		guard FileManager.default.isExecutableFile(atPath: helpers.path) else {
			return nil
		}
		return helpers
	}

	/// Nested Login Items copy when Buddy.app ships with BuddyAgent.
	static func nestedAgentAppURL() -> URL? {
		let agent = Bundle.main.bundleURL
			.appendingPathComponent("Contents/Library/LoginItems/BuddyAgent.app")
		var isDir: ObjCBool = false
		guard FileManager.default.fileExists(atPath: agent.path, isDirectory: &isDir), isDir.boolValue
		else {
			return nil
		}
		return agent
	}

	static func agentSocketPath() -> String {
		let support = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)
			.first?
			.appendingPathComponent("Buddy/agent.sock")
			.path ?? ""
		return support
	}
}
