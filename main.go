package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"
)

func buildPythonPath() (string, error) {
	rope, err := ReadRopefile()
	if err != nil {
		return "", err
	}

	ropedir, err := filepath.Abs("ropedir")
	if err != nil {
		return "", err
	}
	paths := make([]string, 0, len(rope.Dependencies))
	for _, d := range rope.Dependencies {
		// TODO: This should install the missing dependencies
		glob := filepath.Join(ropedir, "packages", fmt.Sprintf("%s-%s*", d.Name, d.Version))
		matches, err := filepath.Glob(glob)
		if err != nil {
			return "", err
		}

		// TODO: Verify arch etc
		if len(matches) > 0 {
			paths = append(paths, matches[0])
		}
	}

	return strings.Join(paths, string(os.PathListSeparator)), nil
}

const defaultHelp = `Rope is a tool for managing Python dependencies 🧩

Usage:

  rope <command> [options]

The commands are:

  run          run command with PYTHONPATH configured
  init         initializes a new rope project
  install      installs and adds one or more dependencies
  remove       removes one or more dependencies
  show         inspect the current dependencies
  export       export dependency specification
  cache        inspecting and clearing the cache
  pythonpath   prints the configured PYTHONPATH
  version      show rope version
`

func run(args []string) (int, error) {
	arg := ""
	if len(args) > 1 {
		arg = args[1]
	}

	switch arg {
	case "", "help", "--help", "-h":
		fmt.Printf(defaultHelp)
		return 2, nil
	case "version", "--version":
		fmt.Printf("rope version: 0.1.0") // TODO: Inject version
		return 0, nil
	case "init":
		if path, err := FindRopefile(); err == ErrRopefileNotFound {
			// continue
		} else if err != nil {
			return 1, err
		} else {
			return 1, fmt.Errorf("rope.json already found at: %s", path)
		}

		if err := WriteRopefile(&Project{Dependencies: []Dependency{}}, "rope.json"); err != nil {
			return 1, err
		}
		return 0, nil
	case "add":
		fmt.Println("did you mean: 'rope install'?")
		return 2, nil
	case "install":
		flagSet := pflag.NewFlagSet("install", pflag.ContinueOnError)
		timeout := flagSet.Duration("timeout", 0, "Command timeout")
		if err := flagSet.Parse(args[1:]); err == pflag.ErrHelp {
			return 0, nil
		} else if err != nil {
			return 2, err
		}
		if len(flagSet.Args()) < 2 {
			fmt.Println("rope add: package not provided")
			return 2, nil
		}
		packages := flagSet.Args()[1:]

		if err := install(*timeout, packages); err != nil {
			return 1, err
		}
		return 0, nil
	case "remove":
		// TODO: Implement command to remove dependency(error if transitive)
		return 1, fmt.Errorf("not implemented")
	case "export":
		// TODO: Implement command to export to requirements.txt for interop
		return 1, fmt.Errorf("not implemented")
	case "show":
		// TODO: Implement command to show all dependenies along with a tree view
		return 1, fmt.Errorf("not implemented")
	case "cache":
		// TODO: Implement operations for show information/clearing the cache
		return 1, fmt.Errorf("not implemented")
	case "pythonpath":
		pythonPath, err := buildPythonPath()
		if err != nil {
			return 1, err
		}
		fmt.Printf(pythonPath)
		return 0, nil
	case "run":
		pythonPath, err := buildPythonPath()
		if err != nil {
			return 1, err
		}

		cmd := exec.Command(args[2], args[3:]...)
		// TODO: Merge any provided PYTHONPATH with the new one?
		cmd.Env = append(os.Environ(), fmt.Sprintf("PYTHONPATH=%s", pythonPath))
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			return 1, nil
		}
		if err := cmd.Wait(); err != nil {
			return cmd.ProcessState.ExitCode(), nil
		}

		return 0, nil
	default:
		// TODO: "did you mean X"
		// http://norvig.com/spell-correct.html
		fmt.Printf("rope %s: unknown command\n", arg)
		return 2, nil
	}
}

func main() {
	exitCode, err := run(os.Args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
	}
	os.Exit(exitCode)
}
