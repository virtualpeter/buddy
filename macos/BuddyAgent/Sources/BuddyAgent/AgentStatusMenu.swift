import Foundation

extension Notification.Name {
	/// Posted after Settings is on screen. `object` is the auth name to edit.
	static let agentRevealAuth = Notification.Name("BuddyAgentRevealAuth")
	/// Posted after Settings is on screen. `object` is the auth whose cached bearer to show.
	static let agentShowBearer = Notification.Name("BuddyAgentShowBearer")
}

enum AgentRevealAuth {
	static func post(_ name: String) {
		NotificationCenter.default.post(name: .agentRevealAuth, object: name)
	}

	static func postBearer(_ name: String) {
		NotificationCenter.default.post(name: .agentShowBearer, object: name)
	}
}

/// Row text and search for the status panel. The panel draws this; Settings edits it.
enum AgentStatusMenu {
	static func subtitle(auth: AgentAuth, hasSecret: Bool) -> String {
		var parts: [String] = []
		let type = auth.type.trimmingCharacters(in: .whitespacesAndNewlines)
		if !type.isEmpty { parts.append(type) }
		let places = auth.contexts
			.map { $0.trimmingCharacters(in: .whitespacesAndNewlines) }
			.filter { !$0.isEmpty }
		if !places.isEmpty { parts.append(places.joined(separator: ", ")) }
		if !hasSecret { parts.append("secret missing") }
		return parts.joined(separator: " · ")
	}

	/// Case-insensitive match on auth name, type, and linked workplace names.
	static func matches(query: String, name: String, auth: AgentAuth) -> Bool {
		let needle = query.trimmingCharacters(in: .whitespacesAndNewlines)
		if needle.isEmpty { return true }
		let fields = [name, auth.type] + auth.contexts
		return fields.contains { field in
			field.range(of: needle, options: [.caseInsensitive, .diacriticInsensitive]) != nil
		}
	}

	/// Tray highlight for the default context.
	///
	/// A direct link is enough. When the default workplace is an environment,
	/// buddy also auto-picks an instance auth on a Jamf Pro product-tenant child
	/// (`resolveViaJamfProChild`) for Classic and instance calls. Those instance
	/// auths count here too. Platform calls stay on the environment and do not
	/// pull in a child platform auth.
	static func coversDefaultContext(auth: AgentAuth, store: AgentStore) -> Bool {
		let def = store.defaultContext.trimmed
		guard !def.isEmpty else { return false }
		if auth.contexts.contains(def) { return true }
		guard isInstanceAuth(auth.type) else { return false }
		let children = instanceProductTenants(underEnvironment: def, in: store.configTree)
		return auth.contexts.contains { children.contains($0) }
	}

	private static func isInstanceAuth(_ type: String) -> Bool {
		switch type.trimmed.lowercased() {
		case "basic", "oauth": return true
		default: return false
		}
	}

	private static func instanceProductTenants(
		underEnvironment name: String, in roots: [WorkplaceTreeNode]
	) -> Set<String> {
		guard let env = findNode(name, in: roots), env.kind == "environment" else { return [] }
		var out: Set<String> = []
		func walk(_ nodes: [WorkplaceTreeNode]) {
			for node in nodes {
				if (node.kind == "product-tenant" || node.kind == "tenant"), !node.url.trimmed.isEmpty {
					out.insert(node.name)
				}
				walk(node.children)
			}
		}
		walk(env.children)
		return out
	}

	private static func findNode(_ name: String, in nodes: [WorkplaceTreeNode]) -> WorkplaceTreeNode? {
		for node in nodes {
			if node.name == name { return node }
			if let hit = findNode(name, in: node.children) { return hit }
		}
		return nil
	}
}
