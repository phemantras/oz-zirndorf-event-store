---
title: 'Story 2.5 (Teil A): Tägliche Bereinigung'
type: 'feature'
created: '2026-10-05'
status: 'done'
baseline_commit: 'ec4e3c6ae065763c9f2144807617d61ad5304536'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-2-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Vergangene Events tragen keine Markierung, und es gibt keinen Job, der sie setzt (FR-13, AD-5, AD-13). Die ACs von Story 2.5 in `epics.md` gelten, soweit sie `archived_at`, `MarkArchived`, den Job und `SaveEvent` betreffen. Die vollständige Neuberechnung beim Start (`name_key`, „prüfen“ für Orte, Ablaufplan-Grenze) ist Teil B und steht in `deferred-work.md`.

**Approach:** Neue Spalte `events.archived_at` (nullbar, expand). Der Kern-Anwendungsfall `MarkArchived(ctx, Clock)` setzt sie über `TxRunner` mit **einer** Anweisung `effective_end <= $now AND archived_at IS NULL` und liefert die Anzahl. Der Job in `adapter/cleanup` ruft ihn beim Start nach `RecomputeDerived` und vor dem HTTP-Server einmal synchron auf, danach alle 24 Stunden, und loggt die Anzahl. Fehler loggt er nur. `SaveEvent` leert `archived_at` beim Aktualisieren.

## Boundaries & Constraints

**Always:** Zeit nur aus `Clock`, einmal pro Lauf gelesen. Keine Lese-Abfrage und kein API- oder Admin-Feld nutzt `archived_at`; „archiviert“ bleibt `Period.IsOver`. Der Job importiert nur `core`, die Verdrahtung liegt in `cmd/eventstore`. Test-first, 100 % für `core`. Generierter Code nur per sqlc 1.31.1. Die Migration nur vorwärts, ältere Code-Versionen laufen weiter.

**Never:** Kein automatisches Löschen archivierter Events. Kein Ende des Programms durch einen Job-Fehler. Keine Advisory-Sperre in `TxRunner` (Story 3.3). Kein `name_key`, keine Orts-Markierung (Teil B). Keine Änderung an `/v1`-Antworten oder an der Spec. Keine neue Abhängigkeit.

## I/O & Edge-Case Matrix

Uhr: 2026-12-24 12:00 Europe/Berlin.

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Vorbei | Ende 11:00 und 12:00, `archived_at` leer | beide auf `now` gesetzt, Anzahl 2 geloggt | N/A |
| Läuft noch | Ende 12:01 | unverändert | N/A |
| Schon markiert | `archived_at` = 20.12. | bleibt 20.12., zählt nicht | N/A |
| Zweiter Lauf | nichts geändert | Anzahl 0, keine Zeile geändert | N/A |
| Wieder aktiv | archiviertes Event per `SaveEvent` auf 2027 verschoben | `archived_at` leer | N/A |
| Gleichzeitig | Job und `SaveEvent` (wieder aktiv) auf dasselbe Event | Event endet ohne `archived_at` | Bedingung steckt in der einen UPDATE-Anweisung |
| Repo-Fehler | Port oder Transaktion scheitert | `MarkArchived` gibt den Fehler weiter, Job loggt ihn, nächster Lauf läuft | Programm läuft weiter |
| Kontext beendet | Shutdown | Job-Schleife endet ohne Fehler-Log | N/A |

**Entscheidungen (Mensch, 2026-10-05):** `SaveEvent` leert `archived_at` bei **jedem** Update (`UpdateEvent` setzt `archived_at = NULL`), ohne Signaturänderung; bleibt das Event vergangen, markiert der nächste Lauf es neu. Volle Spec trotz ca. 2.700 Tokens.

</frozen-after-approval>

## Code Map

- `internal/adapter/postgres/migrations/00007_event_archived_at.sql` -- neu: `ALTER TABLE events ADD COLUMN archived_at timestamptz;`, Kommentar im Stil von `00006` (AD-17, Expand, nie ändern). Kein Index.
- `internal/adapter/postgres/queries/events.sql` -- neu `MarkEventsArchived :execrows` (`UPDATE events SET archived_at = @now WHERE effective_end <= @now AND archived_at IS NULL`). `UpdateEvent` setzt `archived_at = NULL`. Lese-Abfragen bleiben ohne `archived_at`.
- `internal/adapter/postgres/db/*` -- neu erzeugen. Weil die SELECT-Spalten nicht mehr der Tabelle entsprechen, erzeugt sqlc eigene Zeilentypen (`ListEventsRow`, `GetEventRow` …) statt `db.Event`. `eventFromRow`, `eventsWithTimetables` und `withTimetable` (`internal/adapter/postgres/events.go:208`, `:61`, `:160`) auf **einen** Zeilentyp umstellen, die anderen per Typkonvertierung (gleiche Felder). `archived_at` nicht in die SELECTs aufnehmen.
- `internal/adapter/postgres/events.go` -- neu `MarkArchived(ctx, now) (int, error)`.
- `internal/core/event_service.go:17` -- Port `EventRepo` um `MarkArchived(ctx, now time.Time) (int, error)` erweitern. Neu `(*EventService).MarkArchived(ctx, clock) (int, error)`: Hülle über `s.tx.InTx`, gebundener Kern `markArchived(ctx, repos, now)`, Muster wie `SaveEvent`/`saveEvent`. Kommentar am Port und bei `Derived` nicht ändern.
- `internal/core/event_service_test.go:63` -- Fake-Repo um `MarkArchived` (zählt die Aufrufe und hält `now`).
- `internal/adapter/cleanup/` -- `doc.go` existiert. Neu `cleanup.go`: Port `Archiver` (`MarkArchived(ctx, core.Clock) (int, error)`), `Job` mit `Archiver`, `Clock`, `Logger`. `RunOnce(ctx)` loggt Anzahl oder Fehler, `RunDaily(ctx, interval)` startet einen `time.Ticker` und endet mit `ctx`. Konstanten für Log-Texte und das 24-h-Intervall.
- `cmd/eventstore/main.go:83` -- nach `recomputeDerived`: `job.RunOnce(ctx)`, dann `go job.RunDaily(ctx, …)`, danach `net.Listen`. Paketkommentar und Kommentar von `run` um die Bereinigung ergänzen. `systemClock{}` wiederverwenden.
- `README.md` -- Startreihenfolge und tägliche Bereinigung kurz ergänzen.

## Nahtstellen

- `core.(*EventService).SaveEvent` / `saveEvent` -- 1.7, 1.9, 1.10 -- Validierung, Duplikatprüfung, Ablaufplan und `clearReview` unverändert; neu wird `archived_at` geleert -- bestehende Kern-Tests plus Postgres-Test „Wieder aktiv“.
- `postgres.EventRepo` `Get`/`List`/`ListOverlapping`/`FindByDuplicateKey`/`Create`/`Update` -- 1.7, 1.9, 1.10, 2.3, 2.4 -- liefern nach der Umstellung der Zeilentypen dieselben `core.Event` -- bestehende `internal/adapter/postgres/*_test.go`.
- `cmd/eventstore.run` -- 1.6, 1.7 -- Fehler bei `RecomputeDerived` bricht den Start weiter ab, ein Job-Fehler nicht; Startreihenfolge Recompute → Bereinigung → Listen -- `TestRunLogsEventsWhoseRecomputationFailsAndStartsAnyway`, neuer Test zur Reihenfolge.
- `publicapi/v1` `ListEvents`/`ListArchivedEvents`/`ListEventTypes` -- 2.1, 2.3, 2.4 -- Antworten vor und nach dem Job byte-gleich -- AC 2.
- Admin-Eventliste (`ListEvents`, `Archived`) -- 1.7 -- bleibt `IsOver`, nicht `archived_at` -- bestehende Admin-Tests.

## Tasks & Acceptance

**Execution:**
- [x] `internal/core/event_service_test.go`, `event_service.go` -- Test zuerst: `MarkArchived` reicht `clock.Now()` einmal an das Repo in `InTx`, liefert die Anzahl und gibt Repo- und Tx-Fehler weiter; danach implementieren.
- [x] `internal/adapter/postgres/migrations/00007_event_archived_at.sql`, `queries/events.sql`, `db/*`, `events.go`, `events_test.go` -- Postgres-Test zuerst für die Matrix-Zeilen Vorbei/Läuft noch/Schon markiert/Zweiter Lauf/Wieder aktiv; dann Migration, Query, sqlc, Repo.
- [x] `internal/adapter/postgres/queries_test.go` -- Test ohne Datenbank: in `queries/*.sql` steht `archived_at` nur in `MarkEventsArchived` und `UpdateEvent`.
- [x] `internal/adapter/cleanup/cleanup_test.go`, `cleanup.go` -- Test zuerst mit Fake-`Archiver` und fester `Clock`: Log mit Anzahl, Log bei Fehler ohne Abbruch, `RunDaily` läuft bei kurzem Intervall wiederholt und endet mit `ctx`.
- [x] `cmd/eventstore/main.go`, `main_test.go` -- Postgres-Test zuerst: `run` markiert ein vergangenes Event vor dem ersten Health-Check und loggt die Anzahl.
- [x] `cmd/eventstore/cleanup_test.go` -- Postgres-Test für AC 2.
- [x] `README.md` -- Bereinigung beschreiben.

**Acceptance Criteria:**
- Given der Programmstart, when Migrationen und `RecomputeDerived` durch sind, then läuft `MarkArchived` einmal, bevor der HTTP-Server lauscht, und danach alle 24 Stunden.
- Given ein Bestand mit vergangenen, laufenden und künftigen Events und eine feste `Clock`, when `/v1/events?from=1900-01-01`, `/v1/archive/events` und `/v1/event-types` vor und nach `MarkArchived` abgefragt werden, then sind die Antworten byte-gleich und mindestens ein Event wurde markiert.
- Given die Postgres-Tests, when sie gegen PostgreSQL 18 laufen, then belegen sie Migration, Bedingung der UPDATE-Anweisung und das Leeren beim Speichern.

## Implementation Notes

- sqlc erzeugt nach der neuen Spalte je Lese-Query einen eigenen Zeilentyp; alle werden per Typkonvertierung auf `db.GetEventRow` abgebildet (generische Hilfe `eventsFromRows`), `eventsWithTimetables` ordnet Programmpunkte über die String-ID zu.
- `UpdateEventDerived` leert `archived_at` ebenfalls (Review #1): Eine Neuberechnung, die ein Event wieder aktiv macht, hinterlässt sonst eine Markierung, die kein Lauf mehr entfernt. Noch vergangene Events markiert der direkt folgende Start-Lauf neu.
- `startCleanup(ctx, job, interval)` läuft mit eigenem `jobCtx`, der beim Verlassen von `run` vor `pool.Close` abgebrochen wird.
- Der Test „Gleichzeitig“ wartet über `pg_stat_activity` (eigene Datenbank, fremde PID), bis der Bereinigungs-UPDATE an der Zeilensperre hängt, und committet erst dann.
- `TestRunMigratesThenServesHealthUntilCancelled` braucht im Paketlauf gelegentlich ~5 s in `stop`: `http.Server.Shutdown` wartet bis zu 5 s auf vom Client vorab geöffnete, ungenutzte Verbindungen (StateNew). Timing-abhängig, gleicher Effekt wie bei `TestServeAnswersUntilContextIsCancelled` vor dieser Story; betrifft nur die Testdauer.
- Story 2.5 bleibt im Sprint-Status `in-progress`, bis Teil B (`deferred-work.md`) umgesetzt ist.

## Spec Change Log

## Review Triage Log

| # | Layer | Befund | Verdict | Evidenz | Route |
|---|-------|--------|---------|---------|-------|
| 1 | alle | `UpdateEventDerived` leert `archived_at` nicht; nach Regeländerung bleibt ein wieder aktives Event dauerhaft markiert | low | `MarkEventsArchived` füllt nur `IS NULL`; Fix ist eine SQL-Zeile wie in `UpdateEvent` | patch |
| 2 | blind | Port-Kommentare `Update`/`UpdateDerived` verschweigen das Leeren | low | Kern verlässt sich darauf; Fix ist Kommentar | patch |
| 3 | vg/seam | Täglicher Lauf und „Job-Fehler stoppt Start nicht“ in `run` ungetestet | medium | `startCleanup` nirgends getestet; Löschen von `go job.RunDaily` bliebe grün | patch |
| 4 | vg | Reihenfolge „vor Listen“ nur über Timing belegt | low | `go job.RunOnce` würde Test wohl bestehen; deterministischer Test bräuchte injizierbaren Archiver in `run`; Markierung ist über API nicht sichtbar | reject |
| 5 | edge/blind | Start-`RunOnce` ohne Timeout | low | gleiches gilt für `RecomputeDerived`; eine Replika (railway), Zeilensperren nur Millisekunden; Fix fügt Zweig hinzu | reject |
| 6 | edge | `RunDaily` mit Intervall <= 0 panikt | false | nur mit Konstante `DailyInterval` aufgerufen | reject |
| 7 | edge | Echter Fehler bei beendetem `ctx` nicht geloggt | low | gewollt beim Shutdown; Fehler entsteht dann durch den Abbruch | reject |
| 8 | edge/blind | `RunDaily`-Goroutine wird nicht gejoint | low | `stopJob` läuft vor `pool.Close` (defer LIFO), laufende Abfrage wird abgebrochen; WaitGroup wäre zusätzliche Maschinerie | reject |
| 9 | edge/blind | `loggedMarkedCount` scheitert bei leerem Log irreführend | low | `strings.Split("", "\n")` = `[""]`; direkte Korrektur | patch |
| 10 | blind | Doppelte Log-Key-Konstante in `main_test.go` | low | `logKeyMarked` ist unexportiert in anderem Paket; Drift fiele als Testfehler auf | reject |
| 11 | edge | `queries_test` ordnet Kommentar vor `-- name:` falsch zu | low | Datei schreibt Kommentare nach `-- name:`; unwahrscheinlich | reject |
| 12 | edge/blind | Epic-Kontext behauptet Advisory-Sperre in `TxRunner` | low | `tx.go` ohne Sperre; Satz direkt korrigiert (kommt mit 3.3) | patch |
| 13 | blind | Epic-Kontext beschreibt Teil B als aktuell | false | Teil B gehört weiter zu Epic 2 (Story 2.5), Kontext gilt fürs Epic | reject |
| 14 | blind | Epic-Kontext kürzt Fakten (Enums, Feldliste …) | low | Neu erzeugt per `compile-epic-context`; Quelle bleibt `openapi.yaml` und Code | reject |
| 15 | blind | AC-2-Test: `marked >= 1` auch durch fremde Zeilen erfüllbar | low | geteilte Testdatenbank; direkte Prüfung des Fixture-Events | patch |
| 16 | blind | `waitForBlockedCleanup` filtert nicht nach Datenbank | low | `pg_stat_activity` ist clusterweit; Filter ist eine Zeile | patch |
| 17 | blind | Spec- und Sprint-Status widersprechen sich | false | Sprint-Status wird am Ende des Workflows nachgezogen | reject |
| 18 | blind | Deferred-Eintrag ohne `source_spec`/`status`/`target` | false | Format vom Workflow vorgegeben (Split vor Spec) | reject |
| 19 | blind | Kein Index für den täglichen UPDATE | low | Hobby-Bestand, ein Lauf pro Tag | reject |
| 20 | blind | README ohne `pg_dump`-Hinweis | false | README-Abschnitt „Migrationen und Datensicherung (AD-17)“ deckt jede Migration ab | reject |
| 21 | blind | Schwache Tests im Cleanup-Paket (Intervall-Test tautologisch, Logs verworfen, `clocks[0]` ohne Mutex) | low | Fehler-Log deckt `TestRunOnceLogsAFailureAndReturns`; `RunOnce` ist synchron, kein Race; Intervall-Weitergabe kommt mit #3 | reject |
| 22 | blind | Leere Implementation Notes | false | Fix wäre Spec-Bearbeitung; Notes werden am Ende ergänzt | reject |

## Design Notes

Zum gleichzeitigen Lauf: Unter READ COMMITTED wartet die UPDATE-Anweisung des Jobs auf die Zeilensperre einer laufenden `SaveEvent`-Transaktion. Danach prüft sie `WHERE` an der neuen Zeilenversion erneut. Ein wieder aktives Event erfüllt `effective_end <= now` nicht mehr und bleibt deshalb leer. Läuft der Job zuerst, überschreibt das spätere `UPDATE` von `SaveEvent` den Wert. Deshalb liest `SaveEvent` `archived_at` nie und setzt es nur.

## Verification

**Commands:**
- `sqlc generate` (1.31.1, Release-Binary unter Windows) und `go generate ./...`; danach `git status --porcelain` -- nur die erwarteten Dateien
- `CI= go test ./...` -- grün
- `bash scripts/check-coverage.sh` -- 100 %
- `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./...` -- ohne Befund
- `EVENTSTORE_TEST_DATABASE_URL=… go test -p 1 ./internal/adapter/postgres/... ./cmd/eventstore/...` -- grün (lokal natives PG 18)
