package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/pflag"
)

// Version identifies the version of rope. This can be modified by CI during
// the release process.
var Version = "dev"

const defaultHelp = `Rope is a tool for managing Python dependencies 🧩

Usage:

  rope <command> [options]

The commands are:

  run          run command with PYTHONPATH configured
  init         initializes a new rope project
  add          installs and adds one or more dependencies
  remove       removes one or more dependencies
  show         inspect the current dependencies
  export       export dependency specification
  cache        inspecting and clearing the cache
  pythonpath   prints the configured PYTHONPATH
  version      show rope version
`

// TODO: Figure out how to better interact with this.
// Abstract methods that needs this type of interaction into its own type.
var cache *Cache
var env *Environment

// Should move the main package into a cli folder and let the top-level package be 'rope'
// Which can be directly used in the test harness.
func run(args []string) (int, error) {
	arg := ""
	if len(args) > 1 {
		arg = args[1]
	}

	cache = &Cache{}
	defer cache.Close()

	// Lazy-loaded environment
	env = &Environment{}

	switch arg {
	case "", "help", "--help", "-h":
		fmt.Printf(defaultHelp)
		return 2, nil
	case "version", "--version":
		fmt.Printf("rope version: %s\n", Version)
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
	case "install":
		fmt.Println("did you mean: 'rope add'?")
		return 2, nil
	case "add":
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

		if err := add(*timeout, packages); err != nil {
			return 1, err
		}
		return 0, nil
	case "remove":
		// TODO: Implement command to remove dependency(error if transitive)
		return 1, fmt.Errorf("not implemented")
	case "export":
		if err := ExportRequirements(context.Background(), os.Stdout); err != nil {
			return 1, err
		}
		return 0, nil
	case "show":
		// TODO: Implement command to show all dependenies along with a tree view
		return 1, fmt.Errorf("not implemented")
	case "cache":
		// TODO: Implement operations for show information/clearing the cache
		return 1, fmt.Errorf("not implemented")
	case "pythonpath":
		pythonPath, err := buildPythonPath(context.Background())
		if err != nil {
			return 1, err
		}
		fmt.Printf(pythonPath)
		return 0, nil
	case "run":
		pythonPath, err := buildPythonPath(context.Background())
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
			return 1, err
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
