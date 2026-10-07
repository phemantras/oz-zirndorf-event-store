---
title: 'D1: Epic-4-Nahtstellen (Retro Epic 4)'
type: 'bugfix'
created: '2026-10-07'
status: 'done'
route: 'dispatch'
baseline_commit: '5799f080235b70b658b61ae063cf61eaf45a9f0e'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-4-retro-2026-10-07.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Retro Epic 4, D1: `shutdownTimeout` (10 s) liegt unter `requestTimeout` (20 s), ein SIGTERM während eines langsamen Requests endet mit Exit 1 (R-1). Der Admin antwortet bei abgelaufener Deadline mit englischem Klartext-500 statt 503 und deutscher Meldung (S-1, P-2). Der Admin loggt einen vom Client abgebrochenen Request als `ERROR`, die öffentliche API als `INFO` (D-1). Ungeklärt ist, ob eine durch die Deadline abgebrochene Abfrage auf dem Datenbankserver weiterläuft (S-3, Deferred-Eintrag aus Spec 4.1).

**Approach:** Test-first: (a) `shutdownTimeout` auf 25 s, ein Test hält `requestTimeout < shutdownTimeout` fest; (b) eine zentrale Fehlerantwort im Admin: Deadline → 503 + `WARN`, Abbruch → 500 + `INFO`, sonst 500 + `ERROR`, jeweils als deutsche Seite im Layout mit Link zurück zur Liste; (c) S-3: `postgres.Connect` setzt `pgconn.CancelRequestContextWatcherHandler` (Cancel-Request sofort, Socket-Frist als benannte Konstante), damit eine abgelaufene Abfrage auch auf dem Server endet; weil pgx dann `PgError 57014` ohne `DeadlineExceeded` liefert, klassifizieren Admin und `publicapi/v1` zusätzlich über `r.Context().Err()`.

**Entscheidungen (2026-10-07, Andreas):** S-3 nach Option A (Handler + Klassifizierung über den Request-Kontext + Postgres-Test per `pg_stat_activity`), Messung gegen Railway entfällt, die Linux-CI misst. Die deutsche Fehlerseite gilt für jeden Admin-Fehler, nicht nur für die Deadline. Volle Spec ohne Split.

## Boundaries & Constraints

**Always:** Test-first, 100 % Abdeckung in `admin` und `publicapi/v1`. Meldungen deutsch in Du-Form wie `msgTooManyLogins`, Log-Meldungen englisch. Die bestehenden `logMsg*`-Konstanten bleiben. Der Import-Commit zeigt seine Meldung weiter auf der Import-Seite, bei Deadline mit 503. Die Antwort beim Template-Fehler in `renderFragment` bleibt Klartext-500 (ohne Template keine Seite).

**Never:** Keine Änderung an `internal/core`, Migrationen, `openapi.yaml`, generiertem Code, an der htmx-`responseHandling` im Layout oder an `arc42.md`/Spine (D2 nicht übernommen). Kein gemeinsames Paket für die Klassifizierung zwischen Admin und öffentlicher API. Keine Änderung an Railway-Einstellungen.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Admin-Deadline | Use Case liefert umhülltes `context.DeadlineExceeded` (Events, Orte, neuer Ort, Import-Vorschau) | 503, HTML-Seite im Layout „Die Datenbank antwortet gerade zu langsam. Bitte versuch es gleich noch einmal.“ + Link zur Liste | `WARN`, kein `ERROR` |
| Admin-Abbruch | Fehler umhüllt `context.Canceled` | 500, deutsche Fehlerseite | `INFO`, kein `ERROR` |
| Admin-Fehler | anderer Fehler | 500, deutsche Fehlerseite „Etwas ist schiefgegangen. …“ + Link, kein englischer Text | `ERROR` mit Fehlertext |
| Commit-Deadline | `CommitImport` liefert umhülltes `DeadlineExceeded` | 503, Import-Seite mit Meldung „… es wurde nichts übernommen …“ | `WARN` |
| Commit-Fehler | anderer Fehler | wie bisher 500 + `msgImportCommitFailed` | `ERROR` bzw. `INFO` bei Abbruch |
| Shutdown | SIGTERM, Request wartet bis zur Deadline | Request endet mit Antwort, `serve` liefert `nil` | – |
| Server-Abbruch | Abfrage wartet auf `LOCK TABLE`, Deadline läuft ab | Aufrufer erhält einen Fehler; kurz danach wartet kein Backend des Pools mehr (`pg_stat_activity`, `wait_event_type = 'Lock'`, per `application_name`) | – |
| 57014 an der Deadline | Fehler ohne `DeadlineExceeded`, `r.Context().Err()` ist `DeadlineExceeded` | `/v1`: 503 Problem Details + `WARN`; Admin wie Admin-Deadline | – |
| Echte Deadline `/v1` | gesperrte Tabelle, echte pgx-Deadline | weiterhin 503 + `WARN` (`TestSlowQueryEndsWithServiceUnavailableAtTheDeadline`) | – |

</frozen-after-approval>

## Code Map

- `cmd/eventstore/main.go` -- Konstante `shutdownTimeout` (Z. 33) auf 25 s, Kommentar nennt Bezug zu `requestTimeout`. Tests pollen mit `shutdownTimeout` als Obergrenze (`main_test.go:152,566,711`, `cleanup_test.go:126`) – bleibt gültig.
- `cmd/eventstore/deadline_test.go` -- `TestRequestTimeoutEndsRequestsBeforeTheWriteTimeout` um das Verhältnis zu `shutdownTimeout` erweitern oder eigener Test daneben.
- `internal/adapter/admin/handler.go` -- `logRequestFailure` (Z. 312–321) wird zur zentralen Fehlerantwort: Status und Log-Level nach `DeadlineExceeded`/`Canceled`/sonst. `renderFragment` unverändert.
- `internal/adapter/admin/events.go:427`, `locations.go:351` -- `failEventRequest`/`failLocationRequest` rendern die neue Seite mit `BackURL` `eventsPath`/`locationsPath`; `new_location.go:86,95` laufen über `failLocationRequest`.
- `internal/adapter/admin/import.go:397-399` (Vorschau) und `:423-425` (Commit) -- Vorschau über die zentrale Seite (Link `importPath`), Commit mit Status aus derselben Klassifizierung und neuer Konstante für die Deadline-Meldung.
- `internal/adapter/admin/templates/not_found.html`, `forms.go:46` (`notFoundPage`), `templates.go:19,50` -- Vorbild; neue Seite `failure.html` + `failurePage` + `failureTemplate` analog.
- Tests: `events_test.go:574-625` (`assertServerErrorLogged`, Deadline-Test), `locations_test.go:745-780`, `new_location_test.go:272`, `delete_test.go:312,332,499`, `import_key_test.go:158`, `import_test.go:303-325`, `import_commit_test.go:308-335`; Hilfen `errRequestTimedOut`, `assertLoggedAt`, `assertLoggedAsWarningOnly` in `handler_test.go:851-891`.
- `internal/adapter/publicapi/v1/problem.go:98-115` -- `answerFailedRequest` bekommt den Request bereits (`_ *http.Request`); Deadline/Abbruch zusätzlich über `r.Context().Err()` erkennen. Tests in `events_test.go` mit Fehler ohne Kontext-Wrapping und abgelaufenem bzw. abgebrochenem Request-Kontext.
- Admin: die Fail-Funktionen bekommen `r`, damit dieselbe Prüfung über `r.Context().Err()` möglich ist.
- `internal/adapter/postgres/postgres.go:32-43` (`Connect`) -- `config.ConnConfig.BuildContextWatcherHandler` liefert `&pgconn.CancelRequestContextWatcherHandler{Conn: c, CancelRequestDelay: 0, DeadlineDelay: <Konstante>}`. Neuer Postgres-Test in `postgres_test.go` nach dem Muster von `TestSlowQueryEndsWithServiceUnavailableAtTheDeadline` (`cmd/eventstore/deadline_test.go:200`, Sperre per `LOCK TABLE events IN ACCESS EXCLUSIVE MODE`); Pool über `Connect` mit `application_name` in der URL, Zählung über einen zweiten Pool mit `datname = current_database()`, Polling mit Frist.
- `cmd/eventstore/deadline_test.go:200,219` -- echte Deadline-Tests der öffentlichen API müssen mit dem Handler weiter 503 + `WARN` liefern.
- `_bmad-output/implementation-artifacts/deferred-work.md` -- Eintrag „abgebrochene Abfrage läuft weiter“ (Spec 4.1) auf `status: done`, `target` nennt D1 und den Test.

## Nahtstellen

- `failEventRequest` -- 1.7/1.9/1.11/4.1 -- Lesen, Speichern, Löschen von Events und Ablaufplan antworten bei Fehlern weiter mit 5xx und loggen `logMsgEventsFailed` -- `TestEventPagesAnswerFailuresWithServerErrorAndLog`, `delete_test.go`, `import_key_test.go:158`; Deadline-Test erwartet künftig 503.
- `failLocationRequest` -- 1.4/1.8/1.11/4.1 -- Orte-Seiten und htmx-Fragment „neuer Ort“: Fragment-Request bekommt bei 5xx keine Ersetzung (htmx `swap:false`), das bleibt so -- `TestLocationPagesAnswerStorageFailureWithServerErrorAndLog`, `TestSavingNewLocationAnswersFailuresWithServerErrorAndLog`.
- `previewImport`/`commitImport` -- 3.2/3.3/4.1 -- 422/413 und Erfolg unverändert; Commit-Meldung bei sonstigem Fehler unverändert -- `TestImportCommitFailureSavesNothingAndSaysSo`, `TestImportFailureIsAServerErrorAndLogged`.
- Login-503 (`msgTooManyLogins`) -- 4.2 -- unverändert -- bestehende Login-Tests.
- `serve` -- 1.1/B1 -- Shutdown-Verhalten und Pool-Schließen -- `TestRunClosesTheDatabasePoolOnShutdown`, `TestServeAnswersUntilContextIsCancelled`.

## Tasks & Acceptance

**Execution:**
- [x] `cmd/eventstore/deadline_test.go`, `main.go` -- Verhältnistest, dann `shutdownTimeout` 25 s -- R-1.
- [x] `internal/adapter/admin/*_test.go` -- Tests für Matrix 1–5 (Status, Seiteninhalt deutsch ohne `Internal Server Error`, Log-Level), bestehende Deadline-Tests auf 503 -- S-1, P-2, D-1.
- [x] `internal/adapter/admin/templates/failure.html`, `templates.go`, `forms.go`, `handler.go`, `events.go`, `locations.go`, `import.go` -- Fehlerseite und zentrale Klassifizierung.
- [x] `internal/adapter/publicapi/v1/events_test.go`, `problem.go` -- Tests für 57014-artige Fehler bei abgelaufenem/abgebrochenem Request-Kontext, dann Klassifizierung über `r.Context().Err()` -- Matrix 8.
- [x] `internal/adapter/postgres/postgres_test.go`, `postgres.go` -- Server-Abbruch-Test, dann Handler in `Connect` -- Matrix 7, S-3.
- [x] `deferred-work.md` -- S-3-Eintrag schließen.

**Acceptance Criteria:**
- Given eine Admin-Seite, deren Use Case an der Deadline scheitert, when der Browser sie lädt, then sieht Andreas eine deutsche Seite mit Status 503 und einem Link zurück.
- Given alle Änderungen, when die CI läuft, then sind Tests, Abdeckung, Lint, Generator-Diff und Postgres-Tests grün.

## Implementation Notes

- Admin: `requestFailureOf` (handler.go) klassifiziert über Fehler und `r.Context().Err()`; `failRequest` loggt und rendert `failure.html` mit `failedArea` (Log-Meldung, Link zurück). Die Vorschau verlinkt mit „Zurück zum Import“ (`backToImport`) auf `importPath`. Der Commit nimmt Status aus derselben Klassifizierung; bei Deadline `msgImportCommitTimedOut`, sonst unverändert `msgImportCommitFailed`.
- `TestSavingNewLocationAnswersFailuresWithServerErrorAndLog` prüft jetzt `id="location-choice"`/`id="new-location"` statt der nackten IDs, weil die Fehlerseite das Layout mit der CSS-Klasse `new-location` enthält.
- S-3, beobachtet lokal (PG 18 unter Windows): Die erste Fassung von `TestQueryPastItsDeadlineEndsOnTheServer` war auch ohne Handler grün (wie in Spec 4.1), weil sie nur `err != nil` prüfte und `pg_stat_activity` in der offenen Sperr-Transaktion las (ein Snapshot je Transaktion). Nach dem Review fragt der Test über den `observer`-Pool außerhalb der Transaktion ab und verlangt einen `*pgconn.PgError` mit SQLSTATE 57014, den nur der Cancel-Handler erzeugt. Gegenversuch mit auskommentierter `BuildContextWatcherHandler`-Zeile: rot („timeout: context deadline exceeded, want SQLSTATE 57014“); mit Handler grün. Mit Handler liefert pgx nach ca. 320 ms 57014 ohne `DeadlineExceeded`; `TestSlowQueryEndsWithServiceUnavailableAtTheDeadline` bleibt dank der Kontext-Klassifizierung bei 503 + `WARN`. Ob ein Backend ohne Handler unter Linux weiterläuft, misst weiter nur die Linux-CI.
- `TestTxRunnerRollsBackAQueryTheServerCancelsAtTheDeadline` (tx_test.go): Schreiben in `InTx`, dann eine Abfrage auf das gesperrte `events`, die an der Deadline vom Server abgebrochen wird; `InTx` scheitert, nichts gespeichert, der Pool antwortet danach.
- `cancelSocketDelay` = 2 s: Socket-Frist nach dem Cancel-Request; liegt unter der Lücke `writeTimeout` − `requestTimeout` (10 s) und unter dem Shutdown-Puffer (5 s).
- Matrix-Audit (Orchestrator): Zeile „Shutdown“ war nur über den Verhältnistest belegt. `TestServeLetsARunningRequestAnswerDuringShutdown` (`cmd/eventstore/main_test.go`) zeigt, dass ein laufender Request beim Shutdown noch antwortet und `serve` `nil` liefert; zusammen mit `requestTimeout < shutdownTimeout` deckt das die Zeile.

## Spec Change Log

## Review Triage Log

| # | Layer | Befund | Verdikt | Begründung | Route |
|---|-------|--------|---------|------------|-------|
| 1 | edge | `pg_stat_activity` wird in der offenen Locker-Transaktion gepollt | medium | PostgreSQL friert die Aktivitätsdaten pro Transaktion ein; das Polling wiederholt sich nicht wirklich. | patch |
| 2 | edge/blind/verification | Server-Abbruch-Test prüft nur `err != nil`, war ohne Handler grün, kein Rot-Nachweis | medium | Bestätigt (Implementation Notes). Mit Prüfung auf SQLSTATE 57014 ist der Gegenversuch lokal rot. | patch |
| 3 | verification/blind | Admin: kein Test für 57014-Fehler mit vom Client abgebrochenem Kontext; Commit-Pfad nur mit umhülltem Fehler getestet | medium | Vorgeprüft: Ein `errors.Is`-Rückbau im Canceled-Fall bliebe unbemerkt. | patch |
| 4 | seam | Kein Test für eine Abfrage in `InTx`, die an der Deadline vom Server abgebrochen wird | medium | Abgebrochene Transaktion und Rollback mit abgelaufenem Kontext sind neu und betreffen Schreibpfade (Import). | patch |
| 5 | blind/edge | `TestServeLetsARunningRequestAnswerDuringShutdown` hängt bei frühem Fehler bzw. wenn der Request nie ankommt | low | Direkte Korrektur (Cleanup, Select mit Frist). | patch |
| 6 | edge/blind/seam/verification | Railway gibt standardmäßig 0 s zwischen SIGTERM und SIGKILL; 25 s `shutdownTimeout` wirkt in Produktion nicht | medium | Railway-Doku „Deployment Teardown“: Standard 0 s; `railway.json` setzt nichts. Frozen-Block schließt Railway-Einstellungen aus. | defer |
| 7 | blind/edge | htmx-Anlage eines Orts zeigt bei 500/503 nichts | medium | Vorbestehend; `responseHandling` laut Frozen-Block unverändert. | defer |
| 8 | blind/edge | `endedWith` stuft jeden Fehler nach Kontextende als Timeout/Abbruch ein, auch echte Fehler | low | Trifft nur, wenn ein echter Fehler genau mit Deadline oder Abbruch zusammenfällt. Die Korrektur bräuchte pgx-Wissen im Adapter oder eine Normalisierung in allen Repositories. Klassifizierung über `r.Context().Err()` ist im Frozen-Block entschieden. | reject |
| 9 | edge | Deadline während `COMMIT` meldet „nichts übernommen“ | low | Retro Epic 4 R-5 akzeptiert. | reject |
| 10 | blind/edge | Verhältnistest berücksichtigt `cancelSocketDelay` nicht | low | 20 s + 2 s < 25 s; die 2 s gelten nur, wenn der Cancel-Request scheitert. Ein Test über Paketgrenzen bräuchte einen Export. | reject |
| 11 | blind | Fehlgeschlagenes Speichern verliert die Eingaben, Link nur zur Liste | low | Vorbestehend (vorher englischer Klartext ohne Link); Browser-Zurück behält die Eingaben. | reject |
| 12 | blind | 503 ohne `Retry-After` | low | Retro Epic 4 R-6 akzeptiert. | reject |
| 13 | blind | `requestFailure.timedOut()` leitet die Art aus dem Status ab | low | Eine Zuordnung an einer Stelle; kein Aufrufer weicht ab. | reject |
| 14 | blind | `backToImport` im Meldungsblock, Wortgleichheit der Meldungen, gofmt-Ausrichtung | low | Kosmetisch. | reject |
| 15 | edge | `renderFragment` und 400-Antworten bleiben englischer Klartext | false | Frozen-Block: `renderFragment` bleibt Klartext; die Entscheidung „jeder Admin-Fehler“ betraf die 500-Antworten der Use Cases. 400 erreichen nur manipulierte Requests. | – |
| 16 | edge | AC 503-Seite gilt auch für die htmx-Ortsanlage | false | AC spricht vom Laden einer Seite im Browser; htmx-Fall siehe #7. | – |
| 17 | seam | Orte-Löschtests in `delete_test.go` prüfen nur den Status | low | Die Fehlerseite ist über `failLocationRequest` in den Orte-Tests geprüft; dieselbe Funktion. | reject |

## Design Notes

25 s statt `writeTimeout` (30 s): Nach der Deadline (20 s) endet die Abfrage, die Antwort ist in Millisekunden geschrieben; 5 s Puffer reichen. Railway beendet den Container nach dem SIGTERM möglicherweise früher; das ändert diese PR nicht.

Lokal (PostgreSQL unter Windows) war der Server-Abbruch-Test laut Spec 4.1 auch ohne Handler grün; rot erwartet nur unter Linux. Der Test-first-Nachweis für den Handler ist deshalb der CI-Lauf bzw. ein Gegenversuch, falls lokal möglich; im Implementation Notes festhalten, was beobachtet wurde. Die Klassifizierung über `r.Context().Err()` ist unabhängig davon lokal test-first.

## Verification

**Commands:**
- `CI= go test ./...` -- grün
- `bash scripts/check-coverage.sh` -- 100 %
- `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./...` -- keine Befunde
- `EVENTSTORE_TEST_DATABASE_URL=postgres://eventstore:eventstore@localhost:5432/eventstore_test?sslmode=disable go test -p 1 ./internal/adapter/postgres/... ./cmd/eventstore/...` -- grün
