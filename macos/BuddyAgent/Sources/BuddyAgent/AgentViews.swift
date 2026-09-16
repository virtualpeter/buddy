import SwiftUI
import UniformTypeIdentifiers

/// Main window: what the Agent owns (integrations), what it has minted
/// (tokens), and how the CLI reaches it (status). No secret value is ever
/// displayed - only whether one is stored.
struct AgentRootView: View {
	@ObservedObject var store: AgentStore
	@ObservedObject var state: AgentState
	let socketPath: String

	var body: some View {
		TabView {
			IntegrationsView(store: store)
				.tabItem { Label("Integrations", systemImage: "building.2") }
			TokensView(state: state)
				.tabItem { Label("Tokens", systemImage: "key") }
			StatusView(store: store, state: state, socketPath: socketPath)
				.tabItem { Label("Status", systemImage: "info.circle") }
		}
		.padding()
		.frame(minWidth: 640, minHeight: 440)
	}
}

struct IntegrationsView: View {
	@ObservedObject var store: AgentStore
	@State private var editing: Integration?
	@State private var backup: BackupPrompt?
	@State private var message: String?

	var body: some View {
		VStack(alignment: .leading, spacing: 12) {
			if store.integrations.isEmpty {
				VStack(alignment: .leading, spacing: 6) {
					Text("No integrations yet").font(.headline)
					Text(
						"Add a Jamf Pro workplace and its API client here. buddy reads this list over the socket and never sees the secret."
					)
					.foregroundStyle(.secondary)
				}
				.frame(maxWidth: .infinity, alignment: .leading)
			} else {
				List {
					ForEach(store.integrations) { row in
						IntegrationRow(row: row)
							.contentShape(Rectangle())
							.onTapGesture { editing = row }
					}
				}
			}

			if let message {
				Text(message).font(.callout).foregroundStyle(.secondary)
			}

			HStack {
				Button("Add Integration") { editing = Integration() }
				Menu("Import from buddy") {
					Button("Config only (setup export --json)...") { importConfig() }
					Button("Config and secrets (cred backup)...") { pickBackup() }
				}
				.menuStyle(.borderlessButton)
				.fixedSize()
				Spacer()
				if let def = defaultContextLabel {
					Text("default context: \(def)").font(.callout).foregroundStyle(.secondary)
				}
			}
			Text(
				"A 'setup export --json' file is identity only - workplaces and auth names, no secrets. A 'cred backup' file also carries the secrets and needs its passphrase."
			)
			.font(.caption)
			.foregroundStyle(.secondary)
		}
		.sheet(item: $editing) { row in
			IntegrationEditor(store: store, draft: row)
		}
		.sheet(item: $backup) { prompt in
			BackupPassphraseSheet(url: prompt.url) { passphrase in
				importBackup(prompt.url, passphrase: passphrase)
			}
		}
	}

	private var defaultContextLabel: String? {
		let value = store.defaultContext.trimmed
		return value.isEmpty ? nil : value
	}

	private func importConfig() {
		guard
			let url = chooseFile(
				types: [.json], prompt: "Choose the JSON written by 'buddy setup export --json'")
		else { return }
		do {
			let added = try store.importExport(Data(contentsOf: url))
			message =
				"Imported \(added) integration(s) from \(url.lastPathComponent). Add each secret before minting."
		} catch {
			message = "Import failed: \(error)"
		}
	}

	private func pickBackup() {
		guard
			let url = chooseFile(
				types: [.json, .data], prompt: "Choose the file written by 'buddy cred backup'")
		else { return }
		backup = BackupPrompt(url: url)
	}

	private func importBackup(_ url: URL, passphrase: String) {
		do {
			let result = try store.importBackup(Data(contentsOf: url), passphrase: passphrase)
			message =
				"Imported \(result.integrations) integration(s) and \(result.secrets) secret(s) from \(url.lastPathComponent)."
		} catch {
			message = "Import failed: \(error)"
		}
	}

	private func chooseFile(types: [UTType], prompt: String) -> URL? {
		let panel = NSOpenPanel()
		panel.allowedContentTypes = types
		panel.allowsMultipleSelection = false
		panel.message = prompt
		guard panel.runModal() == .OK else { return nil }
		return panel.url
	}
}

private struct BackupPrompt: Identifiable {
	let id = UUID()
	let url: URL
}

/// Asks for the passphrase typed at `buddy cred backup` time. It is used once,
/// to decrypt the file in memory, and never stored.
struct BackupPassphraseSheet: View {
	let url: URL
	let onImport: (String) -> Void

	@State private var passphrase: String = ""
	@Environment(\.dismiss) private var dismiss

	var body: some View {
		VStack(alignment: .leading, spacing: 12) {
			Text("Import \(url.lastPathComponent)").font(.title3)
			Text(
				"Type the passphrase used for this backup. Its secrets go straight into the Agent's Keychain."
			)
			.font(.callout)
			.foregroundStyle(.secondary)
			SecureField("Backup passphrase", text: $passphrase)
				.onSubmit(submit)
			HStack {
				Spacer()
				Button("Cancel") { dismiss() }
				Button("Import", action: submit)
					.keyboardShortcut(.defaultAction)
					.disabled(passphrase.isEmpty)
			}
		}
		.padding()
		.frame(minWidth: 420)
	}

	private func submit() {
		guard !passphrase.isEmpty else { return }
		onImport(passphrase)
		passphrase = ""
		dismiss()
	}
}

struct IntegrationRow: View {
	let row: Integration

	var body: some View {
		HStack(alignment: .firstTextBaseline) {
			VStack(alignment: .leading, spacing: 2) {
				Text(row.context.isEmpty ? "(no context)" : row.context).font(.headline)
				Text("\(row.authName) · \(row.authType)")
					.font(.callout)
					.foregroundStyle(.secondary)
				if !row.url.isEmpty {
					Text(row.url).font(.caption).foregroundStyle(.secondary)
				}
			}
			Spacer()
			if let problem = row.problem {
				Label(problem, systemImage: "exclamationmark.triangle")
					.font(.caption)
					.foregroundStyle(.orange)
			} else {
				Label("ready", systemImage: "checkmark.seal")
					.font(.caption)
					.foregroundStyle(.secondary)
			}
		}
		.padding(.vertical, 2)
	}
}

struct IntegrationEditor: View {
	@ObservedObject var store: AgentStore
	@State var draft: Integration
	@State private var secret: String = ""
	@Environment(\.dismiss) private var dismiss

	private let types = ["oauth", "basic", "platform"]

	var body: some View {
		VStack(alignment: .leading, spacing: 12) {
			Text(draft.authName.isEmpty ? "New integration" : draft.authName).font(.title3)
			Form {
				Section("Workplace") {
					TextField("Context name", text: $draft.context)
					TextField("Jamf Pro URL", text: $draft.url)
						.help("https://example.jamfcloud.com")
				}
				Section("Auth") {
					TextField("Auth name", text: $draft.authName)
					Picker("Type", selection: $draft.authType) {
						ForEach(types, id: \.self) { Text($0) }
					}
					if draft.authType == "basic" {
						TextField("Username", text: $draft.username)
					} else {
						TextField("Client ID", text: $draft.clientID)
					}
					SecureField(
						draft.hasSecret ? "Replace stored secret" : "Secret", text: $secret)
					Picker("Access", selection: $draft.access) {
						Text("full").tag("")
						Text("readonly").tag("readonly")
					}
				}
				if draft.authType == "platform" {
					Section("Jamf Account (optional)") {
						TextField("Region", text: $draft.region)
						TextField("Environment ID", text: $draft.environmentID)
						TextField("Tenant ID", text: $draft.tenantID)
						Picker("Scope", selection: $draft.scope) {
							Text("(none)").tag("")
							Text("organization").tag("organization")
							Text("environment").tag("environment")
							Text("tenant").tag("tenant")
						}
					}
				}
			}
			.formStyle(.grouped)

			HStack {
				if store.integration(forAuth: draft.authName) != nil {
					Button("Delete", role: .destructive) {
						store.remove(draft)
						dismiss()
					}
				}
				Spacer()
				Button("Cancel") { dismiss() }
				Button("Save") { save() }
					.keyboardShortcut(.defaultAction)
					.disabled(draft.context.trimmed.isEmpty || draft.authName.trimmed.isEmpty)
			}
		}
		.padding()
		.frame(minWidth: 480, minHeight: 460)
	}

	private func save() {
		draft.context = draft.context.trimmed
		draft.authName = draft.authName.trimmed
		draft.url = draft.url.trimmed
		store.upsert(draft)
		if !secret.isEmpty {
			store.setSecret(secret, for: draft)
			secret = ""
		}
		dismiss()
	}
}

struct TokensView: View {
	@ObservedObject var state: AgentState

	var body: some View {
		VStack(alignment: .leading, spacing: 8) {
			if state.tokenSummaries.isEmpty {
				Text("No bearers minted in this session.").foregroundStyle(.secondary)
			} else {
				List {
					ForEach(state.tokenSummaries, id: \.baseURL) { row in
						VStack(alignment: .leading, spacing: 2) {
							Text(row.auth).font(.headline)
							Text(row.baseURL).font(.caption).foregroundStyle(.secondary)
							Text(expiryText(row.expiresAt)).font(.callout)
						}
						.padding(.vertical, 2)
					}
				}
			}
			Text("Lock clears every cached bearer.").font(.caption).foregroundStyle(.secondary)
		}
	}

	private func expiryText(_ date: Date) -> String {
		let remaining = Int(date.timeIntervalSinceNow)
		if remaining <= 0 { return "expired" }
		let minutes = remaining / 60
		return minutes > 0 ? "expires in \(minutes) min" : "expires in \(remaining)s"
	}
}

struct StatusView: View {
	@ObservedObject var store: AgentStore
	@ObservedObject var state: AgentState
	let socketPath: String

	var body: some View {
		VStack(alignment: .leading, spacing: 10) {
			Label(
				state.unlocked ? "Unlocked - minting allowed" : "Locked - config only",
				systemImage: state.unlocked ? "lock.open.fill" : "lock.fill"
			)
			.font(.headline)

			Grid(alignment: .leading, horizontalSpacing: 12, verticalSpacing: 6) {
				row("Version", agentVersion)
				row("Socket", socketPath)
				row("Config", store.path.path)
				row("Integrations", "\(store.integrations.count)")
				row("Peer check", PeerCheck.allowUnsigned ? "unsigned allowed (dev)" : "codesign")
			}
			.font(.callout)

			if PeerCheck.allowUnsigned {
				Label(
					"BUDDY_AGENT_ALLOW_UNSIGNED=1: any local process can ask this Agent to mint.",
					systemImage: "exclamationmark.triangle"
				)
				.foregroundStyle(.orange)
			}
			if AgentKeychain.usingFallback {
				Label(
					"Keychain writes failed; secrets are held in memory for this session only.",
					systemImage: "exclamationmark.triangle"
				)
				.foregroundStyle(.orange)
			}
			if let error = store.lastError {
				Label(error, systemImage: "xmark.octagon").foregroundStyle(.red)
			}
			Spacer()
			Text("Config is served to buddy while locked. Unlock gates minting only.")
				.font(.caption)
				.foregroundStyle(.secondary)
		}
		.frame(maxWidth: .infinity, alignment: .leading)
	}

	private func row(_ label: String, _ value: String) -> some View {
		GridRow {
			Text(label).foregroundStyle(.secondary)
			Text(value).textSelection(.enabled)
		}
	}
}
