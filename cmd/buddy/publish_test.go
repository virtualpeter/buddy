package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"buddy/internal/buddy"
)

func TestRequireCleanGitDirty(t *testing.T) {
	orig := runGit
	t.Cleanup(func() { runGit = orig })
	runGit = func(args ...string) (string, error) {
		if len(args) >= 2 && args[0] == "status" && args[1] == "--porcelain" {
			return " M README.md\n", nil
		}
		t.Fatalf("unexpected git %v", args)
		return "", nil
	}
	err := requireCleanGit()
	if err == nil || !strings.Contains(err.Error(), "dirty") {
		t.Fatalf("got %v", err)
	}
}

func TestRequireCleanGitClean(t *testing.T) {
	orig := runGit
	t.Cleanup(func() { runGit = orig })
	runGit = func(args ...string) (string, error) {
		if len(args) >= 1 && args[0] == "status" {
			return "", nil
		}
		t.Fatalf("unexpected git %v", args)
		return "", nil
	}
	if err := requireCleanGit(); err != nil {
		t.Fatal(err)
	}
}

func TestDistAssetsSkipsAppBundles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "buddy-1.0.0.pkg"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "buddy-linux-amd64"), []byte("x"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "Buddy.app", "Contents"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Buddy.app", "Contents", "Info.plist"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "BuddyAgent.app"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".DS_Store"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "buddy"), []byte("x"), 0755); err != nil {
		t.Fatal(err)
	}
	files, err := distAssets(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("files = %v", files)
	}
	for _, f := range files {
		base := filepath.Base(f)
		if base == "Buddy.app" || base == "BuddyAgent.app" || base == ".DS_Store" || base == "buddy" {
			t.Fatalf("should skip app bundles, publisher CLI, and dotfiles: %v", files)
		}
	}
}

func TestDistAssetsEmpty(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "Buddy.app"), 0755); err != nil {
		t.Fatal(err)
	}
	_, err := distAssets(dir)
	if err == nil || !strings.Contains(err.Error(), "no regular files") {
		t.Fatalf("got %v", err)
	}
}

func TestReleaseNotesForTag(t *testing.T) {
	md := "# Release notes\n\n## v1.2.3\n\nHello **there**.\n\n### Detail\n\nmore\n\n## v1.2.2\n\nOld.\n"
	got, err := releaseNotesForTag(md, "v1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "Hello **there**") || !strings.Contains(got, "### Detail") {
		t.Fatalf("body = %q", got)
	}
	if strings.Contains(got, "Old.") {
		t.Fatalf("leaked next section: %q", got)
	}
	got, err = releaseNotesForTag(md, "1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "Hello") {
		t.Fatalf("bare tag should match ## v1.2.3: %q", got)
	}
	if _, err := releaseNotesForTag(md, "v9.9.9"); err == nil {
		t.Fatal("expected missing-section error")
	}
}

func TestGitHubRepoFromURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"git@github.com:acme/buddy.git", "acme/buddy", true},
		{"https://github.com/acme/buddy.git", "acme/buddy", true},
		{"https://github.com/acme/buddy", "acme/buddy", true},
		{"ssh://git@github.com/acme/buddy.git", "acme/buddy", true},
		{"https://gitlab.com/acme/buddy.git", "", false},
		{"acme/buddy", "", false},
	}
	for _, tc := range cases {
		got, ok := githubRepoFromURL(tc.in)
		if ok != tc.ok || got != tc.want {
			t.Errorf("%q: got %q %v want %q %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestRunPublishDirtyRefusesGH(t *testing.T) {
	origGit, origGH, origLook := runGit, runGH, lookPath
	t.Cleanup(func() {
		runGit, runGH, lookPath = origGit, origGH, origLook
	})
	called := false
	runGit = func(args ...string) (string, error) {
		if len(args) >= 1 && args[0] == "status" {
			return "?? scratch.txt\n", nil
		}
		t.Fatalf("git should stop at dirty; got %v", args)
		return "", nil
	}
	runGH = func(args []string) error {
		called = true
		return nil
	}
	lookPath = func(string) (string, error) { return "/usr/bin/gh", nil }
	err := runPublish(buddy.Opts{SkipConfirm: true}, []string{"v1.2.3"})
	if err == nil || !strings.Contains(err.Error(), "dirty") {
		t.Fatalf("got %v", err)
	}
	if called {
		t.Fatal("gh must not run on a dirty tree")
	}
}

func TestRunPublishCreatesRelease(t *testing.T) {
	origGit, origGH, origLook := runGit, runGH, lookPath
	t.Cleanup(func() {
		runGit, runGH, lookPath = origGit, origGH, origLook
	})
	dir := t.TempDir()
	pkg := filepath.Join(dir, "buddy-1.2.3.pkg")
	if err := os.WriteFile(pkg, []byte("pkg"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "Buddy.app"), 0755); err != nil {
		t.Fatal(err)
	}
	notes := filepath.Join(dir, "notes.md")
	if err := os.WriteFile(notes, []byte("## v1.2.3\n\nShip it.\n"), 0644); err != nil {
		t.Fatal(err)
	}

	runGit = func(args ...string) (string, error) {
		switch args[0] {
		case "status":
			return "", nil
		case "describe":
			return "v1.2.3", nil
		case "remote":
			return "git@github.com:acme/buddy.git", nil
		default:
			t.Fatalf("unexpected git %v", args)
			return "", nil
		}
	}
	var got []string
	runGH = func(args []string) error {
		got = append([]string{}, args...)
		return nil
	}
	lookPath = func(string) (string, error) { return "/usr/bin/gh", nil }

	err := runPublish(buddy.Opts{SkipConfirm: true}, []string{
		"--dist=" + dir,
		"--notes-file=" + notes,
		"v1.2.3",
	})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(got, " ")
	if !strings.Contains(joined, "release create v1.2.3") {
		t.Fatalf("gh args = %v", got)
	}
	if !containsArgPair(got, "--repo", "acme/buddy") || !containsArgPair(got, "--title", "v1.2.3") {
		t.Fatalf("gh args = %v", got)
	}
	if !containsArg(got, pkg) {
		t.Fatalf("missing pkg asset: %v", got)
	}
	if containsArg(got, filepath.Join(dir, "Buddy.app")) {
		t.Fatalf("must not upload Buddy.app: %v", got)
	}
}

func TestRunPublishRequiresTag(t *testing.T) {
	err := runPublish(buddy.Opts{SkipConfirm: true}, nil)
	if err == nil || !strings.Contains(err.Error(), "<tag>") {
		t.Fatalf("got %v", err)
	}
}

func TestRunPublishTagMismatch(t *testing.T) {
	origGit, origGH := runGit, runGH
	t.Cleanup(func() {
		runGit, runGH = origGit, origGH
	})
	runGit = func(args ...string) (string, error) {
		switch args[0] {
		case "status":
			return "", nil
		case "describe":
			return "v1.2.3", nil
		default:
			t.Fatalf("unexpected git %v", args)
			return "", nil
		}
	}
	called := false
	runGH = func(args []string) error {
		called = true
		return nil
	}
	err := runPublish(buddy.Opts{SkipConfirm: true}, []string{"v9.9.9"})
	if err == nil || !strings.Contains(err.Error(), "does not match HEAD") {
		t.Fatalf("got %v", err)
	}
	if called {
		t.Fatal("gh must not run when the tag does not match HEAD")
	}
}

func containsArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func containsArgPair(args []string, flag, val string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag && args[i+1] == val {
			return true
		}
	}
	return false
}
