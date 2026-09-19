import SwiftUI

/// Policy tab: buddy `features:` and `channels:` (separate from workplaces/auths).
struct PolicyView: View {
	@ObservedObject var store: AgentStore
	@State private var draftFeatures = AgentFeatures()
	@State private var draftChannels: [String: ChannelPolicy] = [:]

	private var isDirty: Bool {
		if !store.managedPolicyLocked("features"), draftFeatures != store.features {
			return true
		}
		for channel in BuddyChannel.allCases {
			guard !store.isManagedChannel(channel) else { continue }
			let draft = draftChannels[channel.rawValue] ?? ChannelPolicy()
			if draft != store.channelPolicy(for: channel) { return true }
		}
		return false
	}

	var body: some View {
		ScrollView(.vertical, showsIndicators: true) {
			VStack(alignment: .leading, spacing: 20) {
				HStack(alignment: .firstTextBaseline) {
					VStack(alignment: .leading, spacing: 4) {
						Text("Policy").font(.title3)
						Text("Changes are written to disk when you save.")
							.font(.caption)
							.foregroundStyle(.secondary)
					}
					Spacer()
					if isDirty {
						Text("Unsaved changes")
							.font(.callout)
							.foregroundStyle(.orange)
					}
				}
				if let err = store.lastError, err.contains("could not read") {
					Text(err)
						.font(.callout)
						.foregroundStyle(.red)
				}
				if !store.managedPrefs.isEmpty {
					Label(
						"MDM locks: \(store.managedPrefs.summaryLine()). Buddy merges these over the Agent document at runtime.",
						systemImage: "lock.fill"
					)
					.font(.callout)
					.foregroundStyle(.secondary)
				}
				ConfigPolicySection(
					store: store,
					features: $draftFeatures,
					channels: $draftChannels)
				policyActions
			}
			.frame(maxWidth: .infinity, alignment: .leading)
			.padding(.vertical, 4)
		}
		.onAppear {
			store.reloadManagedPrefs()
			loadDraftFromStore()
		}
	}

	private var policyActions: some View {
		HStack {
			Spacer()
			Button("Revert") { loadDraftFromStore() }
				.disabled(!isDirty)
			Button("Save") { saveDraft() }
				.keyboardShortcut(.defaultAction)
				.disabled(!isDirty)
		}
	}

	private func loadDraftFromStore() {
		draftFeatures = store.features
		draftChannels = [:]
		for channel in BuddyChannel.allCases {
			let policy = store.channelPolicy(for: channel)
			if !policy.isEmpty {
				draftChannels[channel.rawValue] = policy
			}
		}
	}

	private func saveDraft() {
		store.savePolicy(features: draftFeatures, channels: draftChannels)
		loadDraftFromStore()
	}
}

/// Editors for buddy `features:` and `channels:` policy.
struct ConfigPolicySection: View {
	@ObservedObject var store: AgentStore
	@Binding var features: AgentFeatures
	@Binding var channels: [String: ChannelPolicy]

	var body: some View {
		VStack(alignment: .leading, spacing: 20) {
			featuresSection
			channelsSection
		}
	}

	private var featuresSection: some View {
		VStack(alignment: .leading, spacing: 8) {
			Text("Features").font(.headline)
			Text("Buddy reads these from the Agent document instead of config.yaml when `use_agent` is on.")
				.font(.caption)
				.foregroundStyle(.secondary)
			Toggle(
				"Serve config to buddy (`features.use_agent`)",
				isOn: Binding(
					get: { features.useAgent },
					set: { features.useAgent = $0 }))
			.disabled(store.managedPolicyLocked("features"))
			Toggle(
				"Classic instance only (`features.classic_instance_only`)",
				isOn: Binding(
					get: { features.classicInstanceOnly },
					set: { features.classicInstanceOnly = $0 }))
			.disabled(store.managedPolicyLocked("features"))
			Text(
				"When classic instance only is on, Classic API calls stay on Jamf Pro even when a platform auth exists."
			)
			.font(.caption2)
			.foregroundStyle(.tertiary)
		}
	}

	private var channelsSection: some View {
		VStack(alignment: .leading, spacing: 8) {
			Text("Channel defaults").font(.headline)
			Text(
				"Per-client overrides for CLI, MCP, and IDE (`channels.cli|mcp|ide`). Buddy applies MCP’s default `jamf_*` deny at runtime unless you override with allow/deny lists here."
			)
			.font(.caption)
			.foregroundStyle(.secondary)
			ForEach(BuddyChannel.allCases) { channel in
				ChannelPolicyDisclosure(
					store: store,
					channel: channel,
					policy: channelBinding(channel))
			}
		}
	}

	private func channelBinding(_ channel: BuddyChannel) -> Binding<ChannelPolicy> {
		Binding(
			get: { channels[channel.rawValue] ?? ChannelPolicy() },
			set: { newValue in
				let key = channel.rawValue
				if newValue.isEmpty {
					channels.removeValue(forKey: key)
				} else {
					channels[key] = newValue
				}
			})
	}
}

private struct ChannelPolicyDisclosure: View {
	@ObservedObject var store: AgentStore
	let channel: BuddyChannel
	@Binding var policy: ChannelPolicy

	private var managed: Bool {
		store.isManagedChannel(channel)
	}

	var body: some View {
		DisclosureGroup {
			VStack(alignment: .leading, spacing: 10) {
				PolicyAccessMenu(
					label: "Access",
					access: binding(\.access))
				PolicyContextMenu(
					label: "Default workplace",
					store: store,
					context: binding(\.context))
				PolicyContextAllowlist(
					label: "Allowed workplaces",
					store: store,
					contexts: binding(\.contexts))
				BuiltinPatternsField(
					title: "Deny builtins",
					help: "One pattern per line — exact name, trailing-* glob, or \"*\".",
					patterns: binding(\.denyBuiltins))
				BuiltinPatternsField(
					title: "Allow builtins",
					help: "Overrides deny at this layer; \"*\" allows all.",
					patterns: binding(\.allowBuiltins))
				if channel == .mcp && policy.isEmpty {
					Text("With no MCP policy set, buddy still denies `jamf_*` builtins by default.")
						.font(.caption2)
						.foregroundStyle(.tertiary)
				}
			}
			.padding(.vertical, 4)
			.disabled(managed)
		} label: {
			HStack {
				Text(channel.label)
				if managed {
					Image(systemName: "lock.fill")
						.font(.caption)
						.foregroundStyle(.secondary)
				}
				Spacer()
				if !policy.isEmpty {
					Text(policySummary)
						.font(.caption)
						.foregroundStyle(.secondary)
				}
			}
		}
	}

	private var policySummary: String {
		var parts: [String] = []
		if policy.access.trimmed == "readonly" { parts.append("readonly") }
		if !policy.context.trimmed.isEmpty { parts.append("→ \(policy.context.trimmed)") }
		if !policy.contexts.isEmpty { parts.append("\(policy.contexts.count) allowed") }
		if !policy.denyBuiltins.isEmpty { parts.append("deny \(policy.denyBuiltins.count)") }
		if !policy.allowBuiltins.isEmpty { parts.append("allow \(policy.allowBuiltins.count)") }
		return parts.joined(separator: " · ")
	}

	private func binding<T>(_ keyPath: WritableKeyPath<ChannelPolicy, T>) -> Binding<T> {
		Binding(
			get: { policy[keyPath: keyPath] },
			set: { policy[keyPath: keyPath] = $0 })
	}
}

private struct PolicyAccessMenu: View {
	let label: String
	@Binding var access: String

	var body: some View {
		HStack {
			Text(label)
			Spacer()
			Menu {
				Button {
					access = ""
				} label: {
					if access.trimmed.isEmpty {
						Label("full", systemImage: "checkmark")
					} else {
						Text("full")
					}
				}
				Button {
					access = "readonly"
				} label: {
					if access.trimmed == "readonly" {
						Label("readonly", systemImage: "checkmark")
					} else {
						Text("readonly")
					}
				}
			} label: {
				Text(access.trimmed.isEmpty ? "full" : access.trimmed)
			}
			.menuStyle(.borderlessButton)
			.fixedSize()
		}
	}
}

private struct PolicyContextMenu: View {
	let label: String
	@ObservedObject var store: AgentStore
	@Binding var context: String

	var body: some View {
		HStack {
			Text(label)
			Spacer()
			Menu {
				Button {
					context = ""
				} label: {
					if context.trimmed.isEmpty {
						Label("(none)", systemImage: "checkmark")
					} else {
						Text("(none)")
					}
				}
				ForEach(store.sortedWorkplaceNames(), id: \.self) { name in
					Button {
						context = name
					} label: {
						if context == name {
							Label(name, systemImage: "checkmark")
						} else {
							Text(name)
						}
					}
				}
			} label: {
				Text(context.trimmed.isEmpty ? "(none)" : context.trimmed)
			}
			.menuStyle(.borderlessButton)
			.fixedSize()
		}
	}
}

private struct PolicyContextAllowlist: View {
	let label: String
	@ObservedObject var store: AgentStore
	@Binding var contexts: [String]

	var body: some View {
		VStack(alignment: .leading, spacing: 4) {
			Text(label).font(.callout)
			Text("Empty = any configured workplace.")
				.font(.caption2)
				.foregroundStyle(.tertiary)
			if store.workplaces.isEmpty {
				Text("Add workplaces first").foregroundStyle(.secondary)
			} else {
				ForEach(store.sortedWorkplaceNames(), id: \.self) { name in
					Toggle(
						name,
						isOn: Binding(
							get: { contexts.contains(name) },
							set: { on in
								if on {
									if !contexts.contains(name) { contexts.append(name) }
								} else {
									contexts.removeAll { $0 == name }
								}
								contexts = AgentConfigLists.normalized(contexts)
							}))
				}
			}
		}
	}
}

struct BuiltinPatternsField: View {
	let title: String
	let help: String
	@Binding var patterns: [String]

	private var linesBinding: Binding<String> {
		Binding(
			get: { AgentConfigLists.linesText(patterns) },
			set: { patterns = AgentConfigLists.patterns(fromLines: $0) })
	}

	var body: some View {
		VStack(alignment: .leading, spacing: 4) {
			Text(title).font(.callout)
			Text(help).font(.caption2).foregroundStyle(.tertiary)
			TextEditor(text: linesBinding)
				.font(.system(.caption, design: .monospaced))
				.frame(minHeight: 52, maxHeight: 88)
		}
	}
}

struct AuthChannelsField: View {
	@Binding var channels: [String]

	var body: some View {
		VStack(alignment: .leading, spacing: 4) {
			Text("Buddy clients").font(.callout)
			Text("Empty = all channels (cli, mcp, ide).")
				.font(.caption2)
				.foregroundStyle(.tertiary)
			HStack {
				ForEach(BuddyChannel.allCases) { channel in
					Toggle(
						channel.label,
						isOn: Binding(
							get: { channels.isEmpty || channels.contains(channel.rawValue) },
							set: { on in
								var next = channels.isEmpty
									? BuddyChannel.allCases.map(\.rawValue) : channels
								if on {
									if !next.contains(channel.rawValue) {
										next.append(channel.rawValue)
									}
								} else {
									next.removeAll { $0 == channel.rawValue }
								}
								next = AgentConfigLists.normalized(next)
								if next.count == BuddyChannel.allCases.count {
									channels = []
								} else {
									channels = next
								}
							}))
				}
			}
		}
	}
}
