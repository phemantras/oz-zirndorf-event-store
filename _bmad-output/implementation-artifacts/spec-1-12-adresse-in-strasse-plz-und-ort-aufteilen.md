---
title: 'Story 1.12: Adresse in Straße, PLZ und Ort aufteilen'
type: 'feature'
created: '2026-10-02'
status: 'done'
baseline_commit: '147c1c8223f3ff54eede359422aba1e70d290cf4'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-1-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Ein Ort hat nur eine Freitext-Adresse; Abnehmer müssten Straße, PLZ und Ort daraus raten (FR-6, Sprint Change Proposal 2026-10-02). Die Akzeptanzkriterien der Story 1.12 in `epics.md` gelten vollständig.

**Approach:** Kern ersetzt `Address` durch `Street`, `PostalCode`, `City` mit Pflicht- und PLZ-Prüfung. Migration `00003` als Expand-Schritt, sqlc-Abfragen und Repository auf die neuen Spalten. Admin-Formular mit drei Feldern, Liste zeigt „Straße, PLZ Ort“. README-Beispiel mit Objekt `address`.

## Boundaries & Constraints

**Always:** Alle drei Teile über `normalizeText` (NFC + Trim); nur Leerraum = fehlt. PLZ nach dem Trimmen genau fünf ASCII-Ziffern `0`–`9`, sonst neuer Problemcode `invalidFormat` mit eigener Meldung „Die PLZ muss aus genau fünf Ziffern bestehen.“. Feldnamen-Konstanten `street`, `postalCode`, `city` ersetzen `LocationFieldAddress`; das Formular nutzt sie als `name`. Deutsche Meldungen: „Bitte Straße und Hausnummer angeben.“, „Bitte eine PLZ angeben.“, „Bitte einen Ort angeben.“. Listenzelle setzt nur nicht leere Teile zusammen (`Straße, PLZ Ort`); ganz leer = leere Zelle. Test-first, 100 % Abdeckung für `internal/core` und `internal/adapter/admin`.

**Never:** `00002` ändern. Spalte `address` lesen, schreiben oder per SQL aufteilen. `address` oder Defaults entfernen (Contract, Story 1.7). Fachregeln in SQL (`CHECK` für PLZ). Kartenpicker (1.5). Generierten sqlc-Code von Hand ändern.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Anlegen ok | `Volkhardtstraße 2`, `" 90513 "`, `Zirndorf` | 303, gespeichert mit PLZ `90513` | N/A |
| Teil fehlt | Straße `"   "`, PLZ leer, Ort `" "` | 422, drei Feldmeldungen, Eingaben erhalten | `ErrValidation`, je Feld `missing` |
| PLZ falsch | `9051`, `90513a`, `D-90513`, `９０５１３` | 422, PLZ-Formatmeldung | `ErrValidation`, `postalCode invalidFormat` |
| Bearbeiten | nur PLZ/Ort geändert | gleiche `id` | N/A |
| Altbestand | Zeile mit `address`, Teile `''` | Liste ohne Fehler, Zelle leer; Formular öffnet mit leeren Feldern, Speichern mit Teilen klappt | N/A |

**Entscheidungen (2026-10-02):** Volle Spec trotz Überlänge (~1.900 Tokens) behalten.

</frozen-after-approval>

## Code Map

- `internal/core/location.go` -- `Location`/`LocationInput`/`newLocation`/Feld-Konstanten; Muster `report(field, problem)` fortführen. PLZ-Prüfung als eigene kleine Funktion analog `parsePrecision`.
- `internal/core/errors.go` -- `FieldProblem`-Konstanten; neue `ProblemInvalidFormat = "invalidFormat"`.
- `internal/core/location_test.go`, `location_service_test.go` -- Fixtures mit `Address:` auf drei Teile umstellen.
- `internal/adapter/postgres/migrations/` -- neue `00003_location_address_parts.sql` (nur `-- +goose Up`, Kommentarstil wie `00002`).
- `internal/adapter/postgres/queries/locations.sql` -- explizite Spaltenlisten ohne `address`. Dadurch erzeugt sqlc je Abfrage eine eigene Row-Struktur statt `db.Location`; gleiche Felder → per Typkonvertierung auf eine `locationFromRow` abbilden.
- `internal/adapter/postgres/locations.go` (+ `locations_test.go`) -- Params/Mapping umstellen; Test für Altbestand per direktem `INSERT … (name, name_key, address, …)` über den Pool.
- `internal/adapter/admin/locations.go` -- `fieldMessages`, `locationRow.Address` (zusammengesetzt), `inputFromForm`, `inputFromLocation`.
- `internal/adapter/admin/templates/location_form.html`, `locations.html` -- drei Felder statt „Adresse“; Spalte „Adresse“ bleibt.
- `internal/adapter/admin/locations_test.go` -- `hallAddress`/Formular-Fixture auf drei Teile; Tests für alle Matrix-Zeilen.
- `cmd/eventstore/main_test.go` -- `emptyLocations`/Postgres-`run`-Test nur anpassen, falls sie Adressfelder nutzen.
- `README.md:66` -- `"address": { "street": "Marktplatz", "postalCode": "90513", "city": "Zirndorf" }`.

## Tasks & Acceptance

**Execution:**
- [x] `internal/core/errors.go`, `location.go` (+ Tests) -- `ProblemInvalidFormat`, drei Felder, Pflicht- und PLZ-Prüfung, NFC -- Kernregel ohne DB.
- [x] `internal/adapter/postgres/migrations/00003_location_address_parts.sql` -- `ADD COLUMN street|postal_code|city text NOT NULL DEFAULT ''`, `ALTER COLUMN address DROP NOT NULL` -- Expand (AD-17).
- [x] `queries/locations.sql` + `sqlc generate` (1.31.1), `locations.go` (+ Test gegen PG 18) -- neue Spalten, Altbestand lesbar und per `Update` vervollständigbar.
- [x] `internal/adapter/admin/locations.go`, Templates (+ Tests) -- Formular, Meldungen, Listen-Zusammensetzung.
- [x] `README.md` -- Beispiel mit Objekt `address`.

**Acceptance Criteria:**
- Given das Ortsformular, when es sich öffnet, then hat es „Straße und Hausnummer“, „PLZ“ und „Ort“ und kein Feld „Adresse“.
- Given ein Ort mit allen drei Teilen, when die Liste lädt, then zeigt die Spalte „Adresse“ `Volkhardtstraße 2, 90513 Zirndorf`.
- Given die Migration auf einer DB mit Altbestand, when `goose up` läuft, then bleiben vorhandene Zeilen erhalten, und `sqlc generate` erzeugt keinen Diff.

## Implementation Notes

- sqlc erzeugt je Abfrage eine eigene Row-Struktur; `postgres/locations.go` konvertiert sie in `locationRow` (= `db.GetLocationRow`) und bildet sie über eine `locationFromRow` ab.
- Migrationstest `TestMigrateKeepsLegacyLocationsWhenSplittingAddress` migriert in einem eigenen Schema bis Version 2, legt einen Altort an und migriert weiter.
- Lokal verifiziert (2026-10-02): `CI= go test ./...`, Coverage-Gate 355/355, `go vet`, golangci-lint v2.14.0 ohne Befund, `sqlc generate` ohne Diff. Postgres-Tests nur in der CI (kein Docker).

## Spec Change Log

## Review Triage Log

| # | Quelle | Befund | Verdikt | Evidenz | Route |
|---|--------|--------|---------|---------|-------|
| 1 | blind + edge + gap | Rollback bricht: neue Orte haben `address` NULL, alter Code scannt in `string` | medium | Alter `db.Location.Address` ist `string`; pgx scannt NULL nicht in `string`; Migration setzt keinen Default | patch |
| 2 | edge | Bearbeitungen während eines Rollbacks ändern nur `address`, Teile veralten | low | Nur bei Rollback mit Bearbeitung; Produktionsdaten sind Testdaten | reject |
| 3 | blind | Status in Spec und Sprint-Status uneinheitlich | false | Workflow setzt den Sprint-Status beim Abschluss | reject |
| 4 | blind | Admin sieht alte Freitext-Adresse nicht | low | Intent: `address` wird nicht gelesen; Testdaten werden von Hand nachgepflegt oder gelöscht | reject |
| 5 | blind | Unvollständige Altorte in der Liste nicht markiert | low | Nur Testdaten; 1.7 prüft vor dem Contract per SQL | reject |
| 6 | blind | `<td></td>`-Assertion zu schwach | false | `locations.html` hat sonst keine leere Zelle: Genauigkeit und Koordinaten sind gesetzt | reject |
| 7 | blind | PLZ-Feld ohne `maxlength`/`pattern`/`autocomplete` | low | Formular ist `novalidate`; Autofill würde die Adresse des Admins einsetzen, nicht die des Orts | reject |
| 8 | blind | Altbestand-INSERT doppelt in zwei Testdateien | low | Nur Testcode; Helper bringt kaum Nutzen | reject |
| 9 | blind | Migrationstest liest `os.DirFS` statt eingebettetem FS | low | Gleiches Verzeichnis, das eingebettet wird | reject |
| 10 | blind | Flache Feldnamen kollidieren mit späterem `address`-Objekt | low | Story-AC legt `street`, `postalCode`, `city` als Kern-Feldnamen fest; Story 2.1 entscheidet die API-Form | reject |
| 11 | blind | Kopfkommentar der Query-Datei landet am generierten `ListLocations` | low | Schon vor dieser Story so; kosmetisch | reject |
| 12 | blind | Kein Test für PLZ mit führender Null | low | Prüfung ist rein textbasiert, kein Fehlverhalten nachweisbar | reject |
| 13 | blind | Diff enthält Änderungen an `epic-1-context.md` | false | Vom Workflow neu kompilierter Epic-Kontext, gehört dazu | reject |
| 14 | blind | sqlc-Aktualität nur manuell geprüft; AGENTS.md-TODO offen | low | CI-Lint-Job prüft sqlc schon (`ci.yaml`); TODO betrifft Agent-Kontextdatei | defer |

## Design Notes

Der Problemcode `invalidFormat` ist allgemein benannt, damit spätere Formatregeln (Import, Story 3.1) ihn wiederverwenden. Zusammensetzen der Adresse ist Anzeige und gehört in den Admin, nicht in den Kern; die API liefert später die Teile einzeln.

## Verification

**Commands:**
- `CI= go test ./...` -- grün
- `bash scripts/check-coverage.sh` -- 100 %
- `go vet ./...`, golangci-lint v2.14.0 -- ohne Befund
- `sqlc generate` (1.31.1) -- kein Diff nach dem Commit

**Manual checks (if no CLI):**
- Postgres-Tests laufen in der CI (lokal ohne Docker nicht).
- Vor dem Merge (= Deploy): `pg_dump` nach README ziehen und Dump-Ende prüfen (AD-17). Nach dem Deploy Testorte nachpflegen oder löschen.
