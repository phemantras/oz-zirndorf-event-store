---
title: 'A1: Fix-PR Epic-1-Nahtstellen'
type: 'bugfix'
created: '2026-10-05'
status: 'done'
baseline_commit: '2d424e09be653eb2dc11b8d32f224d868e03dc15'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-1-retro-2026-10-03.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Die Retro zu Epic 1 fand Fehler an Nahtstellen zwischen Stories und im Betrieb (R1, R2, R3, R4, R6, B1, B2). Ein in der Zwischenzeit gelöschter Ort führt beim Speichern eines Events zu einer 500er-Seite. „Trotzdem speichern“ gilt auch für geänderte Eingaben. `RecomputeDerived` lässt `title_key` leer, wenn die Periode scheitert, und markiert auch bei DB-Fehlern mit „prüfen“. Die Löschrückfrage nennt nach einem Fehler den ungespeicherten Titel bzw. Namen. Dem Server fehlen Timeouts, und zwei Instanzen können gleichzeitig migrieren.

**Approach:** Ein PR, jeder Punkt test-first. Der Kern übersetzt `ErrConflict` beim Schreiben eines Events in den Feldfehler „Ort nicht gefunden“. Die Bestätigung einer Duplikatwarnung trägt einen Fingerabdruck des Duplikatschlüssels aus dem Kern. `RecomputeDerived` trennt Regelfehler (markieren) von Infrastrukturfehlern (abbrechen). Der Admin holt für die Löschrückfrage den gespeicherten Titel bzw. Namen. `http.Server` bekommt Read-, Write- und Idle-Timeout, goose einen Postgres-Session-Locker.

## Boundaries & Constraints

**Always:** Test-first, 100 % für `internal/core` und `internal/adapter/admin`. Typisierte Kernfehler, die Übersetzung in HTTP nur im Adapter. Benannte Konstanten. Deutsche Texte, englische Bezeichner und Logs.

**Never:** Keine Migration, keine Änderung an generiertem Code, keine neue Modulabhängigkeit (`goose/v3/lock` gehört zum vorhandenen Modul). Keine Signaturänderung von `SaveEvent` oder `DuplicatePolicy`. Nicht hier: R5, R7, R8, R9, B3 bis B8, S2 (`name_key`).

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| R1 Ort weg beim Schreiben | Repo `Create`/`Update` liefert `ErrConflict` | `SaveEvent` liefert `*ValidationError{locationId notFound}`, Admin 422 mit Ortsmeldung, Eingaben bleiben | Transaktion rollt zurück |
| R2 bestätigt, unverändert | 409, Admin klickt „Trotzdem speichern“ | gespeichert, 303 | N/A |
| R2 bestätigt, Schlüssel geändert | Nach 409 Titel/Datum/Ort geändert, dann „Trotzdem speichern“ | `RejectDuplicates`: neue Prüfung; neue Kandidaten → 409, sonst gespeichert | N/A |
| R2 alter Wert | Formular sendet `duplicates=allow` | wie ohne Bestätigung | N/A |
| R3 Periode scheitert | Gespeichertes Event mit ungültigen Zeiten, `title_key` veraltet | `title_key` wird neu gespeichert, Periode bleibt, Event „prüfen“, Failure gemeldet | N/A |
| R4 DB-Fehler | `UpdateDerived` liefert Fehler außer `ErrNotFound`, oder Kontext abgebrochen | `RecomputeDerived` liefert Fehler, Start bricht ab, keine Markierung | Fehler gewrappt |
| R4 Event weg | `UpdateDerived` liefert `ErrNotFound` | Event übersprungen, keine Markierung, kein Fehler | N/A |
| R6 Event | 422/409 beim Bearbeiten, Titel geändert | Rückfrage nennt den gespeicherten Titel | gespeichertes Event weg → 404-Seite „nicht gefunden“ |
| R6 Ort | 422/409 beim Speichern oder 409 beim Löschen mit ungespeicherter htmx-Eingabe | Rückfrage nennt den gespeicherten Namen | Ort weg → 404-Seite |
| B1 Lock belegt | Fremde Session hält das goose-Advisory-Lock | `Migrate` wartet und liefert bei abgelaufenem Kontext einen Fehler | Fehler gewrappt |

</frozen-after-approval>

## Code Map

- `internal/core/event_service.go:128-155` -- `saveEvent`/`writeEvent`: `ErrConflict` aus `writeEvent` in `ValidationError` umsetzen; Vorbild `location_service.go:94-97`. Port-Doku `EventRepo.Create/Update` um `ErrConflict` ergänzen.
- `internal/core/event_service.go:324-358` -- `RecomputeDerived`/`recomputeEvent` umbauen; Doku nachziehen.
- `internal/core/duplicates.go` -- `DuplicateConfirmationOf(in EventInput) string` ergänzen; `NormalizeKey` wiederverwenden.
- `internal/adapter/admin/duplicates.go` -- `duplicatePolicyOf(form, values)`, `duplicatesAllow` entfällt; `duplicateWarningOf` bekommt den Fingerabdruck.
- `internal/adapter/admin/events.go:201-305` -- `saveEvent`, `eventForm` (Feld für gespeicherten Titel), `renderEventForm` Confirm-Text.
- `internal/adapter/admin/locations.go:220-300, 343-361` -- `saveLocation`, `renderLocationInUse`, `newLocationFormPage` mit gespeichertem Namen.
- `internal/adapter/admin/templates/event_form.html:78-81` -- Bestätigungsbutton nutzt `ConfirmValue` (bleibt so).
- `cmd/eventstore/recompute.go` -- Doku: Fehler beim Speichern stoppen den Start.
- `cmd/eventstore/main.go:27-34, 127-133` -- Timeout-Konstanten in `newServer`.
- `internal/adapter/postgres/postgres.go:53` -- `goose.WithSessionLocker(lock.NewPostgresSessionLocker(...))`.
- Tests: `internal/core/event_service_test.go`, `duplicates_test.go`, `internal/adapter/admin/events_test.go`, `duplicates_test.go`, `locations_test.go`, `delete_test.go`, `cmd/eventstore/main_test.go`, `recompute_test.go`, `internal/adapter/postgres/postgres_test.go`, `delete_test.go:163-174` (reverse race, bleibt).

## Tasks & Acceptance

**Execution:**
- [x] `internal/core/event_service.go` + Test -- R1: `ErrConflict` beim Schreiben → `ValidationError` mit `EventFieldLocationID`/`ProblemNotFound`.
- [x] `internal/core/event_service.go` + Test -- R3/R4: Periode scheitert → `title_key` (und nur er) bei Abweichung speichern, markieren, Failure. Speicherfehler `ErrNotFound` → überspringen. Andere Speicherfehler oder `ctx.Err()` → sofort Fehler zurückgeben, nicht markieren.
- [x] `internal/core/duplicates.go` + Test -- `DuplicateConfirmationOf`: deterministischer, nicht leerer Hex-SHA-256 über `NormalizeKey(Title)`, `StartDate` und `LocationID` wie eingegeben, eindeutig getrennt (z. B. je `strconv.Quote`).
- [x] `internal/adapter/admin/duplicates.go`, `events.go` + Tests -- R2: Warnung sendet den Fingerabdruck der angezeigten Werte; `AllowDuplicates` nur bei Gleichheit mit dem Fingerabdruck der gesendeten Werte.
- [x] `internal/adapter/admin/events.go`, `locations.go` + Tests -- R6: Bei Fehler-Re-Render mit ID gespeicherten Titel/Namen über `GetEvent`/`GetLocation` holen; `ErrNotFound` → 404-Seite, sonst 500.
- [x] `cmd/eventstore/main.go` + Test -- B2: `ReadTimeout` 15 s, `WriteTimeout` 30 s, `IdleTimeout` 120 s als benannte Konstanten.
- [x] `internal/adapter/postgres/postgres.go` + Test -- B1: Session-Locker mit Standard-Lock-ID; Test hält `pg_advisory_lock(lock.DefaultLockID)` auf eigener Verbindung und erwartet Fehler von `Migrate` mit kurzem Kontext.
- [x] `cmd/eventstore/recompute.go` -- Doku angleichen.

**Acceptance Criteria:**
- Given alle Fixes, when `bash scripts/check-coverage.sh` läuft, then 100 % und `go test ./...` sowie die Postgres-Tests (`-p 1`) sind grün.
- Given `golangci-lint` v2.14.0, when er über `./...` läuft, then keine Befunde.

## Implementation Notes

- Umgesetzt direkt in der Session (keine Subagenten), Branch `fix/a1-epic-1-nahtstellen`.
- B1: goose prüft `HasPending` ohne Lock und nimmt das Lock nur bei offenen Migrationen. Der Test läuft deshalb im leeren Schema von `poolInLegacySchema`; ohne Locker schlägt er fehl, mit Locker ist er grün.
- R6 Orte: `renderLocationInUse` lädt den gespeicherten Ort jetzt immer und übernimmt nur die Werte aus dem htmx-Formular; `locationValuesShownAgain` entfällt. Neuer Typ `locationForm` bündelt id, gespeicherten Namen und Werte.
- R4: Der Fehler beim Speichern nennt die Event-ID (`store derived values of event …`).
- Verifikation: `go test ./...`, Postgres-Tests mit `-p 1`, Coverage-Gate 1342/1342, golangci-lint 0 issues.

## Spec Change Log

## Review Triage Log

| # | Quelle | Befund | Verdikt | Begründung / Route |
| --- | --- | --- | --- | --- |
| 1 | gap | `Update` → `ErrConflict` bei gelöschtem Ort nur mit Fakes geprüft | medium | Kein Postgres-Test für den Update-Pfad. patch: `TestEventRepoUpdateReportsAMissingLocationAsConflict`. |
| 2 | blind | Abbruch in `RecomputeDerived` verwirft gesammelte Regelfehler | low | Real, aber der Start bricht ab, und der nächste Start meldet sie erneut. Fix bräuchte neuen Rückgabe- und Logpfad. reject. |
| 3 | blind | Lock-Test akzeptiert jeden Fehler | low | Direkte Korrektur. patch: prüft `context.DeadlineExceeded`. |
| 4 | blind | Doc „wartet“, goose gibt nach 5 min auf | low | Direkte Doc-Korrektur. patch. |
| 5 | blind | Adapter-Doku zu `ErrConflict` fehlt | low | Direkte Doc-Korrektur. patch in `postgres/events.go`. |
| 6 | blind | Jedes `ErrConflict` gilt als „Ort weg“ | false | Der Port-Vertrag legt `ErrConflict` bei `Create`/`Update` jetzt ausdrücklich auf den fehlenden Ort fest; die Tabelle hat keinen anderen Unique-/FK-Konflikt. |
| 7 | blind | htmx-Pfad von `renderLocationInUse` ohne Test für Ladefehler | false | Der Ladepfad hängt nicht vom Formular ab; beide Zweige sind abgedeckt (Gate 100 %). |
| 8 | blind | Ladefehler beim Event ohne Kontext geloggt | low | Direkte Korrektur. patch: `load event refused for saving`. |
| 9 | blind | `locationFormPageAgain` hat zwei Aufgaben | low | Stil; keine benannte Fehlfolge. reject. |
| 10 | blind | Test-Helfer doppelt, deutscher Text hart kodiert | false | Ausgeschriebene Texte sind Absicht (Kopf von `delete_test.go`); Duplikat ist Testcode ohne Fehlfolge. |
| 11 | blind + edge | `WriteTimeout` bricht Handler nicht ab, `shutdownTimeout` < `writeTimeout` | low | Verhalten von `net/http`; Spec verlangt nur die Server-Timeouts. reject. |
| 12 | blind | Fingerabdruck nicht an Event-ID gebunden | low | Nur per handgebautem Request erreichbar; die Oberfläche sendet die Bestätigung nur aus der gezeigten Warnung. reject. |
| 13 | edge | Neuer Zwilling zwischen Warnung und Bestätigung | low | Ein Admin; Gleichzeitigkeit ist R8 (vor Epic 3). reject. |
| 14 | edge | Advisory-Lock bleibt nach gescheitertem Unlock an Pool-Verbindung | maybe-false | Nur wenn 30 Unlock-Versuche scheitern; würde `low`. reject. |
| 15 | edge | Re-Render nach 422 lädt Event, 404 statt Feldfehler | false | `SaveEvent` liefert für unbekannte oder kaputte IDs `ErrNotFound` vor der Validierung; 404 nur, wenn das Event dazwischen gelöscht wurde, und das ist richtig. |
| 16 | edge | `UpdateDerived` schreibt auch die Periode aus dem Snapshot | low | Wie bisher bei jeder Neuberechnung; Deploy-Überlappung ist R9 (deferred). reject. |

## Design Notes

R2 bleibt beim Adapter-Formular und ändert keine Kern-Signatur: Der Kern definiert, welche Eingaben den Duplikatschlüssel bilden (`DuplicateConfirmationOf`), der Admin vergleicht nur Zeichenketten. Die Rohwerte von Datum und Ort sind strenger als der gespeicherte Schlüssel; im Zweifel erscheint die Warnung erneut, nie fehlt sie.

R4 präzisiert AD-16, wie in der Retro beschlossen: „Neuberechnung schlägt für ein Event fehl“ meint Regelfehler der Periode. Ein Fehler der Datenbank ist kein Fehler des Events und stoppt den Start wie schon ein Fehler beim Lesen der Liste.

## Verification

**Commands:**
- `CI= go test ./...` -- grün
- `EVENTSTORE_TEST_DATABASE_URL=postgres://eventstore:eventstore@localhost:5432/eventstore_test?sslmode=disable go test -p 1 ./internal/adapter/postgres/... ./cmd/eventstore/...` -- grün
- `bash scripts/check-coverage.sh` -- 100 %
- `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./...` -- 0 issues
