import SwiftUI

enum AgentSettingsTab: String, CaseIterable, Identifiable {
	case setup
	case policy
	case log
	case status

	var id: String { rawValue }

	var title: String {
		switch self {
		case .setup: return "Setup"
		case .policy: return "Policy"
		case .log: return "Log"
		case .status: return "Status"
		}
	}

	var icon: String {
		switch self {
		case .setup: return "list.bullet.indent"
		case .policy: return "slider.horizontal.3"
		case .log: return "list.bullet.rectangle"
		case .status: return "info.circle"
		}
	}
}

/// Label on the left, menu or control aligned trailing — avoids negative widths when
/// the proposed width is smaller than the control's intrinsic size.
struct LabeledTrailingRow<Trailing: View>: View {
	let label: String
	@ViewBuilder var trailing: () -> Trailing

	var body: some View {
		HStack(spacing: 8) {
			Text(label)
				.lineLimit(1)
				.layoutPriority(1)
			trailing()
				.fixedSize(horizontal: true, vertical: false)
				.frame(minWidth: 0, maxWidth: .infinity, alignment: .trailing)
		}
		.frame(minWidth: 0, maxWidth: .infinity, alignment: .leading)
	}
}

extension View {
	func agentFlexibleWidth(alignment: Alignment = .leading) -> some View {
		frame(minWidth: 0, maxWidth: .infinity, alignment: alignment)
	}

	func agentLabeledField() -> some View {
		frame(minWidth: 0, maxWidth: .infinity, alignment: .leading)
	}
}
