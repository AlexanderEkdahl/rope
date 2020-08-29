package main

import (
	"fmt"
	"testing"
)

func TestParseWheelFilename(t *testing.T) {
	x, _ := ParseWheelFilename("distribution-1.0-1-py27-none-any.whl")
	t.Logf("%#v\n", x)
	x, _ = ParseWheelFilename("tqdm-4.48.2-py2.py3-none-any.whl")
	t.Logf("%#v\n", x)
	x, _ = ParseWheelFilename("numpy-1.14.5-cp27-cp27m-macosx_10_6_intel.macosx_10_9_intel.macosx_10_9_x86_64.macosx_10_10_intel.macosx_10_10_x86_64.whl")
	t.Logf("%#v\n", x)
}

func TestCompatiblePython(t *testing.T) {
	testCases := []struct {
		a      string
		b      string
		result bool
	}{
		{
			"py2.py3",
			"cp36",
			true,
		},
		{
			"cp3",
			"cp36",
			true,
		},
		{
			"cp27",
			"cp36",
			false,
		},
	}
	for i, tC := range testCases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			result := CompatiblePython(tC.a, tC.b)
			if result != tC.result {
				t.Fatalf("CompatiblePython(%s, %s) == %v, expected: %v", tC.a, tC.b, result, tC.result)
			}
		})
	}
}

func TestCompatiblePlatform(t *testing.T) {
	testCases := []struct {
		platform string
		goos     string
		goarch   string
		result   bool
	}{
		{
			"macosx_10_6_intel.macosx_10_9_intel.macosx_10_9_x86_64.macosx_10_10_intel.macosx_10_10_x86_64",
			"darwin",
			"amd64",
			true,
		},
		{
			"any",
			"darwin",
			"amd64",
			true,
		},
		{
			"manylinux2014_aarch64",
			"linux",
			"amd64",
			false,
		},
		{
			"manylinux2010_x86_64",
			"linux",
			"amd64",
			true,
		},
	}
	for i, tC := range testCases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			result := CompatiblePlatform(tC.platform, tC.goos, tC.goarch)
			if result != tC.result {
				t.Fatalf("CompatiblePlatform(%s, %s) == %v, expected: %v", tC.platform, tC.goos, result, tC.result)
			}
		})
	}
}
