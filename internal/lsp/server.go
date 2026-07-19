package lsp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
)

// Server is a buddy Starlark language server (stdio JSON-RPC / LSP).
type Server struct {
	Version string

	in  io.Reader
	out io.Writer

	mu       sync.Mutex
	docs     map[string]*document
	cellMode bool
	initDone bool
}

type document struct {
	URI  string
	Text string
}

// Run starts the LSP on stdin/stdout until the client disconnects or exits.
func Run(version string) error {
	s := &Server{
		Version: version,
		in:      os.Stdin,
		out:     os.Stdout,
		docs:    make(map[string]*document),
	}
	return s.serve(context.Background())
}

func (s *Server) serve(ctx context.Context) error {
	r := bufio.NewReader(s.in)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		msg, err := readMessage(r)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if err := s.handle(msg); err != nil {
			return err
		}
	}
}

func readMessage(r *bufio.Reader) (*message, error) {
	headers := map[string]string{}
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("lsp: bad header line %q", line)
		}
		headers[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(val)
	}
	n, err := strconv.Atoi(headers["content-length"])
	if err != nil || n < 0 {
		return nil, fmt.Errorf("lsp: missing Content-Length")
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	var msg message
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

func (s *Server) write(msg any) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err = fmt.Fprintf(s.out, "Content-Length: %d\r\n\r\n%s", len(body), body)
	return err
}

func (s *Server) reply(id json.RawMessage, result any) error {
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return s.write(message{
		JSONRPC: "2.0",
		ID:      id,
		Result:  raw,
	})
}

func (s *Server) replyNull(id json.RawMessage) error {
	return s.write(message{
		JSONRPC: "2.0",
		ID:      id,
		Result:  json.RawMessage("null"),
	})
}

func (s *Server) replyError(id json.RawMessage, code int, msg string) error {
	return s.write(message{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &respError{Code: code, Message: msg},
	})
}

func (s *Server) notify(method string, params any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return s.write(message{
		JSONRPC: "2.0",
		Method:  method,
		Params:  raw,
	})
}

func (s *Server) handle(msg *message) error {
	if msg.Method == "" {
		return nil // response we don't expect
	}
	switch msg.Method {
	case "initialize":
		return s.onInitialize(msg)
	case "initialized":
		return nil
	case "shutdown":
		return s.replyNull(msg.ID)
	case "exit":
		os.Exit(0)
		return nil
	case "textDocument/didOpen":
		return s.onDidOpen(msg)
	case "textDocument/didChange":
		return s.onDidChange(msg)
	case "textDocument/didClose":
		return s.onDidClose(msg)
	case "textDocument/completion":
		return s.onCompletion(msg)
	case "textDocument/hover":
		return s.onHover(msg)
	case "textDocument/signatureHelp":
		return s.onSignatureHelp(msg)
	case "$/cancelRequest":
		return nil
	default:
		if msg.ID != nil {
			return s.replyError(msg.ID, -32601, "method not found: "+msg.Method)
		}
		return nil
	}
}

func (s *Server) onInitialize(msg *message) error {
	var params initializeParams
	if len(msg.Params) > 0 {
		_ = json.Unmarshal(msg.Params, &params)
	}
	if params.InitializationOptions != nil {
		s.cellMode = params.InitializationOptions.CellMode
	}
	s.initDone = true
	ver := s.Version
	if ver == "" {
		ver = "dev"
	}
	return s.reply(msg.ID, initializeResult{
		Capabilities: serverCapabilities{
			TextDocumentSync: 1,
			CompletionProvider: &completionOptions{
				TriggerCharacters: []string{".", "\"", "'"},
			},
			HoverProvider: true,
			SignatureHelpProvider: &signatureHelpOptions{
				TriggerCharacters: []string{"(", ","},
			},
		},
		ServerInfo: &serverInfo{Name: "buddy-lsp", Version: ver},
	})
}

func (s *Server) mode() AnalyzeMode {
	if s.cellMode {
		return ModeCell
	}
	return ModeFile
}

func (s *Server) setDoc(uri, text string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.docs[uri] = &document{URI: uri, Text: text}
}

func (s *Server) getDoc(uri string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.docs[uri]
	if !ok {
		return "", false
	}
	return d.Text, true
}

func (s *Server) deleteDoc(uri string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.docs, uri)
}

func (s *Server) onDidOpen(msg *message) error {
	var params didOpenParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return nil
	}
	s.setDoc(params.TextDocument.URI, params.TextDocument.Text)
	return s.publishDiags(params.TextDocument.URI, params.TextDocument.Text)
}

func (s *Server) onDidChange(msg *message) error {
	var params didChangeParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return nil
	}
	if len(params.ContentChanges) == 0 {
		return nil
	}
	// Full sync only.
	text := params.ContentChanges[len(params.ContentChanges)-1].Text
	s.setDoc(params.TextDocument.URI, text)
	return s.publishDiags(params.TextDocument.URI, text)
}

func (s *Server) onDidClose(msg *message) error {
	var params didCloseParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return nil
	}
	s.deleteDoc(params.TextDocument.URI)
	return s.notify("textDocument/publishDiagnostics", publishDiagnosticsParams{
		URI:         params.TextDocument.URI,
		Diagnostics: []lspDiagnostic{},
	})
}

func (s *Server) publishDiags(uri, text string) error {
	filename := uriToPath(uri)
	diags := Analyze(filename, text, s.mode())
	out := make([]lspDiagnostic, 0, len(diags))
	for _, d := range diags {
		out = append(out, lspDiagnostic{
			Range: rangePos{
				Start: position{Line: d.Line, Character: d.Col},
				End:   position{Line: d.EndLine, Character: d.EndCol},
			},
			Severity: d.Severity,
			Source:   "buddy",
			Message:  d.Message,
		})
	}
	return s.notify("textDocument/publishDiagnostics", publishDiagnosticsParams{
		URI:         uri,
		Diagnostics: out,
	})
}

func (s *Server) onCompletion(msg *message) error {
	var params textDocumentPositionParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return s.replyNull(msg.ID)
	}
	text, ok := s.getDoc(params.TextDocument.URI)
	if !ok {
		return s.reply(msg.ID, completionList{})
	}
	cat, err := getCatalog()
	if err != nil {
		return s.replyError(msg.ID, -32603, err.Error())
	}

	// load("… completion
	if modPrefix, inLoad, loadOK := loadStringPrefix(text, params.Position.Line, params.Position.Character); loadOK && inLoad {
		items := []completionItem{}
		for _, m := range cat.loadModules {
			if modPrefix == "" || strings.HasPrefix(m, modPrefix) {
				items = append(items, completionItem{
					Label:      m,
					Kind:       completionItemModule,
					InsertText: m,
					Detail:     "buddy load module",
				})
			}
		}
		return s.reply(msg.ID, completionList{Items: items})
	}

	prefix, _ := wordAt(text, params.Position.Line, params.Position.Character)
	syms := cat.filterCompletion(prefix)
	items := make([]completionItem, 0, len(syms))
	for _, sym := range syms {
		kind := completionItemFunction
		switch sym.Kind {
		case "predeclared", "universe":
			kind = completionItemVariable
		case "stdlib":
			kind = completionItemModule
		}
		item := completionItem{
			Label:      sym.Name,
			Kind:       kind,
			Detail:     sym.Signature,
			InsertText: sym.Name,
		}
		if sym.Detail != "" {
			item.Documentation = &markupContent{Kind: "markdown", Value: hoverMarkdown(sym)}
		}
		items = append(items, item)
	}
	return s.reply(msg.ID, completionList{Items: items})
}

func (s *Server) onHover(msg *message) error {
	var params textDocumentPositionParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return s.replyNull(msg.ID)
	}
	text, ok := s.getDoc(params.TextDocument.URI)
	if !ok {
		return s.replyNull(msg.ID)
	}
	prefix, startCol := wordAt(text, params.Position.Line, params.Position.Character)
	if prefix == "" {
		return s.replyNull(msg.ID)
	}
	// Full word at cursor (may extend past character).
	full, _ := fullWordAt(text, params.Position.Line, params.Position.Character)
	cat, err := getCatalog()
	if err != nil {
		return s.replyError(msg.ID, -32603, err.Error())
	}
	sym, ok := cat.lookup(full)
	if !ok {
		return s.replyNull(msg.ID)
	}
	endCol := startCol + len([]rune(full))
	rng := rangePos{
		Start: position{Line: params.Position.Line, Character: startCol},
		End:   position{Line: params.Position.Line, Character: endCol},
	}
	return s.reply(msg.ID, hover{
		Contents: markupContent{Kind: "markdown", Value: hoverMarkdown(sym)},
		Range:    &rng,
	})
}

func (s *Server) onSignatureHelp(msg *message) error {
	var params textDocumentPositionParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return s.replyNull(msg.ID)
	}
	text, ok := s.getDoc(params.TextDocument.URI)
	if !ok {
		return s.replyNull(msg.ID)
	}
	name, argIndex, ok := callAt(text, params.Position.Line, params.Position.Character)
	if !ok {
		return s.replyNull(msg.ID)
	}
	cat, err := getCatalog()
	if err != nil {
		return s.replyError(msg.ID, -32603, err.Error())
	}
	sym, ok := cat.lookup(name)
	if !ok {
		return s.replyNull(msg.ID)
	}
	paramsList := signatureParams(sym.Signature)
	info := signatureInformation{
		Label: signatureLabel(sym),
	}
	if sym.Detail != "" {
		info.Documentation = &markupContent{Kind: "markdown", Value: sym.Detail}
	}
	for _, p := range paramsList {
		info.Parameters = append(info.Parameters, parameterInformation{Label: p})
	}
	if argIndex >= len(paramsList) && len(paramsList) > 0 {
		argIndex = len(paramsList) - 1
	}
	return s.reply(msg.ID, signatureHelp{
		Signatures:      []signatureInformation{info},
		ActiveSignature: 0,
		ActiveParameter: argIndex,
	})
}

func fullWordAt(src string, line, character int) (string, int) {
	lines := strings.Split(src, "\n")
	if line < 0 || line >= len(lines) {
		return "", character
	}
	runes := []rune(lines[line])
	if character > len(runes) {
		character = len(runes)
	}
	i := character
	if i < len(runes) && isIdentPart(runes[i]) {
		// inside word — extend right later
	} else if i > 0 && isIdentPart(runes[i-1]) {
		i--
	}
	start := i
	for start > 0 && isIdentPart(runes[start-1]) {
		start--
	}
	end := start
	for end < len(runes) && isIdentPart(runes[end]) {
		end++
	}
	if start >= end {
		return "", character
	}
	return string(runes[start:end]), start
}

// loadStringPrefix reports whether the cursor is inside a load("…") module string.
func loadStringPrefix(src string, line, character int) (prefix string, inLoad bool, ok bool) {
	lines := strings.Split(src, "\n")
	if line < 0 || line >= len(lines) {
		return "", false, false
	}
	row := lines[line]
	runes := []rune(row)
	if character > len(runes) {
		character = len(runes)
	}
	before := string(runes[:character])
	// Match load( then optional whitespace then quote then prefix
	idx := strings.LastIndex(before, "load(")
	if idx < 0 {
		return "", false, false
	}
	rest := strings.TrimSpace(before[idx+len("load("):])
	if rest == "" {
		return "", true, true
	}
	quote := rune(0)
	if strings.HasPrefix(rest, "\"") {
		quote = '"'
		rest = rest[1:]
	} else if strings.HasPrefix(rest, "'") {
		quote = '\''
		rest = rest[1:]
	} else {
		return "", false, false
	}
	if strings.ContainsRune(rest, quote) {
		return "", false, false // already closed
	}
	return rest, true, true
}

func uriToPath(uri string) string {
	const prefix = "file://"
	if strings.HasPrefix(uri, prefix) {
		p := strings.TrimPrefix(uri, prefix)
		// file:///Users/... on macOS
		if strings.HasPrefix(p, "/") && len(p) > 2 && p[2] == ':' {
			// Windows file:///C:/...
			return p[1:]
		}
		return p
	}
	return uri
}

// ServeForTest runs the server against in/out (used by tests).
func ServeForTest(ctx context.Context, in io.Reader, out io.Writer, version string) error {
	s := &Server{
		Version: version,
		in:      in,
		out:     out,
		docs:    make(map[string]*document),
	}
	return s.serve(ctx)
}

// EncodeMessageForTest builds a framed LSP message body for tests.
func EncodeMessageForTest(v any) []byte {
	body, _ := json.Marshal(v)
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Content-Length: %d\r\n\r\n%s", len(body), body)
	return buf.Bytes()
}
