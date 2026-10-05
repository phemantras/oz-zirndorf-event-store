---
title: 'Story 2.6: Lesbare API-Dokumentation und Abnahme'
type: 'feature'
created: '2026-10-05'
status: 'done'
baseline_commit: '6c8fa6f00fcb5b75a2ab8b30f3cb997d240222e5'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-2-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Die Spec `api/v1/openapi.yaml` ist nur als YAML abrufbar; eine lesbare Doku unter `/v1/docs` fehlt (NFR-3, ENT-14), einzelne Felder haben kein Beispiel, Zeit- und Ortsgenauigkeit sind ohne Beispiel erklärt, und SM-1 (Weihnachtsmarkt per Zeitraumabfrage) ist nicht als Abnahmetest belegt. Es gelten die ACs von Story 2.6 in `epics.md`.

**Approach:** Redoc 2.5.4 (`redoc.standalone.js`, unverändert aus npm) und eine kleine HTML-Seite liegen eingebettet in `internal/adapter/publicapi/v1/static`; der Handler liefert `/v1/docs` und das Skript mit offenem CORS. Die Spec wird um fehlende Beispiele und Erklärungen ergänzt. Ein Postgres-Abnahmetest in `cmd/eventstore` speichert den Weihnachtsmarkt 2026 über `SaveEvent` und prüft `GET /v1/events?from=2026-11-29&to=2026-12-24` bei fester `Clock`.

## Boundaries & Constraints

**Always:** Kein CDN, keine externe Ressource auf der Doku-Seite. Die Seite verlinkt `/v1/openapi.yaml` sichtbar. Doku-Pfade durchlaufen `readOnlyCORS` wie alle `/v1`-Pfade. Spec-Texte englisch. Test-first, 100 % für `publicapi/v1`. Nach Spec-Änderung `go generate ./...`, der generierte Code ändert sich höchstens in Kommentaren.

**Never:** Keine Änderung an Pfaden, Parametern, Feldnamen, Enum-Codes oder Schemas der Spec (nur `description`/`example`/`examples`). Kein `/v1/import-v1.schema.json` (Story 3.1). Keine neue Go-Abhängigkeit (kein YAML-Parser). Keine Änderung an Kern, Postgres-Adapter oder Admin. Die Prüfung durch ein OZ-Mitglied (SM-4) ist keine Abschlussbedingung.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Doku-Seite | `GET /v1/docs` | 200 `text/html; charset=utf-8`, lädt `docs/redoc.standalone.js` relativ, `<redoc spec-url>` und `<a>` auf `openapi.yaml`, CORS `*` | N/A |
| Skript | `GET /v1/docs/redoc.standalone.js` | 200, JavaScript-Content-Type, Bytes = eingebettete Datei | N/A |
| HEAD | `HEAD /v1/docs` | 200 ohne Body | N/A |
| Schreibmethode | `POST /v1/docs` | 405 Problem Details | wie bisher |
| Unbekannt | `GET /v1/docs/nope` | 404 Problem Details | wie bisher |
| Abnahme SM-1 | Weihnachtsmarkt gespeichert, `Clock` 2026-11-20 | Antwort enthält ihn mit Zeitraum, Ort mit Koordinaten, `startPrecision`, `endPrecision`, `location.precision`, `source` | N/A |

**Entscheidung (Mensch, 2026-10-05):** Volle Spec trotz ca. 2.000 Tokens.

**Entscheidung (Agent, 2026-10-05):** Der Weihnachtsmarkt wird wie im echten Bestand je zusammenhängendem Block erfasst (ein Event je Adventswochenende, Programm im Ablaufplan). Fixture und das Beispiel der Spec folgen diesem Muster.

</frozen-after-approval>

## Code Map

- `internal/adapter/publicapi/v1/handler.go:69` -- `newHandler`: Routen `GET /v1/docs` und `GET /v1/docs/redoc.standalone.js` per `http.ServeFileFS` aus eingebettetem FS registrieren, vor dem Catch-all `basePath+"/"`. Pfade als Konstanten neben `specPath`.
- `internal/adapter/publicapi/v1/static/` (neu) -- `docs.html` (Titel, Link auf `openapi.yaml`, `<redoc spec-url="openapi.yaml">`, `<script src="docs/redoc.standalone.js">`), `redoc.standalone.js` aus `redoc@2.5.4/bundles/`, `LICENSE` (MIT, aus dem Paket). Neue Datei `static.go` mit `//go:embed static`.
- `internal/adapter/publicapi/v1/handler_test.go` -- Muster `TestServesTheEmbeddedSpec` (`:117`), `TestPreflightIsAnsweredOnEveryPath` (`:132`) um Doku-Pfade erweitern.
- `api/v1/openapi.yaml` -- `info.description`: Abschnitt „Machine-readable contract“ nennt `/v1/docs`; Zeitgenauigkeit mit Beispiel je Code (z. B. „starts 19:00“ → `exact`), Ortsgenauigkeit mit Beispiel je Code; `allDay`-Regel inkl. Ablehnung gemischter Angaben (AD-4, Spine Z. 67). `Event.endPrecision` `example`, `timetable` `example`. Beispiel in `/events` (`:175`) auf ein Adventswochenende mit Ablaufplan umstellen.
- `cmd/eventstore/acceptance_test.go` (neu) -- Muster `cleanup_test.go:39` (`testDatabaseURL`, `insertCleanupLocation`-Art, `fixedClock`, `publicapi.NewHandler`). Ort „Marktplatz Zirndorf“ mit Adresse, Koordinaten, `street`; Event per `SaveEvent` mit Quelle; nach dem Test aufräumen.
- `cmd/eventstore/health_test.go:99` -- `/v1/docs` in die Pfadliste des Routers.
- `.gitattributes` -- `internal/adapter/publicapi/v1/static/redoc.standalone.js -text` wie Leaflet.
- `README.md:55`, `:5` -- `/v1/docs` nennen; Abschnitt zur Abnahme: SM-4 als manueller Schritt (OZ-Mitglied sieht `/v1/docs` durch), SM-1 in Produktion folgt in Story 3.5; Redoc-Herkunft wie Leaflet (`README.md:134`).

## Nahtstellen

- `publicapi/v1.newHandler` -- 2.1 -- Catch-all 404, 405 für Schreibmethoden, offenes CORS und Preflight gelten unverändert und jetzt auch für die Doku-Pfade; `/v1/openapi.yaml` bleibt byte-gleich zur eingebetteten Spec -- `TestUnknownPathsAreNotFound`, `TestWriteMethodsAreRejectedOnEveryPath`, `TestPreflightIsAnsweredOnEveryPath`, `TestEveryAnswerAllowsAnyOrigin`, `TestServesTheEmbeddedSpec`.
- `api/v1/openapi.yaml` → `api.gen.go` -- 2.1, 2.3, 2.4 -- nur Texte und Beispiele ändern sich; generierte Typen, Enum-Abgleich und die wörtlich zitierten Fehlerbeispiele (`events_test.go:263`) bleiben gültig -- `go generate`-Diff nur Kommentare, `enum_test.go`, `events_test.go`.
- `cmd/eventstore.newRouter` -- 2.1 -- `/v1/…` geht weiter an die öffentliche API -- `TestRouterMountsPublicAPIBelowV1` mit Doku-Pfaden.

## Tasks & Acceptance

**Execution:**
- [x] `internal/adapter/publicapi/v1/handler_test.go` -- Tests zuerst für alle Doku-Zeilen der Matrix inkl. Preflight; dann `static/`, `static.go`, `handler.go`.
- [x] `.gitattributes` -- Redoc-Datei vor Zeilenende-Umwandlung schützen.
- [x] `api/v1/openapi.yaml` -- Beschreibungen und Beispiele ergänzen; `go generate ./...`.
- [x] `cmd/eventstore/acceptance_test.go`, `health_test.go` -- Abnahmetest SM-1 und Router-Pfad.
- [x] `README.md` -- Doku-Pfad, Redoc-Herkunft, Abnahme SM-1/SM-4.

**Acceptance Criteria:**
- Given das laufende Programm, when ich `/v1/docs` im Browser öffne, then rendert Redoc die Spec ohne Anfrage an fremde Hosts, und `/v1/openapi.yaml` ist verlinkt.
- Given die Spec, when ich sie durchsehe, then haben alle Endpunkte, Parameter und Felder eine englische Beschreibung und ein Beispiel (direkt oder über das referenzierte Schema), Zeit-/Ortsgenauigkeit sind mit Bedeutung und Beispiel erklärt, ebenso Vorbei-Regel, `effective*`, `archived`, Standardwerte von `from`/`to` je Endpunkt und `allDay`, und es gibt Fehlerbeispiele.

## Implementation Notes

- Redoc 2.5.4 lädt im Footer ein Logo von `cdn.redoc.ly`; eine CSP im Meta-Tag von `docs.html` (`img-src`/`font-src`/`connect-src 'self'`) verhindert die Anfrage. Im headless Edge geprüft.
- Content-Types werden explizit gesetzt, weil die Endungs-Zuordnung je Betriebssystem abweicht.
- Zusätzlich `redoc.standalone.js.LICENSE.txt` (Hinweise der gebündelten Bibliotheken) vendort.
- Die Spec ergänzt Fehlerbeispiele `unknownType`, `emptyPeriod` und ein 405-Beispiel in KON-3.
- Nahtstellen-Abschnitt beim Review nachgetragen (Pflicht nach Persistent Facts, beim Planen vergessen).

## Spec Change Log

## Review Triage Log

| # | Layer | Befund | Verdict | Evidenz | Route |
|---|-------|--------|---------|---------|-------|
| 1 | vg/seam/blind | Fehlerbeispiele `unknownType`, `emptyPeriod`, 405 nicht wörtlich gegen die API geprüft | medium | Nur `beforeToday`/`afterNow` wörtlich (`events_test.go:262`); Umformulieren von `detailEmptyPeriod` bliebe grün | patch |
| 2 | vg/blind/edge | CSP der Doku-Seite von keinem Test festgehalten | low | Entfernen des Meta-Tags bliebe grün, Logo käme wieder von `cdn.redoc.ly`; Testzeile ist direkte Ergänzung | patch |
| 3 | edge | `ServeFileFS` liefert 416/412 als text/plain statt Problem Details | low | `Range`/`If-Match` werden von `ServeFileFS` ausgewertet; Bytes direkt schreiben wie bei der Spec ist direkte Vereinfachung | patch |
| 4 | blind/edge | `Event.timetable`-Beispiel: Christkind `endTime` null statt 21:00 | low | widerspricht `/events`-Beispiel und Fixture | patch |
| 5 | blind | README-Beispiel: Ort und Quelle weichen von Spec und Fixture ab | low | „Marktplatz“, 49.4425/10.9547 vs. „Marktplatz Zirndorf“, 49.4427/10.9545 | patch |
| 6 | blind | „Abnahme“ vor dem CI-Absatz eingefügt | low | CI-Absatz steht jetzt unter „Abnahme“ | patch |
| 7 | blind | README/Test sagen „den Weihnachtsmarkt“, Fixture hat zwei von vier Wochenenden | low | direkte Umformulierung | patch |
| 8 | blind | `allDay`-Prosa „all-day start with an end time“ nicht ausdrückbar | low | `allDay` gilt gemeinsam; direkte Umformulierung | patch |
| 9 | blind | Kommentar von `serveStaticFile` missverständlich | low | wird mit #3 neu geschrieben | patch |
| 10 | blind/edge | Kein Caching/ETag/Kompression für das 1-MB-Bundle | low | nur Bandbreite bei seltenem Doku-Aufruf; Fix ist neues Verhalten | reject |
| 11 | blind/edge | CSP ohne `default-src` | low | heute einzige externe Ladung (Logo) blockiert, Spec ist eigener Inhalt; strengere Policy bräuchte Browserprüfung der Redoc-Ausnahmen | reject |
| 12 | blind | Kein Test, dass der Markt an Werktagen zwischen Wochenenden fehlt | false | Überschneidungsfilter ist in 2.3 getestet; keine Anforderung dieser Story | reject |
| 13 | blind | `listedEventsByTitle` überschreibt gleiche Titel | false | Test entfernt Ort samt Events vor und nach dem Lauf | reject |
| 14 | blind | Keine Prüfsumme des Bundles, `-text` nicht für LICENSE-Dateien | low | `cmp` gegen npm bei Umsetzung; LF-Dateien, `text=auto` ändert keine Leerzeichen | reject |
| 15 | blind | Tests nutzen Literale statt Konstanten | false | Literale sind unabhängiges Orakel, wie `application/yaml` im bestehenden Test | reject |
| 16 | blind | HEAD nur für `docsPath` getestet | low | gleiche Route-Mechanik wie GET; Zusatztest ohne Befund | reject |
| 17 | blind | Status Spec vs. Sprint-Status uneinheitlich | false | Sprint-Status wird am Ende des Workflows synchronisiert | reject |
| 18 | vg | Relative URLs der Seite nur über Literal an die Route gebunden | low | Kopplung hält heute (Seam-Review bestätigt), bricht nur bei Umbenennung | reject |

## Design Notes

Relative URLs auf der Seite (`openapi.yaml`, `docs/redoc.standalone.js`) lösen von `/v1/docs` korrekt auf und halten die Seite vom Host unabhängig. Die Vollständigkeit der Spec prüft kein Test, weil ein YAML-Parser eine neue Abhängigkeit wäre; sie wird im Review gegen die AC geprüft.

## Verification

**Commands:**
- `go generate ./...`; `git status --porcelain` -- nur erwartete Dateien
- `CI= go test ./...` -- grün
- `bash scripts/check-coverage.sh` -- 100 %
- `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./...` -- ohne Befund
- `EVENTSTORE_TEST_DATABASE_URL=… go test -p 1 ./internal/adapter/postgres/... ./cmd/eventstore/...` -- grün

**Manual checks:**
- Programm starten, `/v1/docs` öffnen: Spec lesbar, Netzwerk-Tab ohne fremde Hosts.
