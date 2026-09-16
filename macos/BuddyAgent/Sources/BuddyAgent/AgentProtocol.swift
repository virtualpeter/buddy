import Foundation

/// Wire version reported by `status`.
let agentVersion = "0.2.0"

/// Error codes returned to the CLI. `unsupported` is the only code the Go side
/// treats as "fall back to the in-process vault"; everything else is fatal for
/// that call so a running Agent never silently hands secrets back to Go.
enum AgentErrorCode: String {
	case locked
	case unsupported
	case denied
	case badRequest = "bad_request"
	case mintFailed = "mint_failed"
}

/// One newline-delimited JSON request.
///
///   {"op":"status"} / {"op":"config"} / {"op":"integrations"} / {"op":"tokens"}
///   {"op":"token","auth":"…","base_url":"…","kind":"oauth|platform|basic","force":false}
///   {"op":"lock"}
///
/// Only `token` requires an unlocked Agent: config is identity, not secrets.
struct AgentRequest: Decodable {
	let op: String
	let auth: String?
	let baseURL: String?
	let kind: String?
	let force: Bool?

	enum CodingKeys: String, CodingKey {
		case op, auth, kind, force
		case baseURL = "base_url"
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
