import SwiftUI

struct BuddyToolbar: View {
	@EnvironmentObject private var serve: ServeProcess
	@Binding var surface: ServeSurface

	private var tabs: [ServeSurface] {
		ServeSurface.available(configReadonly: serve.configReadonly)
	}

	var body: some View {
		HStack(spacing: 12) {
			Button {
				surface = .docs
			} label: {
				BuddyLogoView()
					.frame(width: 26, height: 26)
			}
			.buttonStyle(.plain)
			.help("Show docs")

			Picker("Surface", selection: $surface) {
				ForEach(tabs) { tab in
					Text(tab.rawValue).tag(tab)
				}
			}
			.pickerStyle(.segmented)
			.labelsHidden()

			Spacer(minLength: 8)

			Button {
				AgentSettingsOpener.show()
			} label: {
				Label("Agent Settings", systemImage: "gearshape")
			}
			.help("Open Buddy Agent settings")
		}
		.padding(.horizontal, 12)
		.padding(.vertical, 8)
		.onChange(of: serve.configReadonly) { readonly in
			if readonly, surface == .config {
				surface = .docs
			}
		}
		.onAppear {
			if !tabs.contains(surface) {
				surface = .docs
			}
		}
	}
}

