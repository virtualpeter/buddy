package buddy

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/aymanbagabas/go-udiff"
)

// ErrScriptDiffers is returned after the diff text has been printed.
// The bodies differ. Callers that want diff(1) behavior exit 1 without printing it.
var ErrScriptDiffers = errors.New("script diff: differ")

// Script diff classes. match and missing line up with script check; differ means
// the local file and the Jamf body are not the same bytes.
const (
	ScriptDiffMatch   = "match"
	ScriptDiffDiffer  = "differ"
	ScriptDiffMissing = "missing"
)

// ScriptDiffResult is the report from ScriptDiff.
type ScriptDiffResult struct {
	OK    bool
	Class string
	ID    string
	Name  string
	File  string
	Diff  string
}

// ScriptDiff compares file with a Jamf script body. ref empty resolves file by
// the download name of its basename. ref set is an id, an exact name, or a
// local filename. A missing script is Class missing. Several download-name
// owners is an error. Bytes are compared as stored.
func ScriptDiff(credName, file, ref string, logger *slog.Logger) (ScriptDiffResult, error) {
	_ = logger
	data, err := os.ReadFile(file)
	if err != nil {
		return ScriptDiffResult{}, fmt.Errorf("read script file %q: %w", file, err)
	}
	base := filepath.Base(file)
	ref = strings.TrimSpace(ref)
	var s *ScriptDetail
	if ref == "" {
		hits, err := scriptHitsByFileName(credName, file)
		if err != nil {
			return ScriptDiffResult{}, err
		}
		if len(hits) > 1 {
			filename := scriptFilename(base)
			return ScriptDiffResult{}, fmt.Errorf("multiple scripts download as %q: %s", filename, formatScriptHits(hits))
		}
		if len(hits) == 0 {
			return ScriptDiffResult{Class: ScriptDiffMissing, File: base}, nil
		}
		got, err := scriptGetFull(credName, hits[0].ID)
		if err != nil {
			return ScriptDiffResult{}, err
		}
		s = got
	} else {
		resolved, err := resolveScriptRef(credName, ref)
		if err != nil {
			return ScriptDiffResult{}, err
		}
		got, err := scriptGetFull(credName, resolved.ID)
		if err != nil {
			if scriptIsNotFound(err) {
				return ScriptDiffResult{Class: ScriptDiffMissing, File: base}, nil
			}
			return ScriptDiffResult{}, err
		}
		s = got
	}
	text := normalDiff(s.ScriptContents, string(data))
	res := ScriptDiffResult{
		Class: ScriptDiffDiffer, ID: s.ID, Name: s.Name, File: base, Diff: text,
	}
	if text == "" {
		res.Class = ScriptDiffMatch
		res.OK = true
	}
	return res, nil
}

// ScriptDiffPrint compares and prints diff text. API and I/O errors are
// returned before a diff. Identical bytes print nothing and return nil.
// A difference prints only the diff and returns ErrScriptDiffers.
func ScriptDiffPrint(credName, file, ref string, logger *slog.Logger) error {
	res, err := ScriptDiff(credName, file, ref, logger)
	if err != nil {
		return err
	}
	if res.Diff != "" {
		fmt.Print(res.Diff)
	}
	if res.OK {
		return nil
	}
	if res.Class == ScriptDiffDiffer {
		return ErrScriptDiffers
	}
	return fmt.Errorf("script diff: %s", res.Class)
}

// normalDiff is the text `diff` prints when the Jamf body is the first file
// and the local body is the second. `<` lines are Jamf. `>` lines are local.
func normalDiff(from, to string) string {
	edits := udiff.Lines(from, to)
	if len(edits) == 0 {
		return ""
	}
	fromLines := splitDiffLines(from)
	var buf strings.Builder
	fromAt := 0
	toAt := 0
	for _, e := range edits {
		i1 := lineIndexAt(fromLines, e.Start)
		i2 := lineIndexAt(fromLines, e.End)
		inserted := splitDiffLines(e.New)
		toAt += i1 - fromAt
		fromAt = i2
		switch {
		case i1 < i2 && len(inserted) > 0:
			fmt.Fprintf(&buf, "%sc%s\n", diffRange(i1+1, i2), diffRange(toAt+1, toAt+len(inserted)))
			writeDiffSide(&buf, '<', fromLines[i1:i2])
			buf.WriteString("---\n")
			writeDiffSide(&buf, '>', inserted)
		case i1 < i2:
			fmt.Fprintf(&buf, "%sd%d\n", diffRange(i1+1, i2), toAt)
			writeDiffSide(&buf, '<', fromLines[i1:i2])
		default:
			fmt.Fprintf(&buf, "%da%s\n", i1, diffRange(toAt+1, toAt+len(inserted)))
			writeDiffSide(&buf, '>', inserted)
		}
		toAt += len(inserted)
	}
	return buf.String()
}

// lineIndexAt reports the line whose byte range contains off, or the line
// that starts at off when off is a boundary. udiff.Lines uses those boundaries.
func lineIndexAt(lines []string, off int) int {
	pos := 0
	for i, line := range lines {
		next := pos + len(line)
		if off < next {
			return i
		}
		pos = next
	}
	return len(lines)
}

func diffRange(start, end int) string {
	if start == end {
		return strconv.Itoa(start)
	}
	return strconv.Itoa(start) + "," + strconv.Itoa(end)
}

func writeDiffSide(buf *strings.Builder, mark byte, lines []string) {
	for _, line := range lines {
		body := strings.TrimSuffix(line, "\n")
		fmt.Fprintf(buf, "%c %s\n", mark, body)
		if !strings.HasSuffix(line, "\n") {
			fmt.Fprintf(buf, "\\ No newline at end of file\n")
		}
	}
}

func splitDiffLines(text string) []string {
	if text == "" {
		return nil
	}
	lines := strings.SplitAfter(text, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
