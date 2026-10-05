---
title: 'Story 2.7: Events mit „prüfen“ öffentlich ausblenden'
type: 'feature'
created: '2026-10-05'
status: 'done'
baseline_commit: 'f6a34164fc565f029741e77acc9bb05ca6e85e41'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-2-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Ein Event, dessen Neuberechnung beim Start gescheitert ist, trägt im Admin „prüfen“, wird aber von `/v1/events` und `/v1/archive/events` mit veraltetem Zeitraum und womöglich falschem `archived` ausgeliefert (ENT-5, AD-16). Es gelten die ACs von Story 2.7 in `epics.md`.

**Approach:** Die gemeinsame Kern-Abfrage `listMatchingEvents` lässt Events aus, die der `EventService` als „prüfen“ führt, also dieselbe Menge, die `ListEvents` für die Admin-Liste liest. Spec-Texte und README nennen die Ausnahme von „jedes Event in genau einer Liste“.

## Boundaries & Constraints

**Always:** Auswahl nur im Kern über `needsReview`. Test-first, feste `Clock`, Fake-Repos, 100 % für `core`. Spec-Texte englisch; nach Spec-Änderung `go generate ./...`, generierter Code ändert sich höchstens in Kommentaren/eingebetteter Spec. Das AC aus 2.5 „Antworten vor und nach dem Job identisch“ bleibt unverändert grün.

**Never:** Keine Migration, keine Spalte, keine SQL-/sqlc-Änderung, kein neuer Port, kein Filter im Handler `publicapi/v1`. Kein neues Feld im Schema `Event` und keine Änderung an Pfaden, Parametern oder Enum-Codes (NFR-2). Keine Änderung an Admin-Liste, `SaveEvent`, `DeleteEvent`, `RecomputeDerived`.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Aktiv, markiert | aktives Event mit „prüfen“; ohne Filter, mit `from`/`to`, mit `type` | fehlt in `ListActiveEvents` | N/A |
| Archiv, markiert | vergangenes Event mit „prüfen“; ohne und mit Filter | fehlt in `ListArchivedEvents` | N/A |
| Admin | dasselbe Event | `ListEvents` liefert es mit `NeedsReview` | N/A |
| Gespeichert | markiertes Event per `SaveEvent` erfolgreich gespeichert | mit neuen Werten in genau einer Liste | gescheitertes Speichern lässt es ausgeblendet |
| Gelöscht | markiertes Event per `DeleteEvent` gelöscht | in keiner Liste | N/A |
| Partition | Bestand mit markierten und nicht markierten Events, mehrere `Clock`-Zeitpunkte | jedes nicht markierte in genau einer, kein markiertes in einer Liste | N/A |

**Entscheidung (Mensch, 2026-10-05):** Volle Spec trotz ca. 2.200 Tokens.

</frozen-after-approval>

## Code Map

- `internal/core/event_query.go:154` -- `listMatchingEvents`: in der Schleife `s.needsReview(event.ID)` überspringen (vor oder nach dem Typ-Test, vor dem Ortsabgleich). Doc-Kommentare von `ListActiveEvents` (`:90`), `ListArchivedEvents` (`:113`) und `listMatchingEvents` um „Events marked for review are left out“ ergänzen.
- `internal/core/event_service.go:438` -- `markForReview`/`needsReview` wiederverwenden, nicht ändern. Die IDs stammen aus `events.List` und `ListOverlapping` desselben Repos, die Schreibweise passt.
- `internal/core/event_query_test.go:636` -- `TestEveryEventIsInExactlyOneOfActiveAndArchive` erweitern: je `Clock` zusätzlich ein aktives und ein vergangenes Event per `service.markForReview` markieren (beide Abfragen über denselben Service). Helfer `listActiveTitles`/`listArchivedTitles` (`:29`) bauen je einen Service; für markierte Fälle einen Service teilen. Fixtures `archiveEvents` (`:427`), `christmasNoon`, `clockAt`, `newQueryService` (`:24`).
- `internal/core/event_service_test.go` -- Muster für Save/Delete mit `fakeTx` (`:211`, `:745`, `:890`), `brokenEvent` (`:452`), `assertNeedsReview` (`:712`). Ein Test markiert über `RecomputeDerived` mit `brokenEvent`, damit der echte Weg abgedeckt ist.
- `api/v1/openapi.yaml:96` -- Abschnitt „Time model“, neuer Punkt (z. B. „Events awaiting correction“): Events, deren gespeicherte Angaben nach einer Regeländerung nicht neu berechnet werden konnten, erscheinen in keiner der beiden Listen, bis sie korrigiert sind. `:261` -- Satz „every event is in exactly one of …“ um diese Ausnahme ergänzen.
- `README.md:53`, `:205` -- „genau einer der beiden Listen“ um die Ausnahme ergänzen; im Absatz zur Neuberechnung: markierte Events fehlen öffentlich bis zum erfolgreichen Speichern.
- `cmd/eventstore/main_test.go:221` -- `TestRunLogsEventsWhoseRecomputationFailsAndStartsAnyway`: zusätzlich `/v1/events?from=1900-01-01` und `/v1/archive/events` abrufen; `brokenEventTitle` fehlt in beiden. Belegt, dass `run` der öffentlichen API denselben `EventService` gibt (`main.go:102`).

## Nahtstellen

- `core.(*EventService).listMatchingEvents` -- 2.3, 2.4 -- Filter, Standardwerte, Sortierung, Fehler bei fehlendem Ort, Leerlauf des Repos bei ungültigem Filter unverändert; neu Auslassen markierter Events -- bestehende `event_query_test.go`-Tests plus Matrix-Zeilen „Aktiv/Archiv, markiert“.
- Partitionstest im Kern -- 2.4 -- bleibt für unmarkierte Events gültig, neu kein markiertes in einer Liste -- Matrix „Partition“.
- `publicapi/v1` `TestEveryEventIsInExactlyOneOfEventsAndArchive` (`archive_test.go:187`) -- 2.4 -- unverändert grün, Handler filtert nichts.
- API-Vergleich `TestCleanupLeavesThePublicAnswersUnchanged` (`cmd/eventstore/cleanup_test.go:39`) -- 2.5 -- unverändert grün, keine Markierung im frischen Service.
- Menge „prüfen“ und ihr Leeren nach Commit (`SaveEvent`/`DeleteEvent`) -- 2.5b -- unverändert; neu sichtbar in den öffentlichen Listen -- Matrix „Gespeichert“/„Gelöscht“.
- Admin-Liste `ListEvents` -- 1.7, 2.5b -- zeigt markierte Events weiter -- Matrix „Admin“, `TestRunLogsEventsWhoseRecomputationFailsAndStartsAnyway`.
- `api/v1/openapi.yaml` → `api.gen.go` -- 2.1, 2.6 -- nur Beschreibungstexte -- `go generate`-Diff, `enum_test.go`.

## Tasks & Acceptance

**Execution:**
- [x] `internal/core/event_query_test.go` -- Tests zuerst für alle Matrix-Zeilen (Save/Delete mit `fakeTx` als eigene Tests, ggf. in `event_service_test.go`); dann `event_query.go`.
- [x] `cmd/eventstore/main_test.go` -- Postgres-Test um die öffentlichen Listen erweitern.
- [x] `api/v1/openapi.yaml` -- Ausnahme in „Time model“ und `/archive/events`; `go generate ./...`.
- [x] `README.md` -- Ausnahme an beiden Stellen.

**Acceptance Criteria:**
- Given ein beim Start als „prüfen“ markiertes Event, when das Programm läuft, then fehlt es in `/v1/events` und `/v1/archive/events`, und die Admin-Event-Liste zeigt es mit „prüfen“.
- Given die Spec, when ich „Time model“ und `/v1/archive/events` lese, then steht dort die Ausnahme, auch bei „every event is in exactly one of …“, und das Schema `Event` ist unverändert.

## Implementation Notes

## Spec Change Log

## Review Triage Log

| # | Layer | Befund | Verdict | Evidenz | Route |
|---|-------|--------|---------|---------|-------|
| 1 | edge/blind | Markierung wird je Event nach `ListOverlapping` gelesen; `SaveEvent` dazwischen liefert die alte Zeile einmal aus; Mutex je Event | low | Rennen zwischen Lesen und Commit möglich; Kopie der Menge vor `ListOverlapping` ist direkte Umstellung | patch |
| 2 | seam | Markiertes Event mit fehlendem Ort wird übersprungen statt `ErrNotFound` | low | Prüfung steht vor dem Ortsabgleich; durch FK in Produktion unerreichbar, Umstellen ist direkt | patch |
| 3 | edge/blind | Lösch-Test besteht auch ohne Feature | low | gelöschte Zeile fehlt im Fake ohnehin; Prüfung der Markierung ist direkte Ergänzung | patch |
| 4 | blind | ID-Konstanten doppeln Literale der Fixtures | low | Änderung einer Fixture-ID ließe den Partitionstest nichts markieren | patch |
| 5 | edge/blind | Postgres-Test ohne Positivkontrolle | low | leere oder kaputte Antwort bestünde; JSON-Prüfung auf `data` ist direkt | patch |
| 6 | blind | Spec-Text: Neuberechnung „wenn sich Regeln ändern“ | low | `run` berechnet bei jedem Start alles neu | patch |
| 7 | blind | README:53 nennt Löschen nicht | low | README:205 und Spec nennen es | patch |
| 8 | blind | Titel- statt ID-Prüfung im Speichern-Test | low | Fake schreibt bei `writeErr` nicht, echte Tx rollt zurück; Umbau auf IDs ist mehr als direkt | reject |
| 9 | blind | Vertrag sagt nicht, dass Fehlen keine Löschung ist und die ID gleich bleibt | false | API liefert keine IDs (NFR); Text sagt „until it is corrected“ | reject |
| 10 | blind | Admin-Liste zeigt nicht, dass das Event öffentlich fehlt | low | Intent hält die Admin-Liste unverändert; README erklärt es | reject |
| 11 | blind | Start-Log nennt die öffentliche Wirkung nicht | low | neues Verhalten, nicht verlangt | reject |
| 12 | vg | keine Lücken | false | Tests nicht vakuös, Verdrahtung per Postgres-Test belegt | reject |

## Verification

**Commands:**
- `go generate ./...`; `git status --porcelain` -- nur erwartete Dateien
- `CI= go test ./...` -- grün
- `bash scripts/check-coverage.sh` -- 100 %
- `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./...` -- ohne Befund
- `EVENTSTORE_TEST_DATABASE_URL=postgres://eventstore:eventstore@localhost:5432/eventstore_test?sslmode=disable go test -p 1 ./internal/adapter/postgres/... ./cmd/eventstore/...` -- grün
