---
title: 'Story 2.4: Archiv abfragen'
type: 'feature'
created: '2026-10-04'
status: 'done'
baseline_commit: '93d5eb34758958e0c4be41e20ce17bef7a304783'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-2-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Die Public API liefert nur aktive Events. Vergangene Events sind nicht abrufbar, und `/v1/events` verweist bei `to` vor heute auf ein noch fehlendes `/v1/archive/events`. Die Akzeptanzkriterien der Story 2.4 in `epics.md` gelten vollständig.

**Approach:** Spec erweitert um `GET /archive/events` mit `from`, `to`, `type` und derselben Leseform `Event`. Der Kern normalisiert den Filter wie in 2.3, aber mit eigenen Standardwerten (fehlendes `from` offen, fehlendes `to` = `now`), holt über dasselbe Prädikat und behält nur Events mit `effectiveEnd <= now`; `ListArchivedEvents` sortiert nach `effectiveStart` absteigend, Gleichstand nach `ID`.

## Boundaries & Constraints

**Always:** Normalisierung von Datum/Zeitpunkt, Typprüfung und 400-Details aus 2.3 wiederverwenden, nicht kopieren. Zeit nur aus `Clock`, einmal pro Abfrage gelesen. Archiv-Bedingung `Period.IsOver(now)` im Kern, nie per SQL; Typfilter und Sortierung im Kern. Leseform unverändert, `archived: true`. Test-first, 100 % für `core` und `publicapi/v1`; generierter Code nur per Generator.

**Never:** Keine `archived_at`-Spalte, keine Migration, kein Bereinigungsjob (2.5). Keine Änderung an `ListActiveEvents`-Verhalten. Keine neue Abhängigkeit. Kein Einzelabruf, keine Paginierung.

## I/O & Edge-Case Matrix

Uhr: 2026-12-24 12:00 Europe/Berlin.

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Ohne Parameter | Bestand mit vergangenen, laufenden, künftigen Events | alle mit `effectiveEnd <= now`, `effectiveStart` absteigend | N/A |
| Gerade vorbei | Event endet 11:59, keine Bereinigung | enthalten | N/A |
| Endet jetzt | Event endet 12:00 | enthalten; in `/v1/events` nicht | N/A |
| Läuft noch | Event endet 12:01 | nicht enthalten | N/A |
| Nur `to` | `to=2026-06-30` | vergangene Events mit `effectiveStart < 2026-07-01 00:00`, `from` offen | N/A |
| Nur `from` | `from=2026-12-01` | vergangene Events mit `effectiveEnd > 01.12. 00:00`, bis `now` | N/A |
| Zukunft | `from=2027-01-01&to=2027-01-31` | 200, leere Liste (kein Event dort ist vorbei) | N/A |
| Typen | `type=market&type=market` | nur `market`, keine Doppelten | N/A |
| Ungültig | `from=24.12.2026`, Zeitpunkt ohne Offset, `type=foo`, `type=`, `from=` | 400, `detail` nennt den Parameter | kein Repository-Aufruf |
| Leeres Intervall | `from=2026-12-28&to=2026-12-27` | 400 wie 2.3 | kein Repository-Aufruf |
| Genau eine Liste | Kern: `ListActiveEvents` ohne Untergrenze vs. `ListArchivedEvents` ohne Filter; API: `/v1/events?from=1900-01-01` vs. `/v1/archive/events` | jedes Event in genau einer Liste | N/A |
| Nach jetzt | nur `from=2027-01-01` (`[from, now)` leer) | 400, `detail`: Archiv enthält nur vergangene Events, Verweis auf `/v1/events` | kein Repository-Aufruf |
| Repository-Fehler | Port liefert Fehler | 500 Problem, geloggt | N/A |

**Entscheidungen (Mensch, 2026-10-04):** Nur `from` nach `now` → 400 mit neuem Problem `afterNow` am Feld `from` (Spiegel zu „nur `to` vor heute“). Liegen `from` und `to` beide in der Zukunft, 200 mit leerer Liste (wie 2.3). Volle Spec trotz ca. 2.100 Tokens.

</frozen-after-approval>

## Code Map

- `internal/core/event_query.go` -- `ListActiveEvents`, `normalizeActiveFilter`, `emptyPeriodProblem`, `parseFilterBound`, `parseEventTypeFilter`, `listedEventOf`, `compareByStartThenID` (2.3). Neu `ListArchivedEvents(ctx, Clock, EventFilter)` und `normalizeArchiveFilter`; gemeinsames Parsen (Bounds + Typen → Problems) aus `normalizeActiveFilter` herausziehen; Schleife „Typ filtern, Ort zuordnen, `listedEventOf`“ als gemeinsame Funktion mit Prädikat (aktiv: alle, Archiv: `Period.IsOver(now)`). `Overlap.Lo` wird `*time.Time` (nil = offen), Kommentar anpassen.
- `internal/core/errors.go` -- neu `ProblemAfterNow` (Feld `from`), analog `ProblemBeforeToday`; `emptyPeriodProblem` liefert es, wenn `to` fehlt (= `now`).
- `internal/core/event_service.go:20` -- Port-Kommentar von `ListOverlapping` um offenes `Lo` ergänzen; `ListEvents` (Admin) nicht ändern.
- `internal/core/event_service_test.go:63` -- Fake `ListOverlapping` an `Lo *time.Time` anpassen; `event_query_test.go` `assertOverlap` ebenso.
- `internal/adapter/postgres/queries/events.sql:9` -- `ListEventsOverlapping`: `(sqlc.narg(lo)::timestamptz IS NULL OR effective_end > sqlc.narg(lo))`; `db/` mit sqlc 1.31.1 neu erzeugen (kein gcc: Release-Binary `sqlc_1.31.1_windows_amd64` in den Scratchpad laden).
- `internal/adapter/postgres/events.go:50` -- `ListOverlapping` setzt `Lo` als nullbaren Parameter (wie `Hi`); `events_test.go` um offenes `lo` ergänzen.
- `api/v1/openapi.yaml` -- `type`-Parameter nach `components/parameters/EventTypeFilter` ziehen und in beiden Pfaden per `$ref`; neuer Pfad `/archive/events` (`operationId: listArchivedEvents`, eigene `from`/`to`-Beschreibungen mit Standardwerten, Beispiel mit `archived: true`, 200/400/500); `BadRequest`-Beschreibung verallgemeinern; KON-6 bleibt. `go generate ./...`.
- `internal/adapter/publicapi/v1/events.go` -- `EventLister` um `ListArchivedEvents`; Handler `ListArchivedEvents` analog `ListEvents`, gemeinsame Antwort-Bildung; `eventFilterOf` für beide Param-Typen (gleiche Felder, generische Hilfsfunktion oder zwei dünne Adapter). `filterProblemDetails` um `{from, afterNow}` mit Verweis auf `/v1/events`; `BadRequest`-Beispiel `afterNow` in der Spec.
- `internal/adapter/publicapi/v1/events_test.go` -- `recordingLister`, `coreEvents`/`overlapRepo` (echter Kern mit Fake-Repo) wiederverwenden.
- `cmd/eventstore/health_test.go:99` -- `/v1/archive/events` in den Router-Test. `main.go` braucht keine Änderung (`cases.events` ist `*core.EventService`).
- `README.md` -- `/v1/archive/events` ergänzen.

## Tasks & Acceptance

**Execution:**
- [x] `internal/core/event_query_test.go`, `event_query.go`, `event_service.go`, `event_service_test.go`, `errors.go` -- Test zuerst mit fester `Clock`: jede Kern-Zeile der Matrix, Sortierung absteigend mit Gleichstand nach `ID`, `Overlap` an das Repo (`Lo` nil ohne `from`, `Hi` = `now` ohne `to`), `archived: true`, Genau-eine-Liste-Vergleich, Fehler weitergereicht; dann implementieren und `ListActiveEvents` auf die gemeinsamen Teile umstellen (bestehende Tests bleiben grün).
- [x] `internal/adapter/postgres/queries/events.sql`, `db/*`, `events.go`, `events_test.go` -- Postgres-Test zuerst: `Lo` nil liefert auch früheste Events; Grenzen aus 2.3 bleiben.
- [x] `api/v1/openapi.yaml` -- Pfad, Parameter (englisch, mit Beispielen), Fehlerbeispiel; `go generate ./...`.
- [x] `internal/adapter/publicapi/v1/events_test.go`, `events.go` -- Test zuerst je HTTP-Zeile der Matrix, Leseform mit `archived: true`, API-Vergleich `/v1/events?from=1900-01-01` vs. `/v1/archive/events` über den echten Kern; dann implementieren.
- [x] `cmd/eventstore/health_test.go` -- Router-Test `GET /v1/archive/events`.
- [x] `README.md` -- Endpunkt mit Standardwerten.

**Acceptance Criteria:**
- Given eine Antwort von `GET /v1/archive/events`, when ihr JSON geprüft wird, then hat jedes Event dieselben Schlüssel wie in `/v1/events`, `archived: true`, und keine internen Werte.
- Given der Postgres-Test, when er gegen PostgreSQL 18 läuft, then bestätigt er das offene `lo`.

## Implementation Notes

- Gleichstand bei gleichem `effectiveStart` im Archiv nach `ID` aufsteigend, wie in der aktiven Liste.
- `from` genau gleich `now` ohne `to` ergibt `[now, now)` und damit 400 `afterNow`.
- API-Tests zum Archiv in eigener Datei `internal/adapter/publicapi/v1/archive_test.go`.
- Postgres-Tests (offenes `lo`) konnten lokal nicht laufen (kein Docker); sie laufen im CI-Job „PostgreSQL tests“.

## Spec Change Log

## Review Triage Log

| # | Layer | Befund | Verdict | Evidenz | Route |
|---|-------|--------|---------|---------|-------|
| 1 | vg | Reihenfolge der Pfade in den `beforeToday`/`afterNow`-Details ungeprüft; vertauschte `Sprintf`-Argumente blieben grün | medium | Tests prüfen nur `strings.Contains` auf Parameter und einen Pfad | patch |
| 2 | blind | README: allgemeiner Absatz „nur aktive Events, aufsteigend“ steht nach dem Archiv-Absatz und wirkt wie für beide | low | Absatzreihenfolge in `README.md`; Fix ist Verschieben | patch |
| 3 | blind | Archiv-Bedingung nicht in SQL | false | Bewusste Spec-Entscheidung (Design Notes), Grenze = `IsOver` wie `archived` | reject |
| 4 | blind | `hi` bei `to` in der Zukunft nicht auf `now` begrenzt | low | Nur Mehrladen laufender/künftiger Events, Hobby-Bestand; Ergebnis korrekt | reject |
| 5 | blind/edge | Archiv-Antwort unbegrenzt | false | Intent schließt Paginierung aus (Never) | reject |
| 6 | blind/edge | `IS NULL OR` verhindert Indexnutzung | low | Wie 2.3 #7/#8: kein Index vorhanden, Hobby-Bestand | reject |
| 7 | blind | Aufteilung aktiv/Archiv bei Uhr mit Sekunden ungetestet | false | `IsOver` ist `!now.Before(End)` = `End <= now`, exaktes Komplement zu `effective_end > now` für jede Uhrzeit | reject |
| 8 | blind | Cast `ListEventsParams(request.Params)` koppelt generierte Typen | low | Abweichung scheitert laut beim Kompilieren | reject |
| 9 | blind | `from`/`to` in der Spec doppelt beschrieben | low | Standardwerte unterscheiden sich; Drift nur Doku | reject |
| 10 | blind | Gleichstand-Reihenfolge nicht im Vertrag | low | ID ist intern, betrifft auch `/v1/events` (vorbestehend) | reject |
| 11 | blind | Test meldet bei fehlendem Kontext „clock“ | low | `archive_test.go:57`; direkte Korrektur | patch |
| 12 | blind | `EventLister`-Kommentar ungrammatisch | low | „is the core queries“; direkte Korrektur | patch |
| 13 | blind | Diff ohne Spec und Sprint-Status | false | Bewusst: Spec ist Claims-Datei, Status ist Workflow | reject |
| 14 | blind | Kein Postgres-Test für `/v1/archive/events` über echte Verdrahtung | low | Repo-Test prüft offenes `lo`, Router-Test den Pfad | reject |
| 15 | edge | Fehlender Ort eines laufenden Events → 500 im Archiv | false | FK `events_location_id_fkey … ON DELETE RESTRICT` (Migration 00004) | reject |

## Design Notes

Archiv-Bedingung im Kern statt im Prädikat: `effectiveEnd <= now` lässt sich nicht als Überschneidung `[lo, hi)` ausdrücken. Das Repo liefert die Überschneidung mit `hi = to` bzw. `now`; der Kern verwirft laufende Events mit `IsOver(now)`. Die Grenze ist so dieselbe Funktion, die `archived` setzt, und die Genau-eine-Liste-Eigenschaft folgt direkt (`ListActiveEvents` nutzt `lo' = max(lo, now)` ⇔ `effectiveEnd > now`).

## Verification

**Commands:**
- `sqlc generate` (1.31.1) und `go generate ./... && git status --porcelain` -- zweiter Lauf ohne Diff
- `CI= go test ./...` -- grün, inkl. Archtest und Enum-Abgleich
- `bash scripts/check-coverage.sh` -- 100 %
- `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./...` -- ohne Befund
- Postgres-Tests mit `-p 1` für `internal/adapter/postgres/...` und `cmd/eventstore/...` -- grün (lokal nur mit Docker)
