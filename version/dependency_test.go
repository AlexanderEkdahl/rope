package version

import (
	"testing"
)

func TestParseDependencySpecifications(t *testing.T) {
	testCases := []struct {
		input  string
		result DependencySpecification
		err    error
	}{
		{
			" numpy",
			DependencySpecification{
				"numpy",
				[]VersionRequirement{},
				[]string{},
			},
			nil,
		},
		{
			"A ( >=3.1.2)",
			DependencySpecification{
				"A",
				[]VersionRequirement{
					{Operator: GreaterOrEqual, Version: MustParse("3.1.2")},
				},
				[]string{},
			},
			nil,
		},
		{
			"A.B-C_D[security]",
			DependencySpecification{
				"A.B-C_D",
				[]VersionRequirement{},
				[]string{"security"},
			},
			nil,
		},
		{
			"name<=1",
			DependencySpecification{
				"name",
				[]VersionRequirement{
					{Operator: LessOrEqual, Version: MustParse("1")},
				},
				[]string{},
			},
			nil,
		},
		{
			"name[ extras , potato]<=1",
			DependencySpecification{
				"name",
				[]VersionRequirement{
					{Operator: LessOrEqual, Version: MustParse("1")},
				},
				[]string{"extras", "potato"},
			},
			nil,
		},
		{
			"name>=3,<2",
			DependencySpecification{
				"name",
				[]VersionRequirement{
					{Operator: GreaterOrEqual, Version: MustParse("3")},
					{Operator: Less, Version: MustParse("2")},
				},
				[]string{},
			},
			nil,
		},
		{
			"name@http://foo.com",
			DependencySpecification{},
			ErrURLNotSupported,
		},
		{
			"python-dateutil>=2.1,<3.0.0",
			DependencySpecification{
				"python-dateutil",
				[]VersionRequirement{
					{Operator: GreaterOrEqual, Version: MustParse("2.1")},
					{Operator: Less, Version: MustParse("3.0.0")},
				},
				[]string{},
			},
			nil,
		},
		{
			"apache-beam[gcp] (<3,>=2.21)",
			DependencySpecification{
				"apache-beam",
				[]VersionRequirement{
					{Operator: Less, Version: MustParse("3")},
					{Operator: GreaterOrEqual, Version: MustParse("2.21")},
				},
				[]string{"gcp"},
			},
			nil,
		},
		{
			// Missing comma between versions
			"htmldoom (>=0.3<=0.4)",
			DependencySpecification{
				"htmldoom",
				[]VersionRequirement{
					{Operator: GreaterOrEqual, Version: MustParse("0.3")},
					{Operator: LessOrEqual, Version: MustParse("0.4")},
				},
				nil,
			},
			nil,
		},
		{
			"check-manifest; extra == 'dev'",
			DependencySpecification{
				"check-manifest",
				[]VersionRequirement{},
				[]string{"dev"},
			},
			nil,
		},
		{
			`functools32 (>=3.2.3) ; python_version < "3"`,
			DependencySpecification{},
			ErrUnknownEnvironmentMarker,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			r, err := ParseDependencySpecification(tc.input)
			if err != tc.err {
				t.Fatalf("unexpected error, got: %v, want: %v", err, tc.err)
			}
			if err == nil {
				if tc.result.DistributionName != r.DistributionName {
					t.Fatalf("incorrect distribution name, got: %v, want: %v", r.DistributionName, tc.result.DistributionName)
				}
				if len(tc.result.Versions) != len(r.Versions) {
					t.Fatalf("incorrect versions, got: %v, want: %v", r.Versions, tc.result.Versions)
				}
				for i := 0; i < len(tc.result.Versions); i++ {
					if !tc.result.Versions[i].Equal(r.Versions[i].Version) {
						t.Fatalf("incorrect versions, got: %v, want: %v", r.Versions, tc.result.Versions)
					}
				}
				if len(tc.result.Extras) != len(r.Extras) {
					t.Fatalf("incorrect versions, got: %v, want: %v", r.Versions, tc.result.Versions)
				}
				for i := 0; i < len(tc.result.Extras); i++ {
					if tc.result.Extras[i] != r.Extras[i] {
						t.Fatalf("incorrect extras, got: %v, want: %v", r.Extras, tc.result.Extras)
					}
				}
			}
		})
	}
}
