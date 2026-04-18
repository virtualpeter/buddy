package script

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"buddy/internal/buddy"

	"github.com/qri-io/starlib/re"
	"go.starlark.net/starlark"
)

// Run executes a Starlark script file with buddy builtins.
// When trace is true each builtin call is logged to stderr with its position,
// arguments, and return value.
func Run(credName string, opts buddy.Opts, scriptPath string, args []string, logger *slog.Logger, trace bool) error {
	// Store runner-level opts (e.g. -y flag) for use in run_workflow confirmation.
	runnerOpts = opts

	data, err := os.ReadFile(scriptPath)
	if err != nil {
		return fmt.Errorf("read script: %w", err)
	}

	reModule, err := re.LoadModule()
	if err != nil {
		return fmt.Errorf("load re module: %w", err)
	}

	predeclared := makeBuiltins(credName, logger, trace)
	predeclared["cred"] = starlark.String(credName)
	predeclared["args"] = listFromStrings(args)
	for k, v := range reModule {
		predeclared[k] = v
	}

	thread := &starlark.Thread{
		Name: "buddy",
		Load: loader(reModule),
	}
	thread.SetLocal("cred", credName)

	thread.Print = func(t *starlark.Thread, msg string) {
		if trace {
			pos := t.CallFrame(0).Pos
			fmt.Fprintf(os.Stderr, "[trace] %s:%d  print: %s\n", pos.Filename(), pos.Line, msg)
		}
		fmt.Fprintln(os.Stdout, msg)
	}

	globals, err := starlark.ExecFile(thread, scriptPath, data, predeclared)
	if err != nil {
		var ee *ExitError
		if errors.As(err, &ee) {
			os.Exit(ee.Code)
		}
		return err
	}
	mainFn := globals["main"]
	if mainFn != nil {
		_, err = starlark.Call(thread, mainFn, nil, []starlark.Tuple{
			{starlark.String("cred"), starlark.String(credName)},
		})
		if err != nil {
			var ee *ExitError
			if errors.As(err, &ee) {
				os.Exit(ee.Code)
			}
			return fmt.Errorf("main: %w", err)
		}
	}
	return nil
}

func loader(reModule starlark.StringDict) func(thread *starlark.Thread, module string) (starlark.StringDict, error) {
	return func(thread *starlark.Thread, module string) (starlark.StringDict, error) {
		if module == "re" {
			return reModule, nil
		}
		return nil, fmt.Errorf("unknown module: %s", module)
	}
}

func listFromStrings(ss []string) *starlark.List {
	elems := make([]starlark.Value, len(ss))
	for i, s := range ss {
		elems[i] = starlark.String(s)
	}
	return starlark.NewList(elems)
}
