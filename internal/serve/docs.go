package serve

import (
	"buddy/internal/docpages"
)

// DocPage is a navigable documentation page (alias of docpages.Page for serve).
type DocPage = docpages.Page

func loadDocPages(docsRoot string) ([]DocPage, string, error) {
	res, err := docpages.Load(docpages.LoadOptions{
		Catalog:    docpages.CatalogHuman,
		DocsRoot:   docsRoot,
		PreferDisk: true, // in-repo `buddy serve` picks up live markdown when present
	})
	if err != nil {
		return nil, "", err
	}
	return res.Pages, res.Source, nil
}

func pageByID(pages []DocPage, id string) *DocPage {
	return docpages.ByID(pages, id)
}
