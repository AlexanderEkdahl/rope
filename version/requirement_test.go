package version

import (
	"fmt"
	"testing"
)

func TestMinimal(t *testing.T) {
	testCases := []struct {
		input  []Requirement
		output Version
	}{
		{
			[]Requirement{
				{Less, MustParse("1.19.0")},
				{GreaterOrEqual, MustParse("1.16.0")},
			},
			MustParse("1.16.0"),
		},
		{
			[]Requirement{
				{Less, MustParse("1.3.4")},
				{GreaterOrEqual, MustParse("1.3.6")},
			},
			MustParse("1.3.6"),
		},
		{
			[]Requirement{
				{GreaterOrEqual, MustParse("1.8.6")},
			},
			MustParse("1.8.6"),
		},
		{
			[]Requirement{
				{NotEqual, MustParse("2.0.*")},
				{Less, MustParse("3")},
				{GreaterOrEqual, MustParse("1.15")},
			},
			MustParse("1.15"),
		},
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s", tc.input), func(t *testing.T) {
			if min := Minimal(tc.input); min != tc.output {
				t.Fatalf("incorrect minimal version, got: %s, want: %s", min, tc.output)
			}
		})
	}
}

func TestRequirementContains(t *testing.T) {
	vr, _ := ParseVersionRequirements(">= 3.6")
	if vr[0].Contains(MustParse("3.5")) {
		t.Fatalf("did not expect >=3.6 to contain 3.5")
	}
}
