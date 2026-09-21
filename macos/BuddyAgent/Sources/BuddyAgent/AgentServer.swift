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
	private(set) var socketPath: String = AgentPaths.socketPath

	init(state: AgentState, store: AgentStore, gate: UnlockGate) {
		self.state = state
		self.store = store
		self.gate = gate
	}

	func start() throws {
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

	func stop() {
		if listenFD >= 0 {
			close(listenFD)
			listenFD = -1
		}
		unlink(socketPath)
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
		guard !auth.isEmpty, !baseURL.isEmpty else {
			return logBuddyToken(
				AgentReply.failure(.badRequest, "token requires auth and base_url"),
				"token rejected: missing auth or base_url")
		}
		guard let integration = onMain({ self.store.integration(forAuth: auth) }) else {
			// Not ours: the CLI falls back to its own vault.
			return logBuddyToken(
				AgentReply.failure(.unsupported, "no integration named \(auth)"),
				"token rejected auth=\(auth): unknown integration")
		}
		if !state.unlocked {
			guard request.allowUnlock == true else {
				return logBuddyToken(
					AgentReply.failure(
						.locked, "Buddy Agent is locked — unlock it from the menu bar"),
					"token rejected auth=\(auth): locked")
			}
			// Blocks this connection thread while the system prompt is up; the
			// CLI is waiting on the socket for exactly this.
			switch gate.authorize(reason: "mint a Jamf token for \(auth)") {
			case .unlocked:
				break
			case .declined:
				return logBuddyToken(
					AgentReply.failure(.unlockDeclined, "unlock declined"),
					"token rejected auth=\(auth): unlock declined")
			case .timedOut:
				return logBuddyToken(
					AgentReply.failure(
						.unlockTimeout,
						"unlock prompt went unanswered for \(Int(UnlockGate.timeout))s"),
					"token rejected auth=\(auth): unlock timed out")
			case .unavailable:
				let why =
					UnlockGate.promptingDisabled
					? "unlock prompting is off (BUDDY_AGENT_UNLOCK_TIMEOUT=0)"
					: "this Mac has no presence check available"
				return logBuddyToken(
					AgentReply.failure(
						.unlockUnavailable, "\(why) — unlock from the menu bar"),
					"token rejected auth=\(auth): unlock unavailable")
			}
		}
		let key = "\(auth)@\(baseURL)"
		do {
			// One LA prompt owns the session. Keychain reads must use that
			// context with AuthenticationUIFail — otherwise SecItemCopyMatching
			// draws a second password sheet for the same userPresence item.
			var presence = gate.presenceContext
			if presence == nil {
				switch gate.authorize(reason: "mint a Jamf token for \(auth)") {
				case .unlocked:
					presence = gate.presenceContext
				case .declined:
					return logBuddyToken(
						AgentReply.failure(.unlockDeclined, "unlock declined"),
						"token rejected auth=\(auth): unlock declined")
				case .timedOut:
					return logBuddyToken(
						AgentReply.failure(
							.unlockTimeout,
							"unlock prompt went unanswered for \(Int(UnlockGate.timeout))s"),
						"token rejected auth=\(auth): unlock timed out")
				case .unavailable:
					return logBuddyToken(
						AgentReply.failure(
							.unlockUnavailable, "presence unavailable — unlock from the menu bar"),
						"token rejected auth=\(auth): unlock unavailable")
				}
			}
			if let why = AgentConfigIntegrity.mintBlockedReason(
				configPath: onMain { self.store.path }, presence: presence)
			{
				return logBuddyToken(
					AgentReply.failure(.configTampered, why),
					"token rejected auth=\(auth): config integrity")
			}
			if request.force != true, let cached = state.cachedToken(key: key) {
				return logBuddyToken(
					AgentReply.token(cached.value, expiresAt: cached.expiresAt),
					"token cache hit auth=\(auth) base_url=\(baseURL)")
			}
			let minted = try TokenMinter.mint(
				integration: integration, baseURL: baseURL, kind: kind, presence: presence)
			state.store(
				token: CachedToken(
					auth: auth, baseURL: baseURL, value: minted.value,
					expiresAt: minted.expiresAt),
				key: key)
			AgentLog.shared.mint("buddy minted auth=\(auth) base_url=\(baseURL)")
			return logBuddyToken(
				AgentReply.token(minted.value, expiresAt: minted.expiresAt),
				"token minted auth=\(auth) base_url=\(baseURL)")
		} catch MintError.unsupported(let why) {
			return logBuddyToken(
				AgentReply.failure(.unsupported, why),
				"token failed auth=\(auth) base_url=\(baseURL): \(why)")
		} catch MintError.failed(let why) {
			return logBuddyToken(
				AgentReply.failure(.mintFailed, why),
				"token failed auth=\(auth) base_url=\(baseURL): \(why)")
		} catch {
			return logBuddyToken(
				AgentReply.failure(.mintFailed, "\(error)"),
				"token failed auth=\(auth) base_url=\(baseURL): \(error)")
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
