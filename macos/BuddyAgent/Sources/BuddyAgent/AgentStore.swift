import Foundation
import LocalAuthentication

/// One integration the Agent owns: a buddy workplace (context) plus the auth
/// that covers it. Secrets live in the Keychain, never in this JSON.
struct Integration: Codable, Identifiable, Equatable {
	var id: UUID = UUID()
	/// Workplace name passed to `buddy --context`.
	var context: String = ""
	/// Jamf Pro instance URL. Empty for an organization-only workplace.
	var url: String = ""
	/// Optional Jamf tree metadata (Platform API Gateway).
	var region: String = ""
	var environmentID: String = ""
	var tenantID: String = ""

	var authName: String = ""
	/// oauth | basic | platform
	var authType: String = "oauth"
	var clientID: String = ""
	var username: String = ""
	/// platform only: organization | environment | tenant
	var scope: String = ""
	/// readonly | full (empty = full)
	var access: String = ""

	var secretKind: AgentKeychain.Kind {
		authType == "basic" ? .password : .clientSecret
	}

	var hasSecret: Bool {
		guard !authName.isEmpty else { return false }
		return AgentKeychain.has(auth: authName, kind: secretKind)
	}

	/// Human-readable reason this row is not usable yet, or nil.
	var problem: String? {
		if context.trimmed.isEmpty { return "context name is required" }
		if authName.trimmed.isEmpty { return "auth name is required" }
		if url.trimmed.isEmpty && environmentID.trimmed.isEmpty {
			return "needs a Jamf Pro URL or an environment id"
		}
		switch authType {
		case "basic" where username.trimmed.isEmpty:
			return "username is required"
		case "oauth", "platform":
			if clientID.trimmed.isEmpty { return "client id is required" }
		default:
			break
		}
		if !hasSecret { return "secret not stored yet" }
		return nil
	}
}

extension String {
	var trimmed: String { trimmingCharacters(in: .whitespacesAndNewlines) }
}

/// The Agent's own config: integrations plus the default context. Persisted as
/// JSON next to the socket; served to the CLI as a config.yaml-shaped document.
final class AgentStore: ObservableObject {
	@Published private(set) var integrations: [Integration] = []
	@Published var defaultContext: String = ""
	@Published private(set) var lastError: String?

	private let queue = DispatchQueue(label: "buddy.agent.store")

	init() {
		load()
	}

	var path: URL { AgentPaths.configPath }

	// MARK: persistence

	private struct Document: Codable {
		var version: Int = 1
		var defaultContext: String = ""
		var integrations: [Integration] = []
	}

	func load() {
		guard let data = try? Data(contentsOf: path) else { return }
		guard let doc = try? JSONDecoder().decode(Document.self, from: data) else {
			setError("could not read \(path.lastPathComponent)")
			return
		}
		integrations = doc.integrations
		defaultContext = doc.defaultContext
	}

	func save() {
		let doc = Document(defaultContext: defaultContext, integrations: integrations)
		do {
			try AgentPaths.prepareSocketDirectory()
			let encoder = JSONEncoder()
			encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
			let data = try encoder.encode(doc)
			try data.write(to: path, options: [.atomic])
			try? FileManager.default.setAttributes(
				[.posixPermissions: 0o600], ofItemAtPath: path.path)
			setError(nil)
		} catch {
			setError("could not write \(path.lastPathComponent): \(error)")
		}
	}

	private func setError(_ message: String?) {
		if Thread.isMainThread {
			lastError = message
		} else {
			DispatchQueue.main.async { self.lastError = message }
		}
	}

	// MARK: mutation (GUI)

	func upsert(_ integration: Integration) {
		if let idx = integrations.firstIndex(where: { $0.id == integration.id }) {
			let old = integrations[idx]
			if old.authName != integration.authName {
				AgentKeychain.deleteAll(auth: old.authName)
			}
			integrations[idx] = integration
		} else {
			integrations.append(integration)
		}
		if defaultContext.trimmed.isEmpty {
			defaultContext = integration.context
		}
		save()
	}

	func remove(_ integration: Integration) {
		AgentKeychain.deleteAll(auth: integration.authName)
		integrations.removeAll { $0.id == integration.id }
		if defaultContext == integration.context,
			let first = integrations.first
		{
			defaultContext = first.context
		}
		save()
	}

	func setSecret(_ value: String, for integration: Integration, context: LAContext? = nil) {
		AgentKeychain.set(
			value, auth: integration.authName, kind: integration.secretKind, context: context)
		objectWillChange.send()
	}

	func integration(forAuth auth: String) -> Integration? {
		integrations.first { $0.authName == auth }
	}

	/// Merges the secrets-free JSON written by `buddy setup export --json`.
	/// Existing rows with the same auth name are updated in place; secrets are
	/// never part of that document and must still be typed into the Agent.
	@discardableResult
	func importExport(_ data: Data) throws -> Int {
		guard let root = try JSONSerialization.jsonObject(with: data) as? [String: Any] else {
			throw AgentStoreError.badImport("top level is not an object")
		}
		return try importDocument(root)
	}

	/// Result of importing a cred backup. `secrets` is how many Keychain writes
	/// `has()` can see afterwards — not how many the file contained.
	struct BackupImportResult {
		var integrations: Int
		var secrets: Int
		var ephemeral: Int
		var failed: Int
		var inBackup: Int
	}

	/// Identity half of a cred-backup import (no Keychain writes). Used so the
	/// GUI can merge workplaces on the main queue and write secrets off it.
	@discardableResult
	func importDocumentForBackup(_ root: [String: Any]) throws -> Int {
		try importDocument(root)
	}

	/// Imports the encrypted file `buddy cred backup` writes, which carries the
	/// secrets as well as the identity. The passphrase is the one typed when
	/// the backup was made; it is used once here and never stored.
	///
	/// `presence` is optional and only used when replacing an existing
	/// presence-gated Keychain item (delete/update). New items are created
	/// without it — SecItemAdd attaches the ACL itself.
	@discardableResult
	func importBackup(
		_ data: Data, passphrase: String, presence: LAContext? = nil
	) throws -> BackupImportResult {
		let contents = try AgentBackup.read(data, passphrase: passphrase)
		let added = try importDocument(contents.document)
		var stored = 0
		var ephemeral = 0
		var failed = 0
		for (authName, secret) in contents.secrets {
			guard let row = integration(forAuth: authName) else {
				failed += 1
				continue
			}
			let ok = AgentKeychain.set(
				secret, auth: authName, kind: row.secretKind, context: presence)
			if AgentKeychain.has(auth: authName, kind: row.secretKind) {
				if ok {
					stored += 1
				} else {
					ephemeral += 1
				}
			} else {
				failed += 1
			}
		}
		objectWillChange.send()
		return BackupImportResult(
			integrations: added,
			secrets: stored,
			ephemeral: ephemeral,
			failed: failed,
			inBackup: contents.secrets.count)
	}

	@discardableResult
	private func importDocument(_ root: [String: Any]) throws -> Int {
		var contexts =
			(root["contexts"] as? [String: Any]) ?? (root["tenants"] as? [String: Any]) ?? [:]
		for (name, entry) in workplacesFromOrgs(root["orgs"] as? [String: Any] ?? [:]) {
			if contexts[name] == nil { contexts[name] = entry }
		}
		let auths = (root["auths"] as? [String: Any]) ?? [:]
		if contexts.isEmpty && auths.isEmpty {
			throw AgentStoreError.badImport("no contexts or auths found")
		}
		var added = 0
		for (authName, raw) in auths {
			guard let entry = raw as? [String: Any] else { continue }
			let linked = (entry["contexts"] as? [String]) ?? (entry["tenants"] as? [String]) ?? []
			let contextName = linked.first ?? (contexts.keys.sorted().first ?? "")
			let ctx = contexts[contextName] as? [String: Any] ?? [:]
			var row = integrations.first { $0.authName == authName } ?? Integration()
			row.authName = authName
			row.context = contextName
			row.authType = (entry["type"] as? String) ?? "oauth"
			row.clientID = (entry["clientId"] as? String) ?? ""
			row.username = (entry["username"] as? String) ?? ""
			row.scope = (entry["scope"] as? String) ?? ""
			row.access = (entry["access"] as? String) ?? ""
			row.url = (ctx["url"] as? String) ?? row.url
			row.region = (ctx["region"] as? String) ?? row.region
			row.environmentID =
				(ctx["environmentId"] as? String) ?? (ctx["id"] as? String) ?? row.environmentID
			row.tenantID = (ctx["tenantId"] as? String) ?? row.tenantID
			if let idx = integrations.firstIndex(where: { $0.authName == authName }) {
				integrations[idx] = row
			} else {
				integrations.append(row)
				added += 1
			}
		}
		// The export is identity only and often carries no auths, so seed a row
		// per workplace; the user names the auth and types its secret here.
		for (contextName, raw) in contexts {
			guard !integrations.contains(where: { $0.context == contextName }) else { continue }
			let ctx = raw as? [String: Any] ?? [:]
			// An organization node carries no URL and no environment id, so it
			// cannot be minted against on its own: skip it rather than seed a
			// row that can only ever show "needs a Jamf Pro URL".
			let identity = [ctx["url"], ctx["environmentId"], ctx["id"]].compactMap { $0 as? String }
			guard identity.contains(where: { !$0.trimmed.isEmpty }) else { continue }
			var row = Integration()
			row.context = contextName
			row.url = (ctx["url"] as? String) ?? ""
			row.region = (ctx["region"] as? String) ?? ""
			row.environmentID = (ctx["environmentId"] as? String) ?? (ctx["id"] as? String) ?? ""
			row.tenantID = (ctx["tenantId"] as? String) ?? ""
			integrations.append(row)
			added += 1
		}
		if let ctxSel = root["context"] as? [String: Any],
			let def = ctxSel["default"] as? String, !def.isEmpty
		{
			defaultContext = def
		} else if defaultContext.trimmed.isEmpty {
			defaultContext = integrations.first?.context ?? ""
		}
		save()
		return added
	}

	/// Flattens buddy's orgs tree (organization → environments → product
	/// tenants) into workplace entries. `buddy setup export` writes that shape.
	private func workplacesFromOrgs(_ orgs: [String: Any]) -> [String: [String: Any]] {
		var out: [String: [String: Any]] = [:]
		for (orgName, rawOrg) in orgs {
			guard let org = rawOrg as? [String: Any] else { continue }
			var orgEntry: [String: Any] = [:]
			if let id = org["id"] as? String { orgEntry["environmentId"] = id }
			out[orgName] = orgEntry

			let environments = org["environments"] as? [String: Any] ?? [:]
			for (envName, rawEnv) in environments {
				guard let env = rawEnv as? [String: Any] else { continue }
				var entry: [String: Any] = [:]
				if let url = env["url"] as? String { entry["url"] = url }
				if let region = env["region"] as? String { entry["region"] = region }
				if let id = (env["id"] as? String) ?? (env["environmentId"] as? String) {
					entry["environmentId"] = id
				}
				out[envName] = entry

				let tenants = env["tenants"] as? [String: Any] ?? [:]
				for (tenantName, rawTenant) in tenants {
					guard let tenant = rawTenant as? [String: Any] else { continue }
					let jamfPro = tenant["jamfpro"] as? [String: Any] ?? [:]
					var row: [String: Any] = [:]
					if let url = (tenant["url"] as? String) ?? (jamfPro["url"] as? String) {
						row["url"] = url
					}
					if let tid = (tenant["tenantId"] as? String) ?? (jamfPro["tenantId"] as? String) {
						row["tenantId"] = tid
					}
					out[tenantName] = row
				}
			}
		}
		return out
	}

	// MARK: wire

	/// configDocument is the config.yaml-shaped document served to the CLI.
	/// Flat `contexts:` is fine: buddy folds it into the orgs tree on load.
	/// Never includes secrets.
	func configDocument() -> [String: Any] {
		var contexts: [String: Any] = [:]
		var auths: [String: Any] = [:]
		for row in integrations {
			guard !row.context.trimmed.isEmpty else { continue }
			var ctx = contexts[row.context] as? [String: Any] ?? [:]
			if !row.url.trimmed.isEmpty { ctx["url"] = row.url.trimmed }
			if !row.region.trimmed.isEmpty { ctx["region"] = row.region.trimmed }
			if !row.environmentID.trimmed.isEmpty { ctx["environmentId"] = row.environmentID.trimmed }
			if !row.tenantID.trimmed.isEmpty { ctx["tenantId"] = row.tenantID.trimmed }
			contexts[row.context] = ctx

			guard !row.authName.trimmed.isEmpty else { continue }
			var auth: [String: Any] = [
				"type": row.authType,
				"contexts": [row.context],
			]
			if !row.clientID.trimmed.isEmpty { auth["clientId"] = row.clientID.trimmed }
			if !row.username.trimmed.isEmpty { auth["username"] = row.username.trimmed }
			if !row.scope.trimmed.isEmpty { auth["scope"] = row.scope.trimmed }
			if !row.access.trimmed.isEmpty { auth["access"] = row.access.trimmed }
			auths[row.authName] = auth
		}
		var doc: [String: Any] = [:]
		if !contexts.isEmpty { doc["contexts"] = contexts }
		if !auths.isEmpty { doc["auths"] = auths }
		let def = defaultContext.trimmed.isEmpty ? (integrations.first?.context ?? "") : defaultContext
		if !def.trimmed.isEmpty { doc["context"] = ["default": def.trimmed] }
		return doc
	}

	/// integrationSummaries is the GUI/CLI listing: identity plus whether a
	/// secret is stored. Never the secret itself.
	func integrationSummaries() -> [[String: Any]] {
		integrations.map { row in
			var out: [String: Any] = [
				"context": row.context,
				"auth": row.authName,
				"type": row.authType,
				"secret": row.hasSecret ? "stored" : "missing",
			]
			if !row.url.trimmed.isEmpty { out["url"] = row.url }
			if !row.clientID.trimmed.isEmpty { out["client_id"] = row.clientID }
			if !row.username.trimmed.isEmpty { out["username"] = row.username }
			if let problem = row.problem { out["problem"] = problem }
			return out
		}
	}
}

enum AgentStoreError: Error, CustomStringConvertible {
	case badImport(String)

	var description: String {
		switch self {
		case .badImport(let why): return "import failed: \(why)"
		}
	}
}
