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
		"numpy": {
			steps: []step{
				{
					args:             []string{"init"},
					expectedExitCode: 0,
				},
				{
					args:             []string{"add", "numpy==1.15"},
					expectedExitCode: 0,
				},
			},
			verification: `import numpy; numpy.zeros([1,5])`,
		},
		"wrapt": {
			steps: []step{
				{
					args:             []string{"init"},
					expectedExitCode: 0,
				},
				{
					args:             []string{"add", "wrapt"},
					expectedExitCode: 0,
				},
			},
			verification: `import wrapt`,
		},
		"torch": {
			steps: []step{
				{
					args:             []string{"init"},
					expectedExitCode: 0,
				},
				{
					args:             []string{"add", "torch"},
					expectedExitCode: 0,
				},
			},
			verification: `import torch; torch.tensor([1.0,2.0,3.0]).softmax(-1)`,
			slow:         true,
		},
		"tensorflow": {
			steps: []step{
				{
					args:             []string{"init"},
					expectedExitCode: 0,
				},
				{
					args:             []string{"add", "tensorflow==2.2.0"},
					expectedExitCode: 0,
				},
			},
			verification: `import tensorflow`,
			slow:         true,
		},
		"urllib3 and botocore": {
			steps: []step{
				{
					args:             []string{"init"},
					expectedExitCode: 0,
				},
				{
					args:             []string{"add", "urllib3", "botocore"},
					expectedExitCode: 0,
				},
			},
			verification: `import urllib3; import botocore`,
			slow:         true,
		},
		"with weird casing": {
			steps: []step{
				{
					args:             []string{"init"},
					expectedExitCode: 0,
				},
				{
					args:             []string{"add", "nUmPy"},
					expectedExitCode: 0,
				},
			},
			verification: `import numpy`,
		},
		// - pendulum==1.4.4 provides a tar archive without directory entries
		//   and poorly specified transitive dependency.
		// - python-dateutil poses name normalization challenges.
		// "pendulum": {
		// 	steps: []step{
		// 		{
		// 			args:             []string{"init"},
		// 			expectedExitCode: 0,
		// 		},
		// 		{
		// 			args:             []string{"add", "python-dateutil", "pendulum==1.4.4", "six", "pytzdata"},
		// 			expectedExitCode: 0,
		// 		},
		// 	},
		// 	verification: `import pendulum`,
		// },
		// markdown is distributed as 'Markdown' and optionally requires importlib_metadata depending
		// on Python version.
		"markdown": {
			steps: []step{
				{
					args:             []string{"init"},
					expectedExitCode: 0,
				},
				{
					args:             []string{"add", "markdown"},
					expectedExitCode: 0,
				},
			},
			verification: `import markdown`,
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
