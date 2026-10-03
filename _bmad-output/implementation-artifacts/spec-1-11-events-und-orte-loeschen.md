---
title: 'Story 1.11: Events und Orte löschen'
type: 'feature'
created: '2026-10-03'
status: 'done'
baseline_commit: 'e2e506364d75de32c42ca6fa1cc3ec8f8ae0c991'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-1-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Der Admin kann weder Events noch Orte löschen; FR-7 und AD-6 verlangen `DeleteEvent` und `DeleteLocation` als Kern-Anwendungsfälle mit Löschschutz für Orte. Die Akzeptanzkriterien der Story 1.11 in `epics.md` gelten vollständig.

**Approach:** Der Kern bekommt `DeleteEvent` (Event samt Ablaufplan, entfernt die „prüfen“-Markierung) und `DeleteLocation` (zählt erst die Events des Orts, auch archivierte, und lehnt mit `*LocationInUseError` ab, der `ErrConflict` matcht und die Anzahl trägt). Beide laufen als transaktionsgebundener Kern plus Hülle über `TxRunner`. Der Admin bietet auf den Bearbeitungsseiten „Löschen“ mit Rückfrage und übersetzt die Fehler in deutsche Meldungen.

## Boundaries & Constraints

**Always:** Löschschutz im Kern, Fremdschlüssel `ON DELETE RESTRICT` (Migration `00004`) nur als zweite Sicherung; dessen Verletzung (SQLSTATE `23503`) übersetzt der Repo-Adapter in `ErrConflict`. Ablaufplan geht per `ON DELETE CASCADE` mit. SQL nur `DELETE … WHERE id` und `count(*) … WHERE location_id`. Unbekannte oder nicht parsebare ID → `ErrNotFound`. Löschen nur per POST hinter Session und `CrossOriginProtection`. Test-first, 100 % für Kern und Admin.

**Never:** Keine neue Migration, kein Soft-Delete, kein Löschen aus der Liste heraus ohne Rückfrage, keine Massenlöschung, keine Fehlerseite (500) für „nicht mehr vorhanden“ oder „Ort in Gebrauch“.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Event löschen | aktives oder archiviertes Event mit Ablaufplan, Rückfrage bestätigt | Event und Programmpunkte weg, 303 zur Event-Liste | N/A |
| Event in Prüfung | Event mit „prüfen“-Markierung gelöscht | Markierung entfernt | N/A |
| Ort löschen | Ort ohne Events, bestätigt | Ort weg, 303 zur Ortsliste | N/A |
| Ort in Gebrauch | Ort mit 1 aktivem + 2 archivierten Events | nicht gelöscht, 409 | „… weil noch 3 Events auf ihn verweisen …“ (Singular bei 1) |
| Bereits gelöscht | zweiter Tab/Doppelklick auf gelöschtes Event bzw. Ort | 404, deutsche Seite „… ist nicht mehr vorhanden.“ mit Link zur Liste | kein 500 |
| Rennen | Ort mit 0 gezählten Events, Event kommt parallel dazu | FK scheitert, Ort bleibt | `ErrConflict` → Meldung „wird noch von Events verwendet“ ohne Zahl |
| Direktes SQL | `DELETE FROM locations` auf referenzierten Ort | scheitert am FK | Test in Postgres-Suite |

**Entscheidungen ohne Rückfrage (2026-10-03):** „Löschen“ steht auf den Bearbeitungsseiten, nicht in den Listen. Nach erfolgreichem Löschen Umleitung auf die Liste ohne Erfolgsmeldung. „Nicht mehr vorhanden“ nutzt die bestehende Seite `not_found.html` (404) mit eigener Meldung. „Ort in Gebrauch“ rendert das Ortsformular mit Meldung (`role="alert"`), Eingaben bleiben.

**Entscheidung des Menschen (2026-10-03): Rückfrage per Browser-Dialog (`hx-confirm`).** Ein eigenes Löschformular (`method="post"`, `action` und `hx-post` auf `…/{id}/delete`, `hx-confirm` mit Titel bzw. Name, `hx-target="body"`). Ohne JavaScript wird ohne Rückfrage gelöscht (hingenommen); der Dialog ist nur als Markup getestet. Erfolg: bei htmx `HX-Redirect` auf die Liste, sonst 303. Damit „nicht mehr vorhanden“ (404) auch per htmx angezeigt wird, nimmt `htmx-config` in `layout.html` 404 in die getauschten Codes auf (`404|409|422`).

</frozen-after-approval>

## Code Map

- `internal/core/errors.go` -- neu `LocationInUseError{EventCount int}`, `Unwrap` → `ErrConflict`; Muster `LocationConflictError`.
- `internal/core/event_service.go` -- `EventRepo` um `Delete(ctx, id) error` (`ErrNotFound` bei 0 Zeilen) und `CountByLocation(ctx, locationID) (int, error)`; `DeleteEvent` = Hülle über `s.tx` + `deleteEvent(ctx, repos, id) (string, error)`: `Get` (gespeicherte ID-Schreibweise), dann `Delete`; nach Commit `clearReview(storedID)`.
- `internal/core/location_service.go` -- `LocationRepo.Delete(ctx, id) error` (`ErrNotFound`, FK → `ErrConflict`); `NewLocationService(tx TxRunner, repo LocationRepo)`; `DeleteLocation` = Hülle + `deleteLocation(ctx, repos, id)`: `Get`, `repos.Events.CountByLocation`, >0 → `*LocationInUseError`, sonst `Delete`. `SaveLocation` bleibt ohne Transaktion.
- `internal/core/event_service_test.go`, `location_service_test.go` -- `fakeEventRepo`/`fakeLocationRepo` um `Delete`/`CountByLocation` erweitern; `fakeTx` wiederverwenden.
- `internal/adapter/postgres/queries/events.sql` + `locations.sql`, `sqlc generate` (1.31.1) -- `DeleteEvent :execrows`, `CountEventsByLocation :one`, `DeleteLocation :execrows`.
- `internal/adapter/postgres/events.go`, `locations.go` -- Methoden; `translateError` um `foreignKeyViolation = "23503"` → `ErrConflict`; `parseID` für ungültige IDs.
- `internal/adapter/postgres/*_test.go` -- Löschen mit Ablaufplan, Zählen inkl. archivierter, FK-Test mit direktem SQL, `NewLocationService(postgres.NewTxRunner(pool), repo)`.
- `internal/adapter/admin/events.go`, `locations.go`, `handler.go` -- Interfaces um `DeleteEvent`/`DeleteLocation`; Routen `POST {eventPathPattern}/delete`, `POST {locationPathPattern}/delete`; Fehlerübersetzung wie `saveEvent`/`saveLocation`; HX-Redirect-Muster aus `redirectToLogin`.
- `internal/adapter/admin/templates/event_form.html`, `location_form.html`, `layout.html` -- Löschformular nur bei bestehendem Datensatz (`id` nicht leer), außerhalb des Bearbeitungsformulars und nach dessen Buttons (Enter löst nie Löschen aus); `htmx-config` um 404 (Test `new_location_test.go:114` anpassen).
- `internal/adapter/admin/*_test.go` -- `memoryEventRepo`/`memoryLocationRepo` erweitern; `handler_test.go` übergibt `memoryTx` an `NewLocationService`.
- `cmd/eventstore/main.go`, `main_test.go` -- Verdrahtung mit `TxRunner`; `emptyLocations`/`emptyEvents` um Delete-Methoden (`ErrNotFound`).

## Tasks & Acceptance

**Execution:**
- [x] `internal/core/...` (+Tests) -- Fehlertyp, Ports, `DeleteEvent`, `DeleteLocation`, Review-Markierung; alle Matrix-Kernfälle ohne DB.
- [x] `internal/adapter/postgres/...` (+Tests gegen PG 18) -- Queries, generierter Code, Repos, FK-Übersetzung, direkter SQL-FK-Test.
- [x] `internal/adapter/admin/...` (+Tests) -- Löschformular mit `hx-confirm`, Routen, htmx- und Nicht-htmx-Antworten, Meldungen (Anzahl mit Singular/Plural, „nicht mehr vorhanden“, Rennen), Button nur beim Bearbeiten.
- [x] `cmd/eventstore/...` (+Tests) -- Verdrahtung und Fakes.

**Acceptance Criteria:**
- Given eine Anfrage ohne gültige Session, when sie eine Löschroute aufruft, then wird nichts gelöscht und auf `/admin/login` umgeleitet.
- Given `bash scripts/check-coverage.sh`, golangci-lint v2.14.0, `CI= go test ./...`, Postgres-Tests mit `-p 1` und `sqlc generate` ohne Diff, when sie laufen, then grün und 100 %.

## Implementation Notes

- Abweichung von der Spec (Postgres): `ON DELETE RESTRICT` meldet SQLSTATE `23001` (restrict_violation), nicht `23503`. `translateError`/`isConflict` übersetzen `23505`, `23503` und `23001` in `ErrConflict`; beide FK-Codes sind getestet. Nebenwirkung: Speichern eines Events, dessen Ort gerade gelöscht wurde, liefert jetzt `ErrConflict` statt eines rohen DB-Fehlers (Admin weiterhin 500 für dieses Rennen).
- Admin: Weil `hx-target="body"` die ganze Seite ersetzt, schickt das Löschformular des Orts die ungespeicherten Eingaben mit (`hx-include="#location-form"`); die 409-Seite zeigt sie wieder. Ohne JavaScript zeigt sie den gespeicherten Ort. Gemeinsames Template `delete_form.html`; htmx-Erfolg mit `HX-Redirect` und 204.
- Der Bestätigungstext nimmt Titel bzw. Namen aus dem Formular, nach einem Validierungsfehler also ggf. den ungespeicherten Wert.
- `NewLocationService(tx, repo)`; `main.go` nutzt einen gemeinsamen `TxRunner`.
- Lokal verifiziert (2026-10-03): `CI= go test ./...`, Postgres-Tests mit `-p 1` gegen PG 18, Coverage-Gate 1060/1060, golangci-lint v2.14.0 ohne Befund, `sqlc generate` (1.31.1) ohne Diff.
- Matrix-Audit: alle Zeilen durch Kern-, Admin- und Postgres-Tests abgedeckt; der FK-Test löscht per SQL am Kern vorbei. `hx-confirm`-Dialog und Body-Swap nur als Markup/Antwort getestet, Browserprobe offen.

## Spec Change Log

## Review Triage Log

| # | Quelle | Befund | Verdikt | Evidenz | Route |
|---|--------|--------|---------|---------|-------|
| 1 | blind + edge | Rennen „Ort gelöscht während Event-Speichern“ → FK `23503` → `ErrConflict` → 500 | low | Nur bei gleichzeitigem Commit in zwei Tabs zwischen `storedLocationID` und Insert; der übliche Fall (Ort vorher gelöscht) liefert die Feldmeldung; Abhilfe neuer Zweig in Kern/Admin | reject |
| 2 | blind | `23001` sei toter Code, PG melde RESTRICT als `23503` | false | Implementierung hat es empirisch geprüft; Entfernen von `23001` lässt `TestLocationRepoDeleteReportsTheForeignKeyAsConflict` scheitern (bestätigt durch Verification-Gap-Layer) | reject |
| 3 | blind + edge | Globaler 404-Swap betrifft auch Fragment-Anfragen | low | Kein bestehender htmx-Endpunkt liefert 404, nur Fehlrouting; 404 im `htmx-config` ist Entscheidung im Frozen-Block | reject |
| 4 | blind + edge | Rückfragetext nutzt ungespeicherten bzw. leeren Titel/Namen | low | Nur nach 422/409-Neurendern; betrifft denselben Datensatz, harmlos; Abhilfe gespeicherte Werte durchreichen | reject |
| 5 | blind | Erfolgreiches Löschen wird nicht geloggt | low | Unumkehrbare Aktion ohne Spur; eine Logzeile | patch |
| 6 | blind | Test zum direkten SQL-`DELETE` geht über das Repo und prüft nur `ErrConflict` | medium | Bestünde auch mit `NO ACTION`; Story verlangt direktes SQL | patch: Roh-SQL-Test mit Code `23001` und Constraint-Name |
| 7 | blind | GET-auf-Löschroute-Prüfung akzeptiert jeden Status außer 200/303 | low | Direkte Korrektur | patch: genau 405 |
| 8 | blind | Fehlgeschlagenes htmx-Löschen (500) ohne sichtbare Rückmeldung | low | Nur bei DB-Ausfall; Abhilfe neuer Fehlerpfad im Frontend | reject |
| 9 | blind | `deleteEvent` lädt Event samt Ablaufplan nur für die ID | low | Eine Abfrage mehr bei seltener Admin-Aktion; Muster wie `eventToUpdate` | reject |
| 10 | blind | `LocationInUseError.Error()` „1 events“, schwacher Test | low | Nur Logtext, nie angezeigt | reject |
| 11 | blind | Singular-Meldung enthält die „1“ als Literal neben `singleEvent` | low | Kosmetik, Singular ist per Definition 1 | reject |

## Verification

**Commands:**
- `CI= go test ./...` -- grün
- `go test -p 1 ./internal/adapter/postgres/... ./cmd/eventstore/...` mit `EVENTSTORE_TEST_DATABASE_URL` -- grün
- `bash scripts/check-coverage.sh` -- 100 %

**Manual checks (if no CLI):**
- Browser: Event mit Ablaufplan löschen; Ort mit Events löschen → Meldung mit Anzahl; gelöschten Datensatz im zweiten Tab erneut löschen → „nicht mehr vorhanden“.
