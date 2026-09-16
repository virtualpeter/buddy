import Foundation
import Security

/// Per-auth secret storage owned by the Agent. One generic-password item per
/// secret, service `virtualpete.buddy.agent`, account `<auth>/<kind>`.
///
/// The buddy CLI never reads these and never writes them: the user types a
/// secret into the Agent GUI, and only the mint path reads it back. A
/// `userPresence` SecAccessControl (real biometrics) needs the signed Xcode
/// target, so the spike stores plain items accessible when unlocked.
enum AgentKeychain {
	static let service = "virtualpete.buddy.agent"

	enum Kind: String, CaseIterable {
		case clientSecret = "client_secret"
		case password
	}

	static func account(auth: String, kind: Kind) -> String {
		"\(auth)/\(kind.rawValue)"
	}

	/// memoryFallback keeps the Agent usable when the keychain refuses writes
	/// (an unsigned build with no code signature at all). Cleared on quit.
	private static var memoryFallback: [String: String] = [:]
	private static let lock = NSLock()
	private(set) static var usingFallback = false

	@discardableResult
	static func set(_ value: String, auth: String, kind: Kind) -> Bool {
		let acct = account(auth: auth, kind: kind)
		let query: [CFString: Any] = [
			kSecClass: kSecClassGenericPassword,
			kSecAttrService: service,
			kSecAttrAccount: acct,
		]
		SecItemDelete(query as CFDictionary)
		var add = query
		add[kSecAttrLabel] = "Buddy Agent (\(auth))"
		add[kSecAttrAccessible] = kSecAttrAccessibleWhenUnlockedThisDeviceOnly
		add[kSecValueData] = Data(value.utf8)
		let status = SecItemAdd(add as CFDictionary, nil)
		if status == errSecSuccess {
			return true
		}
		FileHandle.standardError.write(
			Data("buddy-agent: keychain write failed (OSStatus \(status)); keeping \(acct) in memory only\n".utf8))
		lock.lock()
		memoryFallback[acct] = value
		usingFallback = true
		lock.unlock()
		return false
	}

	static func get(auth: String, kind: Kind) -> String? {
		let acct = account(auth: auth, kind: kind)
		let query: [CFString: Any] = [
			kSecClass: kSecClassGenericPassword,
			kSecAttrService: service,
			kSecAttrAccount: acct,
			kSecReturnData: true,
			kSecMatchLimit: kSecMatchLimitOne,
		]
		var item: CFTypeRef?
		if SecItemCopyMatching(query as CFDictionary, &item) == errSecSuccess,
			let data = item as? Data,
			let value = String(data: data, encoding: .utf8), !value.isEmpty
		{
			return value
		}
		lock.lock()
		defer { lock.unlock() }
		return memoryFallback[acct]
	}

	static func has(auth: String, kind: Kind) -> Bool {
		get(auth: auth, kind: kind) != nil
	}

	static func delete(auth: String, kind: Kind) {
		let acct = account(auth: auth, kind: kind)
		let query: [CFString: Any] = [
			kSecClass: kSecClassGenericPassword,
			kSecAttrService: service,
			kSecAttrAccount: acct,
		]
		SecItemDelete(query as CFDictionary)
		lock.lock()
		memoryFallback.removeValue(forKey: acct)
		lock.unlock()
	}

	static func deleteAll(auth: String) {
		for kind in Kind.allCases {
			delete(auth: auth, kind: kind)
		}
	}

	/// Drops the in-memory fallback copies (Lock / Quit). Keychain items stay.
	static func forgetMemory() {
		lock.lock()
		memoryFallback.removeAll()
		lock.unlock()
	}
}
