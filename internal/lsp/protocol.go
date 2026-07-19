package lsp

import "encoding/json"

// Minimal LSP 3.17 types used by buddy lsp.

type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *respError      `json:"error,omitempty"`
}

type respError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type initializeParams struct {
	ProcessID             *int               `json:"processId"`
	InitializationOptions *initOptions       `json:"initializationOptions,omitempty"`
	Capabilities          clientCapabilities `json:"capabilities"`
}

type initOptions struct {
	CellMode bool `json:"cellMode"`
}

type clientCapabilities struct{}

type initializeResult struct {
	Capabilities serverCapabilities `json:"capabilities"`
	ServerInfo   *serverInfo        `json:"serverInfo,omitempty"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type serverCapabilities struct {
	TextDocumentSync       int                   `json:"textDocumentSync"` // 1 = Full
	CompletionProvider     *completionOptions    `json:"completionProvider,omitempty"`
	HoverProvider          bool                  `json:"hoverProvider"`
	SignatureHelpProvider  *signatureHelpOptions `json:"signatureHelpProvider,omitempty"`
	DocumentSymbolProvider bool                  `json:"documentSymbolProvider,omitempty"`
}

type completionOptions struct {
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
}

type signatureHelpOptions struct {
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
}

type textDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

type versionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

type textDocumentIdentifier struct {
	URI string `json:"uri"`
}

type position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type rangePos struct {
	Start position `json:"start"`
	End   position `json:"end"`
}

type textDocumentPositionParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     position               `json:"position"`
}

type didOpenParams struct {
	TextDocument textDocumentItem `json:"textDocument"`
}

type didChangeParams struct {
	TextDocument   versionedTextDocumentIdentifier `json:"textDocument"`
	ContentChanges []textDocumentContentChange     `json:"contentChanges"`
}

type textDocumentContentChange struct {
	Text string `json:"text"`
}

type didCloseParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
}

type publishDiagnosticsParams struct {
	URI         string          `json:"uri"`
	Diagnostics []lspDiagnostic `json:"diagnostics"`
}

type lspDiagnostic struct {
	Range    rangePos `json:"range"`
	Severity int      `json:"severity,omitempty"`
	Source   string   `json:"source,omitempty"`
	Message  string   `json:"message"`
}

type completionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []completionItem `json:"items"`
}

type completionItem struct {
	Label         string         `json:"label"`
	Kind          int            `json:"kind,omitempty"`
	Detail        string         `json:"detail,omitempty"`
	Documentation *markupContent `json:"documentation,omitempty"`
	InsertText    string         `json:"insertText,omitempty"`
}

type markupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type hover struct {
	Contents markupContent `json:"contents"`
	Range    *rangePos     `json:"range,omitempty"`
}

type signatureHelp struct {
	Signatures      []signatureInformation `json:"signatures"`
	ActiveSignature int                    `json:"activeSignature"`
	ActiveParameter int                    `json:"activeParameter"`
}

type signatureInformation struct {
	Label         string                 `json:"label"`
	Documentation *markupContent         `json:"documentation,omitempty"`
	Parameters    []parameterInformation `json:"parameters,omitempty"`
}

type parameterInformation struct {
	Label string `json:"label"`
}

// Completion item kinds (LSP).
const (
	completionItemFunction = 3
	completionItemVariable = 6
	completionItemModule   = 9
	completionItemKeyword  = 14
	completionItemConstant = 21
)
