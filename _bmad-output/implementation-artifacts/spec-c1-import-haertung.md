---
title: 'C1: Import-Härtung (Retro Epic 3)'
type: 'bugfix'
created: '2026-10-06'
status: 'done'
route: 'dispatch'
baseline_commit: '4582f0b0819da0f3a3563fecbe3e31e6efa32e63'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-3-retro-2026-10-06.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Der Import verfälscht Daten oder scheitert ohne Hinweis bei ungewöhnlichen Eingaben (Retro Epic 3, R1–R4): Eine Datei, die kein UTF-8 ist, wird still mit U+FFFD gelesen. NUL-Zeichen lassen den ganzen Commit in PostgreSQL scheitern. Ein Ortsname, den mehrere Bestandsorte nach `NormalizeKey` tragen, wird zufällig zugeordnet. „Überschreiben“ wird auch dann angeboten, wenn es sicher scheitert, und die Ergebnisliste nennt für „fehlerhaft“ keinen Grund.

**Approach:** Test-first im Kern und im Admin: (a) Nicht-UTF-8-Datei als Ganzes ablehnen (neues `ImportFileProblem`); (b) Steuerzeichen in Textfeldern beim Lesen am Feld melden; (c) mehrdeutigen Ortsnamen als Fehler am Feld `location.name` melden; (d) „überschreiben“ nur für Kandidaten anbieten, deren `importKey` nicht widerspricht, und `ImportResult` um einen Grund für `error` ergänzen (gleiches Ziel, anderer Schlüssel), den die Ergebnisliste zeigt.

## Boundaries & Constraints

**Always:** Test-first, 100 % Abdeckung in Kern und Admin. Die UTF-8-Prüfung läuft nach dem Größen-Check und nach dem Entfernen der BOM, vor dem JSON-Parsen. Steuerzeichen = `unicode.IsControl` außer Tab, LF, CR; gemeldet mit eigenem Problem-Code `controlCharacter` am Pfad des Feldes (wie JSON-Typfehler, sodass für das Feld kein weiteres Problem erscheint). Mehrdeutig heißt: zwei oder mehr Bestandsorte mit gleichem `NormalizeKey(name)`; Problem-Code `ambiguous` am Feld `location.name`, Klasse `error`. Ob „überschreiben“ möglich ist, entscheidet der Kern (dieselbe Regel wie `hasOtherImportKey`); der Admin liest es nur. Deutsche Meldungen, englischer Code, Codes als Kern-Konstanten mit Test.

**Never:** Keine Migration, keine Änderung an `openapi.yaml`/`import-v1.schema.json` (die Problem-Codes sind kernintern wie `duplicateInFile`). Keine Änderung an Admin-Formularen für Events/Orte und an `SaveEvent`/`SaveLocation`. Keine deterministische Auswahl bei mehrdeutigem Ortsnamen. Kein Grund für `error` bei Einträgen mit Problemen (die Vorschau zeigt sie schon).

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| ANSI-Datei | Bytes `0xFC` (ü in Windows-1252) | Datei abgelehnt, 422, „Die Datei ist nicht in UTF-8 gespeichert …“ | nichts gelesen |
| UTF-8 mit BOM | gültige Datei mit BOM | wie bisher gelesen | – |
| NUL im Titel | `"title": "A\u0000B"` | Eintrag `error`, Problem `title`/`controlCharacter`, übrige Einträge unberührt | Commit: `error`, kein DB-Fehler |
| Zeilenumbruch in Notiz | `"note": "a\nb"` | gültig | – |
| Steuerzeichen verschachtelt | in `location.address.street`, `source.url`, `timetable[0].description`, `importKey` | Problem am jeweiligen Pfad | – |
| Mehrdeutiger Ort | Bestand: „Stadthalle“ und „Stadthalle “ (gleicher Schlüssel), Eintrag nennt „stadthalle“ | `error`, `location.name`/`ambiguous`; Vorschau und Commit gleich | – |
| Eindeutiger Ort | genau ein Bestandsort mit dem Schlüssel | wie bisher zugeordnet | – |
| Kandidat mit anderem Schlüssel | Verdacht, Eintrag `importKey` „a“, Kandidat „b“ | keine Option „überschreiben“ für ihn, Hinweis dazu; „überspringen“/„neu“ bleiben | – |
| Überschreiben trotzdem gesendet | Formular mit `overwrite:<id>` auf solchen Kandidaten | Ergebnis `error`, Grund „anderer Import-Schlüssel“ | – |
| Gleiches Ziel | zwei Einträge schreiben dasselbe Event | beide `error`, Grund „gleiches Ziel“ | – |

</frozen-after-approval>

## Code Map

- `internal/core/import.go` -- `readImportFile` (UTF-8 nach `TrimPrefix` BOM, neues `ImportProblemInvalidEncoding = "invalidEncoding"`); `readEventInput`/`readLocationInput`/`readTimetable`: alle `readJSON[string]` über neue Funktion `readText` (meldet `controlCharacter` per `reader`); `readImportEntry`/`resolveImportLocation` bekommen statt `map[string]Location` die Ortsliste je Schlüssel.
- `internal/core/import_classify.go` -- `newImportClassifier`: `locationsByKey` als `map[string][]Location`; `ImportCandidate` neu `CanOverwrite bool` (in `classifyAgainstStore` aus `hasOtherImportKey(c.eventsByID[id], *entry)`).
- `internal/core/import_commit.go` -- `ImportResult` neu `Reason ImportErrorReason` (`sharedTarget`, `otherImportKey`); `importPlan` trägt den Grund; `planImport`/`planEntry` setzen ihn.
- `internal/core/errors.go` -- `ProblemControlCharacter`, `ProblemAmbiguous`; Code-Test in `errors_test.go` wie `TestFieldProblemDuplicateInFileCode`.
- `internal/adapter/admin/import.go` -- `importFileMessages` (neue Meldung), `importOnlyMessages` (`location.name`/`ambiguous`), Meldung für `controlCharacter` an jedem Feld in `importProblemMessage`; `importChoicesOf` filtert `CanOverwrite`, Hinweis je ausgelassenem Kandidaten in `importRowOf`; `importSummaryViewOf` hängt den Grund an die Ergebnis-Bezeichnung („fehlerhaft: …“). Template bleibt unverändert.
- Tests: `internal/core/import_test.go`, `import_classify_test.go`, `import_commit_test.go`, `internal/adapter/admin/import_test.go`, `import_commit_test.go`.

## Nahtstellen

- `readImportFile` -- Story 3.1 -- BOM-Toleranz, Reihenfolge der Ablehnungsgründe (`tooLarge` zuerst) bleiben -- bestehende Tests in `import_test.go`, neue Zeile ANSI/BOM.
- `jsonReader`/`withoutFieldsWithin` -- Story 3.1 -- Typfehler bleiben `invalidFormat`; Steuerzeichen unterdrücken das „missing“ desselben Feldes -- Test NUL im Titel (nur ein Problem).
- `resolveImportLocation`/`storedLocationHints` -- Story 3.2 -- Zuordnung und Abweichungshinweise bei eindeutigem Ort unverändert -- `import_classify_test.go`.
- `RecomputeNameKeys` -- Story 2.5b -- lässt Kollisionsgruppen stehen; genau diese Gruppen meldet der Import jetzt als `ambiguous` -- Kern-Test mit zwei Orten gleichen Schlüssels.
- `planImport`/`planEntry` -- Stories 3.3, 3.6 -- Reihenfolge stale → error → gleiches Ziel und Fingerabdruck-Prüfung unverändert; nur der Grund kommt hinzu -- `import_commit_test.go`.
- `importChoicesOf`/`importHiddenFieldsOf` -- Stories 3.3, 3.6 -- Kandidaten-IDs und Fingerabdrücke werden weiter für alle Bestandskandidaten gesendet (sonst `stale`) -- Admin-Test.
- `importSummaryViewOf` -- Story 3.3 -- `stale`/`undecided` ohne Grund wie bisher -- Admin-Test.
- Abnahme `cmd/eventstore/testcollection_test.go` -- Story 3.5 -- Testsammlung enthält keine Steuerzeichen (geprüft) und keine mehrdeutigen Orte; Läufe unverändert -- Postgres-Test in der CI.

## Tasks & Acceptance

**Execution:**
- [x] `internal/core/errors.go` + Test -- zwei Problem-Codes.
- [x] `internal/core/import.go` + `import_test.go` -- UTF-8-Prüfung, `readText`, mehrdeutiger Ort; Matrix-Zeilen 1–7.
- [x] `internal/core/import_classify.go` + Test -- `locationsByKey` als Liste, `CanOverwrite`.
- [x] `internal/core/import_commit.go` + Test -- `ImportErrorReason`, Matrix-Zeilen 9–10; Einträge mit Problemen ohne Grund.
- [x] `internal/adapter/admin/import.go` + Tests -- Meldungen, Optionen, Hinweis, Grund in der Ergebnisliste; Matrix-Zeile 8.

**Acceptance Criteria:**
- Given eine Vorschau mit Verdachtsfall, dessen einziger Bestandskandidat einen anderen `importKey` hat, when ich die Seite sehe, then gibt es nur „Überspringen“ und „Als neues Event anlegen“ und einen Hinweis, warum „Überschreiben“ fehlt.
- Given alle Änderungen, when die CI läuft, then sind Abdeckung (100 % Kern/Admin), Lint, Generator-Diff und Postgres-Tests grün.

## Implementation Notes

- Ein Textfeld mit Steuerzeichen wird wie ein Typfehler als leerer Text gelesen; ein Titel mit NUL erscheint in Vorschau und Ergebnisliste daher ohne Titel (Triage #3).
- Postgres-Tests lokal nicht ausgeführt (kein Server, kein Docker); sie laufen in der CI. Manuelle Browserprüfung steht aus.

## Spec Change Log

## Review Triage Log

| # | Layer | Befund | Verdikt | Begründung | Route |
|---|-------|--------|---------|------------|-------|
| 1 | verification | Steuerzeichen in `source.description`, `location.note`, `location.address.city` ungetestet; Rückfall auf `readJSON` bliebe unbemerkt | medium | Vorgeprüft; genau diese Felder erreichen PostgreSQL als freier Text. | patch |
| 2 | edge/blind | Escapte einzelne Surrogate (`\udcfc`) werden von `encoding/json` still zu U+FFFD | low | Stimmt, aber kein Editor schreibt solche Escapes; die Prüfung bräuchte einen weiteren Zweig. | reject |
| 3 | edge/seam | Titel mit Steuerzeichen erscheint in Vorschau und Ergebnisliste leer | low | Stimmt (wie bei Typfehlern); Position und Meldung „Titel: …“ benennen den Eintrag. Ein bereinigter Anzeigetitel wäre neue Logik. | reject |
| 4 | edge/blind | Fehlendes Label für künftigen `ImportErrorReason` ergäbe „fehlerhaft: “ | low | Beide heutigen Gründe haben Labels (Test); eine Liste `ImportErrorReasons()` wäre neue öffentliche Fläche. | reject |
| 5 | blind | Tab/LF/CR auch in einzeiligen Feldern erlaubt | false | So im eingefrorenen Intent festgelegt; vorher galt dasselbe. | – |
| 6 | blind | Format-/Bidi-Zeichen (Cf, Zl, Zp) nicht geprüft | false | Intent definiert Steuerzeichen als `unicode.IsControl`; Cf liegt außerhalb. | – |
| 7 | blind | Meldung nennt Code-Point/Stelle des Steuerzeichens nicht | low | Komfort; bräuchte neue Felder in `FieldError`. | reject |
| 8 | blind | Mehrdeutiger Ort verlinkt die kollidierenden Orte nicht | low | Die Ortsliste zeigt sie nebeneinander; Links wären neue Hinweisart. Selten (nur Restkollisionen aus 2.5b). | reject |
| 9 | blind | Kein Test für mehrdeutigen Ort bei `update`/`unchanged` | low | Verhalten folgt dem Intent (jeder Eintrag); Kollisionsgruppen sind selten. | reject |
| 10 | blind | Kein Test mit gemischten Kandidaten (einer überschreibbar, einer nicht) | low | Filter wirkt je Kandidat; beide Zweige einzeln getestet. | reject |
| 11 | blind | Admin-Test „Grund“ prüft nur ein Feld und besteht auch bei fehlendem Event | low | Stimmt; direkte Korrektur im Test. | patch |
| 12 | blind | Admin-Test Steuerzeichen/mehrdeutig ordnet Meldungen keinen Zeilen zu, Eintrag 1 hat Zusatzprobleme | low | Stimmt; direkte Korrektur im Test. | patch |
| 13 | blind | `CanOverwrite` auf Hinweis-Kandidaten verwirrend; Admin verlässt sich auf Kern-Invariante | low | `TestImportOffersNoOverwriteForADuplicateOnlyOfTheFile` sichert die Invariante. | reject |
| 14 | blind | Neuer Deferred-Eintrag hat `source_spec: none` | false | Das Format der Scope-Teilung in Schritt 1 schreibt `none` vor. | – |
| 15 | blind | Spec wirkt unfertig (leere Abschnitte, keine Belege) | – | Fix wäre eine Änderung dieser Spec. | reject |
| 16 | seam | Steuerzeichen in ignorierten Feldern eines Bestandsorts (z. B. `location.note`) machen den Eintrag jetzt fehlerhaft | low | Folgt dem Intent („Steuerzeichen in Textfeldern“); eine Datei mit NUL ist ohnehin defekt. | reject |

## Design Notes

Die Retro nennt für Steuerzeichen `invalidFormat`. Der Admin übersetzt `invalidFormat` aber feldabhängig (z. B. PLZ „fünfstellig“, sonst „falscher Typ“), das wäre bei NUL irreführend; deshalb ein eigener Code. Offene Deferred-Einträge mit Ziel C1 gibt es keine; „Browser-Tests“ zielt auf C3.

## Verification

**Commands:**
- `CI= go test ./...` -- grün
- `bash scripts/check-coverage.sh` -- 100 %
- `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./...` -- keine Befunde
- `EVENTSTORE_TEST_DATABASE_URL=… go test -p 1 ./internal/adapter/postgres/... ./cmd/eventstore/...` -- grün (lokal oder CI)

**Manual checks:**
- Vorschau mit ANSI-Datei und mit einem Verdachtsfall mit anderem Import-Schlüssel im Browser.
