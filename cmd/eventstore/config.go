package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// Names of the environment variables the service reads at startup.
const (
	envDatabaseURL       = "DATABASE_URL"
	envPort              = "PORT"
	envAdminUser         = "ADMIN_USER"
	envAdminPasswordHash = "ADMIN_PASSWORD_HASH"
	envSessionSecret     = "SESSION_SECRET"
)

const (
	minPort = 1
	maxPort = 65535
	// minSessionSecretBytes is the HMAC-SHA256 key length the session cookie
	// needs (AD-12).
	minSessionSecretBytes = 32
	// minBcryptCost is the lowest accepted work factor of the admin
	// password hash (AD-12).
	minBcryptCost = 12
	// bcryptHashLength is the fixed length of every bcrypt hash; bcrypt.Cost
	// only parses the prefix, so a truncated or extended hash would pass it
	// and then never match any password.
	bcryptHashLength = 60
)

// requiredVariables lists every variable without a default, in the order a
// missing-variables error names them.
var requiredVariables = []string{
	envDatabaseURL, envPort, envAdminUser, envAdminPasswordHash, envSessionSecret,
}

var (
	// errInvalidPort marks a PORT value that is not an integer in the valid
	// TCP port range.
	errInvalidPort = errors.New("invalid port")
	// errInvalidPasswordHash marks an ADMIN_PASSWORD_HASH that is no bcrypt
	// hash.
	errInvalidPasswordHash = errors.New("invalid admin password hash")
	// errPasswordHashCostTooLow marks a bcrypt hash below minBcryptCost.
	errPasswordHashCostTooLow = errors.New("admin password hash cost too low")
	// errSessionSecretTooShort marks a SESSION_SECRET below
	// minSessionSecretBytes.
	errSessionSecretTooShort = errors.New("session secret too short")
)

// missingVariablesError lists every required environment variable that is
// unset or blank, so one failed start names all of them at once.
type missingVariablesError struct {
	Names []string
}

func (e *missingVariablesError) Error() string {
	return "missing required environment variables: " + strings.Join(e.Names, ", ")
}

// config holds the validated startup configuration.
type config struct {
	DatabaseURL       string
	Port              int
	AdminUser         string
	AdminPasswordHash []byte
	SessionSecret     []byte
}

// ListenAddress returns the address the HTTP server binds to.
func (c config) ListenAddress() string {
	return ":" + strconv.Itoa(c.Port)
}

// loadConfig reads and validates the configuration through getenv. Error
// messages never contain variable values other than PORT, because
// DATABASE_URL, ADMIN_PASSWORD_HASH and SESSION_SECRET are secrets.
func loadConfig(getenv func(string) string) (config, error) {
	values, err := readRequiredVariables(getenv)
	if err != nil {
		return config{}, err
	}
	port, err := parsePort(values[envPort])
	if err != nil {
		return config{}, err
	}
	passwordHash, err := parsePasswordHash(values[envAdminPasswordHash])
	if err != nil {
		return config{}, err
	}
	sessionSecret, err := parseSessionSecret(values[envSessionSecret])
	if err != nil {
		return config{}, err
	}
	return config{
		DatabaseURL:       values[envDatabaseURL],
		Port:              port,
		AdminUser:         values[envAdminUser],
		AdminPasswordHash: passwordHash,
		SessionSecret:     sessionSecret,
	}, nil
}

// readRequiredVariables returns the trimmed value of every required
// variable, or a *missingVariablesError naming all blank ones.
func readRequiredVariables(getenv func(string) string) (map[string]string, error) {
	values := make(map[string]string, len(requiredVariables))
	var missing []string
	for _, name := range requiredVariables {
		value := strings.TrimSpace(getenv(name))
		if value == "" {
			missing = append(missing, name)
		}
		values[name] = value
	}
	if len(missing) > 0 {
		return nil, &missingVariablesError{Names: missing}
	}
	return values, nil
}

// parsePort converts raw into a TCP port number between minPort and maxPort.
func parsePort(raw string) (int, error) {
	port, err := strconv.Atoi(raw)
	if err != nil || port < minPort || port > maxPort {
		return 0, fmt.Errorf("%w: %s=%q must be an integer between %d and %d",
			errInvalidPort, envPort, raw, minPort, maxPort)
	}
	return port, nil
}

// parsePasswordHash accepts only a bcrypt hash with at least minBcryptCost.
func parsePasswordHash(raw string) ([]byte, error) {
	hash := []byte(raw)
	cost, err := bcrypt.Cost(hash)
	if err != nil || len(hash) != bcryptHashLength {
		return nil, fmt.Errorf("%w: %s must be a bcrypt hash", errInvalidPasswordHash, envAdminPasswordHash)
	}
	if cost < minBcryptCost {
		return nil, fmt.Errorf("%w: %s has bcrypt cost %d, need at least %d",
			errPasswordHashCostTooLow, envAdminPasswordHash, cost, minBcryptCost)
	}
	return hash, nil
}

// parseSessionSecret accepts a key of at least minSessionSecretBytes.
func parseSessionSecret(raw string) ([]byte, error) {
	if len(raw) < minSessionSecretBytes {
		return nil, fmt.Errorf("%w: %s must be at least %d bytes",
			errSessionSecretTooShort, envSessionSecret, minSessionSecretBytes)
	}
	return []byte(raw), nil
}
