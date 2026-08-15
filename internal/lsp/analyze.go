package lsp

import (
	"fmt"
	"strings"

	"go.starlark.net/resolve"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

// Diagnostic is an LSP-style diagnostic (0-based positions).
type Diagnostic struct {
	Message  string
	Severity int // 1 Error, 2 Warning, 3 Information, 4 Hint
	Line     int // 0-based
	Col      int // 0-based
	EndLine  int
	EndCol   int
}

const (
	severityError       = 1
	severityWarning     = 2
	severityInformation = 3
	severityHint        = 4
)

// AnalyzeMode controls Starlark FileOptions for analysis.
type AnalyzeMode int

const (
	// ModeFile is the default for .buddy scripts (ExecFile dialect).
	ModeFile AnalyzeMode = iota
	// ModeCell allows top-level if/for and global reassign (-e / REPL).
	ModeCell
)

func fileOptions(mode AnalyzeMode) *syntax.FileOptions {
	switch mode {
	case ModeCell:
		return &syntax.FileOptions{
			Set:             true,
			TopLevelControl: true,
			GlobalReassign:  true,
		}
	default:
		return &syntax.FileOptions{}
	}
}

// Analyze parses and resolves source, returning diagnostics.
func Analyze(filename, src string, mode AnalyzeMode) []Diagnostic {
	cat, err := getCatalog()
	if err != nil {
		return []Diagnostic{{
			Message:  "buddy catalog: " + err.Error(),
			Severity: severityError,
			Line:     0,
			Col:      0,
			EndLine:  0,
			EndCol:   1,
		}}
	}

	opts := fileOptions(mode)
	f, err := opts.Parse(filename, src, 0)
	if err != nil {
		return parseDiagnostics(err)
	}

	var diags []Diagnostic
	diags = append(diags, loadDiagnostics(f)...)

	if err := resolve.File(f, cat.isPredeclared, starlark.Universe.Has); err != nil {
		diags = append(diags, resolveDiagnostics(err)...)
	}
	return diags
}

func parseDiagnostics(err error) []Diagnostic {
	switch e := err.(type) {
	case syntax.Error:
		return []Diagnostic{posDiag(e.Pos, e.Msg, severityError)}
	default:
		// Multi-error or wrapped — best effort from Error() string.
		return []Diagnostic{{
			Message:  err.Error(),
			Severity: severityError,
			Line:     0,
			Col:      0,
			EndLine:  0,
			EndCol:   1,
		}}
	}
}

func resolveDiagnostics(err error) []Diagnostic {
	switch e := err.(type) {
	case resolve.ErrorList:
		out := make([]Diagnostic, 0, len(e))
		for _, item := range e {
			out = append(out, posDiag(item.Pos, item.Msg, severityError))
		}
		return out
	case resolve.Error:
		return []Diagnostic{posDiag(e.Pos, e.Msg, severityError)}
	default:
		return []Diagnostic{{
			Message:  err.Error(),
			Severity: severityError,
			Line:     0,
			Col:      0,
			EndLine:  0,
			EndCol:   1,
		}}
	}
}

func posDiag(pos syntax.Position, msg string, severity int) Diagnostic {
	line := int(pos.Line) - 1
	col := int(pos.Col) - 1
	if line < 0 {
		line = 0
	}
	if col < 0 {
		col = 0
	}
	return Diagnostic{
		Message:  msg,
		Severity: severity,
		Line:     line,
		Col:      col,
		EndLine:  line,
		EndCol:   col + 1,
	}
}

func loadDiagnostics(f *syntax.File) []Diagnostic {
	var diags []Diagnostic
	syntax.Walk(f, func(n syntax.Node) bool {
		load, ok := n.(*syntax.LoadStmt)
		if !ok {
			return true
		}
		module := load.Module.Value.(string)
		if !validLoadModule(module) {
			msg := fmt.Sprintf("unknown module %q (try load(\"re\", \"re\"))", module)
			diags = append(diags, posDiag(load.Module.TokenPos, msg, severityError))
		}
		return true
	})
	return diags
}

// wordAt returns the identifier under/before the cursor (UTF-16-ish approx via runes).
func wordAt(src string, line, character int) (prefix string, startCol int) {
	lines := strings.Split(src, "\n")
	if line < 0 || line >= len(lines) {
		return "", character
	}
	row := lines[line]
	runes := []rune(row)
	if character > len(runes) {
		character = len(runes)
	}
	i := character
	for i > 0 {
		r := runes[i-1]
		if isIdentPart(r) {
			i--
			continue
		}
		break
	}
	j := character
	for j < len(runes) && isIdentPart(runes[j]) {
		j++
	}
	if i >= j {
		return "", character
	}
	// Prefix for completion is text before cursor within the ident.
	end := character
	if end < i {
		end = i
	}
	if end > j {
		end = j
	}
	return string(runes[i:end]), i
}

func isIdentPart(r rune) bool {
	return r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

// callAt finds the call expression and argument index at the cursor for signature help.
func callAt(src string, line, character int) (fnName string, argIndex int, ok bool) {
	lines := strings.Split(src, "\n")
	if line < 0 || line >= len(lines) {
		return "", 0, false
	}
	offset := 0
	for i := 0; i < line; i++ {
		offset += len(lines[i]) + 1
	}
	row := []rune(lines[line])
	if character > len(row) {
		character = len(row)
	}
	offset += len(string(row[:character]))

	runes := []rune(src)
	if offset > len(runes) {
		offset = len(runes)
	}
	depth := 0
	for i := offset - 1; i >= 0; i-- {
		switch runes[i] {
		case ')':
			depth++
		case '(':
			if depth == 0 {
				j := i - 1
				for j >= 0 && (runes[j] == ' ' || runes[j] == '\t' || runes[j] == '\n') {
					j--
				}
				end := j + 1
				for j >= 0 && isIdentPart(runes[j]) {
					j--
				}
				name := string(runes[j+1 : end])
				if name == "" {
					return "", 0, false
				}
				argIndex = 0
				d := 0
				for k := i + 1; k < offset; k++ {
					switch runes[k] {
					case '(', '[', '{':
						d++
					case ')', ']', '}':
						d--
					case ',':
						if d == 0 {
							argIndex++
						}
					}
				}
				return name, argIndex, true
			}
			depth--
		}
	}
	return "", 0, false
}

func hoverMarkdown(sym Symbol) string {
	var b strings.Builder
	b.WriteString("```starlark\n")
	if sym.Signature != "" {
		b.WriteString(sym.Signature)
	} else {
		b.WriteString(sym.Name)
	}
	b.WriteString("\n```\n")
	if sym.Detail != "" {
		b.WriteString("\n")
		b.WriteString(sym.Detail)
		b.WriteString("\n")
	}
	return b.String()
}

func signatureLabel(sym Symbol) string {
	if sym.Signature != "" {
		return sym.Signature
	}
	return sym.Name + "()"
}

func signatureParams(sig string) []string {
	start := strings.Index(sig, "(")
	end := strings.LastIndex(sig, ")")
	if start < 0 || end <= start {
		return nil
	}
	inner := strings.TrimSpace(sig[start+1 : end])
	if inner == "" {
		return nil
	}
	parts := strings.Split(inner, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}
