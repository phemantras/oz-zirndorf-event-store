// Package scripts holds the tests of the project scripts.
package scripts

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

const (
	coverageScript  = "check-coverage.sh"
	coverageFixture = "testdata/coveragefixture"
)

// coveredSummary matches the gate's summary line "covered of total".
var coveredSummary = regexp.MustCompile(`coverage gate: (\d+) of (\d+) statements covered`)

// runCoverageGate runs the coverage gate in the fixture module on the
// package pattern and returns its combined output and whether it passed.
func runCoverageGate(t *testing.T, pattern string) (string, bool) {
	t.Helper()
	bash := findBash(t)
	script, err := filepath.Abs(coverageScript)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), bash, filepath.ToSlash(script), pattern)
	cmd.Dir = coverageFixture
	cmd.Env = append(os.Environ(), "GOWORK=off")
	output, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		t.Fatalf("run coverage gate: %v", err)
	}
	return string(output), err == nil
}

// findBash returns the bash to run the gate with. On Windows, bash on the
// path is often the WSL launcher, so Git Bash next to git comes first.
func findBash(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		if git, err := exec.LookPath("git"); err == nil {
			gitBash := filepath.Join(filepath.Dir(git), "..", "bin", "bash.exe")
			if _, err := os.Stat(gitBash); err == nil {
				return gitBash
			}
		}
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatalf("the coverage gate needs bash: %v", err)
	}
	return bash
}

func TestCoverageGateLeavesOutGeneratedFiles(t *testing.T) {
	output, passed := runCoverageGate(t, "./generated/...")

	if !passed {
		t.Fatalf("gate failed, want it to pass:\n%s", output)
	}
	match := coveredSummary.FindStringSubmatch(output)
	if match == nil {
		t.Fatalf("output lacks the summary:\n%s", output)
	}
	covered, total := mustAtoi(t, match[1]), mustAtoi(t, match[2])
	if total == 0 || covered != total {
		t.Errorf("summary %q, want N of N with N > 0", match[0])
	}
	if strings.Contains(output, "gen.go") {
		t.Errorf("output names the generated file:\n%s", output)
	}
}

func TestCoverageGateCountsAFileWhoseHeaderFollowsThePackageClause(t *testing.T) {
	output, passed := runCoverageGate(t, "./lateheader/...")

	if passed {
		t.Errorf("gate passed, want it to fail:\n%s", output)
	}
	if !strings.Contains(output, "uncovered: coveragefixture/lateheader/late.go") {
		t.Errorf("output does not name the uncovered file:\n%s", output)
	}
}

func TestCoverageGateFailsOnAnUntestedFunction(t *testing.T) {
	output, passed := runCoverageGate(t, "./gap/...")

	if passed {
		t.Errorf("gate passed, want it to fail:\n%s", output)
	}
	if !strings.Contains(output, "coverage gate: 100 % required") {
		t.Errorf("output does not demand full coverage:\n%s", output)
	}
}

func mustAtoi(t *testing.T, digits string) int {
	t.Helper()
	n, err := strconv.Atoi(digits)
	if err != nil {
		t.Fatal(err)
	}
	return n
}
