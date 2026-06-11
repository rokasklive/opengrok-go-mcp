// SPDX-License-Identifier: Apache-2.0

package evals

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func loadScenarios(dir string) ([]Scenario, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read scenarios dir: %w", err)
	}

	var scenarios []Scenario
	seen := map[string]struct{}{}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}

		var fileScenarios []Scenario
		if err := unmarshalScenarios(raw, &fileScenarios); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}

		for _, sc := range fileScenarios {
			if err := validateScenario(sc, path); err != nil {
				return nil, err
			}
			if _, ok := seen[sc.ID]; ok {
				return nil, fmt.Errorf("%s: duplicate scenario id %q", path, sc.ID)
			}
			seen[sc.ID] = struct{}{}
			scenarios = append(scenarios, sc)
		}
	}

	if len(scenarios) == 0 {
		return nil, fmt.Errorf("no scenarios found in %s", dir)
	}
	return scenarios, nil
}

func unmarshalScenarios(raw []byte, out *[]Scenario) error {
	// Allow a single object or an array in each file.
	if err := json.Unmarshal(raw, out); err == nil && len(*out) > 0 {
		return nil
	}
	var one Scenario
	if err := json.Unmarshal(raw, &one); err != nil {
		return err
	}
	*out = []Scenario{one}
	return nil
}

func validateScenario(sc Scenario, path string) error {
	if strings.TrimSpace(sc.ID) == "" {
		return fmt.Errorf("%s: scenario missing id", path)
	}
	if len(sc.Steps) == 0 {
		return fmt.Errorf("%s: scenario %q has no steps", path, sc.ID)
	}
	for i, step := range sc.Steps {
		if strings.TrimSpace(step.Op) == "" {
			return fmt.Errorf("%s: scenario %q step %d missing op", path, sc.ID, i)
		}
		if !isKnownOp(step.Op) {
			return fmt.Errorf("%s: scenario %q step %d unknown op %q", path, sc.ID, i, step.Op)
		}
	}
	return nil
}
