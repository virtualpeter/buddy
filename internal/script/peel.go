package script

import (
	"strings"

	"go.starlark.net/syntax"
)

// stripTrailingNoise removes trailing blank lines and full-line # comments.
// End-of-line comments on code lines are left alone.
func stripTrailingNoise(src string) string {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	lines := strings.Split(src, "\n")
	end := len(lines)
	for end > 0 {
		line := strings.TrimSpace(lines[end-1])
		if line == "" || strings.HasPrefix(line, "#") {
			end--
			continue
		}
		break
	}
	if end == len(lines) {
		return src
	}
	return strings.Join(lines[:end], "\n")
}

// isNoiseStmt reports statements that should not defeat last-expression Out:
// pass, string/bytes literals (docstring-as-comment), and bare None.
func isNoiseStmt(stmt syntax.Stmt) bool {
	switch s := stmt.(type) {
	case *syntax.BranchStmt:
		return s.Token == syntax.PASS
	case *syntax.ExprStmt:
		return isNoiseExpr(s.X)
	default:
		return false
	}
}

func isNoiseExpr(expr syntax.Expr) bool {
	switch e := expr.(type) {
	case *syntax.Literal:
		return e.Token == syntax.STRING || e.Token == syntax.BYTES
	case *syntax.Ident:
		return e.Name == "None"
	default:
		return false
	}
}

// peelLastExpr finds the last meaningful expression statement, walking back
// past trailing noise (pass, string literals, None). It detaches that ExprStmt
// from f.Stmts and returns its expression. Noise stmts stay in the prefix.
func peelLastExpr(f *syntax.File) syntax.Expr {
	if f == nil {
		return nil
	}
	n := len(f.Stmts)
	i := n - 1
	for i >= 0 && isNoiseStmt(f.Stmts[i]) {
		i--
	}
	if i < 0 {
		return nil
	}
	es, ok := f.Stmts[i].(*syntax.ExprStmt)
	if !ok {
		return nil
	}
	if isNoiseExpr(es.X) {
		return nil
	}
	expr := es.X
	// Remove the peeled ExprStmt; keep trailing noise in place (no-ops).
	f.Stmts = append(f.Stmts[:i], f.Stmts[i+1:]...)
	return expr
}
