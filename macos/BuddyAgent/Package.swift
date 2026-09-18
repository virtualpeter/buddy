// swift-tools-version: 5.9
import PackageDescription

// Spike scaffold for the optional macOS Buddy Agent. Built as a SwiftPM
// executable so it runs (and compiles in CI) without an .xcodeproj; see
// README.md for wrapping the same sources in an Xcode app target when the
// spike needs Developer ID signing and Keychain Sharing.
let package = Package(
	name: "BuddyAgent",
	platforms: [.macOS(.v13)],
	targets: [
		.executableTarget(
			name: "BuddyAgent",
			path: "Sources/BuddyAgent"
		)
	]
)
