import SwiftUI
import AppKit
import LocalAuthentication
import UniformTypeIdentifiers

/// Main window: config the Agent owns, minted tokens, and CLI status.
struct AgentRootView: View {
	@ObservedObject var store: AgentStore
	@ObservedObject var state: AgentState
	@ObservedObject var log: AgentLog
	let gate: UnlockGate
	let socketPath: String
	var onImportFinished: () -> Void = {}

	@State private var showHelp = false
	@State private var selectedTab: AgentSettingsTab = .setup
	@State private var revealAuth: String?
	@State private var revealBearer: String?

	var body: some View {
		HStack(spacing: 0) {
			settingsSidebar
			Divider()
			Group {
				switch selectedTab {
				case .setup:
					ConfigView(
						store: store, state: state, gate: gate, showHelp: $showHelp,
						revealAuth: $revealAuth,
						revealBearer: $revealBearer,
						onImportFinished: onImportFinished,
						onShowStatus: { selectedTab = .status })
				case .policy:
					PolicyView(
						store: store, state: state, gate: gate,
						onShowStatus: { selectedTab = .status })
				case .log:
					LogView(log: log)
				case .status:
					StatusView(store: store, state: state, gate: gate, socketPath: socketPath)
				}
			}
			.frame(minWidth: 0, maxWidth: .infinity, minHeight: 0, maxHeight: .infinity, alignment: .topLeading)
			.padding(12)
		}
		.frame(minWidth: 760, minHeight: 520)
		.sheet(isPresented: $showHelp) {
			AgentHelpView()
		}
		.onReceive(NotificationCenter.default.publisher(for: .agentShowHelp)) { _ in
			showHelp = true
		}
		.onReceive(NotificationCenter.default.publisher(for: .agentRevealAuth)) { note in
			selectedTab = .setup
			revealAuth = note.object as? String
		}
		.onReceive(NotificationCenter.default.publisher(for: .agentShowBearer)) { note in
			selectedTab = .setup
			revealBearer = note.object as? String
		}
	}

	private var settingsSidebar: some View {
		VStack(alignment: .leading, spacing: 16) {
			LazyVGrid(
				columns: [GridItem(.flexible(), spacing: 8), GridItem(.flexible(), spacing: 8)],
				spacing: 8
			) {
				ForEach(AgentSettingsTab.allCases) { tab in
					SettingsSectionCard(
						tab: tab,
						count: sectionCount(tab),
						selected: selectedTab == tab,
						action: { selectedTab = tab })
				}
			}
			.accessibilityLabel("Settings section")
			Spacer(minLength: 0)
			AgentLockButton(state: state, gate: gate)
				.frame(maxWidth: .infinity, alignment: .leading)
		}
		.padding(12)
		.frame(width: 196)
		.frame(maxHeight: .infinity, alignment: .top)
	}

	private func sectionCount(_ tab: AgentSettingsTab) -> Int? {
		switch tab {
		case .setup: return store.workplaces.count
		case .policy: return nil
		case .log: return log.entries.count
		case .status: return nil
		}
	}
}

/// Passwords-style section tile: colored glyph, count, and label.
private struct SettingsSectionCard: View {
	let tab: AgentSettingsTab
	let count: Int?
	let selected: Bool
	let action: () -> Void

	var body: some View {
		Button(action: action) {
			VStack(alignment: .leading, spacing: 0) {
				HStack(alignment: .top, spacing: 4) {
					Image(systemName: tab.icon)
						.font(.system(size: 18, weight: .semibold))
						.symbolRenderingMode(.hierarchical)
						.foregroundStyle(selected ? Color.white : tab.tint)
					Spacer(minLength: 0)
					if let count {
						Text("\(count)")
							.font(.system(size: 13, weight: .semibold))
							.foregroundStyle(selected ? Color.white.opacity(0.92) : Color.secondary)
					}
				}
				Spacer(minLength: 10)
				Text(tab.title)
					.font(.system(size: 13, weight: .semibold))
					.foregroundStyle(selected ? Color.white : Color.primary)
			}
			.padding(10)
			.frame(maxWidth: .infinity, minHeight: 74, alignment: .leading)
			.background(
				selected ? tab.tint : Color.primary.opacity(0.07),
				in: RoundedRectangle(cornerRadius: 16, style: .continuous))
		}
		.buttonStyle(.plain)
		.accessibilityLabel(tab.title)
		.accessibilityAddTraits(selected ? .isSelected : [])
	}
}

private enum ConfigActiveSheet: Identifiable {
	case workplace(WorkplaceEditDraft)
	case auth(AuthEditDraft)
	case backup(BackupPrompt)
	case exportBackup
	case bearer(BearerDetailsDraft)
	case quickSetup

	var id: String {
		switch self {
		case .workplace(let draft): return "workplace-\(draft.id.uuidString)"
		case .auth(let draft): return "auth-\(draft.id.uuidString)"
		case .backup(let prompt): return "backup-\(prompt.id.uuidString)"
		case .exportBackup: return "export-backup"
		case .bearer(let draft): return "bearer-\(draft.authName)"
		case .quickSetup: return "quick-setup"
		}
	}
}

struct ConfigReviewLockBanner: View {
	@ObservedObject var store: AgentStore
	var onShowStatus: () -> Void = {}

	var body: some View {
		if store.configEditsBlocked {
			VStack(alignment: .leading, spacing: 6) {
				Label(message, systemImage: "exclamationmark.triangle")
					.font(.callout)
					.foregroundStyle(.orange)
					.fixedSize(horizontal: false, vertical: true)
				Button("Open Status", action: onShowStatus)
			}
		}
	}

	private var message: String {
		store.configReviewRequired
			? AgentStore.configReviewBlockMessage
			: AgentStore.configTamperBackupMessage
	}
}

private func showAgentAlert(
	title: String, message: String, style: NSAlert.Style = .warning
) {
	let alert = NSAlert()
	alert.alertStyle = style
	alert.messageText = title
	alert.informativeText = message
	alert.runModal()
}

private struct DestructiveTrashButton: View {
	let dialogTitle: String
	let dialogMessage: String
	let action: () -> Void
	var disabled: Bool = false
	var accessibilityLabel: String = "Delete"
	@State private var showConfirm = false

	var body: some View {
		Button(role: .destructive) { showConfirm = true } label: {
			Image(systemName: "trash")
		}
		.buttonStyle(.borderless)
		.disabled(disabled)
		.accessibilityLabel(accessibilityLabel)
		.confirmationDialog(dialogTitle, isPresented: $showConfirm, titleVisibility: .visible) {
			Button("Delete", role: .destructive, action: action)
			Button("Cancel", role: .cancel) {}
		} message: {
			Text(dialogMessage)
		}
	}
}

struct ConfigView: View {
	@ObservedObject var store: AgentStore
	@ObservedObject var state: AgentState
	let gate: UnlockGate
	@Binding var showHelp: Bool
	@Binding var revealAuth: String?
	@Binding var revealBearer: String?
	var onImportFinished: () -> Void = {}
	var onShowStatus: () -> Void = {}
	@State private var selectedWorkplace: String?
	@State private var activeSheet: ConfigActiveSheet?
	@State private var message: String?

	var body: some View {
		ScrollView(.vertical, showsIndicators: true) {
			VStack(alignment: .leading, spacing: 20) {
				HStack(alignment: .firstTextBaseline) {
					Text("Setup").font(.title3)
					Spacer()
					Button { showHelp = true } label: {
						Image(systemName: "questionmark.circle")
							.font(.title3)
							.foregroundStyle(.secondary)
					}
					.buttonStyle(.plain)
					.accessibilityLabel("Buddy Agent Help")
				}
				ConfigReviewLockBanner(store: store, onShowStatus: onShowStatus)
				if store.isEmpty {
					EmptySetupBanner(
						onQuickSetup: { presentSheet(.quickSetup) },
						onImportBackup: { pickBackup() },
						onImportConfig: { importConfig() })
				}
				workplacesSection
				transferSection
				if let message {
					Text(message).font(.callout).foregroundStyle(.secondary)
				}
			}
			.frame(minWidth: 0, maxWidth: .infinity, alignment: .leading)
		}
		.sheet(item: $activeSheet) { sheet in
			sheetContent(sheet)
				.frame(width: configSheetSize(for: sheet).width, height: configSheetSize(for: sheet).height)
		}
		.onAppear(perform: consumeRevealAuth)
		.onChange(of: revealAuth) { _ in consumeRevealAuth() }
		.onAppear(perform: consumeRevealBearer)
		.onChange(of: revealBearer) { _ in consumeRevealBearer() }
	}

	/// Tray row asked for this auth's editor. Clear first so a second delivery is a no-op.
	private func consumeRevealAuth() {
		guard let name = revealAuth else { return }
		revealAuth = nil
		guard let auth = store.auths[name] else { return }
		presentSheet(.auth(AuthEditDraft(name: name, previousName: name, auth: auth)))
	}

	/// Tray key asked for this auth's cached bearer. Clear first so a second delivery is a no-op.
	private func consumeRevealBearer() {
		guard let name = revealBearer else { return }
		revealBearer = nil
		presentSheet(.bearer(BearerDetailsDraft(authName: name)))
	}

	/// Present on the next run-loop turn so AppKit is not mid-layout in the Setup scroll view.
	private func presentSheet(_ sheet: ConfigActiveSheet) {
		DispatchQueue.main.async {
			activeSheet = sheet
		}
	}

	private func configSheetSize(for sheet: ConfigActiveSheet) -> NSSize {
		switch sheet {
		case .workplace: return NSSize(width: 480, height: 420)
		case .auth: return NSSize(width: 480, height: 560)
		case .quickSetup: return NSSize(width: 520, height: 620)
		case .backup, .exportBackup: return NSSize(width: 420, height: 240)
		case .bearer: return NSSize(width: 400, height: 280)
		}
	}

	@ViewBuilder
	private func sheetContent(_ sheet: ConfigActiveSheet) -> some View {
		switch sheet {
		case .workplace(let draft):
			WorkplaceEditor(store: store, state: state, gate: gate, draft: draft)
		case .auth(let draft):
			AuthEditor(store: store, state: state, gate: gate, draft: draft)
		case .backup(let prompt):
			BackupPassphraseSheet(url: prompt.url) { passphrase in
				importBackup(prompt.url, passphrase: passphrase)
			}
		case .exportBackup:
			BackupExportPassphraseSheet { passphrase in
				exportBackupFile(passphrase: passphrase)
			}
		case .bearer(let draft):
			AuthTokenSheet(
				state: state, authName: draft.authName,
				onDismiss: { activeSheet = nil })
		case .quickSetup:
			QuickSetupSheet(store: store, state: state, gate: gate) { result in
				message = result
				onImportFinished()
			}
		}
	}

	private var workplacesSection: some View {
		VStack(alignment: .leading, spacing: 8) {
			HStack {
				Text("Workplaces").font(.headline)
				Spacer()
				if let def = defaultContextLabel {
					HStack(spacing: 4) {
						Image(systemName: "arrowtriangle.right.fill")
							.font(.system(size: 11, weight: .bold))
							.foregroundStyle(Color.accentColor)
							.accessibilityHidden(true)
						Text("default: \(def)")
							.font(.callout)
							.foregroundStyle(.secondary)
					}
					.accessibilityElement(children: .combine)
					.accessibilityLabel("Default context \(def). The blue arrow in the tree marks it.")
				}
			}
			Text(
				"Org → environment → product-tenant. Auths are listed under each workplace they link to. The server URL is in the workplace editor."
			)
			.font(.caption)
			.foregroundStyle(.secondary)

			if store.configTree.isEmpty && unlinkedAuthNames.isEmpty {
				VStack(alignment: .leading, spacing: 6) {
					Text("No workplaces yet").foregroundStyle(.secondary)
					if !store.sortedAuthNames.isEmpty {
						Text("Add a workplace and link it from each auth before minting.")
							.font(.caption)
							.foregroundStyle(.orange)
					}
				}
			} else {
				VStack(alignment: .leading, spacing: 8) {
					if !store.configTree.isEmpty {
						WorkplaceTreeList(
							store: store,
							state: state,
							gate: gate,
							nodes: store.configTree,
							selectedWorkplace: selectedWorkplace,
							onSelect: { selectedWorkplace = $0 },
							onEdit: { node in
								presentSheet(.workplace(
									WorkplaceEditDraft(
										name: node.name, previousName: node.name, workplace: node.workplace,
										kind: node.kind)))
							},
							onSetDefault: { name in
								commitConfig { store.setDefaultContext(name) }
							},
							onDelete: { name in
								commitConfig { store.removeWorkplace(name) }
							},
							onShowBearer: { presentSheet(.bearer(BearerDetailsDraft(authName: $0))) },
							onEditAuth: { name in
								presentSheet(.auth(
									AuthEditDraft(
										name: name, previousName: name,
										auth: store.auths[name] ?? AgentAuth())))
							},
							onDeleteAuth: { name in
								commitConfig { store.removeAuth(name) }
							})
					}
					if !unlinkedAuthNames.isEmpty {
						Text("Not linked to a workplace")
							.font(.caption)
							.foregroundStyle(.secondary)
							.padding(.top, store.configTree.isEmpty ? 0 : 6)
						ForEach(unlinkedAuthNames, id: \.self) { name in
							WorkplaceAuthRow(
								store: store,
								state: state,
								gate: gate,
								name: name,
								auth: store.auths[name] ?? AgentAuth(),
								onShowBearer: { presentSheet(.bearer(BearerDetailsDraft(authName: name))) },
								onEdit: {
									presentSheet(.auth(
										AuthEditDraft(
											name: name, previousName: name,
											auth: store.auths[name] ?? AgentAuth())))
								},
								onDelete: { commitConfig { store.removeAuth(name) } })
						}
					}
				}
				.padding(8)
				.frame(maxWidth: .infinity, alignment: .leading)
				.background(.quaternary.opacity(0.25), in: RoundedRectangle(cornerRadius: 8))
				.overlay(
					RoundedRectangle(cornerRadius: 8)
						.strokeBorder(Color.primary.opacity(0.08), lineWidth: 1))
			}

			HStack {
				Button("Add Organization") {
					presentSheet(.workplace(
						WorkplaceEditDraft(
							name: "", previousName: nil, workplace: Workplace(), kind: "organization")))
				}
				Button("Add Child") {
					let parent = selectedNode
					let kind: String
					if parent == nil || parent?.kind == "organization" {
						kind = "environment"
					} else if parent?.kind == "environment" || parent?.kind == "instance" {
						kind = "product-tenant"
					} else {
						kind = "instance"
					}
					presentSheet(.workplace(
						WorkplaceEditDraft(
							name: "", previousName: nil,
							workplace: Workplace(org: parent?.kind == "organization" ? parent?.name ?? "" : parent?.org ?? ""),
							kind: kind, parent: parent)))
				}
				Button("Add Auth") {
					var auth = AgentAuth()
					if let selectedWorkplace {
						auth.contexts = [selectedWorkplace]
					}
					presentSheet(.auth(AuthEditDraft(name: "", previousName: nil, auth: auth)))
				}
			}
			.disabled(store.configEditsBlocked)
		}
	}

	/// Auths whose contexts do not name a workplace that exists.
	private var unlinkedAuthNames: [String] {
		store.sortedAuthNames.filter { name in
			let contexts = store.auths[name]?.contexts ?? []
			return !contexts.contains { store.workplaces[$0] != nil }
		}
	}

	private var transferSection: some View {
		VStack(alignment: .leading, spacing: 6) {
			HStack(spacing: 12) {
				Menu("Import from buddy") {
					Button("Config only (setup export --json)...") { importConfig() }
					Button("Config and secrets (cred backup)...") { pickBackup() }
				}
				.menuStyle(.borderlessButton)
				.disabled(store.configEditsBlocked)
				Menu("Export for buddy") {
					Button("Config only (setup export --json)...") { exportConfig() }
					Button("Config and secrets (cred backup)...") { presentSheet(.exportBackup) }
				}
				.menuStyle(.borderlessButton)
			}
			.frame(minWidth: 0, maxWidth: .infinity, alignment: .leading)
			Text(
				"Setup export is identity only. Cred backup also carries secrets — exporting one asks you to unlock."
			)
			.font(.caption)
			.foregroundStyle(.secondary)
		}
	}

	private var selectedNode: WorkplaceTreeNode? {
		guard let name = selectedWorkplace else { return nil }
		return findNode(name: name, in: store.configTree)
	}

	private var defaultContextLabel: String? {
		let value = store.defaultContext.trimmed
		return value.isEmpty ? nil : value
	}

	private func findNode(name: String, in nodes: [WorkplaceTreeNode]) -> WorkplaceTreeNode? {
		for node in nodes {
			if node.name == name { return node }
			if let hit = findNode(name: name, in: node.children) { return hit }
		}
		return nil
	}

	private func commitConfig(_ action: @escaping () -> Void) {
		AgentLockUI.perform(gate: gate, state: state, action: action)
	}

	private func importConfig() {
		guard
			let url = chooseFile(
				types: [.json], prompt: "Choose the JSON written by 'buddy setup export --json'")
		else { return }
		commitConfig {
			do {
				let added = try store.importExport(Data(contentsOf: url))
				message =
					"Imported \(added) item(s) from \(url.lastPathComponent). Add secrets before minting."
				onImportFinished()
			} catch {
				message = "Import failed: \(error)"
			}
		}
	}

	private func pickBackup() {
		guard
			let url = chooseFile(
				types: [.json, .data], prompt: "Choose the file written by 'buddy cred backup'")
		else { return }
		presentSheet(.backup(BackupPrompt(url: url)))
	}

	private func importBackup(_ url: URL, passphrase: String) {
		commitConfig {
			message = "Importing \(url.lastPathComponent)…"
			Task { @MainActor in
				do {
					let data = try Data(contentsOf: url)
					let contents = try AgentBackup.read(data, passphrase: passphrase)
					let added = try store.importDocumentForBackup(contents.document)
					let secrets = contents.secrets
					let fileName = url.lastPathComponent
					if secrets.isEmpty && !store.auths.isEmpty {
						showAgentAlert(
							title: "Backup has no secrets",
							message:
								"\(fileName) decrypted successfully but contains no client secrets or passwords. Config was imported — re-type secrets in each auth, or restore from a backup that was exported while the Agent was unlocked."
						)
					}
					DispatchQueue.global(qos: .userInitiated).async { [gate, store] in
						var presence: LAContext?
						var unlockFailed = false
						if !secrets.isEmpty, AgentKeychain.backend == .dataProtection {
							switch gate.authorize(reason: "import secrets from your cred backup") {
							case .unlocked:
								presence = gate.presenceContext
							default:
								unlockFailed = true
							}
						}
						let result = store.importBackupSecrets(secrets, presence: presence)
						DispatchQueue.main.async {
							var parts: [String] = []
							parts.append("Imported \(added) item(s) from \(fileName).")
							if result.secrets > 0 {
								parts.append("\(result.secrets) secret(s) stored in the keychain.")
							}
							if result.ephemeral > 0 {
								parts.append(
									"\(result.ephemeral) secret(s) are memory-only until quit — unlock and re-import if needed.")
							}
							if result.failed > 0 {
								parts.append("\(result.failed) secret(s) did not land.")
							}
							message = parts.joined(separator: " ")
							if unlockFailed && !secrets.isEmpty {
								showAgentAlert(
									title: "Secrets not imported",
									message:
										"Config from \(fileName) was imported, but the unlock prompt was declined. Import the backup again to store secrets."
								)
							} else if !secrets.isEmpty && result.secrets == 0 && result.ephemeral == 0 {
								showAgentAlert(
									title: "Secrets not imported",
									message:
										"None of the \(secrets.count) secret(s) in this backup could be stored. Try the import again, or enter secrets manually in each auth."
								)
							} else if result.failed > 0 {
								showAgentAlert(
									title: "Some secrets missing",
									message:
										"\(result.failed) of \(secrets.count) secret(s) from the backup did not land in the Keychain. Check the Activity log and re-import or type secrets in Setup."
								)
							}
							onImportFinished()
						}
					}
				} catch {
					message = "Import failed: \(error)"
					showAgentAlert(title: "Import failed", message: String(describing: error), style: .critical)
				}
			}
		}
	}

	private func exportConfig() {
		do {
			let data = try store.exportSetupJSON()
			guard
				let url = chooseSaveFile(
					name: "buddy-agent-export.json",
					types: [.json],
					prompt: "Save JSON for 'buddy setup import' or the Agent import picker")
			else { return }
			try writeExport(data, to: url)
			message = "Exported identity to \(url.lastPathComponent) (no secrets)."
		} catch {
			message = "Export failed: \(error)"
		}
	}

	private func exportBackupFile(passphrase: String) {
		message = "Writing cred backup…"
		Task { @MainActor in
			DispatchQueue.global(qos: .userInitiated).async { [gate, store] in
				var presence: LAContext?
				if AgentKeychain.backend == .dataProtection {
					switch gate.authorize(reason: "export secrets for your cred backup") {
					case .unlocked:
						presence = gate.presenceContext
					case .declined:
						DispatchQueue.main.async {
							message = "Export cancelled — unlock is required to read secrets."
							showAgentAlert(
								title: "Unlock required",
								message:
									"Cred backups include secrets from the Keychain. Try exporting again and complete the unlock prompt, or use config-only export if you only need identity."
							)
						}
						return
					case .timedOut:
						DispatchQueue.main.async {
							message = "Export cancelled — unlock prompt timed out."
							showAgentAlert(
								title: "Unlock timed out",
								message: "Try exporting again and complete the unlock prompt.")
						}
						return
					case .unavailable:
						DispatchQueue.main.async {
							message = "Export cancelled — unlock is not available."
							showAgentAlert(
								title: "Unlock unavailable",
								message:
									"This Mac cannot authenticate for Keychain access. Use config-only export if you only need identity."
							)
						}
						return
					}
				}
				do {
					let packed = try store.credBackupData(
						passphrase: passphrase, presence: presence)
					DispatchQueue.main.async {
						guard
							let url = chooseSaveFile(
								name: "buddy-agent.creds",
								types: [.json, .data],
								prompt: "Save encrypted backup for 'buddy cred restore' or Agent import")
						else {
							message = "Export cancelled."
							return
						}
						do {
							try writeExport(packed.data, to: url)
							var parts = [
								"Exported \(packed.result.auths) auth(s) to \(url.lastPathComponent).",
							]
							if !packed.result.missingSecrets.isEmpty {
								parts.append(
									"Missing secrets: \(packed.result.missingSecrets.joined(separator: ", ")).")
								let allMissing =
									packed.result.auths > 0
									&& packed.result.missingSecrets.count == packed.result.auths
								showAgentAlert(
									title: allMissing ? "Backup has no secrets" : "Some secrets missing",
									message: allMissing
										? "The file was written but none of your auth secrets could be read. Export while unlocked, or secrets will be missing on restore."
										: "The file was written but these secrets could not be read: \(packed.result.missingSecrets.joined(separator: ", ")). Unlock and export again if you need a full backup."
								)
							}
							message = parts.joined(separator: " ")
						} catch {
							message = "Export failed: \(error)"
							showAgentAlert(
								title: "Export failed", message: String(describing: error), style: .critical)
						}
					}
				} catch {
					DispatchQueue.main.async {
						message = "Export failed: \(error)"
						showAgentAlert(
							title: "Export failed", message: String(describing: error), style: .critical)
					}
				}
			}
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

	private func chooseSaveFile(name: String, types: [UTType], prompt: String) -> URL? {
		let panel = NSSavePanel()
		panel.allowedContentTypes = types
		panel.canCreateDirectories = true
		panel.nameFieldStringValue = name
		panel.message = prompt
		guard panel.runModal() == .OK, let url = panel.url else { return nil }
		return url
	}

	private func writeExport(_ data: Data, to url: URL) throws {
		try data.write(to: url, options: [.atomic])
		try FileManager.default.setAttributes(
			[.posixPermissions: 0o600], ofItemAtPath: url.path)
	}
}

private struct BackupPrompt: Identifiable {
	let id = UUID()
	let url: URL
}

/// Shown when no workplaces or auths exist yet — points people at the fastest path to try buddy.
private struct EmptySetupBanner: View {
	let onQuickSetup: () -> Void
	let onImportBackup: () -> Void
	let onImportConfig: () -> Void

	var body: some View {
		VStack(alignment: .leading, spacing: 12) {
			Label("Get started", systemImage: "sparkles")
				.font(.headline)
			Text(
				"Buddy needs one workplace and one auth before it can mint tokens. Quick setup covers basic login, Jamf Pro API clients, or Account platform (environment scope)."
			)
			.font(.callout)
			.foregroundStyle(.secondary)
			HStack(spacing: 12) {
				Button("Quick setup…", action: onQuickSetup)
					.buttonStyle(.borderedProminent)
				Button("Import cred backup…", action: onImportBackup)
				Button("Import config only…", action: onImportConfig)
			}
			Text(
				"Or add workplaces and auths manually below. Saving while locked asks you to unlock. Run `buddy cred status` when a row shows secret stored."
			)
			.font(.caption2)
			.foregroundStyle(.tertiary)
		}
		.padding(14)
		.background(Color.accentColor.opacity(0.08), in: RoundedRectangle(cornerRadius: 10))
		.overlay(
			RoundedRectangle(cornerRadius: 10)
				.strokeBorder(Color.accentColor.opacity(0.25), lineWidth: 1))
	}
}

/// One workplace + one auth — basic, instance oauth, or platform environment.
private enum QuickSetupMode: String, CaseIterable, Identifiable {
	case basic
	case oauth
	case platformEnvironment

	var id: String { rawValue }

	var label: String {
		switch self {
		case .basic: return "Basic (Jamf Pro username & password)"
		case .oauth: return "OAuth (Jamf Pro API client)"
		case .platformEnvironment: return "Platform (Account API, environment)"
		}
	}

	var intro: String {
		switch self {
		case .basic:
			return "Jamf Pro local login — workplace is your Jamf Pro URL plus a basic auth with username and password."
		case .oauth:
			return "Jamf Pro API client — paste JSON or curl from Settings → System Settings → API Roles and Clients."
		case .platformEnvironment:
			return "Jamf Account environment scope — region and environment ID on the workplace; paste the Platform integration curl or enter client ID and secret."
		}
	}
}

private struct QuickSetupSheet: View {
	@ObservedObject var store: AgentStore
	@ObservedObject var state: AgentState
	let gate: UnlockGate
	let onFinished: (String) -> Void

	@State private var authMode: QuickSetupMode = .basic
	@State private var workplaceName = "sandbox"
	@State private var jamfURL = ""
	@State private var region = PlatformRegions.defaultValue
	@State private var environmentId = ""
	@State private var authName = ""
	@State private var username = ""
	@State private var pasteBlob = ""
	@State private var clientId = ""
	@State private var secret = ""
	@State private var pasteError: String?
	@State private var saveError: String?
	@State private var saving = false
	@Environment(\.dismiss) private var dismiss

	private var normalizedJamfURL: String? {
		JamfProURL.normalized(jamfURL)
	}

	private var jamfURLError: String? {
		JamfProURL.validationError(for: jamfURL)
	}

	var body: some View {
		VStack(alignment: .leading, spacing: 12) {
			Text("Quick setup").font(.title3)
			Text(authMode.intro)
				.font(.callout)
				.foregroundStyle(.secondary)
			Form {
				Section("Auth type") {
					FormMenuPicker(
						label: "How you sign in",
						selection: Binding(
							get: { authMode.rawValue },
							set: { raw in
								if let mode = QuickSetupMode(rawValue: raw) {
									authMode = mode
								}
							}),
						choices: QuickSetupMode.allCases.map { ($0.label, $0.rawValue) })
				}
				Section(workplaceSectionTitle) {
					TextField("Workplace name", text: $workplaceName)
					if authMode == .platformEnvironment {
						FormMenuPicker(
							label: "Region",
							selection: $region,
							choices: PlatformRegions.known)
						TextField("Environment ID", text: $environmentId)
							.autocorrectionDisabled()
						Text("Environment-scoped platform auth — no Jamf Pro URL on this workplace.")
							.font(.caption)
							.foregroundStyle(.secondary)
					} else {
						HStack(alignment: .firstTextBaseline, spacing: 8) {
							Text("JamfPro URL")
								.fixedSize(horizontal: true, vertical: false)
							TextField("", text: $jamfURL)
								.agentLabeledField()
								.autocorrectionDisabled()
						}
						if let jamfURLError {
							Text(jamfURLError).font(.caption).foregroundStyle(.red)
						}
					}
				}
				if authMode != .basic {
					Section("Paste from Jamf (optional)") {
						jamfPasteEditor(text: $pasteBlob, placeholder: pastePlaceholder)
						HStack {
							Button("Apply paste") { applyPaste() }
								.disabled(pasteBlob.trimmed.isEmpty)
							Button("Clear") {
								pasteBlob = ""
								pasteError = nil
							}
							.disabled(pasteBlob.isEmpty && pasteError == nil)
						}
						if let pasteError {
							Text(pasteError).font(.caption).foregroundStyle(.red)
						}
					}
				}
				Section("Credentials") {
					TextField("Auth name", text: $authName)
					if authMode == .basic {
						TextField("Username", text: $username)
							.textContentType(.username)
							.autocorrectionDisabled()
						SecureField("Password", text: $secret)
							.textContentType(.password)
					} else {
						if authMode == .platformEnvironment {
							Text("Scope: environment (Account gateway for this region + env ID)")
								.font(.caption)
								.foregroundStyle(.secondary)
						}
						TextField("Client ID", text: $clientId)
							.textContentType(.username)
							.autocorrectionDisabled()
						SecureField("Client secret", text: $secret)
							.textContentType(.password)
					}
				}
			}
			.formStyle(.grouped)

			if let saveError {
				Text(saveError).font(.callout).foregroundStyle(.red)
			}

			HStack {
				Spacer()
				Button("Cancel") { dismiss() }
					.disabled(saving)
				Button("Save and close") { save() }
					.keyboardShortcut(.defaultAction)
					.disabled(saving || !canSave)
			}
		}
		.padding()
		.frame(minWidth: 480, minHeight: 560)
	}

	private var workplaceSectionTitle: String {
		switch authMode {
		case .basic, .oauth: return "Jamf Pro site"
		case .platformEnvironment: return "Account environment"
		}
	}

	private var pastePlaceholder: String {
		authMode == .platformEnvironment
			? "Paste Platform integration curl…"
			: "Paste API client JSON or curl…"
	}

	private var canSave: Bool {
		if store.configEditsBlocked { return false }
		guard !workplaceName.trimmed.isEmpty, !authName.trimmed.isEmpty, !secret.isEmpty else {
			return false
		}
		switch authMode {
		case .basic:
			return normalizedJamfURL != nil && !username.trimmed.isEmpty
		case .oauth:
			return normalizedJamfURL != nil && !clientId.trimmed.isEmpty
		case .platformEnvironment:
			return !region.trimmed.isEmpty && !environmentId.trimmed.isEmpty && !clientId.trimmed.isEmpty
		}
	}

	private func applyPaste() {
		pasteError = nil
		do {
			let parsed = try AgentPasteCreds.parse(pasteBlob)
			if authName.trimmed.isEmpty, !parsed.clientName.isEmpty {
				authName = parsed.clientName
			}
			if !parsed.clientId.isEmpty { clientId = parsed.clientId }
			if !parsed.clientSecret.isEmpty { secret = parsed.clientSecret }
			switch parsed.type {
			case "platform":
				authMode = .platformEnvironment
				if !parsed.region.isEmpty {
					let parsedRegion = PlatformRegions.normalized(parsed.region)
					if PlatformRegions.isKnown(parsedRegion) {
						region = parsedRegion
					} else {
						pasteError =
							"Region \"\(parsed.region)\" is not us, eu, or apac — pick a region below, or use full Setup for other regions."
					}
				}
				if parsed.scope == "organization" {
					pasteError =
						"Paste is organization-scoped platform — pick Platform (environment) and set region manually, or use full Setup."
				}
			case "oauth":
				authMode = .oauth
				if !parsed.instanceURL.isEmpty && jamfURL.trimmed.isEmpty {
					jamfURL = parsed.instanceURL
				}
			default:
				break
			}
			pasteBlob = ""
		} catch {
			pasteError = error.localizedDescription
		}
	}

	private func save() {
		let wpName = workplaceName.trimmed
		let name = authName.trimmed
		guard canSave else { return }
		if authMode == .basic || authMode == .oauth {
			guard let url = normalizedJamfURL else {
				saveError = jamfURLError ?? "Enter a valid Jamf Pro URL."
				return
			}
			jamfURL = url
		}
		saving = true
		saveError = nil
		let workplace = buildWorkplace()
		var auth = buildAuth(linkedTo: wpName)
		let secretValue = secret
		DispatchQueue.global(qos: .userInitiated).async { [gate] in
			var presence: LAContext?
			if AgentKeychain.backend == .dataProtection {
				switch gate.authorize(reason: "store secrets for \(name)") {
				case .unlocked:
					presence = gate.presenceContext
				case .declined:
					DispatchQueue.main.async {
						saving = false
						saveError = "Unlock declined — try again and complete the prompt."
					}
					return
				case .timedOut:
					DispatchQueue.main.async {
						saving = false
						saveError = "Unlock timed out — try again."
					}
					return
				case .unavailable:
					DispatchQueue.main.async {
						saving = false
						saveError = "Unlock unavailable on this Mac."
					}
					return
				}
			}
			DispatchQueue.main.async {
				store.upsertWorkplace(name: wpName, workplace: workplace)
				store.setDefaultContext(wpName)
				store.upsertAuth(name: name, auth: auth)
				var ok = AgentKeychain.set(
					secretValue, auth: name, kind: auth.secretKind, context: presence)
				if !ok, presence == nil, AgentKeychain.backend == .dataProtection,
					gate.authorize(reason: "store secrets for \(name)") == .unlocked
				{
					ok = AgentKeychain.set(
						secretValue, auth: name, kind: auth.secretKind,
						context: gate.presenceContext)
				}
				store.markSecretsChanged()
				saving = false
				if !AgentKeychain.hasAny(auth: name) {
					saveError = "Keychain write failed — secret not stored."
					return
				}
				if !ok, AgentKeychain.backend == .dataProtection {
					saveError = "Secret is memory-only until quit — unlock and save again."
					return
				}
				onFinished(
					"Quick setup saved workplace \(wpName) and \(authMode.label.lowercased()) auth \(name). Run `buddy cred status` to verify."
				)
				dismiss()
			}
		}
	}

	private func buildWorkplace() -> Workplace {
		switch authMode {
		case .basic, .oauth:
			return Workplace(url: normalizedJamfURL ?? jamfURL.trimmed)
		case .platformEnvironment:
			var wp = Workplace()
			wp.region = PlatformRegions.normalized(region)
			wp.environmentId = environmentId.trimmed
			return wp
		}
	}

	private func buildAuth(linkedTo workplace: String) -> AgentAuth {
		var auth = AgentAuth()
		auth.contexts = [workplace]
		switch authMode {
		case .basic:
			auth.type = "basic"
			auth.username = username.trimmed
		case .oauth:
			auth.type = "oauth"
			auth.clientId = clientId.trimmed
		case .platformEnvironment:
			auth.type = "platform"
			auth.scope = "environment"
			auth.clientId = clientId.trimmed
		}
		return auth
	}
}

struct WorkplaceEditDraft: Identifiable {
	let id = UUID()
	var name: String
	var previousName: String?
	var workplace: Workplace
	var kind: String
	var parent: WorkplaceTreeNode?

	var isNew: Bool { previousName == nil || previousName?.isEmpty == true }
}

struct AuthEditDraft: Identifiable {
	let id = UUID()
	var name: String
	var previousName: String?
	var auth: AgentAuth
}

private struct BearerDetailsDraft: Identifiable {
	let authName: String
	var id: String { authName }
}

/// Recursive org → environment → product-tenant list. Linked auths hang under each workplace.
private struct WorkplaceTreeList: View {
	@ObservedObject var store: AgentStore
	@ObservedObject var state: AgentState
	let gate: UnlockGate
	let nodes: [WorkplaceTreeNode]
	let selectedWorkplace: String?
	let onSelect: (String) -> Void
	let onEdit: (WorkplaceTreeNode) -> Void
	let onSetDefault: (String) -> Void
	let onDelete: (String) -> Void
	let onShowBearer: (String) -> Void
	let onEditAuth: (String) -> Void
	let onDeleteAuth: (String) -> Void

	var body: some View {
		VStack(alignment: .leading, spacing: 0) {
			ForEach(Array(nodes.enumerated()), id: \.element.id) { index, node in
				if index > 0 {
					Divider().opacity(0.45)
				}
				WorkplaceTreeRow(
					store: store,
					node: node,
					isSelected: selectedWorkplace == node.name,
					onSelect: { onSelect(node.name) },
					onEdit: { onEdit(node) },
					onSetDefault: { onSetDefault(node.name) },
					onDelete: { onDelete(node.name) })
				let linked = authNames(forWorkplace: node.name)
				if !linked.isEmpty {
					VStack(alignment: .leading, spacing: 0) {
						ForEach(linked, id: \.self) { name in
							WorkplaceAuthRow(
								store: store,
								state: state,
								gate: gate,
								name: name,
								auth: store.auths[name] ?? AgentAuth(),
								onShowBearer: { onShowBearer(name) },
								onEdit: { onEditAuth(name) },
								onDelete: { onDeleteAuth(name) })
						}
					}
					.padding(.leading, 46)
				}
				if !node.children.isEmpty {
					WorkplaceTreeList(
						store: store,
						state: state,
						gate: gate,
						nodes: node.children,
						selectedWorkplace: selectedWorkplace,
						onSelect: onSelect,
						onEdit: onEdit,
						onSetDefault: onSetDefault,
						onDelete: onDelete,
						onShowBearer: onShowBearer,
						onEditAuth: onEditAuth,
						onDeleteAuth: onDeleteAuth)
						.padding(.leading, 22)
				}
			}
		}
	}

	private func authNames(forWorkplace name: String) -> [String] {
		store.sortedAuthNames.filter { authName in
			store.auths[authName]?.contexts.contains(name) == true
		}
	}
}

private struct WorkplaceTreeRow: View {
	@ObservedObject var store: AgentStore
	let node: WorkplaceTreeNode
	let isSelected: Bool
	let onSelect: () -> Void
	let onEdit: () -> Void
	let onSetDefault: () -> Void
	let onDelete: () -> Void

	private var isDefault: Bool {
		store.defaultContext.trimmed == node.name
	}

	var body: some View {
		HStack(spacing: 8) {
			DefaultContextButton(isDefault: isDefault, action: onSetDefault)
			.disabled(store.configEditsBlocked)
			Button(action: onSelect) {
				HStack(spacing: 8) {
					WorkplaceKindBadge(kind: node.kind)
					Text(node.name)
						.font(.callout.monospaced())
						.fontWeight(.medium)
					if let problem = node.workplace.problem() {
						Image(systemName: "exclamationmark.triangle.fill")
							.font(.caption)
							.foregroundStyle(.orange)
							.accessibilityLabel(problem)
					}
				}
				.frame(minWidth: 0, maxWidth: .infinity, alignment: .leading)
			}
			.buttonStyle(.plain)
			HStack(spacing: 2) {
				Button(action: onEdit) {
					Image(systemName: "pencil")
				}
				.buttonStyle(.borderless)
				.disabled(store.configEditsBlocked)
				.accessibilityLabel("Edit")
				DestructiveTrashButton(
					dialogTitle: "Delete workplace?",
					dialogMessage:
						"Remove “\(node.name)” from the Agent config? Auths may lose this context; empty auths are removed.",
					action: onDelete,
					disabled: store.configEditsBlocked || !node.canDelete,
					accessibilityLabel: node.canDelete ? "Delete" : "Remove children first"
				)
			}
		}
		.padding(.vertical, 6)
		.padding(.horizontal, 8)
		.background(
			isSelected ? Color.accentColor.opacity(0.14) : Color.clear,
			in: RoundedRectangle(cornerRadius: 6))
		.contentShape(RoundedRectangle(cornerRadius: 6))
	}
}

private struct DefaultContextButton: View {
	let isDefault: Bool
	let action: () -> Void
	@State private var hovered = false

	var body: some View {
		Button(action: action) {
			Image(systemName: isDefault ? "arrowtriangle.right.fill" : "arrowtriangle.right")
				.font(.system(size: 11, weight: .bold))
				.foregroundStyle(isDefault ? Color.accentColor : Color.secondary.opacity(hovered ? 0.85 : 0.28))
				.frame(width: 16, height: 16)
				.contentShape(Rectangle())
		}
		.buttonStyle(.plain)
		.onHover { hovered = $0 }
		.help(isDefault ? "Default context" : "Set as default context")
		.accessibilityLabel(isDefault ? "Default context" : "Set as default context")
	}
}

private struct WorkplaceAuthRow: View {
	@ObservedObject var store: AgentStore
	@ObservedObject var state: AgentState
	let gate: UnlockGate
	let name: String
	let auth: AgentAuth
	let onShowBearer: () -> Void
	let onEdit: () -> Void
	let onDelete: () -> Void

	private var caption: String {
		var parts = [auth.type]
		let scope = auth.displayScope(workplaces: store.workplaces)
		if scope != "—", !scope.isEmpty { parts.append(scope) }
		return parts.joined(separator: " · ")
	}

	var body: some View {
		HStack(spacing: 8) {
			AuthTokenCell(
				store: store, state: state, gate: gate, authName: name,
				onShowDetails: onShowBearer)
			Button(action: onEdit) {
				HStack(spacing: 8) {
					Text(name)
						.font(.callout.monospaced())
						.lineLimit(1)
						.truncationMode(.tail)
					Text(caption)
						.font(.caption)
						.foregroundStyle(.secondary)
						.lineLimit(1)
					if let problem = auth.problem(name: name, workplaces: store.workplaces),
						!(store.hasSecret(forAuth: name) && problem == "secret not stored yet")
					{
						Image(systemName: "exclamationmark.triangle")
							.foregroundStyle(.orange)
							.accessibilityLabel(problem)
					}
					Spacer(minLength: 0)
				}
				.contentShape(Rectangle())
			}
			.buttonStyle(.plain)
			.accessibilityLabel("Edit \(name)")
			DestructiveTrashButton(
				dialogTitle: "Delete auth?",
				dialogMessage:
					"Remove “\(name)” from the Agent config? Keychain secrets for this auth will be deleted.",
				action: onDelete,
				disabled: store.configEditsBlocked
			)
		}
		.padding(.vertical, 3)
		.padding(.trailing, 8)
	}
}

private struct WorkplaceKindBadge: View {
	let kind: String

	private var style: WorkplaceGlyph {
		WorkplaceGlyph.style(for: kind)
	}

	var body: some View {
		Image(systemName: style.symbol)
			.font(.system(size: 13, weight: .semibold))
			.symbolRenderingMode(.hierarchical)
			.foregroundStyle(style.tint)
			.frame(width: 28, height: 28)
			.background(
				style.tint.opacity(0.18),
				in: RoundedRectangle(cornerRadius: 8, style: .continuous))
			.accessibilityLabel(style.label)
	}
}

private struct WorkplaceGlyph {
	var symbol: String
	var tint: Color
	var label: String

	static func style(for kind: String) -> WorkplaceGlyph {
		switch kind {
		case "organization":
			return WorkplaceGlyph(symbol: "building.2.fill", tint: Color(red: 0.16, green: 0.45, blue: 0.96), label: "Organization")
		case "environment":
			return WorkplaceGlyph(symbol: "globe.americas.fill", tint: Color(red: 0.12, green: 0.68, blue: 0.55), label: "Environment")
		case "product-tenant", "tenant":
			return WorkplaceGlyph(symbol: "desktopcomputer", tint: Color(red: 0.56, green: 0.36, blue: 0.95), label: "Product tenant")
		case "instance":
			return WorkplaceGlyph(symbol: "server.rack", tint: Color(red: 0.95, green: 0.58, blue: 0.12), label: "Instance")
		default:
			return WorkplaceGlyph(symbol: "folder.fill", tint: Color.secondary, label: kind)
		}
	}
}

private struct AuthTokenCell: View {
	@ObservedObject var store: AgentStore
	@ObservedObject var state: AgentState
	let gate: UnlockGate
	let authName: String
	let onShowDetails: () -> Void
	@State private var minting = false
	@State private var mintError: String?

	private var tokens: [TokenSummary] { state.tokens(forAuth: authName) }
	private var canMint: Bool { store.canMint(forAuth: authName) }
	private var hasTokens: Bool { !tokens.isEmpty }

	var body: some View {
		Button(action: primaryAction) {
			Group {
				if minting {
					ProgressView().controlSize(.small)
				} else if hasTokens {
					Image(systemName: "key.fill")
						.foregroundStyle(tokenColor)
				} else if canMint {
					Image(systemName: "key")
						.font(.caption)
						.foregroundStyle(Color.accentColor)
				} else {
					Image(systemName: "key")
						.font(.caption)
						.foregroundStyle(.quaternary)
				}
			}
			.frame(width: 28, height: 20)
		}
		.buttonStyle(.borderless)
		.disabled(minting || (!hasTokens && !canMint))
		.accessibilityHint(hasTokens ? "Cached bearer — click for status" : bearerHelp)
	}

	private func primaryAction() {
		if hasTokens {
			onShowDetails()
		} else {
			mintBearer()
		}
	}

	private var bearerHelp: String {
		if let mintError { return mintError }
		if canMint { return "No bearer cached — click to mint" }
		return store.mintUnavailableReason(forAuth: authName)
	}

	private var tokenColor: Color {
		tokens.contains(where: \.usable) ? .green : .orange
	}

	private func mintBearer() {
		minting = true
		mintError = nil
		DispatchQueue.global(qos: .userInitiated).async {
			let error = store.mintTokens(forAuth: authName, gate: gate, state: state)
			DispatchQueue.main.async {
				minting = false
				mintError = error
			}
		}
	}
}

private struct AuthTokenSheet: View {
	@ObservedObject var state: AgentState
	let authName: String
	let onDismiss: () -> Void

	private var tokens: [TokenSummary] { state.tokens(forAuth: authName) }

	var body: some View {
		VStack(alignment: .leading, spacing: 12) {
			HStack {
				Text("Cached bearer — \(authName)").font(.headline)
				Spacer()
				Button("Done", action: onDismiss)
			}
			if tokens.isEmpty {
				Text("No bearer cached.").foregroundStyle(.secondary)
			} else {
				ForEach(tokens) { token in
					VStack(alignment: .leading, spacing: 6) {
						Text(token.baseURL).font(.caption.monospaced()).textSelection(.enabled)
						Text(tokenExpiryText(token))
							.font(.callout)
							.foregroundStyle(token.usable ? Color.secondary : Color.orange)
						Button("Drop bearer") {
							state.dropToken(key: token.key)
							if state.tokens(forAuth: authName).isEmpty {
								DispatchQueue.main.async { onDismiss() }
							}
						}
					}
					if token.id != tokens.last?.id {
						Divider()
					}
				}
			}
			Text("Lock clears every cached bearer.").font(.caption2).foregroundStyle(.tertiary)
		}
		.padding()
		.frame(minWidth: 300)
	}

	private func tokenExpiryText(_ token: TokenSummary) -> String {
		let remaining = Int(token.expiresAt.timeIntervalSinceNow)
		if remaining <= 0 { return "Expired — drop to force a fresh mint" }
		let minutes = remaining / 60
		return minutes > 0 ? "Expires in \(minutes) min" : "Expires in \(remaining)s"
	}
}

struct WorkplaceEditor: View {
	@ObservedObject var store: AgentStore
	@ObservedObject var state: AgentState
	let gate: UnlockGate
	@State var draft: WorkplaceEditDraft
	@State private var useAsDefault = false
	@State private var saveError: String?
	@State private var showDeleteConfirm = false
	@Environment(\.dismiss) private var dismiss

	private var deleteWorkplaceName: String {
		draft.previousName ?? draft.name
	}

	var body: some View {
		Form {
			Section {
				Text(draft.isNew ? "New workplace" : draft.name).font(.title3)
				if store.isManagedWorkplace(draft.name) || store.isManagedWorkplace(draft.previousName ?? "") {
					Label("This workplace is managed by MDM.", systemImage: "lock.fill")
						.font(.callout)
						.foregroundStyle(.secondary)
				}
			}
			Section("Identity") {
				TextField("Name", text: $draft.name)
				TextField("Kind", text: .constant(kindLabel))
					.disabled(true)
			}
			if draft.kind != "organization" {
				Section("Jamf Account") {
					if draft.kind == "environment" {
						TextField("Region", text: $draft.workplace.region)
						Text("Any Jamf Account region code (e.g. us, eu, apac).")
							.font(.caption2)
							.foregroundStyle(.tertiary)
						TextField("Environment ID", text: $draft.workplace.environmentId)
					}
					if draft.kind == "product-tenant" || draft.kind == "tenant" {
						TextField("Jamf Pro tenant ID", text: $draft.workplace.tenantId)
					}
					if draft.kind == "instance" || draft.kind == "product-tenant" || draft.kind == "tenant" {
						TextField("Jamf Pro URL", text: $draft.workplace.url)
							.autocorrectionDisabled()
					}
					if draft.kind != "organization" {
						TextField("Parent org", text: $draft.workplace.org)
					}
				}
			} else {
				Section("Organization") {
					Text("Organization workplaces sit above environments. No URL or region.")
						.font(.caption)
						.foregroundStyle(.secondary)
				}
			}
			Section("Default") {
				Toggle("Use as default context", isOn: $useAsDefault)
					.disabled(store.managedPolicyLocked("defaultContext"))
			}
			Section("Builtin policy") {
				Text("Optional deny/allow lists for this workplace (`deny_builtins` / `allow_builtins`).")
					.font(.caption)
					.foregroundStyle(.secondary)
				BuiltinPatternsField(
					title: "Deny builtins",
					help: "One pattern per line.",
					patterns: $draft.workplace.denyBuiltins)
				BuiltinPatternsField(
					title: "Allow builtins",
					help: "Overrides deny at this workplace.",
					patterns: $draft.workplace.allowBuiltins)
			}
		}
		.formStyle(.grouped)
		.safeAreaInset(edge: .bottom, spacing: 0) {
			VStack(alignment: .leading, spacing: 8) {
				if let saveError {
					Text(saveError).font(.callout).foregroundStyle(.red)
				}
				HStack {
					if !draft.isNew {
						Button("Delete", role: .destructive) { showDeleteConfirm = true }
							.disabled(!maySaveConfig)
					}
					Spacer()
					Button("Cancel") { dismiss() }
					Button("Save") { save() }
						.keyboardShortcut(.defaultAction)
						.disabled(
							!maySaveConfig
								|| draft.name.trimmed.isEmpty || store.isManagedWorkplace(draft.name)
								|| store.isManagedWorkplace(draft.previousName ?? ""))
				}
			}
			.padding(.horizontal, 16)
			.padding(.vertical, 10)
			.background(.bar)
		}
		.confirmationDialog(
			"Delete workplace?", isPresented: $showDeleteConfirm, titleVisibility: .visible
		) {
			Button("Delete", role: .destructive) {
				let name = deleteWorkplaceName
				AgentLockUI.perform(gate: gate, state: state) {
					store.removeWorkplace(name)
					dismiss()
				}
			}
			Button("Cancel", role: .cancel) {}
		} message: {
			Text(
				"Remove “\(deleteWorkplaceName)” from the Agent config? Auths may lose this context; empty auths are removed."
			)
		}
		.onAppear {
			let name = draft.previousName ?? draft.name
			useAsDefault = store.defaultContext == name && !name.trimmed.isEmpty
		}
	}

	private var maySaveConfig: Bool {
		!store.configEditsBlocked
	}

	private var kindLabel: String {
		switch draft.kind {
		case "tenant": return "product-tenant"
		default: return draft.kind
		}
	}

	private func save() {
		let name = draft.name.trimmed
		guard !name.isEmpty else { return }
		guard !store.isManagedWorkplace(name), !store.isManagedWorkplace(draft.previousName ?? "") else { return }
		let isNew = draft.isNew
		let kind = draft.kind
		let parent = draft.parent
		let workplace = draft.workplace
		let previousName = draft.previousName
		let makeDefault = useAsDefault
		let close = dismiss
		AgentLockUI.perform(gate: gate, state: state) {
			if isNew {
				if kind == "organization" {
					store.addOrganization(name: name)
				} else {
					store.addChildWorkplace(
						name: name, parent: parent, kind: kind,
						orgName: workplace.org)
				}
				store.upsertWorkplace(name: name, workplace: workplace)
			} else {
				store.upsertWorkplace(
					name: name, workplace: workplace, previousName: previousName)
			}
			if makeDefault || store.defaultContext.trimmed.isEmpty {
				store.setDefaultContext(name)
			}
			close()
		}
	}
}

@ViewBuilder
private func jamfPasteEditor(text: Binding<String>, placeholder: String) -> some View {
	TextEditor(text: text)
		.font(.system(.caption, design: .monospaced))
		.frame(height: 96)
		.overlay(alignment: .topLeading) {
			if text.wrappedValue.isEmpty {
				Text(placeholder)
					.font(.caption)
					.foregroundStyle(.tertiary)
					.padding(.top, 8)
					.padding(.leading, 4)
					.allowsHitTesting(false)
			}
		}
		.help(AgentPasteCreds.pasteFieldHelp)
}

private struct FormMenuPicker: View {
	let label: String
	@Binding var selection: String
	let choices: [(label: String, value: String)]
	var onChange: (() -> Void)? = nil

	var body: some View {
		LabeledTrailingRow(label: label) {
			Menu {
				ForEach(choices, id: \.value) { choice in
					Button {
						selection = choice.value
						onChange?()
					} label: {
						if selection == choice.value {
							Label(choice.label, systemImage: "checkmark")
						} else {
							Text(choice.label)
						}
					}
				}
			} label: {
				Text(displayLabel)
			}
			.menuStyle(.borderlessButton)
		}
	}

	private var displayLabel: String {
		choices.first { $0.value == selection }?.label ?? selection
	}
}

struct AuthEditor: View {
	@ObservedObject var store: AgentStore
	@ObservedObject var state: AgentState
	let gate: UnlockGate
	@State var draft: AuthEditDraft
	@State private var secret: String = ""
	@State private var pasteBlob: String = ""
	@State private var pasteError: String?
	@State private var pasteHint: String?
	@State private var saveError: String?
	@State private var showDeleteConfirm = false
	@Environment(\.dismiss) private var dismiss

	private let types = ["oauth", "basic", "platform"]

	private var deleteAuthName: String {
		draft.previousName ?? draft.name
	}

	var body: some View {
		Form {
			Section {
				Text(draft.name.isEmpty ? "New auth" : draft.name).font(.title3)
				if store.isManagedAuth(draft.name) || store.isManagedAuth(draft.previousName ?? "") {
					Label("This auth is managed by MDM.", systemImage: "lock.fill")
						.font(.callout)
						.foregroundStyle(.secondary)
				}
			}
			Section("Paste from Jamf") {
					Text(
						"Jamf Pro API-client JSON or Platform integration curl — same as `buddy cred add --paste`."
					)
					.font(.caption)
					.foregroundStyle(.secondary)
					jamfPasteEditor(text: $pasteBlob, placeholder: "Paste JSON or curl here…")
					HStack {
						Button("Apply paste") { applyPaste() }
							.disabled(pasteBlob.trimmed.isEmpty)
						Button("Clear") {
							pasteBlob = ""
							pasteError = nil
							pasteHint = nil
						}
						.disabled(pasteBlob.isEmpty && pasteError == nil && pasteHint == nil)
					}
					if let pasteError {
						Text(pasteError).font(.caption).foregroundStyle(.red)
					}
					if let pasteHint {
						Text(pasteHint).font(.caption).foregroundStyle(.secondary)
					}
				}
				Section("Identity") {
					TextField("Auth name", text: $draft.name)
					FormMenuPicker(
						label: "Type", selection: $draft.auth.type,
						choices: types.map { ($0, $0) },
						onChange: { pruneInvalidWorkplaceLinks() })
					if draft.auth.type == "platform" {
						FormMenuPicker(
							label: "Scope", selection: $draft.auth.scope,
							choices: [
								("environment", "environment"),
								("organization", "organization"),
								("tenant", "tenant"),
							],
							onChange: { pruneInvalidWorkplaceLinks() })
					}
					FormMenuPicker(
						label: "Access", selection: $draft.auth.access,
						choices: [("full", ""), ("readonly", "readonly")])
				}
				Section("Credentials") {
					if draft.auth.type == "basic" {
						TextField("Username", text: $draft.auth.username)
							.textContentType(.username)
							.autocorrectionDisabled()
					} else {
						TextField("Client ID", text: $draft.auth.clientId)
							.textContentType(.username)
							.autocorrectionDisabled()
					}
					SecureField(
						draft.auth.hasSecret(name: draft.name) ? "Replace stored secret" : "Secret",
						text: $secret)
					.textContentType(.password)
				}
				Section("Linked workplaces") {
					Text(draft.auth.linkableWorkplaceHint())
						.font(.caption)
						.foregroundStyle(.secondary)
					if store.workplaces.isEmpty {
						Text("Add a workplace first").foregroundStyle(.secondary)
					} else {
						let linkable = linkableWorkplaceNames
						if linkable.isEmpty {
							Text("No workplaces match this auth type and scope.")
								.foregroundStyle(.secondary)
						} else {
							ForEach(linkable, id: \.self) { ctx in
								Toggle(
									ctx,
									isOn: Binding(
										get: { draft.auth.contexts.contains(ctx) },
										set: { on in
											if on {
												if !draft.auth.contexts.contains(ctx) {
													draft.auth.contexts.append(ctx)
												}
											} else {
												draft.auth.contexts.removeAll { $0 == ctx }
											}
										}))
							}
						}
					}
				}
				Section("Channel & builtin policy") {
					AuthChannelsField(channels: $draft.auth.channels)
					BuiltinPatternsField(
						title: "Deny builtins",
						help: "One pattern per line.",
						patterns: $draft.auth.denyBuiltins)
					BuiltinPatternsField(
						title: "Allow builtins",
						help: "Overrides deny on this auth.",
						patterns: $draft.auth.allowBuiltins)
				}
		}
		.formStyle(.grouped)
		.onAppear { pruneInvalidWorkplaceLinks() }
		.safeAreaInset(edge: .bottom, spacing: 0) {
			VStack(alignment: .leading, spacing: 8) {
				if let saveError {
					Text(saveError).font(.callout).foregroundStyle(.red)
				}
				HStack {
					if draft.previousName != nil {
						Button("Delete", role: .destructive) { showDeleteConfirm = true }
							.disabled(!maySaveConfig)
					}
					Spacer()
					Button("Cancel") { dismiss() }
					Button("Save") { save() }
						.keyboardShortcut(.defaultAction)
						.disabled(
							!maySaveConfig
								|| draft.name.trimmed.isEmpty || draft.auth.contexts.isEmpty
								|| store.isManagedAuth(draft.name)
								|| store.isManagedAuth(draft.previousName ?? ""))
				}
			}
			.padding(.horizontal, 16)
			.padding(.vertical, 10)
			.background(.bar)
		}
		.confirmationDialog(
			"Delete auth?", isPresented: $showDeleteConfirm, titleVisibility: .visible
		) {
			Button("Delete", role: .destructive) {
				let name = deleteAuthName
				AgentLockUI.perform(gate: gate, state: state) {
					store.removeAuth(name)
					dismiss()
				}
			}
			Button("Cancel", role: .cancel) {}
		} message: {
			Text(
				"Remove “\(deleteAuthName)” from the Agent config? Keychain secrets for this auth will be deleted."
			)
		}
	}

	private var maySaveConfig: Bool {
		!store.configEditsBlocked
	}

	private var linkableWorkplaceNames: [String] {
		store.workplaces.keys.sorted().filter { name in
			draft.auth.fitsWorkplace(store.workplaces[name] ?? Workplace())
		}
	}

	private func pruneInvalidWorkplaceLinks() {
		draft.auth.contexts.removeAll { name in
			guard let wp = store.workplaces[name] else { return true }
			return !draft.auth.fitsWorkplace(wp)
		}
	}

	private func applyPaste() {
		pasteError = nil
		pasteHint = nil
		do {
			let parsed = try AgentPasteCreds.parse(pasteBlob)
			if draft.name.trimmed.isEmpty, !parsed.clientName.isEmpty {
				draft.name = parsed.clientName
			}
			draft.auth.type = parsed.type
			if !parsed.clientId.isEmpty {
				draft.auth.clientId = parsed.clientId
			}
			if !parsed.clientSecret.isEmpty {
				secret = parsed.clientSecret
			}
			if parsed.type == "platform", !parsed.scope.isEmpty {
				draft.auth.scope = parsed.scope
			}
			pruneInvalidWorkplaceLinks()
			var hints: [String] = []
			switch parsed.type {
			case "platform":
				if parsed.scope == "organization" {
					hints.append("Platform auth — organization scope (US gateway, no env header).")
				} else if !parsed.region.isEmpty {
					hints.append("Platform auth — region \(parsed.region). Set region on linked workplace(s).")
				} else {
					hints.append("Platform auth — pick scope and link workplace(s).")
				}
			case "oauth":
				if !parsed.instanceURL.isEmpty {
					hints.append(
						"Instance oauth — set Jamf Pro URL to \(parsed.instanceURL) on linked workplace(s).")
				} else {
					hints.append("Instance oauth — link workplace(s) and set their Jamf Pro URL.")
				}
			default:
				break
			}
			pasteHint = hints.joined(separator: " ")
			pasteBlob = ""
		} catch {
			pasteError = error.localizedDescription
		}
	}

	private func save() {
		let name = draft.name.trimmed
		guard !name.isEmpty else { return }
		guard !store.isManagedAuth(name), !store.isManagedAuth(draft.previousName ?? "") else { return }
		let secretValue = secret
		let row = draft.auth
		let previousName = draft.previousName
		saveError = nil
		AgentLockUI.perform(gate: gate, state: state, reason: "save Buddy Agent config") {
			store.upsertAuth(name: name, auth: row, previousName: previousName)
			guard !secretValue.isEmpty else {
				dismiss()
				return
			}
			DispatchQueue.global(qos: .userInitiated).async { [gate] in
				let context = AgentKeychain.backend == .dataProtection ? gate.presenceContext : nil
				let ok = AgentKeychain.set(
					secretValue, auth: name, kind: row.secretKind, context: context)
				DispatchQueue.main.async {
					self.store.markSecretsChanged()
					self.secret = ""
					if !AgentKeychain.hasAny(auth: name) {
						self.saveError = "Keychain write failed — secret not stored."
						return
					}
					if !ok, AgentKeychain.backend == .dataProtection {
						self.saveError = "Secret is memory-only until quit — Keychain ACL write failed."
						return
					}
					self.dismiss()
				}
			}
		}
	}
}

struct BackupExportPassphraseSheet: View {
	let onExport: (String) -> Void

	@State private var passphrase: String = ""
	@State private var confirm: String = ""
	@Environment(\.dismiss) private var dismiss

	init(onExport: @escaping (String) -> Void) {
		self.onExport = onExport
	}

	private var canSubmit: Bool {
		!passphrase.isEmpty && passphrase == confirm
	}

	var body: some View {
		VStack(alignment: .leading, spacing: 12) {
			Text("Export cred backup").font(.title3)
			Text(
				"Choose a passphrase for this file. You will need it for `buddy cred restore` or Agent import."
			)
			.font(.callout)
			.foregroundStyle(.secondary)
			SecureField("Passphrase", text: $passphrase)
				.textContentType(.password)
			SecureField("Confirm passphrase", text: $confirm)
				.textContentType(.password)
				.onSubmit(submit)
			HStack {
				Spacer()
				Button("Cancel") { dismiss() }
				Button("Export", action: submit)
					.keyboardShortcut(.defaultAction)
					.disabled(!canSubmit)
			}
		}
		.padding()
		.frame(minWidth: 420)
	}

	private func submit() {
		guard canSubmit else { return }
		onExport(passphrase)
		passphrase = ""
		confirm = ""
		dismiss()
	}
}

struct BackupPassphraseSheet: View {
	let url: URL
	let onImport: (String) -> Void

	@State private var passphrase: String = ""
	@Environment(\.dismiss) private var dismiss

	var body: some View {
		VStack(alignment: .leading, spacing: 12) {
			Text("Import \(url.lastPathComponent)").font(.title3)
			Text("Type the passphrase used for this backup.")
				.font(.callout)
				.foregroundStyle(.secondary)
			SecureField("Backup passphrase", text: $passphrase)
				.textContentType(.password)
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

struct LogView: View {
	@ObservedObject var log: AgentLog

	private static let timeFormatter: DateFormatter = {
		let f = DateFormatter()
		f.dateFormat = "HH:mm:ss"
		return f
	}()

	var body: some View {
		VStack(alignment: .leading, spacing: 8) {
			HStack {
				Text("Activity").font(.headline)
				Spacer()
				Button("Clear") { log.clear() }
					.disabled(log.entries.isEmpty)
			}
			if log.entries.isEmpty {
				Text("Config edits, token mints, buddy socket requests, and keychain diagnostics appear here.")
					.foregroundStyle(.secondary)
				Spacer()
			} else {
				ScrollViewReader { proxy in
					ScrollView {
						LazyVStack(alignment: .leading, spacing: 4) {
							ForEach(log.entries) { entry in
								logRow(entry)
									.id(entry.id)
							}
						}
					}
					.onChange(of: log.entries.last?.id) { target in
						guard let target else { return }
						withAnimation(.easeOut(duration: 0.15)) {
							proxy.scrollTo(target, anchor: .bottom)
						}
					}
					.onAppear {
						if let target = log.entries.last?.id {
							proxy.scrollTo(target, anchor: .bottom)
						}
					}
				}
			}
			Text(
				"Also in Unified Log: log stream --predicate 'subsystem == \"\(AgentLog.subsystem)\"'"
			)
			.font(.caption2)
			.foregroundStyle(.tertiary)
			.textSelection(.enabled)
		}
		.frame(minWidth: 0, maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
	}

	private func logRow(_ entry: AgentLogEntry) -> some View {
		HStack(alignment: .top, spacing: 8) {
			Text(Self.timeFormatter.string(from: entry.date))
				.font(.caption.monospaced())
				.foregroundStyle(.secondary)
				.frame(width: 64, alignment: .leading)
			Text(entry.category.label)
				.font(.caption.weight(.semibold))
				.foregroundStyle(categoryColor(entry.category))
				.frame(width: 60, alignment: .leading)
			Text(entry.message)
				.font(.caption.monospaced())
				.textSelection(.enabled)
				.agentFlexibleWidth()
		}
		.frame(minWidth: 0, maxWidth: .infinity, alignment: .leading)
	}

	private func categoryColor(_ category: AgentLogCategory) -> Color {
		switch category {
		case .config: return .secondary
		case .mint: return .green
		case .buddy: return .accentColor
		case .keychain: return .orange
		}
	}
}

private struct ConfigIntegrityLastGoodSection: View {
	@ObservedObject var store: AgentStore
	@ObservedObject var state: AgentState
	let diagnostics: AgentConfigLastGood.Diagnostics
	let onReview: () -> Void
	@State private var expandDiff = false

	private static let stampStyle = Date.FormatStyle(date: .abbreviated, time: .standard)

	private var alertStyle: Bool { diagnostics.hasIntegrityConcern }

	var body: some View {
		VStack(alignment: .leading, spacing: 8) {
			Text("Config integrity")
				.font(.subheadline.weight(.semibold))

			infoLine("Status", diagnostics.integrity.wireValue)
			configPathRow

			if diagnostics.integrity == .sessionStale {
				Label(
					"The Keychain no longer accepts this unlock session. The config and its seal are intact — lock and unlock Buddy Agent from the menu bar. Do not re-save.",
					systemImage: "lock.rotation"
				)
				.font(.caption)
				.foregroundStyle(.orange)
			} else if diagnostics.agentAttributedDrift, diagnostics.integrity != .ok {
				Label(
					"This change is saved, but the Keychain seal could not be read back. Minting stays blocked until you Save again while unlocked.",
					systemImage: "exclamationmark.triangle"
				)
				.font(.caption)
				.foregroundStyle(.orange)
			} else if diagnostics.needsLineReview {
				Label(
					"Config was edited outside the Agent. Minting and Setup/Policy edits stay blocked until you accept or revert every changed line.",
					systemImage: "exclamationmark.triangle"
				)
				.font(.caption)
				.foregroundStyle(.orange)
				Button(state.unlocked ? "Review changes…" : "Unlock and review changes…", action: onReview)
			} else if diagnostics.integrity == .tampered {
				Label(
					"Config was edited outside the Agent and there is no last-good snapshot to review. Restore the config from backup. Minting and Setup/Policy edits stay blocked.",
					systemImage: "exclamationmark.triangle"
				)
				.font(.caption)
				.foregroundStyle(.orange)
			} else if diagnostics.hasIntegrityConcern, diagnostics.fileDriftSinceSnapshot {
				Label(
					"Config file no longer matches the last-good snapshot.",
					systemImage: "exclamationmark.triangle"
				)
				.font(.caption)
				.foregroundStyle(.orange)
			}

			if !diagnostics.needsLineReview, diagnostics.integrity == .sealed, diagnostics.fileDriftSinceSnapshot {
				Label(
					"Config file changed since the last-good snapshot. Unlock to verify the Keychain seal; diff below is from on-disk bytes.",
					systemImage: "exclamationmark.triangle"
				)
				.font(.caption)
				.foregroundStyle(.orange)
			} else if diagnostics.integrity == .sealed, !diagnostics.snapshotMissing {
				Text("Integrity shows sealed while locked — unlock to verify the Keychain seal.")
					.font(.caption)
					.foregroundStyle(.secondary)
			}

			if diagnostics.snapshotMissing {
				Text(
					"No snapshot on disk yet — it is written when you save with a valid seal, or when unlock verifies the existing seal. Future drift will show a diff here."
				)
				.font(.caption)
				.foregroundStyle(.secondary)
			} else if let meta = diagnostics.meta {
				timestampRow("Last sealed (Agent)", meta.sealedAt)
				if let at = meta.configModifiedAt {
					timestampRow("Config modified at seal", at)
				}
				if let rev = meta.agentRevision {
					infoLine("Agent at seal", rev)
				}
			}

			if let modified = diagnostics.currentModifiedAt {
				timestampRow("Config file modified now", modified)
			}
			if diagnostics.integrity != .ok, let observed = diagnostics.meta?.tamperObservedAt {
				timestampRow("Tamper first seen (unlocked)", observed)
			}

			if diagnostics.integrity != .ok, diagnostics.onlyFormattingDrift {
				Label(
					"JSON content matches the last-good snapshot; the seal mismatch is likely whitespace, key order, or a partial write. Unlock to re-seal automatically.",
					systemImage: "info.circle"
				)
				.font(.caption)
				.foregroundStyle(.secondary)
			} else if !diagnostics.needsLineReview, diagnostics.hasIntegrityConcern, !diagnostics.snapshotMissing,
				diagnostics.diffChangeCount > 0
			{
				DisclosureGroup(isExpanded: $expandDiff) {
					diffScroll
				} label: {
					Text(
						"Diff vs last-good (\(diagnostics.diffChangeCount) line\(diagnostics.diffChangeCount == 1 ? "" : "s") changed)"
					)
					.font(.caption)
				}
			} else if diagnostics.integrity == .tampered, !diagnostics.snapshotMissing,
				diagnostics.diffChangeCount == 0
			{
				Text(
					"On-disk config matches the last-good snapshot but the Keychain seal does not. Unlock to re-seal automatically."
				)
				.font(.caption)
				.foregroundStyle(.secondary)
			} else if diagnostics.integrity == .ok, !diagnostics.snapshotMissing {
				Text("Keychain seal matches the on-disk config.")
					.font(.caption)
					.foregroundStyle(.secondary)
			}

			Text(
				"Config is served while locked. Minting requires unlock and a verified config seal (Keychain-protected)."
			)
			.font(.caption2)
			.foregroundStyle(.tertiary)
			.padding(.top, 4)
		}
		.padding(10)
		.frame(maxWidth: .infinity, alignment: .leading)
		.background(
			alertStyle ? Color.orange.opacity(0.12) : Color.primary.opacity(0.05),
			in: RoundedRectangle(cornerRadius: 8))
		.onAppear {
			if alertStyle { expandDiff = true }
		}
		.onChange(of: diagnostics.fileDriftSinceSnapshot) { drift in
			if drift || diagnostics.integrity == .tampered { expandDiff = true }
		}
	}

	private var configPath: String { store.path.path }

	private var diffScroll: some View {
		ScrollView {
			LazyVStack(alignment: .leading, spacing: 0) {
				ForEach(Array(displayDiff.enumerated()), id: \.offset) { _, line in
					diffLineView(line)
				}
			}
			.frame(maxWidth: .infinity, alignment: .leading)
		}
		.frame(maxHeight: 220)
	}

	private var displayDiff: [AgentConfigLastGood.DiffLine] {
		let lines = diagnostics.normalizedDiff
		let maxLines = 400
		if lines.count <= maxLines { return lines }
		return Array(lines.prefix(maxLines))
	}

	@ViewBuilder
	private func diffLineView(_ line: AgentConfigLastGood.DiffLine) -> some View {
		switch line {
		case .unchanged(let text):
			Text("  \(text)")
				.font(.system(.caption2, design: .monospaced))
				.foregroundStyle(.secondary)
		case .removed(let text):
			Text("- \(text)")
				.font(.system(.caption2, design: .monospaced))
				.foregroundStyle(.red)
		case .added(let text):
			Text("+ \(text)")
				.font(.system(.caption2, design: .monospaced))
				.foregroundStyle(.green)
		}
	}

	private func timestampRow(_ label: String, _ date: Date) -> some View {
		infoLine(label, date.formatted(Self.stampStyle))
	}

	private func infoLine(_ label: String, _ value: String) -> some View {
		HStack(alignment: .firstTextBaseline, spacing: 8) {
			Text(label)
				.foregroundStyle(.secondary)
				.frame(width: 168, alignment: .leading)
			Text(value)
				.textSelection(.enabled)
				.font(.caption)
		}
	}

	private var configPathRow: some View {
		HStack(alignment: .firstTextBaseline, spacing: 8) {
			Text("Config file")
				.foregroundStyle(.secondary)
				.frame(width: 168, alignment: .leading)
			Text(configPath)
				.textSelection(.enabled)
				.font(.caption)
				.agentFlexibleWidth()
		}
	}
}

private struct ConfigTamperReviewDialog: View {
	@ObservedObject var store: AgentStore
	let diagnostics: AgentConfigLastGood.Diagnostics
	let onClose: () -> Void
	@State private var decisions: [Int: AgentConfigLastGood.LineDecision] = [:]
	@State private var applying = false
	@State private var reviewError: String?

	private var lines: [AgentConfigLastGood.ReviewLine] {
		AgentConfigLastGood.reviewLines(in: diagnostics.normalizedDiff)
	}

	private var decidedCount: Int {
		lines.filter { decisions[$0.id] != nil }.count
	}

	var body: some View {
		ZStack {
			Color.black.opacity(0.35)
			VStack(alignment: .leading, spacing: 12) {
				Text("Review config changes").font(.title3)
				Text(
					"Accept keeps the outside edit. Revert restores the last sealed line. A changed line is one choice, not separate + and − rows. The config is resealed only after every change has a choice."
				)
				.font(.caption)
				.foregroundStyle(.secondary)
				ScrollView {
					LazyVStack(alignment: .leading, spacing: 8) {
						ForEach(lines) { line in
							reviewRow(line)
						}
					}
				}
				.frame(maxHeight: .infinity)
				if let reviewError {
					Text(reviewError).font(.callout).foregroundStyle(.red)
				}
				HStack {
					Text("\(decidedCount) of \(lines.count) decided")
						.font(.caption)
						.foregroundStyle(.secondary)
					Spacer()
					Button("Cancel", action: onClose)
						.disabled(applying)
				}
			}
			.padding(16)
			.frame(width: 640, height: 480)
			.background(.background, in: RoundedRectangle(cornerRadius: 12))
		}
	}

	private func reviewRow(_ line: AgentConfigLastGood.ReviewLine) -> some View {
		HStack(alignment: .center, spacing: 8) {
			lineBody(line)
				.font(.system(.caption, design: .monospaced))
				.textSelection(.enabled)
				.frame(maxWidth: .infinity, alignment: .leading)
			decisionButton("Revert", line: line, decision: .revert, selectedTint: .orange)
			decisionButton("Accept", line: line, decision: .accept, selectedTint: .green)
		}
	}

	private func lineBody(_ line: AgentConfigLastGood.ReviewLine) -> some View {
		VStack(alignment: .leading, spacing: 2) {
			switch line.change {
			case .replaced(let old, let new):
				Text(marked("−", old)).foregroundStyle(.red)
				Text(marked("+", new)).foregroundStyle(.green)
			case .removed(let text):
				Text(marked("−", text)).foregroundStyle(.red)
			case .added(let text):
				Text(marked("+", text)).foregroundStyle(.green)
			}
		}
	}

	private func marked(_ mark: String, _ text: String) -> String {
		let shown = text.isEmpty ? "(blank line)" : text
		return "\(mark) \(shown)"
	}

	private func decisionButton(
		_ title: String, line: AgentConfigLastGood.ReviewLine,
		decision: AgentConfigLastGood.LineDecision, selectedTint: Color
	) -> some View {
		let selected = decisions[line.id] == decision
		return Button(title) { choose(line, decision) }
			.disabled(applying)
			.buttonStyle(.bordered)
			.tint(selected ? selectedTint : .secondary)
			.fontWeight(selected ? .semibold : .regular)
			.accessibilityLabel("\(title) \(accessibilityKind(line.change))")
	}

	private func accessibilityKind(_ change: AgentConfigLastGood.ReviewLine.Change) -> String {
		switch change {
		case .replaced: return "changed line"
		case .added: return "added line"
		case .removed: return "removed line"
		}
	}

	private func choose(_ line: AgentConfigLastGood.ReviewLine, _ decision: AgentConfigLastGood.LineDecision) {
		guard !applying else { return }
		decisions[line.id] = decision
		reviewError = nil
		guard lines.allSatisfy({ decisions[$0.id] != nil }) else { return }
		guard let resolved = AgentConfigLastGood.resolve(
			diff: diagnostics.normalizedDiff, decisions: decisions)
		else { return }
		applying = true
		store.applyTamperReview(
			resolvedJSON: resolved,
			decisions: decisions,
			diff: diagnostics.normalizedDiff
		) { ok in
			applying = false
			if ok {
				onClose()
			} else {
				reviewError = store.lastError ?? "Could not reseal the reviewed config."
			}
		}
	}
}

struct StatusView: View {
	@ObservedObject var store: AgentStore
	@ObservedObject var state: AgentState
	let gate: UnlockGate
	let socketPath: String
	@AppStorage(AgentIdleLock.userDefaultsKey) private var idleLockSeconds: Double =
		AgentIdleLock.defaultSeconds
	@AppStorage(AgentIdleLock.lastSecondsKey) private var lastIdleLockSeconds: Double =
		AgentIdleLock.defaultSeconds
	@AppStorage(AgentUnlockTimeout.userDefaultsKey) private var unlockTimeoutSeconds: Double =
		AgentUnlockTimeout.defaultSeconds
	@AppStorage(AgentUnlockTimeout.lastSecondsKey) private var lastUnlockTimeoutSeconds: Double =
		AgentUnlockTimeout.defaultSeconds

	private let idleLockChoices: [(label: String, seconds: TimeInterval)] = [
		("5 minutes", 5 * 60),
		("15 minutes", 15 * 60),
		("30 minutes", 30 * 60),
		("1 hour", 60 * 60),
		("3 hours", 3 * 60 * 60),
		("8 hours", 8 * 60 * 60),
	]

	@State private var reviewSnapshot: AgentConfigLastGood.Diagnostics?
	@State private var keychainRecoveryEpoch = 0
	@ObservedObject private var updateCheck = AgentUpdateCheck.shared

	var body: some View {
		statusScroll
			.overlay {
				if let reviewSnapshot {
					ConfigTamperReviewDialog(
						store: store,
						diagnostics: reviewSnapshot,
						onClose: { self.reviewSnapshot = nil })
				}
			}
		.onAppear {
			store.refreshConfigDiagnostics(unlocked: state.unlocked)
			refreshUpdateCheck()
		}
		.onChange(of: store.features.checkUpdates) { _ in
			refreshUpdateCheck()
		}
		.onChange(of: state.unlocked) { unlocked in
			store.refreshConfigDiagnostics(unlocked: unlocked)
		}
		.onChange(of: store.configIntegrityGeneration) { _ in
			store.refreshConfigDiagnostics(unlocked: state.unlocked)
		}
		.onReceive(NotificationCenter.default.publisher(for: AgentKeychain.backendChanged)) { _ in
			keychainRecoveryEpoch += 1
		}
	}

	private func refreshUpdateCheck() {
		let enabled = store.managedPrefs.checkUpdates ?? (store.features.checkUpdates != false)
		updateCheck.refresh(enabled: enabled, current: AgentBranding.revision)
	}

	private func beginReview() {
		let snapshot = store.configDiagnostics
		guard snapshot.needsLineReview else { return }
		if state.unlocked {
			reviewSnapshot = snapshot
			return
		}
		AgentLockUI.unlock(gate: gate) { outcome in
			if outcome == .unlocked {
				reviewSnapshot = snapshot
			}
		}
	}

	private var statusScroll: some View {
		ScrollView {
			VStack(alignment: .leading, spacing: 10) {
			Label(
				state.unlocked ? "Unlocked - minting allowed" : "Locked - config only",
				systemImage: state.unlocked ? "lock.open.fill" : "lock.fill"
			)
			.font(.headline)

			VStack(alignment: .leading, spacing: 6) {
				infoRow("Version", "\(AgentBranding.revision) (\(AgentBranding.buildTime))")
				if let url = updateCheck.updateURL, let ver = updateCheck.updateVersion {
					Link("Update \(ver)", destination: url)
				}
				infoRow("Socket", socketPath)
				infoRow("Workplaces", "\(store.workplaces.count)")
				infoRow("Auths", "\(store.authCount)")
				if !store.managedPrefs.isEmpty {
					infoRow("Managed prefs", store.managedPrefs.summaryLine())
				}
				infoRow("Peer check", PeerCheck.allowUnsigned ? "unsigned allowed (dev)" : "codesign")
				infoRow("Secrets", keychainText)
				infoRow("Presence", presenceText)
			}
			.font(.callout)

			HStack(alignment: .top, spacing: 12) {
				idleLockSettings
					.frame(maxWidth: .infinity, alignment: .leading)
				unlockTimeoutSettings
					.frame(maxWidth: .infinity, alignment: .leading)
			}

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
			ConfigIntegrityLastGoodSection(
				store: store,
				state: state,
				diagnostics: store.configDiagnostics,
				onReview: beginReview
			)

			if let error = store.lastError {
				Label(error, systemImage: "xmark.octagon").foregroundStyle(.red)
			}
			}
			.frame(minWidth: 0, maxWidth: .infinity, alignment: .leading)
		}
		.onAppear {
			applyEmptyConfigIdleLock()
		}
		.onChange(of: store.isEmpty) { empty in
			if empty { applyEmptyConfigIdleLock() }
		}
	}

	private var keychainText: String {
		_ = keychainRecoveryEpoch
		if AgentKeychainRecovery.needsManualQuit {
			return "data protection keychain unavailable — quit Buddy Agent and open it again"
		}
		if AgentKeychain.isDeferred {
			return "data protection keychain, recovering"
		}
		switch AgentKeychain.backend {
		case .dataProtection: return "data protection keychain, presence ACL"
		case .memory:
			if AgentKeychain.keychainAccessGroup == nil {
				return "memory only (unsigned — run signed BuddyAgent.app)"
			}
			return "memory only (ACL probe failed)"
		}
	}

	private var presenceText: String {
		switch UnlockGate.mode {
		case .acl: return "Keychain userPresence"
		case .simulated:
			if UnlockGate.promptingDisabled { return "LocalAuthentication (prompting off)" }
			if AgentKeychain.keychainAccessGroup == nil {
				return "LocalAuthentication (unsigned — secrets are memory-only)"
			}
			return "LocalAuthentication (ACL unavailable — secrets are memory-only)"
		case .unavailable: return "unavailable - unlock from the menu bar"
		}
	}

	private var idleLockDisabled: Bool { idleLockSeconds <= 0 }

	private var autoLockEnabled: Binding<Bool> {
		Binding(
			get: { idleLockSeconds > 0 },
			set: { enabled in
				if enabled {
					idleLockSeconds =
						lastIdleLockSeconds > 0 ? lastIdleLockSeconds : AgentIdleLock.defaultSeconds
				} else {
					if idleLockSeconds > 0 {
						lastIdleLockSeconds = idleLockSeconds
					}
					idleLockSeconds = 0
				}
				state.refreshIdleLockTimer()
			})
	}

	private var idleLockInterval: Binding<Double> {
		Binding(
			get: { idleLockSeconds > 0 ? idleLockSeconds : lastIdleLockSeconds },
			set: { newValue in
				lastIdleLockSeconds = newValue
				if idleLockSeconds > 0 {
					idleLockSeconds = newValue
				}
			})
	}

	private var idleLockIntervalLabel: String {
		let current = idleLockSeconds > 0 ? idleLockSeconds : lastIdleLockSeconds
		return idleLockChoices.first { $0.seconds == current }?.label ?? "15 minutes"
	}

	private var idleLockSettings: some View {
		VStack(alignment: .leading, spacing: 8) {
			Toggle("Auto-lock when idle", isOn: autoLockEnabled)
				.disabled(AgentIdleLock.envOverrideActive)

			if idleLockDisabled {
				Label(
					"Auto-lock is off — minting stays unlocked until you lock from the menu or quit.",
					systemImage: "exclamationmark.triangle.fill"
				)
				.font(.callout)
				.foregroundStyle(.primary)
			} else {
				LabeledTrailingRow(label: "Interval") {
					Menu {
						ForEach(idleLockChoices, id: \.seconds) { choice in
							Button {
								idleLockInterval.wrappedValue = choice.seconds
							} label: {
								if idleLockInterval.wrappedValue == choice.seconds {
									Label(choice.label, systemImage: "checkmark")
								} else {
									Text(choice.label)
								}
							}
						}
					} label: {
						Text(idleLockIntervalLabel)
					}
					.menuStyle(.borderlessButton)
				}
				.disabled(AgentIdleLock.envOverrideActive)

				if state.unlocked, let until = state.unlockedUntil {
					let remaining = max(0, Int(until.timeIntervalSinceNow))
					Text("Relocks in \(remaining)s if idle")
						.font(.caption)
						.foregroundStyle(.secondary)
				}
			}

			if AgentIdleLock.envOverrideActive {
				Text("BUDDY_AGENT_IDLE_LOCK overrides this setting.")
					.font(.caption)
					.foregroundStyle(.secondary)
			}
		}
		.padding(10)
		.background(idleLockBackground, in: RoundedRectangle(cornerRadius: 8))
		.overlay(
			RoundedRectangle(cornerRadius: 8)
				.strokeBorder(idleLockBorder, lineWidth: 1))
		.onChange(of: idleLockSeconds) { _ in
			state.refreshIdleLockTimer()
		}
	}

	private var idleLockBackground: Color {
		if idleLockDisabled {
			return Color(red: 0.42, green: 0.28, blue: 0.08).opacity(0.22)
		}
		return Color.primary.opacity(0.04)
	}

	private var idleLockBorder: Color {
		idleLockDisabled ? Color.brown.opacity(0.45) : Color.primary.opacity(0.08)
	}

	private let unlockTimeoutChoices: [(label: String, seconds: TimeInterval)] = [
		("30 seconds", 30),
		("1 minute", 60),
		("2 minutes", 2 * 60),
		("5 minutes", 5 * 60),
	]

	private var unlockPromptDisabled: Bool { unlockTimeoutSeconds <= 0 }

	private var unlockPromptEnabled: Binding<Bool> {
		Binding(
			get: { unlockTimeoutSeconds > 0 },
			set: { enabled in
				if enabled {
					unlockTimeoutSeconds =
						lastUnlockTimeoutSeconds > 0
						? lastUnlockTimeoutSeconds : AgentUnlockTimeout.defaultSeconds
				} else {
					if unlockTimeoutSeconds > 0 {
						lastUnlockTimeoutSeconds = unlockTimeoutSeconds
					}
					unlockTimeoutSeconds = 0
				}
			})
	}

	private var unlockTimeoutInterval: Binding<Double> {
		Binding(
			get: { unlockTimeoutSeconds > 0 ? unlockTimeoutSeconds : lastUnlockTimeoutSeconds },
			set: { newValue in
				lastUnlockTimeoutSeconds = newValue
				if unlockTimeoutSeconds > 0 {
					unlockTimeoutSeconds = newValue
				}
			})
	}

	private var unlockTimeoutLabel: String {
		let current = unlockTimeoutSeconds > 0 ? unlockTimeoutSeconds : lastUnlockTimeoutSeconds
		return unlockTimeoutChoices.first { $0.seconds == current }?.label ?? "1 minute"
	}

	private var unlockTimeoutSettings: some View {
		VStack(alignment: .leading, spacing: 8) {
			Toggle("Allow unlock prompts from buddy", isOn: unlockPromptEnabled)
				.disabled(AgentUnlockTimeout.envOverrideActive)

			if unlockPromptDisabled {
				Label(
					"Buddy CLI cannot trigger unlock — use Lock/Unlock in the menu bar.",
					systemImage: "exclamationmark.triangle.fill"
				)
				.font(.callout)
				.foregroundStyle(.primary)
			} else {
				LabeledTrailingRow(label: "Prompt timeout") {
					Menu {
						ForEach(unlockTimeoutChoices, id: \.seconds) { choice in
							Button {
								unlockTimeoutInterval.wrappedValue = choice.seconds
							} label: {
								if unlockTimeoutInterval.wrappedValue == choice.seconds {
									Label(choice.label, systemImage: "checkmark")
								} else {
									Text(choice.label)
								}
							}
						}
					} label: {
						Text(unlockTimeoutLabel)
					}
					.menuStyle(.borderlessButton)
				}
				.disabled(AgentUnlockTimeout.envOverrideActive)
			}

			if AgentUnlockTimeout.envOverrideActive {
				Text("BUDDY_AGENT_UNLOCK_TIMEOUT overrides this setting.")
					.font(.caption)
					.foregroundStyle(.secondary)
			}
		}
		.padding(10)
		.background(
			unlockPromptDisabled
				? Color(red: 0.42, green: 0.28, blue: 0.08).opacity(0.22)
				: Color.primary.opacity(0.04),
			in: RoundedRectangle(cornerRadius: 8))
		.overlay(
			RoundedRectangle(cornerRadius: 8)
				.strokeBorder(
					unlockPromptDisabled ? Color.brown.opacity(0.45) : Color.primary.opacity(0.08),
					lineWidth: 1))
	}

	private func infoRow(_ label: String, _ value: String) -> some View {
		HStack(alignment: .firstTextBaseline, spacing: 12) {
			Text(label)
				.foregroundStyle(.secondary)
				.frame(width: 120, alignment: .leading)
			Text(value)
				.textSelection(.enabled)
				.agentFlexibleWidth()
		}
	}

	private func applyEmptyConfigIdleLock() {
		guard store.isEmpty, !AgentIdleLock.envOverrideActive else { return }
		AgentIdleLock.applyEmptyConfigDefault()
		if idleLockSeconds <= 0 {
			idleLockSeconds = AgentIdleLock.defaultSeconds
		}
		if lastIdleLockSeconds <= 0 {
			lastIdleLockSeconds = AgentIdleLock.defaultSeconds
		}
		state.refreshIdleLockTimer()
	}
}
