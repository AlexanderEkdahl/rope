package main

import (
	"testing"
)

func TestParsingSdist(t *testing.T) {
	sdist, err := ParseSdistFilename("python-slugify-3.0.0.tar.gz", ".tar.gz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sdist.name != "python-slugify" {
		t.Fatalf("wrong name, got: %s, expected: %s", sdist.name, "python-slugify")
	}
	if sdist.version.String() != "3.0.0" {
		t.Fatalf("wrong version, got: %s, expected: %s", sdist.version, "3.0.0")
	}
}
