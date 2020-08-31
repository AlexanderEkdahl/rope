package main

import (
	"context"
	"fmt"
	"io"
)

// ExportRequirements exports all of the versions found by minimal
// version selection in a format that can be consumed by pip.
func ExportRequirements(ctx context.Context, output io.Writer) error {
	project, err := ReadRopefile()
	if err != nil {
		return err
	}

	index := &Index{
		url: DefaultIndex,
	}
	list, _, err := MinimalVersionSelection(ctx, project.Dependencies, index)
	if err != nil {
		return fmt.Errorf("failed version selection: %w", err)
	}

	for _, v := range list {
		fmt.Fprintf(output, "%s==%s\n", v.Name, v.Version)
	}

	return nil
}
