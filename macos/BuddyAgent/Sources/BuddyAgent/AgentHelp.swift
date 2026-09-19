import AppKit
import SwiftUI

extension Notification.Name {
	static let agentShowHelp = Notification.Name("BuddyAgentShowHelp")
}

/// Bundled `AgentHelp.md` and in-app help sheet.
enum AgentHelp {
	static let resourceName = "AgentHelp"

	private static var bundle: Bundle {
		#if SWIFT_PACKAGE
		.module
		#else
		.main
		#endif
	}

	static var url: URL? {
		bundle.url(forResource: resourceName, withExtension: "md")
	}

	static var markdown: String {
		guard let url, let text = try? String(contentsOf: url, encoding: .utf8) else {
			return """
				# Buddy Agent Help

				Help file not found in the app bundle. Rebuild Buddy Agent or open \
				Resources/AgentHelp.md in the source tree.
				"""
		}
		return text
	}

	static func postShowHelpNotification() {
		NotificationCenter.default.post(name: .agentShowHelp, object: nil)
	}
}

// MARK: - Markdown → SwiftUI (block layout; AttributedString alone collapses paragraphs)

private enum HelpBlock: Identifiable {
	case title(String)
	case heading(String)
	case subheading(String)
	case paragraph(String)
	case numbered(Int, String)
	case bullet(String)
	case tableHeader([String])
	case tableRow([String])
	case code(String)

	var id: String {
		switch self {
		case .title(let t): return "title-\(t)"
		case .heading(let t): return "h2-\(t)"
		case .subheading(let t): return "h3-\(t)"
		case .paragraph(let t): return "p-\(t.prefix(40))"
		case .numbered(let n, let t): return "n-\(n)-\(t.prefix(24))"
		case .bullet(let t): return "b-\(t.prefix(24))"
		case .tableHeader(let cells): return "th-\(cells.joined())"
		case .tableRow(let cells): return "tr-\(cells.joined())"
		case .code(let t): return "code-\(t.prefix(24))"
		}
	}
}

private enum HelpMarkdownParser {
	static func blocks(from markdown: String) -> [HelpBlock] {
		var result: [HelpBlock] = []
		var paragraphLines: [String] = []
		var codeLines: [String] = []
		var inCode = false
		var tableHeader: [String]?
		var tableHeaderEmitted = false

		func flushParagraph() {
			guard !paragraphLines.isEmpty else { return }
			let text = paragraphLines.joined(separator: " ").trimmingCharacters(in: .whitespaces)
			paragraphLines.removeAll()
			guard !text.isEmpty else { return }
			result.append(.paragraph(text))
		}

		func endTable() {
			if let tableHeader, !tableHeaderEmitted {
				result.append(.tableHeader(tableHeader))
			}
			tableHeader = nil
			tableHeaderEmitted = false
		}

		for rawLine in markdown.components(separatedBy: .newlines) {
			let line = rawLine.trimmingCharacters(in: .whitespaces)

			if inCode {
				if line.hasPrefix("```") {
					result.append(.code(codeLines.joined(separator: "\n")))
					codeLines.removeAll()
					inCode = false
				} else {
					codeLines.append(rawLine)
				}
				continue
			}

			if line.hasPrefix("```") {
				flushParagraph()
				endTable()
				inCode = true
				continue
			}

			if line.isEmpty {
				flushParagraph()
				endTable()
				continue
			}

			if line.hasPrefix("# ") {
				flushParagraph()
				endTable()
				result.append(.title(String(line.dropFirst(2))))
				continue
			}
			if line.hasPrefix("## ") {
				flushParagraph()
				endTable()
				result.append(.heading(String(line.dropFirst(3))))
				continue
			}
			if line.hasPrefix("### ") {
				flushParagraph()
				endTable()
				result.append(.subheading(String(line.dropFirst(4))))
				continue
			}

			if line.hasPrefix("|"), line.hasSuffix("|") {
				flushParagraph()
				let cells = parseTableCells(line)
				if cells.allSatisfy({ $0.replacingOccurrences(of: "-", with: "").isEmpty }) {
					continue
				}
				if tableHeader == nil {
					tableHeader = cells
					tableHeaderEmitted = false
				} else {
					if !tableHeaderEmitted, let tableHeader {
						result.append(.tableHeader(tableHeader))
						tableHeaderEmitted = true
					}
					result.append(.tableRow(cells))
				}
				continue
			}
			endTable()

			if let numbered = parseNumberedList(line) {
				flushParagraph()
				result.append(.numbered(numbered.0, numbered.1))
				continue
			}

			if line.hasPrefix("- ") {
				flushParagraph()
				result.append(.bullet(String(line.dropFirst(2))))
				continue
			}

			paragraphLines.append(line)
		}

		if inCode, !codeLines.isEmpty {
			result.append(.code(codeLines.joined(separator: "\n")))
		}
		flushParagraph()
		endTable()
		return result
	}

	private static func parseTableCells(_ line: String) -> [String] {
		line.split(separator: "|", omittingEmptySubsequences: false)
			.map { $0.trimmingCharacters(in: .whitespaces) }
			.filter { !$0.isEmpty }
	}

	private static func parseNumberedList(_ line: String) -> (Int, String)? {
		guard let dot = line.firstIndex(of: ".") else { return nil }
		let prefix = line[..<dot]
		guard let number = Int(prefix), dot < line.endIndex else { return nil }
		var rest = line[line.index(after: dot)...]
		if rest.first == " " { rest = rest.dropFirst() }
		let text = String(rest).trimmingCharacters(in: .whitespaces)
		guard !text.isEmpty else { return nil }
		return (number, text)
	}
}

private struct HelpInlineText: View {
	let markdown: String

	var body: some View {
		if let attr = try? AttributedString(
			markdown: markdown,
			options: AttributedString.MarkdownParsingOptions(interpretedSyntax: .inlineOnlyPreservingWhitespace))
		{
			Text(attr)
		} else {
			Text(markdown)
		}
	}
}

private struct HelpBlockView: View {
	let block: HelpBlock

	var body: some View {
		switch block {
		case .title(let text):
			HelpInlineText(markdown: text)
				.font(.title.bold())
				.padding(.bottom, 4)
		case .heading(let text):
			HelpInlineText(markdown: text)
				.font(.title3.weight(.semibold))
				.padding(.top, 8)
		case .subheading(let text):
			HelpInlineText(markdown: text)
				.font(.headline)
				.padding(.top, 4)
		case .paragraph(let text):
			HelpInlineText(markdown: text)
				.font(.body)
				.fixedSize(horizontal: false, vertical: true)
		case .numbered(let index, let text):
			HStack(alignment: .firstTextBaseline, spacing: 8) {
				Text("\(index).")
					.font(.body.monospacedDigit())
					.foregroundStyle(.secondary)
					.frame(width: 22, alignment: .trailing)
				HelpInlineText(markdown: text)
					.font(.body)
					.frame(maxWidth: .infinity, alignment: .leading)
			}
		case .bullet(let text):
			HStack(alignment: .firstTextBaseline, spacing: 8) {
				Text("•")
					.foregroundStyle(.secondary)
					.frame(width: 12, alignment: .center)
				HelpInlineText(markdown: text)
					.font(.body)
					.frame(maxWidth: .infinity, alignment: .leading)
			}
		case .tableHeader(let cells):
			HelpTableRow(cells: cells, isHeader: true)
				.padding(.top, 4)
		case .tableRow(let cells):
			HelpTableRow(cells: cells, isHeader: false)
		case .code(let text):
			ScrollView(.horizontal, showsIndicators: false) {
				Text(text)
					.font(.system(.caption, design: .monospaced))
					.textSelection(.enabled)
					.padding(10)
					.frame(maxWidth: .infinity, alignment: .leading)
			}
			.background(Color.primary.opacity(0.06), in: RoundedRectangle(cornerRadius: 8))
		}
	}
}

private struct HelpTableRow: View {
	let cells: [String]
	let isHeader: Bool

	var body: some View {
		HStack(alignment: .top, spacing: 12) {
			ForEach(Array(cells.enumerated()), id: \.offset) { index, cell in
				HelpInlineText(markdown: cell)
					.font(isHeader ? .caption.weight(.semibold) : .callout)
					.foregroundStyle(isHeader ? .secondary : .primary)
					.frame(maxWidth: index == 0 ? 140 : .infinity, alignment: .leading)
			}
		}
		.padding(.vertical, 4)
		if isHeader {
			Divider()
		}
	}
}

struct AgentHelpView: View {
	@Environment(\.dismiss) private var dismiss

	private var bodyBlocks: [HelpBlock] {
		HelpMarkdownParser.blocks(from: AgentHelp.markdown).filter { block in
			if case .title = block { return false }
			return true
		}
	}

	var body: some View {
		VStack(spacing: 0) {
			HStack {
				Text("Buddy Agent Help")
					.font(.headline)
				Spacer()
				Button("Done") { dismiss() }
					.keyboardShortcut(.cancelAction)
			}
			.padding(.horizontal, 20)
			.padding(.vertical, 14)

			Divider()

			ScrollView {
				VStack(alignment: .leading, spacing: 10) {
					ForEach(bodyBlocks) { block in
						HelpBlockView(block: block)
					}
				}
				.textSelection(.enabled)
				.padding(.horizontal, 20)
				.padding(.vertical, 16)
				.frame(maxWidth: .infinity, alignment: .leading)
			}
		}
		.frame(minWidth: 560, idealWidth: 600, minHeight: 500, idealHeight: 620)
	}
}
