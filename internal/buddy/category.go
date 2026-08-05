package buddy

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
)

const uriCategories = "/api/v1/categories"

// CategoryDetail is a Jamf Pro category from GET/POST/PUT /api/v1/categories.
type CategoryDetail struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name"`
	Priority int    `json:"priority"`
}

// CategoryResolved is the destination category id+name after ResolveCategoryOnDest.
type CategoryResolved struct {
	ID   string // empty or "-1" means uncategorized
	Name string // "No category assigned" when uncategorized
}

// Uncategorized is the Classic / Pro sentinel for no category.
var Uncategorized = CategoryResolved{ID: "-1", Name: "No category assigned"}

// CategorySearch lists categories (paginated). Plain filter = client-side name substring;
// RSQL filter is passed to the API.
func CategorySearch(credName, filter string, opts PageOpts, _ *slog.Logger) ([]CategoryDetail, error) {
	rsql := filter != "" && isRSQLFilter(filter)
	extra := url.Values{}
	if rsql {
		extra.Set("filter", filter)
	}
	var all []CategoryDetail
	err := fetchJamfPaged(credName, uriCategories, extra, opts, func(resultsJSON []byte, _ int) (int, error) {
		var page []CategoryDetail
		if err := json.Unmarshal(resultsJSON, &page); err != nil {
			return 0, fmt.Errorf("parse categories: %w", err)
		}
		all = append(all, page...)
		return len(page), nil
	})
	if err != nil {
		return nil, err
	}
	if filter != "" && !rsql {
		filterLower := strings.ToLower(strings.TrimSpace(filter))
		var out []CategoryDetail
		for _, c := range all {
			if strings.Contains(strings.ToLower(c.Name), filterLower) {
				out = append(out, c)
			}
		}
		return out, nil
	}
	return all, nil
}

// CategoryFetch loads one category by id.
func CategoryFetch(credName, id string) (*CategoryDetail, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("category id is required")
	}
	body, err := jamfGET(credName, uriCategories+"/"+id)
	if err != nil {
		return nil, err
	}
	var c CategoryDetail
	if err := json.Unmarshal(body, &c); err != nil {
		return nil, fmt.Errorf("parse category %s: %w", id, err)
	}
	return &c, nil
}

// CategoryFindIDByExactName returns the category id for an exact name match (case-sensitive like legacy lookup).
func CategoryFindIDByExactName(credName, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("category name is required")
	}
	cats, err := CategorySearch(credName, "", PageOpts{Page: PageAll}, nil)
	if err != nil {
		return "", err
	}
	for _, c := range cats {
		if c.Name == name {
			return c.ID, nil
		}
	}
	return "", fmt.Errorf("category %q not found", name)
}

// CategoryFindByExactName returns the full category for an exact name match.
func CategoryFindByExactName(credName, name string) (*CategoryDetail, bool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, false, fmt.Errorf("category name is required")
	}
	cats, err := CategorySearch(credName, "", PageOpts{Page: PageAll}, nil)
	if err != nil {
		return nil, false, err
	}
	for i := range cats {
		if cats[i].Name == name {
			return &cats[i], true, nil
		}
	}
	return nil, false, nil
}

// CategoryPost creates a category and returns the created id/href.
func CategoryPost(credName string, opts Opts, c CategoryDetail, logger *slog.Logger) (*GroupCreateResult, error) {
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return nil, fmt.Errorf("category name is required")
	}
	c.ID = "" // Jamf ignores ids on create
	payload, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("marshal category: %w", err)
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("\nCreate category %q (priority %d)?", c.Name, c.Priority), logger) {
			return nil, nil
		}
	}
	body, err := jamfPOST(credName, uriCategories, payload, "application/json")
	if err != nil {
		return nil, fmt.Errorf("create category: %w", err)
	}
	out, err := parseGroupCreateResult(body)
	if err != nil {
		return nil, err
	}
	fmt.Printf("Created category %q (ID %s)\n", c.Name, out.ID)
	return out, nil
}

// CategoryPut updates a category by id.
func CategoryPut(credName string, opts Opts, id string, c CategoryDetail, logger *slog.Logger) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("category id is required")
	}
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return fmt.Errorf("category name is required")
	}
	c.ID = id
	payload, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal category: %w", err)
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("\nUpdate category %s (%q)?", id, c.Name), logger) {
			return nil
		}
	}
	if err := jamfPUT(credName, uriCategories+"/"+id, payload, "application/json"); err != nil {
		return fmt.Errorf("update category %s: %w", id, err)
	}
	fmt.Printf("Updated category %s (%q)\n", id, c.Name)
	return nil
}

// CategoryDelete deletes a category by id.
func CategoryDelete(credName string, opts Opts, id string, logger *slog.Logger) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("category id is required")
	}
	name := id
	if c, err := CategoryFetch(credName, id); err == nil {
		name = fmt.Sprintf("%s (%q)", id, c.Name)
	}
	if !opts.SkipConfirm {
		if !ConfirmPrompt(fmt.Sprintf("\nDelete category %s?", name), logger) {
			return nil
		}
	}
	if err := jamfDELETE(credName, uriCategories+"/"+id); err != nil {
		return fmt.Errorf("delete category %s: %w", id, err)
	}
	fmt.Printf("Deleted category %s\n", id)
	return nil
}

// ResolveCategoryOnDest maps a source category name (or override id) onto the destination tenant.
// Missing names become Uncategorized (soft). When keepSourceID is set (same-session clone), that id is kept.
func ResolveCategoryOnDest(dstCred, sourceCategoryName, overrideCategoryID, keepSourceID string) (CategoryResolved, error) {
	if id := strings.TrimSpace(overrideCategoryID); id != "" {
		if id == "-1" {
			return Uncategorized, nil
		}
		c, err := CategoryFetch(dstCred, id)
		if err != nil {
			return CategoryResolved{}, fmt.Errorf("override category_id %q: %w", id, err)
		}
		return CategoryResolved{ID: c.ID, Name: c.Name}, nil
	}
	if keep := strings.TrimSpace(keepSourceID); keep != "" && keep != "-1" {
		name := strings.TrimSpace(sourceCategoryName)
		if name == "" {
			name = keep
		}
		return CategoryResolved{ID: keep, Name: name}, nil
	}
	name := strings.TrimSpace(sourceCategoryName)
	if name == "" || strings.EqualFold(name, "No category assigned") {
		return Uncategorized, nil
	}
	c, found, err := CategoryFindByExactName(dstCred, name)
	if err != nil {
		return CategoryResolved{}, err
	}
	if !found {
		return Uncategorized, nil
	}
	return CategoryResolved{ID: c.ID, Name: c.Name}, nil
}

// CategoryClassicXML returns Classic <category>…</category> for embedding in policy/profile XML.
func (c CategoryResolved) CategoryClassicXML() string {
	id := c.ID
	if id == "" {
		id = "-1"
	}
	name := c.Name
	if name == "" {
		name = Uncategorized.Name
	}
	idNum, err := strconv.Atoi(id)
	if err != nil {
		idNum = -1
	}
	return fmt.Sprintf("<category><id>%d</id><name>%s</name></category>", idNum, xmlEscapeText(name))
}

func xmlEscapeText(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
