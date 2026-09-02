package help

import (
	"strings"
)

// LibraryFunc is one browsable function for the IDE libraries API / LSP.
type LibraryFunc struct {
	Name      string `json:"name"`
	Signature string `json:"signature"`
	Summary   string `json:"summary,omitempty"` // Does: purpose sentence
	Params    string `json:"params,omitempty"`
	Returns   string `json:"returns,omitempty"`
	Notes     string `json:"notes,omitempty"`
	Insert    string `json:"insert"`
	Section   string `json:"section,omitempty"`
	Group     string `json:"group,omitempty"` // getting-started | advanced | "" (resource section title)
}

// LibrarySection groups builtins by help section heading or curated group.
type LibrarySection struct {
	Title     string        `json:"title"`
	Functions []LibraryFunc `json:"functions"`
}

// BuiltinsLibrary returns the builtins catalog parsed from Builtins help text,
// then regrouped for IDE discoverability (Getting started / Advanced).
func BuiltinsLibrary() (sections []LibrarySection, flat []LibraryFunc) {
	sections, flat = parseBuiltinsCatalog(Builtins)
	return curateBuiltinsLibrary(sections, flat)
}

func parseBuiltinsCatalog(src string) (sections []LibrarySection, flat []LibraryFunc) {
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	i := 0
	for i < len(lines) {
		line := lines[i]
		if strings.HasPrefix(line, "===") || strings.HasPrefix(line, "buddy built-in") {
			i++
			continue
		}
		break
	}
	for i < len(lines) {
		line := lines[i]
		if isBuiltinsSection(line) {
			break
		}
		i++
	}

	var cur *LibrarySection
	flush := func() {
		if cur != nil && len(cur.Functions) > 0 {
			sections = append(sections, *cur)
		}
	}

	for i < len(lines) {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			i++
			continue
		}
		if isBuiltinsSection(line) {
			flush()
			cur = &LibrarySection{Title: line}
			i++
			continue
		}
		if name, sigLines, next, ok := consumeBuiltinSignature(lines, i); ok {
			if cur == nil {
				cur = &LibrarySection{Title: "BUILTINS"}
			}
			i = next
			fields, next := consumeBuiltinFields(lines, i)
			i = next
			sig := strings.Join(sigLines, " ")
			sig, inlineRet := splitInlineReturn(sig)
			if fields.Returns == "" {
				fields.Returns = inlineRet
			}
			summary := fields.Does
			if summary == "" {
				summary = fields.Returns
			}
			if summary == "" {
				summary = fields.Params
			}
			fn := LibraryFunc{
				Name:      name,
				Signature: sig,
				Summary:   summary,
				Params:    fields.Params,
				Returns:   fields.Returns,
				Notes:     fields.Notes,
				Insert:    defaultInsert(name),
				Section:   cur.Title,
			}
			cur.Functions = append(cur.Functions, fn)
			flat = append(flat, fn)
			continue
		}
		i++
	}
	flush()
	return sections, flat
}

// splitInlineReturn pulls "→ …" off the end of a signature line.
func splitInlineReturn(sig string) (clean, ret string) {
	idx := strings.Index(sig, "→")
	if idx < 0 {
		idx = strings.Index(sig, "->")
	}
	if idx < 0 {
		return sig, ""
	}
	clean = strings.TrimSpace(sig[:idx])
	ret = strings.TrimSpace(sig[idx:])
	ret = strings.TrimSpace(strings.TrimPrefix(ret, "→"))
	ret = strings.TrimSpace(strings.TrimPrefix(ret, "->"))
	return clean, ret
}

func defaultInsert(name string) string {
	switch name {
	case "change_batch":
		return `change = change_batch(title = "My change")
steps = [
    # step_set_package(name = "…", version = "…", from_trigger = "…"),
]
planned = plan_change(change, steps)
# run_change(change, steps, planned = planned)`
	case "plan_change":
		return "planned = plan_change(change, steps)"
	case "run_change":
		return "run_change(change, steps, planned = planned)"
	case "app_reconcile":
		return `desired = {"name": "santa", "version": "2026.3", "kind": "cache"}
app_reconcile(desired = desired)  # plan; apply = True to write`
	default:
		return name + "()"
	}
}

var gettingStartedOrder = []string{
	"policies", "policy", "packages", "package", "computers", "computer",
	"smart_groups", "profiles", "change_batch", "plan_change", "run_change",
}

var gettingStartedSet = func() map[string]bool {
	m := make(map[string]bool, len(gettingStartedOrder))
	for _, n := range gettingStartedOrder {
		m[n] = true
	}
	return m
}()

func isAdvancedBuiltin(name, section string) bool {
	if strings.HasPrefix(name, "step_") {
		return true
	}
	switch name {
	case "policy_set_ss", "policy_set_install", "policy_set_package", "policy_set_scope_all",
		"policy_create_ss", "policy_create_package", "policy_delete", "policy_put_xml",
		"policy_set_icon",
		"run_workflow":
		return true
	}
	if strings.HasPrefix(name, "payloads_") || strings.HasPrefix(name, "rsql_") ||
		strings.HasPrefix(name, "plist_") || strings.HasPrefix(name, "profile_xml_") ||
		strings.HasPrefix(name, "xml_") || strings.HasPrefix(name, "file_") {
		return true
	}
	sec := strings.ToUpper(section)
	if strings.Contains(sec, "DECLARATIVE") || strings.Contains(sec, "OFFLINE") ||
		strings.Contains(sec, "PAYLOAD") || strings.Contains(sec, "RSQL") ||
		strings.Contains(sec, "PLIST") || strings.Contains(sec, "XML HELPER") {
		return true
	}
	return false
}

// curateBuiltinsLibrary rebuilds sections: Getting started, resource sections, Advanced.
func curateBuiltinsLibrary(rawSections []LibrarySection, flat []LibraryFunc) ([]LibrarySection, []LibraryFunc) {
	byName := make(map[string]LibraryFunc, len(flat))
	for _, f := range flat {
		byName[f.Name] = f
	}

	var out []LibrarySection
	var outFlat []LibraryFunc
	seen := make(map[string]bool)

	appendFn := func(sec *LibrarySection, f LibraryFunc, group string) {
		if seen[f.Name] {
			return
		}
		seen[f.Name] = true
		f.Group = group
		sec.Functions = append(sec.Functions, f)
		outFlat = append(outFlat, f)
	}

	start := LibrarySection{Title: "Getting started"}
	for _, name := range gettingStartedOrder {
		f, ok := byName[name]
		if !ok {
			continue
		}
		appendFn(&start, f, "getting-started")
	}
	if len(start.Functions) > 0 {
		out = append(out, start)
	}

	for _, sec := range rawSections {
		resource := LibrarySection{Title: sec.Title}
		for _, f := range sec.Functions {
			if gettingStartedSet[f.Name] || isAdvancedBuiltin(f.Name, f.Section) {
				continue
			}
			appendFn(&resource, f, "")
		}
		if len(resource.Functions) > 0 {
			out = append(out, resource)
		}
	}

	adv := LibrarySection{Title: "Advanced"}
	for _, sec := range rawSections {
		for _, f := range sec.Functions {
			if gettingStartedSet[f.Name] || !isAdvancedBuiltin(f.Name, f.Section) {
				continue
			}
			appendFn(&adv, f, "advanced")
		}
	}
	if len(adv.Functions) > 0 {
		out = append(out, adv)
	}

	return out, outFlat
}

// LookupBuiltin returns catalog metadata for a builtin name (case-sensitive).
func LookupBuiltin(name string) (LibraryFunc, bool) {
	_, flat := BuiltinsLibrary()
	for _, f := range flat {
		if f.Name == name {
			return f, true
		}
	}
	return LibraryFunc{}, false
}

// FormatBuiltinHelp renders catalog help for a bare builtin name (shell / IDE result).
func FormatBuiltinHelp(f LibraryFunc) string {
	var b strings.Builder
	b.WriteString(f.Signature)
	if f.Summary != "" {
		b.WriteString("\nDoes: ")
		b.WriteString(f.Summary)
	}
	if f.Returns != "" {
		b.WriteString("\n→ ")
		b.WriteString(f.Returns)
	}
	if f.Params != "" {
		b.WriteString("\nParams: ")
		b.WriteString(f.Params)
	}
	if f.Notes != "" {
		b.WriteString("\nNotes: ")
		b.WriteString(f.Notes)
	}
	return b.String()
}

// FormatBuiltinHoverMarkdown renders the Function index card as markdown for LSP hover.
func FormatBuiltinHoverMarkdown(f LibraryFunc) string {
	var b strings.Builder
	b.WriteString(formatSignatureMarkdown(f.Name, f.Signature))
	if f.Summary != "" {
		b.WriteString("\n\n")
		b.WriteString(f.Summary)
	}
	ret := displayReturns(f.Returns)
	if ret != "" {
		b.WriteString("\n\n**Returns:** ")
		b.WriteString(ret)
	}
	if strings.TrimSpace(f.Params) != "" {
		items := splitParamItems(f.Params)
		b.WriteString("\n\n**Params:**")
		if len(items) == 1 {
			b.WriteString(" ")
			b.WriteString(formatParamItemMarkdown(items[0]))
		} else {
			b.WriteByte('\n')
			for _, it := range items {
				b.WriteString("- ")
				b.WriteString(formatParamItemMarkdown(it))
				b.WriteByte('\n')
			}
		}
	}
	if strings.TrimSpace(f.Notes) != "" {
		b.WriteString("\n\n**Notes:** ")
		b.WriteString(strings.TrimSpace(f.Notes))
	}
	return strings.TrimSpace(b.String()) + "\n"
}

func formatSignatureMarkdown(name, sig string) string {
	bold := "**" + name + "**"
	sig = strings.TrimSpace(sig)
	if sig == "" || sig == name {
		return bold
	}
	token := name + "("
	if !strings.Contains(sig, token) {
		return bold + " `" + sig + "`"
	}
	return strings.ReplaceAll(sig, token, bold+"(")
}
