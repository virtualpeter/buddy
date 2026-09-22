import AppKit
import Foundation
import LocalAuthentication

/// How the Agent proves the person is there before it mints.
enum PresenceMode: String {
	/// Keychain userPresence ACL on the secret itself. Needs the signed app.
	case acl
	/// LocalAuthentication as a stand-in: a pass/fail Boolean, not a gate on
	/// the secret. Honest name for what an unsigned build can do.
	case simulated
	/// No device-owner authentication available at all; the Agent fails closed.
	case unavailable
}

enum UnlockOutcome {
	case unlocked
	case declined
	case timedOut
	case unavailable
}

/// Persisted unlock-prompt timeout (`UserDefaults`). 0 = buddy cannot trigger prompts.
enum AgentUnlockTimeout {
	static let userDefaultsKey = "BuddyAgentUnlockTimeoutSeconds"
	static let lastSecondsKey = "BuddyAgentUnlockTimeoutLastSeconds"
	static let defaultSeconds: TimeInterval = 60

	static var persistedSeconds: TimeInterval {
		guard UserDefaults.standard.object(forKey: userDefaultsKey) != nil else {
			return defaultSeconds
		}
		return max(0, UserDefaults.standard.double(forKey: userDefaultsKey))
	}

	/// Dev override via `BUDDY_AGENT_UNLOCK_TIMEOUT`; otherwise the persisted preference.
	static var effectiveSeconds: TimeInterval {
		if let raw = ProcessInfo.processInfo.environment["BUDDY_AGENT_UNLOCK_TIMEOUT"],
			let seconds = TimeInterval(raw), seconds >= 0
		{
			return seconds
		}
		return persistedSeconds
	}

	static var envOverrideActive: Bool {
		ProcessInfo.processInfo.environment["BUDDY_AGENT_UNLOCK_TIMEOUT"] != nil
	}
}

/// The single presence path: the tray Unlock item and an incoming token request
/// both come through here, so there is one prompt, one timeout, and one place
/// that decides what "unlocked" means.
///
/// Called from socket connection threads, never from the main queue: it blocks
/// its caller until LocalAuthentication answers. The prompt itself is drawn by
/// the system, so the main queue stays free to serve the GUI meanwhile.
final class UnlockGate {
	/// Runs on the presence session queue right after unlock (same thread as `LAContext`).
	var onSessionOpened: ((LAContext) -> Void)?

	private let state: AgentState
	private let sessionQueue = DispatchQueue(label: "buddy.agent.presence.session")
	private let sessionQueueKey = DispatchSpecificKey<Void>()
	private let lock = NSLock()
	/// Waiters parked behind the prompt that is already on screen.
	private var pending: [DispatchSemaphore] = []
	private var pendingOutcome: UnlockOutcome = .declined
	private var prompting = false
	/// The context the person authenticated. On a signed Agent this is what
	/// releases the keychain items, so it is the session: invalidating it is
	/// what makes relocking real rather than a flag flip.
	private var authenticated: LAContext?

	init(state: AgentState) {
		self.state = state
		sessionQueue.setSpecific(key: sessionQueueKey, value: ())
		// AgentState owns lock/relock but must not import LocalAuthentication
		// just to end a session.
		state.onRelock = { [weak self] in self?.endSession() }
	}

	/// Keychain + config seal work must run on the queue that owns `LAContext`.
	func withAuthenticatedContext(_ work: (LAContext) -> Bool) -> Bool {
		runOnSessionQueue { performWithAuthenticatedContext(work) }
	}

	func withAuthenticatedContext<T>(_ work: (LAContext) -> T) -> T? {
		var value: T?
		runOnSessionQueue {
			guard state.unlocked else { return }
			lock.lock()
			let context = authenticated
			lock.unlock()
			guard let context else { return }
			value = work(context)
		}
		return value
	}

	private func runOnSessionQueue<T>(_ block: () -> T) -> T {
		if DispatchQueue.getSpecific(key: sessionQueueKey) != nil {
			return block()
		}
		return sessionQueue.sync(execute: block)
	}

	private func performWithAuthenticatedContext(_ work: (LAContext) -> Bool) -> Bool {
		guard state.unlocked else { return false }
		lock.lock()
		let context = authenticated
		lock.unlock()
		guard let context else { return false }
		return work(context)
	}

	/// The authenticated context for the current session, or nil when locked.
	var presenceContext: LAContext? {
		lock.lock()
		defer { lock.unlock() }
		return authenticated
	}

	/// Drops the authenticated context, revoking keychain access until the next
	/// prompt. Called on tray Lock, `agent lock`, and the idle relock.
	func endSession() {
		runOnSessionQueue {
			lock.lock()
			let context = authenticated
			authenticated = nil
			lock.unlock()
			context?.invalidate()
		}
	}

	/// Seconds the prompt may stay unanswered. 0 disables prompting entirely,
	/// which leaves the tray as the only way to unlock.
	static var timeout: TimeInterval { AgentUnlockTimeout.effectiveSeconds }

	static var promptingDisabled: Bool { timeout == 0 }

	/// What this build can actually do. Recomputed per call: a Mac can gain or
	/// lose Touch ID (lid closed on a clamshell, biometry lockout).
	static var mode: PresenceMode {
		if fakePresence != nil { return .simulated }
		var error: NSError?
		guard LAContext().canEvaluatePolicy(.deviceOwnerAuthentication, error: &error) else {
			return .unavailable
		}
		// A signed Agent holds its secrets behind a userPresence ACL, so the
		// prompt releases the item. Unsigned, all it can do is ask and believe
		// its own answer.
		return AgentKeychain.backend == .dataProtection ? .acl : .simulated
	}

	/// BUDDY_AGENT_FAKE_PRESENCE=allow|deny answers without a prompt so the
	/// soak can run unattended. Only honoured on a dev Agent.
	static var fakePresence: Bool? {
		guard PeerCheck.allowUnsigned else { return nil }
		switch ProcessInfo.processInfo.environment["BUDDY_AGENT_FAKE_PRESENCE"] {
		case "allow": return true
		case "deny": return false
		default: return nil
		}
	}

	/// authorize returns .unlocked when the Agent may mint. Already-unlocked is
	/// an immediate .unlocked: presence is what opens the session, and the idle
	/// timer is what closes it again.
	func authorize(reason: String) -> UnlockOutcome {
		// Blocking wait for LocalAuthentication: never call this on the main
		// queue (LA's reply needs the run loop). Socket threads are fine.
		if Thread.isMainThread {
			FileHandle.standardError.write(
				Data("buddy-agent: UnlockGate.authorize called on the main thread\n".utf8))
			assertionFailure("UnlockGate.authorize must not run on the main thread")
		}
		if state.unlocked { return .unlocked }
		if UnlockGate.promptingDisabled { return .unavailable }
		return runOnSessionQueue { authorizeOnSessionQueue(reason: reason) }
	}

	private func authorizeOnSessionQueue(reason: String) -> UnlockOutcome {
		if state.unlocked { return .unlocked }

		lock.lock()
		if prompting {
			let waiter = DispatchSemaphore(value: 0)
			pending.append(waiter)
			lock.unlock()
			waiter.wait()
			lock.lock()
			let outcome = pendingOutcome
			lock.unlock()
			return outcome
		}
		prompting = true
		lock.unlock()

		let (outcome, context) = prompt(reason: reason)
		if outcome == .unlocked, let context {
			lock.lock()
			let previous = authenticated
			authenticated = context
			lock.unlock()
			previous?.invalidate()
			state.unlock()
			onSessionOpened?(context)
		}

		lock.lock()
		pendingOutcome = outcome
		let waiters = pending
		pending = []
		prompting = false
		lock.unlock()
		for waiter in waiters { waiter.signal() }
		return outcome
	}

	/// Returns the outcome and, on success, the context that carries it. The
	/// caller keeps that context: it is the session. Always `.useItem` — that
	/// is what releases an existing presence-gated secret for minting.
	private func prompt(reason: String) -> (UnlockOutcome, LAContext?) {
		if let faked = UnlockGate.fakePresence {
			FileHandle.standardError.write(
				Data("buddy-agent: BUDDY_AGENT_FAKE_PRESENCE=\(faked ? "allow" : "deny")\n".utf8))
			return faked ? (.unlocked, LAContext()) : (.declined, nil)
		}
		let context = LAContext()
		// Reusing the Mac login made evaluatePolicy succeed with no UI at all
		// when buddy's Go path tried this, so ask every time.
		context.touchIDAuthenticationAllowableReuseDuration = 0
		var policyError: NSError?
		guard context.canEvaluatePolicy(.deviceOwnerAuthentication, error: &policyError) else {
			return (.unavailable, nil)
		}

		// An accessory app has no windows to bring forward, so make sure the
		// system sheet is not drawn behind whatever the user is looking at.
		DispatchQueue.main.async {
			AgentAppActivation.orderForward(AgentWindowController.settingsWindow)
			AgentAppActivation.activateIfAllowed()
		}

		let done = DispatchSemaphore(value: 0)
		var granted = false
		var failure: Error?
		// LocalAuthentication has no timeout of its own: invalidating the
		// context dismisses the prompt and completes with LAError.appCancel.
		var timedOut = false
		let timer = DispatchWorkItem {
			timedOut = true
			context.invalidate()
		}
		DispatchQueue.global().asyncAfter(deadline: .now() + UnlockGate.timeout, execute: timer)
		evaluate(context: context, reason: reason) { ok, err in
			granted = ok
			failure = err
			done.signal()
		}
		done.wait()
		timer.cancel()

		if granted {
			// Accessory apps lose key-window status while the system sheet is up;
			// bring Settings back so mint-from-locked does not look like it closed.
			DispatchQueue.main.async { AgentWindowRefocus.bringSettingsForward() }
			return (.unlocked, context)
		}
		context.invalidate()
		if timedOut { return (.timedOut, nil) }
		switch (failure as? LAError)?.code {
		case .biometryNotAvailable, .passcodeNotSet:
			return (.unavailable, nil)
		default:
			break
		}
		return (.declined, nil)
	}

	/// Authenticate the session. Secrets are still gated by a userPresence ACL
	/// on the data-protection keychain; `evaluatePolicy` produces an LAContext
	/// that SecItemCopyMatching accepts via kSecUseAuthenticationContext.
	/// (evaluateAccessControl against a freshly built SecAccessControl did not
	/// reliably release those items — reads came back errSecItemNotFound.)
	private func evaluate(
		context: LAContext, reason: String, completion: @escaping (Bool, Error?) -> Void
	) {
		context.evaluatePolicy(.deviceOwnerAuthentication, localizedReason: reason, reply: completion)
	}
}
