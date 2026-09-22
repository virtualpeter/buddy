import Foundation

/// Reads Jamf / MDM managed preferences for `virtualpete.buddy.cli` so the Agent
/// UI can lock entries buddy already treats as org-owned. The Agent still serves
/// its own document on `config`; buddy merges managed prefs on top at runtime.
enum AgentManagedPrefs {
	static let domain = "virtualpete.buddy.cli"

	struct Snapshot: Equatable {
		var authNames: Set<String> = []
		var contextNames: Set<String> = []
		var channelNames: Set<String> = []
		var locksFeatures = false
		var locksDefaultContext = false
		/// Explicit `features.check_updates` from managed prefs, when set.
		var checkUpdates: Bool?

		var isEmpty: Bool {
			authNames.isEmpty && contextNames.isEmpty && channelNames.isEmpty
				&& !locksFeatures && !locksDefaultContext
		}

		func summaryLine() -> String {
			var parts: [String] = []
			if !contextNames.isEmpty {
				parts.append("contexts \(contextNames.sorted().joined(separator: ", "))")
			}
			if !authNames.isEmpty {
				parts.append("auths \(authNames.sorted().joined(separator: ", "))")
			}
			if !channelNames.isEmpty {
				parts.append("channels \(channelNames.sorted().joined(separator: ", "))")
			}
			if locksDefaultContext { parts.append("default context") }
			if locksFeatures { parts.append("features") }
			return parts.joined(separator: "; ")
		}
	}

	static func load() -> Snapshot {
		var snap = Snapshot()
		for path in plistPaths() {
			guard let root = loadPlist(path) else { continue }
			let doc = unwrap(root)
			merge(into: &snap, doc: doc)
		}
		return snap
	}

	// MARK: paths

	private static func plistPaths() -> [String] {
		var paths = ["/Library/Managed Preferences/\(domain).plist"]
		let home = FileManager.default.homeDirectoryForCurrentUser.path
		paths.append("\(home)/Library/Managed Preferences/\(domain).plist")
		return paths
	}

	private static func loadPlist(_ path: String) -> [String: Any]? {
		guard FileManager.default.fileExists(atPath: path),
			let data = FileManager.default.contents(atPath: path),
			let root = try? PropertyListSerialization.propertyList(from: data, format: nil)
		else { return nil }
		return root as? [String: Any]
	}

	// MARK: unwrap (mirrors buddy managed_config.go)

	private static func unwrap(_ raw: [String: Any]) -> [String: Any] {
		if hasConfigKeys(raw) { return raw }
		if let mcx = raw["mcx_preference_settings"] as? [String: Any] {
			return unwrap(mcx)
		}
		for key in ["Forced", "forced"] {
			guard let arr = raw[key] as? [Any], let first = arr.first as? [String: Any] else {
				continue
			}
			return unwrap(first)
		}
		return raw
	}

	private static func hasConfigKeys(_ raw: [String: Any]) -> Bool {
		for key in ["tenant", "tenants", "context", "contexts", "orgs", "auths", "channels", "features"] {
			if raw[key] != nil { return true }
		}
		return false
	}

	private static func merge(into snap: inout Snapshot, doc: [String: Any]) {
		if let auths = doc["auths"] as? [String: Any] {
			for name in auths.keys { snap.authNames.insert(name) }
		}
		for key in ["contexts", "tenants"] {
			if let contexts = doc[key] as? [String: Any] {
				for name in contexts.keys { snap.contextNames.insert(name) }
			}
		}
		if let orgs = doc["orgs"] as? [String: Any] {
			for name in orgs.keys { snap.contextNames.insert(name) }
		}
		if let channels = doc["channels"] as? [String: Any] {
			for name in channels.keys { snap.channelNames.insert(name) }
		}
		if let feat = doc["features"] as? [String: Any] {
			snap.locksFeatures = true
			if let v = feat["check_updates"] as? Bool {
				snap.checkUpdates = v
			}
		} else if doc["features"] != nil {
			snap.locksFeatures = true
		}
		if let ctx = doc["context"] as? [String: Any], ctx["default"] != nil {
			snap.locksDefaultContext = true
		}
		if let tenant = doc["tenant"] as? [String: Any], tenant["default"] != nil {
			snap.locksDefaultContext = true
		}
	}
}
