import SwiftUI

struct ContentView: View {
	@EnvironmentObject private var serve: ServeProcess
	@EnvironmentObject private var appearance: BuddyAppearanceStore
	@State private var surface: ServeSurface = .docs

	var body: some View {
		Group {
			if serve.needsSetup {
				SetupWelcomeView()
			} else {
				switch serve.phase {
				case .idle, .starting:
					ProgressView("Starting buddy serve…")
						.frame(maxWidth: .infinity, maxHeight: .infinity)
				case .failed(let message):
					VStack(spacing: 12) {
						Text("Buddy could not start")
							.font(.headline)
						Text(message)
							.multilineTextAlignment(.center)
							.foregroundStyle(.secondary)
					}
					.padding()
					.frame(maxWidth: .infinity, maxHeight: .infinity)
				case .ready:
					VStack(spacing: 0) {
						BuddyToolbar(surface: $surface)
						Divider()
						if let pageURL = serve.url(for: surface) {
							ServeWebView(url: pageURL, appearance: appearance)
						} else {
							Text("Missing serve URL")
								.frame(maxWidth: .infinity, maxHeight: .infinity)
						}
					}
				}
			}
		}
		.frame(minWidth: 720, minHeight: 480)
	}
}
