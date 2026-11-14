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
		case .setup: return "building.2.fill"
		case .policy: return "slider.horizontal.3"
		case .log: return "list.bullet.rectangle.fill"
		case .status: return "info.circle.fill"
		}
	}

	/// Tile color, in the same role as a Passwords sidebar card.
	var tint: Color {
		switch self {
		case .setup: return Color(red: 0.16, green: 0.45, blue: 0.96)
		case .policy: return Color(red: 0.95, green: 0.58, blue: 0.12)
		case .log: return Color(red: 0.56, green: 0.36, blue: 0.95)
		case .status: return Color(red: 0.18, green: 0.70, blue: 0.42)
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
