package main

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

// railwayConfigPath is the Railway deploy configuration at the repository root.
const railwayConfigPath = "../../railway.json"

// expectedReplicas is the single replica the in-memory login lockout relies on.
const expectedReplicas = 1

type railwayConfig struct {
	Deploy struct {
		HealthcheckPath string `json:"healthcheckPath"`
		NumReplicas     int    `json:"numReplicas"`
		// DrainingSeconds is the time Railway waits between SIGTERM and
		// SIGKILL; Railway's default is 0.
		DrainingSeconds float64 `json:"drainingSeconds"`
	} `json:"deploy"`
}

func TestRailwayConfigMatchesHealthRouteAndSingleReplica(t *testing.T) {
	cfg := readRailwayConfig(t)

	if cfg.Deploy.HealthcheckPath != healthPath {
		t.Errorf("deploy.healthcheckPath = %q, want %q", cfg.Deploy.HealthcheckPath, healthPath)
	}
	if cfg.Deploy.NumReplicas != expectedReplicas {
		t.Errorf("deploy.numReplicas = %d, want %d", cfg.Deploy.NumReplicas, expectedReplicas)
	}
}

// TestRailwayWaitsForTheShutdownBeforeKillingTheOldDeploy keeps Railway
// from killing the previous deploy before shutdownTimeout has run out, so
// a request that waits until its deadline still gets its answer.
func TestRailwayWaitsForTheShutdownBeforeKillingTheOldDeploy(t *testing.T) {
	cfg := readRailwayConfig(t)

	draining := time.Duration(cfg.Deploy.DrainingSeconds * float64(time.Second))
	if draining <= shutdownTimeout {
		t.Errorf("deploy.drainingSeconds = %v, must lie above shutdownTimeout %v", draining, shutdownTimeout)
	}
}

// readRailwayConfig decodes the Railway deploy configuration.
func readRailwayConfig(t *testing.T) railwayConfig {
	t.Helper()
	raw, err := os.ReadFile(railwayConfigPath)
	if err != nil {
		t.Fatalf("read %s: %v", railwayConfigPath, err)
	}
	var cfg railwayConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("decode %s: %v", railwayConfigPath, err)
	}
	return cfg
}
