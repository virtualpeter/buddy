import Foundation

/// What buddy extracts from Jamf Pro API-client JSON or a Platform curl sample.
struct PastedClientCreds: Equatable {
	var clientId: String = ""
	var clientSecret: String = ""
	var clientName: String = ""
	var type: String = "oauth"
	var scope: String = ""
	var region: String = ""
	var instanceURL: String = ""
}

enum AgentPasteCredsError: Error, CustomStringConvertible {
	case empty
	case invalidJSON(String)
	case missingFields(String)

	var description: String {
		switch self {
		case .empty:
			return "empty paste — provide Jamf client JSON or curl"
		case .invalidJSON(let why):
			return "paste JSON: \(why)"
		case .missingFields(let why):
			return why
		}
	}
}

/// Parser matching `buddy cred add --paste` (`internal/buddy/paste_creds.go`).
enum AgentPasteCreds {
	/// Tooltip on paste fields in Setup and Quick setup.
	static let pasteFieldHelp =
		"When you create an API client or Platform integration in the Jamf admin UI, Jamf presents either a JSON blob or an example curl command. Copy that entire snippet from Jamf and paste it here — Buddy reads the client ID, secret, and related settings from it."

	static func parse(_ raw: String) throws -> PastedClientCreds {
		let normalized = normalizePastedClientBlob(raw)
		guard !normalized.isEmpty else { throw AgentPasteCredsError.empty }
		if normalized.hasPrefix("{") {
			return try parseJSON(normalized)
		}
		return try parseCurl(normalized)
	}

	private static func normalizePastedClientBlob(_ raw: String) -> String {
		var s = raw.trimmed
		if s.hasPrefix("\u{FEFF}") {
			s.removeFirst()
			s = s.trimmed
		}
		if let fenced = firstCodeFenceMatch(in: s) {
			s = fenced.trimmed
		}
		s = s.replacingOccurrences(of: "\\\r\n", with: " ")
		s = s.replacingOccurrences(of: "\\\n", with: " ")
		return s.trimmed
	}

	private static func firstCodeFenceMatch(in s: String) -> String? {
		guard
			let re = try? NSRegularExpression(
				pattern: "(?s)^```(?:json|bash|sh|shell|curl)?\\s*(.*)\\s*```$")
		else { return nil }
		let range = NSRange(s.startIndex..<s.endIndex, in: s)
		guard let m = re.firstMatch(in: s, range: range), m.numberOfRanges >= 2,
			let inner = Range(m.range(at: 1), in: s)
		else { return nil }
		return String(s[inner])
	}

	private static func parseJSON(_ s: String) throws -> PastedClientCreds {
		guard let data = s.data(using: .utf8),
			let raw = try JSONSerialization.jsonObject(with: data) as? [String: Any]
		else {
			throw AgentPasteCredsError.invalidJSON("top level is not an object")
		}
		var out = PastedClientCreds(
			clientId: firstPastedString(raw, "client_id", "clientId"),
			clientSecret: firstPastedString(raw, "client_secret", "clientSecret"),
			clientName: firstPastedString(raw, "client_name", "clientName"),
			type: "oauth")
		let tokenURL = firstPastedString(raw, "url", "token_url", "tokenUrl")
		if !tokenURL.isEmpty {
			applyPastedURL(&out, tokenURL)
		}
		guard !out.clientId.isEmpty, !out.clientSecret.isEmpty else {
			throw AgentPasteCredsError.missingFields("paste JSON needs client_id and client_secret")
		}
		return out
	}

	private static func parseCurl(_ s: String) throws -> PastedClientCreds {
		var out = PastedClientCreds(type: "oauth")
		if let u = firstCurlURL(in: s), !u.isEmpty {
			applyPastedURL(&out, u)
		} else if let u = firstHTTPURL(in: s), !u.isEmpty {
			applyPastedURL(&out, u)
		}
		let form = curlFormFields(in: s)
		out.clientId = (form["client_id"] ?? form["clientId"] ?? "").trimmed
		out.clientSecret = (form["client_secret"] ?? form["clientSecret"] ?? "").trimmed
		guard !out.clientId.isEmpty, !out.clientSecret.isEmpty else {
			throw AgentPasteCredsError.missingFields(
				"paste curl needs client_id and client_secret (--data)")
		}
		if out.type.isEmpty { out.type = "oauth" }
		return out
	}

	private static func firstPastedString(_ raw: [String: Any], _ keys: String...) -> String {
		for key in keys {
			if let value = raw[key] as? String {
				let trimmed = value.trimmed
				if !trimmed.isEmpty { return trimmed }
			}
		}
		return ""
	}

	private static func firstCurlURL(in s: String) -> String? {
		guard
			let re = try? NSRegularExpression(
				pattern: "(?i)--url(?:=|\\s+)(?:'([^']+)'|\"([^\"]+)\"|(\\S+))")
		else { return nil }
		return firstCurlCapture(re, in: s)
	}

	private static func curlFormFields(in s: String) -> [String: String] {
		guard
			let re = try? NSRegularExpression(
				pattern: "(?i)(?:--data(?:-urlencode|-raw)?|-d)(?:=|\\s+)(?:'([^']*)'|\"([^\"]*)\"|(\\S+))")
		else { return [:] }
		let range = NSRange(s.startIndex..<s.endIndex, in: s)
		var form: [String: String] = [:]
		for m in re.matches(in: s, range: range) {
			guard let chunk = firstCapture(m, in: s), !chunk.isEmpty else { continue }
			mergeFormPairs(into: &form, chunk)
		}
		return form
	}

	private static func firstCurlCapture(_ re: NSRegularExpression, in s: String) -> String? {
		let range = NSRange(s.startIndex..<s.endIndex, in: s)
		guard let m = re.firstMatch(in: s, range: range) else { return nil }
		guard let value = firstCapture(m, in: s) else { return nil }
		return value.trimmingCharacters(in: CharacterSet(charactersIn: "\\"))
	}

	private static func firstCapture(_ m: NSTextCheckingResult, in s: String) -> String? {
		for idx in 1..<m.numberOfRanges {
			guard let r = Range(m.range(at: idx), in: s) else { continue }
			let value = String(s[r]).trimmed
			if !value.isEmpty { return value }
		}
		return nil
	}

	private static func firstHTTPURL(in s: String) -> String? {
		guard let re = try? NSRegularExpression(pattern: "https?://[^\\s'\"\\\\]+") else {
			return nil
		}
		let range = NSRange(s.startIndex..<s.endIndex, in: s)
		guard let m = re.firstMatch(in: s, range: range),
			let r = Range(m.range, in: s)
		else { return nil }
		return String(s[r])
	}

	private static func mergeFormPairs(into dst: inout [String: String], _ raw: String) {
		for part in raw.split(separator: "&") {
			let piece = String(part).trimmed
			guard !piece.isEmpty else { continue }
			let bits = piece.split(separator: "=", maxSplits: 1).map(String.init)
			guard bits.count == 2 else { continue }
			let key = bits[0].trimmed
			var value = bits[1].trimmed
			if let decoded = value.removingPercentEncoding { value = decoded }
			if !key.isEmpty { dst[key] = value }
		}
	}

	private static func applyPastedURL(_ out: inout PastedClientCreds, _ raw: String) {
		let url = raw.trimmed.trimmingCharacters(in: CharacterSet(charactersIn: "\\"))
		if looksLikeOrgPlatformGateway(url) {
			out.type = "platform"
			out.scope = "organization"
			return
		}
		if looksLikePlatformGateway(url) || looksLikeRetiredPlatformGateway(url) {
			out.type = "platform"
			let region = regionFromGatewayURL(url)
			if !region.isEmpty {
				out.region = region
			}
			return
		}
		guard let parsed = URL(string: url), let host = parsed.host, !host.isEmpty,
			let scheme = parsed.scheme, !scheme.isEmpty
		else { return }
		out.type = "oauth"
		out.instanceURL = "\(scheme)://\(host)"
	}

	private static func gatewayHost(_ rawURL: String) -> String {
		var u = rawURL.trimmed.lowercased()
		if u.hasPrefix("https://") { u.removeFirst("https://".count) }
		else if u.hasPrefix("http://") { u.removeFirst("http://".count) }
		if let slash = u.firstIndex(of: "/") {
			u = String(u[..<slash])
		}
		return u
	}

	private static func regionFromGatewayURL(_ rawURL: String) -> String {
		let host = gatewayHost(rawURL)
		if host == "api.jamfcloud.com" { return "" }
		let suffix = ".api.jamfcloud.com"
		guard host.hasSuffix(suffix) else { return "" }
		return String(host.dropLast(suffix.count))
	}

	private static func looksLikeOrgPlatformGateway(_ rawURL: String) -> Bool {
		gatewayHost(rawURL) == "api.jamfcloud.com"
	}

	private static func looksLikeRetiredPlatformGateway(_ rawURL: String) -> Bool {
		let host = gatewayHost(rawURL)
		return host == "apigw.jamf.com" || host.hasSuffix(".apigw.jamf.com")
	}

	private static func looksLikePlatformGateway(_ rawURL: String) -> Bool {
		!regionFromGatewayURL(rawURL).isEmpty || looksLikeOrgPlatformGateway(rawURL)
	}
}
