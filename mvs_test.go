package main

import (
	"context"
	"testing"

	"github.com/AlexanderEkdahl/rope/version"
	"github.com/blang/semver/v4"
)

type testPackageIndex struct {
	index map[string][]testPackage
}

func (pi *testPackageIndex) FindPackage(ctx context.Context, name string, v version.Version) (Package, error) {
	var foundPackage Package
	for _, p := range pi.index[name] {
		if v.Equals(semver.Version{}) {
			foundPackage = p
			// Keep searching as there may be another package with a higher
			// version matching.
		} else if v.EQ(p.version.Version) {
			return p, nil
		}
	}

	if foundPackage == nil {
		return nil, ErrPackageNotFound
	}

	return foundPackage, nil
}

type testPackage struct {
	name         string
	version      version.Version
	dependencies []Dependency
}

func (p testPackage) Name() string {
	return p.name
}

func (p testPackage) Version() version.Version {
	return p.version
}

func (p testPackage) Dependencies() []Dependency {
	return p.dependencies
}

func (p testPackage) Install(context.Context) (string, error) {
	return "", nil
}

func TestVersionSelection(t *testing.T) {
	// Dependency graph taken from: https://research.swtch.com/vgo-mvs
	index := &testPackageIndex{
		map[string][]testPackage{
			"B": {
				{
					name:    "B",
					version: version.MustParse("1.1.0"),
					dependencies: []Dependency{
						{
							Name:    "D",
							Version: version.MustParse("1.1.0"),
						},
					},
				},
				{
					name:    "B",
					version: version.MustParse("1.2.0"),
					dependencies: []Dependency{
						{
							Name:    "D",
							Version: version.MustParse("1.3.0"),
						},
					},
				},
			},
			"C": {
				{
					name:    "C",
					version: version.MustParse("1.1.0"),
				},
				{
					name:    "C",
					version: version.MustParse("1.2.0"),
					dependencies: []Dependency{
						{
							Name:    "D",
							Version: version.MustParse("1.4.0"),
						},
					},
				},
				{
					name:    "C",
					version: version.MustParse("1.3.0"),
					dependencies: []Dependency{
						{
							Name:    "F",
							Version: version.MustParse("1.1.0"),
						},
					},
				},
			},
			"D": {
				{
					name:    "D",
					version: version.MustParse("1.1.0"),
					dependencies: []Dependency{
						{
							Name:    "E",
							Version: version.MustParse("1.1.0"),
						},
					},
				},
				{
					name:    "D",
					version: version.MustParse("1.2.0"),
					dependencies: []Dependency{
						{
							Name:    "E",
							Version: version.MustParse("1.1.0"),
						},
					},
				},
				{
					name:    "D",
					version: version.MustParse("1.3.0"),
					dependencies: []Dependency{
						{
							Name:    "E",
							Version: version.MustParse("1.2.0"),
						},
					},
				},
				{
					name:    "D",
					version: version.MustParse("1.4.0"),
					dependencies: []Dependency{
						{
							Name:    "E",
							Version: version.MustParse("1.2.0"),
						},
					},
				},
			},
			"E": {
				{
					name:    "E",
					version: version.MustParse("1.1.0"),
				},
				{
					name:    "E",
					version: version.MustParse("1.2.0"),
				},
				{
					name:    "E",
					version: version.MustParse("1.3.0"),
				},
			},
			"F": {
				{
					name:    "F",
					version: version.MustParse("1.1.0"),
					dependencies: []Dependency{
						{
							Name:    "G",
							Version: version.MustParse("1.1.0"),
						},
					},
				},
			},
			"G": {
				{
					name:    "G",
					version: version.MustParse("1.1.0"),
					dependencies: []Dependency{
						{
							Name:    "F",
							Version: version.MustParse("1.1.0"),
						},
					},
				},
			},
		},
	}

	verifyMinimalVersionSelection(
		t,
		index,
		[]Dependency{
			{
				Name:    "B",
				Version: version.MustParse("1.2.0"),
			},
			{
				Name:    "C",
				Version: version.MustParse("1.2.0"),
			},
		},
		[]Dependency{
			{
				Name:    "B",
				Version: version.MustParse("1.2.0"),
			},
			{
				Name:    "C",
				Version: version.MustParse("1.2.0"),
			},
			{
				Name:    "D",
				Version: version.MustParse("1.4.0"),
			},
			{
				Name:    "E",
				Version: version.MustParse("1.2.0"),
			},
		},
		[]Dependency{
			{
				Name:    "B",
				Version: version.MustParse("1.2.0"),
			},
			{
				Name:    "C",
				Version: version.MustParse("1.2.0"),
			},
		},
	)

	verifyMinimalVersionSelection(
		t,
		index,
		[]Dependency{
			{
				Name:    "B",
				Version: version.MustParse("1.2.0"),
			},
			{
				Name:    "C",
				Version: version.MustParse("1.3.0"), // Leads to cyclical import
			},
		},
		[]Dependency{
			{
				Name:    "B",
				Version: version.MustParse("1.2.0"),
			},
			{
				Name:    "C",
				Version: version.MustParse("1.3.0"),
			},
			{
				Name:    "D",
				Version: version.MustParse("1.3.0"),
			},
			{
				Name:    "E",
				Version: version.MustParse("1.2.0"),
			},
			{
				Name:    "F",
				Version: version.MustParse("1.1.0"),
			},
			{
				Name:    "G",
				Version: version.MustParse("1.1.0"),
			},
		},
		[]Dependency{
			{
				Name:    "B",
				Version: version.MustParse("1.2.0"),
			},
			{
				Name:    "C",
				Version: version.MustParse("1.3.0"),
			},
		},
	)
}

func TestVersionSelectionTransitiveUnbounded(t *testing.T) {
	index := &testPackageIndex{
		map[string][]testPackage{
			"torch": {
				{
					name:    "torch",
					version: version.MustParse("1.6.0"),
					dependencies: []Dependency{
						{
							Name: "numpy",
						},
					},
				},
			},
			"tensorflow": {
				{
					name:    "tensorflow",
					version: version.MustParse("2.3.0"),
					dependencies: []Dependency{
						{
							Name:    "numpy",
							Version: version.MustParse("1.14.0"),
						},
					},
				},
			},
			"numpy": {
				{
					name:    "numpy",
					version: version.MustParse("1.13.0"),
				},
				{
					name:    "numpy",
					version: version.MustParse("1.14.0"),
				},
				{
					name:    "numpy",
					version: version.MustParse("1.15.0"),
				},
			},
		},
	}

	verifyMinimalVersionSelection(
		t,
		index,
		[]Dependency{
			{
				Name: "torch",
			},
			{
				Name: "tensorflow",
			},
		},
		[]Dependency{
			{
				Name: "numpy",
				// Version selection should not blindly select the latest version in case of
				// a transitive dependency not specifying a version at all.
				// 1.14.0 is the highest lower bound specified by any transitive dependency.
				Version: version.MustParse("1.14.0"),
			},
			{
				Name:    "tensorflow",
				Version: version.MustParse("2.3.0"),
			},
			{
				Name:    "torch",
				Version: version.MustParse("1.6.0"),
			},
		},
		[]Dependency{
			{
				Name:    "tensorflow",
				Version: version.MustParse("2.3.0"),
			},
			{
				Name:    "torch",
				Version: version.MustParse("1.6.0"),
			},
		},
	)
}

func TestVersionSelectionUnboundedReproduceable(t *testing.T) {
	index := &testPackageIndex{
		map[string][]testPackage{
			"torch": {
				{
					name:    "torch",
					version: version.MustParse("1.6.0"),
					dependencies: []Dependency{
						{
							Name: "numpy",
						},
					},
				},
			},
			"numpy": {
				{
					name:    "numpy",
					version: version.MustParse("1.19.0"),
				},
				{
					name:    "numpy",
					version: version.MustParse("1.19.1"),
				},
			},
		},
	}

	minimal := []Dependency{
		{
			Name:    "numpy",
			Version: version.MustParse("1.19.1"),
		},
		{
			Name:    "torch",
			Version: version.MustParse("1.6.0"),
		},
	}
	verifyMinimalVersionSelection(
		t,
		index,
		[]Dependency{
			{
				Name: "torch",
			},
		},
		minimal,
		minimal,
	)

	// New version of numpy is released. Ensure build is reproduceable.
	index.index["numpy"] = append(index.index["numpy"], testPackage{
		name:    "numpy",
		version: version.MustParse("1.19.2"),
	})
	verifyMinimalVersionSelection(
		t,
		index,
		minimal,
		minimal,
		minimal,
	)
}

func verifyMinimalVersionSelection(
	t *testing.T,
	index PackageIndex,
	baseDependencies []Dependency,
	expectedBuild []Dependency,
	expectedMinimal []Dependency,
) {
	build, minimal, err := MinimalVersionSelection(context.Background(), baseDependencies, index)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(expectedBuild) != len(build) {
		t.Fatalf("build list != expected build list: got: %v, want: %v", build, expectedBuild)
	}
	for i := 0; i < len(expectedBuild); i++ {
		if expectedBuild[i].Name != build[i].Name || expectedBuild[i].Version.NE(build[i].Version.Version) {
			t.Fatalf("build list != expected build list: got: %v, want: %v", build, expectedBuild)
		}
	}

	if len(expectedMinimal) > 0 {
		if len(expectedMinimal) != len(minimal) {
			t.Fatalf("minimal list != expected minimal list: got: %v, want: %v", minimal, expectedMinimal)
		}
		for i := 0; i < len(expectedMinimal); i++ {
			if expectedMinimal[i].Name != minimal[i].Name || expectedMinimal[i].Version.NE(minimal[i].Version.Version) {
				t.Fatalf("minimal list != expected minimal list: got: %v, want: %v", minimal, expectedMinimal)
			}
		}
	}
}
