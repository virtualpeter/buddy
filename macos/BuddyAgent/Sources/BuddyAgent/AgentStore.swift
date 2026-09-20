import Foundation
import LocalAuthentication

extension String {
	var trimmed: String { trimmingCharacters(in: .whitespacesAndNewlines) }
}

/// The Agent's own config: workplaces, org tree, and auths. Persisted as JSON
/// next to the socket; served to the CLI as a config.yaml-shaped document.
final class AgentStore: ObservableObject {
	@Published private(set) var workplaces: [String: Workplace] = [:]
	@Published private(set) var orgs: [String: OrgRoot] = [:]
	@Published private(set) var auths: [String: AgentAuth] = [:]
	@Published private(set) var features: AgentFeatures = AgentFeatures()
	@Published private(set) var channels: [String: ChannelPolicy] = [:]
	@Published private(set) var managedPrefs: AgentManagedPrefs.Snapshot = AgentManagedPrefs.Snapshot()
	@Published var defaultContext: String = ""
	@Published private(set) var lastError: String?
	private var loadSucceeded = false
	/// Bumped when Keychain secret presence may have changed (import, save auth, etc.).
	@Published private(set) var secretStatusGeneration: Int = 0

	private let queue = DispatchQueue(label: "buddy.agent.store")

	init() {
		reloadManagedPrefs()
		load()
	}

	func reloadManagedPrefs() {
		let snap = AgentManagedPrefs.load()
		if Thread.isMainThread {
			managedPrefs = snap
		} else {
			DispatchQueue.main.async { self.managedPrefs = snap }
		}
	}

	func configIntegrity(unlocked: Bool, presence: LAContext?) -> AgentConfigIntegrity.State {
		AgentConfigIntegrity.state(configPath: path, unlocked: unlocked, presence: presence)
	}

	func isManagedWorkplace(_ name: String) -> Bool {
		managedPrefs.contextNames.contains(name)
	}

	func isManagedAuth(_ name: String) -> Bool {
		managedPrefs.authNames.contains(name)
	}

	func isManagedChannel(_ channel: BuddyChannel) -> Bool {
		managedPrefs.channelNames.contains(channel.rawValue)
	}

	func managedPolicyLocked(_ feature: String) -> Bool {
		switch feature {
		case "features": return managedPrefs.locksFeatures
		case "defaultContext": return managedPrefs.locksDefaultContext
		default: return false
		}
	}

	var path: URL { AgentPaths.configPath }

	var configTree: [WorkplaceTreeNode] {
		AgentConfigTree.build(workplaces: workplaces, orgs: orgs)
	}

	var sortedAuthNames: [String] {
		auths.keys.sorted()
	}

	var isEmpty: Bool {
		workplaces.isEmpty && auths.isEmpty
	}

	/// Count of configured auths (for status / menu bar).
	var authCount: Int { auths.count }

	func hasSecret(forAuth name: String) -> Bool {
		_ = secretStatusGeneration
		return AgentKeychain.hasAny(auth: name)
	}

	func markSecretsChanged() {
		let bump = { [weak self] in
			guard let self else { return }
			secretStatusGeneration &+= 1
			objectWillChange.send()
		}
		if Thread.isMainThread {
			bump()
		} else {
			DispatchQueue.main.async(execute: bump)
		}
	}

	// MARK: persistence

	private struct Document: Codable {
		var version: Int = 3
		var defaultContext: String = ""
		var orgs: [String: OrgRoot] = [:]
		var workplaces: [String: Workplace] = [:]
		var auths: [String: AgentAuth] = [:]
		var features: AgentFeatures = AgentFeatures()
		var channels: [String: ChannelPolicy] = [:]
		var integrations: [Integration]?

		init(
			version: Int = 3,
			defaultContext: String = "",
			orgs: [String: OrgRoot] = [:],
			workplaces: [String: Workplace] = [:],
			auths: [String: AgentAuth] = [:],
			features: AgentFeatures = AgentFeatures(),
			channels: [String: ChannelPolicy] = [:],
			integrations: [Integration]? = nil
		) {
			self.version = version
			self.defaultContext = defaultContext
			self.orgs = orgs
			self.workplaces = workplaces
			self.auths = auths
			self.features = features
			self.channels = channels
			self.integrations = integrations
		}

		init(from decoder: Decoder) throws {
			let c = try decoder.container(keyedBy: CodingKeys.self)
			version = try c.decodeIfPresent(Int.self, forKey: .version) ?? 3
			defaultContext = try c.decodeIfPresent(String.self, forKey: .defaultContext) ?? ""
			orgs = try c.decodeIfPresent([String: OrgRoot].self, forKey: .orgs) ?? [:]
			workplaces = try c.decodeIfPresent([String: Workplace].self, forKey: .workplaces) ?? [:]
			auths = try c.decodeIfPresent([String: AgentAuth].self, forKey: .auths) ?? [:]
			features = try c.decodeIfPresent(AgentFeatures.self, forKey: .features) ?? AgentFeatures()
			channels = try c.decodeIfPresent([String: ChannelPolicy].self, forKey: .channels) ?? [:]
			integrations = try c.decodeIfPresent([Integration].self, forKey: .integrations)
		}
	}

	func load() {
		loadSucceeded = false
		guard let data = try? Data(contentsOf: path) else {
			loadSucceeded = true
			if isEmpty { AgentIdleLock.applyEmptyConfigDefault() }
			return
		}
		do {
			let doc = try JSONDecoder().decode(Document.self, from: data)
			let needsSave = applyDocument(doc)
			loadSucceeded = true
			reloadManagedPrefs()
			if needsSave {
				save()
			} else {
				ensureConfigSealed()
			}
		} catch {
			setError("could not read \(path.lastPathComponent): \(Self.decodeErrorMessage(error))")
		}
		if isEmpty { AgentIdleLock.applyEmptyConfigDefault() }
	}

	@discardableResult
	private func applyDocument(_ doc: Document) -> Bool {
		defaultContext = doc.defaultContext
		if let items = doc.integrations, !items.isEmpty {
			applyV1Integrations(items)
			return true
		}
		workplaces = doc.workplaces
		orgs = doc.orgs
		auths = doc.auths
		features = doc.features
		channels = doc.channels
		syncOrgsFromWorkplaces()
		return doc.version < 3
	}

	func save(allowEmpty: Bool = false) {
		guard loadSucceeded else {
			setError("config not loaded — fix or remove \(path.lastPathComponent) before saving")
			return
		}
		if !allowEmpty, workplaces.isEmpty, auths.isEmpty,
			FileManager.default.fileExists(atPath: path.path),
			let data = try? Data(contentsOf: path),
			let root = try? JSONSerialization.jsonObject(with: data) as? [String: Any]
		{
			let wp = root["workplaces"] as? [String: Any] ?? [:]
			let auth = root["auths"] as? [String: Any] ?? [:]
			if !wp.isEmpty || !auth.isEmpty {
				setError(
					"refusing to overwrite existing config with empty data — restart the Agent after updating")
				return
			}
		}
		syncOrgsFromWorkplaces()
		let doc = Document(
			defaultContext: defaultContext,
			orgs: orgs,
			workplaces: workplaces,
			auths: auths,
			features: features,
			channels: channels)
		do {
			try AgentPaths.prepareSocketDirectory()
			let encoder = JSONEncoder()
			encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
			let data = try encoder.encode(doc)
			try data.write(to: path, options: [.atomic])
			try? FileManager.default.setAttributes(
				[.posixPermissions: 0o600], ofItemAtPath: path.path)
			AgentConfigIntegrity.sign(configPath: path, presence: nil)
			setError(nil)
			if workplaces.isEmpty, auths.isEmpty {
				AgentIdleLock.applyEmptyConfigDefault()
			}
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

	private static func decodeErrorMessage(_ error: Error) -> String {
		guard let decoding = error as? DecodingError else {
			return error.localizedDescription
		}
		func path(_ context: DecodingError.Context) -> String {
			let parts = context.codingPath.map(\.stringValue)
			return parts.isEmpty ? "root" : parts.joined(separator: ".")
		}
		switch decoding {
		case .keyNotFound(let key, let context):
			return "missing key \"\(key.stringValue)\" at \(path(context))"
		case .valueNotFound(_, let context):
			return "missing value at \(path(context))"
		case .typeMismatch(_, let context):
			return "wrong type at \(path(context))"
		case .dataCorrupted(let context):
			return context.debugDescription
		@unknown default:
			return decoding.localizedDescription
		}
	}

	/// Upgrades agent-config.json v1 (`integrations` array) to workplaces + auths.
	private func applyV1Integrations(_ items: [Integration]) {
		workplaces = [:]
		orgs = [:]
		auths = [:]
		for row in items {
			if !row.context.trimmed.isEmpty {
				var wp = workplaces[row.context] ?? Workplace()
				if !row.url.trimmed.isEmpty { wp.url = row.url.trimmed }
				if !row.region.trimmed.isEmpty { wp.region = row.region.trimmed }
				if !row.environmentID.trimmed.isEmpty { wp.environmentId = row.environmentID.trimmed }
				if !row.tenantID.trimmed.isEmpty { wp.tenantId = row.tenantID.trimmed }
				workplaces[row.context] = wp
			}
			if !row.authName.trimmed.isEmpty {
				var auth = auths[row.authName] ?? AgentAuth()
				auth.type = row.authType
				auth.clientId = row.clientID
				auth.username = row.username
				auth.scope = row.scope
				auth.access = row.access
				if !row.context.trimmed.isEmpty, !auth.contexts.contains(row.context) {
					auth.contexts.append(row.context)
				}
				auths[row.authName] = auth
			}
		}
	}

	// MARK: policy

	func channelPolicy(for channel: BuddyChannel) -> ChannelPolicy {
		channels[channel.rawValue] ?? ChannelPolicy()
	}

	/// Writes features and channel policy from the Policy tab in one save.
	func savePolicy(features draftFeatures: AgentFeatures, channels draftChannels: [String: ChannelPolicy]) {
		var changed = false
		if !managedPolicyLocked("features"), features != draftFeatures {
			features = draftFeatures
			changed = true
		}
		for channel in BuddyChannel.allCases {
			guard !isManagedChannel(channel) else { continue }
			let key = channel.rawValue
			let policy = draftChannels[key] ?? ChannelPolicy()
			if policy.isEmpty {
				if channels.removeValue(forKey: key) != nil { changed = true }
			} else if channels[key] != policy {
				channels[key] = policy
				changed = true
			}
		}
		if changed {
			save()
			AgentLog.shared.config("saved policy")
		} else {
			ensureConfigSealed()
		}
	}

	/// Seals agent-config.json when no Keychain seal exists yet (first run or recovery).
	func ensureConfigSealed() {
		guard loadSucceeded else { return }
		guard AgentKeychain.backend == .dataProtection else { return }
		guard configIntegrity(unlocked: false, presence: nil) == .missing else { return }
		guard FileManager.default.fileExists(atPath: path.path) else { return }
		if AgentConfigIntegrity.sign(configPath: path, presence: nil) {
			AgentLog.shared.config("sealed config")
		}
	}

	func sortedWorkplaceNames() -> [String] {
		workplaces.keys.sorted()
	}

	// MARK: workplaces

	func upsertWorkplace(name: String, workplace: Workplace, previousName: String? = nil) {
		let trimmed = name.trimmed
		guard !trimmed.isEmpty else { return }
		if isManagedWorkplace(trimmed) || (previousName.map(isManagedWorkplace) ?? false) {
			setError("workplace \(trimmed) is managed by MDM")
			return
		}
		if let prev = previousName?.trimmed, prev != trimmed {
			renameWorkplace(from: prev, to: trimmed, workplace: workplace)
			return
		}
		let existed = workplaces[trimmed] != nil
		workplaces[trimmed] = workplace
		if defaultContext.trimmed.isEmpty {
			defaultContext = trimmed
		}
		save()
		AgentLog.shared.config(
			existed ? "updated workplace \(trimmed)" : "added workplace \(trimmed)")
	}

	func setDefaultContext(_ name: String) {
		guard !managedPolicyLocked("defaultContext") else { return }
		let trimmed = name.trimmed
		guard !trimmed.isEmpty, workplaces[trimmed] != nil else { return }
		guard defaultContext != trimmed else { return }
		defaultContext = trimmed
		save()
		AgentLog.shared.config("default workplace set to \(trimmed)")
	}

	func removeWorkplace(_ name: String) {
		let trimmed = name.trimmed
		guard !isManagedWorkplace(trimmed) else {
			setError("workplace \(trimmed) is managed by MDM")
			return
		}
		workplaces.removeValue(forKey: trimmed)
		removeFromOrgs(name: trimmed)
		for authName in auths.keys {
			auths[authName]?.contexts.removeAll { $0 == trimmed }
		}
		for (authName, auth) in auths where auth.contexts.isEmpty {
			AgentKeychain.deleteAll(auth: authName)
		}
		auths = auths.filter { !$0.value.contexts.isEmpty }
		if defaultContext == trimmed {
			defaultContext = workplaces.keys.sorted().first ?? ""
		}
		let clearedAll = workplaces.isEmpty && auths.isEmpty
		save(allowEmpty: clearedAll)
		AgentLog.shared.config("removed workplace \(trimmed)")
	}

	func addOrganization(name: String) {
		let trimmed = name.trimmed
		guard !trimmed.isEmpty else { return }
		workplaces[trimmed] = Workplace()
		if orgs[trimmed] == nil {
			orgs[trimmed] = OrgRoot()
		}
		if defaultContext.trimmed.isEmpty {
			defaultContext = trimmed
		}
		save()
		AgentLog.shared.config("added organization \(trimmed)")
	}

	func addChildWorkplace(
		name: String, parent: WorkplaceTreeNode?, kind: String, orgName: String = ""
	) {
		let trimmed = name.trimmed
		guard !trimmed.isEmpty else { return }
		var wp = Workplace()
		let parentKind = parent?.kind ?? ""
		let parentName = parent?.name ?? ""
		switch kind {
		case "organization":
			workplaces[trimmed] = wp
			orgs[trimmed] = OrgRoot()
		case "environment":
			wp.org = orgName.isEmpty ? (parentKind == "organization" ? parentName : parent?.org ?? "") : orgName
			wp.region = PlatformRegions.defaultValue
			workplaces[trimmed] = wp
			attachEnvironment(trimmed, org: wp.org)
		case "product-tenant", "tenant":
			wp.org = parent?.org ?? orgName
			workplaces[trimmed] = wp
			attachTenant(trimmed, org: wp.org, environment: parentName)
		default:
			wp.org = orgName.isEmpty ? parent?.org ?? "" : orgName
			workplaces[trimmed] = wp
			if parentKind == "organization" {
				attachEnvironment(trimmed, org: parentName)
			}
		}
		save()
		AgentLog.shared.config("added workplace \(trimmed) (\(kind))")
	}

	private func renameWorkplace(from: String, to: String, workplace: Workplace) {
		workplaces.removeValue(forKey: from)
		workplaces[to] = workplace
		renameInOrgs(from: from, to: to)
		for authName in auths.keys {
			if let idx = auths[authName]?.contexts.firstIndex(of: from) {
				auths[authName]?.contexts[idx] = to
			}
		}
		if defaultContext == from { defaultContext = to }
		save()
		AgentLog.shared.config("renamed workplace \(from) → \(to)")
	}

	// MARK: auths

	func upsertAuth(name: String, auth: AgentAuth, previousName: String? = nil) {
		let trimmed = name.trimmed
		guard !trimmed.isEmpty else { return }
		if isManagedAuth(trimmed) || (previousName.map(isManagedAuth) ?? false) {
			setError("auth \(trimmed) is managed by MDM")
			return
		}
		if let prev = previousName?.trimmed, prev != trimmed {
			if var row = auths.removeValue(forKey: prev) {
				row = auth
				auths[trimmed] = row
			} else {
				auths[trimmed] = auth
			}
			save()
			AgentLog.shared.config("renamed auth \(prev) → \(trimmed)")
			return
		}
		let existed = auths[trimmed] != nil
		auths[trimmed] = auth
		save()
		AgentLog.shared.config(existed ? "updated auth \(trimmed)" : "added auth \(trimmed)")
	}

	func removeAuth(_ name: String) {
		guard !isManagedAuth(name) else {
			setError("auth \(name) is managed by MDM")
			return
		}
		AgentKeychain.deleteAll(auth: name)
		auths.removeValue(forKey: name)
		markSecretsChanged()
		save(allowEmpty: workplaces.isEmpty && auths.isEmpty)
		AgentLog.shared.config("removed auth \(name)")
	}

	func setSecret(_ value: String, forAuth name: String, kind: AgentKeychain.Kind, context: LAContext? = nil) {
		AgentKeychain.set(value, auth: name, kind: kind, context: context)
		markSecretsChanged()
		AgentLog.shared.config("stored secret for auth \(name)")
	}

	func integration(forAuth auth: String, context ctx: String? = nil) -> Integration? {
		guard let row = auths[auth] else { return nil }
		let ctxName = ctx ?? row.contexts.first ?? ""
		let wp = workplaces[ctxName] ?? Workplace()
		return Integration(
			context: ctxName,
			url: wp.url,
			region: wp.region,
			environmentID: wp.environmentId,
			tenantID: wp.tenantId,
			authName: auth,
			authType: row.type,
			clientID: row.clientId,
			username: row.username,
			scope: row.scope,
			access: row.access)
	}

	struct MintTarget: Equatable {
		let context: String
		let baseURL: String
		let kind: String
	}

	func canMint(forAuth name: String) -> Bool {
		guard let auth = auths[name], hasSecret(forAuth: name), !auth.contexts.isEmpty else {
			return false
		}
		return !mintTargets(forAuth: name).isEmpty
	}

	func mintUnavailableReason(forAuth name: String) -> String {
		guard let auth = auths[name] else { return "Unknown auth" }
		if auth.contexts.isEmpty { return "Link a workplace before minting" }
		if !hasSecret(forAuth: name) { return "Store a secret before minting" }
		if mintTargets(forAuth: name).isEmpty {
			return "Linked workplaces are missing a Jamf URL or platform region"
		}
		return "Cannot mint bearer"
	}

	func mintTargets(forAuth name: String) -> [MintTarget] {
		guard let auth = auths[name] else { return [] }
		var seen: Set<String> = []
		var out: [MintTarget] = []
		for ctx in auth.contexts {
			guard let base = resolveMintBaseURL(auth: auth, context: ctx), seen.insert(base).inserted
			else { continue }
			let kind = auth.type == "basic" ? "basic" : auth.type
			out.append(MintTarget(context: ctx, baseURL: base, kind: kind))
		}
		return out
	}

	/// Mint bearers for every resolvable linked workplace. Call off the main queue.
	/// Returns nil on success (at least one bearer cached), or an error message.
	func mintTokens(forAuth name: String, gate: UnlockGate, state: AgentState, force: Bool = false)
		-> String?
	{
		guard auths[name] != nil else { return "Unknown auth" }
		if !canMint(forAuth: name) {
			let why = mintUnavailableReason(forAuth: name)
			AgentLog.shared.mint("UI mint failed for \(name): \(why)")
			return why
		}
		if let why = authorizeForMint(auth: name, gate: gate, state: state) {
			AgentLog.shared.mint("UI mint failed for \(name): \(why)")
			return why
		}
		let presenceResult = presenceForMint(auth: name, gate: gate, state: state)
		guard let presence = presenceResult.0 else {
			let why = presenceResult.1 ?? "Unlock declined — unlock from the menu bar"
			AgentLog.shared.mint("UI mint failed for \(name): \(why)")
			return why
		}
		if let why = AgentConfigIntegrity.mintBlockedReason(
			configPath: path, presence: presence)
		{
			AgentLog.shared.mint("UI mint failed for \(name): \(why)")
			return why
		}

		let targets = mintTargets(forAuth: name)
		var minted = 0
		var lastError: String?
		for target in targets {
			let key = "\(name)@\(target.baseURL)"
			if !force, state.cachedToken(key: key) != nil {
				AgentLog.shared.mint("UI cache hit auth=\(name) base_url=\(target.baseURL)")
				minted += 1
				continue
			}
			guard let integration = integration(forAuth: name, context: target.context) else {
				continue
			}
			do {
				let token = try TokenMinter.mint(
					integration: integration, baseURL: target.baseURL, kind: target.kind,
					presence: presence)
				state.store(
					token: CachedToken(
						auth: name, baseURL: target.baseURL, value: token.value,
						expiresAt: token.expiresAt),
					key: key)
				minted += 1
				AgentLog.shared.mint("UI minted auth=\(name) base_url=\(target.baseURL)")
			} catch {
				lastError = mintErrorMessage(error)
				AgentLog.shared.mint(
					"UI mint failed auth=\(name) base_url=\(target.baseURL): \(lastError ?? "error")")
			}
		}
		if minted > 0 { return nil }
		return lastError ?? "Mint failed"
	}

	private func resolveMintBaseURL(auth: AgentAuth, context: String) -> String? {
		guard let wp = workplaceEnrichedForMint(name: context, auth: auth) else { return nil }
		switch auth.type {
		case "platform":
			if auth.effectiveScope == "organization" {
				return MintURLHelpers.orgPlatformGateway
			}
			return platformGatewayURL(for: wp)
		case "basic", "oauth":
			return instanceBaseURL(for: wp)
		default:
			return instanceBaseURL(for: wp)
		}
	}

	private func workplaceEnrichedForMint(name: String, auth: AgentAuth) -> Workplace? {
		guard var wp = workplaces[name] else { return nil }
		guard auth.type == "platform", wp.workplaceKind() == "tenant" else { return wp }
		for (_, org) in orgs {
			for (_, env) in org.environments {
				guard env.tenants[name] != nil else { continue }
				if wp.region.trimmed.isEmpty { wp.region = env.region }
				if wp.environmentId.trimmed.isEmpty { wp.environmentId = env.effectiveEnvironmentID }
				return wp
			}
		}
		let eid = wp.effectiveEnvironmentID
		if !eid.isEmpty {
			for (_, candidate) in workplaces where candidate.workplaceKind() == "environment" {
				if candidate.effectiveEnvironmentID == eid {
					if wp.region.trimmed.isEmpty { wp.region = candidate.region }
					break
				}
			}
		}
		return wp
	}

	private func platformGatewayURL(for wp: Workplace) -> String? {
		if wp.isOrganization { return MintURLHelpers.orgPlatformGateway }
		let region = wp.region.trimmed.lowercased()
		if !region.isEmpty { return "https://\(region).api.jamfcloud.com" }
		let url = wp.url.trimmed
		guard !url.isEmpty else { return nil }
		if MintURLHelpers.looksLikeOrgGateway(url) { return MintURLHelpers.orgPlatformGateway }
		if let region = MintURLHelpers.regionFromGateway(url), !region.isEmpty {
			return "https://\(region).api.jamfcloud.com"
		}
		if MintURLHelpers.looksLikePlatformGateway(url) {
			return MintURLHelpers.normalizeBaseURL(url)
		}
		return nil
	}

	private func instanceBaseURL(for wp: Workplace) -> String? {
		let url = wp.url.trimmed
		guard !url.isEmpty else { return nil }
		if MintURLHelpers.looksLikePlatformGateway(url) { return nil }
		return MintURLHelpers.normalizeBaseURL(url)
	}

	private func authorizeForMint(auth: String, gate: UnlockGate, state: AgentState) -> String? {
		if state.unlocked { return nil }
		switch gate.authorize(reason: "mint a Jamf token for \(auth)") {
		case .unlocked:
			return nil
		case .declined:
			return "Unlock declined"
		case .timedOut:
			return "Unlock prompt timed out"
		case .unavailable:
			if UnlockGate.promptingDisabled {
				return "Unlock prompting is off — unlock from the menu bar"
			}
			return "Presence unavailable — unlock from the menu bar"
		}
	}

	private func presenceForMint(auth: String, gate: UnlockGate, state: AgentState)
		-> (LAContext?, String?)
	{
		if let ctx = gate.presenceContext { return (ctx, nil) }
		switch gate.authorize(reason: "mint a Jamf token for \(auth)") {
		case .unlocked:
			return (gate.presenceContext, nil)
		case .declined:
			return (nil, "Unlock declined")
		case .timedOut:
			return (nil, "Unlock prompt timed out")
		case .unavailable:
			if UnlockGate.promptingDisabled {
				return (nil, "Unlock prompting is off — unlock from the menu bar")
			}
			return (nil, "Presence unavailable — unlock from the menu bar")
		}
	}

	private func mintErrorMessage(_ error: Error) -> String {
		switch error {
		case MintError.unsupported(let why): return why
		case MintError.failed(let why): return why
		default: return error.localizedDescription
		}
	}

	// MARK: import

	@discardableResult
	func importExport(_ data: Data) throws -> Int {
		guard let root = try JSONSerialization.jsonObject(with: data) as? [String: Any] else {
			throw AgentStoreError.badImport("top level is not an object")
		}
		return try importDocument(root, replace: true)
	}

	struct BackupImportResult {
		var integrations: Int
		var secrets: Int
		var ephemeral: Int
		var failed: Int
		var inBackup: Int
	}

	@discardableResult
	func importDocumentForBackup(_ root: [String: Any]) throws -> Int {
		try importDocument(root)
	}

	@discardableResult
	func importBackup(
		_ data: Data, passphrase: String, presence: LAContext? = nil
	) throws -> BackupImportResult {
		let contents = try AgentBackup.read(data, passphrase: passphrase)
		let added = try importDocument(contents.document)
		let result = importBackupSecrets(contents.secrets, presence: presence)
		return BackupImportResult(
			integrations: added, secrets: result.secrets, ephemeral: result.ephemeral,
			failed: result.failed, inBackup: contents.secrets.count)
	}

	/// Writes decrypted backup secrets into the Agent Keychain. Safe off the main
	/// queue; pass `presence` when replacing existing ACL-gated items.
	func importBackupSecrets(
		_ secrets: [String: String], presence: LAContext? = nil
	) -> BackupImportResult {
		var stored = 0
		var ephemeral = 0
		var failed = 0
		for authName in secrets.keys.sorted() {
			guard let secret = secrets[authName], let row = auths[authName] else {
				failed += 1
				continue
			}
			AgentKeychain.deleteAll(auth: authName, context: presence)
			let kinds: [AgentKeychain.Kind] =
				row.type == "basic" ? [.password, .clientSecret] : [.clientSecret, .password]
			var landed = false
			for kind in kinds {
				let ok = AgentKeychain.set(secret, auth: authName, kind: kind, context: presence)
				guard AgentKeychain.has(auth: authName, kind: kind) else { continue }
				if ok && AgentKeychain.storedInKeychain(auth: authName, kind: kind) {
					stored += 1
				} else {
					ephemeral += 1
				}
				landed = true
				break
			}
			if !landed {
				failed += 1
			}
		}
		markSecretsChanged()
		return BackupImportResult(
			integrations: 0, secrets: stored, ephemeral: ephemeral,
			failed: failed, inBackup: secrets.count)
	}

	@discardableResult
	private func importDocument(_ root: [String: Any], replace: Bool = false) throws -> Int {
		if Thread.isMainThread {
			return try importDocumentOnMain(root, replace: replace)
		}
		var result = 0
		var failure: Error?
		DispatchQueue.main.sync {
			do {
				result = try importDocumentOnMain(root, replace: replace)
			} catch {
				failure = error
			}
		}
		if let failure { throw failure }
		return result
	}

	@discardableResult
	private func importDocumentOnMain(_ root: [String: Any], replace: Bool) throws -> Int {
		if replace {
			workplaces = [:]
			orgs = [:]
			auths = [:]
			features = AgentFeatures()
			channels = [:]
		}
		if let orgMap = root["orgs"] as? [String: Any], !orgMap.isEmpty {
			orgs = decodeOrgs(orgMap)
		}
		var contexts =
			(root["contexts"] as? [String: Any]) ?? (root["tenants"] as? [String: Any]) ?? [:]
		for (name, entry) in workplacesFromOrgs(root["orgs"] as? [String: Any] ?? [:]) {
			if contexts[name] == nil { contexts[name] = entry }
		}
		let authMap = (root["auths"] as? [String: Any]) ?? [:]
		if contexts.isEmpty && authMap.isEmpty && orgs.isEmpty {
			throw AgentStoreError.badImport("no contexts or auths found")
		}
		var added = 0
		var nextWorkplaces = replace ? [:] : workplaces
		var nextAuths = replace ? [:] : auths
		for (name, raw) in decodeWorkplaces(contexts) {
			if nextWorkplaces[name] == nil { added += 1 }
			nextWorkplaces[name] = raw
		}
		for (authName, raw) in decodeAuths(authMap, contexts: contexts) {
			if nextAuths[authName] == nil { added += 1 }
			nextAuths[authName] = raw
		}
		workplaces = nextWorkplaces
		auths = nextAuths
		if let ctxSel = root["context"] as? [String: Any],
			let def = ctxSel["default"] as? String, !def.isEmpty
		{
			defaultContext = def
		} else if defaultContext.trimmed.isEmpty {
			defaultContext = workplaces.keys.sorted().first ?? ""
		}
		if let feat = root["features"] as? [String: Any] {
			features = AgentConfigExport.decodeFeatures(feat)
		}
		if let ch = root["channels"] as? [String: Any] {
			channels = AgentConfigExport.decodeChannels(ch)
		}
		syncOrgsFromWorkplaces()
		save()
		if replace {
			AgentLog.shared.config(
				"imported setup (\(workplaces.count) workplaces, \(auths.count) auths)")
		} else {
			AgentLog.shared.config("merged import (\(added) new item(s))")
		}
		return added
	}

	// MARK: orgs sync

	private func syncOrgsFromWorkplaces() {
		for (orgName, var org) in orgs {
			if var wp = workplaces[orgName] {
				wp.org = ""
				workplaces[orgName] = wp
			}
			for (envName, var env) in org.environments {
				if var wp = workplaces[envName] {
					env.url = wp.url
					env.region = wp.region
					env.environmentId = wp.environmentId
					if !wp.environmentId.isEmpty { env.id = wp.environmentId }
					wp.org = orgName
					workplaces[envName] = wp
				}
				for (tenName, var ten) in env.tenants {
					if var wp = workplaces[tenName] {
						ten.url = wp.url
						ten.tenantId = wp.tenantId
						env.tenants[tenName] = ten
						wp.org = orgName
						workplaces[tenName] = wp
					}
				}
				org.environments[envName] = env
			}
			orgs[orgName] = org
		}
	}

	private func attachEnvironment(_ name: String, org: String) {
		guard !org.trimmed.isEmpty else { return }
		var root = orgs[org] ?? OrgRoot()
		if root.environments[name] == nil {
			root.environments[name] = OrgEnvironment()
		}
		orgs[org] = root
	}

	private func attachTenant(_ name: String, org: String, environment: String) {
		guard !org.trimmed.isEmpty, !environment.trimmed.isEmpty else { return }
		var root = orgs[org] ?? OrgRoot()
		var env = root.environments[environment] ?? OrgEnvironment()
		if env.tenants[name] == nil {
			env.tenants[name] = OrgProductTenant()
		}
		root.environments[environment] = env
		orgs[org] = root
	}

	private func removeFromOrgs(name: String) {
		if orgs.removeValue(forKey: name) != nil { return }
		for orgName in orgs.keys {
			guard var org = orgs[orgName] else { continue }
			if org.environments.removeValue(forKey: name) != nil {
				orgs[orgName] = org
				return
			}
			for envName in org.environments.keys {
				var env = org.environments[envName] ?? OrgEnvironment()
				if env.tenants.removeValue(forKey: name) != nil {
					org.environments[envName] = env
					orgs[orgName] = org
					return
				}
			}
		}
	}

	private func renameInOrgs(from: String, to: String) {
		if orgs[from] != nil {
			orgs[to] = orgs.removeValue(forKey: from) ?? OrgRoot()
			return
		}
		for orgName in orgs.keys {
			guard var org = orgs[orgName] else { continue }
			if let env = org.environments.removeValue(forKey: from) {
				org.environments[to] = env
				orgs[orgName] = org
				return
			}
			for envName in org.environments.keys {
				var env = org.environments[envName] ?? OrgEnvironment()
				if let ten = env.tenants.removeValue(forKey: from) {
					env.tenants[to] = ten
					org.environments[envName] = env
					orgs[orgName] = org
					return
				}
			}
		}
	}

	private func workplacesFromOrgs(_ orgMap: [String: Any]) -> [String: [String: Any]] {
		var out: [String: [String: Any]] = [:]
		for (orgName, rawOrg) in orgMap {
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
				entry["org"] = orgName
				out[envName] = entry
				let tenants = env["tenants"] as? [String: Any] ?? [:]
				for (tenantName, rawTenant) in tenants {
					guard let tenant = rawTenant as? [String: Any] else { continue }
					let jamfPro = tenant["jamfpro"] as? [String: Any] ?? [:]
					var row: [String: Any] = [:]
					row["org"] = orgName
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

	private func decodeOrgs(_ raw: [String: Any]) -> [String: OrgRoot] {
		var out: [String: OrgRoot] = [:]
		for (name, value) in raw {
			guard let data = try? JSONSerialization.data(withJSONObject: value),
				let org = try? JSONDecoder().decode(OrgRoot.self, from: data)
			else { continue }
			out[name] = org
		}
		return out
	}

	private func decodeWorkplaces(_ raw: [String: Any]) -> [String: Workplace] {
		var out: [String: Workplace] = [:]
		for (name, value) in raw {
			guard let entry = value as? [String: Any] else { continue }
			var wp = workplaces[name] ?? Workplace()
			if let url = entry["url"] as? String { wp.url = url }
			if let region = entry["region"] as? String { wp.region = region }
			if let id = (entry["environmentId"] as? String) ?? (entry["id"] as? String) {
				wp.environmentId = id
			}
			if let tid = entry["tenantId"] as? String { wp.tenantId = tid }
			if let org = entry["org"] as? String { wp.org = org }
			wp.denyBuiltins = AgentConfigLists.parse(entry["deny_builtins"])
			wp.allowBuiltins = AgentConfigLists.parse(entry["allow_builtins"])
			out[name] = wp
		}
		return out
	}

	private func decodeAuths(_ raw: [String: Any], contexts: [String: Any]) -> [String: AgentAuth] {
		var out: [String: AgentAuth] = [:]
		for (authName, value) in raw {
			guard let entry = value as? [String: Any] else { continue }
			var auth = auths[authName] ?? AgentAuth()
			auth.type = (entry["type"] as? String) ?? "oauth"
			auth.clientId = (entry["clientId"] as? String) ?? ""
			auth.username = (entry["username"] as? String) ?? ""
			auth.scope = (entry["scope"] as? String) ?? ""
			auth.access = (entry["access"] as? String) ?? ""
			let linked = (entry["contexts"] as? [String]) ?? (entry["tenants"] as? [String]) ?? []
			if !linked.isEmpty {
				auth.contexts = linked
			} else if auth.contexts.isEmpty {
				auth.contexts = [contexts.keys.sorted().first ?? ""].filter { !$0.isEmpty }
			}
			auth.channels = AgentConfigLists.parse(entry["channels"])
			auth.denyBuiltins = AgentConfigLists.parse(entry["deny_builtins"])
			auth.allowBuiltins = AgentConfigLists.parse(entry["allow_builtins"])
			out[authName] = auth
		}
		return out
	}

	// MARK: export

	func exportSetupJSON() throws -> Data {
		let doc = configDocument()
		guard !doc.isEmpty else {
			throw AgentStoreError.exportFailed("nothing to export (no workplaces or auths)")
		}
		guard JSONSerialization.isValidJSONObject(doc) else {
			throw AgentStoreError.exportFailed("export document is not JSON-serializable")
		}
		var data = try JSONSerialization.data(withJSONObject: doc, options: [.prettyPrinted, .sortedKeys])
		data.append(0x0A)
		return data
	}

	/// Reads secrets for a cred backup. Pass `presence` when items are ACL-gated.
	func secretsForBackup(presence: LAContext? = nil) -> [String: String] {
		var out: [String: String] = [:]
		for (name, auth) in auths.sorted(by: { $0.key < $1.key }) {
			guard auth.type != "command" else { continue }
			let kinds: [AgentKeychain.Kind] =
				auth.type == "basic" ? [.password, .clientSecret] : [.clientSecret, .password]
			for kind in kinds {
				if let value = AgentKeychain.get(auth: name, kind: kind, context: presence),
					!value.trimmed.isEmpty
				{
					out[name] = value
					break
				}
			}
		}
		return out
	}

	func credBackupData(
		passphrase: String, presence: LAContext? = nil
	) throws -> (data: Data, result: AgentBackup.WriteResult) {
		let secrets = secretsForBackup(presence: presence)
		let packed = try AgentBackup.write(
			defaultContext: defaultContext,
			workplaces: workplaces,
			orgs: orgs,
			auths: auths,
			features: features,
			channels: channels,
			secrets: secrets,
			passphrase: passphrase)
		return (packed.data, packed.result)
	}

	// MARK: wire

	func configDocument() -> [String: Any] {
		syncOrgsFromWorkplaces()
		return AgentConfigExport.identityDocument(
			defaultContext: defaultContext,
			workplaces: workplaces,
			orgs: orgs,
			auths: auths,
			features: features,
			channels: channels,
			includeAuths: true)
	}

	func integrationSummaries() -> [[String: Any]] {
		sortedAuthNames.flatMap { name -> [[String: Any]] in
			let row = auths[name] ?? AgentAuth()
			let contexts = row.contexts.isEmpty ? [""] : row.contexts
			return contexts.map { ctx in
				var out: [String: Any] = [
					"context": ctx,
					"auth": name,
					"type": row.type,
					"secret": hasSecret(forAuth: name) ? "stored" : "missing",
				]
				if !row.scope.trimmed.isEmpty { out["scope"] = row.scope }
				if let wp = workplaces[ctx], !wp.url.trimmed.isEmpty { out["url"] = wp.url }
				if !row.clientId.trimmed.isEmpty { out["client_id"] = row.clientId }
				if !row.username.trimmed.isEmpty { out["username"] = row.username }
				if let problem = row.problem(name: name, workplaces: workplaces) {
					out["problem"] = problem
				}
				return out
			}
		}
	}
}

private enum MintURLHelpers {
	/// Organization-scoped Account clients; `api.jamfcloud.com` in samples does not resolve.
	static let orgPlatformGateway = "https://us.api.jamfcloud.com"

	static func normalizeBaseURL(_ raw: String) -> String {
		var url = raw.trimmed
		while url.hasSuffix("/") { url.removeLast() }
		return url
	}

	static func gatewayHost(_ rawURL: String) -> String {
		var u = rawURL.trimmed.lowercased()
		if u.hasPrefix("https://") { u.removeFirst("https://".count) }
		else if u.hasPrefix("http://") { u.removeFirst("http://".count) }
		if let slash = u.firstIndex(of: "/") { u = String(u[..<slash]) }
		return u
	}

	static func regionFromGateway(_ rawURL: String) -> String? {
		let host = gatewayHost(rawURL)
		if host == "api.jamfcloud.com" { return nil }
		let suffix = ".api.jamfcloud.com"
		guard host.hasSuffix(suffix) else { return nil }
		let region = String(host.dropLast(suffix.count))
		return region.isEmpty ? nil : region
	}

	static func looksLikeOrgGateway(_ rawURL: String) -> Bool {
		gatewayHost(rawURL) == "api.jamfcloud.com"
	}

	static func looksLikePlatformGateway(_ rawURL: String) -> Bool {
		regionFromGateway(rawURL) != nil || looksLikeOrgGateway(rawURL)
			|| gatewayHost(rawURL) == "apigw.jamf.com"
			|| gatewayHost(rawURL).hasSuffix(".apigw.jamf.com")
	}
}

enum AgentStoreError: Error, CustomStringConvertible {
	case badImport(String)
	case exportFailed(String)

	var description: String {
		switch self {
		case .badImport(let why): return "import failed: \(why)"
		case .exportFailed(let why): return "export failed: \(why)"
		}
	}
}
