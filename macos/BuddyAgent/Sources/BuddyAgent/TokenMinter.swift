import Foundation
import LocalAuthentication

/// Mints Jamf bearers from the Agent's own credentials. The secret is read from
/// the Agent Keychain inside this call and never leaves the process: only the
/// bearer and its expiry go back over the socket.
enum MintError: Error {
	case unsupported(String)
	case failed(String)
	/// Presence succeeded and the item exists, but this process cannot read it.
	case sessionRejected
}

struct MintedToken {
	let value: String
	let expiresAt: Date
}

enum TokenMinter {
	/// `presence` is the LAContext UnlockGate authenticated for this session.
	/// On a signed Agent it is what releases the keychain item; without it the
	/// read fails rather than raising a second prompt.
	static func mint(
		integration: Integration, baseURL: String, kind: String, presence: LAContext? = nil
	) throws -> MintedToken {
		guard let secret = try secret(for: integration, presence: presence) else {
			throw MintError.unsupported(
				"no secret stored for \(integration.authName) — add it in the Buddy Agent window")
		}
		let request = try tokenRequest(
			integration: integration, baseURL: baseURL, kind: kind, secret: secret)
		let (data, response) = try send(request)
		guard let http = response as? HTTPURLResponse else {
			throw MintError.failed("no HTTP response")
		}
		guard (200..<300).contains(http.statusCode) else {
			throw MintError.failed("token endpoint returned \(http.statusCode)")
		}
		guard let json = try? JSONSerialization.jsonObject(with: data) as? [String: Any] else {
			throw MintError.failed("token endpoint returned no JSON")
		}
		// OAuth / Platform: access_token + expires_in. Classic basic: token + expires.
		if let token = json["access_token"] as? String, !token.isEmpty {
			let expiresIn = (json["expires_in"] as? Double) ?? 1200
			return MintedToken(value: token, expiresAt: Date().addingTimeInterval(expiresIn))
		}
		if let token = json["token"] as? String, !token.isEmpty {
			let expiry = (json["expires"] as? String).flatMap(parseJamfExpiry)
			return MintedToken(value: token, expiresAt: expiry ?? Date().addingTimeInterval(1800))
		}
		throw MintError.failed("token endpoint returned no access_token")
	}

	private static func secret(for integration: Integration, presence: LAContext?) throws -> String? {
		let read = {
			AgentKeychain.read(
				auth: integration.authName, kind: integration.secretKind, context: presence)
		}
		let stored = Thread.isMainThread ? read() : DispatchQueue.main.sync(execute: read)
		switch stored {
		case .value(let secret):
			return secret
		case .sessionRejected:
			throw MintError.sessionRejected
		case .missing:
			break
		}
		// Dev fallback for a run with no stored secret yet. Refused on a signed
		// Agent: an env var is not something a presence prompt can gate.
		guard PeerCheck.allowUnsigned else { return nil }
		let env = ProcessInfo.processInfo.environment
		if integration.secretKind == .clientSecret,
			let value = env["BUDDY_AGENT_SPIKE_CLIENT_SECRET"], !value.isEmpty
		{
			return value
		}
		return nil
	}

	private static func tokenRequest(
		integration: Integration, baseURL: String, kind: String, secret: String
	) throws -> URLRequest {
		guard var components = URLComponents(string: baseURL) else {
			throw MintError.failed("bad base_url")
		}
		let base = components.path.hasSuffix("/") ? String(components.path.dropLast()) : components.path
		var request: URLRequest

		switch kind {
		case "basic":
			guard !integration.username.trimmed.isEmpty else {
				throw MintError.unsupported("no username stored for \(integration.authName)")
			}
			components.path = base + "/api/v1/auth/token"
			guard let url = components.url else { throw MintError.failed("bad base_url") }
			request = URLRequest(url: url)
			request.httpMethod = "POST"
			let pair = Data("\(integration.username.trimmed):\(secret)".utf8).base64EncodedString()
			request.setValue("Basic \(pair)", forHTTPHeaderField: "Authorization")
		case "oauth", "platform":
			guard !integration.clientID.trimmed.isEmpty else {
				throw MintError.unsupported("no client id stored for \(integration.authName)")
			}
			components.path = base + (kind == "platform" ? "/auth/token" : "/api/oauth/token")
			guard let url = components.url else { throw MintError.failed("bad base_url") }
			var body = URLComponents()
			body.queryItems = [
				URLQueryItem(name: "grant_type", value: "client_credentials"),
				URLQueryItem(name: "client_id", value: integration.clientID.trimmed),
				URLQueryItem(name: "client_secret", value: secret),
			]
			request = URLRequest(url: url)
			request.httpMethod = "POST"
			request.setValue("application/x-www-form-urlencoded", forHTTPHeaderField: "Content-Type")
			request.httpBody = Data((body.percentEncodedQuery ?? "").utf8)
		default:
			throw MintError.unsupported("unknown auth kind \(kind)")
		}
		request.setValue("application/json", forHTTPHeaderField: "Accept")
		request.timeoutInterval = 30
		return request
	}

	/// Jamf Classic returns an ISO-8601 string with milliseconds.
	private static func parseJamfExpiry(_ text: String) -> Date? {
		let withFractional = ISO8601DateFormatter()
		withFractional.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
		if let date = withFractional.date(from: text) { return date }
		return ISO8601DateFormatter().date(from: text)
	}

	private static func send(_ request: URLRequest) throws -> (Data, URLResponse) {
		let semaphore = DispatchSemaphore(value: 0)
		var result: Result<(Data, URLResponse), Error> = .failure(MintError.failed("no result"))
		URLSession.shared.dataTask(with: request) { data, response, error in
			if let error {
				result = .failure(error)
			} else if let data, let response {
				result = .success((data, response))
			}
			semaphore.signal()
		}.resume()
		semaphore.wait()
		return try result.get()
	}
}
