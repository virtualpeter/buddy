import Foundation
import os

enum AgentLogCategory: String, CaseIterable {
	case config
	case mint
	case buddy
	case keychain

	var label: String {
		switch self {
		case .config: return "Config"
		case .mint: return "Mint"
		case .buddy: return "Buddy"
		case .keychain: return "Keychain"
		}
	}
}

struct AgentLogEntry: Identifiable, Equatable {
	let id = UUID()
	let date: Date
	let category: AgentLogCategory
	let message: String
}

/// In-memory activity log for the Log tab; every entry is also written to Unified Log.
final class AgentLog: ObservableObject {
	static let shared = AgentLog()
	static let subsystem = "virtualpete.buddy.agent"

	@Published private(set) var entries: [AgentLogEntry] = []

	private let lock = NSLock()
	private var buffer: [AgentLogEntry] = []
	private let maxEntries = 500
	/// Cap in-app config diff blocks (full restore can be large).
	static let maxInAppMessageCharacters = 16_384
	private let loggers: [AgentLogCategory: Logger]

	private init() {
		var map: [AgentLogCategory: Logger] = [:]
		for category in AgentLogCategory.allCases {
			map[category] = Logger(subsystem: Self.subsystem, category: category.rawValue)
		}
		loggers = map
	}

	func config(_ message: String) { record(.config, inApp: message, system: message) }

	/// Full detail for the Log tab; `systemSummary` is what Unified Log records (counts/nature only).
	func config(detail: String, systemSummary: String) {
		record(.config, inApp: detail, system: systemSummary)
	}

	func mint(_ message: String) { record(.mint, inApp: message, system: message) }
	func buddy(_ message: String) { record(.buddy, inApp: message, system: message) }
	func keychain(_ message: String) { record(.keychain, inApp: message, system: message) }

	func clear() {
		lock.lock()
		buffer = []
		lock.unlock()
		publish([])
	}

	private func record(_ category: AgentLogCategory, inApp: String, system: String) {
		let stored = Self.clampInAppMessage(inApp)
		let unified = AgentLogRedaction.forSystemLog(system)
		let entry = AgentLogEntry(date: Date(), category: category, message: stored)
		loggers[category]?.info("\(unified, privacy: .public)")

		lock.lock()
		buffer.append(entry)
		if buffer.count > maxEntries {
			buffer.removeFirst(buffer.count - maxEntries)
		}
		let snapshot = buffer
		lock.unlock()

		publish(snapshot)
	}

	private static func clampInAppMessage(_ message: String) -> String {
		guard message.count > maxInAppMessageCharacters else { return message }
		let omitted = message.count - maxInAppMessageCharacters
		return String(message.prefix(maxInAppMessageCharacters))
			+ "\n… (truncated — \(omitted) characters omitted from Log tab)"
	}

	private func publish(_ snapshot: [AgentLogEntry]) {
		if Thread.isMainThread {
			entries = snapshot
		} else {
			DispatchQueue.main.async {
				self.entries = snapshot
			}
		}
	}
}
