---
title: 'Story 3.6: Import vor verlorenen Änderungen schützen und Import-Schlüssel pflegen'
type: 'feature'
created: '2026-10-06'
status: 'done'
route: 'dispatch'
baseline_commit: '773db0646912d8875d339caac261126d05023e8d'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/planning-artifacts/sprint-change-proposal-2026-10-06-teil-c.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `CommitImport` überschreibt bei `update` und „überschreiben“ ein Event, das seit der Vorschau im Admin geändert wurde, ohne dass jemand die Änderung gesehen hat. Der `importKey` eines Events ist im Admin unsichtbar und lässt sich nicht entfernen; ein falsch überschriebenes Event hängt dauerhaft am Schlüssel.

**Approach:** Der Kern leitet aus jedem gespeicherten Event einen Fingerabdruck ab. Die Vorschau liefert ihn für Ziel und Bestands-Kandidaten, das Formular schickt ihn zurück, und `isStale` meldet `stale`, wenn sich das Event, das geschrieben würde, geändert hat. Neuer Anwendungsfall `RemoveImportKey`; die Bearbeitungsseite zeigt den Schlüssel lesend und bietet „Import-Schlüssel entfernen“ mit Rückfrage. Die Spec-Beschreibung von `importKey` erklärt seine Bedeutung.

## Boundaries & Constraints

**Always:** Test-first. Fingerabdruck = SHA-256 (hex) über die kanonische Form (`EventInputOf(e).Canonicalize()`, also inkl. `LocationID` und sortiertem Ablaufplan) plus `ImportKey`. Geprüft wird nur das Event, das tatsächlich geschrieben würde (Ziel bei `update`, `OverwriteID` bei „überschreiben“). Fehlt der Fingerabdruck in der Entscheidung, ist der Eintrag `stale`. `RemoveImportKey` läuft über `TxRunner`, prüft per `Get` (unbekannt → `ErrNotFound`) und ruft `SetImportKey(id, "")`. Neue Admin-Route nur POST, geschützt wie alle anderen (Session, `CrossOriginProtection`), Rückfrage per `hx-confirm` wie beim Löschen. Deutsche Texte, englischer Code.

**Never:** Kein Zwischenstand auf dem Server, keine Migration, keine Änderung an `api.gen.go` von Hand. `SaveEvent` ändert `importKey` weiterhin nie. Archivmarke und abgeleitete Werte gehören nicht zum Fingerabdruck. Kein Bearbeiten des Schlüssels im Formular, nur Entfernen.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Ziel geändert | `update` auf X, X danach im Admin geändert (Uhrzeit, Notiz, Ablaufplan, Ort) | `stale`, X unverändert | – |
| Überschreiben-Ziel geändert | Verdacht, Wahl „überschreiben“ Y, Y geändert | `stale` | – |
| Anderer Kandidat geändert | Verdacht mit Kandidaten Y, Z; Wahl „überschreiben“ Y oder „neu“/„überspringen“; Z geändert | Entscheidung gilt | – |
| Ziel unverändert | Fingerabdruck stimmt | Verhalten wie Story 3.3 | – |
| Fingerabdruck fehlt/verfälscht | Formular ohne oder mit falschem Wert | `stale` | – |
| Schlüssel entfernen | Event mit `importKey`, Knopf bestätigt | Schlüssel `NULL`, übrige Felder gleich, Weiterleitung auf die Bearbeitungsseite; späterer Import mit dem Schlüssel ist nicht `update` dieses Events | – |
| Unbekanntes Event | POST für gelöschtes/ungültiges ID | Kern `ErrNotFound` | deutsche 404-Seite „Dieses Event ist nicht mehr vorhanden.“ |

</frozen-after-approval>

## Code Map

- `internal/core/import_commit.go` -- `ImportDecision` (neu `Fingerprints map[string]string`, Event-ID → Fingerabdruck), `isStale` (Fingerabdruck des geschriebenen Events prüfen), `planEntry`.
- `internal/core/import_classify.go` -- `classifyAgainstStore`: Fingerabdruck des Ziels (`eventsByImportKey`) und jedes Bestands-Kandidaten aus `c.eventsByID` (nicht aus `FindDuplicateCandidates`, das liefert keinen Ablaufplan).
- `internal/core/import.go` -- `ImportEntry` (neu `TargetFingerprint`), `ImportCandidate` (neu `Fingerprint`, nur Bestand); neue Methode `StoredFingerprints()` neben `StoredCandidateIDs()`.
- `internal/core/event.go` -- neue Funktion `EventFingerprint(Event) string` neben `EventInputOf`.
- `internal/core/event_service.go` -- `RemoveImportKey` nach dem Muster `DeleteEvent`/`deleteEvent`; Doku von `SetImportKey` (ruft auch `RemoveImportKey`).
- `internal/adapter/admin/import.go` -- Feldpräfix `fingerprints` (Wert `id:fp id:fp`), `importHiddenFieldsOf`, `importDecisionsOf`.
- `internal/adapter/admin/events.go` -- `EventUseCases` + `RemoveImportKey`, Route `…/{id}/import-key/remove`, `eventForm.importKey`, `eventFormPage.ImportKey`.
- `internal/adapter/admin/handler.go` -- Route registrieren (Zeilen 146–151).
- `internal/adapter/admin/templates/event_form.html` -- Zeile „Import-Schlüssel“ und Formular außerhalb des Hauptformulars, vor `deleteForm`.
- `api/v1/openapi.yaml:877` -- Beschreibung `importKey`; danach `go generate ./...`.
- `internal/adapter/postgres/events.go:270` -- `SetImportKey` mit `""` schreibt per `optionalText` `NULL`; nicht ändern, nur testen.
- Testhelfer: `internal/core/import_commit_test.go` `decisionsOf`, `internal/adapter/admin/import_commit_test.go` `entryDecision`/`decisionFields`, `cmd/eventstore/testcollection_test.go:120`.

## Nahtstellen

- `isStale`/`planEntry` -- Story 3.3 -- bisherige `stale`-Gründe (Klasse, Ziel, neuer Ort, Kandidaten) bleiben; neu der Fingerabdruck -- I/O-Matrix, bestehende Tests in `import_commit_test.go`.
- Entscheidungsformular `importHiddenFieldsOf`/`importDecisionsOf` -- Story 3.3 -- Felder reisen mit, ohne Feld → `stale` -- Admin-Test Vorschau→Commit.
- `classifyAgainstStore`/`changesOf` -- Story 3.2 -- Klassen und `unchanged`-Vergleich unverändert -- `import_classify_test.go` bleibt grün.
- Abnahme Testsammlung `testcollection_test.go` -- Story 3.5 -- zweiter Lauf nur `unchanged`, 0 neue Events; Helfer trägt Fingerabdrücke -- Postgres-Test.
- `renderEventForm`/`renderEventFormAgain` -- Stories 1.7, 1.11 -- nach 422/409 zeigt die Seite den Schlüssel weiter (gespeichertes Event wird schon geladen), Löschrückfrage unverändert -- Admin-Test.
- `saveEvent` lässt `importKey` stehen -- Story 3.2 -- Speichern im Admin entfernt den Schlüssel nicht -- bestehender Test.

## Tasks & Acceptance

**Execution:**
- [x] `internal/core/event.go` + Test -- `EventFingerprint`: gleich für gleiche kanonische Form, verschieden bei geändertem Feld, Ort, Ablaufplan, `ImportKey`; Reihenfolge des Ablaufplans egal.
- [x] `internal/core/import.go`, `import_classify.go` + Tests -- Fingerabdrücke in Vorschau setzen, `StoredFingerprints()`.
- [x] `internal/core/import_commit.go` + Tests -- `Fingerprints` in `ImportDecision`, `isStale` erweitern; Testhelfer `decisionsOf` füllt sie; I/O-Matrix Zeilen 1–5.
- [x] `internal/core/event_service.go` + Test -- `RemoveImportKey` (Erfolg, `ErrNotFound`, Repo-Fehler, läuft in `InTx`).
- [x] `internal/adapter/admin/import.go` + Tests -- Feld senden und lesen; Helfer `entryDecision` erweitern.
- [x] `internal/adapter/admin/events.go`, `handler.go`, `templates/event_form.html` + Tests -- Anzeige nur bei Schlüssel, Knopf mit Rückfrage, POST → Kern, Weiterleitung (htmx: `HX-Redirect`), 404, sonstiger Fehler 500; Log `admin event import key removed` mit ID.
- [x] `api/v1/openapi.yaml` -- Beschreibung: identifiziert genau ein Event, erneuter Import aktualisiert es auch im Archiv, jeder Termin einer Reihe braucht einen eigenen Schlüssel; `go generate ./...`.
- [x] `cmd/eventstore/testcollection_test.go` -- Helfer trägt `StoredFingerprints()`.
- [x] `internal/adapter/postgres/events_test.go` -- `SetImportKey(id, "")` liefert danach leeren `ImportKey`, Schlüssel wieder frei.

**Acceptance Criteria:**
- Given ein Event mit `importKey`, when ich es bearbeite, then sehe ich den Schlüssel nur lesend; ein Event ohne Schlüssel zeigt keine solche Zeile.
- Given die Spec, when ich `EventInput.importKey` lese, then steht dort die Bedeutung laut Approach.
- Given alle Änderungen, when die CI läuft, then sind Abdeckung (100 % Kern/Admin), Lint, Generator-Diff und Postgres-Tests grün.

## Implementation Notes

- Fingerabdruck: SHA-256 über `fmt.Sprintf("%#v", …)` der kanonischen Eingabe samt `ImportKey`; `EventInputOf` setzt `Location` nie, die Darstellung enthält also keine Zeigeradresse.
- `isStale` aufgeteilt in `hasStaleCandidates` und `storedEventToWrite`; Duplikat-Hinweise bei `update` tragen ebenfalls einen (ungenutzten) Fingerabdruck.
- Formularfeld `fingerprints-<pos>` mit Paaren `id:fp`, nach ID sortiert, nur wenn das Eintrag Bestands-Events hat.
- Go-Multipart erlaubt höchstens 1000 Teile: `TestImportCommitReadsTheFormOfTheMostEntries` sendet `newLocation` nicht mehr für alle 150 Einträge, weil ein Eintrag mit neuem Ort nie Kandidaten hat; höchstens sendet ein Verdachtsfall 6 Felder, ein `update` mit neuem Ort 5 (901 Teile).
- Postgres-Tests in `internal/adapter/postgres/import_key_test.go` (dort liegen die übrigen `SetImportKey`-Tests) statt `events_test.go`; lokal ohne Datenbank übersprungen, laufen in der CI.
- `cmd/eventstore/main_test.go`: Stub `emptyEvents` um `RemoveImportKey` ergänzt.
- Nach CI-Fehler in PR #68: `TestRunCommitThatRepairsAMarkedEventClearsTheMark` (`cmd/eventstore/import_test.go`, Story 2.5 Teil B) baute das Commit-Formular eines `update` von Hand ohne Fingerabdruck und war deshalb korrekt `stale`; der Test sendet jetzt den Fingerabdruck des gespeicherten Ziels. Die Nahtstellen-Liste hatte diesen Ende-zu-Ende-Test nicht erfasst.

## Spec Change Log

## Review Triage Log

| # | Layer | Befund | Verdikt | Begründung | Route |
|---|-------|--------|---------|------------|-------|
| 1 | edge | Admin-Speichern zwischen `List` und `Update` im Commit überschreibt trotz passendem Fingerabdruck | false | Alle Schreib-Hüllen laufen über `TxRunner` mit Advisory-Sperre (AD-6, Story 3.3); `SaveEvent` wartet bis nach dem Commit. | – |
| 2 | edge/blind | Kandidat fehlt in `eventsByID`, Fingerabdruck eines leeren Events | false | `List` und `FindByDuplicateKey` laufen in derselben gesperrten Transaktion über dasselbe Repo; kein Schreiber kann dazwischen ein Event anlegen, IDs kommen in derselben Schreibweise. | – |
| 3 | edge | `RemoveImportKey` parallel zu `CommitImport` | false | Beide laufen über `TxRunner` mit derselben Sperre, also nacheinander. | – |
| 4 | edge | Redirect/Log mit angefragter statt gespeicherter ID | low | Wie `deleteEvent`, das ebenfalls die angefragte ID loggt; eine UUID in Großbuchstaben funktioniert als URL. Kaum je im Alltag. | reject |
| 5 | verification | Kein Admin-Test mit zwei Fingerabdruck-Paaren und Überschreiben des zweiten Kandidaten | medium | Alle Admin-Tests senden höchstens ein Paar und prüfen dann ein Schreiben; ein Parse-Fehler beim zweiten Paar bliebe unbemerkt. | patch |
| 6 | blind | Kommentar zu `TestImportCommitReadsTheFormOfTheMostEntries` falsch (update mit neuem Ort sendet `target`, `newLocation`, `fingerprints`) | low | Stimmt (`classifyAgainstStore` setzt das Ziel vor der Ortsprüfung); das Maximum von 6 Feldern bleibt. Direkte Korrektur. | patch |
| 7 | blind | Wenig Luft bis zur Multipart-Grenze von 1000 Teilen | low | 901 von 1000 Teilen; nur bei weiteren Feldern relevant, eine Absicherung bräuchte neue Logik. | reject |
| 8 | blind | `%#v` könnte Zeigeradressen hashen | low | `EventInputOf` baut aus `Event` ohne Zeigerfelder, `Location` bleibt nil; der Test vergleicht zwei getrennt gebaute Events. Nur bei künftiger Änderung relevant. | reject |
| 9 | blind | Entfernen eines fehlenden Schlüssels loggt „removed“ | low | Idempotent und harmlos; eine Unterscheidung bräuchte einen neuen Zweig. | reject |
| 10 | blind | OpenAPI-Beschreibung irreführend und nennt den Admin | low | Satz beschreibt das Event, das wie eines ohne Schlüssel behandelt wird (FR-16); der Admin-Hinweis folgt der PRD. | reject |
| 11 | blind | Admin-Stale-Test ändert im Fall „target changed“ das gespeicherte Event nicht; `veraltet: 4` zählt unentschiedene Einträge mit | low | Stimmt: der Fall gleicht „falsified“. Direkte Korrektur des Tests. | patch |
| 12 | blind | Postgres-Test vergleicht nur Fingerabdrücke statt aller Felder | low | Archivmarke, abgeleitete Werte und Ablaufplan-IDs bleiben ungeprüft; direkter Vergleich der Structs. | patch |
| 13 | blind | Verification nennt Postgres „grün“, Notes sagen übersprungen | – | Fix wäre eine Änderung dieser Spec. | reject |
| 14 | blind | Abgehakter Task nennt `events_test.go` | – | Fix wäre eine Änderung dieser Spec; die Implementation Notes nennen die Datei. | reject |
| 15 | blind | Spec- und Sprint-Status weichen ab | false | Werden am Ende synchronisiert (Override). | – |
| 16 | blind | Testhelfer `assertDeleteLogged`, `importKeyFormTag` mit festem Schlüssel | low | Rein kosmetisch in Tests. | reject |
| 17 | blind | `target` in deferred-work.md enthält Begründung | false | Die Deferred-Regel des Workflows verlangt, die Begründung an `target` anzuhängen. | – |
| 18 | blind | ACs decken Kernfunktion nicht ab | – | Fix wäre eine Änderung dieser Spec; die I/O-Matrix deckt beides ab. | reject |
| 19 | seam | Fingerabdruck nie gegen Postgres durch einen Import geprüft | low | Vorschau und Commit lesen über denselben Code (`List`); `compareTimetableInputs` vergleicht alle Felder, Gleichstände sind identisch, das Ergebnis ist also deterministisch. Ein neuer Integrationstest geht über eine direkte Korrektur hinaus. | reject |
| 20 | seam | 409-Pfad mit Import-Schlüssel ungetestet | low | Nahtstelle nennt 422/409, getestet ist nur 422; ein weiterer Testfall ist direkt. | patch |

## Design Notes

„Samt Ort“ (Proposal V4) ist die `LocationID` des Events: Nur sie schreibt ein `update`; Änderungen am Ort selbst lässt der Import ohnehin unangetastet. Der Fingerabdruck reist pro Eintrag in einem Feld `fingerprints-<pos>` wie die Kandidaten-IDs; so bleibt das Formular bei 150 Einträgen klein. Die README nennt `importKey` nicht; die Bedeutung steht in `openapi.yaml`, das per `$ref` auch das Import-Schema dokumentiert.

## Verification

**Commands:**
- `CI= go test ./...` -- grün
- `EVENTSTORE_TEST_DATABASE_URL=… go test -p 1 ./internal/adapter/postgres/... ./cmd/eventstore/...` -- grün
- `bash scripts/check-coverage.sh` -- 100 %
- `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./...` -- keine Befunde
- `go generate ./...` danach `git status` -- `api.gen.go` aktualisiert und committet

**Manual checks:**
- Bearbeitungsseite eines importierten Events: Schlüssel sichtbar, Entfernen fragt nach und entfernt.
