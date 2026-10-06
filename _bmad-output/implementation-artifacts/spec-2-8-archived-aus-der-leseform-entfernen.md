---
title: 'Story 2.8: `archived` aus der Leseform entfernen'
type: 'refactor'
created: '2026-10-06'
status: 'done'
baseline_commit: 'b4d87966c3baf9c0a210db9d2ca0490953a1879b'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-2-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Das Pflichtfeld `archived` im Schema `Event` wiederholt nur, welche Liste abgefragt wurde (`/v1/events` immer `false`, `/v1/archive/events` immer `true`), und veraltet bei Abnehmern, die Events zwischenspeichern. Es gelten die ACs von Story 2.8 in `epics.md` (Sprint Change Proposal 2026-10-06 Teil B, einmalige Ausnahme von NFR-2).

**Approach:** `archived` aus Spec (`properties`, `required`, Beispiele, Texte) streichen, `api.gen.go` neu erzeugen, Abbildung im Adapter `publicapi/v1` entfernen. Die Texte erklären: Die Liste sagt, ob ein Event vorbei ist; wer zwischenspeichert, prüft `effectiveEnd` gegen die aktuelle Zeit.

## Boundaries & Constraints

**Always:** Test-first: zuerst die Leseform-Tests auf die Schlüsselmenge ohne `archived` umstellen und rot sehen, dann Spec und Adapter. `api.gen.go` nur per `go generate ./...`. Spec-Texte englisch. 100 % Abdeckung in `publicapi/v1` bleibt.

**Never:** Keine Änderung am Kern (`ListedEvent.Archived`, `archivedEvent`, `ListEvents`), an Admin-Oberfläche, `archived_at`, `MarkArchived`, Migrationen, SQL/sqlc. Keine Änderung an Pfaden, Parametern, Enum-Codes oder anderen Feldern. Abgeschlossene Story-Specs, Retros und Planungsartefakte bleiben unverändert.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Aktive Liste | `GET /v1/events` mit aktivem Event | Event ohne Schlüssel `archived`, sonst unveränderte Leseform | N/A |
| Archiv | `GET /v1/archive/events` mit vergangenem Event | Event ohne Schlüssel `archived`, gleiche Form wie aktiv | N/A |
| Partition | feste `Clock`, gemischter Bestand | jedes Event ohne „prüfen“ in genau einer Liste | N/A |

</frozen-after-approval>

## Code Map

- `api/v1/openapi.yaml:96-99` -- „Past events“ im „Time model“: Satz „`archived` says which of the two lists …“ ersetzen durch: Die Liste, in der ein Event steht, sagt, ob es vorbei ist; Abnehmer, die Events zwischenspeichern, vergleichen `effectiveEnd` mit der aktuellen Zeit (ein eigenes Feld gibt es nicht).
- `api/v1/openapi.yaml:260`, `:366` -- `archived: false`/`true` aus den Beispielen.
- `api/v1/openapi.yaml:270-278` -- Beschreibung `/archive/events`: „with `archived` set to `true`“ streichen; denselben Hinweis zu `effectiveEnd` ergänzen.
- `api/v1/openapi.yaml:662`, `:680`, `:760-767` -- Schema `Event`: `archived` aus Beschreibung, `required`, `properties`.
- `internal/adapter/publicapi/v1/api.gen.go` -- per `go generate ./...` neu; Feld `Archived` fällt weg.
- `internal/adapter/publicapi/v1/events.go:153` -- Zeile `Archived: event.Archived` entfällt.
- `internal/adapter/publicapi/v1/events_test.go:424`, `:475`, `:491` -- Schlüsselmenge und erwartetes JSON ohne `archived`; `TestListEventsMarksArchivedAsTheCoreSays` (`:502`) ersetzen durch einen Test, dass auch ein `ListedEvent` mit `Archived: true` ohne Schlüssel `archived` ausgeliefert wird.
- `internal/adapter/publicapi/v1/archive_test.go:152-166` -- Test umbenennen (z. B. `…DeliversTheReadForm`), Schlüsselmenge ohne `archived`, Prüfung `archived == true` streichen.
- `cmd/eventstore/acceptance_test.go:68` -- `"archived": false` aus dem erwarteten Event.
- Nicht anfassen: `internal/core/event_query.go` (`Archived`, `archivedEvent`), `internal/adapter/admin/*`, `README.md` (zeigt `archived` schon nicht mehr; `archived_at` in Zeile 235 bleibt).

## Nahtstellen

- `publicapi/v1` Abbildung `eventOf` und Leseform-Tests -- 2.3 -- alle übrigen Felder, `null`-Werte und Formate unverändert, nur `archived` fällt weg -- `TestListEventsDeliversExactlyTheReadForm`, `…FormatsDatesTimesAndInstants`, `…DeliversEmptyValuesAsNull`, `…GivesOutNoInternalValues`.
- Archiv-Handler und Partition -- 2.4 -- gleiche Form wie aktiv, jedes Event in genau einer Liste -- `archive_test.go` Leseform-Test und `TestEveryEventIsInExactlyOneOfEventsAndArchive` (unverändert grün), Kern `TestEveryEventIsInExactlyOneOfActiveAndArchive` (unverändert).
- API-Vergleich vor/nach Bereinigung -- 2.5 -- `cmd/eventstore/cleanup_test.go` `TestCleanupLeavesThePublicAnswersUnchanged` unverändert grün.
- „Time model“ in der Spec -- 2.6, 2.7 -- Abschnitt „Events awaiting correction“ und Ausnahme in `/archive/events` bleiben -- Spec-Review, `go generate`-Diff.
- Abnahme SM-1 und Testsammlung in `cmd/eventstore` -- 1.x/2.6, 3.5 -- `acceptance_test.go` ohne `archived`; `testcollection_test.go` liest nur Startdaten, bleibt unverändert grün.
- Admin-Liste „archiviert“ -- 1.7 -- unverändert, Kern liefert `Archived` weiter -- `internal/adapter/admin/events_test.go`.

## Tasks & Acceptance

**Execution:**
- [x] `internal/adapter/publicapi/v1/events_test.go`, `archive_test.go`, `cmd/eventstore/acceptance_test.go` -- Erwartungen ohne `archived` (rot) -- test-first.
- [x] `api/v1/openapi.yaml` -- Feld, Beispiele und Texte wie in der Code Map; dann `go generate ./...`.
- [x] `internal/adapter/publicapi/v1/events.go` -- Abbildung von `Archived` entfernen (grün).

**Acceptance Criteria:**
- Given die Spec, when ich das Schema `Event` lese, then enthält es `archived` weder in `properties` noch in `required`, die Beispiele beider Endpunkte enthalten es nicht, und „Time model“ sowie `/v1/archive/events` erklären die Liste als Antwort auf „vorbei“ und den Abgleich von `effectiveEnd` beim Zwischenspeichern.
- Given `grep -n archived api/v1/openapi.yaml`, when ausgeführt, then trifft es nur Fließtext („archived events“), kein Feld.

## Implementation Notes

## Spec Change Log

## Review Triage Log

| # | Layer | Befund | Verdict | Evidenz | Route |
|---|-------|--------|---------|---------|-------|
| 1 | blind/edge | Abschnitt „Versioning“ der Spec nennt die Ausnahme nicht, `info.version` bleibt 1.0.0 | low | Vertrag widerspricht sich selbst; ein Satz behebt das. Versionssprung nicht nötig, die Ausnahme ist dokumentiert und v1 hat keinen Abnehmer | patch |
| 2 | blind | Caching-Satz doppelt in „Time model“ und `/archive/events` | low | wörtliche Kopie, Verweis wie bei „awaiting correction“ ist direkt | patch |
| 3 | blind/edge | `last_updated` in `sprint-status.yaml` läuft rückwärts (16:00 → 15:58) | low | Diff zeigt es; direkte Korrektur | patch |
| 4 | blind | Caching-Hinweis ohne Richtung der Grenze | false | derselbe Absatz sagt „over … as soon as `effectiveEnd` is reached“ | reject |
| 5 | blind | Caching-Hinweis nennt nicht, dass Events aus beiden Listen fallen können | low | „Events awaiting correction“ steht direkt darunter; Löschen ist allgemeine Cache-Invalidierung | reject |
| 6 | blind | kein End-to-End-Test der Schlüsselmenge im Archiv | false | einziger Weg zur Leseform ist `eventsOf`/`eventOf`, Archiv-Test prüft die Schlüsselmenge | reject |
| 7 | blind | Spec-Abschnitte leer, Code-Map-Zeilen veraltet | low | Behebung ändert diese Spec | reject |
| 8 | blind | `epic-2-context.md` neu kompiliert, verliert Details, „2.8 noch offen“ | low | Cache-Datei, laut Kopf frei editierbar und bei Planungsänderung neu erzeugt; Epic 2 endet mit dieser Story | reject |
| 9 | blind/edge | Archiv-Test prüft nichts Archivspezifisches mehr; neuer Test redundant | false | Auswahl gedeckt durch `archivedCalls` (`archive_test.go:46`) und Partitionstests; neuer Test deckt Eingabe `Archived: true` ab | reject |
| 10 | seam | Beispiele der Spec werden nicht gegen das Schema `Event` geprüft | low | besteht seit Story 2.1, `contract_test.go` prüft keine Beispiele; heute korrekt | defer |
| 11 | vg | keine Lücken | false | Schlüsselmengen exakt geprüft, Abnahme läuft in CI mit DB | reject |

## Verification

**Commands:**
- `go generate ./...`; `git status --porcelain` -- nur erwartete Dateien
- `CI= go test ./...` -- grün
- `bash scripts/check-coverage.sh` -- 100 %
- `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./...` -- ohne Befund
- `EVENTSTORE_TEST_DATABASE_URL=postgres://eventstore:eventstore@localhost:5432/eventstore_test?sslmode=disable go test -p 1 ./internal/adapter/postgres/... ./cmd/eventstore/...` -- grün
