package script

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strings"
	"sync"

	"buddy/internal/buddy"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

const (
	localStdout  = "buddy.stdout"
	localStderr  = "buddy.stderr"
	localSession = "buddy.session"
)

// EvalResult is the outcome of evaluating one cell or snippet.
type EvalResult struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
	Result string `json:"result,omitempty"` // last expression value, if any
	Error  string `json:"error,omitempty"`
}

// Session is a persistent Starlark environment (REPL / notebook / MCP eval / IDE).
type Session struct {
	mu       sync.Mutex
	credName string
	opts     buddy.Opts
	logger   *slog.Logger
	trace    bool
	args     []string
	thread   *starlark.Thread
	globals  starlark.StringDict
	confirm  *confirmBridge
}

// NewSession creates a Starlark session with buddy builtins.
func NewSession(credName string, opts buddy.Opts, logger *slog.Logger, trace bool) (*Session, error) {
	return NewSessionArgs(credName, opts, logger, trace, nil)
}

// NewSessionArgs is like NewSession with initial script args (global args list).
func NewSessionArgs(credName string, opts buddy.Opts, logger *slog.Logger, trace bool, args []string) (*Session, error) {
	s := &Session{
		credName: credName,
		opts:     opts,
		logger:   logger,
		trace:    trace,
		args:     append([]string(nil), args...),
		confirm:  newConfirmBridge(),
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

// Trace reports whether builtin call tracing is enabled.
func (s *Session) Trace() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.trace
}

// Args returns a copy of the session script args.
func (s *Session) Args() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.args...)
}

// SetArgs updates the global args list (and the Starlark args binding).
func (s *Session) SetArgs(args []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.args = append([]string(nil), args...)
	if s.globals != nil {
		s.globals["args"] = listFromStrings(s.args)
	}
}

// SetTrace enables/disables builtin tracing and recreates the thread/globals
// (bindings cleared; args preserved).
func (s *Session) SetTrace(trace bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.trace = trace
	return s.resetLocked()
}

// Reset recreates the thread and globals (clears bindings; keeps args/trace).
func (s *Session) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.resetLocked()
}

func (s *Session) resetLocked() error {
	thread, globals, err := newThread(s.credName, s.opts, s.logger, s.trace, s.args)
	if err != nil {
		return err
	}
	thread.SetLocal(localSession, true)
	s.thread = thread
	s.globals = globals
	return nil
}

// PendingConfirm returns the in-flight confirm prompt, if any (notebook/IDE poll).
func (s *Session) PendingConfirm() *PendingConfirm {
	if s == nil {
		return nil
	}
	return s.confirm.snapshot()
}

// AnswerConfirm delivers the user's answer token for a pending confirm.
func (s *Session) AnswerConfirm(answer string) error {
	if s == nil {
		return fmt.Errorf("nil session")
	}
	return s.confirm.reply(answer)
}

func (s *Session) pushConfirmAsk() (restore func()) {
	if s.opts.SkipConfirm {
		return func() {}
	}
	return buddy.PushConfirmAsk(s.confirm.ask)
}

// Eval executes src in the session. filename is used for error positions (e.g. "cell-1").
// Multi-statement cells behave like a notebook: statements run for effect, and if the
// last statement is an expression its value is returned in Result (None is omitted).
func (s *Session) Eval(filename, src string) EvalResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	restore := s.pushConfirmAsk()
	defer restore()

	var stdout, stderr bytes.Buffer
	s.thread.SetLocal(localStdout, &stdout)
	s.thread.SetLocal(localStderr, &stderr)
	defer func() {
		s.thread.SetLocal(localStdout, nil)
		s.thread.SetLocal(localStderr, nil)
	}()

	src = strings.TrimSpace(src)
	src = stripTrailingNoise(src)
	if src == "" {
		return EvalResult{}
	}
	if filename == "" {
		filename = "<cell>"
	}

	var out EvalResult
	finish := func() {
		out.Stdout = stdout.String()
		out.Stderr = stderr.String()
	}

	opts := &syntax.FileOptions{
		Set:             true,
		TopLevelControl: true, // allow if/for at cell top level (notebook UX)
		GlobalReassign:  true,
	}

	f, err := opts.Parse(filename, src, 0)
	if err != nil {
		finish()
		out.Error = formatEvalErr(err)
		return out
	}

	lastExpr := peelLastExpr(f)

	if len(f.Stmts) > 0 {
		if err := starlark.ExecREPLChunk(f, s.thread, s.globals); err != nil {
			finish()
			out.Error = formatEvalErr(err)
			return out
		}
	}

	if lastExpr != nil {
		val, err := starlark.EvalExprOptions(opts, s.thread, lastExpr, s.globals)
		finish()
		if err != nil {
			out.Error = formatEvalErr(err)
			return out
		}
		if val != nil && val != starlark.None {
			out.Result = FormatDisplayValue(val)
			s.globals["_"] = val
		}
		return out
	}

	finish()
	return out
}

// EvalOptions controls optional post-exec behavior.
type EvalOptions struct {
	// CallMain, if true, calls a top-level main() after a successful multi-statement
	// exec when result is still empty. Do not also invoke main() in the source.
	CallMain bool
}

// EvalOpts is like Eval with options (used by MCP buddyscript).
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
	return s.callMainLocked(out)
}

func (s *Session) callMainLocked(out EvalResult) EvalResult {
	mainFn := s.globals["main"]
	if mainFn == nil {
		return out
	}
	restore := s.pushConfirmAsk()
	defer restore()
	var stdout, stderr bytes.Buffer
	s.thread.SetLocal(localStdout, &stdout)
	s.thread.SetLocal(localStderr, &stderr)
	defer func() {
		s.thread.SetLocal(localStdout, nil)
		s.thread.SetLocal(localStderr, nil)
	}()
	val, err := callScriptMain(s.thread, mainFn, s.credName)
	out.Stdout += stdout.String()
	out.Stderr += stderr.String()
	if err != nil {
		out.Error = formatEvalErr(err)
		return out
	}
	if val != nil && val != starlark.None {
		out.Result = FormatDisplayValue(val)
	}
	return out
}

// RunScript executes a full script buffer using file dialect (like buddy <path>),
// then calls main() if defined. Returns eval streams plus a globals snapshot.
func (s *Session) RunScript(filename, src string) (EvalResult, []GlobalVar) {
	s.mu.Lock()
	defer s.mu.Unlock()
	restore := s.pushConfirmAsk()
	defer restore()

	var stdout, stderr bytes.Buffer
	s.thread.SetLocal(localStdout, &stdout)
	s.thread.SetLocal(localStderr, &stderr)
	defer func() {
		s.thread.SetLocal(localStdout, nil)
		s.thread.SetLocal(localStderr, nil)
	}()

	var out EvalResult
	finish := func() {
		out.Stdout = stdout.String()
		out.Stderr = stderr.String()
	}
	if filename == "" {
		filename = "<script>"
	}
	src = strings.ReplaceAll(src, "\r\n", "\n")
	if strings.HasPrefix(src, "#!") {
		if i := strings.IndexByte(src, '\n'); i >= 0 {
			src = src[i+1:]
		} else {
			src = ""
		}
	}

	fileOpts := &syntax.FileOptions{} // file dialect (no top-level if/for)
	f, err := fileOpts.Parse(filename, src, 0)
	if err != nil {
		finish()
		out.Error = formatEvalErr(err)
		return out, s.inspectGlobalsLocked()
	}
	if err := starlark.ExecREPLChunk(f, s.thread, s.globals); err != nil {
		finish()
		out.Error = formatEvalErr(err)
		return out, s.inspectGlobalsLocked()
	}
	finish()
	out = s.callMainLocked(out)
	return out, s.inspectGlobalsLocked()
}

// GlobalVar is one session global for the IDE variables panel.
type GlobalVar struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value string `json:"value"`
}

// InspectGlobals returns user-visible globals (skips builtins and the re module).
func (s *Session) InspectGlobals() []GlobalVar {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inspectGlobalsLocked()
}

func (s *Session) inspectGlobalsLocked() []GlobalVar {
	if s.globals == nil {
		return nil
	}
	names := make([]string, 0, len(s.globals))
	for k, v := range s.globals {
		if k == "" || k == "re" {
			continue
		}
		if _, ok := v.(*starlark.Builtin); ok {
			continue
		}
		names = append(names, k)
	}
	sort.Strings(names)
	out := make([]GlobalVar, 0, len(names))
	for _, k := range names {
		v := s.globals[k]
		out = append(out, GlobalVar{
			Name:  k,
			Type:  v.Type(),
			Value: FormatDisplayValue(v),
		})
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
