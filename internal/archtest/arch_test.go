// Package archtest enforces the dependency direction of AD-1: the core
// imports only the standard library plus golang.org/x/text/unicode/norm,
// and adapters never import another adapter.
package archtest

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const (
	modulePath = "github.com/phemantras/oz-zirndorf-event-store"
	repoRoot   = "../.."
	// violationsFixture is a miniature repository whose files break AD-1 on
	// purpose, proving that the checker detects violations.
	violationsFixture = "testdata/violations"

	corePrefix    = "internal/core"
	adapterPrefix = "internal/adapter/"
	// cgoPseudoPackage has no dot but is not standard library: it links C code.
	cgoPseudoPackage = "C"
)

// allowedCoreImports lists the only non-standard-library imports the core
// may use (AD-1, AD-11).
var allowedCoreImports = []string{"golang.org/x/text/unicode/norm"}

type violation struct {
	file       string
	importPath string
}

func TestRepositoryFollowsDependencyDirection(t *testing.T) {
	// Guards against a wrong repoRoot, which would make the scan pass vacuously.
	if _, err := os.Stat(filepath.Join(repoRoot, "go.mod")); err != nil {
		t.Fatalf("repository root not found: %v", err)
	}
	violations, err := findViolations(repoRoot)
	if err != nil {
		t.Fatalf("scan repository: %v", err)
	}
	for _, v := range violations {
		t.Errorf("%s imports %s, which AD-1 forbids", v.file, v.importPath)
	}
}

func TestCheckerDetectsViolationsInFixture(t *testing.T) {
	want := []violation{
		{file: "internal/adapter/admin/bad.go", importPath: modulePath + "/internal/adapter/postgres"},
		{file: "internal/adapter/publicapi/v1/bad.go", importPath: modulePath + "/internal/adapter/admin"},
		{file: "internal/core/bad.go", importPath: "github.com/jackc/pgx/v5"},
		{file: "internal/core/cgo.go", importPath: "C"},
		{file: "internal/core/sub/bad.go", importPath: "golang.org/x/text/cases"},
	}

	got, err := findViolations(violationsFixture)
	if err != nil {
		t.Fatalf("scan fixture: %v", err)
	}
	if !slices.Equal(got, want) {
		t.Errorf("violations =\n%v\nwant\n%v", got, want)
	}
}

func TestFindViolationsFailsOnUnparsableFile(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "internal", "core")
	if err := mkdirAndWrite(dir, "broken.go", "package core\nimport ("); err != nil {
		t.Fatal(err)
	}
	if _, err := findViolations(root); err == nil {
		t.Fatal("findViolations returned no error for an unparsable file")
	}
}

// findViolations scans every Go file below root, skipping testdata and
// hidden or underscore directories like the go tool does, and returns all
// imports that break AD-1, sorted by file.
func findViolations(root string) ([]violation, error) {
	var violations []violation
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(filePath string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if filePath != root && isIgnoredDir(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".go") {
			return nil
		}
		parsed, err := parser.ParseFile(fset, filePath, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, filePath)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		for _, spec := range parsed.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			if !isImportAllowed(path.Dir(rel), importPath) {
				violations = append(violations, violation{file: rel, importPath: importPath})
			}
		}
		return nil
	})
	return violations, err
}

func isIgnoredDir(name string) bool {
	return name == "testdata" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

// isImportAllowed reports whether a package in dir (relative to the module
// root) may import importPath under AD-1.
func isImportAllowed(dir, importPath string) bool {
	if isWithin(dir, corePrefix) {
		return isStandardLibrary(importPath) ||
			slices.Contains(allowedCoreImports, importPath) ||
			isWithin(importPath, modulePath+"/"+corePrefix)
	}
	ownAdapter, inAdapter := adapterName(dir)
	if !inAdapter {
		return true
	}
	importedRel, isModuleImport := strings.CutPrefix(importPath, modulePath+"/")
	if !isModuleImport {
		return true
	}
	importedAdapter, importsAdapter := adapterName(importedRel)
	return !importsAdapter || importedAdapter == ownAdapter
}

func mkdirAndWrite(dir, name, content string) error {
	const dirPerm, filePerm = 0o755, 0o644
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name), []byte(content), filePerm)
}

// adapterName returns the adapter a module-relative path belongs to, for
// example "publicapi" for "internal/adapter/publicapi/v1".
func adapterName(relPath string) (string, bool) {
	rest, found := strings.CutPrefix(relPath, adapterPrefix)
	if !found || rest == "" {
		return "", false
	}
	name, _, _ := strings.Cut(rest, "/")
	return name, true
}

// isStandardLibrary uses the go tool's convention: standard library import
// paths have no dot in their first element.
func isStandardLibrary(importPath string) bool {
	if importPath == cgoPseudoPackage {
		return false
	}
	first, _, _ := strings.Cut(importPath, "/")
	return !strings.Contains(first, ".")
}

func isWithin(p, prefix string) bool {
	return p == prefix || strings.HasPrefix(p, prefix+"/")
}
