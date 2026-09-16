import Foundation

struct CachedToken {
	let auth: String
	let baseURL: String
	let value: String
	let expiresAt: Date

	var usable: Bool { expiresAt.timeIntervalSinceNow > 30 }
}

/// Lock state plus the in-memory bearer cache. Lock() drops every cached token
/// and any keychain fallback copies; the Agent's config stays on disk.
///
/// Socket threads mutate this, so SwiftUI change notifications are published on
/// the main queue.
final class AgentState: ObservableObject {
	private let lock = NSLock()
	private var unlockedFlag = false
	private var tokens: [String: CachedToken] = [:]
	/// Extra hook for the AppKit status item (SwiftUI uses objectWillChange).
	var onChange: (() -> Void)?

	var unlocked: Bool {
		lock.lock()
		defer { lock.unlock() }
		return unlockedFlag
	}

	/// Cached bearers, newest expiry last. Values are never exposed to the GUI.
	var tokenSummaries: [(auth: String, baseURL: String, expiresAt: Date)] {
		lock.lock()
		defer { lock.unlock() }
		return tokens.values
			.map { ($0.auth, $0.baseURL, $0.expiresAt) }
			.sorted { $0.expiresAt < $1.expiresAt }
	}

	var tokenCount: Int {
		lock.lock()
		defer { lock.unlock() }
		return tokens.count
	}

	func unlock() {
		lock.lock()
		unlockedFlag = true
		lock.unlock()
		notify()
	}

	func lockVault() {
		lock.lock()
		unlockedFlag = false
		tokens.removeAll()
		lock.unlock()
		AgentKeychain.forgetMemory()
		notify()
	}

	func cachedToken(key: String) -> CachedToken? {
		lock.lock()
		defer { lock.unlock() }
		guard let tok = tokens[key], tok.usable else { return nil }
		return tok
	}

	func store(token: CachedToken, key: String) {
		lock.lock()
		tokens[key] = token
		lock.unlock()
		notify()
	}

	private func notify() {
		DispatchQueue.main.async {
			self.objectWillChange.send()
			self.onChange?()
		}
	}
}
