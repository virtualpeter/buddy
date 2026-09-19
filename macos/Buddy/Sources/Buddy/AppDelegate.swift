import AppKit

final class AppDelegate: NSObject, NSApplicationDelegate {
	weak var serve: ServeProcess?

	func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
		true
	}

	func applicationShouldTerminate(_ sender: NSApplication) -> NSApplication.TerminateReply {
		// Do not block the main thread here — a semaphore + MainActor Task deadlocks.
		Task { @MainActor in
			serve?.stop()
			NSApp.reply(toApplicationShouldTerminate: true)
		}
		return .terminateLater
	}

	func applicationShouldHandleReopen(_ sender: NSApplication, hasVisibleWindows flag: Bool) -> Bool {
		if !flag {
			for window in sender.windows where window.canBecomeMain {
				window.makeKeyAndOrderFront(self)
				return false
			}
		}
		return true
	}
}
