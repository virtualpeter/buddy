package buddy

import (
	"strings"
	"testing"
)

func TestPrepareCategoryForCreate(t *testing.T) {
	src := &CategoryDetail{ID: "3", Name: "Apps", Priority: 9}
	out, err := PrepareCategoryForCreate(src, CategoryPrepareSpec{Name: "Apps Copy"})
	if err != nil {
		t.Fatal(err)
	}
	if out.ID != "" {
		t.Fatalf("id should be cleared, got %q", out.ID)
	}
	if out.Name != "Apps Copy" || out.Priority != 9 {
		t.Fatalf("got %+v", out)
	}
	p := 1
	out2, err := PrepareCategoryForCreate(src, CategoryPrepareSpec{Name: "X", Priority: &p})
	if err != nil {
		t.Fatal(err)
	}
	if out2.Priority != 1 {
		t.Fatalf("priority override: got %d", out2.Priority)
	}
}

func TestCategoryResolvedClassicXML(t *testing.T) {
	xml := CategoryResolved{ID: "5", Name: "Mac & Apps"}.CategoryClassicXML()
	if !strings.Contains(xml, "<id>5</id>") || !strings.Contains(xml, "Mac &amp; Apps") {
		t.Fatalf("unexpected xml: %s", xml)
	}
	xml2 := Uncategorized.CategoryClassicXML()
	if !strings.Contains(xml2, "<id>-1</id>") || !strings.Contains(xml2, "No category assigned") {
		t.Fatalf("uncategorized: %s", xml2)
	}
}
