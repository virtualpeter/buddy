import Foundation

@MainActor
final class ServeProcess: ObservableObject {
	enum Phase: Equatable {
		case idle
		case starting
		case ready(URL)
		case failed(String)
	}

	@Published private(set) var phase: Phase = .idle
	@Published private(set) var configReadonly = false
	@Published private(set) var needsSetup = false
	private var process: Process?
	private var stdoutPipe: Pipe?
	private var readTask: Task<Void, Never>?
	private var configPollTask: Task<Void, Never>?

	func startIfNeeded() async {
		guard case .idle = phase else { return }
		phase = .starting

		guard let helper = BundlePaths.cliHelperURL() else {
			phase = .failed("CLI helper not found at Contents/Helpers/buddy")
			return
		}

		AgentBootstrap.ensureRunning()

		let proc = Process()
		proc.executableURL = helper
		proc.arguments = ["serve", "--addr=127.0.0.1:0", "--embed"]
		proc.currentDirectoryURL = FileManager.default.homeDirectoryForCurrentUser

		let pipe = Pipe()
		stdoutPipe = pipe
		proc.standardOutput = pipe
		proc.standardError = pipe

		proc.terminationHandler = { [weak self] finished in
			Task { @MainActor in
				guard let self else { return }
				if case .ready = self.phase { return }
				if finished.terminationStatus != 0 {
					if self.needsSetup {
						return
					}
					if AgentBootstrap.agentReachable() {
						self.needsSetup = true
						self.stop()
						self.phase = .idle
						self.startConfigPollingIfNeeded()
						return
					}
					self.phase = .failed("buddy serve exited (code \(finished.terminationStatus))")
				}
			}
		}

		do {
			try proc.run()
		} catch {
			phase = .failed("Could not start buddy serve: \(error.localizedDescription)")
			return
		}

		process = proc
		readTask = Task { await self.readServeURL(from: pipe, process: proc) }
	}

	func stopConfigPolling() {
		configPollTask?.cancel()
		configPollTask = nil
	}

	func startConfigPollingIfNeeded() {
		guard needsSetup else { return }
		stopConfigPolling()
		configPollTask = Task { [weak self] in
			while !Task.isCancelled {
				try? await Task.sleep(nanoseconds: 2_000_000_000)
				guard let self, !Task.isCancelled else { return }
				if case .ready = self.phase {
					await self.refreshConfigState()
					if !self.needsSetup {
						self.stopConfigPolling()
					}
				} else {
					await self.retryServeWhenConfigured()
				}
			}
		}
	}

	private func retryServeWhenConfigured() async {
		switch phase {
		case .failed, .starting:
			stop()
			phase = .idle
			fallthrough
		case .idle:
			await startIfNeeded()
		default:
			break
		}
	}

	func stop() {
		stopConfigPolling()
		readTask?.cancel()
		readTask = nil
		if let stdoutPipe {
			try? stdoutPipe.fileHandleForReading.close()
			self.stdoutPipe = nil
		}
		guard let process else { return }
		if process.isRunning {
			process.terminate()
			let proc = process
			DispatchQueue.global(qos: .utility).asyncAfter(deadline: .now() + 2) {
				if proc.isRunning {
					proc.interrupt()
				}
			}
		}
		self.process = nil
	}

	var logoURL: URL? {
		guard case .ready(let docsURL) = phase,
			var components = URLComponents(url: docsURL, resolvingAgainstBaseURL: false)
		else {
			return nil
		}
		components.path = "/buddy.png"
		return components.url
	}

	func url(for surface: ServeSurface) -> URL? {
		guard case .ready(let docsURL) = phase,
			var components = URLComponents(url: docsURL, resolvingAgainstBaseURL: false)
		else {
			return nil
		}
		components.path = surface.path
		var query = components.queryItems ?? []
		query.append(URLQueryItem(name: "embed", value: "1"))
		components.queryItems = query
		return components.url
	}

	private func readServeURL(from pipe: Pipe, process: Process) async {
		let handle = pipe.fileHandleForReading
		defer { try? handle.close() }

		var buffer = ""
		let deadline = Date().addingTimeInterval(30)
		let marker = "buddy serve: "

		while !Task.isCancelled, Date() < deadline, process.isRunning {
			let chunk = handle.availableData
			if chunk.isEmpty {
				try? await Task.sleep(nanoseconds: 50_000_000)
				continue
			}
			guard let text = String(data: chunk, encoding: .utf8) else { continue }
			buffer.append(text)
			if detectSetupRequired(in: buffer), !process.isRunning {
				needsSetup = true
				stop()
				phase = .idle
				AgentBootstrap.ensureRunning()
				startConfigPollingIfNeeded()
				return
			}
			guard let range = buffer.range(of: marker) else {
				if buffer.count > 65_536 {
					buffer = String(buffer.suffix(32_768))
				}
				continue
			}
			let rest = buffer[range.upperBound...]
			let line = rest.split(whereSeparator: \.isNewline).first.map(String.init) ?? ""
			let trimmed = line.trimmingCharacters(in: .whitespacesAndNewlines)
			if let url = URL(string: trimmed), url.scheme == "http" {
				phase = .ready(url)
				await refreshConfigState()
				if needsSetup {
					startConfigPollingIfNeeded()
				}
				return
			}
		}

		if case .starting = phase {
			if detectSetupRequired(in: buffer) {
				needsSetup = true
				stop()
				phase = .idle
				AgentBootstrap.ensureRunning()
				startConfigPollingIfNeeded()
			} else {
				phase = .failed("Timed out waiting for buddy serve")
				stop()
			}
		}
	}

	func detectSetupRequired(in output: String) -> Bool {
		output.contains("Buddy Agent needs config")
			|| output.contains("Failed to load config")
			|| output.contains("needs_setup")
	}
}

enum ServeSurface: String, CaseIterable, Identifiable {
	case docs = "Docs"
	case config = "Config"
	case ide = "IDE"

	var id: String { rawValue }

	var path: String {
		switch self {
		case .docs: return "/docs/"
		case .config: return "/config"
		case .ide: return "/ide"
		}
	}

	static func available(configReadonly: Bool) -> [ServeSurface] {
		if configReadonly {
			return allCases.filter { $0 != .config }
		}
		return allCases
	}
}

private extension ServeProcess {
	struct ConfigSnapshot: Decodable {
		let needs_setup: Bool?
		let config_readonly: Bool?
	}

	func refreshConfigState() async {
		guard case .ready(let docsURL) = phase,
			var components = URLComponents(url: docsURL, resolvingAgainstBaseURL: false)
		else {
			return
		}
		components.path = "/api/config"
		components.query = nil
		guard let apiURL = components.url else { return }
		do {
			let (data, response) = try await URLSession.shared.data(from: apiURL)
			guard let http = response as? HTTPURLResponse, http.statusCode == 200 else { return }
			let snap = try JSONDecoder().decode(ConfigSnapshot.self, from: data)
			configReadonly = snap.config_readonly ?? false
			needsSetup = snap.needs_setup ?? false
		} catch {
			configReadonly = false
			needsSetup = false
		}
	}
}
