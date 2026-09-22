import Foundation

/// Unified Log (os.log) must not carry config secrets or identifiers; the Log tab may show more.
enum AgentLogRedaction {
	private static let sensitivePrefixes = [
		"added auth ",
		"updated auth ",
		"removed auth ",
		"renamed auth ",
		"stored secret for auth ",
		"added workplace ",
		"updated workplace ",
		"removed workplace ",
		"renamed workplace ",
		"added organization ",
		"default workplace set to ",
	]

	static func forSystemLog(_ message: String) -> String {
		message
			.split(separator: "\n", omittingEmptySubsequences: false)
			.map { redactLine(String($0)) }
			.joined(separator: "\n")
	}

	private static func redactLine(_ line: String) -> String {
		for prefix in sensitivePrefixes where line.hasPrefix(prefix) {
			return prefix + "<redacted>"
		}
		var s = line
		s = replacePattern(s, #"auth=[^\s]+"#, "auth=<redacted>")
		s = replacePattern(s, #"base_url=[^\s]+"#, "base_url=<redacted>")
		s = redactJSONStringValues(
			s,
			keys: [
				"clientId", "clientID", "tenantId", "tenantID", "environmentId", "environmentID",
				"username", "url", "scope", "access",
			])
		s = replacePattern(
			s,
			#"[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}"#,
			"<uuid>")
		return s
	}

	private static func redactJSONStringValues(_ text: String, keys: [String]) -> String {
		var out = text
		for key in keys {
			let pattern = #""\#(key)"\s*:\s*"([^"\\]|\\.)*""#
			out = replacePattern(out, pattern, "\"\(key)\": \"<redacted>\"")
		}
		return out
	}

	private static func replacePattern(_ text: String, _ pattern: String, _ template: String) -> String {
		guard let regex = try? NSRegularExpression(pattern: pattern) else { return text }
		let range = NSRange(text.startIndex..<text.endIndex, in: text)
		return regex.stringByReplacingMatches(in: text, range: range, withTemplate: template)
	}
}
