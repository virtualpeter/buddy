//go:build darwin

package main

import (
	"fmt"
	"os"
	"syscall"
)

func main() {
	exe, err := os.Executable()
	if err != nil {
		fatalf("buddy: %v", err)
	}
	helper, err := helperPath(exe)
	if err != nil {
		fatalf("buddy: %v", err)
	}
	st, err := os.Stat(helper)
	if err != nil || st.IsDir() {
		fatalf("buddy: CLI helper not found at %s", helper)
	}

	args := os.Args[1:]
	if len(args) == 0 {
		args = []string{"serve", "--open"}
	}
	argv := append([]string{helper}, args...)
	if err := syscall.Exec(helper, argv, os.Environ()); err != nil {
		fatalf("buddy: exec %s: %v", helper, err)
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
