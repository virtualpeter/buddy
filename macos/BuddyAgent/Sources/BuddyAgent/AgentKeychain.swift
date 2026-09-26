import Foundation
import LocalAuthentication
import Security

/// Per-auth secret storage owned by the Agent. One generic-password item per
/// secret, service `virtualpete.buddy.agent`, account `<auth>/<kind>`.
///
/// The buddy CLI never reads these and never writes them: the user types a
/// secret into the Agent GUI, and only the mint path reads it back.
///
/// A signed Agent stores each secret in the **data protection** keychain behind
/// a `userPresence` SecAccessControl (Keychain Access shows these under
/// Local Items / iCloud Keychain — not the login keychain). An unsigned build
/// (or one whose ACL probe fails) keeps secrets in process memory only.
enum AgentKeychain {
	static let service = "virtualpete.buddy.agent"

	enum Kind: String, CaseIterable {
		case clientSecret = "client_secret"
		case password
	}

	/// Where secrets live for this process.
	enum Backend {
		/// Data-protection keychain, userPresence ACL on every item.
		case dataProtection
		/// In-memory only (unsigned / no ACL). Cleared on Lock and Quit.
		case memory
	}

	static func account(auth: String, kind: Kind) -> String {
		"\(auth)/\(kind.rawValue)"
	}

	private static var memoryStore: [String: String] = [:]
	private static let lock = NSLock()
	/// True when at least one secret is held only in memory (Status warning).
	private(set) static var usingFallback = false

	/// The app's keychain-access-groups entry, when the profile granted one.
	static let keychainAccessGroup: String? = {
		guard let task = SecTaskCreateFromSelf(nil) else { return nil }
		var error: Unmanaged<CFError>?
		guard let value = SecTaskCopyValueForEntitlement(
			task, "keychain-access-groups" as CFString, &error) as? [String],
			let first = value.first, !first.isEmpty
		else { return nil }
		return first
	}()

	static let backend: Backend = probeBackend()

	private static func probeBackend() -> Backend {
		guard let control = accessControl() else {
			log("no SecAccessControl; memory-only secrets")
			return .memory
		}
		let acct = "__buddy_acl_probe__"
		var add = dpQuery(account: acct)
		add[kSecAttrAccessControl] = control
		add[kSecValueData] = Data("probe".utf8)
		let status = SecItemAdd(add as CFDictionary, nil)
		silentDelete(account: acct)
		if status == errSecSuccess || status == errSecDuplicateItem {
			log("data protection + userPresence ACL available\(keychainAccessGroup.map { " (group \($0))" } ?? "")")
			return .dataProtection
		}
		log("userPresence ACL probe failed (OSStatus \(status)); memory-only secrets")
		return .memory
	}

	/// Wipe pre-publish login-keychain leftovers for this service without
	/// reading them. Explicitly targets the file-based keychain only — never
	/// the data-protection items the Agent actually uses.
	static func purgeLoginLeftovers() {
		var query: [CFString: Any] = [
			kSecClass: kSecClassGenericPassword,
			kSecAttrService: service,
			kSecUseDataProtectionKeychain: false,
			kSecUseAuthenticationUI: kSecUseAuthenticationUIFail,
		]
		let status = SecItemDelete(query as CFDictionary)
		switch status {
		case errSecSuccess:
			log("purged login-keychain leftovers for \(service)")
		case errSecItemNotFound:
			break
		default:
			log("login-keychain purge OSStatus \(status)")
		}
	}

	@discardableResult
	private static func silentDelete(account acct: String) -> OSStatus {
		var query = dpQuery(account: acct)
		query[kSecUseAuthenticationUI] = kSecUseAuthenticationUIFail
		return SecItemDelete(query as CFDictionary)
	}

	private static func dpQuery(account acct: String) -> [CFString: Any] {
		var query: [CFString: Any] = [
			kSecClass: kSecClassGenericPassword,
			kSecAttrService: service,
			kSecAttrAccount: acct,
			kSecUseDataProtectionKeychain: true,
		]
		if let accessGroup = keychainAccessGroup {
			query[kSecAttrAccessGroup] = accessGroup
		}
		return query
	}

	private static func accessControl() -> SecAccessControl? {
		var error: Unmanaged<CFError>?
		let control = SecAccessControlCreateWithFlags(
			nil, kSecAttrAccessibleWhenUnlockedThisDeviceOnly, .userPresence, &error)
		if control == nil {
			let reason = error?.takeRetainedValue().localizedDescription ?? "unknown"
			log("could not build a presence ACL (\(reason))")
		}
		return control
	}

	/// True when a data-protection item exists for this account. Never prompts:
	/// UIFail + InteractionNotAllowed / AuthFailed mean "present but gated".
	private static func keychainExists(account acct: String) -> Bool {
		var query = dpQuery(account: acct)
		query[kSecMatchLimit] = kSecMatchLimitOne
		query[kSecReturnAttributes] = true
		query[kSecUseAuthenticationUI] = kSecUseAuthenticationUIFail
		var item: CFTypeRef?
		let status = SecItemCopyMatching(query as CFDictionary, &item)
		switch status {
		case errSecSuccess, errSecInteractionNotAllowed, errSecAuthFailed, errSecUserCanceled:
			return true
		case errSecItemNotFound:
			return false
		default:
			log("keychainExists(\(acct)) OSStatus \(status); treating as absent")
			return false
		}
	}

	/// Returns true only when the secret is durable in the data-protection
	/// keychain. Memory-only writes return false; `has` still sees them until
	/// Lock/Quit so mint can work for unsigned agent-dev.
	@discardableResult
	static func set(_ value: String, auth: String, kind: Kind, context: LAContext? = nil) -> Bool {
		let acct = account(auth: auth, kind: kind)

		guard backend == .dataProtection, let control = accessControl() else {
			remember(acct, value)
			usingFallback = true
			log("memory write \(acct)")
			return false
		}

		let base = dpQuery(account: acct)
		var deleteStatus = silentDelete(account: acct)
		if deleteStatus != errSecSuccess && deleteStatus != errSecItemNotFound,
			let context
		{
			var authed = base
			authed[kSecUseAuthenticationContext] = context
			deleteStatus = SecItemDelete(authed as CFDictionary)
		}

		var add = base
		add[kSecAttrLabel] = "Buddy Agent (\(auth))"
		add[kSecAttrAccessControl] = control
		add[kSecValueData] = Data(value.utf8)
		var status = SecItemAdd(add as CFDictionary, nil)
		if status == errSecDuplicateItem {
			if let context {
				var updateQuery = base
				updateQuery[kSecUseAuthenticationContext] = context
				let update: [CFString: Any] = [kSecValueData: Data(value.utf8)]
				status = SecItemUpdate(updateQuery as CFDictionary, update as CFDictionary)
			} else {
				// Stale presence-gated item: cannot replace without a session.
				log("ACL write \(acct) blocked by existing item (delete \(deleteStatus)); need unlock")
				remember(acct, value)
				usingFallback = true
				return false
			}
		}
		if status == errSecSuccess, keychainExists(account: acct) {
			forget(acct)
			log("ACL write \(acct) ok")
			return true
		}
		if status == errSecSuccess {
			// SecItemAdd sometimes reports success while the item is not
			// queryable — do not tell the UI the secret is in the keychain.
			log("ACL write \(acct) Add ok but not readable (delete \(deleteStatus)); memory-only")
		} else {
			log("ACL write \(acct) failed (OSStatus \(status), delete \(deleteStatus)); memory-only")
		}
		remember(acct, value)
		usingFallback = true
		return false
	}

	/// Prefer the presence-gated item with UnlockGate's LAContext. Never draw a
	/// Keychain password sheet. Memory is the unsigned/dev path only.
	static func get(auth: String, kind: Kind, context: LAContext? = nil) -> String? {
		let acct = account(auth: auth, kind: kind)
		if backend == .dataProtection, let context {
			if let value = copySecret(account: acct, context: context) {
				return value
			}
		}
		return recalled(acct)
	}

	private static func copySecret(account acct: String, context: LAContext) -> String? {
		var query = dpQuery(account: acct)
		query[kSecUseAuthenticationContext] = context
		query[kSecUseAuthenticationUI] = kSecUseAuthenticationUIFail
		query[kSecReturnData] = true
		query[kSecMatchLimit] = kSecMatchLimitOne
		var item: CFTypeRef?
		let status = SecItemCopyMatching(query as CFDictionary, &item)
		if status == errSecSuccess, let data = item as? Data,
			let value = String(data: data, encoding: .utf8), !value.isEmpty
		{
			return value
		}
		log("\(acct) ACL read OSStatus \(status)")
		return nil
	}

	/// Existence only — never prompts. Safe while the Agent is locked.
	static func has(auth: String, kind: Kind) -> Bool {
		let acct = account(auth: auth, kind: kind)
		if recalled(acct) != nil { return true }
		guard backend == .dataProtection else { return false }
		return keychainExists(account: acct)
	}

	/// True when either secret kind is present (covers basic/oauth mismatches).
	static func hasAny(auth: String) -> Bool {
		Kind.allCases.contains { has(auth: auth, kind: $0) }
	}

	/// True when the secret is in the data-protection keychain (not memory).
	static func storedInKeychain(auth: String, kind: Kind) -> Bool {
		guard backend == .dataProtection else { return false }
		return keychainExists(account: account(auth: auth, kind: kind))
	}

	static func delete(auth: String, kind: Kind, context: LAContext? = nil) {
		let acct = account(auth: auth, kind: kind)
		if backend == .dataProtection {
			if let context {
				var query = dpQuery(account: acct)
				query[kSecUseAuthenticationContext] = context
				query[kSecUseAuthenticationUI] = kSecUseAuthenticationUIFail
				_ = SecItemDelete(query as CFDictionary)
			} else {
				_ = silentDelete(account: acct)
			}
		}
		forget(acct)
	}

	static func deleteAll(auth: String, context: LAContext? = nil) {
		for kind in Kind.allCases {
			delete(auth: auth, kind: kind, context: context)
		}
	}

	/// Drops in-memory copies (Lock / Quit). Keychain items stay.
	static func forgetMemory() {
		lock.lock()
		memoryStore.removeAll()
		protectedMemoryStore.removeAll()
		usingFallback = false
		lock.unlock()
	}

	/// Drops cached protected blobs (e.g. config seal) without touching Keychain.
	static func forgetProtectedMemory(for accounts: [String]) {
		lock.lock()
		for account in accounts {
			protectedMemoryStore.removeValue(forKey: account)
		}
		lock.unlock()
	}

	// MARK: protected binary blobs (config integrity, etc.)

	@discardableResult
	static func setProtectedData(
		_ data: Data, account: String, label: String, context: LAContext? = nil
	) -> Bool {
		guard backend == .dataProtection, let control = accessControl() else {
			rememberProtected(account, data)
			return false
		}

		let base = dpQuery(account: account)
		// Replace the payload on the existing userPresence item. Delete + SecItemAdd
		// mints a new ACL the current unlock LAContext often cannot read back, which
		// leaves a saved config looking unsealed.
		if let context {
			var updateQuery = base
			updateQuery[kSecUseAuthenticationContext] = context
			updateQuery[kSecUseAuthenticationUI] = kSecUseAuthenticationUIFail
			let update: [CFString: Any] = [kSecValueData: data]
			let updateStatus = SecItemUpdate(updateQuery as CFDictionary, update as CFDictionary)
			switch updateStatus {
			case errSecSuccess:
				forgetProtected(account)
				return true
			case errSecItemNotFound:
				break
			default:
				log("protected update \(account) failed (OSStatus \(updateStatus))")
				rememberProtected(account, data)
				return false
			}
		} else {
			let deleteStatus = silentDelete(account: account)
			if deleteStatus != errSecSuccess && deleteStatus != errSecItemNotFound {
				log("protected delete \(account) failed (OSStatus \(deleteStatus))")
			}
		}

		var add = base
		add[kSecAttrLabel] = label
		add[kSecAttrAccessControl] = control
		add[kSecValueData] = data
		if let context {
			add[kSecUseAuthenticationContext] = context
			add[kSecUseAuthenticationUI] = kSecUseAuthenticationUIFail
		}
		let status = SecItemAdd(add as CFDictionary, nil)
		if status == errSecSuccess, keychainExists(account: account) {
			forgetProtected(account)
			return true
		}
		log("protected write \(account) failed (OSStatus \(status))")
		rememberProtected(account, data)
		return false
	}

	static func protectedData(account: String, context: LAContext?) -> Data? {
		if let context, backend == .dataProtection {
			let (value, status) = copyProtected(account: account, context: context)
			if let value { return value }
			if status != errSecItemNotFound {
				log("protected read \(account) OSStatus \(status)")
			}
		}
		return recalledProtected(account)
	}

	/// Last Keychain status for a presence read. Used when a seal write cannot be read back.
	static func protectedReadStatus(account: String, context: LAContext) -> OSStatus {
		copyProtected(account: account, context: context).1
	}

	/// Presence read without an Activity-log line. Seal read-back retries this.
	static func protectedPayload(account: String, context: LAContext) -> Data? {
		copyProtected(account: account, context: context).0
	}

	static func protectedDataExists(account: String) -> Bool {
		if recalledProtected(account) != nil { return true }
		guard backend == .dataProtection else { return false }
		return keychainExists(account: account)
	}

	static func deleteProtectedData(account: String, context: LAContext? = nil) {
		if backend == .dataProtection {
			if let context {
				var query = dpQuery(account: account)
				query[kSecUseAuthenticationContext] = context
				query[kSecUseAuthenticationUI] = kSecUseAuthenticationUIFail
				_ = SecItemDelete(query as CFDictionary)
			} else {
				_ = silentDelete(account: account)
			}
		}
		forgetProtected(account)
	}

	private static func copyProtected(account: String, context: LAContext) -> (Data?, OSStatus) {
		var query = dpQuery(account: account)
		query[kSecUseAuthenticationContext] = context
		query[kSecUseAuthenticationUI] = kSecUseAuthenticationUIFail
		query[kSecReturnData] = true
		query[kSecMatchLimit] = kSecMatchLimitOne
		var item: CFTypeRef?
		let status = SecItemCopyMatching(query as CFDictionary, &item)
		if status == errSecSuccess, let data = item as? Data, !data.isEmpty {
			return (data, status)
		}
		return (nil, status)
	}

	private static var protectedMemoryStore: [String: Data] = [:]

	private static func rememberProtected(_ account: String, _ data: Data) {
		lock.lock()
		protectedMemoryStore[account] = data
		lock.unlock()
	}

	private static func forgetProtected(_ account: String) {
		lock.lock()
		protectedMemoryStore.removeValue(forKey: account)
		lock.unlock()
	}

	private static func recalledProtected(_ account: String) -> Data? {
		lock.lock()
		defer { lock.unlock() }
		return protectedMemoryStore[account]
	}

	// MARK: memory

	private static func remember(_ acct: String, _ value: String) {
		lock.lock()
		memoryStore[acct] = value
		lock.unlock()
	}

	private static func forget(_ acct: String) {
		lock.lock()
		memoryStore.removeValue(forKey: acct)
		lock.unlock()
	}

	private static func recalled(_ acct: String) -> String? {
		lock.lock()
		defer { lock.unlock() }
		return memoryStore[acct]
	}

	// MARK: logging

	private static func log(_ line: String) {
		AgentLog.shared.keychain(line)
	}
}
