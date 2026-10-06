package v1

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	apispec "github.com/phemantras/oz-zirndorf-event-store/api/v1"
	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// coreSourceGlob finds the source files of the core, whose field name
// constants mirror the spec (AD-9).
const coreSourceGlob = "../../../core/*.go"

// Prefixes of the core constants that name fields. Struct fields are
// compared with the schemas, filter fields with the query parameters.
const (
	eventFieldPrefix     = "EventField"
	locationFieldPrefix  = "LocationField"
	timetableFieldPrefix = "TimetableField"
	filterFieldPrefix    = "FilterField"
)

// adminOnlyFields are core field names the spec deliberately lacks: the
// admin refers to a stored location by its ID, which the API never gives
// out (AD-14).
var adminOnlyFields = []string{"EventFieldLocationID"}

// fieldSchemas names, per prefix of the core constants, the generated
// write-form type whose JSON fields, nested ones as dotted paths, must
// carry each constant of that prefix.
var fieldSchemas = map[string]string{
	eventFieldPrefix:     "EventInput",
	locationFieldPrefix:  "EventInputLocation",
	timetableFieldPrefix: "TimetableEntry",
}

// locationAddressPrefix is where the location schema nests the address
// parts, whose core constants name them without it.
const locationAddressPrefix = "address."

// filterParams are the generated types of the query parameters.
var filterParams = []string{"ListEventsParams", "ListArchivedEventsParams"}

// Struct tag keys of the generated types.
const (
	jsonTagKey = "json"
	formTagKey = "form"
	// tagOptionSeparator ends the name in a tag such as "note,omitempty".
	tagOptionSeparator = ","
)

// parseGoFile parses one Go source file for a contract test.
func parseGoFile(t *testing.T, path string) *ast.File {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return file
}

// stringConstants returns the name and value of every constant in files
// that is initialized with a string literal and whose name starts with one
// of prefixes.
func stringConstants(t *testing.T, files []*ast.File, prefixes ...string) map[string]string {
	t.Helper()
	constants := map[string]string{}
	for _, file := range files {
		for _, decl := range file.Decls {
			gen, isGen := decl.(*ast.GenDecl)
			if !isGen || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				value := spec.(*ast.ValueSpec)
				for i, name := range value.Names {
					if i >= len(value.Values) || !hasAnyPrefix(name.Name, prefixes) {
						continue
					}
					literal, isLiteral := value.Values[i].(*ast.BasicLit)
					if !isLiteral || literal.Kind != token.STRING {
						continue
					}
					text, err := strconv.Unquote(literal.Value)
					if err != nil {
						t.Fatalf("unquote %s: %v", literal.Value, err)
					}
					constants[name.Name] = text
				}
			}
		}
	}
	return constants
}

func hasAnyPrefix(name string, prefixes []string) bool {
	return slices.ContainsFunc(prefixes, func(prefix string) bool { return strings.HasPrefix(name, prefix) })
}

// structField is one field of a generated struct: its name in a tag and
// the name of the type it refers to, behind pointers and slices.
type structField struct {
	name     string
	typeName string
}

// structFields returns, per struct type of file, its fields named by the
// struct tag key. Fields without that tag are left out.
func structFields(file *ast.File, tagKey string) map[string][]structField {
	structs := map[string][]structField{}
	ast.Inspect(file, func(node ast.Node) bool {
		spec, isSpec := node.(*ast.TypeSpec)
		if !isSpec {
			return true
		}
		fields, isStruct := spec.Type.(*ast.StructType)
		if !isStruct {
			return false
		}
		structs[spec.Name.Name] = []structField{}
		for _, field := range fields.Fields.List {
			name := tagName(field.Tag, tagKey)
			if name == "" {
				continue
			}
			structs[spec.Name.Name] = append(structs[spec.Name.Name], structField{name: name, typeName: referredTypeName(field.Type)})
		}
		return false
	})
	return structs
}

// tagName returns the name a struct tag gives for key, without options.
func tagName(tag *ast.BasicLit, key string) string {
	if tag == nil {
		return ""
	}
	text, err := strconv.Unquote(tag.Value)
	if err != nil {
		return ""
	}
	name, _, _ := strings.Cut(reflect.StructTag(text).Get(key), tagOptionSeparator)
	return name
}

// referredTypeName returns the name of the type expr refers to behind
// pointers and slices, or "" for any other type expression.
func referredTypeName(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.StarExpr:
		return referredTypeName(typed.X)
	case *ast.ArrayType:
		return referredTypeName(typed.Elt)
	default:
		return ""
	}
}

// fieldPaths returns every field of the struct root as a dotted path, the
// fields of nested structs included, such as source.description.
func fieldPaths(structs map[string][]structField, root string) []string {
	var paths []string
	for _, field := range structs[root] {
		paths = append(paths, field.name)
		for _, nested := range fieldPaths(structs, field.typeName) {
			paths = append(paths, field.name+"."+nested)
		}
	}
	return paths
}

// namesOf returns the paths of every root.
func namesOf(structs map[string][]structField, roots []string) map[string]bool {
	names := map[string]bool{}
	for _, root := range roots {
		for _, path := range fieldPaths(structs, root) {
			names[path] = true
		}
	}
	return names
}

func coreSourceFiles(t *testing.T) []*ast.File {
	t.Helper()
	paths, err := filepath.Glob(coreSourceGlob)
	if err != nil {
		t.Fatalf("glob core sources: %v", err)
	}
	var files []*ast.File
	for _, path := range paths {
		if !strings.HasSuffix(path, "_test.go") {
			files = append(files, parseGoFile(t, path))
		}
	}
	return files
}

func TestCoreFieldNamesAreFieldsOfTheSpec(t *testing.T) {
	generated := parseGoFile(t, generatedFile)
	jsonStructs := structFields(generated, jsonTagKey)
	known := map[string]map[string]bool{filterFieldPrefix: namesOf(structFields(generated, formTagKey), filterParams)}
	for prefix, schema := range fieldSchemas {
		known[prefix] = namesOf(jsonStructs, []string{schema})
	}
	prefixes := []string{eventFieldPrefix, locationFieldPrefix, timetableFieldPrefix, filterFieldPrefix}
	constants := stringConstants(t, coreSourceFiles(t), prefixes...)

	for _, prefix := range prefixes {
		if !slices.ContainsFunc(slices.Collect(maps.Keys(constants)), func(name string) bool { return strings.HasPrefix(name, prefix) }) {
			t.Errorf("no core constant %s* found", prefix)
		}
	}
	for name, field := range constants {
		prefix := prefixes[slices.IndexFunc(prefixes, func(prefix string) bool { return strings.HasPrefix(name, prefix) })]
		fields := known[prefix]
		if slices.Contains(adminOnlyFields, name) {
			if fields[field] {
				t.Errorf("%s = %q is listed as admin-only but is a field of the spec", name, field)
			}
			continue
		}
		isAddressPart := prefix == locationFieldPrefix && fields[locationAddressPrefix+field]
		if !fields[field] && !isAddressPart {
			t.Errorf("core constant %s = %q is no field of %s", name, field, schemaOf(prefix))
		}
	}
}

// schemaOf names the spec types a prefix of core constants is checked
// against.
func schemaOf(prefix string) string {
	if schema, ok := fieldSchemas[prefix]; ok {
		return schema
	}
	return strings.Join(filterParams, ", ")
}

func TestEventInputHasNoIdentifiers(t *testing.T) {
	structs := structFields(parseGoFile(t, generatedFile), jsonTagKey)
	for _, path := range fieldPaths(structs, "EventInput") {
		last := path[strings.LastIndex(path, ".")+1:]
		if last == "id" || last == core.EventFieldLocationID {
			t.Errorf("EventInput has the identifier %s", path)
		}
	}
}

// The field comparison is only meaningful if the walkers find nested
// fields and the constants; a fixture proves it.
func TestFieldWalkersFindNestedFieldsAndConstants(t *testing.T) {
	fixture := parseGoFile(t, "testdata/fields.go.txt")

	got := fieldPaths(structFields(fixture, jsonTagKey), "Outer")
	want := []string{"name", "inner", "inner.street", "list", "list.street", "plain"}
	if !slices.Equal(got, want) {
		t.Errorf("paths = %v, want %v", got, want)
	}
	params := namesOf(structFields(fixture, formTagKey), []string{"Params"})
	if !reflect.DeepEqual(params, map[string]bool{"from": true}) {
		t.Errorf("params = %v, want only from", params)
	}
	constants := stringConstants(t, []*ast.File{fixture}, "Field")
	if !reflect.DeepEqual(constants, map[string]string{"FieldName": "name", "FieldStreet": "street"}) {
		t.Errorf("constants = %v", constants)
	}
}

// specLimits names, per path in openapi.yaml, the core constant that
// mirrors the limit (ENT-24).
var specLimits = map[string]int{
	"components.schemas.EventInput.properties.title.maxLength":           core.MaxTitleLength,
	"components.schemas.EventInput.properties.note.maxLength":            core.MaxNoteLength,
	"components.schemas.EventInput.properties.timetable.maxItems":        core.MaxTimetableEntries,
	"components.schemas.EventInput.properties.importKey.maxLength":       core.MaxImportKeyLength,
	"components.schemas.Source.properties.description.maxLength":         core.MaxSourceDescriptionLength,
	"components.schemas.Source.properties.url.maxLength":                 core.MaxSourceURLLength,
	"components.schemas.TimetableEntry.properties.description.maxLength": core.MaxTimetableDescriptionLength,
	"components.schemas.EventInputLocation.properties.name.maxLength":    core.MaxLocationNameLength,
	"components.schemas.EventInputLocation.properties.note.maxLength":    core.MaxLocationNoteLength,
	"components.schemas.Address.properties.street.maxLength":             core.MaxStreetLength,
	"components.schemas.Address.properties.city.maxLength":               core.MaxCityLength,
}

// Keys of a limit in the spec.
const (
	maxLengthKey = "maxLength"
	maxItemsKey  = "maxItems"
)

func TestSpecLimitsMatchTheCore(t *testing.T) {
	scalars := yamlScalars(string(apispec.OpenAPISpec))

	for path, want := range specLimits {
		if got := scalars[path]; got != strconv.Itoa(want) {
			t.Errorf("%s = %q, core %d", path, got, want)
		}
	}
	for path := range scalars {
		last := path[strings.LastIndex(path, ".")+1:]
		if (last == maxLengthKey || last == maxItemsKey) && !slices.Contains(slices.Collect(maps.Keys(specLimits)), path) {
			t.Errorf("limit %s has no core constant in this test", path)
		}
	}
}

func TestYAMLScalarsReadsIndentedPathsOnly(t *testing.T) {
	text, err := os.ReadFile("testdata/limits.yaml.txt")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	got := yamlScalars(string(text))

	want := map[string]string{
		"title":              "Fixture",
		"schemas.Thing.type": "object",
		"schemas.Thing.properties.name.maxLength":      "200",
		"schemas.Thing.properties.list.maxItems":       "100",
		"schemas.Thing.properties.list.example.-.date": "2026-11-27",
		"schemas.Thing.properties.quoted.pattern":      "^[0-9]{5}$",
		"schemas.Other.maxLength":                      "7",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("scalars = %v, want %v", got, want)
	}
}

// YAML syntax the reader of yamlScalars understands.
const (
	yamlKeySeparator   = ":"
	yamlListItem       = "- "
	yamlListPathKey    = "-"
	yamlComment        = "#"
	yamlBlockLiteral   = "|"
	yamlBlockFolded    = ">"
	yamlQuotes         = `'"`
	yamlPathSeparator  = "."
	yamlListItemIndent = len(yamlListItem)
)

// yamlLevel is one open mapping key with the indentation of its line.
type yamlLevel struct {
	indent int
	key    string
}

// yamlScalars reads the block mappings of a YAML document and returns
// every scalar value by its dotted key path, such as
// components.schemas.EventInput.properties.title.maxLength. Entries of a
// list appear under the key "-". It skips comments and block scalars, so
// text within a description never counts as a key. It understands just
// the YAML the spec uses, which needs no YAML module.
func yamlScalars(text string) map[string]string {
	scalars := map[string]string{}
	var open []yamlLevel
	blockIndent := -1
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if blockIndent >= 0 && (trimmed == "" || indent > blockIndent) {
			continue
		}
		blockIndent = -1
		if trimmed == "" || strings.HasPrefix(trimmed, yamlComment) {
			continue
		}
		for len(open) > 0 && open[len(open)-1].indent >= indent {
			open = open[:len(open)-1]
		}
		for strings.HasPrefix(trimmed, yamlListItem) {
			open = append(open, yamlLevel{indent: indent, key: yamlListPathKey})
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, yamlListItem))
			indent += yamlListItemIndent
		}
		key, value, isMapping := strings.Cut(trimmed, yamlKeySeparator)
		if !isMapping {
			continue
		}
		key, value = strings.Trim(key, yamlQuotes), strings.TrimSpace(value)
		switch {
		case value == "":
			open = append(open, yamlLevel{indent: indent, key: key})
		case strings.HasPrefix(value, yamlBlockLiteral) || strings.HasPrefix(value, yamlBlockFolded):
			blockIndent = indent
		default:
			scalars[yamlPath(open, key)] = strings.Trim(value, yamlQuotes)
		}
	}
	return scalars
}

func yamlPath(open []yamlLevel, key string) string {
	keys := make([]string, 0, len(open)+1)
	for _, level := range open {
		keys = append(keys, level.key)
	}
	return strings.Join(append(keys, key), yamlPathSeparator)
}

// importSchemaRefPrefix is how the import schema refers to a schema of the
// spec served next to it.
const importSchemaRefPrefix = "openapi.yaml#/components/schemas/"

// Keys of the import schema the contract checks.
const (
	jsonSchemaRefKey  = "$ref"
	specSchemasPrefix = "components.schemas."
)

// forbiddenImportKeys are identifiers an import file never carries (AD-14).
var forbiddenImportKeys = []string{"id", core.EventFieldLocationID}

func TestImportSchemaIsValidJSONAndRefersOnlyToSchemasOfTheSpec(t *testing.T) {
	var schema any
	if err := json.Unmarshal(apispec.ImportSchemaV1, &schema); err != nil {
		t.Fatalf("import schema is no valid JSON: %v", err)
	}
	scalars := yamlScalars(string(apispec.OpenAPISpec))

	keys, refs := jsonKeysAndRefs(schema)

	if len(refs) == 0 {
		t.Error("import schema refers to no schema of the spec")
	}
	for _, ref := range refs {
		name, found := strings.CutPrefix(ref, importSchemaRefPrefix)
		if !found || !hasKeyWithPrefix(scalars, specSchemasPrefix+name+yamlPathSeparator) {
			t.Errorf("$ref %q points to no schema of openapi.yaml", ref)
		}
	}
	for _, forbidden := range forbiddenImportKeys {
		if slices.Contains(keys, forbidden) {
			t.Errorf("import schema has the key %q", forbidden)
		}
	}
	properties := schema.(map[string]any)["properties"].(map[string]any)
	formatVersion := properties["formatVersion"].(map[string]any)["const"]
	if formatVersion != float64(core.ImportFormatVersion) {
		t.Errorf("formatVersion const = %v, core %d", formatVersion, core.ImportFormatVersion)
	}
	maxEntries := properties["events"].(map[string]any)[maxItemsKey]
	if maxEntries != float64(core.MaxImportEntries) {
		t.Errorf("events maxItems = %v, core %d", maxEntries, core.MaxImportEntries)
	}
}

// jsonKeysAndRefs returns every object key in value and the targets of its
// $ref keys.
func jsonKeysAndRefs(value any) (keys, refs []string) {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			keys = append(keys, key)
			if ref, isText := nested.(string); key == jsonSchemaRefKey && isText {
				refs = append(refs, ref)
			}
			nestedKeys, nestedRefs := jsonKeysAndRefs(nested)
			keys, refs = append(keys, nestedKeys...), append(refs, nestedRefs...)
		}
	case []any:
		for _, nested := range typed {
			nestedKeys, nestedRefs := jsonKeysAndRefs(nested)
			keys, refs = append(keys, nestedKeys...), append(refs, nestedRefs...)
		}
	}
	return keys, refs
}

func hasKeyWithPrefix(scalars map[string]string, prefix string) bool {
	for path := range scalars {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}
