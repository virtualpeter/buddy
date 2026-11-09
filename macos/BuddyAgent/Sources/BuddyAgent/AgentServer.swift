import Darwin
import Foundation

/// Unix-domain socket server speaking newline-delimited JSON. One request and
/// one response per connection is the normal CLI pattern, but the read loop
/// keeps serving lines until the peer closes.
final class AgentServer {
	private let state: AgentState
	private let store: AgentStore
	private let gate: UnlockGate
	private let queue = DispatchQueue(label: "buddy.agent.server")
	private var listenFD: Int32 = -1
	/// Set when a replacement process is about to bind the same path. stop()
	/// must not unlink that new socket on the way out.
	private var leaveSocketInPlace = false
	private(set) var socketPath: String = AgentPaths.socketPath

	init(state: AgentState, store: AgentStore, gate: UnlockGate) {
		self.state = state
		self.store = store
		self.gate = gate
	}

	func start() throws {
		if listenFD >= 0 { return }
		leaveSocketInPlace = false
		try AgentPaths.prepareSocketDirectory()
		socketPath = AgentPaths.socketPath
		unlink(socketPath)

		let fd = socket(AF_UNIX, SOCK_STREAM, 0)
		guard fd >= 0 else { throw AgentServerError.posix("socket", errno) }

		var address = sockaddr_un()
		address.sun_family = sa_family_t(AF_UNIX)
		let pathBytes = Array(socketPath.utf8)
		guard pathBytes.count < MemoryLayout.size(ofValue: address.sun_path) else {
			close(fd)
			throw AgentServerError.message("socket path too long: \(socketPath)")
		}
		withUnsafeMutableBytes(of: &address.sun_path) { raw in
			raw.copyBytes(from: pathBytes)
		}
		address.sun_len = UInt8(MemoryLayout<sockaddr_un>.size)

		let bound = withUnsafePointer(to: &address) { pointer in
			pointer.withMemoryRebound(to: sockaddr.self, capacity: 1) { sa in
				bind(fd, sa, socklen_t(MemoryLayout<sockaddr_un>.size))
			}
		}
		guard bound == 0 else {
			close(fd)
			throw AgentServerError.posix("bind", errno)
		}
		// Only this user may connect; the codesign check narrows it further.
		chmod(socketPath, 0o600)
		guard listen(fd, 16) == 0 else {
			close(fd)
			throw AgentServerError.posix("listen", errno)
		}
		listenFD = fd
		queue.async { [weak self] in self?.acceptLoop(fd) }
	}

	/// Close the listener without deleting the socket path, so a replacement
	/// Agent can unlink and bind it. A later stop() must not remove that path.
	func handoffSocket() {
		leaveSocketInPlace = true
		if listenFD >= 0 {
			close(listenFD)
			listenFD = -1
		}
	}

	func stop() {
		if listenFD >= 0 {
			close(listenFD)
			listenFD = -1
		}
		if !leaveSocketInPlace {
			unlink(socketPath)
		}
	}

	private func acceptLoop(_ fd: Int32) {
		while true {
			let conn = accept(fd, nil, nil)
			if conn < 0 {
				if errno == EINTR { continue }
				return  // listener closed
			}
			let peer = PeerCheck.capturePeer(at: conn)
			let handler = DispatchQueue(label: "buddy.agent.conn", qos: .userInitiated)
			handler.async { [weak self] in
				self?.serve(conn, peer: peer)
				close(conn)
			}
		}
	}

	private func serve(_ fd: Int32, peer: PeerIdentity) {
		if let reason = PeerCheck.reject(peer: peer) {
			write(fd, AgentReply.failure(.denied, reason))
			FileHandle.standardError.write(Data("buddy-agent: rejected peer: \(reason)\n".utf8))
			return
		}
		var buffer = Data()
		var chunk = [UInt8](repeating: 0, count: 4096)
		while true {
			let n = read(fd, &chunk, chunk.count)
			if n <= 0 { return }
			buffer.append(contentsOf: chunk[0..<n])
			while let newline = buffer.firstIndex(of: 0x0A) {
				let line = buffer[buffer.startIndex..<newline]
				buffer = buffer[buffer.index(after: newline)...]
				write(fd, handle(line: Data(line)))
			}
			if buffer.count > 64 * 1024 {
				write(fd, AgentReply.failure(.badRequest, "request too large"))
				return
			}
		}
	}

	private func handle(line: Data) -> [String: Any] {
		guard !line.isEmpty else { return AgentReply.failure(.badRequest, "empty request") }
		guard let request = try? JSONDecoder().decode(AgentRequest.self, from: line) else {
			return AgentReply.failure(.badRequest, "malformed JSON request")
		}
		switch request.op {
		case "status":
			return statusReply()
		case "config":
			// Identity, not secrets: served while locked so buddy can resolve a
			// session without a biometric prompt.
			return AgentReply.config(onMain { self.store.configDocument() })
		case "integrations":
			return AgentReply.integrations(onMain { self.store.integrationSummaries() })
		case "tokens":
			return AgentReply.tokens(tokenRows())
		case "lock":
			state.lockVault()
			return statusReply()
		case "unlock":
			// The tray is the only unlock UI; the CLI may not unlock us.
			return AgentReply.failure(.denied, "unlock from the Buddy Agent menu")
		case "token":
			return mint(request)
		default:
			return AgentReply.failure(.badRequest, "unknown op \(request.op)")
		}
	}

	private func statusReply() -> [String: Any] {
		let (count, integrity, managed) = onMain {
			(
				self.store.authCount,
				self.store.configIntegrity(
					unlocked: self.state.unlocked,
					presence: self.state.unlocked ? self.gate.presenceContext : nil
				).wireValue,
				self.store.managedPrefs.isEmpty ? nil : self.store.managedPrefs.summaryLine()
			)
		}
		return AgentReply.status(
			unlocked: state.unlocked,
			integrations: count,
			tokens: state.tokenCount,
			configIntegrity: integrity,
			managedPrefs: managed)
	}

	private func tokenRows() -> [[String: Any]] {
		let formatter = ISO8601DateFormatter()
		return state.tokenSummaries.map { row in
			[
				"auth": row.auth,
				"base_url": row.baseURL,
				"expires_at": formatter.string(from: row.expiresAt),
			]
		}
	}

	private func mint(_ request: AgentRequest) -> [String: Any] {
		let auth = (request.auth ?? "").trimmed
		let baseURL = (request.baseURL ?? "").trimmed
		let kind = (request.kind ?? "oauth").trimmed
		let who = callerName(request.client)
		guard !auth.isEmpty, !baseURL.isEmpty else {
			return logBuddyToken(
				AgentReply.failure(.badRequest, "token requires auth and base_url"),
				"\(who) requested a token without an auth or base URL; refused")
		}
		guard let integration = onMain({ self.store.integration(forAuth: auth) }) else {
			// Not ours: the CLI falls back to its own vault.
			return logBuddyToken(
				AgentReply.failure(.unsupported, "no integration named \(auth)"),
				"\(who) requested a token for \(auth); no integration by that name, refused")
		}
		let lockedReply = {
			self.logBuddyToken(
				AgentReply.failure(.locked, "Buddy Agent is locked — unlock it from the menu bar"),
				"\(who) requested a token for \(auth); Agent is locked and will not prompt, refused")
		}
		// True when this request opened the session. A context that fails a
		// second later is not stale; prompting again would not help.
		var freshSession = false
		if !state.unlocked {
			guard request.allowUnlock == true else { return lockedReply() }
			// Blocks this connection thread while the system prompt is up; the
			// CLI is waiting on the socket for exactly this.
			if let failure = unlockFailure(gate.authorize(
				reason: "get \(who) a token for \(auth)", keepCallerFocus: true), who: who, auth: auth)
			{
				return failure
			}
			freshSession = true
		}
		let key = "\(auth)@\(baseURL)"
		do {
			// One LA prompt owns the session. Keychain reads must use that
			// context with AuthenticationUIFail — otherwise SecItemCopyMatching
			// draws a second password sheet for the same userPresence item.
			var presence = gate.presenceContext
			if presence == nil {
				if let failure = unlockFailure(gate.authorize(
					reason: "get \(who) a token for \(auth)", keepCallerFocus: true), who: who, auth: auth)
				{
					return failure
				}
				presence = gate.presenceContext
				freshSession = true
			}
			// SecItem with an LAContext has to run on the main run loop. This
			// connection thread is what returns errSecInteractionNotAllowed.
			var block = onMain { self.store.configMintBlock(presence: presence) }
			if let current = block, current.staleSession, !freshSession {
				// The seal is intact; the Keychain stopped accepting the session
				// context. Behave as if the Agent had already been locked: drop the
				// session and prompt once. One sheet from the caller's side.
				AgentLog.shared.keychain(
					"unlock session not accepted by Keychain — relocking and prompting again")
				state.lockVault()
				guard request.allowUnlock == true else { return lockedReply() }
				if let failure = unlockFailure(gate.authorize(
					reason: "get \(who) a token for \(auth)", keepCallerFocus: true), who: who, auth: auth)
				{
					return failure
				}
				presence = gate.presenceContext
				freshSession = true
				block = onMain { self.store.configMintBlock(presence: presence) }
			}
			if let block {
				if block.staleSession, freshSession, AgentKeychainRecovery.recoverStuckSession() {
					return keychainRestartReply(who: who, auth: auth)
				}
				let why =
					block.staleSession
					? "Keychain will not release the config seal in this process — quit Buddy Agent and open it again"
					: block.reason
				return logBuddyToken(
					AgentReply.failure(.configTampered, why),
					"\(who) requested a token for \(auth); refused because the config seal check failed (\(why))")
			}
			if request.force != true, let cached = state.cachedToken(key: key) {
				return logBuddyToken(
					AgentReply.token(cached.value, expiresAt: cached.expiresAt),
					"\(who) requested a token for \(auth) at \(baseURL); sent the cached token (no Jamf API call)")
			}
			let minted = try TokenMinter.mint(
				integration: integration, baseURL: baseURL, kind: kind, presence: presence)
			state.store(
				token: CachedToken(
					auth: auth, baseURL: baseURL, value: minted.value,
					expiresAt: minted.expiresAt),
				key: key)
			AgentLog.shared.mint(
				"called the Jamf token API for \(auth) at \(baseURL) and received a token")
			return logBuddyToken(
				AgentReply.token(minted.value, expiresAt: minted.expiresAt),
				"\(who) requested a token for \(auth) at \(baseURL); sent the new token")
		} catch MintError.sessionRejected {
			if AgentKeychainRecovery.recoverStuckSession() {
				return keychainRestartReply(who: who, auth: auth)
			}
			return logBuddyToken(
				AgentReply.failure(
					.configTampered,
					"Keychain will not release the secret in this process — quit Buddy Agent and open it again"),
				"\(who) requested a token for \(auth); Keychain rejected the unlock and relaunch did not start")
		} catch MintError.unsupported(let why) {
			return logBuddyToken(
				AgentReply.failure(.unsupported, why),
				"\(who) requested a token for \(auth) at \(baseURL); Jamf token API failed: \(why)")
		} catch MintError.failed(let why) {
			return logBuddyToken(
				AgentReply.failure(.mintFailed, why),
				"\(who) requested a token for \(auth) at \(baseURL); Jamf token API failed: \(why)")
		} catch {
			return logBuddyToken(
				AgentReply.failure(.mintFailed, "\(error)"),
				"\(who) requested a token for \(auth) at \(baseURL); Jamf token API failed: \(error)")
		}
	}

	/// `cli` / `script` / `mcp` / `ide` become `buddy cli` and so on. Anything else,
	/// including a missing field from an older CLI, is just `buddy`.
	private func callerName(_ client: String?) -> String {
		guard let client, client == "cli" || client == "script" || client == "mcp" || client == "ide" else {
			return "buddy"
		}
		return "buddy \(client)"
	}

	private func keychainRestartReply(who: String, auth: String) -> [String: Any] {
		logBuddyToken(
			AgentReply.failure(
				.keychainRestart,
				"Buddy Agent is restarting to recover Keychain access — this command is retried once"),
			"\(who) requested a token for \(auth); relaunching so the data protection keychain can be opened")
	}

	/// Reply for a failed unlock prompt, or nil when the session opened.
	private func unlockFailure(_ outcome: UnlockOutcome, who: String, auth: String) -> [String: Any]? {
		switch outcome {
		case .unlocked:
			return nil
		case .declined:
			return logBuddyToken(
				AgentReply.failure(.unlockDeclined, "unlock declined"),
				"\(who) requested a token for \(auth); unlock declined, refused")
		case .timedOut:
			return logBuddyToken(
				AgentReply.failure(
					.unlockTimeout,
					"unlock prompt went unanswered for \(Int(UnlockGate.timeout))s"),
				"\(who) requested a token for \(auth); unlock timed out, refused")
		case .unavailable:
			let why =
				UnlockGate.promptingDisabled
				? "unlock prompting is off (BUDDY_AGENT_UNLOCK_TIMEOUT=0)"
				: "this Mac has no presence check available"
			return logBuddyToken(
				AgentReply.failure(.unlockUnavailable, "\(why) — unlock from the menu bar"),
				"\(who) requested a token for \(auth); unlock unavailable, refused")
		}
	}

	private func logBuddyToken(_ reply: [String: Any], _ message: String) -> [String: Any] {
		AgentLog.shared.buddy(message)
		return reply
	}

	/// onMain reads Agent store state, which the GUI mutates on the main queue.
	private func onMain<T>(_ body: @escaping () -> T) -> T {
		if Thread.isMainThread { return body() }
		return DispatchQueue.main.sync(execute: body)
	}

	private func write(_ fd: Int32, _ reply: [String: Any]) {
		let data = AgentReply.encode(reply)
		data.withUnsafeBytes { raw in
			var offset = 0
			while offset < raw.count {
				let n = Darwin.write(fd, raw.baseAddress!.advanced(by: offset), raw.count - offset)
				if n <= 0 { return }
				offset += n
			}
		}
	}
}

enum AgentServerError: Error, CustomStringConvertible {
	case posix(String, Int32)
	case message(String)

	var description: String {
		switch self {
		case .posix(let call, let code):
			return "\(call): \(String(cString: strerror(code)))"
		case .message(let text):
			return text
		}
	}
}
