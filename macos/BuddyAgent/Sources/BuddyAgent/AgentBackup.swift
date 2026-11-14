import CommonCrypto
import CryptoKit
import Foundation
import Security

/// Setup document shared by both Agent exports. Config-only is this JSON.
/// Cred backup is the same document plus an encrypted `secrets` block.
/// Older `buddy cred backup` files are still a single encrypted envelope.
enum AgentBackup {
	/// What an imported backup contributes: a document shaped like the
	/// `setup export --json` identity file, plus one secret per auth.
	struct Contents {
		var document: [String: Any]
		var secrets: [String: String]
	}

	/// What a setup file contains before any passphrase is tried.
	struct Inspection {
		var sealed: Bool
		var hasWorkplaces: Bool
		var hasAuths: Bool
		var hasPolicy: Bool
		var hasSecrets: Bool
	}

	/// Reads the cleartext shape of a setup JSON file. A legacy whole-file
	/// cred backup is `sealed` and its sections stay unknown until `read`.
	static func inspect(_ raw: Data) throws -> Inspection {
		guard let root = try JSONSerialization.jsonObject(with: raw) as? [String: Any] else {
			throw AgentBackupError.corrupt("file is not a JSON object")
		}
		if (root["buddy"] as? String) == kind {
			return Inspection(
				sealed: true, hasWorkplaces: false, hasAuths: false, hasPolicy: false, hasSecrets: false)
		}
		let found = sections(in: root, secrets: [:])
		var inspection = found
		inspection.hasSecrets = root["secrets"] is [String: Any]
		if !inspection.hasWorkplaces && !inspection.hasAuths && !inspection.hasPolicy && !inspection.hasSecrets {
			throw AgentBackupError.corrupt("no workplaces, auths, policy, or secrets")
		}
		return inspection
	}

	/// Sections present in an already-decoded setup document.
	static func sections(in document: [String: Any], secrets: [String: String]) -> Inspection {
		let selection = (document["context"] as? [String: Any]) ?? (document["tenant"] as? [String: Any])
		let hasDefault = (selection?["default"] as? String)?.trimmed.isEmpty == false
		let hasWorkplaces =
			!(document["orgs"] as? [String: Any] ?? [:]).isEmpty
			|| !(document["contexts"] as? [String: Any] ?? [:]).isEmpty
			|| !(document["tenants"] as? [String: Any] ?? [:]).isEmpty
			|| hasDefault
		let hasAuths = !(document["auths"] as? [String: Any] ?? [:]).isEmpty
		let hasPolicy = document["features"] != nil || document["channels"] != nil
		return Inspection(
			sealed: false,
			hasWorkplaces: hasWorkplaces,
			hasAuths: hasAuths,
			hasPolicy: hasPolicy,
			hasSecrets: !secrets.isEmpty)
	}

	static func read(_ raw: Data, passphrase: String) throws -> Contents {
		guard let root = try JSONSerialization.jsonObject(with: raw) as? [String: Any] else {
			throw AgentBackupError.corrupt("payload is not an object")
		}
		// Older files are one envelope around a Go cred-backup payload.
		if (root["buddy"] as? String) == kind {
			let plain = try decrypt(raw, passphrase: passphrase)
			guard let payload = try JSONSerialization.jsonObject(with: plain) as? [String: Any] else {
				throw AgentBackupError.corrupt("payload is not an object")
			}
			return normalise(payload)
		}
		// Same document as config-only export. Secrets, when present, are one block.
		var document = root
		var secrets: [String: String] = [:]
		if let block = root["secrets"] as? [String: Any] {
			let blockData = try JSONSerialization.data(withJSONObject: block)
			let plain = try decrypt(blockData, passphrase: passphrase)
			secrets = decodeSecretMap(plain)
			document.removeValue(forKey: "secrets")
		}
		return Contents(document: document, secrets: secrets)
	}

	private static func decodeSecretMap(_ plain: Data) -> [String: String] {
		guard let obj = try? JSONSerialization.jsonObject(with: plain) else { return [:] }
		if let typed = obj as? [String: String] {
			return typed.filter { !$0.value.trimmed.isEmpty }
		}
		guard let raw = obj as? [String: Any] else { return [:] }
		var out: [String: String] = [:]
		for (name, value) in raw {
			if let secret = value as? String, !secret.trimmed.isEmpty {
				out[name] = secret
			}
		}
		return out
	}

	/// Same PBKDF2 iteration count as `buddy cred backup` (600_000).
	static let defaultIterations = 600_000

	struct WriteResult {
		var auths: Int
		var missingSecrets: [String]
	}

	/// Builds the same document as a config-only export, plus an encrypted
	/// `secrets` block. Identity and policy stay in the clear; only secret
	/// values are sealed.
	static func write(
		defaultContext: String,
		workplaces: [String: Workplace],
		orgs: [String: OrgRoot],
		auths: [String: AgentAuth],
		features: AgentFeatures,
		channels: [String: ChannelPolicy],
		secrets: [String: String],
		passphrase: String,
		includeWorkplaces: Bool = true,
		includeAuths: Bool = true,
		includePolicy: Bool = true,
		includeSecrets: Bool = true
	) throws -> (data: Data, result: WriteResult) {
		let authsInFile = includeAuths || includeSecrets
		var doc = AgentConfigExport.exportDocument(
			defaultContext: defaultContext,
			workplaces: workplaces,
			orgs: orgs,
			auths: auths,
			features: features,
			channels: channels,
			includeWorkplaces: includeWorkplaces,
			includeAuths: authsInFile,
			includePolicy: includePolicy)
		let secretMap = includeSecrets ? secrets.filter { !$0.value.trimmed.isEmpty } : [:]
		if includeSecrets {
			let plain = try JSONSerialization.data(withJSONObject: secretMap)
			let envelopeData = try encrypt(plain, passphrase: passphrase)
			guard let envelope = try JSONSerialization.jsonObject(with: envelopeData) as? [String: Any] else {
				throw AgentBackupError.corrupt("secrets block is not an object")
			}
			doc["secrets"] = envelope
		}
		if doc.isEmpty {
			throw AgentBackupError.corrupt("nothing to export")
		}
		guard JSONSerialization.isValidJSONObject(doc) else {
			throw AgentBackupError.corrupt("backup document is not JSON-serializable")
		}
		var data = try JSONSerialization.data(withJSONObject: doc, options: [.prettyPrinted, .sortedKeys])
		data.append(0x0A)
		let missing = includeSecrets
			? auths.keys.sorted().compactMap { name -> String? in
				guard let auth = auths[name], auth.type != "command" else { return nil }
				if secretMap[name] != nil { return nil }
				return auth.type == "basic" ? "\(name) password" : "\(name) client-secret"
			}
			: []
		return (data, WriteResult(auths: authsInFile ? auths.count : 0, missingSecrets: missing))
	}

	// MARK: envelope

	private struct Envelope: Decodable {
		let buddy: String
		let v: Int
		let kdf: String
		let iter: Int?
		let salt: String?
		let data: String
	}

	private static let kind = "cred-backup"
	private static let aad = "buddy-cred-backup-v1"

	static func encrypt(_ plain: Data, passphrase: String, iterations: Int = defaultIterations) throws -> Data {
		guard iterations >= 1000, iterations <= 10_000_000 else {
			throw AgentBackupError.corrupt("invalid kdf iteration count")
		}
		let salt = try randomBytes(count: 16)
		let key = try pbkdf2(passphrase: passphrase, salt: salt, iterations: iterations)
		let nonce = try randomBytes(count: 12)
		let box = try AES.GCM.seal(
			plain,
			using: SymmetricKey(data: key),
			nonce: AES.GCM.Nonce(data: nonce),
			authenticating: Data(aad.utf8))
		let sealed = nonce + box.ciphertext + box.tag
		let envelope: [String: Any] = [
			"buddy": kind,
			"v": 1,
			"kdf": "pbkdf2-sha256",
			"iter": iterations,
			"salt": salt.base64EncodedString(),
			"data": sealed.base64EncodedString(),
		]
		var raw = try JSONSerialization.data(withJSONObject: envelope, options: [.prettyPrinted, .sortedKeys])
		raw.append(0x0A)
		return raw
	}

	static func decrypt(_ raw: Data, passphrase: String) throws -> Data {
		guard let env = try? JSONDecoder().decode(Envelope.self, from: raw) else {
			throw AgentBackupError.notABackup
		}
		guard env.buddy == kind, env.v == 1 else {
			throw AgentBackupError.notABackup
		}
		guard env.kdf == "pbkdf2-sha256" else {
			throw AgentBackupError.corrupt("unsupported kdf \(env.kdf)")
		}
		let iterations = env.iter ?? 0
		guard iterations >= 1000, iterations <= 10_000_000 else {
			throw AgentBackupError.corrupt("invalid kdf iteration count")
		}
		guard let saltText = env.salt, let salt = Data(base64Encoded: saltText), salt.count >= 8
		else {
			throw AgentBackupError.corrupt("invalid salt")
		}
		guard let sealed = Data(base64Encoded: env.data), sealed.count > 12 + 16 else {
			throw AgentBackupError.corrupt("invalid ciphertext")
		}
		let key = try pbkdf2(passphrase: passphrase, salt: salt, iterations: iterations)
		let nonce = sealed.prefix(12)
		let body = sealed.dropFirst(12).dropLast(16)
		let tag = sealed.suffix(16)
		do {
			let box = try AES.GCM.SealedBox(
				nonce: AES.GCM.Nonce(data: nonce), ciphertext: body, tag: tag)
			return try AES.GCM.open(
				box, using: SymmetricKey(data: key), authenticating: Data(aad.utf8))
		} catch {
			throw AgentBackupError.wrongPassphrase
		}
	}

	private static func randomBytes(count: Int) throws -> Data {
		var bytes = [UInt8](repeating: 0, count: count)
		let status = SecRandomCopyBytes(kSecRandomDefault, count, &bytes)
		guard status == errSecSuccess else {
			throw AgentBackupError.corrupt("random bytes failed")
		}
		return Data(bytes)
	}

	private static func pbkdf2(passphrase: String, salt: Data, iterations: Int) throws -> Data {
		var derived = Data(count: 32)
		let status = derived.withUnsafeMutableBytes { out -> Int32 in
			salt.withUnsafeBytes { saltBytes -> Int32 in
				CCKeyDerivationPBKDF(
					CCPBKDFAlgorithm(kCCPBKDF2),
					passphrase, passphrase.utf8.count,
					saltBytes.bindMemory(to: UInt8.self).baseAddress, salt.count,
					CCPseudoRandomAlgorithm(kCCPRFHmacAlgSHA256),
					UInt32(iterations),
					out.bindMemory(to: UInt8.self).baseAddress, 32)
			}
		}
		guard Int(status) == kCCSuccess else {
			throw AgentBackupError.corrupt("key derivation failed")
		}
		return derived
	}

	// MARK: legacy payload

	/// Older `buddy cred backup` bodies are Go structs marshalled without json
	/// tags, so keys are field names (`ClientSecret`, `URL`, …). Lookups are
	/// case-insensitive, and the result is the lowercase identity document.
	static func normalise(_ payload: [String: Any]) -> Contents {
		var contexts: [String: Any] = [:]
		for (name, raw) in payload.dict("tenants") ?? payload.dict("contexts") ?? [:] {
			guard let entry = raw as? [String: Any] else { continue }
			var out: [String: Any] = [:]
			if let url = entry.string("url") { out["url"] = url }
			if let region = entry.string("region") { out["region"] = region }
			if let id = entry.string("environmentId") ?? entry.string("id") {
				out["environmentId"] = id
			}
			if let tid = entry.string("tenantId") { out["tenantId"] = tid }
			contexts[name] = out
		}

		var auths: [String: Any] = [:]
		var secrets: [String: String] = [:]
		for (name, raw) in payload.dict("auths") ?? [:] {
			guard let entry = raw as? [String: Any] else { continue }
			var out: [String: Any] = ["type": entry.string("type") ?? "oauth"]
			if let clientID = entry.string("clientId") { out["clientId"] = clientID }
			if let username = entry.string("username") { out["username"] = username }
			if let scope = entry.string("scope") { out["scope"] = scope }
			if let access = entry.string("access") { out["access"] = access }
			let linked = entry.strings("contexts") ?? entry.strings("tenants") ?? []
			if !linked.isEmpty { out["contexts"] = linked }
			auths[name] = out
			if let secret = entry.string("clientSecret") ?? entry.string("password") {
				secrets[name] = secret
			}
		}

		var document: [String: Any] = [:]
		if !contexts.isEmpty { document["contexts"] = contexts }
		if !auths.isEmpty { document["auths"] = auths }
		if let orgs = payload.dict("orgs"), !orgs.isEmpty { document["orgs"] = orgs }
		if let selection = payload.dict("context") ?? payload.dict("tenant"),
			let def = selection.string("default")
		{
			document["context"] = ["default": def]
		}
		if let feat = payload.dict("features"), !feat.isEmpty {
			document["features"] = feat
		}
		if let ch = payload.dict("channels"), !ch.isEmpty {
			document["channels"] = ch
		}
		return Contents(document: document, secrets: secrets)
	}
}

enum AgentBackupError: Error, CustomStringConvertible {
	case notABackup
	case wrongPassphrase
	case corrupt(String)

	var description: String {
		switch self {
		case .notABackup:
			return "not a buddy cred backup file"
		case .wrongPassphrase:
			return "wrong passphrase, or the file is damaged"
		case .corrupt(let why):
			return "unreadable backup: \(why)"
		}
	}
}

extension [String: Any] {
	/// Case-insensitive lookup: the Go backup writes `ClientSecret`, the
	/// identity export writes `clientSecret`.
	fileprivate func any(_ key: String) -> Any? {
		if let hit = self[key] { return hit }
		let wanted = key.lowercased()
		return first { $0.key.lowercased() == wanted }?.value
	}

	fileprivate func string(_ key: String) -> String? {
		guard let value = any(key) as? String, !value.trimmed.isEmpty else { return nil }
		return value.trimmed
	}

	fileprivate func strings(_ key: String) -> [String]? {
		guard let value = any(key) as? [Any] else { return nil }
		let out = value.compactMap { ($0 as? String)?.trimmed }.filter { !$0.isEmpty }
		return out.isEmpty ? nil : out
	}

	fileprivate func dict(_ key: String) -> [String: Any]? {
		any(key) as? [String: Any]
	}
}
