package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"buddy/internal/buddy"
	"buddy/internal/clispec"
)

// Overridable for tests.
var (
	runGit = func(args ...string) (string, error) {
		cmd := exec.Command("git", args...)
		out, err := cmd.Output()
		return strings.TrimSpace(string(out)), err
	}
	runGH = func(args []string) error {
		cmd := exec.Command("gh", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		return cmd.Run()
	}
	lookPath = exec.LookPath
)

func runPublish(opts buddy.Opts, args []string) error {
	if len(args) > 0 && args[0] == clispec.SubHelp {
		fmt.Print(helpPublish)
		return nil
	}

	distDefault := "dist"
	if v := strings.TrimSpace(os.Getenv("DIST")); v != "" {
		distDefault = v
	}

	fs := flag.NewFlagSet("publish", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() { fmt.Print(helpPublish) }
	dist := fs.String("dist", distDefault, "")
	repoFlag := fs.String("repo", "", "")
	notesFile := fs.String("notes-file", "docs/releases.md", "")
	title := fs.String("title", "", "")
	draft := fs.Bool("draft", false, "")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() < 1 {
		return missingArg("<tag>", clispec.UsageHelp(clispec.CmdPublish))
	}
	if fs.NArg() > 1 {
		return fmt.Errorf("buddy publish: unexpected argument %q; see '%s'", fs.Arg(1), clispec.UsageHelp(clispec.CmdPublish))
	}
	tag := strings.TrimSpace(fs.Arg(0))

	if err := requireCleanGit(); err != nil {
		return err
	}

	if err := checkPublishTag(tag); err != nil {
		return err
	}

	assets, err := distAssets(*dist)
	if err != nil {
		return err
	}

	repo := strings.TrimSpace(*repoFlag)
	if repo == "" {
		repo, err = gitHubPublishRepo()
		if err != nil {
			return err
		}
	} else if parsed, ok := githubRepoFromURL(repo); ok {
		repo = parsed
	} else if !validGitHubRepo(repo) {
		return fmt.Errorf("buddy publish: --repo must be owner/name (got %q)", repo)
	}

	notes, err := loadReleaseNotes(*notesFile, tag)
	if err != nil {
		return err
	}

	relTitle := strings.TrimSpace(*title)
	if relTitle == "" {
		relTitle = tag
	}

	if _, err := lookPath("gh"); err != nil {
		return fmt.Errorf("buddy publish: gh is not on PATH")
	}

	fmt.Printf("GitHub release %s on %s\n", tag, repo)
	for _, a := range assets {
		fmt.Printf("  %s\n", a)
	}

	if !opts.SkipConfirm {
		if !buddy.ConfirmPrompt(fmt.Sprintf("Create GitHub release %s on %s?", tag, repo), buddy.GetLogger()) {
			return fmt.Errorf("aborted")
		}
	}

	notesPath, err := writeTempNotes(notes)
	if err != nil {
		return err
	}
	defer os.Remove(notesPath)

	ghArgs := []string{"release", "create", tag, "--repo", repo, "--title", relTitle, "--notes-file", notesPath}
	if *draft {
		ghArgs = append(ghArgs, "--draft")
	}
	ghArgs = append(ghArgs, assets...)
	if err := runGH(ghArgs); err != nil {
		return fmt.Errorf("buddy publish: gh: %w", err)
	}
	return nil
}

func requireCleanGit() error {
	out, err := runGit("status", "--porcelain")
	if err != nil {
		return fmt.Errorf("buddy publish: git status: %w", gitCmdErr(err))
	}
	if strings.TrimSpace(out) != "" {
		return fmt.Errorf("buddy publish: working tree is dirty; commit or stash first")
	}
	return nil
}

func exactGitTag() (string, error) {
	tag, err := runGit("describe", "--tags", "--exact-match")
	if err != nil || tag == "" {
		return "", fmt.Errorf("buddy publish: HEAD is not an exact git tag (git describe --tags --exact-match)")
	}
	return tag, nil
}

func checkPublishTag(want string) error {
	head, err := exactGitTag()
	if err != nil {
		return err
	}
	if want != head {
		return fmt.Errorf("buddy publish: tag %q does not match HEAD %q", want, head)
	}
	return nil
}

func distAssets(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("buddy publish: %s: %w (run make release first)", dir, err)
	}
	var files []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") || name == "buddy" || name == "buddy-lite" {
			continue
		}
		p := filepath.Join(dir, name)
		info, err := e.Info()
		if err != nil {
			return nil, err
		}
		if isDir(p) {
			fmt.Fprintf(os.Stderr, "buddy publish: skipping %s (directory)\n", p)
			continue
		}
		if !info.Mode().IsRegular() {
			fmt.Fprintf(os.Stderr, "buddy publish: skipping %s\n", p)
			continue
		}
		files = append(files, p)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("buddy publish: no regular files in %s (run make release first)", dir)
	}
	sort.Strings(files)
	return files, nil
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func gitHubPublishRepo() (string, error) {
	for _, remote := range []string{"public", "origin"} {
		url, err := runGit("remote", "get-url", remote)
		if err != nil || url == "" {
			continue
		}
		if repo, ok := githubRepoFromURL(url); ok {
			return repo, nil
		}
	}
	return "", fmt.Errorf("buddy publish: no GitHub remote named public or origin; pass --repo=<owner/name>")
}

func githubRepoFromURL(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	s = strings.TrimSuffix(s, ".git")
	s = strings.TrimRight(s, "/")
	var path string
	switch {
	case strings.HasPrefix(s, "git@github.com:"):
		path = strings.TrimPrefix(s, "git@github.com:")
	case strings.HasPrefix(s, "ssh://git@github.com/"):
		path = strings.TrimPrefix(s, "ssh://git@github.com/")
	case strings.HasPrefix(s, "https://github.com/"):
		path = strings.TrimPrefix(s, "https://github.com/")
	case strings.HasPrefix(s, "http://github.com/"):
		path = strings.TrimPrefix(s, "http://github.com/")
	default:
		return "", false
	}
	path = strings.TrimPrefix(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", false
	}
	return parts[0] + "/" + parts[1], true
}

func validGitHubRepo(s string) bool {
	owner, name, ok := strings.Cut(s, "/")
	if !ok || owner == "" || name == "" {
		return false
	}
	if strings.Contains(owner, "/") || strings.Contains(name, "/") {
		return false
	}
	return !strings.ContainsAny(s, " \t\n")
}

func loadReleaseNotes(path, tag string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("buddy publish: notes file: %w", err)
	}
	notes, err := releaseNotesForTag(string(b), tag)
	if err != nil {
		return "", fmt.Errorf("buddy publish: %s: %w", path, err)
	}
	return notes, nil
}

func releaseNotesForTag(markdown, tag string) (string, error) {
	headings := []string{"## " + tag}
	if trimmed := strings.TrimPrefix(tag, "v"); trimmed != tag {
		headings = append(headings, "## "+trimmed)
	} else {
		headings = append(headings, "## v"+tag)
	}
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	start := -1
	used := ""
	for i, line := range lines {
		for _, h := range headings {
			if line == h {
				start = i + 1
				used = h
				break
			}
		}
		if start >= 0 {
			break
		}
	}
	if start < 0 {
		return "", fmt.Errorf("no %q section", "## "+tag)
	}
	end := len(lines)
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "## ") {
			end = i
			break
		}
	}
	body := strings.TrimSpace(strings.Join(lines[start:end], "\n"))
	if body == "" {
		return "", fmt.Errorf("%s section is empty", used)
	}
	return body, nil
}

func writeTempNotes(notes string) (string, error) {
	f, err := os.CreateTemp("", "buddy-release-notes-*.md")
	if err != nil {
		return "", err
	}
	path := f.Name()
	if _, err := f.WriteString(notes); err != nil {
		f.Close()
		os.Remove(path)
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(path)
		return "", err
	}
	return path, nil
}

func gitCmdErr(err error) error {
	var ee *exec.ExitError
	if errors.As(err, &ee) && len(ee.Stderr) > 0 {
		return fmt.Errorf("%s", strings.TrimSpace(string(ee.Stderr)))
	}
	return err
}
