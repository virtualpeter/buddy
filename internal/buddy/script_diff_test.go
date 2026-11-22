package buddy

import (
	"fmt"
	"strings"
	"testing"
)

func TestNormalDiffMatchesDiffCommand(t *testing.T) {
	got := normalDiff("alpha\nbeta\ngamma\n", "alpha\nBETA\ngamma\ndelta\n")
	want := "" +
		"2c2\n" +
		"< beta\n" +
		"---\n" +
		"> BETA\n" +
		"3a4\n" +
		"> delta\n"
	if got != want {
		t.Fatalf("diff=\n%s\nwant=\n%s", got, want)
	}
	if normalDiff("alpha\n", "alpha\n") != "" {
		t.Fatal("identical bodies should be empty")
	}
	nl := normalDiff("alpha\n", "alpha")
	wantNL := "" +
		"1c1\n" +
		"< alpha\n" +
		"---\n" +
		"> alpha\n" +
		"\\ No newline at end of file\n"
	if nl != wantNL {
		t.Fatalf("newline diff=\n%s", nl)
	}
	del := normalDiff("alpha\nbeta\ngamma\n", "alpha\ngamma\n")
	if del != "2d1\n< beta\n" {
		t.Fatalf("delete=\n%s", del)
	}
}

func TestNormalDiffOneLineInALongFile(t *testing.T) {
	var a, b strings.Builder
	for i := 0; i < 2000; i++ {
		if i == 1000 {
			a.WriteString("old value\n")
			b.WriteString("new value\n")
			continue
		}
		line := fmt.Sprintf("line %d\n", i)
		a.WriteString(line)
		b.WriteString(line)
	}
	got := normalDiff(a.String(), b.String())
	want := "" +
		"1001c1001\n" +
		"< old value\n" +
		"---\n" +
		"> new value\n"
	if got != want {
		t.Fatalf("diff=\n%s", got)
	}
}
