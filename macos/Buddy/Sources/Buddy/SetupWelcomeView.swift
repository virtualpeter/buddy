import SwiftUI

/// Shown on first launch before Buddy Agent has a workplace and auth configured.
struct SetupWelcomeView: View {
	@EnvironmentObject private var serve: ServeProcess
	@State private var agentRunning = AgentBootstrap.agentReachable()

	var body: some View {
		VStack(spacing: 24) {
			BuddyLogoView(size: 72)

			VStack(spacing: 8) {
				Text("Welcome to Buddy")
					.font(.title)
					.fontWeight(.semibold)
				Text("Before you can browse docs or run scripts, Buddy needs a Jamf workplace and sign-in stored in Buddy Agent.")
					.multilineTextAlignment(.center)
					.foregroundStyle(.secondary)
					.frame(maxWidth: 420)
			}

			VStack(alignment: .leading, spacing: 10) {
				setupRow(
					title: "Workplace",
					detail: "A Jamf Pro site URL, or a Jamf Account environment (region + environment ID)."
				)
				setupRow(
					title: "Auth",
					detail: "Basic, OAuth, or platform credentials with the secret stored in the Agent Keychain."
				)
				setupRow(
					title: "Unlock",
					detail: "Unlock Buddy Agent from the menu bar when saving secrets."
				)
			}
			.padding(16)
			.frame(maxWidth: 440)
			.background(.quaternary.opacity(0.5), in: RoundedRectangle(cornerRadius: 10))

			HStack(spacing: 6) {
				Image(systemName: agentRunning ? "checkmark.circle.fill" : "exclamationmark.circle.fill")
					.foregroundStyle(agentRunning ? .green : .orange)
				Text(agentRunning ? "Buddy Agent is running" : "Starting Buddy Agent…")
					.font(.callout)
					.foregroundStyle(.secondary)
			}

			Button {
				AgentSettingsOpener.show()
			} label: {
				Label("Open Agent Settings", systemImage: "gearshape")
			}
			.buttonStyle(.borderedProminent)
			.controlSize(.large)
			.keyboardShortcut(",", modifiers: [.command, .shift])
		}
		.padding(32)
		.frame(maxWidth: .infinity, maxHeight: .infinity)
		.onAppear {
			AgentBootstrap.ensureRunning()
			refreshAgentStatus()
			serve.startConfigPollingIfNeeded()
		}
		.onDisappear {
			serve.stopConfigPolling()
		}
	}

	private func setupRow(title: String, detail: String) -> some View {
		HStack(alignment: .top, spacing: 10) {
			Image(systemName: "circle.fill")
				.font(.system(size: 6))
				.foregroundStyle(.secondary)
				.padding(.top, 6)
			VStack(alignment: .leading, spacing: 2) {
				Text(title)
					.fontWeight(.medium)
				Text(detail)
					.font(.callout)
					.foregroundStyle(.secondary)
			}
		}
	}

	private func refreshAgentStatus() {
		Task {
			for _ in 0..<40 {
				let reachable = AgentBootstrap.agentReachable()
				await MainActor.run { agentRunning = reachable }
				if reachable { return }
				try? await Task.sleep(nanoseconds: 250_000_000)
			}
		}
	}
}
