package script

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"buddy/internal/buddy"

	"github.com/qri-io/starlib/re"
	"go.starlark.net/repl"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

// inREPL is true while the interactive shell is running (used by exit()).
var inREPL bool

// ParseScriptInvocation extracts script path and args from CLI positionals.
// Leading "--trace" flags are stripped; scriptArgs begin after an optional "--".
func ParseScriptInvocation(args []string) (trace bool, scriptPath string, scriptArgs []string) {
	filtered := args[:0]
	for _, a := range args {
		if a == "--trace" {
			trace = true
		} else {
			filtered = append(filtered, a)
		}
	}
	if len(filtered) == 0 {
		return trace, "", nil
	}
	scriptPath = filtered[0]
	scriptArgs = filtered[1:]
	if len(filtered) > 1 && filtered[1] == "--" {
		scriptArgs = filtered[2:]
	}
	return trace, scriptPath, scriptArgs
}

// Run executes a Starlark script file with buddy builtins.
// When trace is true each builtin call is logged to stderr with its position,
// arguments, and return value.
func Run(credName string, opts buddy.Opts, scriptPath string, args []string, logger *slog.Logger, trace bool) error {
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		return fmt.Errorf("read script: %w", err)
	}

	thread, predeclared, err := newThread(credName, opts, logger, trace, args)
	if err != nil {
		return err
	}

	globals, err := starlark.ExecFile(thread, scriptPath, data, predeclared)
	if err != nil {
		return handleExitError(err)
	}
	mainFn := globals["main"]
	if mainFn != nil {
		_, err = starlark.Call(thread, mainFn, nil, []starlark.Tuple{
			{starlark.String("cred"), starlark.String(credName)},
		})
		if err != nil {
			return handleExitError(fmt.Errorf("main: %w", err))
		}
	}
	return nil
}

// ParseEvalArgs returns script args after an optional leading "--".
func ParseEvalArgs(args []string) []string {
	if len(args) > 0 && args[0] == "--" {
		return args[1:]
	}
	return args
}

// RunSource executes a Starlark snippet (e.g. buddy -e / --eval).
// Filename for diagnostics is "<eval>". Top-level if/for are allowed.
// If the last statement is an expression, its value is printed to stdout via
// FormatDisplayValue (JSON-friendly). If main is defined, it is called afterward
// (same convention as file scripts).
func RunSource(credName string, opts buddy.Opts, source string, args []string, logger *slog.Logger, trace bool) error {
	thread, globals, err := newThread(credName, opts, logger, trace, args)
	if err != nil {
		return err
	}

	fileOpts := &syntax.FileOptions{
		Set:             true,
		TopLevelControl: true,
		GlobalReassign:  true,
	}
	const filename = "<eval>"
	f, err := fileOpts.Parse(filename, source, 0)
	if err != nil {
		return err
	}

	var lastExpr syntax.Expr
	if n := len(f.Stmts); n > 0 {
		if es, ok := f.Stmts[n-1].(*syntax.ExprStmt); ok {
			lastExpr = es.X
			f.Stmts = f.Stmts[:n-1]
		}
	}

	if len(f.Stmts) > 0 {
		if err := starlark.ExecREPLChunk(f, thread, globals); err != nil {
			return handleExitError(err)
		}
	}

	if lastExpr != nil {
		// Track stdout from the last expression (unwrap/print) so we do not
		// double-print when also emitting FormatDisplayValue for bare values.
		cw := &countingWriter{w: os.Stdout}
		thread.SetLocal(localStdout, cw)
		val, err := starlark.EvalExprOptions(fileOpts, thread, lastExpr, globals)
		thread.SetLocal(localStdout, nil)
		if err != nil {
			return handleExitError(err)
		}
		if val != nil && val != starlark.None {
			globals["_"] = val
			if cw.n == 0 {
				fmt.Fprintln(os.Stdout, FormatDisplayValue(val))
			}
		}
	}

	mainFn := globals["main"]
	if mainFn != nil {
		_, err = starlark.Call(thread, mainFn, nil, []starlark.Tuple{
			{starlark.String("cred"), starlark.String(credName)},
		})
		if err != nil {
			return handleExitError(fmt.Errorf("main: %w", err))
		}
	}
	return nil
}

// REPL starts an interactive Starlark shell on stdin/stdout.
func REPL(credName string, opts buddy.Opts, logger *slog.Logger, trace bool) error {
	thread, globals, err := newThread(credName, opts, logger, trace, nil)
	if err != nil {
		return err
	}

	inREPL = true
	defer func() { inREPL = false }()

	repl.REPLOptions(&syntax.FileOptions{}, thread, globals)
	return nil
}

func newThread(credName string, opts buddy.Opts, logger *slog.Logger, trace bool, scriptArgs []string) (*starlark.Thread, starlark.StringDict, error) {
	runnerOpts = opts

	reModule, err := re.LoadModule()
	if err != nil {
		return nil, nil, fmt.Errorf("load re module: %w", err)
	}

	predeclared := makeBuiltins(credName, logger, trace)
	predeclared["cred"] = starlark.String(credName)
	predeclared["args"] = listFromStrings(scriptArgs)
	for k, v := range reModule {
		predeclared[k] = v
	}

	thread := &starlark.Thread{
		Name: "buddy",
		Load: stdlibLoader(reModule, predeclared),
	}
	thread.SetLocal("cred", credName)

	thread.Print = func(t *starlark.Thread, msg string) {
		if trace {
			pos := t.CallFrame(0).Pos
			fmt.Fprintf(writerFromLocal(t, localStderr, os.Stderr), "[trace] %s:%d  print: %s\n", pos.Filename(), pos.Line, msg)
		}
		fmt.Fprintln(writerFromLocal(t, localStdout, os.Stdout), msg)
	}

	return thread, predeclared, nil
}

func handleExitError(err error) error {
	var ee *ExitError
	if errors.As(err, &ee) {
		os.Exit(ee.Code)
	}
	return err
}

func listFromStrings(ss []string) *starlark.List {
	elems := make([]starlark.Value, len(ss))
	for i, s := range ss {
		elems[i] = starlark.String(s)
	}
	return starlark.NewList(elems)
}

// countingWriter wraps a writer and counts bytes written.
type countingWriter struct {
	w io.Writer
	n int
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += n
	return n, err
}
