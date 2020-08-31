package main

import (
	"context"
	"fmt"
	"time"

	"github.com/AlexanderEkdahl/rope/pep508"
	"github.com/AlexanderEkdahl/rope/version"
)

// TODO: Add update parameter for when to use the latest version of transitive
// dependency.
func install(timeout time.Duration, packages []string) error {
	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	project, err := ReadRopefile()
	if err != nil {
		return err
	}

	for _, p := range packages {
		d, err := pep508.ParseDependency(p)
		if err != nil {
			return err
		}

		if len(d.Versions) > 1 {
			return fmt.Errorf("expected at most a single version, got: %s", d.Versions)
		}

		var version version.Version
		if len(d.Versions) > 0 {
			version = d.Versions[0].Version
		}

		// TODO: This should search for the packages on the index to make sure
		// the name/version exists. The inputs to MinimalVersionSelection should
		// not have to deal with normalizing names.
		project.Dependencies = append(project.Dependencies, Dependency{
			Name:    d.DistributionName,
			Version: version,
		})
	}

	index := &Index{
		url: "https://pypi.python.org/simple",
	}

	list, minimalRequirements, err := MinimalVersionSelection(ctx, project.Dependencies, index)
	if err != nil {
		return fmt.Errorf("failed version selection: %w", err)
	}

	for _, d := range list {
		p, err := index.FindPackage(ctx, d.Name, d.Version)
		if err != nil {
			return fmt.Errorf("failed to find package after version selection: %w", err)
		}

		// TODO: This function need to find the package AGAIN? doesn't make sense
		if err := p.Install(ctx); err != nil {
			return fmt.Errorf("installing '%s-%s': %w", d.Name, d.Version, err)
		}
	}

	project.Dependencies = minimalRequirements
	return WriteRopefile(project, "")
}
