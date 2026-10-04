---
title: 'Story 2.1: API-Vertrag v1 und Liste der Event-Typen'
type: 'feature'
created: '2026-10-04'
status: 'done'
baseline_commit: '7025815d6058b38ead2bbdbbbb5ff80f7c8ec030'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-2-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Es gibt noch keine öffentliche API: kein `api/v1/openapi.yaml`, kein generiertes Gerüst, keine Route unter `/v1/`. Abnehmer können weder den Vertrag lesen noch die Event-Typen abfragen. Die Akzeptanzkriterien der Story 2.1 in `epics.md` gelten vollständig.

**Approach:** Spec-first: `api/v1/openapi.yaml` (3.1) mit Konventionen in `info`, Enum-Schemas, Listenhülle, `Problem` und `GET /event-types` (Server `/v1`). `oapi-codegen` v2.8.0 erzeugt das Gerüst nach `internal/adapter/publicapi/v1`. Der Kern liefert `ListEventTypes()` (Code + deutsche Beschriftung, eine Quelle auch für den Admin). Ein Handler unter `/v1/` setzt CORS, lehnt Schreibmethoden mit 405 ab, beantwortet unbekannte Pfade mit 404 und liefert die Spec unter `/v1/openapi.yaml` aus.

## Boundaries & Constraints

**Always:** Feldnamen und Codes nur aus der Spec; Kern-Konstanten spiegeln sie, ein Test vergleicht. Fehler als `application/problem+json` (RFC 9457, `type` `about:blank`, englischer `title`/`detail`). CORS `Access-Control-Allow-Origin: *` auf jeder `/v1/`-Antwort. `publicapi/v1` importiert nur `core` (und das Spec-Paket `api/v1`), nie andere Adapter. Generierter Code wird nie von Hand geändert und vom Coverage-Gate ausgenommen. Test-first, 100 % für `core` und `publicapi/v1`.

**Never:** Keine Event-, Orts- oder Archiv-Endpunkte (2.2–2.4), kein `EventInput`, keine Redoc-Seite `/v1/docs` und kein `/v1/import-v1.schema.json` (2.6/3.1), kein `id`-Feld. Keine neue Migration, keine Datenbankabfrage. Keine Abhängigkeit außerhalb der Stack-Tabelle außer der von oapi-codegen erzwungenen `github.com/oapi-codegen/runtime`.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Typen | `GET /v1/event-types` | 200, `application/json`, `{"data":[{"code":"festival","label":"Fest/Kirchweih"},…]}`, sieben Einträge in Kern-Reihenfolge | N/A |
| HEAD | `HEAD /v1/event-types` | 200 ohne Body | N/A |
| Spec | `GET /v1/openapi.yaml` | 200, Bytes der eingebetteten Spec, `application/yaml` | N/A |
| Preflight | `OPTIONS` auf beliebigen `/v1/`-Pfad | 204, `Allow-Origin: *`, `Allow-Methods: GET, HEAD, OPTIONS`, `Allow-Headers` spiegelt `Access-Control-Request-Headers`, `Max-Age: 86400` | N/A |
| Schreibmethode | `POST`/`PUT`/`PATCH`/`DELETE` auf `/v1/…` (auch unbekannten Pfad) | 405, `Allow: GET, HEAD, OPTIONS`, Problem mit CORS-Header | kein Routing |
| Unbekannt | `GET /v1/nope`, `GET /v1/` | 404, Problem mit CORS-Header | N/A |
| Antwortfehler | Strict-Server meldet Request-/Response-Fehler | 500 Problem, Fehler geloggt | kein Panik-Absturz |

**Entscheidungen ohne Rückfrage (2026-10-04):** Eintragsfelder `code`/`label`. Spec-Pfade ohne Präfix, `servers: [{url: /v1}]`, `BaseURL` `/v1`. Enum-Abgleich ohne YAML-Bibliothek: Test parst die generierte Datei per `go/parser`, sammelt alle Konstanten der Typen `EventType`, `TimePrecision`, `LocationPrecision` und vergleicht sie als Menge mit `core.EventTypes()`, `TimePrecisions()`, `LocationPrecisions()`; die CI-Prüfung „generierter Code aktuell“ bindet die generierte Datei an die Spec. Scheitert `oapi-codegen` an 3.1, Rückfall 3.0.3 und Vermerk in den Implementation Notes.

**Entscheidungen des Menschen (2026-10-04):** Story 2.1 wird vor dem Correct Course zu öffentlichen Kennungen umgesetzt, weil sie keine Kennung enthält; der Correct Course folgt vor Story 2.2. `info` erwähnt kein ID-Format. Die Spec bleibt trotz rund 2.600 Tokens ungeteilt.

</frozen-after-approval>

## Code Map

- `internal/core/event.go` -- `EventTypes()` bleibt; neu `EventTypeEntry{Code EventType; Label string}` und `ListEventTypes() []EventTypeEntry` mit den deutschen Beschriftungen aus FR-5; Kommentare „will define … Story 2.1“ auf „defines“ aktualisieren (auch `eventtimes.go`, `location.go`).
- `internal/adapter/admin/events.go:76-85` -- `eventTypeLabels` entfernen, Beschriftungen aus `core.ListEventTypes()` beziehen (Nutzung Z. 311, 373).
- `api/v1/.gitkeep` -- ersetzen durch `openapi.yaml` und `spec.go` (`package v1`, `//go:embed openapi.yaml`, `var OpenAPISpec []byte`); `publicapi/v1` importiert es mit Alias.
- `internal/adapter/publicapi/v1/doc.go` -- `//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 -config oapi-codegen.yaml ../../../../api/v1/openapi.yaml`; Konfig: `package: v1`, `generate: {std-http-server, strict-server, models}`, Ausgabe `api.gen.go`.
- `internal/adapter/publicapi/v1/` neu -- `server.go` (Strict-Implementierung `ListEventTypes`, nur Abbildung), `handler.go` (`NewHandler(Config{Logger}) http.Handler`: eigener `ServeMux` mit generierten Routen, `GET /v1/openapi.yaml`, Catch-all `/v1/` → 404; Middleware für CORS/OPTIONS/405), `problem.go` (`writeProblem` mit generiertem `Problem`-Typ, Strict-Error-Handler → 500).
- `cmd/eventstore/health.go:42` -- `newRouter` bekommt den Public-Handler und hängt ihn unter `/v1/` ein; `main.go:112`, `health_test.go:55,75` anpassen.
- `internal/archtest/arch_test.go` -- unverändert; muss mit `api/v1`-Import grün bleiben.
- `scripts/check-coverage.sh` -- Dateien mit Kopfzeile `// Code generated … DO NOT EDIT.` vor dem Zählen aus dem Profil filtern (Profilpfade beginnen mit dem Modulpfad).
- `.github/workflows/ci.yaml:27` -- Schritt „generierter Code aktuell“ um `go generate ./internal/adapter/publicapi/v1/...` erweitern (danach dieselbe Diff-/Untracked-Prüfung).

## Tasks & Acceptance

**Execution:**
- [x] `internal/core/event_test.go`, `event.go` -- Test zuerst: sieben Einträge, Reihenfolge wie `EventTypes()`, Beschriftungen laut FR-5 -- eine Quelle für Beschriftungen.
- [x] `internal/adapter/admin/events.go` -- auf Kern-Beschriftungen umstellen, Admin-Tests bleiben grün.
- [x] `api/v1/openapi.yaml`, `api/v1/spec.go` -- Spec mit `info` (KON-1–KON-8, NFR-2, ≥ 6 Monate Parallelbetrieb), Schemas `EventType`, `TimePrecision`, `LocationPrecision`, `EventTypeEntry`, `EventTypeList`, `Problem`, Pfad `/event-types`; jedes Feld und jeder Code englisch beschrieben, mit Beispiel.
- [x] `internal/adapter/publicapi/v1/oapi-codegen.yaml`, `doc.go`, `api.gen.go`, `go.mod`/`go.sum` -- generieren, `oapi-codegen/runtime` per `go mod tidy`.
- [x] `internal/adapter/publicapi/v1/*_test.go` -- Test zuerst je Zeile der I/O-Matrix (per `httptest`) und Enum-Abgleich inkl. Gegenprobe, dass der Parser Konstanten findet.
- [x] `internal/adapter/publicapi/v1/server.go`, `handler.go`, `problem.go` -- implementieren.
- [x] `cmd/eventstore/health.go`, `main.go`, `health_test.go` -- `/v1/` einhängen, Router-Test für `/v1/event-types`.
- [x] `scripts/check-coverage.sh`, `.github/workflows/ci.yaml` -- Generiertes ausnehmen, Codegen-Prüfung.
- [x] `README.md` -- Codegen-Befehl und `/v1/event-types`, `/v1/openapi.yaml` ergänzen; `deferred-work.md`: AGENTS.md-TODO für oapi-codegen per Refresh.

**Acceptance Criteria:**
- Given eine geänderte Spec ohne neu generierten Code, when die CI läuft, then scheitert der Codegen-Schritt.
- Given ein Code in der Spec, der im Kern fehlt (oder umgekehrt), when `go test ./...` läuft, then scheitert der Enum-Abgleich.
- Given generierter Code ohne Tests, when `scripts/check-coverage.sh` läuft, then zählt er nicht mit, eigener Code muss 100 % erreichen.
- Given der laufende Server, when `/admin/` und `/healthz` aufgerufen werden, then verhalten sie sich unverändert (kein CORS auf `/admin/`).

## Implementation Notes

- OpenAPI 3.1 läuft mit oapi-codegen v2.8.0, kein Rückfall auf 3.0.3 nötig.
- Das erzeugte Gerüst importiert `github.com/oapi-codegen/runtime` nicht (keine Parameter, Strict-Server für `net/http` ohne Runtime-Middleware); `go mod tidy` ändert `go.mod`/`go.sum` daher nicht. Die Abhängigkeit kommt mit den Parametern ab Story 2.2/2.3.
- `oapi-codegen.yaml` setzt `output-options.skip-prune: true`, sonst fehlen die noch von keinem Pfad genutzten Enums `TimePrecision` und `LocationPrecision` im Gerüst und der Enum-Abgleich wäre unvollständig, und `compatibility.always-prefix-enum-values: true`, damit Konstanten wie `EventTypeMarket` und `LocationPrecisionStreet` nicht kollidieren.
- `cmd/eventstore`: `newRouter`/`newServer` nehmen ein Struct `routeHandlers{admin, public}`; `newRouteHandlers` verdrahtet Admin und Public API.
- Der Strict-Request-Fehler-Handler ist ohne Parameter nicht erreichbar; er teilt sich die 500-Funktion mit dem Response-Fehler-Handler, die per Test mit fehlschlagendem Server abgedeckt ist. Parameterfehler (400) des `std-http-server` bleiben beim generierten Standard, bis Story 2.2/2.3 Parameter einführt.
- Preflight setzt zusätzlich `Vary: Access-Control-Request-Headers`, weil `Allow-Headers` die Anfrage spiegelt.

## Spec Change Log

## Review Triage Log

| # | Layer | Befund | Verdict | Evidenz | Route |
|---|-------|--------|---------|---------|-------|
| 1 | verification-gap | `Vary: Access-Control-Request-Headers` im Preflight von keinem Test geprüft | low | Kein `Vary` in einem `*_test.go`; Entfernen der Zeile bleibt grün | patch |
| 2 | vg/edge/blind | `/v1` ohne Schrägstrich bzw. `/v1//…` → 301 des äußeren Mux ohne CORS | low | Stimmt (ServeMux-Redirect), aber kein Pfad der Spec; Abnehmer rufen `/v1/event-types` auf, Fix bräuchte zusätzliche Route plus Tests | reject |
| 3 | edge | Abbruch des Clients nach `WriteHeader(200)` → Error-Log und überflüssiges `WriteHeader(500)` | low | Real (`api.gen.go:410-411`), bei 7 Einträgen selten; Fix bräuchte Writer-Wrapper | reject |
| 4 | edge/blind | `ErrorHandlerFunc` nicht gesetzt → Parameterfehler als text/plain | false | Ohne Parameter ruft das Gerüst `ErrorHandlerFunc` nie auf (`api.gen.go:292` nur Bindung); in Implementation Notes für 2.2/2.3 vermerkt | reject |
| 5 | edge | Code ohne Beschriftung → leeres Label | false | `TestListEventTypesLabelsEveryCodeInGermanInOrder` vergleicht die vollständige Liste samt Labels | reject |
| 6 | edge/blind | Leere Liste → `"data": null` | false | `core.ListEventTypes` liefert immer sieben Einträge, eine leere Liste ist unerreichbar | reject |
| 7 | edge | Coverage-Skript außerhalb des Modulwurzelverzeichnisses | false | Skript nutzte schon vorher relative Paketpfade; CI und README rufen es im Wurzelverzeichnis auf | reject |
| 8 | edge/blind | Header-Grep durchsucht die ganze Datei statt nur vor `package` | low | Handgeschriebene Datei mit solcher Zeile fiele still aus dem Gate (widerspricht „nie durch Ausschließen“); Fix ist direkte Korrektur | patch |
| 9 | edge | Enum-Reihenfolge nicht verglichen | low | Reihenfolge der Antwort kommt aus dem Kern und ist getestet; Spec-Enum-Reihenfolge ist kein Vertragsteil | reject |
| 10 | edge | Implizite Typwiederholung im const-Block | false | Würde laut scheitern („no constants“/Mengenabweichung), nicht still | reject |
| 11 | edge | Behauptung `runtime` per `go mod tidy` | false | Implementation Notes sagen ausdrücklich, dass `go.mod` unverändert bleibt | reject |
| 12 | blind | 404/405 nicht als `components/responses` in der Spec | low | In `info` (KON-3) beschrieben; Fehlerbeispiele gehören zu Story 2.6 | reject |
| 13 | blind | `OPTIONS` ohne `Allow`-Header | low | RFC 9110 §9.3.7, 405-Pfad setzt `Allow` bereits; eine Zeile | patch |
| 14 | blind | Kein OpenAPI-Linter | low | Neue Werkzeugkette, nicht durch Intent verlangt | reject |
| 15 | blind | Label-Beispiel in der Spec kann vom Kern abweichen | low | Real, Prüfung bräuchte YAML-Parser, den die Spec bewusst vermeidet | reject |
| 16 | blind | Admin behält eigene Map | false | Map wird aus `core.ListEventTypes()` gebaut, Strings nur im Kern | reject |
| 17 | blind | `problemWriter.writeBody` liefert die Spec aus, Name passt nicht | low | Clean-Code-Regel „Namen sagen, was es tut“; Referenzprojekt, Umbenennung ist mechanisch | patch |
| 18 | blind | Sprint-Status `in-progress` vs. Spec `in-review` | false | Workflow synchronisiert den Sprint-Status beim Abschluss | reject |
| 19 | blind | `epic-2-context.md` nennt Correct Course weiter „vor 2.1“ | low | Widerspricht der Entscheidung vom 2026-10-04; direkte Korrektur | patch |
| 20 | blind | CI generiert nur `publicapi/v1` neu | low | Weitere `//go:generate` (z. B. v2) blieben ungeprüft; `./...` ist direkte Korrektur | patch |
| 21 | blind | Fehlende Tests: HEAD auf Spec, Plain-`OPTIONS`, nil-Logger | low | Plain-`OPTIONS` deckt `TestPreflightWithoutRequestHeadersAllowsNoHeaders` ab; HEAD-Verhalten ist net/http; `Config{}` ohne Logger ruft niemand auf | reject |

## Verification

**Commands:**
- `go generate ./internal/adapter/publicapi/v1/... && git status --porcelain` -- nur erwartete Dateien geändert, zweiter Lauf ohne Diff
- `CI= go test ./...` -- grün, inkl. Archtest und Enum-Abgleich
- `bash scripts/check-coverage.sh` -- 100 %, generierter Code ausgenommen
- `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./...` -- ohne Befund
- Postgres-Tests mit `-p 1` für `cmd/eventstore` -- grün
