import SwiftUI
import WebKit

struct ServeWebView: NSViewRepresentable {
	let url: URL
	@ObservedObject var appearance: BuddyAppearanceStore

	func makeCoordinator() -> Coordinator {
		Coordinator(appearance: appearance)
	}

	func makeNSView(context: Context) -> WKWebView {
		let config = WKWebViewConfiguration()
		syncUserScripts(on: config.userContentController, appearance: appearance.choice)
		let view = WKWebView(frame: .zero, configuration: config)
		view.navigationDelegate = context.coordinator
		view.setValue(false, forKey: "drawsBackground")
		context.coordinator.webView = view
		view.load(URLRequest(url: url))
		return view
	}

	func updateNSView(_ view: WKWebView, context: Context) {
		context.coordinator.appearance = appearance
		context.coordinator.webView = view

		syncUserScripts(on: view.configuration.userContentController, appearance: appearance.choice)

		let appearanceChanged = context.coordinator.lastApplied != appearance.choice
		let urlChanged = view.url != url

		if urlChanged {
			view.load(URLRequest(url: url))
		} else if appearanceChanged {
			appearance.choice.apply(to: view)
			context.coordinator.lastApplied = appearance.choice
		}
	}

	private func syncUserScripts(on controller: WKUserContentController, appearance: BuddyAppearance) {
		controller.removeAllUserScripts()
		controller.addUserScript(appearance.userScript())
	}

	final class Coordinator: NSObject, WKNavigationDelegate {
		var appearance: BuddyAppearanceStore
		weak var webView: WKWebView?
		var lastApplied: BuddyAppearance?

		init(appearance: BuddyAppearanceStore) {
			self.appearance = appearance
		}

		func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
			let choice = appearance.choice
			choice.apply(to: webView)
			lastApplied = choice
		}
	}
}
