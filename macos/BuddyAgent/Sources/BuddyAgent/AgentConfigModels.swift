import Foundation

/// Flat workplace metadata (buddy `contexts:` / `tenants:` entry).
struct Workplace: Codable, Equatable {
	var url: String = ""
	var region: String = ""
	var environmentId: String = ""
	var tenantId: String = ""
	/// Parent organization workplace name (not set on org nodes).
	var org: String = ""
	var denyBuiltins: [String] = []
	var allowBuiltins: [String] = []

	init(
		url: String = "",
		region: String = "",
		environmentId: String = "",
		tenantId: String = "",
		org: String = "",
		denyBuiltins: [String] = [],
		allowBuiltins: [String] = []
	) {
		self.url = url
		self.region = region
		self.environmentId = environmentId
		self.tenantId = tenantId
		self.org = org
		self.denyBuiltins = denyBuiltins
		self.allowBuiltins = allowBuiltins
	}

	init(from decoder: Decoder) throws {
		let c = try decoder.container(keyedBy: CodingKeys.self)
		url = try c.decodeIfPresent(String.self, forKey: .url) ?? ""
		region = try c.decodeIfPresent(String.self, forKey: .region) ?? ""
		environmentId = try c.decodeIfPresent(String.self, forKey: .environmentId) ?? ""
		tenantId = try c.decodeIfPresent(String.self, forKey: .tenantId) ?? ""
		org = try c.decodeIfPresent(String.self, forKey: .org) ?? ""
		denyBuiltins = try c.decodeIfPresent([String].self, forKey: .denyBuiltins) ?? []
		allowBuiltins = try c.decodeIfPresent([String].self, forKey: .allowBuiltins) ?? []
	}

	func workplaceKind() -> String {
		if isOrganization {
			return "organization"
		}
		if !tenantId.trimmed.isEmpty {
			return "tenant"
		}
		if !effectiveEnvironmentID.isEmpty || (!region.trimmed.isEmpty && url.trimmed.isEmpty) {
			return "environment"
		}
		return "instance"
	}

	var displayKind: String {
		workplaceKind() == "tenant" ? "product-tenant" : workplaceKind()
	}

	var isOrganization: Bool {
		org.trimmed.isEmpty && region.trimmed.isEmpty && effectiveEnvironmentID.isEmpty
			&& tenantId.trimmed.isEmpty && url.trimmed.isEmpty
	}

	var effectiveEnvironmentID: String {
		environmentId.trimmed
	}

	func problem() -> String? {
		switch workplaceKind() {
		case "organization":
			return nil
		case "environment":
			if region.trimmed.isEmpty { return "region is required" }
			if effectiveEnvironmentID.isEmpty { return "environment id is required" }
		case "tenant":
			if tenantId.trimmed.isEmpty && url.trimmed.isEmpty { return "tenant id or url is required" }
		case "instance":
			if url.trimmed.isEmpty { return "Jamf Pro URL is required" }
		default:
			break
		}
		return nil
	}
}

/// Known Jamf Account platform regions. Quick setup offers these as a menu; full
/// workplace editors keep a freeform region field for future regions.
enum PlatformRegions {
	static let known: [(label: String, value: String)] = [
		("us", "us"),
		("eu", "eu"),
		("apac", "apac"),
	]

	static func normalized(_ raw: String) -> String {
		raw.trimmed.lowercased()
	}

	static func isKnown(_ region: String) -> Bool {
		let value = normalized(region)
		return known.contains { $0.value == value }
	}

	static var defaultValue: String { "us" }
}

/// Jamf Pro instance URL validation (basic/oauth workplaces — not Account gateway hosts).
enum JamfProURL {
	static func normalized(_ raw: String) -> String? {
		var text = raw.trimmed
		guard !text.isEmpty else { return nil }
		let lower = text.lowercased()
		if !lower.hasPrefix("http://"), !lower.hasPrefix("https://") {
			text = "https://\(text)"
		}
		guard let url = URL(string: text),
			let scheme = url.scheme?.lowercased(),
			scheme == "http" || scheme == "https",
			let host = url.host?.lowercased(), !host.isEmpty
		else {
			return nil
		}
		if looksLikePlatformGateway(text) { return nil }
		if host != "localhost", !host.contains(".") { return nil }
		var out = text
		while out.hasSuffix("/") { out.removeLast() }
		return out
	}

	static func validationError(for raw: String) -> String? {
		let trimmed = raw.trimmed
		guard !trimmed.isEmpty else { return nil }
		if normalized(trimmed) != nil { return nil }
		var probe = trimmed
		let lower = probe.lowercased()
		if !lower.hasPrefix("http://"), !lower.hasPrefix("https://") {
			probe = "https://\(probe)"
		}
		if looksLikePlatformGateway(probe) {
			return "Account gateway URL — switch auth type to Platform, not Jamf Pro URL."
		}
		return "Enter a valid Jamf Pro URL (https://yourcompany.jamfcloud.com)."
	}

	private static func gatewayHost(_ rawURL: String) -> String {
		var u = rawURL.trimmed.lowercased()
		if u.hasPrefix("https://") { u.removeFirst("https://".count) }
		else if u.hasPrefix("http://") { u.removeFirst("http://".count) }
		if let slash = u.firstIndex(of: "/") { u = String(u[..<slash]) }
		return u
	}

	private static func looksLikePlatformGateway(_ rawURL: String) -> Bool {
		let host = gatewayHost(rawURL)
		if host == "api.jamfcloud.com" { return true }
		if host.hasSuffix(".api.jamfcloud.com") { return true }
		if host == "apigw.jamf.com" || host.hasSuffix(".apigw.jamf.com") { return true }
		return false
	}
}

/// Buddy client channel names (`channels.cli|mcp|ide` in config.yaml).
enum BuddyChannel: String, CaseIterable, Identifiable {
	case cli
	case mcp
	case ide

	var id: String { rawValue }

	var label: String {
		switch self {
		case .cli: return "CLI"
		case .mcp: return "MCP"
		case .ide: return "IDE"
		}
	}
}

/// Agent-served `features:` (wire + export). `use_agent` and `force_agent` are
/// client-side only (config.yaml, MDM plist, or env) and are never encoded here.
/// An empty config.yaml on macOS still uses this document when the Agent is
/// running; `use_agent` / `force_agent` make an absent Agent a hard error.
struct AgentFeatures: Codable, Equatable {
	var classicInstanceOnly: Bool = false

	enum CodingKeys: String, CodingKey {
		case classicInstanceOnly
		case useAgent
		case forceAgent
	}

	init() {}

	init(from decoder: Decoder) throws {
		let c = try decoder.container(keyedBy: CodingKeys.self)
		classicInstanceOnly = try c.decodeIfPresent(Bool.self, forKey: .classicInstanceOnly) ?? false
		_ = try c.decodeIfPresent(Bool.self, forKey: .useAgent)
		_ = try c.decodeIfPresent(Bool.self, forKey: .forceAgent)
	}

	func encode(to encoder: Encoder) throws {
		var c = encoder.container(keyedBy: CodingKeys.self)
		if classicInstanceOnly {
			try c.encode(true, forKey: .classicInstanceOnly)
		}
	}
}

/// Per-channel policy under top-level `channels:`.
struct ChannelPolicy: Codable, Equatable {
	var access: String = ""
	/// Default workplace for this client (`context:` in YAML).
	var context: String = ""
	var contexts: [String] = []
	var denyBuiltins: [String] = []
	var allowBuiltins: [String] = []

	init() {}

	init(from decoder: Decoder) throws {
		let c = try decoder.container(keyedBy: CodingKeys.self)
		access = try c.decodeIfPresent(String.self, forKey: .access) ?? ""
		context = try c.decodeIfPresent(String.self, forKey: .context) ?? ""
		contexts = try c.decodeIfPresent([String].self, forKey: .contexts) ?? []
		denyBuiltins = try c.decodeIfPresent([String].self, forKey: .denyBuiltins) ?? []
		allowBuiltins = try c.decodeIfPresent([String].self, forKey: .allowBuiltins) ?? []
	}

	var isEmpty: Bool {
		access.trimmed.isEmpty && context.trimmed.isEmpty && contexts.isEmpty
			&& denyBuiltins.isEmpty && allowBuiltins.isEmpty
	}
}

enum AgentConfigLists {
	static func normalized(_ items: [String]) -> [String] {
		var seen: Set<String> = []
		var out: [String] = []
		for item in items {
			let t = item.trimmed
			guard !t.isEmpty, !seen.contains(t) else { continue }
			seen.insert(t)
			out.append(t)
		}
		return out
	}

	static func parse(_ value: Any?) -> [String] {
		if let items = value as? [Any] {
			return normalized(items.compactMap { $0 as? String })
		}
		if let scalar = value as? String {
			let t = scalar.trimmed
			return t.isEmpty ? [] : [t]
		}
		return []
	}

	static func linesText(_ patterns: [String]) -> String {
		normalized(patterns).joined(separator: "\n")
	}

	static func patterns(fromLines text: String) -> [String] {
		normalized(text.components(separatedBy: .newlines))
	}
}

/// Auth metadata the Agent owns. Secrets live in Keychain only.
struct AgentAuth: Codable, Equatable {
	var type: String = "oauth"
	var clientId: String = ""
	var username: String = ""
	var scope: String = ""
	var access: String = ""
	var contexts: [String] = []
	/// Buddy clients allowed to use this auth (empty = all: cli, mcp, ide).
	var channels: [String] = []
	var denyBuiltins: [String] = []
	var allowBuiltins: [String] = []

	init() {}

	init(from decoder: Decoder) throws {
		let c = try decoder.container(keyedBy: CodingKeys.self)
		type = try c.decodeIfPresent(String.self, forKey: .type) ?? "oauth"
		clientId = try c.decodeIfPresent(String.self, forKey: .clientId) ?? ""
		username = try c.decodeIfPresent(String.self, forKey: .username) ?? ""
		scope = try c.decodeIfPresent(String.self, forKey: .scope) ?? ""
		access = try c.decodeIfPresent(String.self, forKey: .access) ?? ""
		contexts = try c.decodeIfPresent([String].self, forKey: .contexts) ?? []
		channels = try c.decodeIfPresent([String].self, forKey: .channels) ?? []
		denyBuiltins = try c.decodeIfPresent([String].self, forKey: .denyBuiltins) ?? []
		allowBuiltins = try c.decodeIfPresent([String].self, forKey: .allowBuiltins) ?? []
	}

	var secretKind: AgentKeychain.Kind {
		type == "basic" ? .password : .clientSecret
	}

	func hasSecret(name: String) -> Bool {
		guard !name.trimmed.isEmpty else { return false }
		return AgentKeychain.hasAny(auth: name)
	}

	func displayScope(workplaces: [String: Workplace]) -> String {
		if type != "platform" { return "—" }
		let s = effectiveScope
		return s.isEmpty ? "—" : s
	}

	/// Platform scope with buddy defaults (empty → environment).
	var effectiveScope: String {
		let s = scope.trimmed.lowercased()
		switch s {
		case "org", "organisation", "organization":
			return "organization"
		case "env", "environment":
			return "environment"
		case "tenant":
			return "tenant"
		case "":
			return type == "platform" ? "environment" : ""
		default:
			return s
		}
	}

	/// Whether this auth may be linked to the workplace (`AuthFitsWorkplace` in buddy).
	func fitsWorkplace(_ workplace: Workplace) -> Bool {
		switch type {
		case "platform":
			switch effectiveScope {
			case "organization":
				return workplace.isOrganization
			case "tenant":
				return workplace.workplaceKind() == "tenant"
			case "environment":
				switch workplace.workplaceKind() {
				case "organization", "instance":
					return false
				default:
					return true
				}
			default:
				return true
			}
		case "basic":
			switch workplace.workplaceKind() {
			case "instance", "environment", "tenant":
				return !workplace.url.trimmed.isEmpty
			default:
				return false
			}
		case "oauth":
			return !workplace.isOrganization
		default:
			return true
		}
	}

	func linkableWorkplaceHint() -> String {
		switch type {
		case "platform":
			switch effectiveScope {
			case "organization":
				return "Organization-scoped platform auths link to organization workplaces only."
			case "tenant":
				return "Tenant-scoped platform auths link to product-tenant workplaces."
			default:
				return "Environment-scoped platform auths link to environment or product-tenant workplaces."
			}
		case "basic":
			return "Basic auths link to Jamf Pro instance, environment, or product-tenant workplaces that have a URL."
		case "oauth":
			return "OAuth auths link to Jamf Pro instance or product-tenant workplaces — not organization nodes."
		default:
			return "Workplaces this auth may use."
		}
	}

	func problem(name: String, workplaces: [String: Workplace]) -> String? {
		if name.trimmed.isEmpty { return "auth name is required" }
		if contexts.isEmpty { return "link at least one workplace" }
		switch type {
		case "basic" where username.trimmed.isEmpty:
			return "username is required"
		case "oauth", "platform":
			if clientId.trimmed.isEmpty { return "client id is required" }
		default:
			break
		}
		if !hasSecret(name: name) { return "secret not stored yet" }
		for ctx in contexts {
			guard let wp = workplaces[ctx] else {
				return "linked workplace \(ctx) is missing"
			}
			if !fitsWorkplace(wp) {
				return "auth scope does not fit linked workplace \(ctx)"
			}
			if wp.problem() != nil {
				return "linked workplace \(ctx) is incomplete"
			}
		}
		return nil
	}
}

struct OrgProductTenant: Codable, Equatable {
	var tenantId: String = ""
	var url: String = ""
	var jamfpro: JamfProRef = JamfProRef()

	struct JamfProRef: Codable, Equatable {
		var url: String = ""
		var tenantId: String = ""

		init() {}

		init(from decoder: Decoder) throws {
			let c = try decoder.container(keyedBy: CodingKeys.self)
			url = try c.decodeIfPresent(String.self, forKey: .url) ?? ""
			tenantId = try c.decodeIfPresent(String.self, forKey: .tenantId) ?? ""
		}
	}

	init() {}

	init(from decoder: Decoder) throws {
		let c = try decoder.container(keyedBy: CodingKeys.self)
		tenantId = try c.decodeIfPresent(String.self, forKey: .tenantId) ?? ""
		url = try c.decodeIfPresent(String.self, forKey: .url) ?? ""
		jamfpro = try c.decodeIfPresent(JamfProRef.self, forKey: .jamfpro) ?? JamfProRef()
		if url.trimmed.isEmpty { url = jamfpro.url }
		if tenantId.trimmed.isEmpty { tenantId = jamfpro.tenantId }
	}
}

struct OrgEnvironment: Codable, Equatable {
	var id: String = ""
	var environmentId: String = ""
	var region: String = ""
	var url: String = ""
	var tenants: [String: OrgProductTenant] = [:]

	init() {}

	init(from decoder: Decoder) throws {
		let c = try decoder.container(keyedBy: CodingKeys.self)
		id = try c.decodeIfPresent(String.self, forKey: .id) ?? ""
		environmentId = try c.decodeIfPresent(String.self, forKey: .environmentId) ?? ""
		region = try c.decodeIfPresent(String.self, forKey: .region) ?? ""
		url = try c.decodeIfPresent(String.self, forKey: .url) ?? ""
		tenants = try c.decodeIfPresent([String: OrgProductTenant].self, forKey: .tenants) ?? [:]
		if environmentId.trimmed.isEmpty, !id.trimmed.isEmpty {
			environmentId = id
		}
	}
}

struct OrgRoot: Codable, Equatable {
	var id: String = ""
	var environments: [String: OrgEnvironment] = [:]

	init() {}

	init(from decoder: Decoder) throws {
		let c = try decoder.container(keyedBy: CodingKeys.self)
		id = try c.decodeIfPresent(String.self, forKey: .id) ?? ""
		environments = try c.decodeIfPresent([String: OrgEnvironment].self, forKey: .environments) ?? [:]
	}
}

/// One node in the org → environment → product-tenant tree for the Config UI.
struct WorkplaceTreeNode: Identifiable {
	var id: String { name }
	var name: String
	var kind: String
	var url: String
	var org: String
	var env: String
	var workplace: Workplace
	var children: [WorkplaceTreeNode]
	var canDelete: Bool

	/// OutlineGroup expects nil for leaf nodes.
	var outlineChildren: [WorkplaceTreeNode]? {
		children.isEmpty ? nil : children
	}
}

/// Mint-time view of a linked workplace + auth (also decodes v1 agent-config rows).
struct Integration: Codable, Identifiable, Equatable {
	var id: UUID = UUID()
	var context: String = ""
	var url: String = ""
	var region: String = ""
	var environmentID: String = ""
	var tenantID: String = ""
	var authName: String = ""
	var authType: String = "oauth"
	var clientID: String = ""
	var username: String = ""
	var scope: String = ""
	var access: String = ""

	var secretKind: AgentKeychain.Kind {
		authType == "basic" ? .password : .clientSecret
	}

	var hasSecret: Bool {
		guard !authName.isEmpty else { return false }
		return AgentKeychain.hasAny(auth: authName)
	}

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

enum AgentConfigTree {
	static func build(
		workplaces: [String: Workplace], orgs: [String: OrgRoot]
	) -> [WorkplaceTreeNode] {
		if orgs.isEmpty {
			return nestedTreeFromWorkplaces(workplaces)
		}
		var roots: [WorkplaceTreeNode] = []
		var seen: Set<String> = []
		for orgName in orgs.keys.sorted() {
			let org = orgs[orgName] ?? OrgRoot()
			var root = node(
				name: orgName, kind: "organization", org: orgName, env: "",
				workplace: workplaces[orgName] ?? Workplace())
			for envName in org.environments.keys.sorted() {
				let env = org.environments[envName] ?? OrgEnvironment()
				let kind =
					env.effectiveEnvironmentID.isEmpty && !env.url.trimmed.isEmpty
					&& env.tenants.isEmpty ? "instance" : "environment"
				var envNode = node(
					name: envName, kind: kind, org: orgName, env: envName,
					workplace: mergedWorkplace(
						name: envName, org: orgName, stored: workplaces[envName],
						url: env.url, region: env.region, environmentId: env.effectiveEnvironmentID))
				for tenName in env.tenants.keys.sorted() {
					let ten = env.tenants[tenName] ?? OrgProductTenant()
					let wp = mergedWorkplace(
						name: tenName, org: orgName, stored: workplaces[tenName],
						url: ten.effectiveURL, tenantId: ten.effectiveTenantID)
					var tenNode = node(
						name: tenName, kind: "product-tenant", org: orgName, env: envName,
						workplace: wp)
					tenNode.canDelete = true
					envNode.children.append(tenNode)
					seen.insert(tenName)
				}
				envNode.canDelete = envNode.children.isEmpty
				root.children.append(envNode)
				seen.insert(envName)
			}
			root.canDelete = root.children.isEmpty
			roots.append(root)
			seen.insert(orgName)
		}
		for row in flatTreeRows(workplaces: workplaces) where !seen.contains(row.name) {
			roots.append(row)
		}
		return roots
	}

	/// Builds a nested tree from flat workplaces using org/env metadata and
	/// cred-list order (org → environment → tenant blocks).
	private static func nestedTreeFromWorkplaces(_ workplaces: [String: Workplace]) -> [WorkplaceTreeNode] {
		if workplaces.isEmpty { return [] }
		let order = workplaceTreeOrder(workplaces: workplaces)
		var roots: [MutableTree] = []
		var orgIndex: [String: Int] = [:]
		var lastOrg = ""
		var lastEnv = ""

		for name in order {
			let wp = workplaces[name] ?? Workplace()
			let displayKind = wp.displayKind
			let kind = wp.workplaceKind()
			var org = wp.org.trimmed
			if wp.isOrganization { org = name }
			let parentEnv = parentEnvironment(workplaces: workplaces, name: name)
			var mut = MutableTree(
				name: name, workplace: wp, kind: displayKind, org: org, env: parentEnv)

			switch kind {
			case "organization":
				orgIndex[name] = roots.count
				lastOrg = name
				lastEnv = ""
				roots.append(mut)
			case "environment", "instance":
				lastEnv = name
				let parentOrg = org.isEmpty ? lastOrg : org
				if !parentOrg.isEmpty, let oi = orgIndex[parentOrg] {
					mut.org = parentOrg
					mut.env = name
					roots[oi].children.append(mut)
				} else {
					roots.append(mut)
				}
			case "tenant":
				let envName = parentEnv.isEmpty ? lastEnv : parentEnv
				let orgName = org.isEmpty ? lastOrg : org
				mut.org = orgName
				mut.env = envName
				if attachTenant(to: &roots, orgIndex: orgIndex, orgName: orgName, envName: envName, tenant: mut) {
					continue
				}
				if !lastEnv.isEmpty, !lastOrg.isEmpty, let oi = orgIndex[lastOrg],
					appendTenant(&roots[oi].children, envName: lastEnv, tenant: mut)
				{
					continue
				}
				roots.append(mut)
			default:
				roots.append(mut)
			}
		}
		return roots.map { finalizeNode($0) }
	}

	private struct MutableTree {
		var name: String
		var workplace: Workplace
		var kind: String
		var org: String
		var env: String
		var children: [MutableTree] = []
	}

	private static func attachTenant(
		to roots: inout [MutableTree], orgIndex: [String: Int],
		orgName: String, envName: String, tenant: MutableTree
	) -> Bool {
		guard !envName.isEmpty else { return false }
		if !orgName.isEmpty, let oi = orgIndex[orgName] {
			return appendTenant(&roots[oi].children, envName: envName, tenant: tenant)
		}
		for i in roots.indices where roots[i].name == envName {
			roots[i].children.append(tenant)
			return true
		}
		return false
	}

	private static func appendTenant(
		_ siblings: inout [MutableTree], envName: String, tenant: MutableTree
	) -> Bool {
		for i in siblings.indices where siblings[i].name == envName {
			siblings[i].children.append(tenant)
			return true
		}
		return false
	}

	private static func finalizeNode(_ mut: MutableTree) -> WorkplaceTreeNode {
		var node = node(
			name: mut.name, kind: mut.kind, org: mut.org, env: mut.env, workplace: mut.workplace)
		node.children = mut.children.map { finalizeNode($0) }
		node.canDelete = node.children.isEmpty
		return node
	}

	private static func flatTreeRows(workplaces: [String: Workplace]) -> [WorkplaceTreeNode] {
		let order = workplaceTreeOrder(workplaces: workplaces)
		return order.map { name in
			let wp = workplaces[name] ?? Workplace()
			let kind = wp.displayKind
			var org = wp.org.trimmed
			if wp.isOrganization { org = name }
			return node(
				name: name, kind: kind, org: org, env: parentEnvironment(workplaces: workplaces, name: name),
				workplace: wp)
		}
	}

	private static func node(
		name: String, kind: String, org: String, env: String, workplace: Workplace,
		orgID: String = ""
	) -> WorkplaceTreeNode {
		WorkplaceTreeNode(
			name: name, kind: kind, url: workplace.url.trimmed, org: org, env: env,
			workplace: workplace, children: [], canDelete: true)
	}

	private static func mergedWorkplace(
		name: String, org: String, stored: Workplace?, url: String = "", region: String = "",
		environmentId: String = "", tenantId: String = ""
	) -> Workplace {
		var wp = stored ?? Workplace()
		wp.org = org
		if !url.trimmed.isEmpty { wp.url = url.trimmed }
		if !region.trimmed.isEmpty { wp.region = region.trimmed }
		if !environmentId.trimmed.isEmpty { wp.environmentId = environmentId.trimmed }
		if !tenantId.trimmed.isEmpty { wp.tenantId = tenantId.trimmed }
		return wp
	}

	private static func workplaceTreeOrder(workplaces: [String: Workplace]) -> [String] {
		var orgs: [String] = []
		var envs: [String] = []
		var product: [String] = []
		var inst: [String] = []
		var other: [String] = []
		for (name, wp) in workplaces {
			switch wp.workplaceKind() {
			case "organization": orgs.append(name)
			case "environment": envs.append(name)
			case "tenant": product.append(name)
			case "instance": inst.append(name)
			default: other.append(name)
			}
		}
		orgs.sort()
		envs.sort()
		product.sort()
		inst.sort()
		other.sort()

		var parentEnv: [String: String] = [:]
		var tenantsByEnv: [String: [String]] = [:]
		var orphanProduct: [String] = []
		for n in product {
			let p = parentEnvironment(workplaces: workplaces, name: n)
			parentEnv[n] = p
			if !p.isEmpty {
				tenantsByEnv[p, default: []].append(n)
			} else {
				orphanProduct.append(n)
			}
		}
		for k in tenantsByEnv.keys {
			tenantsByEnv[k]?.sort()
		}

		var envsByOrg: [String: [String]] = [:]
		var rootEnvs: [String] = []
		for n in envs {
			let o = workplaces[n]?.org.trimmed ?? ""
			if o.isEmpty {
				rootEnvs.append(n)
			} else {
				envsByOrg[o, default: []].append(n)
			}
		}
		for k in envsByOrg.keys {
			envsByOrg[k]?.sort()
		}
		rootEnvs.sort()

		var orgSet = Set(orgs)
		for o in envsByOrg.keys { orgSet.insert(o) }
		for n in orphanProduct {
			if let o = workplaces[n]?.org.trimmed, !o.isEmpty { orgSet.insert(o) }
		}
		let orgNames = orgSet.sorted()

		var order: [String] = []
		var emitted: Set<String> = []
		func emit(_ name: String) {
			guard !name.isEmpty, !emitted.contains(name), workplaces[name] != nil else { return }
			emitted.insert(name)
			order.append(name)
		}
		for org in orgNames {
			if workplaces[org]?.workplaceKind() == "organization" {
				emit(org)
			}
			for env in envsByOrg[org] ?? [] {
				emit(env)
				for n in tenantsByEnv[env] ?? [] { emit(n) }
			}
			for n in orphanProduct where workplaces[n]?.org.trimmed == org {
				emit(n)
			}
		}
		for env in rootEnvs {
			emit(env)
			for n in tenantsByEnv[env] ?? [] { emit(n) }
		}
		for n in orphanProduct { emit(n) }
		for n in inst { emit(n) }
		for n in other { emit(n) }
		let rest = workplaces.keys.filter { !emitted.contains($0) }.sorted()
		for n in rest { emit(n) }
		return order
	}

	private static func parentEnvironment(workplaces: [String: Workplace], name: String) -> String {
		guard let wp = workplaces[name], wp.workplaceKind() == "tenant" else { return "" }
		let eid = wp.effectiveEnvironmentID
		guard !eid.isEmpty else { return "" }
		let org = wp.org.trimmed
		var matches: [String] = []
		for (n, o) in workplaces where n != name && o.workplaceKind() == "environment" {
			if o.effectiveEnvironmentID != eid { continue }
			if !org.isEmpty && !o.org.trimmed.isEmpty && o.org.trimmed != org { continue }
			matches.append(n)
		}
		return matches.sorted().first ?? ""
	}
}

extension OrgEnvironment {
	var effectiveEnvironmentID: String {
		if !id.trimmed.isEmpty { return id.trimmed }
		return environmentId.trimmed
	}
}

extension OrgProductTenant {
	var effectiveTenantID: String {
		if !tenantId.trimmed.isEmpty { return tenantId.trimmed }
		return jamfpro.tenantId.trimmed
	}

	var effectiveURL: String {
		if !url.trimmed.isEmpty { return url.trimmed }
		return jamfpro.url.trimmed
	}
}

/// Secrets-free identity export aligned with buddy `setup export` / `marshalConfig`:
/// org tree is primary; flat `contexts` only carries leftovers; tenant nodes never
/// carry `environmentId` (that lives on the parent environment in `orgs:`).
enum AgentConfigExport {
	static func identityDocument(
		defaultContext: String,
		workplaces: [String: Workplace],
		orgs: [String: OrgRoot],
		auths: [String: AgentAuth],
		features: AgentFeatures,
		channels: [String: ChannelPolicy],
		includeAuths: Bool
	) -> [String: Any] {
		let orgMap = effectiveOrgs(workplaces: workplaces, orgs: orgs)
		let implied = impliedWorkplaces(from: orgMap)
		var doc: [String: Any] = [:]
		let def = defaultContext.trimmed.isEmpty
			? (workplaces.keys.sorted().first ?? "") : defaultContext.trimmed
		if !def.isEmpty {
			doc["context"] = ["default": def.trimmed]
		}
		if let feat = encodeFeatures(features) {
			doc["features"] = feat
		}
		if let channelsObj = encodeChannels(channels) {
			doc["channels"] = channelsObj
		}
		let orgsObj = encodeOrgsMap(orgMap)
		if !orgsObj.isEmpty {
			doc["orgs"] = orgsObj
		}
		var contexts: [String: Any] = [:]
		for (name, stored) in workplaces.sorted(by: { $0.key < $1.key }) {
			if let impliedWP = implied[name], workplaceIdentityEqual(stored, impliedWP),
				stored.denyBuiltins.isEmpty && stored.allowBuiltins.isEmpty
			{
				continue
			}
			let entry = encodeContextWorkplace(stored)
			if !entry.isEmpty {
				contexts[name] = entry
			}
		}
		if !contexts.isEmpty {
			doc["contexts"] = contexts
		}
		if includeAuths {
			var authOut: [String: Any] = [:]
			for (name, row) in auths.sorted(by: { $0.key < $1.key }) {
				var auth: [String: Any] = [
					"type": row.type,
					"contexts": row.contexts,
				]
				if !row.clientId.trimmed.isEmpty { auth["clientId"] = row.clientId.trimmed }
				if !row.username.trimmed.isEmpty { auth["username"] = row.username.trimmed }
				if !row.scope.trimmed.isEmpty { auth["scope"] = row.scope.trimmed }
				if !row.access.trimmed.isEmpty { auth["access"] = row.access.trimmed }
				let ch = AgentConfigLists.normalized(row.channels)
				if !ch.isEmpty { auth["channels"] = ch }
				appendBuiltinList(&auth, key: "deny_builtins", patterns: row.denyBuiltins)
				appendBuiltinList(&auth, key: "allow_builtins", patterns: row.allowBuiltins)
				authOut[name] = auth
			}
			if !authOut.isEmpty { doc["auths"] = authOut }
		}
		return doc
	}

	static func credBackupPayload(
		defaultContext: String,
		workplaces: [String: Workplace],
		orgs: [String: OrgRoot],
		auths: [String: AgentAuth],
		features: AgentFeatures,
		channels: [String: ChannelPolicy],
		secrets: [String: String]
	) throws -> [String: Any] {
		let identity = identityDocument(
			defaultContext: defaultContext,
			workplaces: workplaces,
			orgs: orgs,
			auths: auths,
			features: features,
			channels: channels,
			includeAuths: false)
		if identity.isEmpty && auths.isEmpty {
			throw AgentBackupError.corrupt("nothing to backup")
		}
		var payload: [String: Any] = [:]
		if let ctx = identity["context"] as? [String: Any],
			let def = ctx["default"] as? String, !def.isEmpty
		{
			payload["tenant"] = ["Default": def]
		}
		if let orgsObj = identity["orgs"] {
			payload["orgs"] = orgsObj
		}
		if let contexts = identity["contexts"] as? [String: Any], !contexts.isEmpty {
			payload["tenants"] = encodeBackupTenants(contexts)
		}
		var authOut: [String: Any] = [:]
		for (name, row) in auths.sorted(by: { $0.key < $1.key }) {
			var auth: [String: Any] = ["Type": row.type]
			if !row.clientId.trimmed.isEmpty { auth["ClientID"] = row.clientId.trimmed }
			if !row.username.trimmed.isEmpty { auth["Username"] = row.username.trimmed }
			if !row.scope.trimmed.isEmpty { auth["Scope"] = row.scope.trimmed }
			if !row.access.trimmed.isEmpty { auth["Access"] = row.access.trimmed }
			if !row.contexts.isEmpty { auth["Tenants"] = row.contexts }
			if let secret = secrets[name], !secret.isEmpty {
				if row.type == "basic" {
					auth["Password"] = secret
				} else {
					auth["ClientSecret"] = secret
				}
			}
			authOut[name] = auth
		}
		if !authOut.isEmpty { payload["auths"] = authOut }
		if payload["tenants"] == nil && payload["orgs"] == nil && payload["auths"] == nil {
			throw AgentBackupError.corrupt("nothing to backup")
		}
		return payload
	}

	// MARK: org tree

	static func effectiveOrgs(
		workplaces: [String: Workplace], orgs: [String: OrgRoot]
	) -> [String: OrgRoot] {
		let tree = AgentConfigTree.build(workplaces: workplaces, orgs: orgs)
		var rebuilt: [String: OrgRoot] = [:]
		for root in tree where root.kind == "organization" {
			var orgRoot = orgs[root.name] ?? OrgRoot()
			if let wp = workplaces[root.name], !wp.environmentId.trimmed.isEmpty {
				orgRoot.id = wp.effectiveEnvironmentID
			}
			for child in root.children {
				guard child.kind == "environment" || child.kind == "instance" else { continue }
				let wp = workplaces[child.name] ?? child.workplace
				var env = orgRoot.environments[child.name] ?? OrgEnvironment()
				env.region = wp.region
				let eid = wp.effectiveEnvironmentID
				if !eid.isEmpty {
					env.environmentId = eid
					env.id = eid
				}
				env.url = wp.url
				for tenChild in child.children where tenChild.kind == "product-tenant" {
					let twp = workplaces[tenChild.name] ?? tenChild.workplace
					var ten = env.tenants[tenChild.name] ?? OrgProductTenant()
					if !twp.tenantId.trimmed.isEmpty { ten.tenantId = twp.tenantId.trimmed }
					if !twp.url.trimmed.isEmpty { ten.url = twp.url.trimmed }
					env.tenants[tenChild.name] = ten
				}
				orgRoot.environments[child.name] = env
			}
			rebuilt[root.name] = orgRoot
		}
		if !rebuilt.isEmpty { return rebuilt }
		return orgs
	}

	static func encodeOrgsMap(_ orgMap: [String: OrgRoot]) -> [String: Any] {
		var out: [String: Any] = [:]
		for (orgName, org) in orgMap.sorted(by: { $0.key < $1.key }) {
			var orgObj: [String: Any] = [:]
			if !org.id.trimmed.isEmpty { orgObj["id"] = org.id.trimmed }
			var envs: [String: Any] = [:]
			for (envName, env) in org.environments.sorted(by: { $0.key < $1.key }) {
				var envObj: [String: Any] = [:]
				let eid = env.effectiveEnvironmentID
				if !eid.isEmpty { envObj["id"] = eid }
				if !env.region.trimmed.isEmpty { envObj["region"] = env.region.trimmed }
				if !env.url.trimmed.isEmpty { envObj["url"] = env.url.trimmed }
				var tens: [String: Any] = [:]
				for (tenName, ten) in env.tenants.sorted(by: { $0.key < $1.key }) {
					var tenObj: [String: Any] = [:]
					if !ten.tenantId.trimmed.isEmpty { tenObj["tenantId"] = ten.tenantId.trimmed }
					if !ten.url.trimmed.isEmpty { tenObj["url"] = ten.url.trimmed }
					tens[tenName] = tenObj
				}
				if !tens.isEmpty { envObj["tenants"] = tens }
				envs[envName] = envObj
			}
			if !envs.isEmpty { orgObj["environments"] = envs }
			out[orgName] = orgObj
		}
		return out
	}

	// MARK: flat contexts

	static func impliedWorkplaces(from orgMap: [String: OrgRoot]) -> [String: Workplace] {
		var out: [String: Workplace] = [:]
		for (orgName, org) in orgMap {
			out[orgName] = Workplace()
			for (envName, env) in org.environments {
				var wp = Workplace()
				wp.org = orgName
				wp.region = env.region
				wp.environmentId = env.effectiveEnvironmentID
				wp.url = env.url
				out[envName] = wp
				for (tenName, ten) in env.tenants {
					var twp = Workplace()
					twp.org = orgName
					twp.tenantId = ten.effectiveTenantID
					twp.url = ten.effectiveURL
					out[tenName] = twp
				}
			}
		}
		return out
	}

	/// Whether a flat workplace row duplicates a node already expressed under `orgs:`.
	/// Parent org is carried by the tree, not compared on flat rows (buddy omits those
	/// implied contexts even when `org:` was never written on the workplace).
	static func workplaceIdentityEqual(_ a: Workplace, _ b: Workplace) -> Bool {
		let kindA = a.workplaceKind()
		let kindB = b.workplaceKind()
		if kindA == "organization" || kindB == "organization" {
			return kindA == "organization" && kindB == "organization"
		}
		if kindA == "tenant" || kindB == "tenant" {
			return normURL(a.url) == normURL(b.url)
				&& a.tenantId.trimmed == b.tenantId.trimmed
		}
		if kindA == "environment" || kindB == "environment" {
			return a.region.trimmed == b.region.trimmed
				&& a.effectiveEnvironmentID == b.effectiveEnvironmentID
				&& normURL(a.url) == normURL(b.url)
		}
		return normURL(a.url) == normURL(b.url)
			&& a.region.trimmed == b.region.trimmed
			&& a.effectiveEnvironmentID == b.effectiveEnvironmentID
			&& a.tenantId.trimmed == b.tenantId.trimmed
	}

	static func encodeContextWorkplace(_ wp: Workplace) -> [String: Any] {
		var entry: [String: Any] = [:]
		let kind = wp.workplaceKind()
		if kind != "organization", !wp.org.trimmed.isEmpty {
			entry["org"] = wp.org.trimmed
		}
		switch kind {
		case "organization":
			break
		case "environment":
			if !wp.region.trimmed.isEmpty { entry["region"] = wp.region.trimmed }
			if !wp.effectiveEnvironmentID.isEmpty {
				entry["environmentId"] = wp.effectiveEnvironmentID
			}
		case "tenant":
			if !wp.tenantId.trimmed.isEmpty || !wp.url.trimmed.isEmpty {
				var jamf: [String: Any] = [:]
				if !wp.url.trimmed.isEmpty { jamf["url"] = normURL(wp.url) }
				if !wp.tenantId.trimmed.isEmpty { jamf["tenantId"] = wp.tenantId.trimmed }
				if !jamf.isEmpty { entry["jamfpro"] = jamf }
			}
		case "instance":
			if !wp.url.trimmed.isEmpty { entry["url"] = normURL(wp.url) }
		default:
			if !wp.url.trimmed.isEmpty { entry["url"] = normURL(wp.url) }
			if !wp.region.trimmed.isEmpty { entry["region"] = wp.region.trimmed }
			if !wp.effectiveEnvironmentID.isEmpty {
				entry["environmentId"] = wp.effectiveEnvironmentID
			}
			if !wp.tenantId.trimmed.isEmpty { entry["tenantId"] = wp.tenantId.trimmed }
		}
		appendBuiltinList(&entry, key: "deny_builtins", patterns: wp.denyBuiltins)
		appendBuiltinList(&entry, key: "allow_builtins", patterns: wp.allowBuiltins)
		return entry
	}

	static func encodeFeatures(_ features: AgentFeatures) -> [String: Any]? {
		guard features.classicInstanceOnly else { return nil }
		return ["classic_instance_only": true]
	}

	static func decodeFeatures(_ raw: [String: Any]?) -> AgentFeatures {
		guard let raw else { return AgentFeatures() }
		var f = AgentFeatures()
		if let v = raw["classic_instance_only"] as? Bool {
			f.classicInstanceOnly = v
		} else if let v = raw["classic_instance_only"] as? Int {
			f.classicInstanceOnly = v != 0
		}
		return f
	}

	static func decodeChannels(_ raw: [String: Any]?) -> [String: ChannelPolicy] {
		guard let raw else { return [:] }
		var out: [String: ChannelPolicy] = [:]
		for channel in BuddyChannel.allCases {
			guard let entry = raw[channel.rawValue] as? [String: Any] else { continue }
			out[channel.rawValue] = decodeChannelPolicy(entry)
		}
		return out
	}

	static func decodeChannelPolicy(_ entry: [String: Any]) -> ChannelPolicy {
		var p = ChannelPolicy()
		if let access = entry["access"] as? String { p.access = access.trimmed }
		let def = (entry["context"] as? String) ?? (entry["tenant"] as? String) ?? ""
		p.context = def.trimmed
		p.contexts = AgentConfigLists.parse(entry["contexts"] ?? entry["tenants"])
		p.denyBuiltins = AgentConfigLists.parse(entry["deny_builtins"])
		p.allowBuiltins = AgentConfigLists.parse(entry["allow_builtins"])
		return p
	}

	static func encodeChannels(_ channels: [String: ChannelPolicy]) -> [String: Any]? {
		var out: [String: Any] = [:]
		for channel in BuddyChannel.allCases {
			guard let policy = channels[channel.rawValue], !policy.isEmpty else { continue }
			out[channel.rawValue] = encodeChannelPolicy(policy)
		}
		return out.isEmpty ? nil : out
	}

	static func encodeChannelPolicy(_ policy: ChannelPolicy) -> [String: Any] {
		var entry: [String: Any] = [:]
		if policy.access.trimmed == "readonly" { entry["access"] = "readonly" }
		if !policy.context.trimmed.isEmpty { entry["context"] = policy.context.trimmed }
		let contexts = AgentConfigLists.normalized(policy.contexts)
		if !contexts.isEmpty { entry["contexts"] = contexts }
		appendBuiltinList(&entry, key: "deny_builtins", patterns: policy.denyBuiltins)
		appendBuiltinList(&entry, key: "allow_builtins", patterns: policy.allowBuiltins)
		return entry
	}

	static func appendBuiltinList(_ dict: inout [String: Any], key: String, patterns: [String]) {
		let items = AgentConfigLists.normalized(patterns)
		if !items.isEmpty { dict[key] = items }
	}

	private static func encodeBackupTenants(_ contexts: [String: Any]) -> [String: Any] {
		var out: [String: Any] = [:]
		for (name, raw) in contexts.sorted(by: { $0.key < $1.key }) {
			guard let entry = raw as? [String: Any] else { continue }
			var row: [String: Any] = [:]
			if let org = entry["org"] as? String, !org.isEmpty { row["Org"] = org }
			if let region = entry["region"] as? String, !region.isEmpty { row["Region"] = region }
			if let id = entry["environmentId"] as? String, !id.isEmpty {
				row["EnvironmentID"] = id
			}
			if let url = entry["url"] as? String, !url.isEmpty { row["URL"] = url }
			if let jamf = entry["jamfpro"] as? [String: Any] {
				if let url = jamf["url"] as? String, !url.isEmpty { row["URL"] = url }
				if let tid = jamf["tenantId"] as? String, !tid.isEmpty { row["TenantID"] = tid }
			}
			if let tid = entry["tenantId"] as? String, !tid.isEmpty { row["TenantID"] = tid }
			out[name] = row
		}
		return out
	}

	private static func normURL(_ s: String) -> String {
		var t = s.trimmed
		while t.hasSuffix("/") { t.removeLast() }
		return t
	}
}
