import AppKit
import Darwin
import Foundation

/// Ensures the nested Buddy Agent is running before buddy serve starts.
enum AgentBootstrap {
	static func ensureRunning() {
		if agentReachable() { return }
		guard let agentApp = BundlePaths.nestedAgentAppURL() else { return }

		let config = NSWorkspace.OpenConfiguration()
		config.promptsUserIfNeeded = false
		NSWorkspace.shared.openApplication(at: agentApp, configuration: config) { _, _ in }

		if waitForAgent(until: Date().addingTimeInterval(8)) { return }

		let binary = agentApp.appendingPathComponent("Contents/MacOS/BuddyAgent")
		guard FileManager.default.isExecutableFile(atPath: binary.path) else { return }
		let proc = Process()
		proc.executableURL = binary
		proc.standardOutput = FileHandle.nullDevice
		proc.standardError = FileHandle.nullDevice
		try? proc.run()
		_ = waitForAgent(until: Date().addingTimeInterval(8))
	}

	static func agentReachable() -> Bool {
		let path = BundlePaths.agentSocketPath()
		guard !path.isEmpty else { return false }

		var st = stat()
		guard stat(path, &st) == 0, (st.st_mode & S_IFMT) == S_IFSOCK else { return false }

		var addr = sockaddr_un()
		addr.sun_family = sa_family_t(AF_UNIX)
		let pathMax = MemoryLayout.size(ofValue: addr.sun_path)
		let copied = path.withCString { cstr -> Bool in
			strlen(cstr) < pathMax && strlcpy(&addr.sun_path.0, cstr, pathMax) < pathMax
		}
		guard copied else { return false }

		let fd = socket(AF_UNIX, SOCK_STREAM, 0)
		guard fd >= 0 else { return false }
		defer { close(fd) }

		var timeout = timeval(tv_sec: 0, tv_usec: 200_000)
		setsockopt(
			fd, SOL_SOCKET, SO_SNDTIMEO, &timeout, socklen_t(MemoryLayout<timeval>.size))

		let len = socklen_t(MemoryLayout<sockaddr_un>.size)
		let ok = withUnsafePointer(to: &addr) { ptr in
			ptr.withMemoryRebound(to: sockaddr.self, capacity: 1) { sa in
				connect(fd, sa, len) == 0
			}
		}
		return ok
	}

	private static func waitForAgent(until deadline: Date) -> Bool {
		while Date() < deadline {
			if agentReachable() { return true }
			Thread.sleep(forTimeInterval: 0.1)
		}
		return agentReachable()
	}
}
