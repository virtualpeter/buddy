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
	private var process: Process?
	private var stdoutPipe: Pipe?
	private var readTask: Task<Void, Never>?

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

	func stop() {
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
				await refreshConfigReadonly()
				return
			}
		}

		if case .starting = phase {
			phase = .failed("Timed out waiting for buddy serve")
			stop()
		}
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
		let config_readonly: Bool?
	}

	func refreshConfigReadonly() async {
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
		} catch {
			configReadonly = false
		}
	}
}
