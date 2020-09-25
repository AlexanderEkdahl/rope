package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/AlexanderEkdahl/rope/version"
)

// TODO: Add update parameter for when to use the latest version of transitive
// dependency.
func add(timeout time.Duration, packages []string) error {
	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	project, err := ReadRopefile()
	ropefilePath := ""
	if errors.Is(err, ErrRopefileNotFound) {
		project = &Project{Dependencies: []Dependency{}}
		ropefilePath = "rope.json"
	} else if err != nil {
		return err
	}

	// index := &Index{url: DefaultIndex}
	index := &PyPI{}
	for _, p := range packages {
		d, err := version.ParseDependency(p)
		if err != nil {
			return err
		}

		if len(d.Versions) > 1 {
			return fmt.Errorf("expected at most a single version, got: %d", len(d.Versions))
		}

		if len(d.Extras) > 0 {
			// TODO: Support extras
			return fmt.Errorf("extras not supported")
		}

		var version version.Version
		if len(d.Versions) > 0 {
			version = d.Versions[0].Version
		}

		p, err := index.FindPackage(ctx, d.Name, version)
		if err != nil {
			return fmt.Errorf("finding '%s-%s': %w", d.Name, version, err)
		}
		project.Dependencies = append(project.Dependencies, Dependency{
			Name:    p.Name(),
			Version: p.Version(),
		})
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
		if _, err := p.Install(ctx); err != nil {
			return fmt.Errorf("installing '%s-%s': %w", d.Name, d.Version, err)
		}
	}

	project.Dependencies = minimalRequirements
	return WriteRopefile(project, ropefilePath)
}
