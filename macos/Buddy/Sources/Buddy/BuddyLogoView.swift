import AppKit
import SwiftUI

struct BuddyLogoView: View {
	@EnvironmentObject private var serve: ServeProcess
	var size: CGFloat = 26

	var body: some View {
		Group {
			if let url = serve.logoURL {
				AsyncImage(url: url) { phase in
					switch phase {
					case .success(let image):
						image.resizable().scaledToFit()
					default:
						bundledLogo
					}
				}
			} else {
				bundledLogo
			}
		}
		.frame(width: size, height: size)
	}

	@ViewBuilder
	private var bundledLogo: some View {
		if let image = Self.appIconImage {
			Image(nsImage: image)
				.resizable()
				.scaledToFit()
		}
	}

	private static var appIconImage: NSImage? {
		if let icon = NSImage(named: "AppIcon") {
			return icon
		}
		guard let url = Bundle.main.url(forResource: "AppIcon", withExtension: "icns") else {
			return nil
		}
		return NSImage(contentsOf: url)
	}
}
