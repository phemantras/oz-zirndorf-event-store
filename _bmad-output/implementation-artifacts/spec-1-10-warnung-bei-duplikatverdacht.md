---
title: 'Story 1.10: Warnung bei Duplikatverdacht'
type: 'feature'
created: '2026-10-03'
status: 'done'
baseline_commit: 'bdae5b8be9b60578a537bbe23e779226e405a1ca'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-1-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Der Admin kann stille Duplikate anlegen; AD-11 verlangt eine Warnung mit bewusstem „Trotzdem speichern“. Die Akzeptanzkriterien der Story 1.10 in `epics.md` gelten vollständig.

**Approach:** Der Kern leitet `TitleKey = NormalizeKey(title)` ab, speichert ihn (Migration `00006`) und zieht ihn in `RecomputeDerived` nach. `SaveEvent` bekommt eine `DuplicatePolicy`; bei `RejectDuplicates` sucht `FindDuplicateCandidates` in der Transaktion nach Events mit gleichem `title_key`, `startDate` und `locationId` (ohne das Event selbst) und liefert `*DuplicateSuspectError` (matcht `ErrDuplicateSuspect`). Der Admin zeigt die Warnung mit Links und bietet „Trotzdem speichern“.

## Boundaries & Constraints

**Always:** Vergleich nur über die drei Schlüssel, `startDate` ohne Uhrzeit, gegen alle Events inkl. archivierter. Duplikatprüfung erst, wenn die Eingabe gültig ist (Feldfehler gehen vor). SQL nur einfache Gleichheit, `title_key` berechnet nur der Kern. Migration expand: `title_key text NOT NULL DEFAULT ''`, Start-Neuberechnung füllt Bestand; ein Fehler dort wie ENT-5. Test-first, 100 % für Kern und Admin.

**Never:** Keine Eindeutigkeit von `title_key` in der DB, keine Prüfung in SQL, kein Ablehnen bei `AllowDuplicates`. Kein Import-Code (Epic 3). Angewendete Migrationen unverändert.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Treffer | Bestand „Kirchweih“ 16.10. Festplatz (auch archiviert); neu „ kirchweih “ 16.10. 20:00 Festplatz | nicht gespeichert, Warnung, 409 | Kandidaten verlinkt |
| Bestätigt | gleiche Eingabe mit „Trotzdem speichern“ | gespeichert mit `AllowDuplicates` | N/A |
| Abbrechen | Klick „Abbrechen“ in der Warnung | Warnung weg, Eingaben bleiben | N/A |
| Anders | gleicher Titel, anderes Datum oder anderer Ort | gespeichert ohne Warnung | N/A |
| Selbst | Event unverändert erneut speichern | gespeichert ohne Warnung | N/A |
| Bearbeiten | Event B so ändern, dass es A gleicht | Warnung mit A | N/A |
| Ungültig + Treffer | Treffer, aber Quelle fehlt | nur Feldfehler, 422 | keine Warnung |
| Neustart | Event mit leerem `title_key` | Start schreibt `title_key` nach | Fehler: Event behält Werte, „prüfen“ |

**Entscheidungen ohne Rückfrage (2026-10-03):** Status 409 wie beim Ortsnamenkonflikt. „Trotzdem speichern“ ist ein zweiter Submit-Button (`duplicates=allow`) im Warnkasten, „Abbrechen“ entfernt den Kasten per `hx-on:click`. Kandidaten-Links öffnen in neuem Tab (Eingaben bleiben), Text: Titel, Beginn, ggf. „archiviert“. Kandidatensuche als Repo-Abfrage, Filter „nicht selbst“ und Schlüsselbildung im Kern.

</frozen-after-approval>

## Code Map

- `internal/core/event.go` -- `Event.TitleKey`; `newEvent` setzt ihn aus dem normalisierten Titel.
- neu `internal/core/duplicates.go` -- `DuplicatePolicy` (`RejectDuplicates`, `AllowDuplicates`), `DuplicateKey{TitleKey, StartDate, LocationID}`, `FindDuplicateCandidates(ctx, EventRepo, Event) ([]Event, error)` (filtert `event.ID`).
- `internal/core/errors.go` -- `ErrDuplicateSuspect`, `DuplicateSuspectError{Candidates []Event}` mit `Unwrap`; Muster `LocationConflictError`.
- `internal/core/event_service.go` -- `EventRepo.FindByDuplicateKey(ctx, DuplicateKey)` (Kandidaten ohne Ablaufplan); `UpdatePeriod` → `UpdateDerived(ctx, id, Derived{Period, TitleKey})`; `SaveEvent(ctx, id, in, policy)`, Prüfung in `saveEvent` nach Validierung, vor `writeEvent`; `recomputePeriod` → `recomputeDerived`, schreibt bei Abweichung von Zeitraum oder Schlüssel.
- `internal/core/event_service_test.go`, neu `duplicates_test.go` -- `fakeEventRepo` um Schlüsselsuche und `UpdateDerived` erweitern.
- neu `internal/adapter/postgres/migrations/00006_event_title_key.sql` -- Spalte + Index `(title_key, start_date, location_id)`.
- `internal/adapter/postgres/queries/events.sql` + `sqlc generate` (1.31.1) -- `title_key` in allen SELECT/INSERT/UPDATE; `FindEventsByDuplicateKey`; `UpdateEventPeriod` → `UpdateEventDerived`.
- `internal/adapter/postgres/events.go`, `events_test.go`, `timetable_test.go` -- Mapping, neue Methoden, Aufrufe mit Policy.
- `internal/adapter/admin/events.go` -- Interface mit Policy; Formularwert `duplicates=allow` → `AllowDuplicates`, sonst `RejectDuplicates`; `*core.DuplicateSuspectError` → Formular mit `DuplicateWarning` (409); `formatMoment`, `eventURL`, `h.clock` wiederverwenden.
- `internal/adapter/admin/templates/event_form.html` -- Warnkasten (`role="alert"`) oberhalb des Formulars innerhalb von `<form>`, Muster `location_form.html`.
- `internal/adapter/admin/events_test.go`, `cmd/eventstore/main_test.go` (`emptyEvents`) -- Signaturen anpassen.

## Tasks & Acceptance

**Execution:**
- [x] `internal/core/...` (+Tests) -- `TitleKey`, Policy, Fehler, `FindDuplicateCandidates`, `SaveEvent`, Neuberechnung; alle Matrix-Kernfälle ohne DB.
- [x] `internal/adapter/postgres/...` (+Tests gegen PG 18) -- Migration, Queries, generierter Code, Repo; Test: archiviertes Event wird gefunden, Neuberechnung füllt leeren `title_key`.
- [x] `internal/adapter/admin/...` (+Tests) -- Warnung, Links, Bestätigung, Abbrechen-Button, 422 ohne Warnung.
- [x] `cmd/eventstore/...` (+Tests) -- Anpassungen an Signaturen.

**Acceptance Criteria:**
- Given `bash scripts/check-coverage.sh`, golangci-lint v2.14.0, `CI= go test ./...`, Postgres-Tests mit `-p 1` und `sqlc generate` ohne Diff, when sie laufen, then grün und 100 %.

## Implementation Notes

- Abweichung von der Code Map (Admin): Der Warnkasten ist geteilt. Meldung und Links stehen oben im Formular (`role="alert"`), „Trotzdem speichern“ und „Abbrechen“ stehen **nach** dem normalen „Speichern“. Grund: Der erste Submit-Button eines Formulars ist der Standard bei Enter; stünde „Trotzdem speichern“ oben, würde Enter in einem Textfeld das Duplikat bestätigen. „Abbrechen“ entfernt beide Teile (`.duplicate-warning`). Test `TestEnterInTheFormNeverConfirmsTheDuplicate`.
- Kern: `FindDuplicateCandidates` sortiert Kandidaten nach Beginn, dann ID. Jeder Policy-Wert außer `AllowDuplicates` prüft (sicherer Default). `RecomputeDerived` schreibt per `UpdateDerived`, wenn Zeitraum oder Titel-Schlüssel abweichen.
- Postgres: `title_key` steht in SELECT/RETURNING am Ende (Tabellenreihenfolge), damit sqlc weiter `db.Event` nutzt. `FindByDuplicateKey` liefert Kandidaten ohne Ablaufplan.
- Deferred: Contract-Schritt für den Default von `title_key` in `deferred-work.md`.
- Lokal verifiziert (2026-10-03): `CI= go test ./...`, Postgres-Tests mit `-p 1` gegen lokales PostgreSQL 18, Coverage-Gate 975/975, `go vet`, golangci-lint v2.14.0 ohne Befund, `sqlc generate` (1.31.1) ohne Diff.
- Matrix-Audit: alle Zeilen durch Kern- und Admin-Tests abgedeckt, Neustart zusätzlich durch `TestEventServiceRunsAgainstDatabase`. „Abbrechen“ ist nur als Markup getestet; das Entfernen per JavaScript prüft nur die manuelle Browserprobe (vgl. Deferred Browser-Tests aus 1.8).

## Spec Change Log

## Review Triage Log

| # | Quelle | Befund | Verdikt | Evidenz | Route |
|---|--------|--------|---------|---------|-------|
| 1 | edge + blind | Gleichzeitige Saves mit gleichem Schlüssel (Doppelklick) umgehen die Prüfung | low | Read committed, kein Lock; ein Admin, Doppel-Submit erzeugte schon vor 1.10 zwei Events; Abhilfe Advisory-Lock = neue Komplexität | reject |
| 2 | edge + blind | „Trotzdem speichern“ gilt auch für nach der Warnung geänderte Eingaben | low | Bestätigung trägt keinen Schlüssel; der Admin klickt bewusst „Trotzdem“; Abhilfe Hidden-Felder + Vergleich | reject |
| 3 | edge + blind | Event mit fehlschlagender Neuberechnung behält `title_key = ''`, wird nie Kandidat | low | Nur Altbestand vor `00006`, dessen Zeiten zugleich ungültig sind, und „prüfen“-markiert; Abhilfe neuer Zweig in `recomputeEvent` | reject |
| 4 | edge (2×) | Bearbeiten eines bestätigten Zwillings warnt bei jeder Änderung erneut; widerspricht „Selbst“ | medium | Nur die eigene ID wurde gefiltert; Story: Warnung, wenn ich so bearbeite, „dass es ein anderes doppelt“ | patch: Prüfung nur bei neuem Event oder geändertem Schlüssel (`needsDuplicateCheck`) |
| 5 | gap | `Update` schreibt `title_key`, kein Test ändert den Schlüssel | low | Fixture-Key vorher/nachher gleich | patch: Update-Test mit neuem `TitleKey` |
| 6 | blind | Warnung oben sagt nicht, wo die Buttons sind | low | Buttons stehen nach „Speichern“ (Enter-Schutz) | patch: Meldung nennt die Buttons |
| 7 | blind | `TestSameTitleElsewhere…` ohne eigene Assertion | low | Nur implizit über `seedEvent` | patch: explizite Prüfungen |
| 8 | blind | Kein PG-Test mit archiviertem Kandidaten | low | Fixture lag in der Zukunft | patch: Datum 2025 |
| 9 | blind | Mehrere Kandidaten im Admin ungetestet | low | Alle Tests mit einem Kandidaten | patch: Test mit zwei, Reihenfolge und „archiviert“ |
| 10 | blind | Sprint-Status `in-progress` vs. Spec `in-review` | false | Workflow setzt Sprint-Status erst am Ende | reject |
| 11 | blind | Testmeldung „periods updated“, Doc-Kommentar falsch umbrochen | low | Direkte Korrektur | patch |
| 12 | blind | Kein Test für nur geänderten, nicht leeren `title_key` | low | Zweig durch leeren Schlüssel abgedeckt (100 %) | reject |
| 13 | blind | `FindDuplicateCandidates` exportiert ohne Adapter-Nutzer | false | AD-11 nennt die Kernfunktion; `CommitImport` (Epic 3) nutzt sie | reject |
| 14 | blind | `DuplicateSuspectError.Error()` ohne Kandidaten | low | Wird nie leer erzeugt | reject |
| 15 | blind | Index-Kommentar/Deferred-Eintrag erwähnen leere Schlüssel nicht | low | Kosmetik, Contract-Schritt ändert keine Zeilen | reject |

## Design Notes

```go
// Admin: erst prüfen, nach Bestätigung erlauben.
policy := core.RejectDuplicates
if form.Get(duplicatesField) == duplicatesAllow { policy = core.AllowDuplicates }
```

Vor dem Deploy `pg_dump` (neue Migration). Der Default `''` bleibt bis zum Contract-Schritt (Deferred).

## Verification

**Commands:**
- `CI= go test ./...` -- grün
- `go test -p 1 ./internal/adapter/postgres/... ./cmd/eventstore/...` mit `EVENTSTORE_TEST_DATABASE_URL` -- grün
- `bash scripts/check-coverage.sh` -- 100 %

**Manual checks (if no CLI):**
- Browser: Duplikat anlegen → Warnung mit Links, „Abbrechen“ behält Eingaben, „Trotzdem speichern“ legt an.
