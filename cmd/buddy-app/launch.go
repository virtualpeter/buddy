package main

import (
	"fmt"
	"os"
)

func defaultArgs() []string {
	if len(os.Args) > 1 {
		return os.Args[1:]
	}
	return []string{"serve", "--open"}
}

func resolveHelper() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	helper, err := helperPath(exe)
	if err != nil {
		return "", err
	}
	st, err := os.Stat(helper)
	if err != nil {
		return "", err
	}
	if st.IsDir() {
		return "", fmt.Errorf("CLI helper not found at %s", helper)
	}
	return helper, nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
