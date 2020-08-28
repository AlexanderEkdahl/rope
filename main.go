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

const defaultHelp = `Rope is a tool for managing Python dependencies.

Usage:

	rope <command> <arguments>

The commands are:

	run           run command with PYTHONPATH configured
	add           installs and adds the dependency to the current project
	pythonpath    prints the PYTHONPATH to stdout
	init          initializes a new rope project
	version       print rope version
`

func main() {
	arg := ""
	if len(os.Args) > 1 {
		arg = os.Args[1]
	}

	switch arg {
	case "", "help", "--help", "-h":
		fmt.Printf(defaultHelp)
		os.Exit(2)
	case "version", "--version", "-v":
		fmt.Printf("rope version: 0.1.0") // TODO: Inject version
	case "add":
		flagSet := pflag.NewFlagSet("add", pflag.ContinueOnError)
		timeout := flagSet.DurationP("timeout", "t", 0, "Command timeout")
		if err := flagSet.Parse(os.Args[1:]); err == pflag.ErrHelp {
			// prints help
		} else if err != nil {
			fmt.Println("error:", err)
			os.Exit(2)
		}
		if len(flagSet.Args()) < 2 {
			fmt.Println("rope add: package not provided")
			os.Exit(2)
		}
		packages := flagSet.Args()[1:]

		if err := add(*timeout, packages); err != nil {
			fmt.Println("error:", err)
			os.Exit(1)
		}
	case "install":
		fmt.Printf("did you mean 'rope add'?\n")
		os.Exit(2)
	case "pythonpath":
		pythonPath, err := buildPythonPath()
		if err != nil {
			fmt.Println("error:", err)
			return
		}
		fmt.Printf(pythonPath)
	case "run":
		pythonPath, err := buildPythonPath()
		if err != nil {
			fmt.Println("error:", err)
			return
		}

		cmd := exec.Command(os.Args[2], os.Args[3:]...)
		// TODO: Merge any provided PYTHONPATH with the new one?
		cmd.Env = append(os.Environ(), fmt.Sprintf("PYTHONPATH=%s", pythonPath))
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			fmt.Println("error:", err)
			return
		}
		if err := cmd.Wait(); err != nil {
			// Debug log?
		}
	default:
		// TODO: "did you mean X"
		// http://norvig.com/spell-correct.html
		fmt.Printf("rope %s: unknown command\n", arg)
		os.Exit(2)
	}
}
