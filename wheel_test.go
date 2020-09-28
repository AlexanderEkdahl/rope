package main

import (
	"testing"
)

func TestParseWheelFilename(t *testing.T) {
	// TODO: Actually test things
	x, _ := ParseWheelFilename("distribution-1.0-1-py27-none-any.whl")
	t.Logf("%#v\n", x)
	x, _ = ParseWheelFilename("tqdm-4.48.2-py2.py3-none-any.whl")
	t.Logf("%#v\n", x)
	x, _ = ParseWheelFilename("numpy-1.14.5-cp27-cp27m-macosx_10_6_intel.macosx_10_9_intel.macosx_10_9_x86_64.macosx_10_10_intel.macosx_10_10_x86_64.whl")
	t.Logf("%#v\n", x)
	x, _ = ParseWheelFilename("x-1.0-py2.py3-none-any.whl")
	t.Logf("%#v\n", x)
	x, _ = ParseWheelFilename("x-1.0-0a-py2.py3-none-any.whl")
	t.Logf("%#v\n", x)
}
