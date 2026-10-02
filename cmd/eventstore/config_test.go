package main

import (
	"errors"
	"maps"
	"strings"
	"testing"
)

const (
	validDatabaseURL = "postgres://eventstore:secret@localhost:5432/eventstore"
	validAdminUser   = "andreas"
	// validPasswordHash is bcrypt cost 12 of "richtig-und-lang".
	validPasswordHash = "$2a$12$A0tt6BWgL3N.gkWWYcnnbOFw7NcKuzLN.Sek46zm31Vk1ixYhqj/2"
	// lowCostPasswordHash is bcrypt cost 10 of the same password.
	lowCostPasswordHash = "$2a$10$CYSNUbZYy6Jw3oRztVwp7OZbcBkV02vy2nJWQ8mjgYe/UhYbeuaOe"
	// validSessionSecret is exactly minSessionSecretBytes long.
	validSessionSecret = "0123456789abcdef0123456789abcdef"
)

func envFrom(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}

// validEnv returns a complete, valid environment with overrides applied.
func validEnv(overrides map[string]string) func(string) string {
	values := map[string]string{
		envDatabaseURL:       validDatabaseURL,
		envPort:              "8080",
		envAdminUser:         validAdminUser,
		envAdminPasswordHash: validPasswordHash,
		envSessionSecret:     validSessionSecret,
	}
	maps.Copy(values, overrides)
	return envFrom(values)
}

func TestLoadConfigReadsRequiredVariables(t *testing.T) {
	cfg, err := loadConfig(validEnv(nil))
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
	if cfg.AdminUser != validAdminUser {
		t.Errorf("AdminUser = %q, want %q", cfg.AdminUser, validAdminUser)
	}
	if string(cfg.AdminPasswordHash) != validPasswordHash {
		t.Error("AdminPasswordHash does not match the variable")
	}
	if string(cfg.SessionSecret) != validSessionSecret {
		t.Error("SessionSecret does not match the variable")
	}
}

func TestLoadConfigTrimsSurroundingWhitespace(t *testing.T) {
	cfg, err := loadConfig(validEnv(map[string]string{
		envDatabaseURL:       " " + validDatabaseURL + "\n",
		envPort:              " 8080 ",
		envAdminUser:         " " + validAdminUser + " ",
		envAdminPasswordHash: validPasswordHash + "\n",
		envSessionSecret:     " " + validSessionSecret + "\n",
	}))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.DatabaseURL != validDatabaseURL || cfg.Port != 8080 || cfg.AdminUser != validAdminUser ||
		string(cfg.AdminPasswordHash) != validPasswordHash || string(cfg.SessionSecret) != validSessionSecret {
		t.Error("config values are not trimmed")
	}
}

func TestLoadConfigReportsAllMissingVariables(t *testing.T) {
	tests := []struct {
		name        string
		env         func(string) string
		wantMissing []string
	}{
		{name: "all unset", env: envFrom(map[string]string{}), wantMissing: []string{envDatabaseURL, envPort, envAdminUser, envAdminPasswordHash, envSessionSecret}},
		{name: "database url blank", env: validEnv(map[string]string{envDatabaseURL: "   "}), wantMissing: []string{envDatabaseURL}},
		{name: "port empty", env: validEnv(map[string]string{envPort: ""}), wantMissing: []string{envPort}},
		{name: "admin user blank", env: validEnv(map[string]string{envAdminUser: " "}), wantMissing: []string{envAdminUser}},
		{name: "password hash empty", env: validEnv(map[string]string{envAdminPasswordHash: ""}), wantMissing: []string{envAdminPasswordHash}},
		{name: "session secret empty", env: validEnv(map[string]string{envSessionSecret: ""}), wantMissing: []string{envSessionSecret}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadConfig(tt.env)
			var missing *missingVariablesError
			if !errors.As(err, &missing) {
				t.Fatalf("err = %v, want *missingVariablesError", err)
			}
			if strings.Join(missing.Names, ",") != strings.Join(tt.wantMissing, ",") {
				t.Errorf("missing = %v, want %v", missing.Names, tt.wantMissing)
			}
			if !strings.Contains(err.Error(), "missing required environment variables") {
				t.Errorf("error %q lacks the message the CI start check greps for", err)
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
			_, err := loadConfig(validEnv(map[string]string{envPort: port}))
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
			if _, err := loadConfig(validEnv(map[string]string{envPort: port})); err != nil {
				t.Fatalf("loadConfig: %v", err)
			}
		})
	}
}

func TestLoadConfigRejectsWeakAdminCredentialsWithoutLeakingThem(t *testing.T) {
	tests := []struct {
		name     string
		variable string
		value    string
		wantErr  error
	}{
		{name: "secret one byte short", variable: envSessionSecret, value: validSessionSecret[1:], wantErr: errSessionSecretTooShort},
		{name: "hash cost 10", variable: envAdminPasswordHash, value: lowCostPasswordHash, wantErr: errPasswordHashCostTooLow},
		{name: "hash not bcrypt", variable: envAdminPasswordHash, value: "plain-text-password-not-a-hash-at-all-0123456789abcdef", wantErr: errInvalidPasswordHash},
		{name: "hash with bad cost", variable: envAdminPasswordHash, value: "$2a$xx$A0tt6BWgL3N.gkWWYcnnbOFw7NcKuzLN.Sek46zm31Vk1ixYhqj/2", wantErr: errInvalidPasswordHash},
		{name: "hash truncated", variable: envAdminPasswordHash, value: validPasswordHash[:len(validPasswordHash)-1], wantErr: errInvalidPasswordHash},
		{name: "hash extended", variable: envAdminPasswordHash, value: validPasswordHash + "x", wantErr: errInvalidPasswordHash},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadConfig(validEnv(map[string]string{tt.variable: tt.value}))
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.variable) {
				t.Errorf("error %q does not name %s", err, tt.variable)
			}
			if strings.Contains(err.Error(), tt.value) {
				t.Errorf("error %q leaks the value", err)
			}
		})
	}
}

// htpasswd, as documented in the README, writes the $2y$ prefix.
func TestLoadConfigAcceptsHtpasswdHashPrefix(t *testing.T) {
	htpasswdHash := strings.Replace(validPasswordHash, "$2a$", "$2y$", 1)
	if _, err := loadConfig(validEnv(map[string]string{envAdminPasswordHash: htpasswdHash})); err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
}

func TestConfigErrorsNeverContainTheDatabaseURL(t *testing.T) {
	_, err := loadConfig(validEnv(map[string]string{envPort: "abc"}))
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), "secret") {
		t.Errorf("error %q leaks the database password", err)
	}
}
