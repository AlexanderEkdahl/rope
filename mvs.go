package main

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/AlexanderEkdahl/rope/version"
)

var ErrPackageNotFound = errors.New("package not found")

type Package interface {
	// Name must be normalized in its canonical form
	Name() string
	Version() version.Version
	// Dependency names must be normalized in its canonical form
	Dependencies() []Dependency
	Install(context.Context) (string, error)
}

type PackageIndex interface {
	FindPackage(ctx context.Context, name string, v version.Version) (Package, error)
}

// MinimalVersionSelection recursively visits every dependency's dependencies and builds
// a list of the minimal version required by each dependency. This list is then reduced
// to remove duplicate dependencies by only keeping the greatest version of each entry.
// Finally, the list is sorted by name.
//
// Accompanying the full build list is a minimal list of requirements that when used
// induces the same full build list.
//
// The runtime of this algorithm is proportial to the size of the unreduced list(|B|) plus
// the number of dependencies specified by each dependency(at most |B|²).
//
// This algorithm and the analysis is from https://research.swtch.com/vgo-mvs by Russ Cox.
func MinimalVersionSelection(ctx context.Context, base []Dependency, index PackageIndex) ([]Dependency, []Dependency, error) {
	type node struct {
		value        Dependency
		dependencies []Dependency
	}

	// replace returns true if v2 should replace d1
	replace := func(d1 Dependency, v2 version.Version, v2unspecified bool) bool {
		if v2unspecified {
			return false
		} else if d1.Unspecified {
			return true
		}

		return v2.GT(d1.Version.Version)
	}

	visited := make(map[string]struct{})
	buildDependencies := make(map[string]node)

	work := append([]Dependency{}, base...)
	for len(work) > 0 {
		d := work[len(work)-1]
		work = work[:len(work)-1]

		p, err := index.FindPackage(ctx, d.Name, d.Version)
		if err != nil {
			return nil, nil, fmt.Errorf("finding package '%s-%s': %w", d.Name, d.Version, err)
		}

		// TODO: This specialized logic should be moved into an abstraction(Reqs.Max)
		if v, ok := buildDependencies[p.Name()]; !ok || replace(v.value, p.Version(), d.Version.Unspecified()) {
			buildDependencies[p.Name()] = node{
				value:        Dependency{p.Name(), p.Version(), d.Version.Unspecified()},
				dependencies: p.Dependencies(),
			}
		}

		for _, d := range p.Dependencies() {
			dependencyID := d.Name + d.Version.String()
			if _, ok := visited[dependencyID]; ok {
				// prevent cycles
				continue
			}
			visited[dependencyID] = struct{}{}

			work = append(work, d)
		}
	}

	// Compute the minimal list that would induce the complete build list.
	var postorder []node
	cache := make(map[string][]Dependency)
	// Keep track of all unbounded dependencies so that they can be added
	// to the minimal requirement list to ensure reproduceable builds
	unspecified := make(map[string]bool)
	var walk func(node)
	walk = func(n node) {
		dependencyID := n.value.Name + n.value.Version.String()
		if _, ok := cache[dependencyID]; ok {
			return
		}
		cache[dependencyID] = n.dependencies
		if n.value.Unspecified {
			unspecified[n.value.Name] = true
		}
		for _, d := range n.dependencies {
			walk(buildDependencies[d.Name])
		}
		postorder = append(postorder, n)
	}
	for _, node := range buildDependencies {
		walk(node)
	}

	have := map[string]bool{}
	var walk2 func(Dependency)
	walk2 = func(d Dependency) {
		dependencyID := d.Name + d.Version.String()
		if have[dependencyID] {
			return
		}
		have[dependencyID] = true

		for _, d := range cache[dependencyID] {
			walk2(d)
		}
	}
	max := map[string]version.Version{}
	for name, node := range buildDependencies {
		if v, ok := max[name]; ok {
			if node.value.Version.GT(v.Version) {
				max[name] = node.value.Version
			}
		} else {
			max[name] = node.value.Version
		}
	}
	minimalDependencies := make(map[string]Dependency)
	for _, d := range base {
		v, ok := max[d.Name]
		if !ok {
			panic(fmt.Sprintf("mistake: could not find '%s' in max", d.Name))
		}
		minimalDependencies[d.Name] = Dependency{
			Name:    d.Name,
			Version: v,
		}
		walk2(d)
	}
	for i := len(postorder) - 1; i >= 0; i-- {
		n := postorder[i]
		if max[n.value.Name].LE(n.value.Version.Version) {
			continue
		}
		if !have[n.value.Name] {
			minimalDependencies[postorder[i].value.Name] = postorder[i].value
			walk2(n.value)
			panic("this never happens?")
		}
	}
	// Finally add all unbounded versions to the minimal requirement list to ensure
	// results are reproduceable when a new version get released.
	for name := range unspecified {
		minimalDependencies[name] = buildDependencies[name].value
	}

	buildList := make([]Dependency, 0, len(buildDependencies))
	for _, node := range buildDependencies {
		buildList = append(buildList, node.value)
	}
	minimalList := make([]Dependency, 0, len(minimalDependencies))
	for _, node := range minimalDependencies {
		minimalList = append(minimalList, node)
	}

	// Sort to ensure stable results
	sort.Slice(buildList, func(i, j int) bool {
		return buildList[i].Name < buildList[j].Name
	})
	sort.Slice(minimalList, func(i, j int) bool {
		return minimalList[i].Name < minimalList[j].Name
	})

	return buildList, minimalList, nil
}
