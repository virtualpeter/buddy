import CryptoKit
import Foundation

/// Copy of `agent-config.json` captured whenever the Keychain seal is updated,
/// plus metadata for diagnosing unexpected `tampered` integrity state.
enum AgentConfigLastGood {
	private static let snapshotName = "agent-config.last-good.json"
	private static let metaName = "agent-config.last-good.meta.json"

	struct Meta: Codable, Equatable {
		var sealedAt: Date
		var configModifiedAt: Date?
		var agentRevision: String?
		/// First time the Agent observed `tampered` while unlocked (this seal epoch).
		var tamperObservedAt: Date?
	}

	enum DiffLine: Equatable {
		case unchanged(String)
		case removed(String)
		case added(String)
	}

	/// One review decision. A removed line immediately followed by its replacement is one change.
	struct ReviewLine: Identifiable, Equatable {
		enum Change: Equatable {
			case added(String)
			case removed(String)
			case replaced(old: String, new: String)
		}

		let id: Int
		/// Diff indexes covered by this decision.
		let indexes: [Int]
		let change: Change
	}

	enum LineDecision: Equatable {
		/// Keep the outside edit (added line stays, removed line stays gone).
		case accept
		/// Restore the last sealed config (added line drops, removed line returns).
		case revert
	}

	struct Diagnostics: Equatable {
		var integrity: AgentConfigIntegrity.State
		var snapshotMissing: Bool
		var meta: Meta?
		var currentModifiedAt: Date?
		var normalizedDiff: [DiffLine]
		var onlyFormattingDrift: Bool
		/// On-disk bytes differ from the last-good snapshot (no Keychain needed).
		var fileDriftSinceSnapshot: Bool
		/// The drift is the Agent's own save whose Keychain seal did not read back.
		var agentAttributedDrift: Bool

		var hasIntegrityConcern: Bool {
			switch integrity {
			case .tampered, .missing: return true
			case .sealed: return fileDriftSinceSnapshot
			case .ok, .unavailable: return false
			}
		}

		var diffChangeCount: Int {
			normalizedDiff.filter {
				if case .unchanged = $0 { return false }
				return true
			}.count
		}

		var needsLineReview: Bool {
			if agentAttributedDrift { return false }
			return !snapshotMissing && !onlyFormattingDrift && diffChangeCount > 0
		}

		static let pending = Diagnostics(
			integrity: .unavailable,
			snapshotMissing: true,
			meta: nil,
			currentModifiedAt: nil,
			normalizedDiff: [],
			onlyFormattingDrift: false,
			fileDriftSinceSnapshot: false,
			agentAttributedDrift: false)
	}

	/// True when normalized JSON differs from the last-good snapshot, so a save would
	/// bless every outside edit at once.
	static func contentDiff(oldText: String, newText: String) -> [DiffLine] {
		lineDiff(from: normalizedJSONText(oldText), to: normalizedJSONText(newText))
	}

	static func normalizedFileDigest(at configPath: URL) -> String? {
		guard let text = try? String(contentsOf: configPath, encoding: .utf8) else { return nil }
		return normalizedDigest(text)
	}

	static func normalizedDigest(_ text: String) -> String {
		let norm = normalizedJSONText(text)
		let hash = SHA256.hash(data: Data(norm.utf8))
		return hash.map { String(format: "%02x", $0) }.joined()
	}

	static func resetOutsideEditLogEpoch() {
		lastLoggedOutsideEditDigest = nil
	}

	static func requiresLineReview(configPath: URL) -> Bool {
		let snap = snapshotURL(near: configPath)
		guard FileManager.default.fileExists(atPath: snap.path) else { return false }
		guard FileManager.default.fileExists(atPath: configPath.path) else { return false }
		let oldText = (try? String(contentsOf: snap, encoding: .utf8)) ?? ""
		let newText = (try? String(contentsOf: configPath, encoding: .utf8)) ?? ""
		return normalizedJSONText(oldText) != normalizedJSONText(newText)
	}

	static func reviewLines(in diff: [DiffLine]) -> [ReviewLine] {
		var result: [ReviewLine] = []
		var index = 0
		while index < diff.count {
			if case .unchanged = diff[index] {
				index += 1
				continue
			}
			var removed: [(Int, String)] = []
			var added: [(Int, String)] = []
			hunk: while index < diff.count {
				switch diff[index] {
				case .removed(let text):
					if !added.isEmpty { break hunk }
					removed.append((index, text))
					index += 1
				case .added(let text):
					added.append((index, text))
					index += 1
				case .unchanged:
					break hunk
				}
			}
			let pairCount = min(removed.count, added.count)
			for pair in 0..<pairCount {
				let (removedIndex, old) = removed[pair]
				let (addedIndex, new) = added[pair]
				result.append(
					ReviewLine(
						id: removedIndex,
						indexes: [removedIndex, addedIndex],
						change: .replaced(old: old, new: new)))
			}
			for (removedIndex, text) in removed.dropFirst(pairCount) {
				result.append(
					ReviewLine(id: removedIndex, indexes: [removedIndex], change: .removed(text)))
			}
			for (addedIndex, text) in added.dropFirst(pairCount) {
				result.append(
					ReviewLine(id: addedIndex, indexes: [addedIndex], change: .added(text)))
			}
		}
		return result
	}

	/// Builds the reviewed document. Nil until every change has a decision.
	/// A replacement pair shares one decision: accept keeps the new line, revert keeps the old.
	static func resolve(diff: [DiffLine], decisions: [Int: LineDecision]) -> String? {
		var decisionByIndex: [Int: LineDecision] = [:]
		for item in reviewLines(in: diff) {
			guard let decision = decisions[item.id] else { return nil }
			for index in item.indexes {
				decisionByIndex[index] = decision
			}
		}
		var lines: [String] = []
		for (index, line) in diff.enumerated() {
			switch line {
			case .unchanged(let text):
				lines.append(text)
			case .added(let text):
				switch decisionByIndex[index] {
				case .accept: lines.append(text)
				case .revert: break
				case nil: return nil
				}
			case .removed(let text):
				switch decisionByIndex[index] {
				case .revert: lines.append(text)
				case .accept: break
				case nil: return nil
				}
			}
		}
		return lines.joined(separator: "\n")
	}

	static func snapshotURL(near configPath: URL) -> URL {
		configPath.deletingLastPathComponent().appendingPathComponent(snapshotName)
	}

	static func metaURL(near configPath: URL) -> URL {
		configPath.deletingLastPathComponent().appendingPathComponent(metaName)
	}

	/// Records bytes on disk after a successful seal (or backfill when verify is ok).
	@discardableResult
	static func record(configPath: URL, sealedAt: Date = Date()) -> Bool {
		guard let data = try? Data(contentsOf: configPath) else { return false }
		let snap = snapshotURL(near: configPath)
		let metaPath = metaURL(near: configPath)
		do {
			try AgentPaths.prepareSocketDirectory()
			try data.write(to: snap, options: [.atomic])
			try? FileManager.default.setAttributes(
				[.posixPermissions: 0o600], ofItemAtPath: snap.path)
			var meta = (try? loadMeta(at: metaPath)) ?? Meta(sealedAt: sealedAt)
			meta.sealedAt = sealedAt
			meta.configModifiedAt = fileModificationDate(configPath)
			meta.agentRevision = AgentBranding.revision
			meta.tamperObservedAt = nil
			try writeMeta(meta, to: metaPath)
			resetOutsideEditLogEpoch()
			return true
		} catch {
			AgentLog.shared.config("could not write last-good config snapshot: \(error)")
			return false
		}
	}

	static func loadMeta(near configPath: URL) -> Meta? {
		try? loadMeta(at: metaURL(near: configPath))
	}

	static func diagnostics(
		configPath: URL,
		integrity: AgentConfigIntegrity.State,
		observeTamper: Bool,
		agentAttributedDrift: Bool = false
	) -> Diagnostics {
		let snap = snapshotURL(near: configPath)
		let snapshotMissing = !FileManager.default.fileExists(atPath: snap.path)
		let fileDrift =
			!snapshotMissing
			&& !fileBytesMatch(snapshot: snap, configPath: configPath)
		if observeTamper, !agentAttributedDrift, (integrity == .tampered || fileDrift) {
			noteTamperObserved(near: configPath)
		}
		let meta = loadMeta(near: configPath)
		let currentModified = fileModificationDate(configPath)
		let oldText = snapshotMissing ? "" : (try? String(contentsOf: snap, encoding: .utf8)) ?? ""
		let newText = (try? String(contentsOf: configPath, encoding: .utf8)) ?? ""
		let oldNorm = normalizedJSONText(oldText)
		let newNorm = normalizedJSONText(newText)
		let onlyFormatting = !snapshotMissing && oldNorm == newNorm && oldText != newText
		let diff = snapshotMissing ? [] : lineDiff(from: oldNorm, to: newNorm)
		return Diagnostics(
			integrity: integrity,
			snapshotMissing: snapshotMissing,
			meta: meta,
			currentModifiedAt: currentModified,
			normalizedDiff: diff,
			onlyFormattingDrift: onlyFormatting,
			fileDriftSinceSnapshot: fileDrift,
			agentAttributedDrift: agentAttributedDrift
		)
	}

	// MARK: - private

	private static func loadMeta(at url: URL) throws -> Meta {
		let data = try Data(contentsOf: url)
		let decoder = JSONDecoder()
		decoder.dateDecodingStrategy = .iso8601
		return try decoder.decode(Meta.self, from: data)
	}

	private static func writeMeta(_ meta: Meta, to url: URL) throws {
		let encoder = JSONEncoder()
		encoder.outputFormatting = [.prettyPrinted, .sortedKeys]
		encoder.dateEncodingStrategy = .iso8601
		let data = try encoder.encode(meta)
		try data.write(to: url, options: [.atomic])
		try? FileManager.default.setAttributes(
			[.posixPermissions: 0o600], ofItemAtPath: url.path)
	}

	private static func fileBytesMatch(snapshot: URL, configPath: URL) -> Bool {
		guard let a = try? Data(contentsOf: snapshot), let b = try? Data(contentsOf: configPath) else {
			return false
		}
		return a == b
	}

	private static func fileModificationDate(_ url: URL) -> Date? {
		let attrs = try? FileManager.default.attributesOfItem(atPath: url.path)
		return attrs?[.modificationDate] as? Date
	}

	private static var lastLoggedOutsideEditDigest: String?

	private static func noteTamperObserved(near configPath: URL) {
		let metaPath = metaURL(near: configPath)
		if var meta = try? loadMeta(at: metaPath) {
			guard meta.tamperObservedAt == nil else { return }
			meta.tamperObservedAt = Date()
			try? writeMeta(meta, to: metaPath)
		}
		logOutsideEditOnce(configPath: configPath)
	}

	private static func logOutsideEditOnce(configPath: URL) {
		guard let digest = normalizedFileDigest(at: configPath) else { return }
		guard digest != lastLoggedOutsideEditDigest else { return }
		lastLoggedOutsideEditDigest = digest
		AgentConfigChangeLog.logOutsideEdit(configPath: configPath)
	}

	private static func normalizedJSONText(_ text: String) -> String {
		guard let data = text.data(using: .utf8),
			let object = try? JSONSerialization.jsonObject(with: data),
			let pretty = try? JSONSerialization.data(
				withJSONObject: object,
				options: [.prettyPrinted, .sortedKeys]
			),
			let out = String(data: pretty, encoding: .utf8)
		else {
			return text
		}
		return out
	}

	/// Ordered line diff (LCS) for small config files.
	static func lineDiff(from oldText: String, to newText: String) -> [DiffLine] {
		let oldLines = oldText.split(separator: "\n", omittingEmptySubsequences: false).map(String.init)
		let newLines = newText.split(separator: "\n", omittingEmptySubsequences: false).map(String.init)
		if oldLines == newLines { return oldLines.map { .unchanged($0) } }
		let lcs = longestCommonSubsequence(oldLines, newLines)
		var result: [DiffLine] = []
		var oi = 0
		var ni = 0
		for common in lcs {
			while oi < oldLines.count, oldLines[oi] != common {
				result.append(.removed(oldLines[oi]))
				oi += 1
			}
			while ni < newLines.count, newLines[ni] != common {
				result.append(.added(newLines[ni]))
				ni += 1
			}
			result.append(.unchanged(common))
			oi += 1
			ni += 1
		}
		while oi < oldLines.count {
			result.append(.removed(oldLines[oi]))
			oi += 1
		}
		while ni < newLines.count {
			result.append(.added(newLines[ni]))
			ni += 1
		}
		return result
	}

	private static func longestCommonSubsequence(_ a: [String], _ b: [String]) -> [String] {
		let n = a.count
		let m = b.count
		if n == 0 || m == 0 { return [] }
		var dp = Array(repeating: Array(repeating: 0, count: m + 1), count: n + 1)
		for i in 1...n {
			for j in 1...m {
				if a[i - 1] == b[j - 1] {
					dp[i][j] = dp[i - 1][j - 1] + 1
				} else {
					dp[i][j] = max(dp[i - 1][j], dp[i][j - 1])
				}
			}
		}
		var i = n
		var j = m
		var seq: [String] = []
		while i > 0, j > 0 {
			if a[i - 1] == b[j - 1] {
				seq.append(a[i - 1])
				i -= 1
				j -= 1
			} else if dp[i - 1][j] >= dp[i][j - 1] {
				i -= 1
			} else {
				j -= 1
			}
		}
		return seq.reversed()
	}
}
