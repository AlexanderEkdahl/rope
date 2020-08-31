// +build integration

package integration

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestCLI(t *testing.T) {
	type step struct {
		args             []string
		expectedExitCode int
	}

	testCases := map[string]struct {
		steps        []step
		verification string
		slow         bool
	}{
		"version": {
			steps: []step{
				{
					args:             []string{"version"},
					expectedExitCode: 0,
				},
			},
		},
		"init": {
			steps: []step{
				{
					args:             []string{"init"},
					expectedExitCode: 0,
				},
			},
		},
		"install numpy": {
			steps: []step{
				{
					args:             []string{"init"},
					expectedExitCode: 0,
				},
				{
					args:             []string{"install", "numpy==1.15"},
					expectedExitCode: 0,
				},
			},
			verification: `import numpy; numpy.zeros([1,5])`,
		},
		"install wrapt": {
			steps: []step{
				{
					args:             []string{"init"},
					expectedExitCode: 0,
				},
				{
					args:             []string{"install", "wrapt"},
					expectedExitCode: 0,
				},
			},
			verification: `import wrapt`,
		},
		"install torch": {
			steps: []step{
				{
					args:             []string{"init"},
					expectedExitCode: 0,
				},
				{
					args:             []string{"install", "torch"},
					expectedExitCode: 0,
				},
			},
			verification: `import torch; torch.tensor([1.0,2.0,3.0]).softmax(-1)`,
			slow:         true,
		},
		"install tensorflow": {
			steps: []step{
				{
					args:             []string{"init"},
					expectedExitCode: 0,
				},
				{
					args:             []string{"install", "tensorflow==2.2.0"},
					expectedExitCode: 0,
				},
			},
			verification: `import tensorflow`,
			slow:         true,
		},
		"install urllib3 and botocore": {
			steps: []step{
				{
					args:             []string{"init"},
					expectedExitCode: 0,
				},
				{
					args:             []string{"install", "urllib3", "botocore"},
					expectedExitCode: 0,
				},
			},
			verification: `import urllib3; import botocore`,
		},
		"install with weird casing": {
			steps: []step{
				{
					args:             []string{"init"},
					expectedExitCode: 0,
				},
				{
					args:             []string{"install", "nUmPy"},
					expectedExitCode: 0,
				},
			},
			verification: `import numpy`,
		},
	}
	for name, tc := range testCases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			if testing.Short() && tc.slow {
				t.Skip()
			}
			t.Parallel()

			ctx := context.Background()
			if deadline, ok := t.Deadline(); ok {
				var cancel context.CancelFunc
				ctx, cancel = context.WithDeadline(ctx, deadline)
				defer cancel()
			}

			tmp := t.TempDir()
			for _, step := range tc.steps {
				t0 := time.Now()
				cmd := exec.CommandContext(ctx, "rope", step.args...)
				cmd.Dir = tmp

				output, err := cmd.CombinedOutput()
				if cmd.ProcessState.ExitCode() != step.expectedExitCode {
					t.Errorf("wrong exit code, got: %d, expected: %d", cmd.ProcessState.ExitCode(), step.expectedExitCode)
				} else if err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				if t.Failed() {
					t.Log(string(output))
				} else {
					t.Logf("'rope %s' finished in %.3fs", strings.Join(step.args, " "), time.Since(t0).Seconds())
				}
			}

			if !t.Failed() && tc.verification != "" {
				cmd := exec.CommandContext(ctx, "rope", "run", "python", "-c", tc.verification)
				cmd.Dir = tmp
				output, err := cmd.CombinedOutput()
				if cmd.ProcessState.ExitCode() != 0 {
					t.Errorf("verification failed with exit code: %d, expected: %d", cmd.ProcessState.ExitCode(), 0)
				} else if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if t.Failed() {
					t.Log(string(output))
				}
			}
		})
	}
}
