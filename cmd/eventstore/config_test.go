package main

import (
	"errors"
	"strings"
	"testing"
)

const validDatabaseURL = "postgres://eventstore:secret@localhost:5432/eventstore"

func envFrom(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}

func TestLoadConfigReadsRequiredVariables(t *testing.T) {
	cfg, err := loadConfig(envFrom(map[string]string{
		envDatabaseURL: validDatabaseURL,
		envPort:        "8080",
	}))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.DatabaseURL != validDatabaseURL {
		t.Errorf("DatabaseURL = %q, want %q", cfg.DatabaseURL, validDatabaseURL)
	}
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want 8080", cfg.Port)
	}
	if got := cfg.ListenAddress(); got != ":8080" {
		t.Errorf("ListenAddress() = %q, want %q", got, ":8080")
	}
}

func TestLoadConfigTrimsSurroundingWhitespace(t *testing.T) {
	cfg, err := loadConfig(envFrom(map[string]string{
		envDatabaseURL: " " + validDatabaseURL + "\n",
		envPort:        " 8080 ",
	}))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.DatabaseURL != validDatabaseURL || cfg.Port != 8080 {
		t.Errorf("cfg = %+v, want trimmed values", cfg)
	}
}

func TestLoadConfigReportsAllMissingVariables(t *testing.T) {
	tests := []struct {
		name        string
		env         map[string]string
		wantMissing []string
	}{
		{name: "all unset", env: map[string]string{}, wantMissing: []string{envDatabaseURL, envPort}},
		{name: "database url blank", env: map[string]string{envDatabaseURL: "   ", envPort: "8080"}, wantMissing: []string{envDatabaseURL}},
		{name: "port empty", env: map[string]string{envDatabaseURL: validDatabaseURL, envPort: ""}, wantMissing: []string{envPort}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadConfig(envFrom(tt.env))
			var missing *missingVariablesError
			if !errors.As(err, &missing) {
				t.Fatalf("err = %v, want *missingVariablesError", err)
			}
			if strings.Join(missing.Names, ",") != strings.Join(tt.wantMissing, ",") {
				t.Errorf("missing = %v, want %v", missing.Names, tt.wantMissing)
			}
			for _, name := range tt.wantMissing {
				if !strings.Contains(err.Error(), name) {
					t.Errorf("error %q does not name %s", err, name)
				}
			}
		})
	}
}

func TestLoadConfigRejectsInvalidPort(t *testing.T) {
	for _, port := range []string{"abc", "0", "65536", "-1", "80a"} {
		t.Run(port, func(t *testing.T) {
			_, err := loadConfig(envFrom(map[string]string{
				envDatabaseURL: validDatabaseURL,
				envPort:        port,
			}))
			if !errors.Is(err, errInvalidPort) {
				t.Fatalf("err = %v, want errInvalidPort", err)
			}
			if !strings.Contains(err.Error(), envPort) {
				t.Errorf("error %q does not name %s", err, envPort)
			}
		})
	}
}

func TestLoadConfigAcceptsPortRangeBounds(t *testing.T) {
	for _, port := range []string{"1", "65535"} {
		t.Run(port, func(t *testing.T) {
			if _, err := loadConfig(envFrom(map[string]string{
				envDatabaseURL: validDatabaseURL,
				envPort:        port,
			})); err != nil {
				t.Fatalf("loadConfig: %v", err)
			}
		})
	}
}

func TestConfigErrorsNeverContainTheDatabaseURL(t *testing.T) {
	_, err := loadConfig(envFrom(map[string]string{
		envDatabaseURL: validDatabaseURL,
		envPort:        "abc",
	}))
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), "secret") {
		t.Errorf("error %q leaks the database password", err)
	}
}
