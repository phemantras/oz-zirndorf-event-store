---
title: 'Story 1.9: Ablaufplan pflegen'
type: 'feature'
created: '2026-10-03'
status: 'done'
baseline_commit: '40bead59410e1b65ebcc152ae47cd82c0445a60a'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-1-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Ein Event kann noch keinen Ablaufplan haben, ein Festprogramm ist nicht abbildbar (FR-4). Die Akzeptanzkriterien der Story 1.9 in `epics.md` gelten vollständig.

**Approach:** Wertobjekt `TimetableEntry` im Kern, validiert und gegen den effektiven Zeitraum geprüft in `newEvent`; `SaveEvent` schreibt Event und ganze Liste in einer Transaktion über den neuen Port `TxRunner`. Postgres: Migration `00005` mit `timetable_entries`. Admin: Programmpunkte im Event-Formular per htmx hinzufügen/entfernen, Fehler am Punkt.

## Boundaries & Constraints

**Always:** Zeiten nur über `ToInstant`. Punkt mit `endTime < startTime` endet am Folgetag (ENT-3). Grenze nach Spine (AD-15/ENT-20, maßgeblich): Beginn des Punkts in `[effectiveStart, effectiveEnd)`, Ende in `(effectiveStart, effectiveEnd]`; Punkt ohne Uhrzeit: sein Tag `[00:00, 00:00 Folgetag)` überschneidet `[effectiveStart, effectiveEnd)`. Fehler je Punkt mit Index der Eingabereihenfolge, Feldname `timetable[i].description|date|startTime|endTime`. `Canonicalize` normalisiert und sortiert (Datum, ohne Uhrzeit zuerst, `startTime`, `endTime`, Beschreibung); `GetEvent`/`ListEvents` liefern sortiert (zusätzlich nach `id`). Ablaufplan ändert `effective*` nicht. Test-first, 100 % Abdeckung für Kern und Admin.

**Never:** Keine Teil-Updates von Punkten, kein Schreibweg am Kern vorbei, keine Zeitlogik in SQL, angewendete Migrationen unverändert. `SaveLocation` und `RecomputeDerived` bleiben ohne `TxRunner` (Deferred). Kein CDN.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Gültig | Event 16.10. 18:00–17.10., Punkte „Bieranstich“ 16.10. 18:00, „Disco“ 16.10. 22:00–01:00 | gespeichert, Disco endet 17.10. 01:00, sortiert ausgeliefert | N/A |
| Fehlt | Punkt ohne Beschreibung oder Datum | ganzes Speichern abgelehnt, 422 | Meldung am Punkt |
| Außerhalb | Punkt 18.10. bei Event bis 17.10.; oder Event-Ende verkürzt | abgelehnt | „liegt außerhalb des Event-Zeitraums“ am Punkt |
| Endet mit Event | Event bis 23:00, Punkt 22:00–23:00 | gültig | N/A |
| Lücke | Punkt 29.03.2026 02:30 | abgelehnt | Meldung Sommerzeit am Punkt |
| Gleich | `startTime == endTime` | abgelehnt | `notAfterStart` an `endTime` |
| Leere Zeile | alle vier Felder leer | Admin verwirft sie, kein Fehler | N/A |
| Ersetzen | Bearbeiten mit 1 statt 3 Punkten | genau 1 Punkt gespeichert | Fehler beim Schreiben → nichts gespeichert (Rollback) |

**Entscheidungen ohne Rückfrage (2026-10-03):** Grenze wie oben (Spine vor Story-Text „in `[…)`“); leere Zeilen verwirft der Admin; Entfernen per `hx-on:click` ohne Serveranfrage; nur `SaveEvent` bekommt in dieser Story `TxRunner`.

</frozen-after-approval>

## Code Map

- `internal/core/event.go` -- `EventInput`/`Event` um `Timetable` erweitern; `Canonicalize`, `EventInputOf`, `newEvent` (Prüfung nach `parseEventTimes`, nur wenn Zeitraum gültig); `parseLocalDate`/`parseLocalTime` wiederverwenden.
- neu `internal/core/timetable.go` -- `TimetableEntry{ID, Description, Date, StartTime, EndTime *LocalTime}`, `TimetableEntryInput` (Texte), Feldkonstanten, `TimetableField(i, name)`/`SplitTimetableField`, Sortierung, Grenzprüfung; neues `ProblemOutsideEvent` in `errors.go`.
- `internal/core/event_service.go` -- `TxRunner`-Port + `Repos{Events, Locations}`; `NewEventService(tx, events, locations)`; `SaveEvent` = Hülle über `InTx`, transaktionsgebundener Kern nutzt nur `Repos`. `EventRepo.Create/Update` speichern `Timetable` mit (ersetzen).
- `internal/core/*_test.go` -- `fakeEventRepo` speichert Timetable; Fake-`TxRunner`, der Fehler durchreicht.
- `internal/adapter/postgres/migrations/00005_timetable_entries.sql` -- neu; `id uuid DEFAULT uuidv7()`, `event_id` FK `ON DELETE CASCADE` + Index, `description`, `date`, `start_time`, `end_time`.
- `internal/adapter/postgres/queries/events.sql` + `sqlc generate` (v1.31.1) -- Einträge listen (alle / je Event), je Event löschen, einfügen.
- `internal/adapter/postgres/events.go` -- Get/List laden Einträge (List mit einer Abfrage, gruppiert); Create/Update schreiben sie; `dateParam`/`timeParam` wiederverwenden. Neu `tx.go`: `TxRunner` mit `pgx.BeginFunc`.
- `cmd/eventstore/main.go` -- `postgres.NewTxRunner(pool)` verdrahten.
- `internal/adapter/admin/events.go`, `templates/event_form.html`, neues Fragment `templates/timetable_entry.html`, `handler.go`, `templates.go` -- Felder `timetable.description|date|startTime|endTime` als parallele Listen lesen (ungleiche Länge → 400); Route `GET /admin/events/timetable-entry` liefert leere Zeile (`hx-swap="beforeend"`); Labels umschließen Inputs (keine IDs nötig); `maxEventFormBytes` auf 64 KiB; Meldungen je Punkt über `SplitTimetableField`. Muster aus `new_location.go`/`renderFragment` nutzen.
- `internal/adapter/admin/handler_test.go`, `postgres/events_test.go` -- Konstruktor-Aufrufe anpassen.

## Tasks & Acceptance

**Execution:**
- [x] `internal/core/timetable.go`, `event.go`, `errors.go` (+Tests) -- Typen, Parsing, ENT-3, Grenze, Sortierung, Feldpfade; Matrix-Fälle mit fester Zeit testen.
- [x] `internal/core/event_service.go` (+Tests) -- `TxRunner`, `Repos`, `SaveEvent` in Transaktion, sortiertes Lesen.
- [x] `internal/adapter/postgres/...` (+Tests gegen PG 18) -- Migration, Queries, generierter Code, Repo, `TxRunner` inkl. Rollback-Test und Löschweitergabe.
- [x] `cmd/eventstore/main.go` (+Tests) -- Verdrahtung.
- [x] `internal/adapter/admin/...` (+Tests) -- Formular, Fragment-Route, Lesen, Meldungen je Punkt, Bearbeiten zeigt gespeicherte Punkte.

**Acceptance Criteria:**
- Given ein Event mit Punkten, when ich es ohne Änderung erneut speichere, then bleiben `effective*` und die Punkte gleich.
- Given `bash scripts/check-coverage.sh`, golangci-lint, `CI= go test ./...`, Postgres-Tests mit `-p 1` und `sqlc generate` ohne Diff, when sie laufen, then grün und 100 %.

## Implementation Notes

- Kern: `EventInput.normalized()` normalisiert ohne Sortieren, damit Fehler den Index der Eingabe tragen; `Canonicalize` sortiert zusätzlich. `newEvent` prüft die Grenzen nur bei gültigem Zeitraum. Ein Punkt mit fehlender Beschreibung oder fehlerhaftem Feld wird nicht zusätzlich auf Grenzen geprüft. „Außerhalb“ wird am Feld gemeldet, das herausfällt (`startTime`, sonst `endTime`, ohne Uhrzeit `date`).
- Admin: Meldung „Bitte die markierten Programmpunkte prüfen.“ über dem Ablaufplan, Hinweis zu ENT-3 an jedem Punkt; Formular-Limit 64 KiB.
- Postgres: Einträge in `queries/events.sql`; `TxRunner` in `postgres/tx.go` (`pgx.BeginFunc`), Fehler von `fn` unverändert. Test-Truncate um `timetable_entries` ergänzt. Neuer Verdrahtungstest `cmd/eventstore/timetable_test.go`.
- Lokal verifiziert (2026-10-03): `CI= go test ./...`, Coverage-Gate 934/934, `go vet`, golangci-lint v2.14.0 ohne Befund, `sqlc generate` (1.31.1) reproduzierbar. Postgres-Tests lokal nicht ausgeführt (keine Test-Datenbank erreichbar), nur kompiliert; laufen in der CI.
- Matrix-Audit (2026-10-03): alle Zeilen haben Tests; „Ersetzen/Rollback“ prüfen nur `postgres/timetable_test.go`, lokal nicht gelaufen. Nachgeholt (2026-10-03): `go test -p 1 ./internal/adapter/postgres/... ./cmd/eventstore/...` gegen lokales PostgreSQL 18 (`eventstore_test`) grün.

## Spec Change Log

## Review Triage Log

| # | Quelle | Befund | Verdikt | Evidenz | Route |
|---|--------|--------|---------|---------|-------|
| 1 | gap | Kein Test, dass 422 nur mit Event-Feldfehlern kein Ablaufplan-Banner zeigt | low | `splitTimetableProblems` liefert immer eine nicht-nil Map; nur `len > 0` schützt, ungetestet | patch |
| 2 | blind | Doc-Kommentar von `ListEvents` falsch umbrochen | low | Eine überlange Zeile vor den alten kurzen Zeilen | patch |
| 3 | blind | `RecomputeDerived` prüft Ablaufplan-Grenzen nicht neu | low | Nur bei Regeländerung relevant; Abhilfe bräuchte neue Prüfpfade; Intent: Ablaufplan ändert `effective*` nicht | reject |
| 4 | blind + edge | Eintrags-IDs ändern sich bei jedem Speichern | low | AD-15: Wertobjekt, immer ganze Liste ersetzt; IDs sind nur Ordnungskriterium, keine Identität | reject |
| 5 | blind + edge | Get/List lesen Event und Einträge in zwei Statements ohne gemeinsamen Snapshot | low | Ein Admin, gleichzeitiges Speichern während Lesen selten; Abhilfe ist Lese-Transaktion/Join | reject |
| 6 | blind | `List` lädt alle Einträge inkl. Archiv | low | Hobby-Betrieb, kleine Datenmenge; spätere API braucht sie | reject |
| 7 | blind | `storedEventID` lädt Einträge unnötig mit | low | Eine zusätzliche Abfrage je Speichern | reject |
| 8 | blind + edge | Keine Obergrenze für Anzahl/Länge im Kern | low | Admin durch 64 KiB begrenzt; Import (Epic 3) bringt eigene Grenzen; Abhilfe neue Regel | reject |
| 9 | blind | Status Spec `in-review` vs. Sprint `in-progress` | false | Sprint-Status folgt dem Workflow erst am Ende | reject |
| 10 | blind | Postgres-Task abgehakt, Tests lokal nicht gelaufen | low | Entscheidung des Menschen: PR-CI prüft; Fix wäre Spec-Änderung | reject |
| 11 | blind | Einträge ohne `aria-describedby`/`aria-invalid`, gleiche Legenden | low | Bräuchte IDs je Eintrag; bestehendes Muster (1.8 Triage #5) | reject |
| 12 | blind | Kein Fokus-Management beim Hinzufügen/Entfernen | low | Wie 1.8 Triage #5; JS-Zusatz nötig | reject |
| 13 | blind | `SplitTimetableField` akzeptiert `+3`/`007` | low | Einziger Erzeuger ist `TimetableField`; nicht erreichbar | reject |
| 14 | blind | Verdrahtungstest prüft nur Anzahl | low | Inhalte und Rollback decken `postgres/timetable_test.go` ab | reject |
| 15 | edge | Kein Hinweis auf doppelte Herbststunde bei Einträgen | low | Seltener Randfall; Abhilfe neue Problemart | reject |
| 16 | edge | Unbekannter Feldname im Eintrag zeigt nur Banner | false | Kern meldet nur die vier bekannten Feldnamen | reject |

## Design Notes

```go
type TxRunner interface {
	// InTx runs fn in one transaction; an error from fn rolls back.
	InTx(ctx context.Context, fn func(Repos) error) error
}
```

Fehler-Mapping im Admin: `SplitTimetableField("timetable[2].date")` → `(2, "date", true)`; Meldung aus `timetableFieldMessages[{Field: "date", Problem}]`, Ablage in `Entries[2].Errors`. Vor dem Deploy `pg_dump` (neue Migration).

## Verification

**Commands:**
- `CI= go test ./...` -- grün
- `go test -p 1 ./internal/adapter/postgres/... ./cmd/eventstore/...` mit `EVENTSTORE_TEST_DATABASE_URL` -- grün
- `bash scripts/check-coverage.sh` -- 100 %
- golangci-lint v2.14.0 -- ohne Befund

**Manual checks (if no CLI):**
- Browser: Punkte hinzufügen/entfernen ohne Neuladen, ungültigen Punkt speichern (Meldung am Punkt, Eingaben erhalten), Event bearbeiten (Punkte sortiert).
