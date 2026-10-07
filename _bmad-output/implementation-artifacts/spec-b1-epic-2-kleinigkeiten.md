---
title: 'B1: Epic-2-Kleinigkeiten (Retro Epic 2)'
type: 'bugfix'
created: '2026-10-07'
status: 'done'
route: 'dispatch'
baseline_commit: '762a468660ed48f34d8e8fb6765b841efd6081ea'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-2-retro-2026-10-05.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Drei kleine Lücken aus Epic 2 (Retro Epic 2, R4–R6) und ein auf B1 zugeordneter Deferred-Eintrag: Bricht ein Client eine Abfrage der Public API ab, steht `context.Canceled` als `Error` im Log. `/v1/docs/` (mit Schrägstrich) liefert 404. Kein Test prüft, dass `scripts/check-coverage.sh` generierte Dateien richtig ausnimmt, obwohl das Gate die 100-%-Policy allein durchsetzt. Kein Test zeigt, dass `run` den Datenbank-Pool beim Shutdown schließt.

**Approach:** Test-first: (a) `internalServerError` loggt einen vom Client abgebrochenen Request (`errors.Is(err, context.Canceled)`) auf `Info` mit eigener Meldung statt auf `Error`; (b) `GET /v1/docs/` leitet mit 301 auf `/v1/docs` um; (c) das Coverage-Skript nimmt optional Paketmuster als Argumente, ein Go-Test führt es gegen ein Fixture-Modul aus; (d) ein Postgres-Test in `cmd/eventstore` zählt die Verbindungen des Laufs in `pg_stat_activity` vor und nach dem Stopp.

## Boundaries & Constraints

**Always:** Test-first. 100 % Abdeckung in `publicapi/v1` bleibt. Andere Fehler als `context.Canceled` loggt `internalServerError` weiter als `Error`; die Antwort (500 Problem Details) bleibt in beiden Fällen gleich. Ohne Argumente prüft das Skript genau die drei Pflichtpakete wie bisher (CI-Aufruf unverändert). Der Skript-Test läuft in `go test ./...` mit und scheitert, wenn `bash` fehlt (kein Skip). Der Pool-Test folgt dem Muster der übrigen Postgres-Tests (`testDatabaseURL`, `startRun`) und erkennt die Verbindungen des Laufs am `application_name` in der Datenbank-URL.

**Never:** Keine Änderung an `openapi.yaml`, `docs.html`, generiertem Code, Migrationen oder an `main.go`. Keine Umleitung für andere Pfade unter `/v1/docs/`. Kein Ausschluss von Code aus der Abdeckung.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Client bricht ab | Lister liefert Fehler, der `context.Canceled` umhüllt | 500 Problem Details; Log-Level `INFO`, Meldung „public api request cancelled by client“, kein `ERROR` | – |
| Echter Fehler | Lister liefert anderen Fehler | wie bisher: 500, `ERROR` mit Fehlertext | – |
| Docs mit Schrägstrich | `GET`/`HEAD /v1/docs/` | 301, `Location: /v1/docs`, CORS `*` | – |
| Docs exakt | `GET /v1/docs`, `/v1/docs/redoc.standalone.js` | wie bisher 200 | – |
| Anderer Docs-Pfad | `GET /v1/docs/x` | wie bisher 404 Problem Details | – |
| Generierte Datei | Paket: handgeschriebene Datei voll abgedeckt, generierte Datei (Header vor `package`) ungetestet | Gate grün, meldet `N of N` mit N > 0 | – |
| Später Header | Header-Zeile erst nach `package`, Datei ungetestet | Gate scheitert, `uncovered:` nennt die Datei | – |
| Lücke | handgeschriebene Funktion ungetestet | Gate scheitert mit `100 % required` | – |
| Shutdown | `run` gestoppt | keine Verbindung mit dem `application_name` des Laufs mehr in `pg_stat_activity` (Abfrage mit Frist) | vor dem Stopp mindestens eine |

</frozen-after-approval>

## Code Map

- `internal/adapter/publicapi/v1/problem.go` -- `internalServerError` (Z. 91–95) verzweigt nach `context.Canceled`; neue Konstante `logMsgRequestCancelled`. Aufrufer: Strict-Server (`RequestErrorHandlerFunc`, `ResponseErrorHandlerFunc` in `handler.go`) und `serveStaticFile`.
- `internal/adapter/publicapi/v1/handler.go` -- in `newHandler` Route `GET /v1/docs/{$}` (neue Konstante `docsSlashPath`) mit `http.Redirect(…, docsPath, http.StatusMovedPermanently)`; `docs.html` lädt Skript und Spec relativ zu `/v1/docs`, deshalb Umleitung statt Ausliefern.
- `internal/adapter/publicapi/v1/events_test.go` -- neben `TestListEventsFailureIsAnInternalServerErrorAndLogged` Test mit umhülltem `context.Canceled` (Fake `recordingLister`, Log per `bytes.Buffer`).
- `internal/adapter/publicapi/v1/handler_test.go` -- Tests Umleitung (GET, HEAD, CORS) und 404 für `/v1/docs/x`.
- `scripts/check-coverage.sh` -- `go test` über `"$@"`, sonst `REQUIRED_PACKAGES`; Kopfkommentar ergänzen.
- `scripts/check_coverage_test.go` (neu, Paket `scripts`, nur Test) -- führt `bash check-coverage.sh ./<fall>/...` mit `Dir` = `testdata/coveragefixture`, `GOWORK=off` aus; prüft Exit-Code und Ausgabe je Fall.
- `scripts/testdata/coveragefixture/` (neu) -- eigenes `go.mod`, Pakete `generated/`, `lateheader/`, `gap/` je mit Code und Test; `testdata` ist für `./...` und golangci-lint unsichtbar.
- `cmd/eventstore/main_test.go` -- neuer Postgres-Test mit `startRun` und einer URL mit `application_name=eventstore-pool-close-test` (per `net/url` angehängt); Zählung über einen eigenen Pool, Polling mit `healthPollInterval` bis `shutdownTimeout`.
- `_bmad-output/implementation-artifacts/deferred-work.md` -- Eintrag „Pool beim Shutdown schließen“ auf `status: done`, `target` nennt B1 und den Test.

## Nahtstellen

- `internalServerError` -- Story 2.1/2.3 -- Fehler aus Strict-Server und fehlender statischer Datei bleiben `ERROR` mit 500 -- `TestListEventsFailureIsAnInternalServerErrorAndLogged`, `TestListArchivedEventsFailureIsAnInternalServerErrorAndLogged`, `TestMissingStaticFileIsAnInternalServerErrorAndLogged`, `TestFailingAnswerIsAnInternalServerErrorAndLogged`.
- Docs-Routen in `newHandler` -- Story 2.6 -- `/v1/docs`, Skript, Range-Ignoranz, HEAD und CORS-Schleifen über alle Pfade unverändert; Catch-all `/v1/` liefert für andere Pfade weiter 404 -- bestehende Tests in `handler_test.go`, neuer 404-Test.
- `scripts/check-coverage.sh` -- Story 1.1/2.1 -- CI-Aufruf ohne Argumente prüft dieselben Pakete und nimmt generierte Dateien wie bisher aus -- CI-Schritt `Coverage gate`, lokaler Lauf ohne Argumente.

## Tasks & Acceptance

**Execution:**
- [x] `internal/adapter/publicapi/v1/events_test.go`, `problem.go` -- Test Abbruch, dann Verzweigung -- Matrix 1–2.
- [x] `internal/adapter/publicapi/v1/handler_test.go`, `handler.go` -- Tests, dann Umleitung -- Matrix 3–5.
- [x] `scripts/testdata/coveragefixture/…`, `scripts/check_coverage_test.go`, `scripts/check-coverage.sh` -- Fixture und Test, dann Argumente -- Matrix 6–8.
- [x] `cmd/eventstore/main_test.go` -- Pool-Test -- Matrix 9.
- [x] `deferred-work.md` -- Eintrag schließen.

**Acceptance Criteria:**
- Given das Skript ohne Argumente, when es lokal läuft, then meldet es dieselbe Anweisungszahl wie vor der Änderung und ist grün.
- Given alle Änderungen, when die CI läuft, then sind Tests, Abdeckung, Lint, Generator-Diff und Postgres-Tests grün.

## Implementation Notes

- Jeder neue Test lief zuerst rot. Den Pool-Test hat die Umsetzung zusätzlich ohne `defer pool.Close()` in `main.go` laufen lassen: Er scheitert („1 connections of the run are still open“). Danach wurde `main.go` zurückgesetzt.
- Coverage-Gate ohne Argumente: Das alte Skript (HEAD) und das neue melden auf demselben Code beide 2202 von 2202 Anweisungen. Der Anstieg von 2198 kommt nur aus neuem Code.
- In `TestUnknownPathsAreNotFound` steht `/v1/docs/x` statt `/v1/docs/`, denn `/v1/docs/` leitet jetzt um.
- Unter Windows findet `exec.LookPath("bash")` zuerst den WSL-Starter in System32. Der Skript-Test nimmt deshalb unter Windows Git Bash neben `git` (Triage #4).

## Spec Change Log

## Review Triage Log

| # | Layer | Befund | Verdikt | Begründung | Route |
|---|-------|--------|---------|------------|-------|
| 1 | verification/seam/blind | Kein Test prüft, dass ein nicht abgebrochener Fehler weiter als `ERROR` „public api request failed“ geloggt wird | medium | Vorgeprüft: Die vier Fehlertests prüfen nur den Fehlertext. Ein falscher `else`-Zweig bliebe unbemerkt. | patch |
| 2 | blind/seam | `/v1/docs/` fehlt in den Tests, die über jeden Pfad laufen (Preflight, Schreibmethoden, CORS) | low | Stimmt. Das Verhalten folgt aus `readOnlyCORS`, ist für den neuen Pfad aber ungetestet. Die Korrektur ist ein Listeneintrag. | patch |
| 3 | blind | `pg_stat_activity` wird ohne `datname` gezählt | low | Stimmt. Ein paralleler Lauf auf derselben Instanz (Dev- und Test-DB nativ) würde mitgezählt. Die Korrektur ist eine Bedingung in der Abfrage. | patch |
| 4 | blind/edge | Unter Windows findet `LookPath("bash")` den WSL-Starter, `go test ./scripts/` scheitert aus PowerShell bzw. der VS-Code-Go-Erweiterung | medium | Nachgestellt: Ausgabe „Windows-Subsystem für Linux verfügt über keine installierten Distributionen“. Der Entwickler arbeitet unter Windows. | patch |
| 5 | blind/edge | `context.Canceled` aus einer serverseitigen Quelle würde als Client-Abbruch geloggt; `r.Context().Err()` prüfen | false | Die Abfragen laufen nur im Request-Kontext. Den beendet nur ein Client-Abbruch, denn `Shutdown` bricht keine Requests ab. Eine andere Quelle für `Canceled` gibt es nicht. | – |
| 6 | edge | Ein Client-Abbruch beim Schreiben (EPIPE) loggt weiter `ERROR` | low | Der Fehler ist älter als diese Änderung und nicht R4. Die Korrektur wäre ein weiterer Zweig für einen seltenen Fall. | reject |
| 7 | blind | Der Deferred-Eintrag hat `status: done`, aber noch die alte `evidence` | low | `evidence` beschreibt, warum der Eintrag offen war. Die Erledigung steht in `target` mit dem Testnamen. | reject |
| 8 | blind | Neue Tests schreiben Pfade aus, statt die Konstanten zu nutzen | false | Die bestehenden Tests schreiben `/v1/docs/nope` usw. ebenfalls aus. Das ist Muster der Datei. | – |
| 9 | blind | Kein Beleg, dass der Pool-Test ohne `pool.Close()` scheitert | false | Der Gegenversuch lief, siehe Implementation Notes. | – |
| 10 | blind | Der Standardpfad des Skripts (ohne Argumente) hat keinen automatischen Test | false | Der CI-Schritt `Coverage gate` ruft genau diesen Pfad auf. Die Anweisungszahl vorher und nachher ist in den Implementation Notes festgehalten. | – |
| 11 | blind | Kein Fixture für „no statements yet“ und für mehrere Muster | low | Beides ist unverändertes Verhalten. Neue Fixtures wären zusätzlicher Umfang. | reject |
| 12 | blind | Der Unterprozess erbt `GOFLAGS`, `GOCOVERDIR`, `CI` | low | Die Fixture-Tests lesen keine dieser Variablen, und das innere `go test` setzt sein eigenes Profil. Ein Fehlschlag ist nicht gezeigt. | reject |
| 13 | blind/edge | Die Umleitung verwirft den Query-String; die Wahl von 301 ist nicht begründet | low | Die Docs-Seite hat keine Parameter. 301 passt, weil der Pfad `/v1/docs` Teil des Vertrags ist. | reject |
| 14 | blind | Der Abbruch-Test hat eine redundante `ERROR`-Prüfung und prüft das Attribut `error` nicht | low | Kosmetisch: Level und Meldung sind geprüft. | reject |
| 15 | blind | Der Code Map nennt Zeilennummern | – | Die Korrektur wäre eine Änderung dieser Spec. | reject |
| 16 | edge | Eine Test-URL in DSN-Form (`host=…`) würde zerlegt | low | AGENTS.md legt die URL-Form fest. Die Korrektur wäre eine zusätzliche Prüfung. | reject |
| 17 | edge | Skript-Argumente mit `-` werden als Flag gelesen | low | Nur der Selbsttest übergibt Argumente, und zwar Paketmuster. | reject |

## Design Notes

`Info` statt `Debug` für den Abbruch: Der Logger läuft mit Standard-Level `Info`, ein Abbruch bleibt damit sichtbar, alarmiert aber nicht. Die Antwort wird trotzdem geschrieben; der Client ist meist weg, ein Schreibfehler landet wie bisher als `Warn`. Der Pool-Test braucht keine Injektionsnaht in `main.go`, weil PostgreSQL die Verbindungen am `application_name` erkennt.

## Verification

**Commands:**
- `CI= go test ./...` -- grün, inkl. `scripts`
- `bash scripts/check-coverage.sh` -- 100 %
- `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./...` -- keine Befunde
- `EVENTSTORE_TEST_DATABASE_URL=… go test -p 1 ./internal/adapter/postgres/... ./cmd/eventstore/...` -- grün
