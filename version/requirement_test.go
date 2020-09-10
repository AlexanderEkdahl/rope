package version

import (
	"testing"
)

func TestMinimal(t *testing.T) {
	testCases := []struct {
		input  string
		output Version
	}{
		{
			"<1.19.0, >=1.16.0",
			MustParse("1.16.0"),
		},
		{
			"<1.3.4, >=1.3.6",
			MustParse("1.3.6"),
		},
		{
			">=1.8.6",
			MustParse("1.8.6"),
		},
		{
			"!=2.0.*,<3,>=1.15",
			MustParse("1.15"),
		},
	}
	for _, tC := range testCases {
		t.Run(tC.input, func(t *testing.T) {
			vrs, err := ParseVersionRequirements(tC.input)
			if err != nil {
				t.Fatalf("unexpected error, got: %v", err)
			}
			if min := Minimal(vrs); !min.Equal(tC.output) {
				t.Fatalf("incorrect minimal version, got: %s, want: %s", min, tC.output)
			}
		})
	}
}
