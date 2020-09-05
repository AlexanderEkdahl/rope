package main

import (
	"context"
	"fmt"
	"os"
	"strings"
)

func buildPythonPath(ctx context.Context) (string, error) {
	project, err := ReadRopefile()
	if err != nil {
		return "", err
	}

	index := &Index{
		url: DefaultIndex,
	}
	list, _, err := MinimalVersionSelection(ctx, project.Dependencies, index)
	if err != nil {
		return "", fmt.Errorf("failed version selection: %w", err)
	}

	pythonPath := &strings.Builder{}
	for i, d := range list {
		p, err := index.FindPackage(ctx, d.Name, d.Version)
		if err != nil {
			return "", fmt.Errorf("failed to find package after version selection: %w", err)
		}

		// TODO: This function need to find the package AGAIN? doesn't make sense
		installationPath, err := p.Install(ctx)
		if err != nil {
			return "", fmt.Errorf("installing '%s-%s': %w", d.Name, d.Version, err)
		}

		if i > 0 {
			pythonPath.WriteRune(os.PathListSeparator)
		}
		pythonPath.WriteString(installationPath)
	}

	return pythonPath.String(), nil
}
