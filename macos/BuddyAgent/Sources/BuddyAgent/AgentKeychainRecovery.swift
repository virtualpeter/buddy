import AppKit
import Darwin
import Foundation
import LocalAuthentication

// notify(3) is not imported into Swift. These are the registration calls for
// "keychain is unlocked" and "screen is locked".
private let notifyStatusOK: UInt32 = 0
private typealias NotifyBlock = @convention(block) (Int32) -> Void

@_silgen_name("notify_register_dispatch")
private func notifyRegisterDispatch(
	_ name: UnsafePointer<CChar>,
	_ outToken: UnsafeMutablePointer<Int32>,
	_ queue: DispatchQueue,
	_ handler: @escaping NotifyBlock
) -> UInt32

@_silgen_name("notify_register_check")
private func notifyRegisterCheck(
	_ name: UnsafePointer<CChar>, _ outToken: UnsafeMutablePointer<Int32>
) -> UInt32

@_silgen_name("notify_get_state")
private func notifyGetState(_ token: Int32, _ state: UnsafeMutablePointer<UInt64>) -> UInt32

@_silgen_name("notify_cancel")
private func notifyCancel(_ token: Int32) -> UInt32

/// Recovers a data-protection keychain session that macOS left stuck.
///
/// Two launches do this. A process the system keeps across a restart still
/// has its old `securityd` connection, and `kern.boottime` has moved on.
/// A process started again before the keybag is ready (login item, reopen
/// windows) gets `errSecInteractionNotAllowed` from `SecItemAdd` and would
/// otherwise stay in memory for the rest of its life. Quitting and opening
/// the Agent again is what used to clear it. This does that once, after
/// giving the keychain a chance to come up in-process.
enum AgentKeychainRecovery {
	private static let relaunchArgument = "--buddy-keychain-relaunch"
	private static var started = false
	private static var relaunching = false
	private static var loggedGiveUp = false
	private static var launchBoot = 0
	private static var prepareForReplacement: (() -> Void)?
	private static var resumeListening: (() -> Void)?
	private static var quitProcess: (() -> Void)?
	private static var observers: [NSObjectProtocol] = []
	private static var keychainNotifyToken: Int32 = 0

	private static var alreadyRelaunched: Bool {
		ProcessInfo.processInfo.arguments.contains(relaunchArgument)
	}

	/// The one automatic relaunch already ran and the keychain is still stuck.
	static var needsManualQuit: Bool { loggedGiveUp }

	static func start(
		prepareForReplacement: @escaping () -> Void,
		resumeListening: @escaping () -> Void,
		quit: @escaping () -> Void
	) {
		guard !started else { return }
		started = true
		self.prepareForReplacement = prepareForReplacement
		self.resumeListening = resumeListening
		quitProcess = quit
		launchBoot = bootTime()
		observe()
		_ = AgentKeychain.backend
		guard AgentKeychain.isDeferred else { return }
		for delay in [1.0, 3.0] {
			DispatchQueue.main.asyncAfter(deadline: .now() + delay) { retry(allowRelaunch: false) }
		}
		DispatchQueue.main.asyncAfter(deadline: .now() + 8) { retry(allowRelaunch: true) }
	}

	/// A fresh unlock was still rejected. Another prompt in this process will
	/// not release the items. Returns true when a replacement is starting.
	@discardableResult
	static func recoverStuckSession() -> Bool {
		if bootChanged() {
			return relaunch(reason: "macOS kept this process across a restart, and its Keychain session is from the previous boot")
		}
		return relaunch(reason: "the data protection keychain will not accept this process's unlock")
	}

	private static func observe() {
		let workspace = NSWorkspace.shared.notificationCenter
		observers.append(workspace.addObserver(
			forName: NSWorkspace.didWakeNotification, object: nil, queue: .main
		) { _ in
			guard bootChanged() else { return }
			relaunch(reason: "macOS kept this process across a restart, and its Keychain session is from the previous boot")
		})
		observers.append(DistributedNotificationCenter.default().addObserver(
			forName: Notification.Name("com.apple.screenIsUnlocked"),
			object: nil,
			queue: .main
		) { _ in
			retry(allowRelaunch: false)
			DispatchQueue.main.asyncAfter(deadline: .now() + 2) { retry(allowRelaunch: true) }
		})
		var token: Int32 = 0
		let status = "com.apple.system.keychainunlocked".withCString { name in
			notifyRegisterDispatch(name, &token, DispatchQueue.main) { _ in
				retry(allowRelaunch: false)
				DispatchQueue.main.asyncAfter(deadline: .now() + 2) { retry(allowRelaunch: true) }
			}
		}
		if status == notifyStatusOK {
			keychainNotifyToken = token
		}
		// A preserved process may not get a wake notification. Boot time is
		// the signal that this address space survived the restart.
		let timer = Timer.scheduledTimer(withTimeInterval: 30, repeats: true) { _ in
			guard bootChanged() else { return }
			relaunch(reason: "macOS kept this process across a restart, and its Keychain session is from the previous boot")
		}
		RunLoop.main.add(timer, forMode: .common)
	}

	private static func retry(allowRelaunch: Bool) {
		if bootChanged() {
			relaunch(reason: "macOS kept this process across a restart, and its Keychain session is from the previous boot")
			return
		}
		guard AgentKeychain.isDeferred else { return }
		_ = AgentKeychain.refreshBackend()
		guard AgentKeychain.isDeferred else { return }
		guard allowRelaunch, sessionIsUnlocked() else { return }
		relaunch(reason: "data protection keychain stayed unavailable after the Mac was unlocked")
	}

	/// The user is logged in and the screen is not locked, so a WhenUnlocked
	/// item should be writable. A failure here is this process, not the keybag.
	private static func sessionIsUnlocked() -> Bool {
		if screenIsLocked() { return false }
		var error: NSError?
		return LAContext().canEvaluatePolicy(.deviceOwnerAuthentication, error: &error)
	}

	private static func screenIsLocked() -> Bool {
		var token: Int32 = 0
		let status = "com.apple.screenIsLocked".withCString { name in
			notifyRegisterCheck(name, &token)
		}
		guard status == notifyStatusOK else { return false }
		defer { _ = notifyCancel(token) }
		var state: UInt64 = 0
		_ = notifyGetState(token, &state)
		return state != 0
	}

	@discardableResult
	private static func relaunch(reason: String) -> Bool {
		if !Thread.isMainThread {
			var startedRelaunch = false
			DispatchQueue.main.sync { startedRelaunch = relaunch(reason: reason) }
			return startedRelaunch
		}
		guard !relaunching else { return true }
		if ProcessInfo.processInfo.environment["BUDDY_AGENT_NO_RELAUNCH"] == "1" {
			AgentLog.shared.keychain("\(reason); relaunch suppressed")
			return false
		}
		// Unsigned builds have nothing in the data-protection keychain to
		// recover, and a relaunch would drop secrets that exist only in memory.
		guard AgentKeychain.keychainAccessGroup != nil else { return false }
		if alreadyRelaunched {
			if !loggedGiveUp {
				loggedGiveUp = true
				AgentLog.shared.keychain(
					"\(reason). Quit Buddy Agent and open it again — relaunch already ran once in this boot")
				NotificationCenter.default.post(name: AgentKeychain.backendChanged, object: nil)
			}
			return false
		}
		relaunching = true
		AgentLog.shared.keychain("\(reason) — relaunching")
		prepareForReplacement?()
		let config = NSWorkspace.OpenConfiguration()
		config.createsNewApplicationInstance = true
		config.activates = false
		config.arguments = [relaunchArgument]
		NSWorkspace.shared.openApplication(at: Bundle.main.bundleURL, configuration: config) {
			app, error in
			DispatchQueue.main.async {
				if let error {
					relaunching = false
					AgentLog.shared.keychain(
						"relaunch failed (\(error.localizedDescription)) — quit Buddy Agent and open it again")
					resumeListening?()
					return
				}
				// Launch Services sometimes activates this process instead of
				// starting another. Quitting then would take the Agent down.
				if app?.processIdentifier == getpid() {
					relaunching = false
					AgentLog.shared.keychain(
						"relaunch stayed in this process — quit Buddy Agent and open it again")
					resumeListening?()
					return
				}
				// The reply that triggered this, if any, is written on the
				// connection thread before the new process is up. Leave a
				// moment so that write is not cut off by terminate.
				DispatchQueue.main.asyncAfter(deadline: .now() + 0.3) {
					quitProcess?()
				}
			}
		}
		return true
	}

	private static func bootChanged() -> Bool {
		let now = bootTime()
		return launchBoot > 0 && now > 0 && now != launchBoot
	}

	private static func bootTime() -> Int {
		var boot = timeval()
		var size = MemoryLayout<timeval>.stride
		guard sysctlbyname("kern.boottime", &boot, &size, nil, 0) == 0 else { return 0 }
		return Int(boot.tv_sec)
	}
}
