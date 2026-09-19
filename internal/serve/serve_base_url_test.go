package serve

import "testing"

func TestServeBaseURL(t *testing.T) {
	tests := []struct {
		listen, want string
	}{
		{"127.0.0.1:8742", "http://127.0.0.1:8742"},
		{"127.0.0.1:0", "http://127.0.0.1:0"},
		{"[::]:8742", "http://127.0.0.1:8742"},
		{"0.0.0.0:9000", "http://127.0.0.1:9000"},
	}
	for _, tc := range tests {
		if got := serveBaseURL(tc.listen); got != tc.want {
			t.Errorf("serveBaseURL(%q) = %q, want %q", tc.listen, got, tc.want)
		}
	}
}
