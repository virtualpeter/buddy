import CryptoKit
import Foundation
import LocalAuthentication

/// Detects edits to `agent-config.json` outside the Agent. The HMAC key and
/// signature live in the Keychain behind the same user-presence ACL as secrets,
/// so an attacker cannot re-seal a tampered file without unlocking the Agent.
/// Config is still served while locked; verification runs at mint time (after
/// unlock) and status shows `sealed` when a seal exists but presence is unavailable.
enum AgentConfigIntegrity {
	private static let keyAccount = "config-integrity/key"
	private static let sigAccount = "config-integrity/sig"

	/// Set when sign/verify fails (surfaced in Status `lastError`).
	static var lastFailureReason: String?

	enum State: Equatable {
		case ok
		/// Seal exists in Keychain; not verified until unlock / mint.
		case sealed
		case missing
		case tampered
		case unavailable
		/// Seal exists but the Keychain would not release it to the current
		/// unlock `LAContext` (errSecInteractionNotAllowed). The seal is intact;
		/// the session is what expired. Lock and unlock, never re-save.
		case sessionStale

		var wireValue: String {
			switch self {
			case .ok: return "ok"
			case .sealed: return "sealed"
			case .missing: return "missing"
			case .tampered: return "tampered"
			case .unavailable: return "unavailable"
			case .sessionStale: return "session_stale"
			}
		}
	}

	/// User-facing text for `.sessionStale`.
	static let staleSessionReason =
		"unlock session expired — lock and unlock Buddy Agent"

	/// Drops in-memory seal copies on lock. Keychain items survive lock and restart.
	static func forgetMemory() {
		AgentKeychain.forgetProtectedMemory(for: [keyAccount, sigAccount])
	}

	/// States that do not need Keychain verify (or cannot verify while locked).
	static func preflightState(configPath: URL, unlocked: Bool) -> State? {
		guard FileManager.default.fileExists(atPath: configPath.path) else { return .missing }
		guard AgentKeychain.backend == .dataProtection else { return .unavailable }
		guard isSealed else { return .missing }
		guard unlocked else { return .sealed }
		return nil
	}

	static func state(
		configPath: URL, unlocked: Bool, presence: LAContext?
	) -> State {
		if let preflight = preflightState(configPath: configPath, unlocked: unlocked) {
			return preflight
		}
		guard let presence else { return .sealed }
		return verify(configPath: configPath, presence: presence)
	}

	@discardableResult
	static func sign(configPath: URL, presence: LAContext? = nil) -> Bool {
		lastFailureReason = nil
		guard let data = try? Data(contentsOf: configPath) else {
			lastFailureReason = "Could not read config file for sealing."
			return false
		}
		if hasStoredSeal, presence == nil {
			AgentLog.shared.keychain("config seal update requires unlock (Keychain ACL)")
			lastFailureReason = "Unlock the Agent before saving — Keychain needs your session to update the seal."
			return false
		}
		let keyRead = presence.map { AgentKeychain.protectedRead(account: keyAccount, context: $0) }
		if let keyRead, AgentKeychain.isSessionRejected(keyRead.status) {
			// The key is there; this session cannot open it. Creating a new key
			// here would silently re-key the seal off a stale context.
			AgentLog.shared.keychain(
				"config seal not updated: unlock session not accepted for the integrity key (OSStatus \(keyRead.status))")
			lastFailureReason = "Keychain did not accept the unlock session — lock and unlock Buddy Agent."
			return false
		}
		let key: SymmetricKey
		if let loadedKey = keyRead?.data, loadedKey.count == 32 {
			key = SymmetricKey(data: loadedKey)
		} else {
			guard let created = integrityKey(createIfMissing: true, presence: presence) else { return false }
			key = created
			let keyOk = AgentKeychain.setProtectedData(
				key.withUnsafeBytes { Data($0) },
				account: keyAccount,
				label: "Buddy Agent config integrity key",
				context: presence)
			guard keyOk else {
				AgentLog.shared.keychain("config signature in the Keychain did not persist")
				lastFailureReason = "Keychain rejected the config seal (see Activity log)."
				return false
			}
		}
		let sig = hmac(data: data, key: key)
		let sigOk = AgentKeychain.setProtectedData(
			sig,
			account: sigAccount,
			label: "Buddy Agent config seal",
			context: presence)
		guard sigOk, isSealed else {
			AgentLog.shared.keychain("config signature in the Keychain did not persist")
			lastFailureReason = "Keychain rejected the config seal (see Activity log)."
			return false
		}
		if let presence {
			var rawKey: Data?
			var storedSig: Data?
			for attempt in 0..<5 {
				rawKey = AgentKeychain.protectedPayload(account: keyAccount, context: presence)
				storedSig = AgentKeychain.protectedPayload(account: sigAccount, context: presence)
				if rawKey != nil, storedSig?.count == 32 { break }
				if attempt < 4 { Thread.sleep(forTimeInterval: 0.05) }
			}
			guard let rawKey, let storedSig, storedSig.count == 32 else {
				let keyStatus = AgentKeychain.protectedReadStatus(account: keyAccount, context: presence)
				let sigStatus = AgentKeychain.protectedReadStatus(account: sigAccount, context: presence)
				AgentLog.shared.keychain(
					"config seal not readable after write (key \(keyStatus), sig \(sigStatus))")
				if AgentKeychain.isSessionRejected(keyStatus) || AgentKeychain.isSessionRejected(sigStatus) {
					lastFailureReason =
						"Seal written, but Keychain did not accept the unlock session to read it back — lock and unlock Buddy Agent."
				} else {
					lastFailureReason = "Seal not readable after Keychain write — stay unlocked and Save again."
				}
				return false
			}
			let check = hmac(data: data, key: SymmetricKey(data: rawKey))
			guard constantTimeEqual(check, storedSig) else {
				AgentLog.shared.keychain("config seal read-back mismatch")
				lastFailureReason = "Seal read-back mismatch — Keychain may be out of sync; unlock and Save again."
				return false
			}
		}
		AgentConfigLastGood.record(configPath: configPath)
		return true
	}

	static func verify(configPath: URL, presence: LAContext) -> State {
		guard let data = try? Data(contentsOf: configPath) else { return .tampered }
		let keyRead = AgentKeychain.protectedRead(account: keyAccount, context: presence)
		let sigRead = AgentKeychain.protectedRead(account: sigAccount, context: presence)
		if AgentKeychain.isSessionRejected(keyRead.status)
			|| AgentKeychain.isSessionRejected(sigRead.status)
		{
			return .sessionStale
		}
		guard let rawKey = keyRead.data, rawKey.count == 32,
			let sig = sigRead.data, sig.count == 32
		else { return .missing }
		let key = SymmetricKey(data: rawKey)
		let expected = hmac(data: data, key: key)
		let ok = constantTimeEqual(expected, sig)
		if ok, !FileManager.default.fileExists(atPath: AgentConfigLastGood.snapshotURL(near: configPath).path) {
			AgentConfigLastGood.record(configPath: configPath)
		}
		return ok ? .ok : .tampered
	}

	static func mintBlockedReason(configPath: URL, presence: LAContext?) -> String? {
		guard AgentKeychain.backend == .dataProtection else { return nil }
		guard isSealed else {
			return "config is not sealed — open Buddy Agent Settings and save once"
		}
		guard let presence else {
			return "config seal requires unlock — unlock Buddy Agent before minting"
		}
		switch verify(configPath: configPath, presence: presence) {
		case .ok:
			return nil
		case .tampered:
			return "config was modified outside Buddy Agent — review each change in Buddy Agent Status"
		case .missing:
			return "config seal is missing from Keychain — open Buddy Agent Settings and save once"
		case .sessionStale:
			return staleSessionReason
		case .sealed, .unavailable:
			return "config seal could not be verified"
		}
	}

	static var hasStoredSeal: Bool {
		AgentKeychain.protectedDataExists(account: sigAccount)
	}

	private static var isSealed: Bool {
		hasStoredSeal
	}

	// MARK: key

	private static func integrityKey(createIfMissing: Bool, presence: LAContext?) -> SymmetricKey? {
		let read = AgentKeychain.protectedRead(account: keyAccount, context: presence)
		if let raw = read.data, raw.count == 32 {
			return SymmetricKey(data: raw)
		}
		guard createIfMissing else { return nil }
		// Only mint a key when the Keychain says there is none. A rejected
		// session is not "missing".
		guard !AgentKeychain.isSessionRejected(read.status) else {
			AgentLog.shared.keychain(
				"config integrity key not created: unlock session not accepted (OSStatus \(read.status))")
			lastFailureReason = "Keychain did not accept the unlock session — lock and unlock Buddy Agent."
			return nil
		}
		let key = SymmetricKey(size: .bits256)
		let stored = AgentKeychain.setProtectedData(
			key.withUnsafeBytes { Data($0) },
			account: keyAccount,
			label: "Buddy Agent config integrity key",
			context: presence)
		return stored || AgentKeychain.protectedData(account: keyAccount, context: presence) != nil
			? key : nil
	}

	private static func hmac(data: Data, key: SymmetricKey) -> Data {
		Data(HMAC<SHA256>.authenticationCode(for: data, using: key))
	}

	private static func constantTimeEqual(_ a: Data, _ b: Data) -> Bool {
		guard a.count == b.count else { return false }
		var diff: UInt8 = 0
		for i in 0..<a.count {
			diff |= a[i] ^ b[i]
		}
		return diff == 0
	}
}
