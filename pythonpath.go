package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func buildPythonPath(ctx context.Context) (string, error) {
	project, err := ReadRopefile()
	if err != nil {
		return "", err
	}

	index := &Index{
		url: "https://pypi.python.org/simple",
	}

	list, _, err := MinimalVersionSelection(ctx, project.Dependencies, index)
	if err != nil {
		return "", fmt.Errorf("failed version selection: %w", err)
	}

	ropedir, err := filepath.Abs("ropedir")
	if err != nil {
		return "", err
	}
	pythonPath := &strings.Builder{}
	for i, d := range list {
		// TODO: This should install the missing dependencies
		glob := filepath.Join(ropedir, "packages", fmt.Sprintf("%s-%s*", d.Name, d.Version))
		matches, err := filepath.Glob(glob)
		if err != nil {
			return "", err
		}

		// TODO: Verify arch etc
		if len(matches) > 0 {
			if i > 0 {
				pythonPath.WriteRune(os.PathListSeparator)
			}
			pythonPath.WriteString(matches[0])
		}
	}

	return pythonPath.String(), nil
}
