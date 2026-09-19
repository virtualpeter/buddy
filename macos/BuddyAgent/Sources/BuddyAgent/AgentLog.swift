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
	private let loggers: [AgentLogCategory: Logger]

	private init() {
		var map: [AgentLogCategory: Logger] = [:]
		for category in AgentLogCategory.allCases {
			map[category] = Logger(subsystem: Self.subsystem, category: category.rawValue)
		}
		loggers = map
	}

	func config(_ message: String) { record(.config, message) }
	func mint(_ message: String) { record(.mint, message) }
	func buddy(_ message: String) { record(.buddy, message) }
	func keychain(_ message: String) { record(.keychain, message) }

	func clear() {
		lock.lock()
		buffer = []
		lock.unlock()
		publish([])
	}

	private func record(_ category: AgentLogCategory, _ message: String) {
		let entry = AgentLogEntry(date: Date(), category: category, message: message)
		loggers[category]?.info("\(message, privacy: .public)")

		lock.lock()
		buffer.append(entry)
		if buffer.count > maxEntries {
			buffer.removeFirst(buffer.count - maxEntries)
		}
		let snapshot = buffer
		lock.unlock()

		publish(snapshot)
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
