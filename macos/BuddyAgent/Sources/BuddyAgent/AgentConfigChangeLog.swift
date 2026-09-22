import Foundation

/// Activity-log detail for config writes (Setup/Policy) and Status tamper review.
enum AgentConfigChangeLog {
	private static let maxDiffLines = 80

	static func logPersistedWrite(source: String, configPath: URL, newData: Data) {
		let diff = loadDiff(configPath: configPath, newData: newData)
		emitBlock(
			detailLines: persistedWriteLines(source: source, diff: diff),
			systemSummary: persistedWriteSummary(source: source, diff: diff))
	}

	static func logTamperReview(
		decisions: [Int: AgentConfigLastGood.LineDecision],
		diff: [AgentConfigLastGood.DiffLine],
		configPath: URL,
		resolvedData: Data
	) {
		let resolvedDiff = loadDiff(configPath: configPath, newData: resolvedData)
		var detail = tamperReviewLines(decisions: decisions, diff: diff)
		detail.append("")
		detail.append(contentsOf: persistedWriteLines(source: "tamper review", diff: resolvedDiff))
		emitBlock(
			detailLines: detail,
			systemSummary: tamperReviewSummary(
				decisions: decisions, reviewDiff: diff, savedDiff: resolvedDiff))
	}

	static func logOutsideEdit(configPath: URL) {
		let snap = AgentConfigLastGood.snapshotURL(near: configPath)
		guard FileManager.default.fileExists(atPath: snap.path) else {
			emitBlock(
				detailLines: ["outside edit: config differs from seal (no last-good snapshot)"],
				systemSummary: "outside edit: seal mismatch (no last-good snapshot)")
			return
		}
		let oldText = (try? String(contentsOf: snap, encoding: .utf8)) ?? ""
		let newText = (try? String(contentsOf: configPath, encoding: .utf8)) ?? ""
		let diff = AgentConfigLastGood.contentDiff(oldText: oldText, newText: newText)
		var detail = [
			"outside edit: \(changeCount(in: diff)) line\(changeCount(in: diff) == 1 ? "" : "s") vs last-good — review in Status",
		]
		detail.append(contentsOf: diffLines(diff, prefix: "  "))
		emitBlock(
			detailLines: detail,
			systemSummary: outsideEditSummary(diff: diff))
	}

	// MARK: - private

	private static func loadDiff(configPath: URL, newData: Data) -> [AgentConfigLastGood.DiffLine] {
		let prior = (try? Data(contentsOf: configPath)) ?? Data()
		let oldText = String(data: prior, encoding: .utf8) ?? ""
		let newText = String(data: newData, encoding: .utf8) ?? ""
		return AgentConfigLastGood.contentDiff(oldText: oldText, newText: newText)
	}

	private static func emitBlock(detailLines: [String], systemSummary: String) {
		let detail =
			detailLines
			.filter { !$0.isEmpty }
			.joined(separator: "\n")
		guard !detail.isEmpty else { return }
		AgentLog.shared.config(detail: detail, systemSummary: systemSummary)
	}

	private static func persistedWriteLines(
		source: String, diff: [AgentConfigLastGood.DiffLine]
	) -> [String] {
		let changes = changeCount(in: diff)
		if changes == 0 {
			return ["\(source): saved (no JSON content change)"]
		}
		var lines = [
			"\(source): saved (\(changes) line\(changes == 1 ? "" : "s") changed)",
		]
		lines.append(contentsOf: diffLines(diff, prefix: "  "))
		return lines
	}

	private static func persistedWriteSummary(
		source: String, diff: [AgentConfigLastGood.DiffLine]
	) -> String {
		let changes = changeCount(in: diff)
		if changes == 0 {
			return "\(source): saved (no JSON content change)"
		}
		let stats = diffStats(diff)
		return "\(source): saved (\(changes) lines changed; +\(stats.added)/-\(stats.removed))"
	}

	private static func tamperReviewSummary(
		decisions: [Int: AgentConfigLastGood.LineDecision],
		reviewDiff: [AgentConfigLastGood.DiffLine],
		savedDiff: [AgentConfigLastGood.DiffLine]
	) -> String {
		let items = AgentConfigLastGood.reviewLines(in: reviewDiff)
		var accept = 0
		var revert = 0
		for item in items {
			switch decisions[item.id] {
			case .accept: accept += 1
			case .revert: revert += 1
			case nil: break
			}
		}
		let savedStats = diffStats(savedDiff)
		let savedLines = changeCount(in: savedDiff)
		return
			"tamper review: \(items.count) decision(s) (\(accept) accept, \(revert) revert); wrote config (\(savedLines) lines; +\(savedStats.added)/-\(savedStats.removed))"
	}

	private static func outsideEditSummary(diff: [AgentConfigLastGood.DiffLine]) -> String {
		let stats = diffStats(diff)
		return
			"outside edit: \(changeCount(in: diff)) lines vs last-good (+\(stats.added)/-\(stats.removed)) — review in Status"
	}

	private static func tamperReviewLines(
		decisions: [Int: AgentConfigLastGood.LineDecision],
		diff: [AgentConfigLastGood.DiffLine]
	) -> [String] {
		let items = AgentConfigLastGood.reviewLines(in: diff)
		var lines = ["tamper review: applying \(items.count) decision(s)"]
		for item in items {
			guard let decision = decisions[item.id] else { continue }
			let verb = decision == .accept ? "accept" : "revert"
			for line in describe(item.change) {
				lines.append("  \(verb) \(line)")
			}
		}
		return lines
	}

	private static func diffStats(_ diff: [AgentConfigLastGood.DiffLine]) -> (added: Int, removed: Int) {
		var added = 0
		var removed = 0
		for line in diff {
			switch line {
			case .added: added += 1
			case .removed: removed += 1
			case .unchanged: break
			}
		}
		return (added, removed)
	}

	private static func changeCount(in diff: [AgentConfigLastGood.DiffLine]) -> Int {
		diffStats(diff).added + diffStats(diff).removed
	}

	private static func diffLines(_ diff: [AgentConfigLastGood.DiffLine], prefix: String) -> [String] {
		var lines: [String] = []
		var emitted = 0
		let total = changeCount(in: diff)
		for line in diff {
			if case .unchanged = line { continue }
			if emitted >= maxDiffLines {
				let remaining = total - emitted
				if remaining > 0 {
					lines.append("\(prefix)… \(remaining) more changed line(s)")
				}
				break
			}
			lines.append("\(prefix)\(formatDiffLine(line))")
			emitted += 1
		}
		return lines
	}

	private static func formatDiffLine(_ line: AgentConfigLastGood.DiffLine) -> String {
		switch line {
		case .added(let text):
			return "+ \(display(text))"
		case .removed(let text):
			return "− \(display(text))"
		case .unchanged(let text):
			return "  \(display(text))"
		}
	}

	private static func describe(_ change: AgentConfigLastGood.ReviewLine.Change) -> [String] {
		switch change {
		case .added(let text):
			return ["+ \(display(text))"]
		case .removed(let text):
			return ["− \(display(text))"]
		case .replaced(let old, let new):
			return ["− \(display(old))", "+ \(display(new))"]
		}
	}

	private static func display(_ text: String) -> String {
		text.isEmpty ? "(blank line)" : text
	}
}
