---
title: 'Story 2.5 (Teil B): Vollständige Neuberechnung beim Start'
type: 'feature'
created: '2026-10-05'
status: 'done'
baseline_commit: 'f8b75a814b961484ed32ca224a565b5ec6718876'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-2-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Beim Start zieht die Neuberechnung nur `effective*` und `title_key` der Events nach. `name_key` der Orte bleibt veraltet, und ein Ablaufplan, der nicht mehr in den neu berechneten Zeitraum passt, fällt nicht auf (AD-16, ENT-5, ENT-17, AD-15). Es gelten die offenen ACs von Story 2.5 in `epics.md` (`name_key`, Ortskollisionen, „prüfen“ in der Ortsliste, Ablaufplan-Grenze) sowie beide `deferred-work.md`-Einträge mit `target: Story 2.5, Teil B`.

**Approach:** `LocationService.RecomputeNameKeys` berechnet `NormalizeKey(Name)` für alle Orte. Er erkennt Kollisionen vorab über alle Orte und speichert nur Schlüssel, die sich ändern und nicht kollidieren. Kollidierende Orte merkt er sich als Gruppe im Speicher. `ListLocationEntries` liefert die Markierung an die Ortsliste. `RecomputeDerived` prüft zusätzlich jeden Programmpunkt gegen den neuen Zeitraum. Der Start ruft erst die Orte, dann die Events auf, danach `MarkArchived` und den HTTP-Server.

## Boundaries & Constraints

**Always:** Start-Neuberechnung schreibt ohne `TxRunner` (einzige Ausnahme nach AD-6). Markierungen nur im Speicher, per Mutex geschützt. Speichern oder Löschen eines markierten Orts entfernt die Markierung der ganzen Kollisionsgruppe. Fehler bei Lesen/Schreiben (außer `ErrNotFound`/`ErrConflict`) und ein beendeter `ctx` brechen den Start ab, sie markieren nichts. Kern-Tests ohne Datenbank für alle Fälle, ein Postgres-Test für `name_key`. Test-first, 100 % für `core` und `admin`. Generierter Code nur per sqlc 1.31.1.

**Never:** Keine Migration. Keine Änderung an `/v1` oder `openapi.yaml`. Keine Advisory-Sperre (Story 3.3). Kein tx-Kern für `SaveLocation` (G1, Story 3.3). `ListLocations` (Auswahl im Event-Formular) bleibt unverändert. Keine Änderung des Namens eines Orts, nur seines Schlüssels.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Veralteter Schlüssel | Ort „Marktplatz“, `name_key` = `alt` | Schlüssel `marktplatz` gespeichert, keine Markierung | N/A |
| Schlüssel aktuell | `name_key` = `NormalizeKey(name)` | kein Schreiben | N/A |
| Kollision | A und B ergäben denselben neuen Schlüssel (auch wenn A ihn schon hat) | beide behalten ihren Schlüssel, ein Fehler mit beiden IDs (sortiert), beide „prüfen“ | Start läuft weiter |
| Dreifach | A, B, C ergäben denselben Schlüssel | ein Fehler mit drei IDs, alle drei markiert | Start läuft weiter |
| DB meldet Konflikt | `UpdateNameKey` liefert `ErrConflict` (z. B. Kette, Schlüssel ist noch belegt) | Ort und Inhaber des Schlüssels (`FindByNameKey`) als Gruppe markiert und gemeldet | Start läuft weiter |
| Gruppe aufgelöst | A aus Gruppe {A,B} per `SaveLocation`/`DeleteLocation` erfolgreich | A und B ohne „prüfen“ | gescheitertes Speichern lässt die Markierung |
| Ort gelöscht | `UpdateNameKey` liefert `ErrNotFound` | übersprungen | N/A |
| Ablaufplan außerhalb | neuer Zeitraum schließt Programmpunkt aus | Zeitraum bleibt gespeichert, `title_key` wird aktualisiert, Fehler nennt Event-ID, Punkt-ID und Feld, Event „prüfen“ | Start läuft weiter |
| Ablaufplan passt | alle Punkte im neuen Zeitraum | Verhalten wie bisher | N/A |

**Entscheidung (Mensch, 2026-10-05):** Volle Spec trotz ca. 3.000 Tokens.

</frozen-after-approval>

## Code Map

- `internal/core/location_service.go` -- Port `LocationRepo` um `UpdateNameKey(ctx, id, nameKey string) error` erweitern (`ErrNotFound`, `ErrConflict`; Port-Kommentar „nur LocationService“ ergänzen). `LocationService` bekommt `mu sync.Mutex` und `inReview map[string][]string` (ID → sortierte Gruppen-IDs), Muster wie `EventService.markForReview/clearReview/needsReview` (`event_service.go:417`). Neu: `RecomputeNameKeys(ctx) ([]LocationRecomputeFailure, error)`, `ListLocationEntries(ctx) ([]LocationListEntry, error)` (sortiert per `SortLocations`). `SaveLocation`/`DeleteLocation` leeren die Gruppe nur bei Erfolg.
- `internal/core/location.go` oder `location_service.go` -- Typen `LocationListEntry{Location; NeedsReview bool}`, `LocationRecomputeFailure{LocationIDs []string; Err error}`; `Err` wrappt `ErrConflict` und nennt den Schlüssel.
- `internal/core/event_service.go:371` -- `recomputeEvent`: Ist der neue Zeitraum gültig, prüft er die Programmpunkte (`sortedTimetable`) mit `dayEntryProblems`/`timedEntryProblems` (`timetable.go:205`, `:222`) und `timetableEntryFieldsAt`. Beim ersten Problem bleibt `derived.Period` der gespeicherte Wert, `ruleErr` nennt Punkt-ID, Feld und Problem. Doc-Kommentar von `RecomputeDerived` ergänzen.
- `internal/adapter/postgres/queries/locations.sql`, `db/*`, `locations.go` -- Query `UpdateLocationNameKey :execrows` (`UPDATE locations SET name_key = $2 WHERE id = $1`), Repo-Methode `UpdateNameKey` mit `parseID`, `translateError` (ErrConflict), 0 Zeilen → `ErrNotFound`.
- `internal/adapter/admin/locations.go:19` -- `LocationUseCases` um `ListLocationEntries`; `showLocations` (`:172`) nutzt sie; `locationRow` bekommt `NeedsReview`. Template `templates/locations.html`: hinter dem Namen `<strong class="review">prüfen</strong>` wie `events.html:19`.
- `cmd/eventstore/recompute.go` -- Interface `nameKeyRecomputer`; `recomputeDerived` ruft erst Orte, dann Events; loggt jede Ortsgruppe mit Key `locationIds`; Info-Log mit Anzahl beider Fehlerarten. `main.go:83` übergibt den `LocationService` (vor dem Aufruf erzeugen, in `useCases` wiederverwenden).
- Fakes: `internal/core/location_service_test.go:22`, `event_service_test.go` (falls `LocationRepo`-Fake dort), Admin-Fake der `LocationUseCases`, `cmd/eventstore/recompute_test.go`.

## Nahtstellen

- `core.(*EventService).RecomputeDerived`/`recomputeEvent` -- 1.6, 1.7, 1.10, A1 -- schreibt weiter nur Geändertes, `title_key` auch bei scheiterndem Zeitraum, Abbruch bei Speicher-/Kontextfehlern -- bestehende `TestRecomputeDerived*` plus neue Ablaufplan-Tests.
- `core.(*LocationService).SaveLocation` -- 1.4, 1.12 -- Validierung, `LocationConflictError`, DB-`ErrConflict` unverändert; neu Leeren der Gruppe nur bei Erfolg -- bestehende Kern-Tests plus Test „Gruppe aufgelöst“.
- `core.(*LocationService).DeleteLocation` -- 1.11 -- `LocationInUseError`/`ErrConflict` unverändert; Leeren nur nach erfolgreichem `InTx` -- Kern-Tests.
- `admin` Ortsliste `showLocations` -- 1.4, 1.12 -- Spalten und Sortierung unverändert, neu „prüfen“ -- Admin-Tests.
- `admin` Ortsauswahl im Event-Formular (`events.go:289`, `new_location.go:93`) -- 1.7, 1.8 -- nutzt weiter `ListLocations` -- bestehende Admin-Tests.
- `cmd/eventstore.run`/`recomputeDerived` -- 1.6, 2.5A -- Reihenfolge Migrationen → Orte → Events → `MarkArchived` → Listen; Speicherfehler brechen ab, Orts- und Eventfehler nicht -- `recompute_test.go`, `TestRunLogsEventsWhoseRecomputationFailsAndStartsAnyway`, neuer Postgres-Test.

## Tasks & Acceptance

**Execution:**
- [x] `internal/core/location_service_test.go`, `location_service.go` -- Test zuerst für alle Orts-Zeilen der Matrix sowie Listen-/Schreib-/Kontextfehler und `ListLocationEntries`; dann implementieren.
- [x] `internal/core/event_service_test.go`, `event_service.go` -- Test zuerst „Ablaufplan außerhalb“ (Tages- und Zeit-Punkt) und „passt“; dann `recomputeEvent`.
- [x] `internal/adapter/postgres/queries/locations.sql`, `db/*`, `locations.go`, `locations_test.go` -- Postgres-Test zuerst (Schreiben, `ErrNotFound`, `ErrConflict`); dann Query, sqlc, Repo.
- [x] `internal/adapter/admin/locations_test.go`, `locations.go`, `templates/locations.html` -- Test zuerst: markierter Ort zeigt „prüfen“, anderer nicht.
- [x] `cmd/eventstore/recompute_test.go`, `recompute.go`, `main.go`, `main_test.go` -- Tests zuerst: Ortsgruppen mit allen IDs geloggt, Start läuft weiter; Fehler beim Orte-Lesen bricht ab; Postgres-Test: Ort mit per SQL veraltetem `name_key` ist vor dem ersten Health-Check korrigiert.
- [x] `README.md` -- Startreihenfolge um die Orte ergänzen.
- [x] `_bmad-output/implementation-artifacts/deferred-work.md` -- beide Teil-B-Einträge auf `status: done`.

**Acceptance Criteria:**
- Given ein Ort mit veraltetem `name_key`, when das Programm startet, then ist der Schlüssel gespeichert, bevor `MarkArchived` läuft und der HTTP-Server lauscht.
- Given eine Kollision beim Start, when ich die Ortsliste im Admin öffne, then tragen alle beteiligten Orte „prüfen“, bis einer von ihnen erfolgreich gespeichert oder gelöscht ist.
- Given ein Event, dessen Ablaufplan nicht mehr in den neuen Zeitraum passt, when das Programm startet, then nennt das Log Event- und Punkt-ID, und die Event-Liste zeigt „prüfen“.

## Implementation Notes

- `RecomputeNameKeys` sortiert die Orte per `SortLocations`, damit Schreibreihenfolge und Ausgang von Schlüsselketten wiederholbar sind. Eine Kette löst sich in einem Lauf nur, wenn der Inhaber des Schlüssels vorher an der Reihe ist; sonst werden beide markiert (Design Notes).
- Überlappende Gruppen (möglich über den DB-Konfliktpfad) werden in `markForReview` zusammengeführt; Speichern oder Löschen eines Mitglieds leert alle.
- `deleteLocation` liefert jetzt die gespeicherte ID, damit `DeleteLocation` die Gruppe auch bei abweichender Groß-/Kleinschreibung der ID leert.
- Die Kern-Tests der Neuberechnung von Orten liegen in der neuen Datei `internal/core/location_recompute_test.go`.
- Info-Log der Neuberechnung: Schlüssel `failed` heißt jetzt `failedEvents`, dazu `failedLocations`.
- Für „Ablaufplan außerhalb“ gibt es Kern-Tests (Tages- und Zeitpunkt), keinen Postgres-Starttest; die Spec verlangt nur einen Postgres-Test für `name_key`.

## Spec Change Log

## Review Triage Log

| # | Layer | Befund | Verdict | Evidenz | Route |
|---|-------|--------|---------|---------|-------|
| 1 | blind/vg/seam | Kein `run`-Test belegt, dass Admin und Neuberechnung denselben `LocationService` teilen | medium | Rückbau auf eigenen `NewLocationService` in `run` bliebe grün, AC 2 bräche; Event-Pendant existiert (`main_test.go:189`) | patch |
| 2 | seam | Markierung bleibt bei `LocationConflictError`/`LocationInUseError` – ungetestet | low | Code korrekt (Rückkehr vor `clearReview`), Test ist direkte Ergänzung | patch |
| 3 | blind | Log-Key `failedLocations` zählt Gruppen | low | Test-Meldung sagt selbst „location groups“; Umbenennen ist direkt | patch |
| 4 | blind | README: Inhaber eines DB-Konflikts wird markiert, Auflösung (umbenennen) fehlt | low | Satz fehlt; direkte Ergänzung | patch |
| 5 | blind | `epic-2-context.md` widerspricht dem Diff und hat Anforderungen verloren (400-Regeln, Standardwerte, CORS …) | medium | Neukompilierung im Build kürzte Fakten; Proposal Teil B sagt „keine Änderung“; Datei auf HEAD zurückgesetzt | patch |
| 6 | blind | Sprint-Status nicht nachgezogen | false | wird am Ende des Workflows synchronisiert | reject |
| 7 | blind | Speichern eines Mitglieds leert die Markierung, obwohl die Kollision bleibt | false | genau so in der eingefrorenen Matrix/AC („bis einer von ihnen … gespeichert oder gelöscht“) | reject |
| 8 | blind | Übrige Gruppenmitglieder behalten veralteten Schlüssel bis zum Neustart | low | AC verlangt Neuberechnung nur beim Start; Fix bräuchte neue Logik in `SaveLocation`/`DeleteLocation` | reject |
| 9 | blind/edge | Schlüsselketten werden im Lauf nicht wiederholt | low | Design Notes akzeptieren das; Ketten setzen eine Regeländerung mit Kette voraus; zweiter Durchlauf ist neue Verzweigung | reject |
| 10 | blind | Reihenfolge vor `MarkArchived` nicht getestet, erwarteter Schlüssel fest codiert, Nebenwirkung `insertCleanupLocation` | low | Reihenfolge ist sequentieller Code (wie Teil A #4 abgelehnt); feste Erwartung ist gewollt; Zusatzort wird aufgeräumt | reject |
| 11 | blind | Log-Key `"error"` als nackter String | low | bestand vorher in `recompute.go`, nicht durch diese Story | reject |
| 12 | blind | Index im Ablaufplan-Fehler ist sortierte Position | false | Admin-Formular zeigt den Ablaufplan sortiert (`sortedTimetable`), Index passt zur Ansicht; Punkt-ID steht zusätzlich da | reject |
| 13 | blind/edge | Zweiter Aufruf von `RecomputeNameKeys` behält alte Markierungen | low | nur einmal pro Start aufgerufen, neue Instanz pro Prozess | reject |
| 14 | blind | Admin-Test schneidet mit `strings.Index` ohne `-1`-Prüfung | low | Fehler zeigt sich als Panik im Test, nicht im Produkt | reject |
| 15 | edge | Tausch zweier Schlüssel meldet Gruppe doppelt | low | nur bei Regeländerung mit Tausch; Dedupe ist zusätzliche Verzweigung | reject |
| 16 | edge | Programmpunkt mit eigenem Fehler (Zeitlücke, Ende = Beginn) wird „gegen neuen Zeitraum“ gemeldet | low | Meldung enthält den Problem-Code; Event wird zu Recht markiert | reject |

## Design Notes

`name_key` liegt am `LocationService`, die Markierung der Orte deshalb auch: Speichern und Löschen leeren sie ohne Verbindung zum `EventService`. „`RecomputeDerived`“ aus AD-16 ist damit der Start-Schritt `recomputeDerived` in `cmd/eventstore`, der beide Kern-Anwendungsfälle aufruft. Kollisionen vorab: Orte nach neuem Schlüssel gruppieren, jede Gruppe mit mindestens zwei Orten ist ein Fehler, ihre Mitglieder werden nicht geschrieben. Der verbleibende DB-Konflikt (Ketten- oder Tauschfälle) wird wie eine Kollision behandelt; beim nächsten Start löst sich eine Kette meist von selbst.

## Verification

**Commands:**
- sqlc 1.31.1 (Release-Binary) und `go generate ./...`; danach `git status --porcelain` -- nur erwartete Dateien
- `CI= go test ./...` -- grün
- `bash scripts/check-coverage.sh` -- 100 %
- `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./...` -- ohne Befund
- `EVENTSTORE_TEST_DATABASE_URL=… go test -p 1 ./internal/adapter/postgres/... ./cmd/eventstore/...` -- grün (natives PG 18)
