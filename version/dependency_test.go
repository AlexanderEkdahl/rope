package version

import (
	"fmt"
	"testing"
)

func TestParseDependencySpecifications(t *testing.T) {
	testCases := []struct {
		input  string
		result Dependency
		err    error
	}{
		{
			" numpy",
			Dependency{
				"numpy",
				[]Requirement{},
				[]string{},
				nil,
			},
			nil,
		},
		{
			"A ( >=3.1.2)",
			Dependency{
				"A",
				[]Requirement{
					{Operator: GreaterOrEqual, Version: MustParse("3.1.2")},
				},
				[]string{},
				nil,
			},
			nil,
		},
		{
			"A.B-C_D[security]",
			Dependency{
				"A.B-C_D",
				[]Requirement{},
				[]string{"security"},
				nil,
			},
			nil,
		},
		{
			"name<=1",
			Dependency{
				"name",
				[]Requirement{
					{Operator: LessOrEqual, Version: MustParse("1")},
				},
				[]string{},
				nil,
			},
			nil,
		},
		{
			"name[ extras , potato]<=1",
			Dependency{
				"name",
				[]Requirement{
					{Operator: LessOrEqual, Version: MustParse("1")},
				},
				[]string{"extras", "potato"},
				nil,
			},
			nil,
		},
		{
			"name>=3,<2",
			Dependency{
				"name",
				[]Requirement{
					{Operator: GreaterOrEqual, Version: MustParse("3")},
					{Operator: Less, Version: MustParse("2")},
				},
				[]string{},
				nil,
			},
			nil,
		},
		{
			"name@http://foo.com",
			Dependency{},
			ErrURLNotSupported,
		},
		{
			"python-dateutil>=2.1,<3.0.0",
			Dependency{
				"python-dateutil",
				[]Requirement{
					{Operator: GreaterOrEqual, Version: MustParse("2.1")},
					{Operator: Less, Version: MustParse("3.0.0")},
				},
				[]string{},
				nil,
			},
			nil,
		},
		{
			"apache-beam[gcp] (<3,>=2.21)",
			Dependency{
				"apache-beam",
				[]Requirement{
					{Operator: Less, Version: MustParse("3")},
					{Operator: GreaterOrEqual, Version: MustParse("2.21")},
				},
				[]string{"gcp"},
				nil,
			},
			nil,
		},
		{
			// Missing comma between versions
			"htmldoom (>=0.3<=0.4)",
			Dependency{
				"htmldoom",
				[]Requirement{
					{Operator: GreaterOrEqual, Version: MustParse("0.3")},
					{Operator: LessOrEqual, Version: MustParse("0.4")},
				},
				nil,
				nil,
			},
			nil,
		},
		{
			"check-manifest; extra == 'dev'",
			Dependency{
				"check-manifest",
				[]Requirement{},
				nil,
				nil,
			},
			nil,
		},
		{
			`check-test[dev] (!=1!3.2); platform_machine!="windows"`,
			Dependency{
				"check-test",
				[]Requirement{
					{Operator: NotEqual, Version: MustParse("1!3.2")},
				},
				[]string{"dev"},
				nil,
			},
			nil,
		},
		{
			`functools32 (>=3.2.3) ; python_version < "3"`,
			Dependency{
				"functools32",
				[]Requirement{
					{Operator: GreaterOrEqual, Version: MustParse("3.2.3")},
				},
				nil,
				nil,
			},
			nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			r, err := ParseDependency(tc.input)
			if err != tc.err {
				t.Fatalf("unexpected error, got: %v, want: %v", err, tc.err)
			}
			if err == nil {
				if tc.result.Name != r.Name {
					t.Fatalf("incorrect distribution name, got: %v, want: %v", r.Name, tc.result.Name)
				}
				if len(tc.result.Versions) != len(r.Versions) {
					t.Fatalf("incorrect versions, got: %v, want: %v", r.Versions, tc.result.Versions)
				}
				for i := 0; i < len(tc.result.Versions); i++ {
					if tc.result.Versions[i] != r.Versions[i] {
						t.Fatalf("incorrect versions, got: %v, want: %v", r.Versions, tc.result.Versions)
					}
				}
				if len(tc.result.Extras) != len(r.Extras) {
					t.Fatalf("incorrect extras, got: %v, want: %v", r.Extras, tc.result.Extras)
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

type testEnvironment map[string]string

func (e testEnvironment) Get(k string) (string, error) {
	v, ok := e[k]
	if !ok {
		return "", fmt.Errorf("unknown environment variable: '%s'", k)
	}
	return v, nil
}

func TestDependencyEvaluation(t *testing.T) {
	env := testEnvironment{
		"extra": "test",

		"os_name":                        "",
		"sys_platform":                   "",
		"platform_machine":               "",
		"platform_python_implementation": "",
		"platform_release":               "0",
		"platform_system":                "",
		"platform_version":               "0",
		"python_version":                 "3.6",
		"python_full_version":            "0",
		"implementation_name":            "",
		"implementation_version":         "0",
	}

	testCases := []struct {
		input   string
		install bool
	}{
		{
			input:   `numpy`,
			install: true,
		},
		{
			input:   `numpy (>=1.16.0, <1.19.0) ; (python_version == "3.6") and extra == 'test'`,
			install: true,
		},
		{
			input:   `numpy[test, windows]`,
			install: true,
		},
		{
			input:   `numpy[windows]`,
			install: false,
		},
		{
			input:   `enum34; (python_version=='2.7' or python_version=='2.6' or python_version=='3.3')`,
			install: false,
		},
		{
			input:   `test; python_version>'2.7'`,
			install: true,
		},
		{
			input:   `test; python_version<'4'`,
			install: true,
		},
		{
			input:   `test; python_version>'3.8'`,
			install: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			d, err := ParseDependency(tc.input)
			if err != nil {
				t.Fatal(err)
			}

			install, err := d.Evaluate(env)
			if err != nil {
				t.Fatal(err)
			}

			if install != tc.install {
				t.Fatalf("unexpected evaluation result, got: %v, expected: %v", install, tc.install)
			}
		})
	}
}
