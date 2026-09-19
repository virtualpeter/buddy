import AppKit
import SwiftUI
import WebKit

/// Appearance for embedded serve pages (replaces the web theme toggle in --embed mode).
enum BuddyAppearance: String, CaseIterable, Identifiable {
	case system
	case light
	case dark

	var id: String { rawValue }

	var label: String {
		switch self {
		case .system: return "System"
		case .light: return "Light"
		case .dark: return "Dark"
		}
	}

	static let defaultsKey = "buddy.app.appearance"

	static func load() -> BuddyAppearance {
		guard let raw = UserDefaults.standard.string(forKey: defaultsKey),
			let value = BuddyAppearance(rawValue: raw)
		else {
			return .system
		}
		return value
	}

	func persist() {
		UserDefaults.standard.set(rawValue, forKey: BuddyAppearance.defaultsKey)
	}

	/// Injected at document start so serve pages match the app menu.
	func userScript() -> WKUserScript {
		WKUserScript(
			source: Self.bootstrapJS(theme: rawValue),
			injectionTime: .atDocumentStart,
			forMainFrameOnly: true)
	}

	func apply(to webView: WKWebView) {
		webView.evaluateJavaScript(Self.bootstrapJS(theme: rawValue), completionHandler: nil)
	}

	private static func bootstrapJS(theme: String) -> String {
		"""
		(function(){
		  var mode = '\(theme)';
		  function sys(){
		    try { return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'; }
		    catch (e) { return 'dark'; }
		  }
		  var t;
		  if (mode === 'system') {
		    try { localStorage.removeItem('buddy-serve-theme'); } catch (e) {}
		    t = sys();
		  } else {
		    t = mode === 'light' ? 'light' : 'dark';
		    try { localStorage.setItem('buddy-serve-theme', t); } catch (e) {}
		  }
		  document.documentElement.setAttribute('data-theme', t);
		})();
		"""
	}
}

@MainActor
final class BuddyAppearanceStore: ObservableObject {
	@Published var choice: BuddyAppearance {
		didSet { choice.persist() }
	}

	init() {
		choice = BuddyAppearance.load()
	}
}
