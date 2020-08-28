package pep508

import (
	"testing"

	"github.com/AlexanderEkdahl/rope/version"
)

func TestParsePep508Dependency(t *testing.T) {
	testCases := []struct {
		input  string
		result PEP508Dependency
		err    error
	}{
		{
			" numpy",
			PEP508Dependency{
				"numpy",
				[]version.VersionRequirement{},
				[]string{},
			},
			nil,
		},
		{
			"A ( >=3.1.2)",
			PEP508Dependency{
				"A",
				[]version.VersionRequirement{
					{Operator: version.GreaterOrEqual, Version: version.MustParse("3.1.2")},
				},
				[]string{},
			},
			nil,
		},
		{
			"A.B-C_D[security]",
			PEP508Dependency{
				"A.B-C_D",
				[]version.VersionRequirement{},
				[]string{"security"},
			},
			nil,
		},
		{
			"name<=1",
			PEP508Dependency{
				"name",
				[]version.VersionRequirement{
					{Operator: version.LessOrEqual, Version: version.MustParse("1")},
				},
				[]string{},
			},
			nil,
		},
		{
			"name[ extras , potato]<=1",
			PEP508Dependency{
				"name",
				[]version.VersionRequirement{
					{Operator: version.LessOrEqual, Version: version.MustParse("1")},
				},
				[]string{"extras", "potato"},
			},
			nil,
		},
		{
			"name>=3,<2",
			PEP508Dependency{
				"name",
				[]version.VersionRequirement{
					{Operator: version.GreaterOrEqual, Version: version.MustParse("3")},
					{Operator: version.Less, Version: version.MustParse("2")},
				},
				[]string{},
			},
			nil,
		},
		{
			"name@http://foo.com",
			PEP508Dependency{},
			ErrUrlNotSupported,
		},
		{
			"python-dateutil>=2.1,<3.0.0",
			PEP508Dependency{
				"python-dateutil",
				[]version.VersionRequirement{
					{Operator: version.GreaterOrEqual, Version: version.MustParse("2.1")},
					{Operator: version.Less, Version: version.MustParse("3.0.0")},
				},
				[]string{},
			},
			nil,
		},
		{
			"apache-beam[gcp] (<3,>=2.21)",
			PEP508Dependency{
				"apache-beam",
				[]version.VersionRequirement{
					{Operator: version.Less, Version: version.MustParse("3")},
					{Operator: version.GreaterOrEqual, Version: version.MustParse("2.21")},
				},
				[]string{"gcp"},
			},
			nil,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.input, func(t *testing.T) {
			r, err := ParseDependency(tC.input)
			if err != tC.err {
				t.Fatalf("unexpected error, got: %v, want: %v", err, tC.err)
			}
			if err == nil {
				if tC.result.DistributionName != r.DistributionName {
					t.Fatalf("incorrect distribution name, got: %v, want: %v", r.DistributionName, tC.result.DistributionName)
				}
				if len(tC.result.Versions) != len(r.Versions) {
					t.Fatalf("incorrect versions, got: %v, want: %v", r.Versions, tC.result.Versions)
				}
				for i := 0; i < len(tC.result.Versions); i++ {
					if tC.result.Versions[i].Version.NE(r.Versions[i].Version.Version) {
						t.Fatalf("incorrect versions, got: %v, want: %v", r.Versions, tC.result.Versions)
					}
				}
			}
		})
	}
}
