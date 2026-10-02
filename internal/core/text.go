package core

import (
	"strings"

	"golang.org/x/text/unicode/norm"
)

// keySeparator joins the words of a normalized key.
const keySeparator = " "

// sortFolding maps German umlauts and sharp s to their base letters, so the
// sort order matches what a German reader expects from a phone book.
var sortFolding = strings.NewReplacer("ä", "a", "ö", "o", "ü", "u", "ß", "ss")

// normalizeText is the single place where the core canonicalizes text
// input: NFC composition, then surrounding whitespace trimmed (including
// no-break spaces). A text of only whitespace becomes empty, i.e. missing.
func normalizeText(s string) string {
	return strings.TrimSpace(norm.NFC.String(s))
}

// NormalizeKey returns the comparison key of a name: NFC-composed, trimmed,
// every run of whitespace (including no-break spaces) collapsed to one
// space, lowercased. Two names with the same key count as the same name.
func NormalizeKey(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(norm.NFC.String(s)), keySeparator))
}

// sortKey returns the key that orders names: NormalizeKey with ä, ö, ü and
// ß folded to a, o, u and ss.
func sortKey(s string) string {
	return sortFolding.Replace(NormalizeKey(s))
}
