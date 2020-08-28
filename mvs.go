package main

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/AlexanderEkdahl/rope/version"
)

// TODO: Upgrade type to include name and version
var PackageNotFoundErr = errors.New("package not found")

type Package interface {
	Name() string
	Version() version.Version
	Dependencies() []Dependency
	Install(context.Context) error
}

type PackageIndex interface {
	FindPackage(ctx context.Context, name string, v version.Version) (Package, error)
}

type Tree []Node

// Walk invokes f for ever node in the tree in breadth first order
func (t *Tree) Walk(f func(d Dependency, depth int)) {
	walkTree(t, 0, f)
}

func walkTree(t *Tree, depth int, f func(d Dependency, depth int)) {
	for _, d := range *t {
		f(d.Value, depth)

		if len(d.children) > 0 {
			walkTree(&d.children, depth+1, f)
		}
	}
}

type Node struct {
	Value    Dependency
	children Tree
}

// MinimalVersionSelection recursively visits every dependency's dependencies and builds
// a list of the minimal version required by each dependency. This list is then reduced
// to remove duplicate dependencies by only keeping the greatest version of each entry.
// Finally, the list is sorted by name.
//
// The runtime of this algorithm is proportial to the size of the unreduced list(|B|) plus
// the number of dependencies specified by each dependency(at most |B|²).
//
// This algorithm and the analysis is from https://research.swtch.com/vgo-mvs by Russ Cox.
func MinimalVersionSelection(ctx context.Context, dependencies []Dependency, index PackageIndex) ([]Dependency, error) {
	// 1. Find versions
	tree, err := minimalVersionSelection(ctx, dependencies, index, make(map[string]struct{}))
	if err != nil {
		return nil, err
	}

	// tree.Walk(func(d Dependency, depth int) {
	// 	fmt.Printf("%s%s: %s(%s)\n", strings.Repeat("  ", depth), d.Name, d.Version, d.RequestedVersion)
	// })

	// 2. Resolve duplicates
	reduced := reduce(tree)

	// 3. Sort results
	sort.Slice(reduced, func(i, j int) bool {
		return reduced[i].Name < reduced[j].Name
	})

	return reduced, nil
}

func minimalVersionSelection(
	ctx context.Context,
	dependencies []Dependency,
	index PackageIndex,
	visited map[string]struct{},
) (Tree, error) {
	if len(dependencies) == 0 {
		return nil, nil
	}

	nodes := []Node{}
	for _, d := range dependencies {
		// Prevent cycles
		if _, ok := visited[fmt.Sprintf("%s-%s", d.Name, d.Version)]; ok {
			continue
		}
		visited[fmt.Sprintf("%s-%s", d.Name, d.Version)] = struct{}{}

		// TODO: Parallelize by running in separate goroutine
		p, err := index.FindPackage(ctx, d.Name, d.Version)
		if err != nil {
			return nil, fmt.Errorf("finding package '%s-%s': %w", d.Name, d.Version, err)
		}

		transitiveNodes, err := minimalVersionSelection(ctx, p.Dependencies(), index, visited)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, Node{
			// The found dependency is assumed to be the canonical representation of the
			// package as the exact version and casing may differ from the candidate version.
			Value: Dependency{
				Name:    p.Name(),
				Version: p.Version(),

				RequestedVersion: d.Version,
			},
			children: transitiveNodes,
		})
	}

	return nodes, nil
}

// reduce removes duplicate entries in the list and only keeps the greatest
// version of each dependency.
func reduce(t Tree) []Dependency {
	type dependencyDepth struct {
		dependency Dependency
		depth      int

		requestedVersion version.Version
	}

	ps := make(map[string][]dependencyDepth)

	t.Walk(func(d Dependency, depth int) {
		ps[d.Name] = append(ps[d.Name], dependencyDepth{
			dependency: d,
			depth:      depth,

			requestedVersion: d.RequestedVersion,
		})
	})

	reducedList := make([]Dependency, 0)
	for _, d := range ps {
		greatest := d[0].dependency

		for i := 1; i < len(d); i++ {
			if d[i].dependency.Version.GT(greatest.Version.Version) {
				greatest = d[i].dependency
			}
		}

		reducedList = append(reducedList, greatest)
	}

	return reducedList
}
