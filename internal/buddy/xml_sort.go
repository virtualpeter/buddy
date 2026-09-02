package buddy

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/beevik/etree"
)

// XMLSortOpts controls xml_sort behavior.
type XMLSortOpts struct {
	// Element is the parent whose children are sorted. A bare tag name matches
	// that element (see Recursive). A path containing "/" is an etree path
	// (e.g. "./root/items" or "//PayloadContent"). Empty means the document root.
	Element string

	// Key is a child element tag whose text is the sort key. Also supports
	// plist-style <key>Key</key><string>value</string> pairs when no child
	// element named Key exists.
	Key string

	// Attr is an attribute name on each child element to sort by.
	// If both Key and Attr are set, Attr wins.
	Attr string

	// Only, when non-empty, limits sorting to child elements with this tag;
	// other children keep their relative positions.
	Only string

	// Order is "asc" (default) or "desc".
	Order string

	// IgnoreCase compares sort keys case-insensitively.
	IgnoreCase bool

	// Numeric prefers numeric comparison when both keys parse as numbers.
	Numeric bool

	// Recursive, when Element is a bare tag name, sorts children of every
	// matching element in the document (//tag). Ignored for path Elements.
	Recursive bool
}

// XMLSort parses xmlStr, sorts child elements under the selected parent(s),
// and returns the serialized XML.
func XMLSort(xmlStr string, opts XMLSortOpts) (string, error) {
	doc := etree.NewDocument()
	if err := doc.ReadFromString(xmlStr); err != nil {
		return "", fmt.Errorf("parse xml: %w", err)
	}

	order := strings.ToLower(strings.TrimSpace(opts.Order))
	if order == "" {
		order = "asc"
	}
	if order != "asc" && order != "desc" {
		return "", fmt.Errorf("order must be \"asc\" or \"desc\", got %q", opts.Order)
	}
	desc := order == "desc"

	parents, err := xmlSortParents(doc, opts)
	if err != nil {
		return "", err
	}
	if len(parents) == 0 {
		return "", fmt.Errorf("no element matching %q", strings.TrimSpace(opts.Element))
	}

	for _, parent := range parents {
		xmlSortChildren(parent, opts, desc)
	}

	out, err := doc.WriteToString()
	if err != nil {
		return "", fmt.Errorf("serialize xml: %w", err)
	}
	return out, nil
}

func xmlSortParents(doc *etree.Document, opts XMLSortOpts) ([]*etree.Element, error) {
	el := strings.TrimSpace(opts.Element)
	if el == "" {
		root := doc.Root()
		if root == nil {
			return nil, fmt.Errorf("xml has no root element")
		}
		return []*etree.Element{root}, nil
	}
	if strings.Contains(el, "/") {
		return doc.FindElements(el), nil
	}
	if opts.Recursive {
		return doc.FindElements("//" + el), nil
	}
	if root := doc.Root(); root != nil {
		if root.Tag == el {
			return []*etree.Element{root}, nil
		}
		if c := root.SelectElement(el); c != nil {
			return []*etree.Element{c}, nil
		}
	}
	found := doc.FindElements("//" + el)
	if len(found) == 0 {
		return nil, nil
	}
	return []*etree.Element{found[0]}, nil
}

func xmlSortChildren(parent *etree.Element, opts XMLSortOpts, desc bool) {
	match := func(el *etree.Element) bool {
		if opts.Only != "" && el.Tag != opts.Only {
			return false
		}
		return true
	}

	var sortable []*etree.Element
	for _, tok := range parent.Child {
		el, ok := tok.(*etree.Element)
		if !ok || !match(el) {
			continue
		}
		sortable = append(sortable, el)
	}
	if len(sortable) < 2 {
		return
	}

	sorted := append([]*etree.Element(nil), sortable...)
	sort.SliceStable(sorted, func(i, j int) bool {
		ki := xmlSortKey(sorted[i], opts)
		kj := xmlSortKey(sorted[j], opts)
		cmp := xmlCompareKeys(ki, kj, opts)
		if desc {
			return cmp > 0
		}
		return cmp < 0
	})

	// Rebuild children: detach all, then re-add with sorted elements in slot order.
	old := append([]etree.Token(nil), parent.Child...)
	for len(parent.Child) > 0 {
		parent.RemoveChildAt(0)
	}
	si := 0
	for _, tok := range old {
		el, ok := tok.(*etree.Element)
		if ok && match(el) {
			parent.AddChild(sorted[si])
			si++
			continue
		}
		parent.AddChild(tok)
	}
}

func xmlSortKey(el *etree.Element, opts XMLSortOpts) string {
	if opts.Attr != "" {
		return strings.TrimSpace(el.SelectAttrValue(opts.Attr, ""))
	}
	if opts.Key == "" {
		return el.Tag
	}
	if c := el.SelectElement(opts.Key); c != nil {
		return strings.TrimSpace(c.Text())
	}
	// Plist-style dict: <key>Opts.Key</key><string>…</string> (or integer/real/…).
	children := el.ChildElements()
	for i := 0; i+1 < len(children); i++ {
		if children[i].Tag == "key" && children[i].Text() == opts.Key {
			return strings.TrimSpace(children[i+1].Text())
		}
	}
	return ""
}

func xmlCompareKeys(a, b string, opts XMLSortOpts) int {
	if opts.IgnoreCase {
		a = strings.ToLower(a)
		b = strings.ToLower(b)
	}
	if opts.Numeric {
		fa, ea := strconv.ParseFloat(a, 64)
		fb, eb := strconv.ParseFloat(b, 64)
		if ea == nil && eb == nil {
			switch {
			case fa < fb:
				return -1
			case fa > fb:
				return 1
			default:
				return 0
			}
		}
	}
	return strings.Compare(a, b)
}
