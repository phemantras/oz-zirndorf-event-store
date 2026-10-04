package v1

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"testing"

	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// generatedFile is the scaffold oapi-codegen derives from api/v1/openapi.yaml;
// the CI check "generated code is up to date" binds it to the spec.
const generatedFile = "api.gen.go"

// Names of the generated enum types whose codes the core mirrors (AD-9).
const (
	eventTypeName         = "EventType"
	timePrecisionName     = "TimePrecision"
	locationPrecisionName = "LocationPrecision"
)

// enumCodesInFile returns, per type name, the sorted string values of all
// constants declared with that type in a Go file.
func enumCodesInFile(t *testing.T, path string) map[string][]string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	codes := map[string][]string{}
	for _, decl := range file.Decls {
		gen, isGen := decl.(*ast.GenDecl)
		if !isGen || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			value := spec.(*ast.ValueSpec)
			typeName, isIdent := value.Type.(*ast.Ident)
			if !isIdent {
				continue
			}
			for _, expr := range value.Values {
				literal, isLiteral := expr.(*ast.BasicLit)
				if !isLiteral || literal.Kind != token.STRING {
					continue
				}
				code, err := strconv.Unquote(literal.Value)
				if err != nil {
					t.Fatalf("unquote %s: %v", literal.Value, err)
				}
				codes[typeName.Name] = append(codes[typeName.Name], code)
			}
		}
	}
	for name := range codes {
		slices.Sort(codes[name])
	}
	return codes
}

func sortedCodes[T ~string](values []T) []string {
	var codes []string
	for _, value := range values {
		codes = append(codes, string(value))
	}
	slices.Sort(codes)
	return codes
}

func TestSpecEnumCodesMatchTheCore(t *testing.T) {
	spec := enumCodesInFile(t, generatedFile)
	want := map[string][]string{
		eventTypeName:         sortedCodes(core.EventTypes()),
		timePrecisionName:     sortedCodes(core.TimePrecisions()),
		locationPrecisionName: sortedCodes(core.LocationPrecisions()),
	}
	for name, coreCodes := range want {
		if len(spec[name]) == 0 {
			t.Errorf("no constants of type %s found in %s", name, generatedFile)
		}
		if !slices.Equal(spec[name], coreCodes) {
			t.Errorf("%s codes: spec %v, core %v", name, spec[name], coreCodes)
		}
	}
}

// The comparison is only meaningful if the parser finds typed constants and
// ignores untyped ones; a fixture proves it.
func TestEnumCodesInFileFindsTypedStringConstants(t *testing.T) {
	got := enumCodesInFile(t, "testdata/enums.go.txt")
	want := map[string][]string{
		"Color": {"blue", "red"},
		"Size":  {"large"},
	}
	if len(got) != len(want) {
		t.Errorf("types = %v, want %v", got, want)
	}
	for name, codes := range want {
		if !slices.Equal(got[name], codes) {
			t.Errorf("%s = %v, want %v", name, got[name], codes)
		}
	}
}
