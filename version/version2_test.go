package version

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"testing"
)

type versionTestCase struct {
	input     string
	output    Version2
	canonical string
}

var versionTestCases = []versionTestCase{
	{
		"1!1.16rc3.post5.dev2+xyz",
		Version2{
			Epoch:              1,
			Release:            [6]int{1, 16},
			PrereleasePhase:    PrereleaseCandidate,
			PrereleaseVersion:  3,
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
		Version2{
			Release: [6]int{1},
		},
		"1",
	},
	{
		"1.2.3.4",
		Version2{
			Release: [6]int{1, 2, 3, 4},
		},
		"1.2.3.4",
	},
	{
		"1.2-alpha",
		Version2{
			Release:           [6]int{1, 2},
			PrereleasePhase:   PrereleaseAlpha,
			PrereleaseVersion: 0,
		},
		"1.2a0",
	},
	{
		"1.2-dev",
		Version2{
			Release:           [6]int{1, 2},
			DevRelease:        true,
			DevReleaseVersion: 0,
		},
		"1.2.dev0",
	},
	{
		"0!4+latest-ubuntu",
		Version2{
			Epoch:        0,
			Release:      [6]int{4},
			LocalVersion: "latest-ubuntu",
		},
		"4+latest-ubuntu",
	},
	{
		"1.0+abc.7",
		Version2{
			Release:      [6]int{1, 0},
			LocalVersion: "abc.7",
		},
		"1.0+abc.7",
	},
	{
		"3.2.0b6",
		Version2{
			Release:           [6]int{3, 2, 0},
			PrereleasePhase:   PrereleaseBeta,
			PrereleaseVersion: 6,
		},
		"3.2.0b6",
	},
	{
		"1.0.0-Beta",
		Version2{
			Release:           [6]int{1, 0, 0},
			PrereleasePhase:   PrereleaseBeta,
			PrereleaseVersion: 0,
		},
		"1.0.0b0",
	},
}

func TestParse2(t *testing.T) {
	for _, tc := range versionTestCases {
		t.Run(tc.input, func(t *testing.T) {
			v, valid := Parse2(tc.input)
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
			if v.PrereleasePhase != tc.output.PrereleasePhase {
				t.Fatalf("wrong pre-release phase, got: %d, expected: %d", v.PrereleasePhase, tc.output.PrereleasePhase)
			}
			if v.PrereleaseVersion != tc.output.PrereleaseVersion {
				t.Fatalf("wrong pre-release version, got: %d, expected: %d", v.PrereleaseVersion, tc.output.PrereleaseVersion)
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
			if v.Canonical() != tc.canonical {
				t.Fatalf("wrong canonical representation, got: %s, expected: %s", v.Canonical(), tc.canonical)
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

		v, valid := Parse2(rec.Version)
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
			_, err := ParseDependencySpecification(requiresDist)
			totalDependencies++
			if err != nil {
				failedDependencies++
				// t.Errorf("'%s' err: %v", requiresDist, err)
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
}

func BenchmarkVersionParsing(b *testing.B) {
	for _, tc := range versionTestCases {
		b.Run(tc.input, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				Parse2(tc.input)
			}
		})
	}
}
