// SPDX-License-Identifier: AGPL-3.0-or-later

package runner_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	runner "github.com/nisarul/Linea-conformance/runner-go"
)

// TestSuite walks the whole vectors/ tree and runs every vector,
// reporting per-vector pass/fail.
func TestSuite(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "vectors"))
	require.NoError(t, err)

	vectors, err := runner.Load(root)
	require.NoError(t, err)
	require.NotEmpty(t, vectors, "no vectors loaded — check path")

	ctx := context.Background()
	for _, v := range vectors {
		v := v
		t.Run(v.ID, func(t *testing.T) {
			res := runner.Run(ctx, v)
			if !res.Pass {
				t.Fatalf("vector %s failed: %s", v.ID, res.Reason)
			}
		})
	}
}
