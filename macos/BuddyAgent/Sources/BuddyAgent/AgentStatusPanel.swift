import AppKit
import LocalAuthentication
import LocalAuthenticationEmbeddedUI
import SwiftUI

private let agentStatusPanelWidth: CGFloat = 340
private let agentStatusPanelCornerRadius: CGFloat = 28
private let agentStatusPanelMaxHeight: CGFloat = 480
private let agentStatusListMaxHeight: CGFloat = 360

/// Borderless status panel. It may become key for the search field, and it never
/// becomes main. Show orders it front without `makeKey` — AppKit's menu glass
/// window warns when something calls `makeKey` on a window that refuses key.
private final class AgentStatusPanel: NSPanel {
	override var canBecomeKey: Bool { true }
	override var canBecomeMain: Bool { false }
}

/// Status menu as a non-activating panel. Liquid glass on macOS 26+; the same
/// layout with `regularMaterial` on earlier systems.
final class AgentStatusPanelController: NSObject {
	private var panel: AgentStatusPanel?
	private var hosting: NSHostingController<AgentStatusMenuContent>?
	private var anchor: NSStatusBarButton?
	private var outsideClickMonitor: Any?
	private var localClickMonitor: Any?
	/// Set when the click that closed the panel was on the status item, so its
	/// mouse-up action does not open the panel again.
	private var suppressNextOpen = false
	private var measureScheduled = false
	private weak var delegate: AgentAppDelegate?
	/// Bumped when the panel opens or closes so an in-flight Touch ID cannot restart it.
	fileprivate var touchGeneration = 0
	fileprivate var cancelEmbedded: (() -> Void)?

	func toggle(
		relativeTo button: NSStatusBarButton,
		store: AgentStore,
		state: AgentState,
		gate: UnlockGate,
		delegate: AgentAppDelegate
	) {
		self.delegate = delegate
		self.anchor = button
		if suppressNextOpen {
			suppressNextOpen = false
			if panel?.isVisible == true {
				dismiss()
			}
			return
		}
		if panel?.isVisible == true {
			dismiss()
			return
		}
		show(relativeTo: button, store: store, state: state, gate: gate)
	}

	func dismiss() {
		guard let panel, panel.isVisible else { return }
		touchGeneration += 1
		cancelEmbedded?()
		cancelEmbedded = nil
		removeOutsideClickMonitor()
		panel.orderOut(nil)
		delegate?.statusPanelDidClose()
	}

	fileprivate func scheduleMeasure() {
		guard !measureScheduled else { return }
		measureScheduled = true
		DispatchQueue.main.async { [weak self] in
			guard let self else { return }
			self.measureScheduled = false
			self.applyFittedSize()
		}
	}

	fileprivate func openSettings() {
		delegate?.openSettings()
	}

	fileprivate func openAuth(_ name: String) {
		delegate?.openAuthFromPanel(name)
	}

	fileprivate func showBearer(_ name: String) {
		delegate?.openBearerFromPanel(name)
	}

	fileprivate func openBuddy() {
		delegate?.openBuddyApp()
	}

	fileprivate func toggleLogin() {
		delegate?.toggleOpenAtLoginFromPanel()
	}

	fileprivate func showHelp() {
		delegate?.showHelp()
	}

	fileprivate func showAbout() {
		delegate?.showAbout()
	}

	fileprivate func quit() {
		delegate?.quit()
	}

	fileprivate var isPanelVisible: Bool { panel?.isVisible == true }

	/// The embedded Touch ID view only receives the sensor while this panel is key.
	fileprivate func focusForTouchID() {
		guard let panel, panel.isVisible else { return }
		panel.makeKey()
	}

	private func show(
		relativeTo button: NSStatusBarButton,
		store: AgentStore,
		state: AgentState,
		gate: UnlockGate
	) {
		touchGeneration += 1
		let content = AgentStatusMenuContent(
			store: store, state: state, gate: gate, session: UUID(), controller: self)
		let panel = ensurePanel()
		let hosting = install(content, in: panel)
		let size = fittedSize(of: hosting)
		// Size before the panel is ordered front (avoids layout recursion on show).
		panel.setContentSize(size)
		hosting.view.frame = NSRect(origin: .zero, size: size)
		position(panel, below: button, size: size)
		delegate?.statusPanelWillOpen()
		panel.orderFront(nil)
		installOutsideClickMonitor(for: panel)
	}

	private func ensurePanel() -> AgentStatusPanel {
		if let panel { return panel }
		let panel = AgentStatusPanel(
			contentRect: NSRect(x: 0, y: 0, width: agentStatusPanelWidth, height: 160),
			styleMask: [.borderless, .nonactivatingPanel],
			backing: .buffered,
			defer: false)
		panel.isFloatingPanel = true
		panel.level = .popUpMenu
		panel.hasShadow = true
		panel.isOpaque = false
		panel.backgroundColor = .clear
		panel.becomesKeyOnlyIfNeeded = true
		panel.hidesOnDeactivate = false
		panel.isMovable = false
		self.panel = panel
		return panel
	}

	private func install(
		_ content: AgentStatusMenuContent, in panel: AgentStatusPanel
	) -> NSHostingController<AgentStatusMenuContent> {
		if let hosting {
			hosting.rootView = content
			return hosting
		}
		let hosting = NSHostingController(rootView: content)
		let view = hosting.view
		view.wantsLayer = true
		view.layer?.isOpaque = false
		view.layer?.backgroundColor = NSColor.clear.cgColor
		view.autoresizingMask = [.width, .height]
		if #available(macOS 26.0, *) {
			let glass = NSGlassEffectView()
			glass.cornerRadius = agentStatusPanelCornerRadius
			glass.style = .regular
			if #available(macOS 27.0, *) {
				glass.effectIsInteractive = true
			}
			glass.contentView = view
			panel.contentView = glass
		} else {
			panel.contentViewController = hosting
		}
		self.hosting = hosting
		return hosting
	}

	private func fittedSize(of hosting: NSHostingController<AgentStatusMenuContent>) -> NSSize {
		let fit = hosting.sizeThatFits(in: NSSize(width: agentStatusPanelWidth, height: 800))
		let height = min(agentStatusPanelMaxHeight, max(140, ceil(fit.height)))
		return NSSize(width: agentStatusPanelWidth, height: height)
	}

	private func applyFittedSize() {
		guard let panel, panel.isVisible, let hosting, let anchor else { return }
		let size = fittedSize(of: hosting)
		if abs(panel.frame.width - size.width) < 2, abs(panel.frame.height - size.height) < 2 {
			return
		}
		panel.setContentSize(size)
		position(panel, below: anchor, size: size)
	}

	private func position(_ panel: NSPanel, below button: NSStatusBarButton, size: NSSize) {
		guard let window = button.window else { return }
		let buttonOnScreen = window.convertToScreen(button.convert(button.bounds, to: nil))
		var origin = NSPoint(
			x: buttonOnScreen.midX - size.width / 2,
			y: buttonOnScreen.minY - size.height - 4)
		if let screen = window.screen ?? NSScreen.main {
			let visible = screen.visibleFrame
			if origin.x < visible.minX { origin.x = visible.minX + 4 }
			if origin.x + size.width > visible.maxX { origin.x = visible.maxX - size.width - 4 }
			if origin.y < visible.minY { origin.y = visible.minY + 4 }
		}
		panel.setFrameOrigin(origin)
	}

	private func installOutsideClickMonitor(for panel: NSPanel) {
		removeOutsideClickMonitor()
		outsideClickMonitor = NSEvent.addGlobalMonitorForEvents(matching: [.leftMouseDown, .rightMouseDown]) {
			[weak self] _ in
			guard let self else { return }
			self.closeIfClickOutsidePanel()
		}
		// Clicks on the status item belong to this app, so a global monitor never sees them.
		localClickMonitor = NSEvent.addLocalMonitorForEvents(matching: [.leftMouseDown, .rightMouseDown]) {
			[weak self] event in
			guard let self else { return event }
			if self.closeIfClickOutsidePanel() {
				return event
			}
			return event
		}
	}

	/// Closes the panel when `click` is outside it. Returns whether it closed.
	@discardableResult
	private func closeIfClickOutsidePanel() -> Bool {
		guard let panel, panel.isVisible else { return false }
		let click = NSEvent.mouseLocation
		guard !panel.frame.contains(click) else { return false }
		if statusButtonScreenRect()?.contains(click) == true {
			suppressNextOpen = true
			DispatchQueue.main.asyncAfter(deadline: .now() + 0.4) { [weak self] in
				self?.suppressNextOpen = false
			}
		}
		DispatchQueue.main.async { [weak self] in self?.dismiss() }
		return true
	}

	private func statusButtonScreenRect() -> NSRect? {
		guard let anchor, let window = anchor.window else { return nil }
		return window.convertToScreen(anchor.convert(anchor.bounds, to: nil))
	}

	private func removeOutsideClickMonitor() {
		if let outsideClickMonitor {
			NSEvent.removeMonitor(outsideClickMonitor)
			self.outsideClickMonitor = nil
		}
		if let localClickMonitor {
			NSEvent.removeMonitor(localClickMonitor)
			self.localClickMonitor = nil
		}
	}
}

private struct AgentStatusMenuContent: View {
	@ObservedObject var store: AgentStore
	@ObservedObject var state: AgentState
	let gate: UnlockGate
	let session: UUID
	let controller: AgentStatusPanelController

	var body: some View {
		AgentStatusMenuSession(store: store, state: state, gate: gate, controller: controller)
			.id(session)
	}
}

private struct AgentStatusMenuSession: View {
	@ObservedObject var store: AgentStore
	@ObservedObject var state: AgentState
	@ObservedObject private var updates = AgentUpdateCheck.shared
	let gate: UnlockGate
	let controller: AgentStatusPanelController

	@State private var query = ""
	@State private var passwordPrompt = false
	@State private var touchContext: LAContext?
	@State private var listening = false
	@State private var fastFailures = 0
	@FocusState private var searchFocused: Bool

	var body: some View {
		ZStack(alignment: .topTrailing) {
			Group {
				if state.unlocked {
					VStack(alignment: .leading, spacing: 0) {
						topBar
						unlockedBody
					}
				} else {
					lockedBody
				}
			}
			.frame(width: agentStatusPanelWidth, alignment: .top)
			.fixedSize(horizontal: true, vertical: true)
			.modifier(AgentStatusPanelChrome())
			if !state.unlocked {
				gearMenu
					.padding(.top, 10)
					.padding(.trailing, 14)
			}
		}
		.onExitCommand { controller.dismiss() }
		.onAppear {
			controller.scheduleMeasure()
			if !state.unlocked { beginLockedUnlock() }
		}
		.onChange(of: query) { _ in controller.scheduleMeasure() }
		.onChange(of: state.unlocked) { unlocked in
			controller.scheduleMeasure()
			if unlocked {
				listening = false
				passwordPrompt = false
				touchContext = nil
				controller.cancelEmbedded = nil
			} else {
				controller.cancelEmbedded?()
				controller.cancelEmbedded = nil
				listening = false
				passwordPrompt = false
				touchContext = nil
				fastFailures = 0
				controller.focusForTouchID()
				beginLockedUnlock()
			}
		}
		.onChange(of: store.authCount) { _ in controller.scheduleMeasure() }
	}

	private var topBar: some View {
		HStack(spacing: 8) {
			Text("Buddy")
				.font(.system(size: 17, weight: .semibold))
			Spacer(minLength: 0)
			Button {
				state.lockVault()
			} label: {
				Image(systemName: "lock.fill")
					.font(.system(size: 15, weight: .medium))
					.frame(width: 28, height: 28)
			}
			.buttonStyle(.plain)
			.foregroundStyle(.primary)
			.accessibilityLabel("Lock")
			.help("Lock Buddy Agent")
			Button {
				controller.openSettings()
			} label: {
				Image(systemName: windowSymbol)
					.font(.system(size: 16, weight: .medium))
					.frame(width: 28, height: 28)
			}
			.buttonStyle(.plain)
			.foregroundStyle(.primary)
			.accessibilityLabel("Settings")
			.help("Open Settings")
		}
		.padding(.horizontal, 16)
		.padding(.top, 12)
		.padding(.bottom, 8)
	}

	private var unlockedBody: some View {
		VStack(alignment: .leading, spacing: 0) {
			if !store.sortedAuthNames.isEmpty {
				searchField
			}
			if store.sortedAuthNames.isEmpty {
				emptyLine("No auths")
			} else if displayed.isEmpty {
				emptyLine("No matching auths")
			} else {
				ScrollView {
					VStack(spacing: 0) {
						ForEach(displayed, id: \.self) { name in
							AgentStatusAuthRow(
								store: store,
								state: state,
								gate: gate,
								name: name,
								auth: store.auths[name] ?? AgentAuth(),
								hasSecret: store.hasSecret(forAuth: name),
								onOpen: { controller.openAuth(name) },
								onShowBearer: { controller.showBearer(name) })
							if name != displayed.last {
								Divider().padding(.leading, 60)
							}
						}
					}
				}
				.frame(height: listHeight)
			}
		}
		.padding(.bottom, 8)
	}

	private var lockedBody: some View {
		VStack(spacing: 18) {
			lockIcon
			VStack(spacing: 6) {
				Text("Buddy Agent is Locked")
					.font(.system(size: 22, weight: .bold))
					.multilineTextAlignment(.center)
				Text(lockSubtitle)
					.font(.system(size: 15))
					.foregroundStyle(.secondary)
					.multilineTextAlignment(.center)
					.frame(maxWidth: 260)
					.fixedSize(horizontal: false, vertical: true)
			}
			unlockCapsule
				.padding(.top, 6)
		}
		.frame(maxWidth: .infinity)
		.padding(.horizontal, 28)
		.padding(.top, 64)
		.padding(.bottom, 48)
	}

	private var unlockCapsule: some View {
		Button(action: usePassword) {
			ZStack {
				Text(embeddedTouchAvailable ? "Use Password..." : "Unlock")
					.font(.system(size: 15))
					.opacity(passwordPrompt ? 0 : 1)
				if passwordPrompt {
					ProgressView().controlSize(.small)
				}
			}
			.frame(maxWidth: .infinity)
			.frame(height: 34)
			.contentShape(Capsule())
		}
		.buttonStyle(.plain)
		.foregroundStyle(.primary)
		.background(Color.primary.opacity(0.14), in: Capsule())
		.disabled(unlockBlocked || passwordPrompt)
		.opacity(unlockBlocked ? 0.4 : 1)
		.padding(.horizontal, 24)
	}

	private var lockIcon: some View {
		ZStack(alignment: .bottomTrailing) {
			Group {
				if let icon = AgentBranding.menuIcon(size: Self.iconSide) {
					Image(nsImage: icon)
						.resizable()
						.interpolation(.high)
				} else {
					Image(systemName: "lock.fill")
						.font(.system(size: 32))
				}
			}
			.frame(width: Self.iconSide, height: Self.iconSide)
			.clipShape(RoundedRectangle(cornerRadius: 15, style: .continuous))
			touchBadge
				.offset(x: Self.touchBadgeShift.width, y: Self.touchBadgeShift.height)
		}
		.padding(.trailing, Self.touchBadgeShift.width + 4)
		.padding(.bottom, Self.touchBadgeShift.height + 4)
		.accessibilityHidden(true)
	}

	@ViewBuilder
	private var touchBadge: some View {
		ZStack {
			if let touchContext {
				EmbeddedTouchID(context: touchContext) {
					controller.focusForTouchID()
					evaluateEmbedded(touchContext)
				}
				.id(ObjectIdentifier(touchContext))
			}
			Circle().fill(.white)
			Image(systemName: "touchid")
				.font(.system(size: Self.touchBadgeSide * 0.76, weight: .black))
				.foregroundStyle(Self.touchRed)
		}
		.frame(width: Self.touchBadgeSide, height: Self.touchBadgeSide)
		.clipShape(Circle())
		.shadow(color: .black.opacity(0.22), radius: 1.5, y: 0.5)
	}

	/// Square that tightly bounds the app squircle.
	private static let iconSide: CGFloat = 68
	/// Disc is about 70% of that square, matching the Passwords badge. The shift
	/// keeps its center just inside the squircle corner so the rounded corner
	/// does not leave the disc floating below the icon.
	private static let touchBadgeSide: CGFloat = 48
	private static let touchBadgeShift = CGSize(width: 18, height: 13)
	/// Passwords Touch ID red, sampled from the locked menu badge.
	private static let touchRed = Color(red: 0.922, green: 0.267, blue: 0.353)

	private var searchField: some View {
		let shape = RoundedRectangle(cornerRadius: 10, style: .continuous)
		return HStack(spacing: 6) {
			Image(systemName: "magnifyingglass")
				.font(.system(size: 14))
				.foregroundStyle(.secondary)
			TextField("Search", text: $query)
				.textFieldStyle(.plain)
				.focused($searchFocused)
		}
		.padding(.horizontal, 10)
		.padding(.vertical, 7)
		.background(Color.primary.opacity(0.08), in: shape)
		.overlay(
			shape.strokeBorder(
				searchFocused ? Color.accentColor : Color.primary.opacity(0.16),
				lineWidth: searchFocused ? 2.5 : 1)
		)
		.padding(.horizontal, 12)
		.padding(.bottom, 8)
		.accessibilityLabel("Search auths")
	}

	private var gearMenu: some View {
		Menu {
			if let version = updates.updateVersion, updates.updateURL != nil {
				Button("Update Available: \(version)") {
					if let url = updates.updateURL {
						NSWorkspace.shared.open(url)
					}
				}
				Divider()
			}
			if state.unlocked {
				Button("Lock") { state.lockVault() }
			}
			Button("Open Buddy…") { controller.openBuddy() }
				.disabled(AgentBranding.buddyAppURL() == nil)
			Button {
				controller.toggleLogin()
			} label: {
				if AgentLoginItem.isEnabled {
					Label("Open at Login", systemImage: "checkmark")
				} else {
					Text("Open at Login")
				}
			}
			.disabled(!AgentLoginItem.isAvailable)
			Divider()
			Button("Help") { controller.showHelp() }
			Button("About Buddy Agent…") { controller.showAbout() }
			Divider()
			Button("Quit Buddy Agent") { controller.quit() }
		} label: {
			Image(systemName: "gearshape")
				.font(.body)
				.frame(width: 24, height: 24)
		}
		.menuStyle(.borderlessButton)
		.menuIndicator(.hidden)
		.fixedSize()
		.foregroundStyle(.secondary)
		.accessibilityLabel("Menu")
	}

	private var displayed: [String] {
		store.sortedAuthNames.filter { name in
			AgentStatusMenu.matches(
				query: query, name: name, auth: store.auths[name] ?? AgentAuth())
		}
	}

	private var listHeight: CGFloat {
		if displayed.isEmpty { return 72 }
		return min(agentStatusListMaxHeight, CGFloat(displayed.count) * 60)
	}

	private var windowSymbol: String {
		if NSImage(systemSymbolName: "macwindow", accessibilityDescription: nil) != nil {
			return "macwindow"
		}
		return "rectangle.on.rectangle"
	}

	private var unlockBlocked: Bool {
		UnlockGate.mode == .unavailable || UnlockGate.promptingDisabled
	}

	/// Touch ID can be drawn in the panel. Password-only Macs keep the system sheet.
	private var embeddedTouchAvailable: Bool {
		guard !unlockBlocked, UnlockGate.fakePresence == nil else { return false }
		var error: NSError?
		return LAContext().canEvaluatePolicy(.deviceOwnerAuthenticationWithBiometrics, error: &error)
	}

	private var lockSubtitle: String {
		if UnlockGate.mode == .unavailable {
			return "This Mac has no Touch ID or login password available."
		}
		if UnlockGate.promptingDisabled {
			return "Unlock prompts are turned off."
		}
		return "Touch ID or enter your password to continue."
	}

	private func emptyLine(_ title: String) -> some View {
		Text(title)
			.foregroundStyle(.secondary)
			.frame(maxWidth: .infinity, minHeight: 72)
	}

	private func beginLockedUnlock() {
		guard !state.unlocked, !unlockBlocked, !listening, !passwordPrompt else { return }
		if UnlockGate.fakePresence != nil || !embeddedTouchAvailable {
			if UnlockGate.fakePresence != nil { usePasswordSheet() }
			return
		}
		beginEmbedded()
	}

	private func beginEmbedded() {
		let context = LAContext()
		context.touchIDAuthenticationAllowableReuseDuration = 0
		listening = true
		touchContext = context
		controller.cancelEmbedded = { [context] in context.invalidate() }
	}

	private func evaluateEmbedded(_ context: LAContext) {
		let started = Date()
		let generation = controller.touchGeneration
		DispatchQueue.global(qos: .userInitiated).async {
			let outcome = gate.authorizeEmbedded(context: context, reason: "unlock the Buddy Agent")
			DispatchQueue.main.async {
				guard generation == self.controller.touchGeneration else { return }
				self.finishEmbedded(context: context, outcome: outcome, started: started)
			}
		}
	}

	private func finishEmbedded(context: LAContext, outcome: UnlockOutcome, started: Date) {
		guard listening, touchContext === context else { return }
		if outcome == .unlocked || state.unlocked {
			listening = false
			controller.cancelEmbedded = nil
			return
		}
		if passwordPrompt {
			passwordPrompt = false
			listening = false
			touchContext = nil
			usePasswordSheet()
			return
		}
		if outcome == .unavailable {
			listening = false
			presentUnavailable(outcome)
			return
		}
		// A real cancel or timeout: keep the panel listening for the next finger.
		// A failure that returns immediately is the view not being ready. While the
		// tray is still open and locked, keep replacing the context. Once it is
		// closed, stop after two of those early failures.
		let immediate = Date().timeIntervalSince(started) < 0.35
		if immediate {
			fastFailures += 1
			let keepTrying = !state.unlocked && controller.isPanelVisible
			if !keepTrying, fastFailures >= 2 {
				listening = false
				return
			}
			DispatchQueue.main.asyncAfter(deadline: .now() + 0.4) {
				guard listening, !self.state.unlocked, self.controller.isPanelVisible else { return }
				self.beginEmbedded()
			}
			return
		}
		fastFailures = 0
		beginEmbedded()
	}

	/// Cancels the in-panel Touch ID, then shows the system password sheet.
	private func usePassword() {
		guard !unlockBlocked, !passwordPrompt else { return }
		if listening, let touchContext {
			passwordPrompt = true
			touchContext.invalidate()
			return
		}
		usePasswordSheet()
	}

	private func usePasswordSheet() {
		passwordPrompt = true
		AgentLockUI.unlock(gate: gate) { outcome in
			passwordPrompt = false
			guard outcome == .unavailable else { return }
			presentUnavailable(outcome)
		}
	}

	private func presentUnavailable(_ outcome: UnlockOutcome) {
		controller.dismiss()
		let copy = AgentLockUI.failureAlert(for: outcome)
		let alert = NSAlert()
		alert.messageText = copy.title
		alert.informativeText = copy.message
		alert.runModal()
	}
}

/// Compact Touch ID glyph. Evaluation of its context draws here, not in a sheet.
private struct EmbeddedTouchID: NSViewRepresentable {
	let context: LAContext
	let onReady: () -> Void

	func makeNSView(context: Context) -> EmbeddedTouchContainer {
		let view = EmbeddedTouchContainer(authContext: self.context)
		view.onReady = onReady
		return view
	}

	func updateNSView(_ nsView: EmbeddedTouchContainer, context: Context) {
		nsView.onReady = onReady
	}
}

private final class EmbeddedTouchContainer: NSView {
	var onReady: (() -> Void)?
	private var started = false

	init(authContext: LAContext) {
		super.init(frame: NSRect(x: 0, y: 0, width: 48, height: 48))
		let auth = LAAuthenticationView(context: authContext, controlSize: .large)
		auth.translatesAutoresizingMaskIntoConstraints = false
		addSubview(auth)
		NSLayoutConstraint.activate([
			auth.centerXAnchor.constraint(equalTo: centerXAnchor),
			auth.centerYAnchor.constraint(equalTo: centerYAnchor),
			auth.widthAnchor.constraint(lessThanOrEqualTo: widthAnchor),
			auth.heightAnchor.constraint(lessThanOrEqualTo: heightAnchor),
		])
	}

	required init?(coder: NSCoder) { return nil }

	override func viewDidMoveToWindow() {
		super.viewDidMoveToWindow()
		guard window != nil, !started else { return }
		started = true
		// onReady makes the panel key, then starts evaluation.
		DispatchQueue.main.async { [weak self] in
			self?.onReady?()
		}
	}
}

private struct AgentStatusAuthRow: View {
	@ObservedObject var store: AgentStore
	@ObservedObject var state: AgentState
	let gate: UnlockGate
	let name: String
	let auth: AgentAuth
	let hasSecret: Bool
	let onOpen: () -> Void
	let onShowBearer: () -> Void
	@State private var hovered = false
	@State private var minting = false
	@State private var mintError: String?

	private var tokens: [TokenSummary] { state.tokens(forAuth: name) }
	private var canMint: Bool { store.canMint(forAuth: name) }
	private var hasTokens: Bool { !tokens.isEmpty }

	var body: some View {
		HStack(spacing: 12) {
			keyButton
			Button(action: onOpen) {
				HStack(spacing: 12) {
					VStack(alignment: .leading, spacing: 1) {
						Text(name)
							.font(.system(size: 15, weight: .semibold))
							.lineLimit(1)
							.truncationMode(.tail)
						let subtitle = AgentStatusMenu.subtitle(auth: auth, hasSecret: hasSecret)
						if !subtitle.isEmpty {
							Text(subtitle)
								.font(.system(size: 12))
								.foregroundStyle(.secondary)
								.lineLimit(1)
								.truncationMode(.middle)
						}
					}
					Spacer(minLength: 0)
					Image(systemName: "chevron.right")
						.font(.system(size: 11, weight: .semibold))
						.foregroundStyle(.tertiary)
				}
				.contentShape(Rectangle())
			}
			.buttonStyle(.plain)
			.accessibilityLabel(name)
			.accessibilityHint("Open in Settings")
		}
		.padding(.horizontal, 12)
		.padding(.vertical, 7)
		.background(
			RoundedRectangle(cornerRadius: 8, style: .continuous)
				.fill(rowFill)
				.padding(.horizontal, 6)
		)
		.onHover { hovered = $0 }
		.accessibilityHint(isDefaultContext ? "Default context" : "")
	}

	/// Linked to the default context, including instance auths on a Jamf Pro
	/// product-tenant under a default environment.
	private var isDefaultContext: Bool {
		AgentStatusMenu.coversDefaultContext(auth: auth, store: store)
	}

	private var rowFill: Color {
		if isDefaultContext {
			return Color.accentColor.opacity(hovered ? 0.28 : 0.18)
		}
		return hovered ? Color.primary.opacity(0.08) : Color.clear
	}

	private var keyButton: some View {
		Button(action: keyAction) {
			ZStack {
				if minting {
					ProgressView().controlSize(.small)
				} else {
					Image(systemName: hasTokens ? "key.fill" : "key")
						.font(.system(size: 16, weight: .semibold))
						.foregroundStyle(keyColor)
				}
			}
			.frame(width: 36, height: 36)
			.background(
				Color.primary.opacity(0.14),
				in: RoundedRectangle(cornerRadius: 8, style: .continuous))
		}
		.buttonStyle(.plain)
		.disabled(minting || (!hasTokens && !canMint))
		.help(bearerHelp)
		.accessibilityLabel("Bearer for \(name)")
		.accessibilityHint(bearerHelp)
	}

	private var keyColor: Color {
		if hasTokens { return tokens.contains(where: \.usable) ? .green : .orange }
		if canMint { return .accentColor }
		return Color.primary.opacity(0.25)
	}

	private var bearerHelp: String {
		if let mintError { return mintError }
		if hasTokens { return "Cached bearer — click for status" }
		if canMint { return "No bearer cached — click to mint" }
		return store.mintUnavailableReason(forAuth: name)
	}

	private func keyAction() {
		if hasTokens {
			onShowBearer()
		} else {
			mintBearer()
		}
	}

	private func mintBearer() {
		minting = true
		mintError = nil
		DispatchQueue.global(qos: .userInitiated).async {
			let error = store.mintTokens(forAuth: name, gate: gate, state: state)
			DispatchQueue.main.async {
				minting = false
				mintError = error
			}
		}
	}
}

private struct AgentStatusPanelChrome: ViewModifier {
	@ViewBuilder
	func body(content: Content) -> some View {
		let shape = RoundedRectangle(cornerRadius: agentStatusPanelCornerRadius, style: .continuous)
		if #available(macOS 26.0, *) {
			content
				.containerBackground(.clear, for: .window)
				.clipShape(shape)
		} else {
			content
				.background(.regularMaterial, in: shape)
				.clipShape(shape)
		}
	}
}
