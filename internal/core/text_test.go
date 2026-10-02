package core

import "testing"

// Decomposed forms: base letter followed by a combining diaeresis (U+0308).
const (
	decomposedOelmuehle = "O\u0308lmu\u0308hle"
	composedOelmuehle   = "Ölmühle"
)

func TestNormalizeKey(t *testing.T) {
	tests := map[string]struct {
		input string
		want  string
	}{
		"lowercase":                   {"Paul-Metz-Halle", "paul-metz-halle"},
		"surrounding spaces":          {"  paul-metz-halle  ", "paul-metz-halle"},
		"double space":                {"Alte  Feuerwache", "alte feuerwache"},
		"tab and newline":             {"Alte\t\nFeuerwache", "alte feuerwache"},
		"no-break space":              {"Alte\u00a0Feuerwache", "alte feuerwache"},
		"narrow no-break space":       {"Alte\u202fFeuerwache", "alte feuerwache"},
		"no-break space at the edges": {"\u00a0Bibertpark\u202f", "bibertpark"},
		"umlauts are kept":            {"Zirndorfer ÖLMÜHLE", "zirndorfer ölmühle"},
		"decomposed equals composed":  {decomposedOelmuehle, "ölmühle"},
		"only whitespace":             {" \u00a0\t ", ""},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := NormalizeKey(tt.input); got != tt.want {
				t.Errorf("NormalizeKey(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeTextComposesAndTrims(t *testing.T) {
	tests := map[string]struct {
		input string
		want  string
	}{
		"trims spaces":                  {"  Marktplatz  ", "Marktplatz"},
		"trims no-break spaces":         {"\u00a0Marktplatz\u202f", "Marktplatz"},
		"keeps inner spaces":            {"Alte  Feuerwache", "Alte  Feuerwache"},
		"composes to NFC":               {decomposedOelmuehle, composedOelmuehle},
		"only whitespace becomes empty": {" \t\u00a0", ""},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := normalizeText(tt.input); got != tt.want {
				t.Errorf("normalizeText(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSortKeyFoldsCaseUmlautsAndSharpS(t *testing.T) {
	tests := map[string]struct {
		input string
		want  string
	}{
		"umlauts":            {"Ölmühle Äußere", "olmuhle aussere"},
		"decomposed umlauts": {decomposedOelmuehle, "olmuhle"},
		"sharp s":            {"Straße", "strasse"},
		"whitespace":         {" Alte\u00a0 Feuerwache ", "alte feuerwache"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := sortKey(tt.input); got != tt.want {
				t.Errorf("sortKey(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
