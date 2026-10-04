---
title: 'Story 2.3: Events von heute und nach Zeitraum und Typ abfragen'
type: 'feature'
created: '2026-10-04'
status: 'done'
baseline_commit: 'd5806fea531d6149bb3e4b3d04dd3a15a3a9abba'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-2-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Die Public API liefert nur die Event-Typen. Die Karten-App kann weder die Events von heute noch Events eines Zeitraums oder bestimmter Typen abfragen. Die Akzeptanzkriterien der Story 2.3 in `epics.md` gelten vollständig.

**Approach:** Spec erweitert um `GET /events` (`from`, `to`, `type` wiederholbar, 400-Antwort), Leseform `Event` und Schreibform `EventInput` (AD-14). Der Kern normalisiert den Filter zu `[lo, hi)` und liefert über `ListActiveEvents` aktive Events samt Ort, Genauigkeiten und `archived`. Das Repository wendet nur das Prädikat `effective_start < hi AND effective_end > lo` an; der Handler bildet nur ab.

## Boundaries & Constraints

**Always:** Feldnamen, Parameter und Codes nur aus der Spec; Kern-Konstanten für `from`/`to`/`type` spiegeln sie. Zeit nur aus `Clock`, einmal pro Abfrage gelesen; Umrechnung nur über `ToInstant`. Aktiv-Bedingung über das eine Prädikat mit `lo' = max(lo, now)`. Sortierung, Typfilter und Ort-Zuordnung im Kern, nie per SQL. `effective*` mit Berliner Offset. Leere optionale Texte als `null`. Test-first, 100 % für `core` und `publicapi/v1`; generierter Code (oapi-codegen, sqlc) nur per Generator.

**Never:** Kein Archiv-Endpunkt, keine `archived_at`-Spalte, keine Migration (2.4/2.5). Keine Kennung, kein `importKey` in `Event`, kein `locationId`. Kein Einzelabruf, keine Ortsliste, keine Paginierung. Keine Änderung an `core.EventInput` oder Import (Epic 3). Keine Abhängigkeit außer `github.com/oapi-codegen/runtime`.

## I/O & Edge-Case Matrix

Uhr: 2026-12-24 12:00 Europe/Berlin.

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Heute | ohne Parameter | aktive Events mit Überschneidung `[24.12. 00:00, 25.12. 00:00)`; ein heute 14:00 endendes Event ab 14:00 nicht mehr | N/A |
| Mehrtägig | Event 27.11.–24.12., ohne Parameter | enthalten | N/A |
| Datumsbereich | `from=2026-11-29&to=2026-12-24` | `[29.11. 00:00, 25.12. 00:00)`, nur aktive | N/A |
| `to` inklusive | `to=2026-12-24T18:00+01:00`, Event beginnt 18:00 | enthalten (`hi` = 18:01) | N/A |
| Gemischt | `from=2026-12-24T18:00+01:00&to=2026-12-24` | gültig | N/A |
| Nur `from` | `from=2026-12-30` | nach hinten offen | N/A |
| Typen | `type=market&type=club&type=market` | beide Typen, keine Doppelten | N/A |
| Ungültig | `from=24.12.2026`, `to=2026-12-24T18:00` (ohne Offset), `type=foo`, `type=` | 400 Problem, `detail` nennt `from`/`to`/`type` | kein Repository-Aufruf |
| Leeres Intervall | `from=2026-12-28&to=2026-12-27` | 400, `detail` nennt `from` und `to` | N/A |
| Vor heute | nur `to=2026-12-23` | 400, `detail`: nur aktive Events, Verweis auf `/v1/archive/events` | N/A |
| Repository-Fehler | Port liefert Fehler | 500 Problem, geloggt | N/A |

**Entscheidungen ohne Rückfrage (2026-10-04):** Zeitpunkt `YYYY-MM-DDTHH:MM[:SS[.f]]` mit `Z` oder `±HH:MM`. Liegen `from` und `to` in der Vergangenheit, kommt kein 400; geliefert werden nur Events, die noch laufen und den Zeitraum überschneiden (Entscheidung des Menschen, 2026-10-04). Leere Adressteile alter Orte (vor 1.12), leere Notizen und leere Quell-URL als `null`. `importKey` in `EventInput` optional.

</frozen-after-approval>

## Code Map

- `internal/core/event_service.go` -- `EventRepo` um `ListOverlapping(ctx, Overlap) ([]Event, error)` erweitern (Events samt Ablaufplan); `locationsByID`, `sortedTimetable`, `frozenClock` wiederverwenden. `ListEvents`/`compareListEntries` (Admin) nicht ändern.
- `internal/core/` neu `event_query.go` -- `EventFilter{From, To string; Types []string}`, `Overlap{Lo time.Time; Hi *time.Time}` (Hi nil = offen), `ListedEvent{Event; Location; StartPrecision, EndPrecision TimePrecision; Archived bool}`, Konstanten `FilterFieldFrom/To/Type`, `(*EventService).ListActiveEvents(ctx, Clock, EventFilter) ([]ListedEvent, error)`; Normalisierung mit `parseLocalDate`, `ToInstant`, `LocalDate.NextDay`, `dateOf` (in Berlin). Fehler `*ValidationError` mit `FieldError`; neue Problems in `errors.go`: `ProblemEmptyPeriod` (Feld `to`), `ProblemBeforeToday` (Feld `to`). Ergebnis-`Period` per `.In(berlin)`.
- `internal/core/event.go`, `location.go` -- Kommentare „will define … Story 2.2“ an die Spec anpassen; `EventFieldLocationID` als Admin-intern kennzeichnen.
- `internal/adapter/postgres/queries/events.sql` -- `ListEventsOverlapping` (`WHERE effective_end > @lo AND (sqlc.narg(hi)::timestamptz IS NULL OR effective_start < sqlc.narg(hi))`), `ListTimetableEntriesOfEvents` (`event_id = ANY(@event_ids::uuid[])`); `db/` per sqlc 1.31.1 neu erzeugen (kein gcc: Release-Binary `sqlc_1.31.1_windows_amd64` in den Scratchpad).
- `internal/adapter/postgres/events.go` -- `ListOverlapping` mit `eventFromRow`/`timetableEntryFromRow`.
- `api/v1/openapi.yaml` -- `info`: nur Listen, keine Kennungen; Pfad `/events`; Schemas `Event`, `EventList`, `EventLocation` (`name` mit Eindeutigkeits-/Gruppierungshinweis), `Address`, `Source`, `TimetableEntry`, `EventInput`, `EventInputLocation`; nullbare Felder als `type: [string, 'null']` (v2.8.0 erzeugt `*string`, geprüft); `components/responses/BadRequest`.
- `internal/adapter/publicapi/v1/` -- `api.gen.go` neu erzeugen (`go generate`); `server.go` bekommt `events`-Port und `clock`; `handler.go` `Config{Logger, Events EventLister, Clock core.Clock}`, `ErrorHandlerFunc` → 400 Problem mit Parametername; `problem.go` Details für 400; neu `events.go` (Abbildung `ListedEvent` → `Event`, `*ValidationError` → 400).
- `cmd/eventstore/main.go:99` -- `publicapi.Config` um `Events: cases.events`, `Clock: systemClock{}` ergänzen; `health_test.go` Router-Test.

## Tasks & Acceptance

**Execution:**
- [x] `internal/core/event_query_test.go`, `event_query.go`, `errors.go` -- Test zuerst mit fester `Clock` und Fake-Repo: jede Zeile der I/O-Matrix (außer HTTP), Sortierung `effectiveStart` auf, Gleichstand `ID`, zwei Events am selben Ort mit identischem Ort, Ablaufplan sortiert, `lo' = max(lo, now)` an das Repo übergeben, Repo-/Ort-Fehler weitergereicht.
- [x] `internal/adapter/postgres/queries/events.sql`, `db/*`, `events.go`, `events_test.go` -- Postgres-Test zuerst: Grenzfälle `effective_start = hi` (nicht enthalten), `effective_end = lo` (nicht enthalten), offenes `hi`, Ablaufplan nur der Treffer.
- [x] `api/v1/openapi.yaml` -- Pfad, Parameter (jeweils englisch beschrieben, mit Beispielen), Schemas, 400-Beispiel; `go generate ./...`, `go mod tidy`.
- [x] `internal/adapter/publicapi/v1/*_test.go` -- Test zuerst je HTTP-Zeile der Matrix; Leseform-Test: JSON-Schlüssel jeder Ebene (Event, `location`, `address`, `source`, `timetable`-Eintrag) exakt gleich der erwarteten Menge; Formate `HH:MM`/`null`, `YYYY-MM-DD`, `+01:00`.
- [x] `internal/adapter/publicapi/v1/server.go`, `events.go`, `handler.go`, `problem.go` -- implementieren.
- [x] `cmd/eventstore/main.go`, `health_test.go` -- verdrahten, Router-Test `GET /v1/events`.
- [x] `README.md` -- `/v1/events` mit Parametern ergänzen.

**Acceptance Criteria:**
- Given zwei Events am selben Ort, when die Koordinaten des Orts im Admin geändert werden, then liefern beide in `GET /v1/events` die neuen Koordinaten.
- Given eine Antwort von `GET /v1/events`, when ihr JSON geprüft wird, then enthält sie keine Schlüssel `id`, `locationId`, `importKey`, `archivedAt`, `titleKey`, `nameKey` auf keiner Ebene.
- Given der Postgres-Test, when er gegen PostgreSQL 18 läuft, then bestätigt er das Prädikat an beiden Grenzen.

## Implementation Notes

- Vergangener Zeitraum: Das eine Prädikat mit `lo' = max(lo, now)` liefert bei `from`/`to` in der Vergangenheit keine beendeten Events, wohl aber Events, die noch laufen und den Zeitraum überschneiden (z. B. Weihnachtsmarkt 27.11.–24.12. bei `from=2026-12-01&to=2026-12-02`, Uhr 24.12.). Das ist „Überschneidung UND aktiv“; die Entscheidung „leere Liste, kein 400“ gilt damit für alle bereits beendeten Events. Ebenso bei nur `to` früher am heutigen Tag. Tests belegen beides.
- `endPrecision` ist `null`, wenn das Event kein Enddatum hat (Kern liefert dann `""`). Im Schema als `anyOf: [TimePrecision, null]` (oapi-codegen erzeugt `*TimePrecision`).
- `Address`, `Source` und `TimetableEntry` sind in `Event` und `EventInput` dieselben Schemas; ihre Schlüssel sind Pflicht, die Werte nullbar, damit die Leseform jeden Schlüssel immer liefert.
- Ein mehrfach angegebener `from`/`to` scheitert schon beim Binden im generierten Server und wird über `ErrorHandlerFunc` zu 400 mit Parametername.
- `github.com/oapi-codegen/runtime` v1.7.0 (aktuell, nicht im Spine gepinnt) kam für die Parameterbindung und `openapi_types.Date` hinzu; indirekt `apapsch/go-jsonmerge/v2` und `google/uuid`.
- Postgres-Tests (`ListOverlapping`-Grenzen, Router mit echter DB) konnten lokal nicht laufen (kein Docker); sie laufen im CI-Job „PostgreSQL tests“.

## Spec Change Log

## Review Triage Log

| # | Layer | Befund | Verdict | Evidenz | Route |
|---|-------|--------|---------|---------|-------|
| 1 | edge/blind | `from`/`to` in der Vergangenheit: `lo'` > `hi`, 200 statt 400; Asymmetrie zu „nur `to` vor heute“ | false | Genau so entschieden (Mensch, 2026-10-04): kein 400, laufende Events mit Überschneidung; „nur `to` vor heute“ → 400 verlangt das AC | reject |
| 2 | edge/vg | Leeres `from=`/`to=` gilt still als fehlend (heute bzw. offen), `type=` dagegen 400 | medium | `textOf` und `cmp.Or`/`toText != ""` machen nil und `""` gleich; AC: ungültiges Datum → 400 | patch |
| 3 | edge/blind | `NewHandler` ohne `Events`/`Clock` panikt erst beim Aufruf | low | Einziger Aufrufer `main.go` setzt beide; Fix bräuchte Guards | reject |
| 4 | edge/blind | 400-Detail behauptet Wiederholung bei jedem `InvalidParamFormatError`; Fallback ohne Parametername | false | Für die String-Parameter `from`/`to` ist „multiple values“ der einzige Bindefehler (`runtime/bindparam.go`); `type` bindet Strings ohne Formatfehler, Fallback unerreichbar | reject |
| 5 | edge | `ValidationError` ohne Felder → leeres Detail | false | `normalizeActiveFilter` erzeugt `ValidationError` nur mit mindestens einem Feld | reject |
| 6 | blind | `+` im Offset kommt unkodiert als Leerzeichen an → 400; Doku zeigt nacktes `+` | medium | `url.Query` dekodiert `+` als Leerzeichen; Spec, KON-5, README zeigen `+01:00` ohne Hinweis | patch |
| 7 | blind | Kein Index für `effective_end`/`effective_start` | low | Hobby-Datenbestand, Index bräuchte Migration (in dieser Story ausgeschlossen) | reject |
| 8 | blind | `IS NULL OR` verhindert Indexnutzung im generischen Plan | low | Ohne Index (Nr. 7) folgenlos | reject |
| 9 | blind | Typfilter in Go statt SQL | false | Bewusste Spec-Entscheidung (keine Fachlogik in SQL), kein Fehlverhalten | reject |
| 10 | blind | Alle Orte pro Anfrage geladen | low | Wenige Orte, Fix bräuchte neue Repo-Methode | reject |
| 11 | blind | Ablaufplan-Abfrage auch bei leerem Ergebnis | low | Eine leere `ANY`-Abfrage, Fix wäre zusätzlicher Zweig | reject |
| 12 | blind | Überschneidungsregel nur als Kopien in Fakes; Admin-Fake ignoriert `Overlap` | low | Nur Testcode, Postgres-Grenztest prüft die echte Regel | reject |
| 13 | blind | `type`-Fehler nennt den falschen Wert nicht | low | Detail nennt Parameter und gültige Codes (AC erfüllt); Fix ändert Fehlerstruktur | reject |
| 14 | blind | Keine Tests für Datumsfilter an Umstellungstagen | low | `parseFilterDate` nutzt `ToInstant`/`NextDay`, aber kein Filtertest pinnt 23/25-Stunden-Tage; Test ist direkte Ergänzung | patch |
| 15 | blind | `from`/`to` ohne `pattern` in der Spec | low | Prosa beschreibt die Formen, generierter Server validiert Muster nicht | reject |
| 16 | blind | CORS auf 400 nicht getestet | false | `readOnlyCORS` setzt den Header vor jedem Routing, unabhängig vom Status | reject |
| 17 | vg | Berliner Umrechnung in `listedEventOf` von keinem Test gepinnt | medium | Alle Fixtures schon in Berlin; DB liefert andere Zone, Entfernen von `.In(berlin)` bleibt grün | patch |
| 18 | vg | „Heute“ bei Uhr in fremder Zone nicht gepinnt | medium | Alle Test-Uhren in Berlin; `systemClock` liefert Prozesszone | patch |

## Verification

**Commands:**
- `sqlc generate` (1.31.1) und `go generate ./... && git status --porcelain` -- zweiter Lauf ohne Diff
- `CI= go test ./...` -- grün, inkl. Archtest und Enum-Abgleich
- `bash scripts/check-coverage.sh` -- 100 %
- `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./...` -- ohne Befund
- Postgres-Tests mit `-p 1` für `internal/adapter/postgres/...` und `cmd/eventstore/...` -- grün
