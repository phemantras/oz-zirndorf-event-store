package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Names of the environment variables the service reads at startup.
const (
	envDatabaseURL = "DATABASE_URL"
	envPort        = "PORT"
)

const (
	minPort = 1
	maxPort = 65535
)

// errInvalidPort marks a PORT value that is not an integer in the valid TCP
// port range.
var errInvalidPort = errors.New("invalid port")

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
	DatabaseURL string
	Port        int
}

// ListenAddress returns the address the HTTP server binds to.
func (c config) ListenAddress() string {
	return ":" + strconv.Itoa(c.Port)
}

// loadConfig reads and validates the configuration through getenv. Error
// messages never contain variable values other than PORT, because
// DATABASE_URL carries the database password.
func loadConfig(getenv func(string) string) (config, error) {
	databaseURL := strings.TrimSpace(getenv(envDatabaseURL))
	rawPort := strings.TrimSpace(getenv(envPort))

	var missing []string
	if databaseURL == "" {
		missing = append(missing, envDatabaseURL)
	}
	if rawPort == "" {
		missing = append(missing, envPort)
	}
	if len(missing) > 0 {
		return config{}, &missingVariablesError{Names: missing}
	}

	port, err := parsePort(rawPort)
	if err != nil {
		return config{}, err
	}
	return config{DatabaseURL: databaseURL, Port: port}, nil
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
