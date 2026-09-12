//go:build darwin && !cgo

package main

import (
	"os"
	"syscall"
)

func main() {
	helper, err := resolveHelper()
	if err != nil {
		fatalf("buddy: %v", err)
	}
	args := defaultArgs()
	argv := append([]string{helper}, args...)
	if err := syscall.Exec(helper, argv, os.Environ()); err != nil {
		fatalf("buddy: exec %s: %v", helper, err)
	}
}
