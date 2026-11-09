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

	/// Posted when a deferred probe resolves or the keychain becomes unreachable
	/// again, so Status can leave "recovering".
	static let backendChanged = Notification.Name("buddy.agent.keychainBackend")

	private static let stateLock = NSLock()
	/// Set only for a decision that will not change: data protection works, or
	/// this build cannot use it (no ACL, missing entitlement). A not-ready
	/// probe stays nil so a later retry can still commit.
	private static var committed: Backend?
	/// Last probe could not use the data-protection keychain yet. Not a
	/// decision to keep secrets in memory.
	private static var deferredFlag = false
	private static var loggedDeferred = false

	/// True while this process has not been able to use the data-protection
	/// keychain. A signed Agent keeps reporting `.dataProtection` in the
	/// meantime so secrets are not silently diverted into memory.
	static var isDeferred: Bool {
		stateLock.lock()
		defer { stateLock.unlock() }
		return deferredFlag
	}

	static var backend: Backend {
		stateLock.lock()
		let committed = committed
		let deferred = deferredFlag
		stateLock.unlock()
		if let committed { return committed }
		if deferred { return optimisticBackend }
		return probe(force: false)
	}

	/// Run the probe again. A permanent memory decision is left alone.
	@discardableResult
	static func refreshBackend() -> Backend {
		stateLock.lock()
		let committed = committed
		let deferred = deferredFlag
		stateLock.unlock()
		if committed == .memory { return .memory }
		if committed == .dataProtection, !deferred { return .dataProtection }
		return probe(force: true)
	}

	/// Signed builds expect the data-protection keychain even while a probe
	/// is still failing. Unsigned builds have no access group and stay in memory.
	private static var optimisticBackend: Backend {
		keychainAccessGroup == nil ? .memory : .dataProtection
	}

	private enum ProbeOutcome {
		case ready
		case permanent
		case notReady(OSStatus)
	}

	private static func probe(force: Bool) -> Backend {
		stateLock.lock()
		if let committed, committed == .memory || (!force && !deferredFlag) {
			stateLock.unlock()
			return committed
		}
		if deferredFlag, !force {
			stateLock.unlock()
			return optimisticBackend
		}
		stateLock.unlock()

		let outcome = runProbe()

		stateLock.lock()
		var notify = false
		var line: String?
		let resolved: Backend
		switch outcome {
		case .ready:
			let first = committed == nil && !deferredFlag
			let recovered = deferredFlag
			committed = .dataProtection
			deferredFlag = false
			loggedDeferred = false
			if first {
				line =
					"probed the Keychain; a presence check is available\(keychainAccessGroup.map { " (group \($0))" } ?? "")"
			} else if recovered {
				line = "data protection keychain is available"
				notify = true
			}
			resolved = .dataProtection
		case .permanent:
			committed = .memory
			deferredFlag = false
			notify = true
			resolved = .memory
		case .notReady(let status):
			committed = nil
			deferredFlag = true
			if !loggedDeferred {
				loggedDeferred = true
				line =
					"data protection keychain is not usable in this process yet (OSStatus \(status)); will retry"
				notify = true
			}
			resolved = optimisticBackend
		}
		stateLock.unlock()
		if let line { log(line) }
		if notify {
			DispatchQueue.main.async {
				NotificationCenter.default.post(name: backendChanged, object: nil)
			}
		}
		return resolved
	}

	private static func runProbe() -> ProbeOutcome {
		guard let control = accessControl() else {
			log("kept secrets in memory only; could not build a Keychain access control")
			return .permanent
		}
		let acct = "__buddy_acl_probe__"
		var add = dpQuery(account: acct)
		add[kSecAttrAccessControl] = control
		add[kSecValueData] = Data("probe".utf8)
		let status = SecItemAdd(add as CFDictionary, nil)
		silentDelete(account: acct)
		if status == errSecSuccess || status == errSecDuplicateItem {
			return .ready
		}
		// These mean "not in this process, right now" — a launch macOS restored
		// before the keybag was ready, or a process kept across a restart.
		// Missing entitlement is permanent on an unsigned binary and the same
		// stuck-process symptom when the profile granted an access group.
		if isTransientProbeFailure(status) {
			return .notReady(status)
		}
		log("probed the Keychain for a presence check; it failed (OSStatus \(status)); kept secrets in memory only")
		return .permanent
	}

	/// Failures that a later probe, or a fresh process, can still get past.
	private static func isTransientProbeFailure(_ status: OSStatus) -> Bool {
		switch status {
		case errSecInteractionNotAllowed, errSecNotAvailable, errSecAuthFailed:
			return true
		case errSecMissingEntitlement:
			return keychainAccessGroup != nil
		default:
			return false
		}
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
			log("purged leftover items from the login Keychain for \(service)")
		case errSecItemNotFound:
			break
		default:
			log("Keychain refused to purge leftover login Keychain items (OSStatus \(status))")
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

	/// Same ACL stored on every userPresence item. Pass this to
	/// `LAContext.evaluateAccessControl` — a policy-only unlock is not accepted
	/// when reading these items (`errSecInteractionNotAllowed`).
	static func presenceAccessControl() -> SecAccessControl? {
		accessControl()
	}

	private static func accessControl() -> SecAccessControl? {
		var error: Unmanaged<CFError>?
		let control = SecAccessControlCreateWithFlags(
			nil, kSecAttrAccessibleWhenUnlockedThisDeviceOnly, .userPresence, &error)
		if control == nil {
			let reason = error?.takeRetainedValue().localizedDescription ?? "unknown"
			log("could not build a Keychain presence check (\(reason))")
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
			log("Keychain refused to look up \(acct) (OSStatus \(status)); treating it as absent")
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
			log("kept \(acct) in memory only")
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
				log("could not store \(acct) in the Keychain; an existing item blocked the write (delete OSStatus \(deleteStatus)); kept it in memory only")
				remember(acct, value)
				usingFallback = true
				return false
			}
		}
		if status == errSecSuccess, keychainExists(account: acct) {
			forget(acct)
			log("stored \(acct) in the Keychain")
			return true
		}
		if status == errSecSuccess {
			// SecItemAdd sometimes reports success while the item is not
			// queryable — do not tell the UI the secret is in the keychain.
			log("stored \(acct) in the Keychain but it is not readable (delete OSStatus \(deleteStatus)); kept it in memory only")
		} else {
			log("Keychain refused to store \(acct) (OSStatus \(status), delete OSStatus \(deleteStatus)); kept it in memory only")
		}
		remember(acct, value)
		usingFallback = true
		return false
	}

	enum SecretRead {
		case value(String)
		case missing
		/// The item is there, but this process's unlock context cannot open it.
		case sessionRejected(OSStatus)
	}

	/// Prefer the presence-gated item with UnlockGate's LAContext. Never draw a
	/// Keychain password sheet. Memory is the unsigned/dev path only.
	static func get(auth: String, kind: Kind, context: LAContext? = nil) -> String? {
		switch read(auth: auth, kind: kind, context: context) {
		case .value(let value):
			return value
		case .missing, .sessionRejected(_):
			return nil
		}
	}

	static func read(auth: String, kind: Kind, context: LAContext?) -> SecretRead {
		let acct = account(auth: auth, kind: kind)
		if backend == .dataProtection, let context {
			let (value, status) = copySecret(account: acct, context: context)
			if let value { return .value(value) }
			if isSessionRejected(status) { return .sessionRejected(status) }
		}
		if let cached = recalled(acct) { return .value(cached) }
		return .missing
	}

	private static func copySecret(account acct: String, context: LAContext) -> (String?, OSStatus) {
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
			return (value, status)
		}
		log("Keychain refused to read \(acct) (OSStatus \(status)\(statusNote(status)))")
		return (nil, status)
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
				log("Keychain refused to update \(account) (OSStatus \(updateStatus))")
				rememberProtected(account, data)
				return false
			}
		} else {
			let deleteStatus = silentDelete(account: account)
			if deleteStatus != errSecSuccess && deleteStatus != errSecItemNotFound {
				log("Keychain refused to delete \(account) (OSStatus \(deleteStatus))")
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
		log("Keychain refused to store \(account) (OSStatus \(status))")
		rememberProtected(account, data)
		return false
	}

	static func protectedData(account: String, context: LAContext?) -> Data? {
		protectedRead(account: account, context: context).data
	}

	/// Presence read with the Keychain status, so callers can tell "not there"
	/// (`errSecItemNotFound`) from "this session may not open it"
	/// (`errSecInteractionNotAllowed`). Falls back to the in-memory copy.
	static func protectedRead(account: String, context: LAContext?) -> (data: Data?, status: OSStatus) {
		if let context, backend == .dataProtection {
			let (value, status) = copyProtected(account: account, context: context)
			if let value { return (value, status) }
			if status != errSecItemNotFound {
				log("Keychain refused to read \(account) (OSStatus \(status)\(statusNote(status)))")
			}
			if let cached = recalledProtected(account) { return (cached, errSecSuccess) }
			return (nil, status)
		}
		if let cached = recalledProtected(account) { return (cached, errSecSuccess) }
		return (nil, errSecItemNotFound)
	}

	/// True when the item exists but the Keychain would need to show UI for
	/// this context. The item is fine; the unlock session is not accepted.
	static func isSessionRejected(_ status: OSStatus) -> Bool {
		status == errSecInteractionNotAllowed || status == errSecAuthFailed
			|| status == errSecUserCanceled
	}

	private static func statusNote(_ status: OSStatus) -> String {
		switch status {
		case errSecInteractionNotAllowed:
			return " errSecInteractionNotAllowed (unlock session not accepted for this item)"
		case errSecAuthFailed:
			return " errSecAuthFailed"
		case errSecUserCanceled:
			return " errSecUserCanceled"
		default:
			return ""
		}
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
