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
	private var idleTimer: DispatchWorkItem?
	private var relocksAt: Date?
	/// Extra hook for the AppKit status item (SwiftUI uses objectWillChange).
	var onChange: (() -> Void)?
	/// Set by UnlockGate. Ends the presence session — on a signed Agent that
	/// invalidates the LAContext holding the keychain items open, so relocking
	/// revokes access instead of only clearing a flag.
	var onRelock: (() -> Void)?

	/// Seconds of no minting before the Agent relocks itself. 0 = never.
	/// Presence opens the session; this is what closes it again.
	static var idleLockAfter: TimeInterval {
		guard let raw = ProcessInfo.processInfo.environment["BUDDY_AGENT_IDLE_LOCK"],
			let seconds = TimeInterval(raw), seconds >= 0
		else {
			return 15 * 60
		}
		return seconds
	}

	var unlocked: Bool {
		lock.lock()
		defer { lock.unlock() }
		return unlockedFlag
	}

	/// When the idle timer will relock, or nil when locked or set to never.
	var unlockedUntil: Date? {
		lock.lock()
		defer { lock.unlock() }
		return relocksAt
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
		restartIdleTimer()
		notify()
	}

	func lockVault() {
		lock.lock()
		unlockedFlag = false
		tokens.removeAll()
		idleTimer?.cancel()
		idleTimer = nil
		relocksAt = nil
		lock.unlock()
		onRelock?()
		AgentKeychain.forgetMemory()
		notify()
	}

	/// restartIdleTimer pushes the automatic relock out again. Called on unlock
	/// and on every mint or cache hit, so an idle Agent closes but a busy one
	/// does not relock underneath a running command.
	private func restartIdleTimer() {
		let after = AgentState.idleLockAfter
		lock.lock()
		idleTimer?.cancel()
		guard unlockedFlag, after > 0 else {
			idleTimer = nil
			relocksAt = nil
			lock.unlock()
			return
		}
		let work = DispatchWorkItem { [weak self] in
			guard let self, self.unlocked else { return }
			FileHandle.standardError.write(
				Data("buddy-agent: relocked after \(Int(after))s idle\n".utf8))
			self.lockVault()
		}
		idleTimer = work
		relocksAt = Date().addingTimeInterval(after)
		lock.unlock()
		DispatchQueue.global().asyncAfter(deadline: .now() + after, execute: work)
	}

	func cachedToken(key: String) -> CachedToken? {
		lock.lock()
		let hit = tokens[key]
		lock.unlock()
		guard let tok = hit, tok.usable else { return nil }
		restartIdleTimer()
		return tok
	}

	func store(token: CachedToken, key: String) {
		lock.lock()
		tokens[key] = token
		lock.unlock()
		restartIdleTimer()
		notify()
	}

	private func notify() {
		DispatchQueue.main.async {
			self.objectWillChange.send()
			self.onChange?()
		}
	}
}
