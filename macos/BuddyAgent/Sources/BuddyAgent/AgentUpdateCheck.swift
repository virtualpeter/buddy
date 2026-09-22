import Foundation

/// Once-a-day GitHub release check. Shares `update-check.json` with buddy so
/// the Mac asks GitHub at most once per running version per day.
@MainActor
final class AgentUpdateCheck: ObservableObject {
	static let shared = AgentUpdateCheck()

	@Published private(set) var updateVersion: String?
	@Published private(set) var updateURL: URL?

	func refresh(enabled: Bool, current: String) {
		guard enabled else {
			updateVersion = nil
			updateURL = nil
			AgentUpdateMenuCache.store(version: nil, url: nil)
			return
		}
		Task.detached(priority: .utility) {
			let notice = AgentUpdateChecker.notice(current: current)
			await MainActor.run {
				self.updateVersion = notice.version
				self.updateURL = notice.url
				AgentUpdateMenuCache.store(version: notice.version, url: notice.url)
			}
		}
	}
}

func agentUpdateMenuNotice() -> (version: String?, url: URL?) {
	AgentUpdateMenuCache.current()
}

private enum AgentUpdateMenuCache {
	private static let lock = NSLock()
	private static var version: String?
	private static var url: URL?

	static func current() -> (version: String?, url: URL?) {
		lock.lock()
		defer { lock.unlock() }
		return (version, url)
	}

	static func store(version: String?, url: URL?) {
		lock.lock()
		self.version = version
		self.url = url
		lock.unlock()
	}
}

enum AgentUpdateChecker {
	private static let endpoint = URL(string: "https://api.github.com/repos/virtualpeter/buddy/releases/latest")!
	private static let pagePrefix = "https://github.com/virtualpeter/buddy/releases/"
	private static let releaseVersion = try! NSRegularExpression(pattern: #"^v\d+\.\d+(?:\.\d+)*$"#)
	private static let interval: TimeInterval = 24 * 60 * 60

	struct Notice {
		var version: String?
		var url: URL?
	}

	static func notice(current: String) -> Notice {
		let file = loadOrRefresh(current: current)
		guard file.ok, releaseNewer(current, file.latest), let url = validatedURL(file.url) else {
			return Notice()
		}
		return Notice(version: file.latest, url: url)
	}

	private struct Cache: Codable {
		var checkedAt: String = ""
		var current: String = ""
		var latest: String = ""
		var url: String = ""
		var ok: Bool = false

		enum CodingKeys: String, CodingKey {
			case checkedAt = "checked_at"
			case current, latest, url, ok
		}

		init(checkedAt: String, current: String, latest: String = "", url: String = "", ok: Bool = false) {
			self.checkedAt = checkedAt
			self.current = current
			self.latest = latest
			self.url = url
			self.ok = ok
		}

		init(from decoder: Decoder) throws {
			let c = try decoder.container(keyedBy: CodingKeys.self)
			checkedAt = try c.decodeIfPresent(String.self, forKey: .checkedAt) ?? ""
			current = try c.decodeIfPresent(String.self, forKey: .current) ?? ""
			latest = try c.decodeIfPresent(String.self, forKey: .latest) ?? ""
			url = try c.decodeIfPresent(String.self, forKey: .url) ?? ""
			ok = try c.decodeIfPresent(Bool.self, forKey: .ok) ?? false
		}

		func encode(to encoder: Encoder) throws {
			var c = encoder.container(keyedBy: CodingKeys.self)
			try c.encode(checkedAt, forKey: .checkedAt)
			try c.encode(current, forKey: .current)
			if !latest.isEmpty { try c.encode(latest, forKey: .latest) }
			if !url.isEmpty { try c.encode(url, forKey: .url) }
			try c.encode(ok, forKey: .ok)
		}
	}

	private static func loadOrRefresh(current: String) -> Cache {
		if let cached = readCache(), cached.current == current, fresh(cached.checkedAt) {
			return cached
		}
		var file = Cache(
			checkedAt: iso8601.string(from: Date()),
			current: current,
			ok: true)
		if isRelease(current) {
			if let fetched = fetch(current: current) {
				file.latest = fetched.tag
				file.url = fetched.page
				file.ok = true
			} else if let prev = readCache(), prev.current == current, prev.ok,
				isRelease(prev.latest), validatedURL(prev.url) != nil
			{
				file.latest = prev.latest
				file.url = prev.url
				file.ok = true
			} else {
				file.ok = false
			}
		}
		writeCache(file)
		return file
	}

	private static func fetch(current: String) -> (tag: String, page: String)? {
		var request = URLRequest(url: endpoint, timeoutInterval: 8)
		let version = current.trimmingCharacters(in: .whitespacesAndNewlines)
		request.setValue(version.isEmpty ? "buddy" : "buddy/\(version)", forHTTPHeaderField: "User-Agent")
		request.setValue("application/vnd.github+json", forHTTPHeaderField: "Accept")
		let sem = DispatchSemaphore(value: 0)
		var body: Data?
		var status = 0
		let task = URLSession.shared.dataTask(with: request) { data, response, _ in
			body = data
			status = (response as? HTTPURLResponse)?.statusCode ?? 0
			sem.signal()
		}
		task.resume()
		_ = sem.wait(timeout: .now() + 8)
		guard status == 200, let body,
			let obj = try? JSONSerialization.jsonObject(with: body) as? [String: Any],
			let tag = obj["tag_name"] as? String, isRelease(tag)
		else {
			return nil
		}
		var page = (obj["html_url"] as? String) ?? ""
		if validatedURL(page) == nil {
			page = pagePrefix + "tag/" + tag
		}
		guard validatedURL(page) != nil else { return nil }
		return (tag, page)
	}

	private static func cacheURL() -> URL {
		if let override = ProcessInfo.processInfo.environment["BUDDY_UPDATE_CHECK_PATH"], !override.isEmpty {
			return URL(fileURLWithPath: override)
		}
		let home = FileManager.default.homeDirectoryForCurrentUser
		return home
			.appendingPathComponent("Library/Application Support/Buddy", isDirectory: true)
			.appendingPathComponent("update-check.json")
	}

	private static func readCache() -> Cache? {
		guard let data = try? Data(contentsOf: cacheURL()) else { return nil }
		return try? JSONDecoder().decode(Cache.self, from: data)
	}

	private static func writeCache(_ file: Cache) {
		let url = cacheURL()
		let dir = url.deletingLastPathComponent()
		try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
		guard let data = try? JSONEncoder().encode(file) else { return }
		try? data.write(to: url, options: .atomic)
		try? FileManager.default.setAttributes([.posixPermissions: 0o600], ofItemAtPath: url.path)
	}

	private static let iso8601: ISO8601DateFormatter = {
		let f = ISO8601DateFormatter()
		f.formatOptions = [.withInternetDateTime]
		return f
	}()

	private static func fresh(_ checkedAt: String) -> Bool {
		guard let t = iso8601.date(from: checkedAt) else { return false }
		let age = Date().timeIntervalSince(t)
		return age >= 0 && age < interval
	}

	private static func isRelease(_ s: String) -> Bool {
		let text = s.trimmingCharacters(in: .whitespacesAndNewlines)
		let range = NSRange(text.startIndex..<text.endIndex, in: text)
		return releaseVersion.firstMatch(in: text, range: range) != nil
	}

	private static func releaseNewer(_ current: String, _ latest: String) -> Bool {
		guard isRelease(current), isRelease(latest) else { return false }
		return compare(current, latest) < 0
	}

	private static func compare(_ a: String, _ b: String) -> Int {
		let pa = parts(a)
		let pb = parts(b)
		let n = max(pa.count, pb.count)
		for i in 0..<n {
			let av = i < pa.count ? pa[i] : 0
			let bv = i < pb.count ? pb[i] : 0
			if av < bv { return -1 }
			if av > bv { return 1 }
		}
		return 0
	}

	private static func parts(_ s: String) -> [Int] {
		let trimmed = s.hasPrefix("v") ? String(s.dropFirst()) : s
		return trimmed.split(separator: ".").map { Int($0) ?? 0 }
	}

	private static func validatedURL(_ raw: String) -> URL? {
		guard let url = URL(string: raw),
			url.scheme == "https",
			url.host == "github.com",
			url.path.hasPrefix("/virtualpeter/buddy/releases/")
		else {
			return nil
		}
		return url
	}
}
