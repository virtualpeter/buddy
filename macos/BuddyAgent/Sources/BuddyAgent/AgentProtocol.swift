import Foundation

/// Wire version reported by `status` (protocol shape, not marketing).
/// Marketing / About use CFBundleShortVersionString via AgentBranding.
let agentVersion = "0.3.0"

/// Error codes returned to the CLI. `unsupported` is the only code the Go side
/// treats as "fall back to the in-process vault"; everything else is fatal for
/// that call so a running Agent never silently hands secrets back to Go.
enum AgentErrorCode: String {
	case locked
	case unsupported
	case denied
	case badRequest = "bad_request"
	case mintFailed = "mint_failed"
	/// The presence prompt was shown and the person said no.
	case unlockDeclined = "unlock_declined"
	/// The prompt went unanswered for BUDDY_AGENT_UNLOCK_TIMEOUT seconds.
	case unlockTimeout = "unlock_timeout"
	/// No device-owner authentication on this Mac: the tray is the only way in.
	case unlockUnavailable = "unlock_unavailable"
}

/// One newline-delimited JSON request.
///
///   {"op":"status"} / {"op":"config"} / {"op":"integrations"} / {"op":"tokens"}
///   {"op":"token","auth":"…","base_url":"…","kind":"oauth|platform|basic",
///    "force":false,"allow_unlock":true}
///   {"op":"lock"}
///
/// Only `token` requires an unlocked Agent: config is identity, not secrets.
/// With `allow_unlock` the Agent raises its presence prompt and the caller
/// waits; without it, a locked Agent still answers `locked` immediately.
struct AgentRequest: Decodable {
	let op: String
	let auth: String?
	let baseURL: String?
	let kind: String?
	let force: Bool?
	let allowUnlock: Bool?

	enum CodingKeys: String, CodingKey {
		case op, auth, kind, force
		case baseURL = "base_url"
		case allowUnlock = "allow_unlock"
	}
}

/// Replies are built as dictionaries because `config` carries an arbitrary
/// config.yaml-shaped document. No reply ever carries a secret value.
enum AgentReply {
	static func status(unlocked: Bool, integrations: Int, tokens: Int) -> [String: Any] {
		[
			"ok": true,
			"present": true,
			"unlocked": unlocked,
			"version": agentVersion,
			"peer_check": PeerCheck.allowUnsigned ? "unsigned-allowed" : "codesign",
			"integration_count": integrations,
			"token_count": tokens,
			"presence": UnlockGate.mode.rawValue,
			"idle_lock_seconds": Int(AgentState.idleLockAfter),
			"unlock_timeout_seconds": Int(UnlockGate.timeout),
		]
	}

	static func config(_ document: [String: Any]) -> [String: Any] {
		["ok": true, "config": document]
	}

	static func integrations(_ rows: [[String: Any]]) -> [String: Any] {
		["ok": true, "integrations": rows]
	}

	static func tokens(_ rows: [[String: Any]]) -> [String: Any] {
		["ok": true, "tokens": rows]
	}

	static func token(_ value: String, expiresAt: Date) -> [String: Any] {
		[
			"ok": true,
			"access_token": value,
			"expires_at": ISO8601DateFormatter().string(from: expiresAt),
		]
	}

	static func failure(_ code: AgentErrorCode, _ message: String) -> [String: Any] {
		["ok": false, "error": code.rawValue, "message": message]
	}

	/// encode returns one newline-terminated JSON line.
	static func encode(_ reply: [String: Any]) -> Data {
		guard JSONSerialization.isValidJSONObject(reply),
			var data = try? JSONSerialization.data(withJSONObject: reply, options: [.sortedKeys])
		else {
			return Data(#"{"ok":false,"error":"mint_failed","message":"encode"}"#.utf8)
				+ Data("\n".utf8)
		}
		data.append(0x0A)
		return data
	}
}
