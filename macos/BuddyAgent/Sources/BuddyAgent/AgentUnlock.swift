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
	/// `keepCallerFocus` is a CLI mint: the system sheet is enough, so Settings
	/// is not activated or made key.
	func authorize(reason: String, keepCallerFocus: Bool = false) -> UnlockOutcome {
		// Blocking wait for LocalAuthentication: never call this on the main
		// queue (LA's reply needs the run loop). Socket threads are fine.
		if Thread.isMainThread {
			FileHandle.standardError.write(
				Data("buddy-agent: UnlockGate.authorize called on the main thread\n".utf8))
			assertionFailure("UnlockGate.authorize must not run on the main thread")
		}
		if state.unlocked { return .unlocked }
		if UnlockGate.promptingDisabled { return .unavailable }
		return runOnSessionQueue {
			authorizeOnSessionQueue(reason: reason, keepCallerFocus: keepCallerFocus)
		}
	}

	/// Touch ID drawn in an `LAAuthenticationView` already on screen and paired with
	/// `context`. Same session result as `authorize`, without the system sheet.
	/// Blocks; do not call on the main thread.
	func authorizeEmbedded(context: LAContext, reason: String) -> UnlockOutcome {
		if Thread.isMainThread {
			FileHandle.standardError.write(
				Data("buddy-agent: UnlockGate.authorizeEmbedded called on the main thread\n".utf8))
			assertionFailure("UnlockGate.authorizeEmbedded must not run on the main thread")
		}
		if state.unlocked { return .unlocked }
		if UnlockGate.promptingDisabled { return .unavailable }
		return runOnSessionQueue {
			authorizeEmbeddedOnSessionQueue(context: context, reason: reason)
		}
	}

	private func authorizeOnSessionQueue(reason: String, keepCallerFocus: Bool) -> UnlockOutcome {
		if state.unlocked { return .unlocked }
		if let queued = claimPrompt() { return queued }

		let (outcome, context) = prompt(reason: reason, keepCallerFocus: keepCallerFocus)
		return finishPrompt(outcome: outcome, context: context)
	}

	private func authorizeEmbeddedOnSessionQueue(context: LAContext, reason: String) -> UnlockOutcome {
		if state.unlocked { return .unlocked }
		if let queued = claimPrompt() { return queued }

		let (outcome, sessionContext) = embeddedPrompt(context: context, reason: reason)
		return finishPrompt(outcome: outcome, context: sessionContext)
	}

	/// Marks this caller as the prompt owner, or waits out the prompt already running.
	/// Caller holds no lock.
	private func claimPrompt() -> UnlockOutcome? {
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
		return nil
	}

	private func finishPrompt(outcome: UnlockOutcome, context: LAContext?) -> UnlockOutcome {
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
	private func prompt(reason: String, keepCallerFocus: Bool) -> (UnlockOutcome, LAContext?) {
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

		// The Touch ID / password UI is a system sheet. It does not need Settings
		// to be key. Raising that window is only for an unlock started in the app,
		// so a CLI mint leaves the terminal focused.
		if !keepCallerFocus {
			DispatchQueue.main.async {
				AgentAppActivation.orderForward(AgentWindowController.settingsWindow)
				AgentAppActivation.activateIfAllowed()
			}
		}

		let result = waitForEvaluation(context: context, reason: reason)

		if result.granted, !keepCallerFocus {
			// Accessory apps lose key-window status while the system sheet is up;
			// bring Settings back so an in-app unlock does not look like it closed.
			DispatchQueue.main.async { AgentWindowRefocus.bringSettingsForward() }
		}

		return classification(of: result, context: context)
	}

	/// `context` is already paired with an on-screen `LAAuthenticationView`, so
	/// Touch ID is drawn there instead of in the system sheet.
	private func embeddedPrompt(context: LAContext, reason: String) -> (UnlockOutcome, LAContext?) {
		if let faked = UnlockGate.fakePresence {
			FileHandle.standardError.write(
				Data("buddy-agent: BUDDY_AGENT_FAKE_PRESENCE=\(faked ? "allow" : "deny")\n".utf8))
			return faked ? (.unlocked, context) : (.declined, nil)
		}
		context.touchIDAuthenticationAllowableReuseDuration = 0
		var policyError: NSError?
		guard context.canEvaluatePolicy(.deviceOwnerAuthentication, error: &policyError) else {
			return (.unavailable, nil)
		}
		let result = waitForEvaluation(context: context, reason: reason)
		return classification(of: result, context: context)
	}

	private func waitForEvaluation(
		context: LAContext, reason: String
	) -> (granted: Bool, failure: Error?, timedOut: Bool) {
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
		return (granted, failure, timedOut)
	}

	private func classification(
		of result: (granted: Bool, failure: Error?, timedOut: Bool), context: LAContext
	) -> (UnlockOutcome, LAContext?) {
		if result.granted { return (.unlocked, context) }
		if !result.timedOut { context.invalidate() }
		if result.timedOut { return (.timedOut, nil) }
		switch (result.failure as? LAError)?.code {
		case .biometryNotAvailable, .passcodeNotSet:
			return (.unavailable, nil)
		default:
			break
		}
		return (.declined, nil)
	}

	/// Authenticate the session. Data-protection items are userPresence, so the
	/// context has to be evaluated for item use. `evaluatePolicy` alone still
	/// returns errSecInteractionNotAllowed (-25308) from SecItemCopyMatching.
	/// Do not put this access-control object on the SecItem query; it only
	/// authorizes the context.
	private func evaluate(
		context: LAContext, reason: String, completion: @escaping (Bool, Error?) -> Void
	) {
		if AgentKeychain.backend == .dataProtection, let control = AgentKeychain.presenceAccessControl() {
			context.evaluateAccessControl(
				control, operation: .useItem, localizedReason: reason, reply: completion)
			return
		}
		context.evaluatePolicy(.deviceOwnerAuthentication, localizedReason: reason, reply: completion)
	}
}
