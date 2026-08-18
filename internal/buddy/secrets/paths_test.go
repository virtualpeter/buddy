package secrets

import (
	"path/filepath"
	"testing"
)

func TestResolveConfigDir(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name               string
		goos               string
		xdg, app, home, uc string
		want               string
		wantErr            bool
	}{
		{
			name: "xdg wins on darwin",
			goos: "darwin", xdg: "/xdg", home: "/Users/me",
			want: filepath.Join("/xdg", "buddy"),
		},
		{
			name: "xdg wins on windows",
			goos: "windows", xdg: `D:\xdg`, app: `C:\Users\me\AppData\Roaming`,
			want: filepath.Join(`D:\xdg`, "buddy"),
		},
		{
			name: "darwin default",
			goos: "darwin", home: "/Users/me",
			want: filepath.Join("/Users/me", ".config", "buddy"),
		},
		{
			name: "linux default",
			goos: "linux", home: "/home/me",
			want: filepath.Join("/home/me", ".config", "buddy"),
		},
		{
			name: "windows APPDATA",
			goos: "windows", app: `C:\Users\me\AppData\Roaming`,
			want: filepath.Join(`C:\Users\me\AppData\Roaming`, "buddy"),
		},
		{
			name: "windows UserConfigDir fallback",
			goos: "windows", uc: `C:\Users\me\AppData\Roaming`,
			want: filepath.Join(`C:\Users\me\AppData\Roaming`, "buddy"),
		},
		{
			name: "windows missing",
			goos: "windows", wantErr: true,
		},
		{
			name: "unix missing home",
			goos: "linux", wantErr: true,
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveConfigDir(tt.goos, tt.xdg, tt.app, tt.home, tt.uc)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("got %q, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestTokensAndSecretsDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg, err := ConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	tok, err := TokensDir()
	if err != nil {
		t.Fatal(err)
	}
	sec, err := SecretsDir()
	if err != nil {
		t.Fatal(err)
	}
	if tok != filepath.Join(cfg, "tokens") || sec != filepath.Join(cfg, "secrets") {
		t.Fatalf("cfg=%q tok=%q sec=%q", cfg, tok, sec)
	}
	caps, err := CapsDir()
	if err != nil {
		t.Fatal(err)
	}
	if caps != filepath.Join(cfg, "caps") {
		t.Fatalf("caps=%q", caps)
	}
}
