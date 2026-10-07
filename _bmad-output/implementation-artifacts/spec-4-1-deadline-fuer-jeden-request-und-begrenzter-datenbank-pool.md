---
title: '4.1: Deadline für jeden Request und begrenzter Datenbank-Pool'
type: 'feature'
created: '2026-10-07'
status: 'done'
route: 'dispatch'
baseline_commit: '79afcdee2b0b152716590f9595ccbb5fb9e469bb'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-4-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Ein Request hat heute keine eigene Deadline, und der pgx-Pool hat die Standardgröße (max(4, CPU-Zahl), per `DATABASE_URL` überschreibbar). Eine langsame Datenbank oder eine Request-Flut staut deshalb wartende Requests ohne Grenze an, bis der Speicher ausgeht (NFR-5, AD-18).

**Approach:** Eine Middleware in `cmd/eventstore` gibt jedem Request einen Kontext mit `requestTimeout` (20 s, unter `writeTimeout` 30 s). `postgres.Connect` setzt `MaxConns` fest auf 10. Die öffentliche API antwortet bei abgelaufener Deadline mit `503` Problem Details und loggt eine Warnung; der Admin zeigt seine bisherige 500-Antwort, loggt ebenfalls eine Warnung, und die Transaktion wird zurückgerollt. Die Spec beschreibt `503` als gemeinsame Antwort.

## Boundaries & Constraints

**Always:** Test-first, 100 % Abdeckung in `publicapi/v1` und `admin`. Werte als benannte Konstanten. Die Middleware umschließt den ganzen Router; `/healthz` behält `healthPingTimeout`. Fehler, die nicht `context.DeadlineExceeded` umhüllen, verhalten sich wie bisher (`context.Canceled` → 500 + `INFO`, sonst 500 + `ERROR`). `api.gen.go` nur per `go generate ./...`. Entscheidung: „übliche deutsche Fehlerseite“ des Admins ist die bestehende 500-Antwort des jeweiligen Pfads (beim Import-Commit die deutsche Meldung `msgImportCommitFailed`); es gibt keine neue Seite.

**Never:** Kein `statement_timeout`, kein `http.TimeoutHandler`, kein Kern-Code, keine Migration, kein Rate Limiting, kein `Cache-Control` (Story 4.3). Keine Pool-Größe aus Konfiguration oder `DATABASE_URL`.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Langsame Abfrage `/v1` | Tabelle `events` per `LOCK TABLE … ACCESS EXCLUSIVE` gesperrt, kurze Deadline | `503` Problem Details, `detail` englisch, vor der Deadline + Reserve | Log `WARN`, kein `ERROR` |
| Pool erschöpft | alle 10 Verbindungen per `Acquire` belegt | `/v1/events` endet nach der Deadline mit `503` | wie oben |
| Deadline im Lister | Lister liefert Fehler, der `context.DeadlineExceeded` umhüllt | `503` Problem Details | `WARN` |
| Admin-Deadline | Use Case liefert umhülltes `context.DeadlineExceeded` | bisherige 500-Antwort des Pfads | `WARN` statt `ERROR` |
| Abbruch in Transaktion | Kontext läuft in `InTx` nach einem Schreibzugriff ab | `InTx` liefert Fehler, nichts gespeichert | Rollback |
| Pool-Größe | `DATABASE_URL` mit `pool_max_conns=50` | `pool.Config().MaxConns == 10` | – |

</frozen-after-approval>

## Code Map

- `cmd/eventstore/main.go` -- Konstanten (Z. 28–41) um `requestTimeout = 20 * time.Second` ergänzen; `newServer` legt `withRequestDeadline(newRouter(...), requestTimeout)` als `Handler`.
- `cmd/eventstore/health.go` -- `newRouter` bleibt; neue Funktion `withRequestDeadline(next http.Handler, timeout time.Duration) http.Handler` (hier oder neue Datei `deadline.go`) setzt `context.WithTimeout` auf `r.Context()`. `healthPingTimeout` (2 s) bleibt kürzer und wirkt weiter.
- `cmd/eventstore/main_test.go` -- Muster `testDatabaseURL`, `startRun`; Postgres-Tests für langsame Abfrage und erschöpften Pool bauen Pool (`postgres.Connect`), `publicapi.NewHandler` mit echtem `core.EventService` und `withRequestDeadline` mit kurzer Deadline (z. B. 200 ms) und prüfen Status, Body und Log.
- `internal/adapter/postgres/postgres.go` -- `Connect`: nach `ParseConfig` `config.MaxConns = maxConns` (Konstante 10), Doc-Kommentar nennt Grund.
- `internal/adapter/postgres/postgres_test.go` -- Test ohne Datenbank (Pool verbindet lazy): URL mit `pool_max_conns=50` → `MaxConns` 10.
- `internal/adapter/postgres/tx_test.go` -- Postgres-Test: `InTx` mit Kontext, der nach einem Insert abläuft → Fehler, Zeile nicht vorhanden.
- `internal/adapter/publicapi/v1/problem.go` -- `internalServerError` (Z. 96–103): zuerst `errors.Is(err, context.DeadlineExceeded)` → `Warn` mit `logMsgRequestTimedOut`, `writeProblem(503, detailServiceUnavailable)`; neue Konstanten. Aufrufer: Strict-Server (`RequestErrorHandlerFunc`/`ResponseErrorHandlerFunc` in `handler.go`) und `serveStaticFile`.
- `internal/adapter/publicapi/v1/events_test.go` -- neben dem `context.Canceled`-Test (Z. 370) Tests für beide Listen mit umhülltem `context.DeadlineExceeded`.
- `api/v1/openapi.yaml` -- `components.responses.ServiceUnavailable` mit Problem-Beispiel (status 503); `'503': $ref` bei `listEvents`, `listArchivedEvents`, `listEventTypes`; in KON-3 ein Satz zu `503` bei Überlastung. Danach `go generate ./...`.
- `internal/adapter/admin/handler.go` -- neue Methode `logRequestFailure(msg string, err error)`: `DeadlineExceeded` → `Warn`, sonst `Error`.
- `internal/adapter/admin/events.go` (Z. 427), `locations.go` (Z. 351), `import.go` (Z. 398, 424) -- `h.logger.Error(...)` durch `h.logRequestFailure(...)` ersetzen; Antworten unverändert.
- `internal/adapter/admin/*_test.go` -- je Pfad ein Test mit Fake, der umhülltes `DeadlineExceeded` liefert: Status/Seite wie bisher, Log `WARN`.

## Nahtstellen

- `internalServerError` -- 2.1/2.3, B1 -- `context.Canceled` bleibt 500 + `INFO`, andere Fehler 500 + `ERROR`, fehlende statische Datei 500 -- bestehende Tests in `events_test.go`, `handler_test.go` (`TestMissingStaticFileIsAnInternalServerErrorAndLogged`, `TestFailingAnswerIsAnInternalServerErrorAndLogged`).
- `newServer`/Server-Timeouts -- A1 -- alle Timeouts bleiben; `requestTimeout < writeTimeout` per Test.
- `newHealthHandler` -- 1.1 -- `/healthz` liefert bei hängender DB weiter nach 2 s `503` -- bestehende Health-Tests laufen über den umhüllten Router.
- Admin-Fehlerpfade `failEventRequest`, `failLocationRequest`, Import-Vorschau/-Commit -- 1.7, 1.4, 3.2, 3.3 -- Antworten unverändert; nur das Log-Level wechselt bei Deadline -- bestehende Fehler-Tests plus neue Deadline-Tests.
- `TxRunner.InTx` -- 1.7, ENT-15 -- Rollback auch bei abgelaufenem Kontext -- neuer Test in `tx_test.go`.
- Startreihenfolge in `run` -- 2.5 -- Migration und `RecomputeDerived` laufen ohne Request-Deadline auf demselben Pool (kein `statement_timeout`) -- bestehende `run`-Tests.

## Tasks & Acceptance

**Execution:**
- [x] `internal/adapter/postgres/postgres.go` + Test -- `MaxConns` fest 10 -- AD-18
- [x] `internal/adapter/postgres/tx_test.go` -- Rollback bei Deadline belegen -- Admin-AC
- [x] `api/v1/openapi.yaml`, `api.gen.go` -- `503` als gemeinsame Antwort, neu generieren -- Spec-AC
- [x] `internal/adapter/publicapi/v1/problem.go` + Tests -- `503` + `WARN` bei Deadline -- KON-3
- [x] `internal/adapter/admin/handler.go`, `events.go`, `locations.go`, `import.go` + Tests -- `WARN` bei Deadline -- AD-18
- [x] `cmd/eventstore/main.go`, `health.go` (oder `deadline.go`) + Tests -- Middleware, `requestTimeout`, Verhältnistest, Middleware-Test (Kontext hat Deadline ≤ Timeout), Postgres-Tests aus der Matrix

**Acceptance Criteria:**
- Given der Server, when `newServer` ihn baut, then trägt jeder Request (auch Admin und `/healthz`) einen Kontext mit Deadline ≤ `requestTimeout`, und `requestTimeout < writeTimeout`.
- Given die Spec, when ich die drei Operationen unter `/v1` lese, then ist `503` als gemeinsame Antwort mit Beispiel beschrieben, und der Generator-Diff ist leer.

## Implementation Notes

- Middleware in neuer Datei `cmd/eventstore/deadline.go` statt in `health.go`.
- `internalServerError` heißt nach dem Review `answerFailedRequest`, weil er auch 503 antwortet.
- Deferred: ob pgx bei Deadline das Backend per Cancel-Request beendet (lokal kein weiterlaufendes Backend beobachtet).

## Spec Change Log

## Review Triage Log

| # | Quelle | Befund | Verdikt | Begründung | Route |
|---|--------|--------|---------|------------|-------|
| 1 | blind | `internalServerError` antwortet jetzt auch 503, Name passt nicht | low | Clean-Code-Regel aus AGENTS.md; Umbenennen ist direkte Korrektur | patch |
| 2 | blind, edge | Deadline-Tests messen `time.Now()` nach dem Request, nur Obergrenze | low | Ein viel zu kurzer Timeout bliebe unbemerkt; Startzeit vorher + Untergrenze ist direkt | patch |
| 3 | blind | Health-Teil von `TestServerGivesEveryRequestADeadline` belegt die Middleware nicht | false | Ping-Deadline ist min(2 s, 20 s), am Pinger nicht unterscheidbar; die Middleware deckt Admin/Public ab | – |
| 4 | blind | Rollback-Test: `Create` kann selbst an der Deadline scheitern, Test besteht dann leer | low | Bei langsamem Runner real; `Create`-Fehler als Testfehler melden ist direkt | patch |
| 5 | blind | kein Test für Deadline während `COMMIT` | low | Verhalten liegt in pgx, Fenster von Millisekunden; Test bräuchte Fault-Injection | – |
| 6 | blind | 503 ohne `Retry-After` | low | Intent fordert keinen Header; neuer Vertragsteil wäre Zusatz | – |
| 7 | blind | 503 bei `/event-types` kann nicht auftreten | false | Intent verlangt 503 als gemeinsame Antwort aller Operationen unter `/v1` | – |
| 8 | blind | Postgres-Tests prüfen nur irgendein `WARN` | low | Fremde Warnung würde reichen; Level und Meldung prüfen ist direkt | patch |
| 9 | blind | Detail-Literal im Test statt Konstante | false | Paket-Tests prüfen Meldungen bewusst als Literale (`assertLogEntry`) | – |
| 10 | blind | Admin zeigt englischen Klartext statt deutscher Seite | false | Entscheidung im Frozen-Block: bestehende 500-Antwort | – |
| 11 | blind | Sprint-Status und Spec-Status weichen ab | false | Sync am Ende des Workflows (Persistent Fact) | – |
| 12 | blind | `errRequestTimedOut` in `locations_test.go`, Text „list from storage“ | low | Gemeinsamer Fixture-Wert an falscher Stelle; Verschieben ist direkt | patch |
| 13 | blind | `pool_min_conns` > 10 in `DATABASE_URL` | low | Railway setzt den Parameter nicht; Klemmen wäre zusätzlicher Zweig | – |
| 14 | blind | Render-Fehler loggen weiter `Error` | false | Template-Fehler umhüllen kein `DeadlineExceeded` | – |
| 15 | edge | pgx schließt bei Deadline nur den Socket, Backend läuft weiter, DB-Verbindungen > 10 | maybe-false | Lokaler Wegwerf-Test (PG 18, Windows): nach der Deadline wartet kein Backend mehr auf die Sperre. Offen, ob das unter Linux/Railway ebenso gilt | defer |
| 16 | edge | Deadline während `COMMIT` beim Import zeigt „nichts übernommen“, obwohl gespeichert | low | Fenster von Millisekunden in 20 s; Fix bräuchte neuen Zweig und Meldung | – |
| 17 | seam | Health-503 bei hängender DB nicht über `newServer` getestet | low | Verhalten unverändert, aber Spec nennt den Test; Test mit blockierendem Pinger ist direkt | patch |
| 18 | seam | Admin-Fehler ohne Deadline: Level `ERROR` nirgends geprüft | medium | `logRequestFailure` mit immer `Warn` bliebe grün | patch |
| 19 | seam | Import-Commit-Deadline-Test prüft Formular und Fehlen von „nichts gespeichert“ nicht | low | Geschwister-Test prüft beides; Ergänzen ist direkt | patch |

## Verification

**Commands:**
- `CI= go test ./...` -- grün
- `EVENTSTORE_TEST_DATABASE_URL=postgres://eventstore:eventstore@localhost:5432/eventstore_test?sslmode=disable go test -p 1 ./internal/adapter/postgres/... ./cmd/eventstore/...` -- grün
- `bash scripts/check-coverage.sh` -- 100 %
- `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./...` -- keine Befunde
- `go generate ./...` danach `git status` -- kein Diff
