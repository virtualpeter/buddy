package main

import (
	"strings"
	"unicode"

	"buddy/internal/clispec"
)

// extractGlobalFlags removes buddy global flags from argv (any position) and returns the rest.
// Single-dash long flags (-tenant) are rewritten to --tenant. Short flags are not bundled.
// Parsing stops at --. Session flags a command owns (--tenant on cred set channel, etc.)
// are left in place when they appear after the first command word.
func extractGlobalFlags(argv []string) (globalCLI, []string) {
	var g globalCLI
	if len(argv) == 0 {
		return g, argv
	}
	argv = rewriteSingleDashLongs(argv)
	owned := ownedSessionFlags(positionalPath(argv))
	out := []string{argv[0]}
	seenCommand := false
	for i := 1; i < len(argv); i++ {
		a := argv[i]
		if a == "--" {
			out = append(out, argv[i:]...)
			break
		}
		if leaveOwned := seenCommand && isOwnedSessionFlag(a, owned); leaveOwned {
			out = append(out, a)
			continue
		}
		switch {
		case a == clispec.Flag(clispec.FlagVersion):
			g.showVersion = true
		case a == clispec.Short(clispec.ShortSkipConfirm):
			g.skipConfirm = true
		case a == clispec.Short(clispec.ShortVerbose), a == clispec.Flag(clispec.FlagVerbose):
			g.verbose = true
		case a == clispec.Short(clispec.ShortTrace), a == clispec.Flag(clispec.FlagTrace):
			g.trace = true
		case a == clispec.Flag(clispec.FlagScriptTrace):
			g.scriptTrace = true
		case a == clispec.Short(clispec.ShortEval), a == clispec.Flag(clispec.FlagEval):
			if g.evalSource != "" {
				g.evalDup = true
			}
			if i+1 >= len(argv) {
				g.evalMissing = true
			} else {
				i++
				if g.evalSource == "" {
					g.evalSource = argv[i]
				}
			}
		case strings.HasPrefix(a, clispec.FlagPrefix(clispec.FlagEval)):
			if g.evalSource != "" {
				g.evalDup = true
			} else {
				g.evalSource = strings.TrimPrefix(a, clispec.FlagPrefix(clispec.FlagEval))
				if g.evalSource == "" {
					g.evalMissing = true
				}
			}
		case strings.HasPrefix(a, clispec.FlagPrefix(clispec.FlagTenant)):
			g.tenant = strings.TrimPrefix(a, clispec.FlagPrefix(clispec.FlagTenant))
		case a == clispec.Flag(clispec.FlagTenant):
			if i+1 < len(argv) {
				i++
				g.tenant = argv[i]
			}
		case strings.HasPrefix(a, clispec.FlagPrefix(clispec.FlagUser)):
			g.user = strings.TrimPrefix(a, clispec.FlagPrefix(clispec.FlagUser))
		case a == clispec.Flag(clispec.FlagUser):
			if i+1 < len(argv) {
				i++
				g.user = argv[i]
			}
		case strings.HasPrefix(a, clispec.FlagPrefix(clispec.FlagAuth)):
			g.user = strings.TrimPrefix(a, clispec.FlagPrefix(clispec.FlagAuth))
		case a == clispec.Flag(clispec.FlagAuth):
			if i+1 < len(argv) {
				i++
				g.user = argv[i]
			}
		case a == "--cred", strings.HasPrefix(a, "--cred="):
			g.rejectCred = true
			if a == "--cred" && i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "-") {
				i++ // consume value so it is not treated as a command
			}
		default:
			if !strings.HasPrefix(a, "-") {
				seenCommand = true
			}
			out = append(out, a)
		}
	}
	return g, out
}

func rewriteSingleDashLongs(argv []string) []string {
	if len(argv) == 0 {
		return argv
	}
	out := make([]string, 0, len(argv))
	out = append(out, argv[0])
	for i := 1; i < len(argv); i++ {
		a := argv[i]
		if a == "--" {
			out = append(out, argv[i:]...)
			break
		}
		out = append(out, rewriteSingleDashLong(a))
	}
	return out
}

func rewriteSingleDashLong(a string) string {
	if a == "-" || a == "--" || strings.HasPrefix(a, "--") || !strings.HasPrefix(a, "-") {
		return a
	}
	rest := a[1:]
	name := rest
	if i := strings.IndexByte(rest, '='); i >= 0 {
		name = rest[:i]
	}
	if len(name) < 2 || !unicode.IsLetter(rune(name[0])) {
		return a
	}
	return "-" + a
}

func positionalPath(argv []string) []string {
	var path []string
	for i := 1; i < len(argv); i++ {
		a := argv[i]
		if a == "--" {
			break
		}
		if strings.HasPrefix(a, "-") {
			if skip, next := sessionFlagTakesNext(a); skip && next && i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "-") {
				i++
			}
			continue
		}
		path = append(path, a)
	}
	return path
}

func sessionFlagTakesNext(a string) (isSession bool, takesValue bool) {
	if a == clispec.Short(clispec.ShortEval) || a == clispec.Flag(clispec.FlagEval) ||
		a == clispec.Flag(clispec.FlagTenant) || a == clispec.Flag(clispec.FlagUser) ||
		a == clispec.Flag(clispec.FlagAuth) || a == "--cred" {
		return true, true
	}
	if strings.HasPrefix(a, clispec.FlagPrefix(clispec.FlagEval)) ||
		strings.HasPrefix(a, clispec.FlagPrefix(clispec.FlagTenant)) ||
		strings.HasPrefix(a, clispec.FlagPrefix(clispec.FlagUser)) ||
		strings.HasPrefix(a, clispec.FlagPrefix(clispec.FlagAuth)) ||
		strings.HasPrefix(a, "--cred=") {
		return true, false
	}
	return false, false
}

func ownedSessionFlags(path []string) map[string]bool {
	out := map[string]bool{}
	if len(path) == 0 {
		return out
	}
	cmd, _, ok := clispec.ResolveCommand(path[0])
	if !ok {
		return out
	}
	switch cmd {
	case clispec.CmdCred:
		if len(path) < 2 {
			return out
		}
		sub, _ := clispec.MatchToken(path[1], []string{clispec.SubAdd, clispec.SubSet})
		switch sub {
		case clispec.SubAdd:
			out[clispec.FlagTenant] = true
		case clispec.SubSet:
			if len(path) >= 3 {
				if item, _ := clispec.MatchToken(path[2], []string{"channel"}); item == "channel" {
					out[clispec.FlagTenant] = true
				}
			}
		}
	case clispec.CmdSetup:
		if len(path) < 2 {
			return out
		}
		sub, _ := clispec.MatchToken(path[1], []string{"basic-setup", "store-oauth"})
		if sub == "basic-setup" || sub == "store-oauth" {
			out[clispec.FlagTenant] = true
			out[clispec.FlagUser] = true
			out[clispec.FlagAuth] = true
		}
	}
	return out
}

func isOwnedSessionFlag(a string, owned map[string]bool) bool {
	name := strings.TrimPrefix(a, "--")
	if i := strings.IndexByte(name, '='); i >= 0 {
		name = name[:i]
	}
	return owned[name]
}
