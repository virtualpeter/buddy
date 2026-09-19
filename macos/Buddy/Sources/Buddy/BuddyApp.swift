import SwiftUI

@main
struct BuddyApp: App {
	@NSApplicationDelegateAdaptor(AppDelegate.self) private var appDelegate
	@StateObject private var serve = ServeProcess()
	@StateObject private var appearance = BuddyAppearanceStore()

	var body: some Scene {
		WindowGroup {
			ContentView()
				.environmentObject(serve)
				.environmentObject(appearance)
				.onAppear {
					appDelegate.serve = serve
					Task { await serve.startIfNeeded() }
				}
		}
		.commands {
			CommandGroup(replacing: .newItem) {}
			CommandGroup(after: .toolbar) {
				Divider()
				Picker("Appearance", selection: $appearance.choice) {
					ForEach(BuddyAppearance.allCases) { mode in
						Text(mode.label).tag(mode)
					}
				}
			}
			CommandGroup(after: .appSettings) {
				Button("Agent Settings…") {
					AgentSettingsOpener.show()
				}
				.keyboardShortcut(",", modifiers: [.command, .shift])
			}
		}
	}
}
