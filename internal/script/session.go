package script

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"

	"buddy/internal/buddy"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

const (
	localStdout = "buddy.stdout"
	localStderr = "buddy.stderr"
	localSession = "buddy.session"
)

// EvalResult is the outcome of evaluating one cell or snippet.
type EvalResult struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
	Result string `json:"result,omitempty"` // last expression value, if any
	Error  string `json:"error,omitempty"`
}

// Session is a persistent Starlark environment (REPL / notebook / MCP eval).
type Session struct {
	mu       sync.Mutex
	credName string
	opts     buddy.Opts
	logger   *slog.Logger
	trace    bool
	thread   *starlark.Thread
	globals  starlark.StringDict
}

// NewSession creates a Starlark session with buddy builtins.
func NewSession(credName string, opts buddy.Opts, logger *slog.Logger, trace bool) (*Session, error) {
	s := &Session{
		credName: credName,
		opts:     opts,
		logger:   logger,
		trace:    trace,
	}
	if err := s.resetLocked(); err != nil {
		return nil, err
	}
	return s, nil
}

// Cred returns the default credential name for this session.
func (s *Session) Cred() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.credName
}

// Reset recreates the thread and globals (clears bindings).
func (s *Session) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.resetLocked()
}

func (s *Session) resetLocked() error {
	thread, globals, err := newThread(s.credName, s.opts, s.logger, s.trace, nil)
	if err != nil {
		return err
	}
	thread.SetLocal(localSession, true)
	s.thread = thread
	s.globals = globals
	return nil
}

// Eval executes src in the session. filename is used for error positions (e.g. "cell-1").
func (s *Session) Eval(filename, src string) EvalResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	var stdout, stderr bytes.Buffer
	s.thread.SetLocal(localStdout, &stdout)
	s.thread.SetLocal(localStderr, &stderr)
	defer func() {
		s.thread.SetLocal(localStdout, nil)
		s.thread.SetLocal(localStderr, nil)
	}()

	src = strings.TrimSpace(src)
	if src == "" {
		return EvalResult{}
	}
	if filename == "" {
		filename = "<cell>"
	}

	var out EvalResult

	// Prefer expression eval when the whole cell is a single expression (Jupyter-like).
	if expr, err := syntax.ParseExpr(filename, src, 0); err == nil {
		val, err := starlark.EvalExpr(s.thread, expr, s.globals)
		out.Stdout = stdout.String()
		out.Stderr = stderr.String()
		if err != nil {
			out.Error = formatEvalErr(err)
			return out
		}
		if val != nil && val != starlark.None {
			out.Result = val.String()
		}
		return out
	}

	globals, err := starlark.ExecFile(s.thread, filename, src, s.globals)
	out.Stdout = stdout.String()
	out.Stderr = stderr.String()
	if err != nil {
		out.Error = formatEvalErr(err)
		return out
	}
	s.globals = globals
	return out
}

// EvalOptions controls optional post-exec behavior.
type EvalOptions struct {
	// CallMain, if true, calls a top-level main() after a successful multi-statement
	// exec when result is still empty. Do not also invoke main() in the source.
	CallMain bool
}

// EvalOpts is like Eval with options (used by MCP starlark_eval).
func (s *Session) EvalOpts(filename, src string, opts EvalOptions) EvalResult {
	out := s.Eval(filename, src)
	if out.Error != "" || !opts.CallMain || out.Result != "" {
		return out
	}
	return s.CallMainAppend(out)
}

// CallMainAppend invokes globals["main"] if present and merges into a prior EvalResult.
func (s *Session) CallMainAppend(out EvalResult) EvalResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	mainFn := s.globals["main"]
	if mainFn == nil {
		return out
	}
	var stdout, stderr bytes.Buffer
	s.thread.SetLocal(localStdout, &stdout)
	s.thread.SetLocal(localStderr, &stderr)
	defer func() {
		s.thread.SetLocal(localStdout, nil)
		s.thread.SetLocal(localStderr, nil)
	}()
	val, err := starlark.Call(s.thread, mainFn, nil, nil)
	out.Stdout += stdout.String()
	out.Stderr += stderr.String()
	if err != nil {
		out.Error = formatEvalErr(err)
		return out
	}
	if val != nil && val != starlark.None {
		out.Result = val.String()
	}
	return out
}

func formatEvalErr(err error) string {
	var ee *ExitError
	if errors.As(err, &ee) {
		return fmt.Sprintf("exit %d", ee.Code)
	}
	return err.Error()
}

func writerFromLocal(thread *starlark.Thread, key string, fallback io.Writer) io.Writer {
	if thread == nil {
		return fallback
	}
	if v := thread.Local(key); v != nil {
		if w, ok := v.(io.Writer); ok && w != nil {
			return w
		}
	}
	return fallback
}

func inSession(thread *starlark.Thread) bool {
	if thread == nil {
		return false
	}
	v, _ := thread.Local(localSession).(bool)
	return v
}
