package buddy

import (
	"html"
	"strings"
)

// XMLEscape escapes a string for embedding as XML element text content
// (e.g. nesting plist/JSON/XML inside another XML element).
// Escapes &, <, and >. Does not escape quotes (not required for element content).
func XMLEscape(s string) string {
	return xmlEncodePayload(s)
}

// XMLUnescape reverses XML/HTML character entity references (&amp; &lt; &gt; &quot;
// &#39; and numeric entities). Suitable for undelimiting nested XML/JSON/plist
// that was entity-encoded inside an outer XML document.
func XMLUnescape(s string) string {
	return html.UnescapeString(s)
}

// xmlEncodePayload escapes a string for embedding as XML element content.
// & must be replaced before < and > to avoid double-encoding.
func xmlEncodePayload(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
