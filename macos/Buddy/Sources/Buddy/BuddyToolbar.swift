import SwiftUI

private struct BuddyUpdatePayload: Decodable {
	var current: String = ""
	var update: Bool = false
	var checked: Bool = false
	var version: String?
	var url: String?
}

struct BuddyToolbar: View {
	@EnvironmentObject private var serve: ServeProcess
	@Binding var surface: ServeSurface
	@State private var currentVersion = ""
	@State private var updateLabel: String?
	@State private var updateURL: URL?
	@State private var pollTask: Task<Void, Never>?

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

			if !currentVersion.isEmpty {
				Text(currentVersion)
					.font(.caption)
					.foregroundStyle(.secondary)
					.help("buddy version")
			}
			if let updateURL, let updateLabel {
				Link(updateLabel, destination: updateURL)
					.font(.caption)
			}

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
			startUpdatePoll()
		}
		.onChange(of: serve.phase) { _ in
			startUpdatePoll()
		}
		.onDisappear {
			pollTask?.cancel()
		}
	}

	/// `--embed` hides the web header version, so the toolbar shows the helper's
	/// version from `/api/update` and a release link when one exists.
	private func startUpdatePoll() {
		pollTask?.cancel()
		guard case .ready(let docsURL) = serve.phase,
			var components = URLComponents(url: docsURL, resolvingAgainstBaseURL: false)
		else {
			return
		}
		components.path = "/api/update"
		components.query = nil
		guard let endpoint = components.url else { return }
		pollTask = Task {
			for _ in 0..<8 {
				if Task.isCancelled { return }
				if await loadUpdate(from: endpoint) { return }
				try? await Task.sleep(nanoseconds: 2_000_000_000)
			}
		}
	}

	private func loadUpdate(from endpoint: URL) async -> Bool {
		var request = URLRequest(url: endpoint, timeoutInterval: 5)
		request.httpMethod = "GET"
		guard let (data, response) = try? await URLSession.shared.data(for: request),
			let http = response as? HTTPURLResponse, http.statusCode == 200,
			let payload = try? JSONDecoder().decode(BuddyUpdatePayload.self, from: data)
		else {
			return false
		}
		currentVersion = payload.current
		let prefix = "https://github.com/virtualpeter/buddy/releases/"
		if payload.update, let raw = payload.url, raw.hasPrefix(prefix), let url = URL(string: raw) {
			updateLabel = "Update \(payload.version ?? "")"
			updateURL = url
		} else {
			updateLabel = nil
			updateURL = nil
		}
		return payload.checked
	}
}

