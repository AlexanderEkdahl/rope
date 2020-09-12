package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestIndexVersionMatching(t *testing.T) {
	/*
		When iterating through the index multiple package may match the target
		version. In that case the most appropriate version must be selected.
	*/
	testCases := []struct {
		a         string
		b         string
		reflexive bool
	}{
		// TODO: Prefer manylinux2010 over manylinux1
		// {
		// 	"numpy-1.19.2-cp37-cp37m-manylinux1_x86_64.whl",
		// 	"numpy-1.19.2-cp37-cp37m-manylinux2010_x86_64.whl",
		// 	true,
		// },
		{
			"x-1.19.2-cp37-cp37m-manylinux2010_x86_64.whl",
			"x-1.19.2-py.py3-none-any.whl",
			true,
		},
		{
			"six-1.15.0-py2.py3-none-any.whl",
			"six-1.15.0.tar.gz",
			true,
		},
		{
			"wheel-0.1.tgz",
			"wheel-0.1.zip",
			false,
		},
	}
	for i, tc := range testCases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			var a Package
			if strings.HasSuffix(tc.a, ".whl") {
				a, _ = ParseWheelFilename(tc.a)
			} else {
				a, _ = ParseSdistFilename(tc.a, filepath.Ext(tc.a))
			}
			var b Package
			if strings.HasSuffix(tc.b, ".whl") {
				b, _ = ParseWheelFilename(tc.b)
			} else {
				b, _ = ParseSdistFilename(tc.b, filepath.Ext(tc.b))
			}
			if !Preferred(a, b) {
				t.Fatalf("expected %s to be preferred over %s", tc.a, tc.b)
			}
			if tc.reflexive && Preferred(b, a) {
				t.Fatalf("did not expect %s to be preferred over %s", tc.b, tc.a)
			}
		})
	}
}
