import Foundation

/// Cross-process signal for BuddyAgent to open its Settings window.
enum AgentSettingsOpener {
	static func show() {
		CFNotificationCenterPostNotification(
			CFNotificationCenterGetDarwinNotifyCenter(),
			AgentInterprocess.showSettingsNotification,
			nil,
			nil,
			true)
	}
}
