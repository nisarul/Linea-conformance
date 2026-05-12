// SPDX-License-Identifier: AGPL-3.0-or-later

package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Load walks `root` recursively and returns every *.json file
// decoded as a Vector. Vectors are returned sorted by their on-disk
// path so test output is stable across runs.
func Load(root string) ([]Vector, error) {
	var paths []string
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(strings.ToLower(p), ".json") {
			return nil
		}
		paths = append(paths, p)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)

	out := make([]Vector, 0, len(paths))
	for _, p := range paths {
		buf, err := os.ReadFile(p) //nolint:gosec // intentional: read user-controlled vector
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}
		var v Vector
		dec := json.NewDecoder(strings.NewReader(string(buf)))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&v); err != nil {
			return nil, fmt.Errorf("parse %s: %w", p, err)
		}
		// Validate id matches the file path under root.
		rel, _ := filepath.Rel(root, p)
		rel = strings.TrimSuffix(filepath.ToSlash(rel), ".json")
		if v.ID != rel {
			return nil, fmt.Errorf("vector %s: id %q does not match path %q", p, v.ID, rel)
		}
		out = append(out, v)
	}
	return out, nil
}
