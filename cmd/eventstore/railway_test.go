package main

import (
	"encoding/json"
	"os"
	"testing"
)

// railwayConfigPath is the Railway deploy configuration at the repository root.
const railwayConfigPath = "../../railway.json"

// expectedReplicas is the single replica the in-memory login lockout relies on.
const expectedReplicas = 1

type railwayConfig struct {
	Deploy struct {
		HealthcheckPath string `json:"healthcheckPath"`
		NumReplicas     int    `json:"numReplicas"`
	} `json:"deploy"`
}

func TestRailwayConfigMatchesHealthRouteAndSingleReplica(t *testing.T) {
	raw, err := os.ReadFile(railwayConfigPath)
	if err != nil {
		t.Fatalf("read %s: %v", railwayConfigPath, err)
	}
	var cfg railwayConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("decode %s: %v", railwayConfigPath, err)
	}

	if cfg.Deploy.HealthcheckPath != healthPath {
		t.Errorf("deploy.healthcheckPath = %q, want %q", cfg.Deploy.HealthcheckPath, healthPath)
	}
	if cfg.Deploy.NumReplicas != expectedReplicas {
		t.Errorf("deploy.numReplicas = %d, want %d", cfg.Deploy.NumReplicas, expectedReplicas)
	}
}
