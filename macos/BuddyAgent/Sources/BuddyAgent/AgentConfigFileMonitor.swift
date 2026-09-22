import Darwin
import Foundation

/// Watches the Buddy support directory for changes to `agent-config.json`.
enum AgentConfigFileMonitor {
	private static let queue = DispatchQueue(label: "buddy.agent.config.watch")
	private static var source: DispatchSourceFileSystemObject?
	private static var debounceWork: DispatchWorkItem?

	static func start(configPath: URL, onChange: @escaping () -> Void) {
		stop()
		let directory = configPath.deletingLastPathComponent()
		let fd = open(directory.path, O_EVTONLY)
		guard fd >= 0 else { return }
		let monitor = DispatchSource.makeFileSystemObjectSource(
			fileDescriptor: fd,
			eventMask: [.write, .delete, .rename, .extend, .attrib],
			queue: queue)
		monitor.setEventHandler {
			schedule(onChange)
		}
		monitor.setCancelHandler {
			close(fd)
		}
		monitor.resume()
		source = monitor
	}

	static func stop() {
		debounceWork?.cancel()
		debounceWork = nil
		source?.cancel()
		source = nil
	}

	private static func schedule(_ handler: @escaping () -> Void) {
		debounceWork?.cancel()
		let work = DispatchWorkItem {
			DispatchQueue.main.async(execute: handler)
		}
		debounceWork = work
		queue.asyncAfter(deadline: .now() + 0.35, execute: work)
	}
}
