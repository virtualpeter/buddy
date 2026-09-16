import Darwin
import Foundation
import Security

// sys/un.h socket options; the macros are not always imported into Swift.
private let solLocal: Int32 = 0
private let localPeerPID: Int32 = 0x002
private let localPeerToken: Int32 = 0x006

/// Codesign check on the accepted peer. Only a process signed by the same Team
/// ID, with an allowed bundle identifier, may ask the Agent to mint.
enum PeerCheck {
	static let allowedIdentifiers = ["virtualpete.buddy", "virtualpete.buddy.cli"]

	/// Team ID the Agent requires of its peers: BUDDY_AGENT_TEAM_ID when set,
	/// otherwise the Agent's own Team ID. Nil when the Agent itself is unsigned
	/// (`swift run`), which is only usable with the explicit dev escape.
	static let requiredTeamID: String? = {
		let env = ProcessInfo.processInfo.environment
		if let override = env["BUDDY_AGENT_TEAM_ID"], !override.isEmpty { return override }
		return ownTeamID()
	}()

	static var allowUnsigned: Bool {
		ProcessInfo.processInfo.environment["BUDDY_AGENT_ALLOW_UNSIGNED"] == "1"
	}

	/// Returns nil when the peer is acceptable, or a human-readable reason.
	static func reject(connection fd: Int32) -> String? {
		if allowUnsigned { return nil }
		guard let team = requiredTeamID else {
			return "agent is unsigned; set BUDDY_AGENT_TEAM_ID or BUDDY_AGENT_ALLOW_UNSIGNED=1"
		}
		guard let code = peerCode(fd: fd) else {
			return "could not identify calling process"
		}
		let identifiers = allowedIdentifiers.map { "identifier \"\($0)\"" }.joined(separator: " or ")
		let text = """
			anchor apple generic and certificate leaf[subject.OU] = "\(team)" and (\(identifiers))
			"""
		var requirement: SecRequirement?
		guard SecRequirementCreateWithString(text as CFString, [], &requirement) == errSecSuccess,
			let requirement
		else {
			return "could not build codesign requirement"
		}
		let status = SecCodeCheckValidity(code, [], requirement)
		guard status == errSecSuccess else {
			return "peer failed codesign requirement (OSStatus \(status))"
		}
		return nil
	}

	private static func peerCode(fd: Int32) -> SecCode? {
		if let attrs = auditAttributes(fd: fd) ?? pidAttributes(fd: fd) {
			var code: SecCode?
			if SecCodeCopyGuestWithAttributes(nil, attrs as CFDictionary, [], &code) == errSecSuccess {
				return code
			}
		}
		return nil
	}

	/// Audit token is preferred: it is not subject to PID reuse between accept()
	/// and the codesign lookup.
	private static func auditAttributes(fd: Int32) -> [CFString: Any]? {
		var token = audit_token_t()
		var length = socklen_t(MemoryLayout<audit_token_t>.size)
		let ok = withUnsafeMutablePointer(to: &token) { pointer -> Bool in
			getsockopt(fd, solLocal, localPeerToken, pointer, &length) == 0
		}
		guard ok, length == socklen_t(MemoryLayout<audit_token_t>.size) else { return nil }
		let data = withUnsafeBytes(of: token) { Data($0) }
		return [kSecGuestAttributeAudit: data as CFData]
	}

	private static func pidAttributes(fd: Int32) -> [CFString: Any]? {
		var pid: pid_t = 0
		var length = socklen_t(MemoryLayout<pid_t>.size)
		guard getsockopt(fd, solLocal, localPeerPID, &pid, &length) == 0, pid > 0 else { return nil }
		return [kSecGuestAttributePid: NSNumber(value: pid)]
	}

	private static func ownTeamID() -> String? {
		var code: SecCode?
		guard SecCodeCopySelf([], &code) == errSecSuccess, let code else { return nil }
		var staticCode: SecStaticCode?
		guard SecCodeCopyStaticCode(code, [], &staticCode) == errSecSuccess, let staticCode else {
			return nil
		}
		var info: CFDictionary?
		guard SecCodeCopySigningInformation(staticCode, SecCSFlags(rawValue: kSecCSSigningInformation), &info) == errSecSuccess,
			let dict = info as? [String: Any],
			let team = dict["teamid"] as? String, !team.isEmpty
		else { return nil }
		return team
	}
}
