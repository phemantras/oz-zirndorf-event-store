---
title: '4.3: Cache-Header und komprimierte statische Dateien in der öffentlichen API'
type: 'feature'
created: '2026-10-07'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
baseline_commit: '7da986598cca17ce9e067691c5d0eecd67b73599'
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-4-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Die Antworten unter `/v1` tragen kein `Cache-Control`, und Redoc-Skript (1,1 MB), Spec und Import-Schema gehen unkomprimiert raus. Wiederholte Abrufe und die Docs-Seite verursachen so unnötigen Egress (NFR-3, NFR-5, AD-18).

**Approach:** Jede Antwort unter `/v1` bekommt das `Cache-Control` aus AD-18: Listen `public, max-age=60`, Spec, Import-Schema und Docs-Seite `public, max-age=300`, Redoc-Skript `public, max-age=86400`, jede Fehlerantwort `no-store`. Redoc-Skript, Spec und Import-Schema werden beim Bau des Handlers einmal per gzip komprimiert und bei `Accept-Encoding: gzip` komprimiert ausgeliefert. Die Spec erklärt das Caching in `info`.

## Boundaries & Constraints

**Always:** Test-first, 100 % Abdeckung in `publicapi/v1`. Werte als benannte Konstanten. CORS-Header auf jeder Antwort. Preflight (`OPTIONS`) behält `Access-Control-Max-Age` und bekommt kein `Cache-Control`. HEAD trägt dieselben Header wie GET. `Vary: Accept-Encoding` steht auf komprimierter **und** unkomprimierter Antwort der drei Dateien, damit Caches beide Varianten trennen. `api.gen.go` nur per `go generate ./...`.

**Never:** Kein Kern-Code, keine Migration, keine Änderung an `cmd/eventstore` oder Admin. Keine Komprimierung pro Request, keine Komprimierung der Event-Listen und der Docs-Seite. Kein `ETag`/`Last-Modified`, keine Range-Antworten. Keine neuen Response-Header-Definitionen in der Spec (nur der Satz in `info`). Die 301 von `/v1/docs/` bleibt unverändert.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Liste ok | GET/HEAD `/v1/events`, `/v1/archive/events`, `/v1/event-types` | 200, `Cache-Control: public, max-age=60` | – |
| Filterfehler | `/v1/events?to=x` (400 aus dem generierten Strict-Server) | 400 Problem, `no-store` | – |
| Andere Fehler | 400 Parameter doppelt, 404, 405, 500, 503 | Problem, `no-store`, CORS | – |
| Spec/Schema/Docs | GET `/v1/openapi.yaml`, `/v1/import-v1.schema.json`, `/v1/docs` | `public, max-age=300` | – |
| Redoc-Skript | GET `/v1/docs/redoc.standalone.js` | `public, max-age=86400` | – |
| gzip | `Accept-Encoding: gzip, deflate` auf Skript/Spec/Schema | `Content-Encoding: gzip`, `Vary: Accept-Encoding`, entpackt = eingebettete Datei; Skript < 1/3 der Originalgröße | – |
| gzip abgelehnt | kein Header, `identity`, `br` oder `gzip;q=0` | unkomprimiert, kein `Content-Encoding`, `Vary: Accept-Encoding` | – |
| Datei fehlt | `serveStaticFile` mit unbekanntem Namen | 500 Problem, `no-store`, Log nennt Datei | `ERROR` |
| Komprimierung scheitert | ungültige gzip-Stufe | Datei wird nur unkomprimiert ausgeliefert | `WARN` beim Bau |

</frozen-after-approval>

## Code Map

- `internal/adapter/publicapi/v1/problem.go` -- `writeProblem` setzt `Cache-Control: no-store`; damit tragen 400 (`invalidParameter`), 404, 405, 500 und 503 (`answerFailedRequest`) es.
- `internal/adapter/publicapi/v1/handler.go` -- Konstanten für Header (`Cache-Control`, `Content-Encoding`, `Accept-Encoding`), Werte (`no-store`, drei `public, max-age=…`) und `gzipLevel`. `HandlerWithOptions` bekommt `Middlewares: []MiddlewareFunc{cacheFor(cacheControlLists)}`. `serveEmbedded` und `serveStaticFile` bekommen den Cache-Wert und ob komprimiert wird; `serveStaticFile` liest die Datei beim Bau statt pro Request (fehlende Datei → Handler, der `answerFailedRequest` ruft).
- neue Datei `internal/adapter/publicapi/v1/cache.go` -- `cacheFor(value) MiddlewareFunc` umhüllt den `ResponseWriter`: bei `WriteHeader` mit 2xx setzt er `value`, sonst `no-store`; `Write` ohne `WriteHeader` zählt als 200. Grund: die 400 des Strict-Servers (`ListEvents400ApplicationProblemPlusJSONResponse`) läuft nicht über `writeProblem`.
- neue Datei `internal/adapter/publicapi/v1/gzip.go` -- `gzipOf(body, level) ([]byte, error)` (eine Fehlerverzweigung, ungültige Stufe testbar); `acceptsGzip(r)` liest alle `Accept-Encoding`-Werte, Token `gzip` ohne Groß-/Kleinschreibung, `q=0` (auch `0.0`, `0.000`) lehnt ab.
- `internal/adapter/publicapi/v1/handler_test.go` -- Muster `serve`, `assertProblem` (um `no-store` ergänzen), `TestEveryAnswerAllowsAnyOrigin`, `TestPreflightIsAnsweredOnEveryPath` (kein `Cache-Control` prüfen), `TestMissingStaticFileIsAnInternalServerErrorAndLogged` (neue Signatur).
- `internal/adapter/publicapi/v1/events_test.go`, `archive_test.go` -- Erfolg 60 s, Filter-400 `no-store` (über `assertProblem`).
- `api/v1/openapi.yaml` -- in `info.description` neue Konvention **KON-9 Caching** nach KON-8; `api.gen.go` ändert sich dadurch nicht (keine eingebettete Spec), `go generate ./...` trotzdem laufen lassen.

## Nahtstellen

- `serveEmbedded`/`serveStaticFile` -- 2.1, 2.6, 3.1 -- Body, `Content-Type`, HEAD ohne Body, Range wird ignoriert, fehlende Datei 500 + Log -- `TestServesTheEmbedded*`, `TestServesTheDocsPage`, `TestDocsPageIgnoresRangeRequests`, `TestHeadOnTheDocsPageAnswersWithoutBody`, `TestMissingStaticFileIsAnInternalServerErrorAndLogged`.
- `readOnlyCORS`/`answerPreflight` -- 2.1 -- CORS auf jeder Antwort, Preflight 204 ohne `Cache-Control`, `Vary` mit `Access-Control-Request-Headers` bleibt -- Preflight- und CORS-Tests.
- `answerFailedRequest` (500/503) und `invalidParameter` -- 2.3, 4.1 -- Status, Detail und Log-Level unverändert, zusätzlich `no-store` -- bestehende Tests über `assertProblem`.
- Strict-Server-400 mit Filterdetails -- 2.3, 2.4 -- Detail unverändert, zusätzlich `no-store` -- `TestListEventsRejectsInvalidParameters…`, `TestListArchivedEventsRejectsInvalidParameters…`.
- `cmd/eventstore`-Tests mit `publicapi.NewHandler` -- 2.6, 3.5, 4.1 -- `NewHandler`-Signatur bleibt -- `go test ./cmd/...`.

## Tasks & Acceptance

**Execution:**
- [x] `problem.go`, `handler_test.go` -- `no-store` in `writeProblem`, `assertProblem` prüft es -- Fehler-AC
- [x] `cache.go` + Test -- Writer-Hülle, Middleware im Router, Listen-Tests (alle drei, GET und HEAD, Filter-400) -- Listen-AC
- [x] `gzip.go` + Test -- `gzipOf`, `acceptsGzip` mit Tabellentest der Matrix -- gzip-AC
- [x] `handler.go` + Tests -- Cache-Werte und gzip für statische Dateien, Lesen beim Bau, Fallback bei Komprimierungsfehler -- Datei-AC
- [x] `api/v1/openapi.yaml` -- KON-9, danach `go generate ./...` -- Spec-AC

**Acceptance Criteria:**
- Given jeder Pfad aus `TestEveryAnswerAllowsAnyOrigin` mit jeder Methode, when ich ihn abrufe, then trägt die Antwort genau das `Cache-Control` der Matrix (Preflight keins, 301 unverändert) und CORS.
- Given die Spec, when ich `info` lese, then sagt KON-9, dass Listen bis zu 60 Sekunden zwischengespeichert werden dürfen und ein Event deshalb bis zu einer Minute nach seinem Ende noch in `/v1/events` stehen kann, maßgeblich bleibt `effectiveEnd`; der Generator-Diff ist leer.

## Implementation Notes

## Spec Change Log

## Review Triage Log

| # | Quelle | Befund | Urteil | Begründung | Route |
|---|--------|--------|--------|------------|-------|
| 1 | blind | Kein `Content-Length` auf statischen Dateien, HEAD ohne Länge | low | Stimmt, bestand aber schon vor 4.3 (`serveEmbedded` setzte nie eine Länge); chunked Antworten funktionieren. | reject |
| 2 | blind | Redoc-Skript 1 Tag, Docs-Seite 5 min an fester URL | low | Die Werte gibt der Intent (AD-18) vor; ein Mismatch nach Redoc-Update bleibt selten und kurz. | reject (Intent) |
| 3 | blind | `cacheControlWriter` ohne `Unwrap` | low | Keine Route nutzt `Flusher`/`ResponseController`; Erweiterung ohne Bedarf. | reject |
| 4 | blind | Middleware überschreibt ein vom Handler gesetztes `Cache-Control` | false | Kein Listen-Handler setzt eines; der Fehlerfall setzt denselben Wert `no-store`. | reject |
| 5 | blind | gzip mit `BestCompression` bei jedem Handler-Bau verlangsamt Tests | low | Paket läuft in ~9 s, ein Test 2,3 s; Abhilfe (`sync.OnceValue`) fügt Paketzustand hinzu. | reject |
| 6 | blind | `*` und Gewichte wie `q=inf` | low | `identity` ist bei `*` immer zulässig (RFC 9110 §12.5.3), unkomprimiert also korrekt; fehlerhafte Gewichte nur bei kaputten Clients. | reject |
| 7 | blind | Cache-Control-Prüfung in zwei Tests dupliziert statt `assertCacheControl` | low | Referenzprojekt, Duplikat steht direkt neben dem Helper; Korrektur ist direkt. | patch |
| 8 | blind | `/v1/events`, `/v1/archive/events` fehlen in der Matrix für OPTIONS/Schreibmethoden | low | `readOnlyCORS` beantwortet beide vor dem Routing pfadunabhängig; Pfadliste stammt aus 2.1. | reject |
| 9 | blind | 301 von `/v1/docs/` ohne `Cache-Control` | false | Intent: „Die 301 von `/v1/docs/` bleibt unverändert“. | reject (Intent) |
| 10 | blind | KON-9 nennt Archiv und Event-Typen nicht einzeln; Router-weite Policy | low | AC verlangt genau den Satz zu `/v1/events`; „Listen“ deckt alle drei ab. | reject |
| 11 | blind | `Vary` auch nach gescheiterter Komprimierung | false | `Vary: Accept-Encoding` ohne Varianten ist harmlos und von der Matrix gedeckt („auf … unkomprimierter Antwort“). | reject |
| 12 | edge | 1xx-Status setzt `wroteHeader` | false | Keine Route sendet 1xx; der Fall ist unerreichbar. | reject |
| 13 | seam | HEAD ohne Body für die komprimierten Dateien nicht über echten Server getestet | low | Nahtstelle „HEAD ohne Body“ nur für die Docs-Seite (unkomprimiert) geprüft. | patch |
| 14 | seam | „Range wird ignoriert“ für komprimierte Datei nicht getestet | low | Nahtstelle nur für die Docs-Seite geprüft; gzip-Zweig ungeschützt. | patch |
| 15 | verif | keine Lücken | – | – | – |

## Verification

**Commands:**
- `CI= go test ./...` -- grün
- `bash scripts/check-coverage.sh` -- 100 %
- `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./...` -- keine Befunde
- `go generate ./...`, danach `git status` -- kein Diff außer der Spec
