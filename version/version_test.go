package version

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"
)

type versionTestCase struct {
	input     string
	output    Version
	canonical string
}

var versionTestCases = []versionTestCase{
	{
		"1!1.16rc3.post5.dev2+xyz",
		Version{
			Epoch:              1,
			Release:            [6]int{1, 16},
			PreReleasePhase:    PrereleaseCandidate,
			PreReleaseVersion:  3,
			PostRelease:        true,
			PostReleaseVersion: 5,
			DevRelease:         true,
			DevReleaseVersion:  2,
			LocalVersion:       "xyz",
		},
		"1!1.16rc3.post5.dev2+xyz",
	},
	{
		"1",
		Version{
			Release: [6]int{1},
		},
		"1",
	},
	{
		"1.2.3.4",
		Version{
			Release: [6]int{1, 2, 3, 4},
		},
		"1.2.3.4",
	},
	{
		"1.2-alpha",
		Version{
			Release:           [6]int{1, 2},
			PreReleasePhase:   PrereleaseAlpha,
			PreReleaseVersion: 0,
		},
		"1.2a0",
	},
	{
		"1.2-dev",
		Version{
			Release:           [6]int{1, 2},
			DevRelease:        true,
			DevReleaseVersion: 0,
		},
		"1.2.dev0",
	},
	{
		"0!4+latest-ubuntu",
		Version{
			Epoch:        0,
			Release:      [6]int{4},
			LocalVersion: "latest-ubuntu",
		},
		"4+latest-ubuntu",
	},
	{
		"1.0+abc.7",
		Version{
			Release:      [6]int{1, 0},
			LocalVersion: "abc.7",
		},
		"1.0+abc.7",
	},
	{
		"3.2.0b6",
		Version{
			Release:           [6]int{3, 2, 0},
			PreReleasePhase:   PrereleaseBeta,
			PreReleaseVersion: 6,
		},
		"3.2.0b6",
	},
	{
		"1.0.0-Beta",
		Version{
			Release:           [6]int{1, 0, 0},
			PreReleasePhase:   PrereleaseBeta,
			PreReleaseVersion: 0,
		},
		"1.0.0b0",
	},
	{
		"0.6.*",
		Version{
			Release:  [6]int{0, 6},
			Wildcard: true,
		},
		"0.6.*",
	},
}

func TestParse2(t *testing.T) {
	for _, tc := range versionTestCases {
		t.Run(tc.input, func(t *testing.T) {
			v, valid := Parse(tc.input)
			if !valid {
				t.Fatalf("unexpected invalid version: %s", tc.input)
			}
			if v.Epoch != tc.output.Epoch {
				t.Fatalf("wrong epoch, got: %d, expected: %d", v.Epoch, tc.output.Epoch)
			}
			if len(v.Release) != len(tc.output.Release) {
				t.Fatalf("wrong release, got: %d, expected: %d", v.Release, tc.output.Release)
			}
			for i := range v.Release {
				if v.Release[i] != tc.output.Release[i] {
					t.Fatalf("wrong release, got: %d, expected: %d", v.Release, tc.output.Release)
				}
			}
			if v.PreReleasePhase != tc.output.PreReleasePhase {
				t.Fatalf("wrong pre-release phase, got: %d, expected: %d", v.PreReleasePhase, tc.output.PreReleasePhase)
			}
			if v.PreReleaseVersion != tc.output.PreReleaseVersion {
				t.Fatalf("wrong pre-release version, got: %d, expected: %d", v.PreReleaseVersion, tc.output.PreReleaseVersion)
			}
			if v.PostRelease != tc.output.PostRelease && v.PostReleaseVersion != tc.output.PostReleaseVersion {
				t.Fatalf("wrong post-release, got: %d, expected: %d", v.PostReleaseVersion, tc.output.PostReleaseVersion)
			}
			if v.DevRelease != tc.output.DevRelease && v.DevReleaseVersion != tc.output.DevReleaseVersion {
				t.Fatalf("wrong dev-release, got: %d, expected: %d", v.DevReleaseVersion, tc.output.DevReleaseVersion)
			}
			if v.LocalVersion != tc.output.LocalVersion {
				t.Fatalf("wrong local version identifier, got: %s, expected: %s", v.LocalVersion, tc.output.LocalVersion)
			}
			if v.Wildcard != tc.output.Wildcard {
				t.Fatalf("wrong wildcard, got: %v, expected: %v", v.Wildcard, tc.output.Wildcard)
			}
			if v.Canonical() != tc.canonical {
				t.Fatalf("wrong canonical representation, got: %s, expected: %s", v.Canonical(), tc.canonical)
			}
		})
	}
}

func TestVersionEquality(t *testing.T) {
	testCases := []struct {
		v1    string
		v2    string
		equal bool
	}{
		{
			v1:    "3!4",
			v2:    "3!4",
			equal: true,
		},
		{
			v1:    "3.2.0",
			v2:    "3.2",
			equal: true,
		},
		{
			v1:    "4.3+abc",
			v2:    "4.3",
			equal: false,
		},
		{
			v1:    "1.3",
			v2:    "4.5",
			equal: false,
		},
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s-%s", tc.v1, tc.v2), func(t *testing.T) {
			v1 := MustParse(tc.v1)
			v2 := MustParse(tc.v2)
			equal := v1.Equal(v2)
			if v2.Equal(v1) != equal {
				t.Fatalf("equal should be reflexive: %s==%s != %s==%s", v1, v2, v2, v1)
			}

			if equal != tc.equal {
				t.Fatalf("wrong result %s==%s -> %v, expected: %v", v1, v2, equal, tc.equal)
			}
		})
	}
}

func TestVersionMatch(t *testing.T) {
	testCases := []struct {
		v1    string
		v2    string
		match bool
	}{
		{
			v1:    "3!4",
			v2:    "3!4",
			match: true,
		},
		{
			v1:    "3.2",
			v2:    "3.2.0",
			match: true,
		},
		//
		// {
		// 	v1:    "1.1",
		// 	v2:    "1.1.post1",
		// 	match: false,
		// },
		{
			v1:    "1.1.post1",
			v2:    "1.1.post1",
			match: true,
		},
		{
			v1:    "1.1.post1",
			v2:    "1.1.*",
			match: true,
		},
		{
			v1:    "1.2.post1",
			v2:    "1.1.*",
			match: false,
		},
		{
			v1:    "1.1",
			v2:    "1.1.0",
			match: true,
		},
		{
			v1:    "1.1",
			v2:    "1.1.dev1",
			match: false,
		},
		{
			v1:    "1.1",
			v2:    "1.1a1",
			match: false,
		},
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s-%s", tc.v1, tc.v2), func(t *testing.T) {
			v1 := MustParse(tc.v1)
			v2 := MustParse(tc.v2)
			match := v1.Match(v2)

			if match != tc.match {
				t.Fatalf("wrong result %s match %s -> %v, expected: %v", v1, v2, match, tc.match)
			}

			if v2.Match(v1) != match {
				t.Fatalf("match should be reflexive: %s match %s != %s match%s", v1, v2, v2, v1)
			}
		})
	}
}

func TestParseAllVersions(t *testing.T) {
	/*
		versions.json.gz contains the name, version, and requires_dist of every
		distribution uploaded after 2015-01-1 on PyPi.

			SELECT
			  name,
			  version,
			  requires_dist
			FROM
			  `the-psf.pypi.distribution_metadata`
			WHERE
			  upload_time > '2015-01-1'
			ORDER BY
			  name ASC

		Retrieved on 2020-09-02 through Google BigQuery.
	*/

	if testing.Short() {
		return
	}

	file, err := os.Open("testdata/versions.json.gz")
	if err != nil {
		t.Fatalf("failed opening test data file: %v", err)
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()

	r := json.NewDecoder(gz)
	type versionsRecord struct {
		Name         string   `json:"name"`
		Version      string   `json:"version"`
		RequiresDist []string `json:"requires_dist"`
	}

	env := testEnvironment{
		"extra": "",

		"os_name":                        "",
		"sys_platform":                   "",
		"platform_machine":               "",
		"platform_python_implementation": "",
		"platform_release":               "0",
		"platform_system":                "",
		"platform_version":               "0",
		"python_version":                 "0",
		"python_full_version":            "0",
		"implementation_name":            "",
		"implementation_version":         "0",
	}

	failed := 0
	failedDependencies := 0
	total := 0
	totalDependencies := 0
	canonical := 0
	for {
		var rec versionsRecord
		if err := r.Decode(&rec); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("unexpected error reading json: %v", err)
		}

		v, valid := Parse(rec.Version)
		total++
		if !valid {
			failed++
			// t.Logf("%s: %s", rec.Name, rec.Version)
		} else {
			if rec.Version == v.Canonical() {
				canonical++
			} else {
				// t.Logf("%s!=%s", rec.Version, v.Canonical())
			}
		}

		for _, requiresDist := range rec.RequiresDist {
			ds, err := ParseDependency(requiresDist)
			totalDependencies++
			if err != nil {
				failedDependencies++
				// t.Logf("%s-%s '%s' err: %v", rec.Name, rec.Version, requiresDist, err)
			} else {
				if _, err := ds.Evaluate(env); err != nil {
					t.Errorf("evaluation failure: %s-%s '%s' err: %v", rec.Name, rec.Version, requiresDist, err)
				}
			}
		}
	}

	failureRate := float64(failed) / float64(total)
	t.Logf("Failed %d out of %d (%.4f%%)", failed, total, failureRate*100)
	if failureRate > 0.0005 {
		t.FailNow()
	}
	canonicalRate := float64(canonical) / float64(total)
	t.Logf("Canonical form %d out of %d (%.4f%%)", canonical, total, canonicalRate*100)
	if canonicalRate < 0.99 {
		t.FailNow()
	}
	failureDependenciesRate := float64(failedDependencies) / float64(totalDependencies)
	t.Logf("Failed dependencies %d out of %d (%.4f%%)", failedDependencies, totalDependencies, failureDependenciesRate*100)
	if failureDependenciesRate > 0.001 {
		t.FailNow()
	}
}

func BenchmarkVersionParsing(b *testing.B) {
	for _, tc := range versionTestCases {
		b.Run(tc.input, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				Parse(tc.input)
			}
		})
	}
}

func TestVersionComparison(t *testing.T) {
	testCases := []struct {
		a      string
		b      string
		output int
	}{
		{
			"3.2", "3.4", -1,
		},
		{
			"3.2", "3.2", 0,
		},
		{
			"3.2+a", "3.2+b", 0,
		},
		{
			"1!3", "5.3", 1,
		},
		{
			"4.3", "4.3.dev4", 1,
		},
		{
			"4.3b4", "4.3a2", 1,
		},
		{
			"4.3b4", "4.3a6", 1,
		},
		{
			"4.3", "4.3b6", 1,
		},
		{
			"1.2rc1", "1.2", -1,
		},
		{
			"4.3.post1", "4.3", 1,
		},
		{
			"4.3.dev3", "4.3.dev2", 1,
		},
		{
			"4.3.post2", "4.3.post1", 1,
		},
		{
			"2.2.0", "2.3.0", -1,
		},
		{
			"1.12.0", "1.6.1", 1,
		},
		{
			"0.5.0", "0.5", 0,
		},
		{
			"1.11.0rc2", "1.11.0rc1", 1,
		},
		{
			"1.11.dev4", "1.11.dev3", 1,
		},
		{
			"0.22rc3", "0.22rc2.post1", 1,
		},
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s-%s", tc.a, tc.b), func(t *testing.T) {
			a := MustParse(tc.a)
			b := MustParse(tc.b)
			if Compare(a, b) != tc.output {
				t.Fatalf("compare(%s, %s) got: %d, expected: %d", a, b, Compare(a, b), tc.output)
			}
			// Verify that swapping the argument order produces the opposite ordering
			if Compare(b, a) != -1*tc.output {
				t.Fatalf("compare(%s, %s) got: %d, expected: %d", b, a, Compare(b, a), -1*tc.output)
			}
		})
	}
}
