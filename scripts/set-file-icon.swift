import AppKit

if CommandLine.arguments.count != 3 {
	fputs("usage: set-file-icon.swift <icns-or-png> <file>\n", stderr)
	exit(2)
}

let iconPath = CommandLine.arguments[1]
let filePath = CommandLine.arguments[2]
guard let img = NSImage(contentsOfFile: iconPath) else {
	fputs("set-file-icon: could not read \(iconPath)\n", stderr)
	exit(1)
}
if !NSWorkspace.shared.setIcon(img, forFile: filePath, options: []) {
	fputs("set-file-icon: setIcon failed for \(filePath)\n", stderr)
	exit(1)
}
