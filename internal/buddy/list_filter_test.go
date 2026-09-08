package buddy

import "testing"

func TestIsRSQLFilter(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want bool
	}{
		{"Chrome", false},
		{"Google Chrome", false},
		{`packageName=="*Chrome*"`, true},
		{"status==Pending", true},
		{"id!=1", true},
		{"dateCompleted=ge=2024-01-01T00:00:00Z", true},
		{"id=in=(1,2)", true},
		{"id=out=(1)", true},
		{"name=lt=z", true},
	}
	for _, tc := range cases {
		if got := isRSQLFilter(tc.in); got != tc.want {
			t.Errorf("isRSQLFilter(%q)=%v want %v", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeListFilter(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		filter string
		spec   ListFilterSpec
		want   string
	}{
		{"empty", "", FilterSpecPackages, ""},
		{"empty whitespace", "  ", FilterSpecPackages, ""},
		{"packages bare", "Chrome", FilterSpecPackages, `packageName=="*Chrome*"`},
		{"packages spaces", "Google Chrome", FilterSpecPackages, `packageName=="*Google Chrome*"`},
		{"packages rsql pass", `packageName=="*Chrome*";cloudTransferStatus=="READY"`, FilterSpecPackages, `packageName=="*Chrome*";cloudTransferStatus=="READY"`},
		{"computers bare", "MacBook", FilterSpecComputers, `general.name=="*MacBook*"`},
		{"scripts bare", "install", FilterSpecScripts, `name=="*install*"`},
		{"extension attributes bare", "Chrome", FilterSpecExtensionAttributes, `name=="*Chrome*"`},
		{"groups bare", "chrome", FilterSpecGroups, `name=="*chrome*"`},
		{"categories bare", "apps", FilterSpecCategories, `name=="*apps*"`},
		{"devices bare", "Mac", FilterSpecDevices, `name=="*Mac*"`},
		{"device groups bare", "lab", FilterSpecDeviceGroups, `name=="*lab*"`},
		{"ddm bare", "cfg", FilterSpecDDM, `name=="*cfg*"`},
		{"mdm bare eq", "Pending", FilterSpecMDM, `status=="Pending"`},
		{"mdm rsql pass", "status==Failed", FilterSpecMDM, "status==Failed"},
		{"roles bare", "Buddy", FilterSpecRoles, `displayName=="*Buddy*"`},
		{"clients bare", "buddy-cli", FilterSpecClients, `displayName=="*buddy-cli*"`},
		{"escape quotes", `lab "a"`, FilterSpecComputers, `general.name=="*lab \"a\"*"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeListFilter(tc.filter, tc.spec)
			if err != nil {
				t.Fatalf("NormalizeListFilter: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestNormalizeListFilter_MissingDefaultField(t *testing.T) {
	t.Parallel()
	if _, err := NormalizeListFilter("Chrome", ListFilterSpec{}); err == nil {
		t.Fatal("expected error for empty DefaultField")
	}
}
