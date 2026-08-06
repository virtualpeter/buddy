package main

import "testing"

func TestTakeFlagValue(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		args    []string
		i       int
		flag    string
		wantVal string
		wantNext int
		wantMatch bool
		wantErr bool
	}{
		{"equals", []string{"--filter=Nitro"}, 0, "filter", "Nitro", 1, true, false},
		{"space", []string{"--filter", "Nitro"}, 0, "filter", "Nitro", 2, true, false},
		{"space then other", []string{"--filter", "Nitro", "--ids"}, 0, "filter", "Nitro", 2, true, false},
		{"missing value", []string{"--filter"}, 0, "filter", "", 0, true, true},
		{"missing value next flag", []string{"--filter", "--ids"}, 0, "filter", "", 0, true, true},
		{"not this flag", []string{"--sort=version"}, 0, "filter", "", 0, false, false},
		{"empty equals", []string{"--filter="}, 0, "filter", "", 1, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			val, next, matched, err := takeFlagValue(tc.args, tc.i, tc.flag)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if matched != tc.wantMatch {
				t.Fatalf("matched=%v want %v", matched, tc.wantMatch)
			}
			if !tc.wantErr && matched {
				if val != tc.wantVal || next != tc.wantNext {
					t.Fatalf("val=%q next=%d want %q %d", val, next, tc.wantVal, tc.wantNext)
				}
			}
		})
	}
}

func TestExtractStringFlag(t *testing.T) {
	t.Parallel()
	val, rest, err := extractStringFlag([]string{"--filter", "Nitro", "extra"}, "filter")
	if err != nil {
		t.Fatal(err)
	}
	if val != "Nitro" {
		t.Fatalf("val=%q", val)
	}
	if len(rest) != 1 || rest[0] != "extra" {
		t.Fatalf("rest=%v", rest)
	}
	val, rest, err = extractStringFlag([]string{"--filter=Chrome", "--sort=version"}, "filter")
	if err != nil {
		t.Fatal(err)
	}
	if val != "Chrome" || len(rest) != 1 || rest[0] != "--sort=version" {
		t.Fatalf("val=%q rest=%v", val, rest)
	}
}
