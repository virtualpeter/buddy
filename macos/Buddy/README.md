# Buddy.app (macOS shell)

SwiftUI app at `Contents/MacOS/Buddy` inside **Buddy.app**. It is not a second CLI — it spawns the Go helper at `Contents/Helpers/buddy` as:

```text
buddy serve --addr=127.0.0.1:0 --embed
```

and loads Docs, Config, and IDE in `WKWebView` tabs with a native toolbar.

## Behaviour

- **Dynamic port** — parses `buddy serve: http://127.0.0.1:…` from stdout; appends `?embed=1` to page URLs.
- **Embed mode** — `--embed` hides in-page logo, surface nav, and web theme toggle; this app supplies tabs and **View → Appearance** (system / light / dark via `buddy-serve-theme` / `data-theme`).
- **Buddy Agent** — `AgentBootstrap` ensures the nested `LoginItems/BuddyAgent.app` is running. Polls `/api/config` for `config_readonly`; when true, the Config tab is hidden (identity edits belong in Agent Settings).
- **Agent Settings** — toolbar gear or **Agent Settings…** (⇧⌘,) posts Darwin notification `virtualpete.buddy.agent.showSettings`.
- **Quit** — `AppDelegate` stops the serve subprocess on the main actor before terminate (avoids blocking on a stopped run loop).

## Build and run

Production bundle (signed helper + Agent + shell):

```sh
make app              # → dist/Buddy.app
open dist/Buddy.app
```

From Xcode (`Buddy.xcodeproj`), Run needs `Contents/Helpers/buddy` in the built `.app`. Easiest path: run `make app` once, then open the project and point the scheme at `dist/Buddy.app`, or copy a built helper into `DerivedData`’s product bundle after `make buddy-darwin-arm64`.

Version strings match the CLI (`scripts/gen-buddy-version.sh` → `Version.xcconfig`; **Generate Version** build phase).

## Layout

| Path | Role |
|------|------|
| `Sources/Buddy/BuddyApp.swift` | SwiftUI entry, menus |
| `Sources/Buddy/ContentView.swift` | Toolbar + tab content |
| `Sources/Buddy/ServeProcess.swift` | Subprocess lifecycle, config readonly poll |
| `Sources/Buddy/ServeWebView.swift` | WKWebView + theme injection |
| `Sources/Buddy/BuddyToolbar.swift` | Logo (→ Docs), segmented tabs, gear |
| `Sources/Buddy/AgentBootstrap.swift` | Start nested Agent if needed |
| `Sources/Buddy/AgentSettingsOpener.swift` | Raise Agent Settings window |

Assembly and codesign: [`scripts/build-buddy-app.sh`](../../scripts/build-buddy-app.sh).

See also [`macos/BuddyAgent/README.md`](../BuddyAgent/README.md) and [docs/reference-manual.md](../../docs/reference-manual.md) (Local docs and script IDE).
