---
title: 'Story 1.7: Events anlegen, bearbeiten und auflisten'
type: 'feature'
created: '2026-10-03'
status: 'done'
baseline_commit: '8ffe21f5fe246acabc93aa2fde2736a61906ab4b'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-1-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Der Admin kann bisher nur Orte pflegen; Events gibt es weder im Kern noch in der Datenbank oder Oberfläche (FR-1, FR-3, FR-5, FR-7, FR-15, NFR-4, AD-2, AD-6, AD-14, AD-16, ENT-5/9/10/15–18). Die Akzeptanzkriterien der Story 1.7 in `epics.md` gelten vollständig.

**Approach:** Kern-Anwendungsfälle `SaveEvent`, `GetEvent`, `ListEvents`, `RecomputeDerived` auf dem Zeitmodell aus 1.6, Port `EventRepo` mit Postgres-Adapter (Tabelle `events`), Admin-Seiten für Liste und Formular, Neuberechnung beim Start. Der Contract-Schritt der Adress-Migration (`locations.address` entfernen) ist ausgegliedert.

## Boundaries & Constraints

**Always:** Eingabetyp `core.EventInput` mit Texten wie `LocationInput` (`startDate` `YYYY-MM-DD`, Uhrzeiten `HH:MM`, leer = unbekannt) plus `AllDay bool` und `Source{Description, URL}`; Kern normalisiert per `normalizeText` (`Canonicalize`) und meldet alle Probleme gesammelt als `*ValidationError`. Feldnamen als Konstanten: `title`, `type`, `locationId`, Zeitfelder aus 1.6, `source.description`, `source.url`, `note`; das Formular nutzt dieselben Namen. Typ-Codes `festival` … `other` als Kern-Konstanten mit Liste (Muster `LocationPrecisions`). Ort muss existieren (Prüfung im Kern über `LocationRepo.Get`); Event speichert nur `location_id`. Quellen-Link nur `http`/`https` mit Host. `effective_start/end` berechnet `EventTimes.EffectivePeriod`. Status „archiviert“ = `Period.IsOver(Clock)`; `cmd/eventstore` liefert die echte `Clock`. „prüfen“-Menge im Kern, per Mutex, nur im Speicher; erfolgreiches `SaveEvent` entfernt die ID. Startreihenfolge Migrationen → `RecomputeDerived` → HTTP; Fehler je Event mit ID loggen, Start läuft weiter. Admin: deutsche Meldungen je Feld, Eingaben bleiben erhalten, Hinweis „leer = unbekannt“ an beiden Uhrzeitfeldern, Personendaten-Hinweis unter Titel, Notiz und Quelle. Test-first, 100 % Abdeckung für `internal/core` und `internal/adapter/admin`.

**Never:** Kein `title_key`, keine Duplikat-Policy (1.10), kein Ablaufplan und kein `TxRunner` (1.9), kein `import_key` (Epic 3), kein `archived_at`/`MarkArchived` (2.5), kein Löschen (1.11), kein Inline-Ort (1.8), keine Public API, keine Contract-Migration für `locations.address` (eigener PR). Kein `now()`, keine Sortierung oder Logik in SQL. Angewendete Migrationen 00001–00003 nicht ändern.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Neu, vollständig | Titel, `market`, vorhandener Ort, 2026-10-16, Quelle „Amtsblatt“ | gespeichert, `effective` 16. 00:00–17. 00:00, Weiterleitung zur Liste | N/A |
| Pflicht leer | Titel „  “, Typ leer, Ort leer, Datum leer, Quelle leer | 422, Formular mit allen Werten | je Feld `missing` mit deutscher Meldung |
| Unbekannter Typ | `type=concert` | — | `type` `unknownCode` |
| Ort unbekannt | `locationId` ohne Treffer | — | `locationId` `notFound` |
| Quellen-Link | `ftp://x`, `amtsblatt.de` | — | `source.url` `invalidFormat` |
| Textformat | `startDate=16.10.2026`, `startTime=7 Uhr` | — | je Feld `invalidFormat` |
| Zeitregeln | Ende vor Beginn, `allDay` + Uhrzeit, Frühjahrslücke | — | Probleme aus 1.6 mit deutschen Meldungen |
| Reaktivieren | archiviertes Event, Datum in die Zukunft | Liste zeigt „aktiv“ | N/A |
| Neuberechnung scheitert | gespeicherte Werte nach Regeländerung ungültig | Werte bleiben, Log mit ID, Start läuft, Liste „prüfen“ bis Speichern | N/A |
| Event unbekannt | `/admin/events/{id}` ohne Treffer | 404 „Event nicht gefunden.“ | N/A |

**Entscheidungen (2026-10-03):** Contract-Migration (`locations.address` und Defaults entfernen) wird ein eigener PR; die Produktionsorte sind laut Andreas vollständig gepflegt. Event-Liste: aktive Events zuerst, chronologisch aufsteigend nach `effectiveStart`; danach archivierte, die jüngsten zuerst (`effectiveStart` absteigend); Gleichstand nach ID. Volle Spec trotz Überlänge sonst unverändert.

</frozen-after-approval>

## Code Map

- `internal/core/location.go`, `location_service.go` -- Muster für Eingabetyp, `report`-Closure, Feld-/Code-Konstanten mit AD-9-Kommentar, `storedID`, Fehler-Wrapping; `LocationRepo.Get` für die Ortsprüfung wiederverwenden, `SortLocations` für die Ortsauswahl.
- `internal/core/eventtimes.go`, `localtime.go`, `clock.go` -- `EventTimes`, `EffectivePeriod`, Feldkonstanten `EventField*`, `LocalDate`/`LocalTime`, `Period.IsOver`; nicht ändern außer Ergänzungen.
- `internal/core/errors.go` -- neues `ProblemNotFound` (`notFound`); Rest wiederverwenden.
- `internal/core/text.go` -- `normalizeText` ist die einzige Normalisierung.
- `internal/adapter/postgres/locations.go` -- Muster für Repo, `parseID`, `translateError`, `optionalText`; `parseID` meldet „location id“, für Events verallgemeinern.
- `internal/adapter/postgres/db/` -- generiert; per sqlc 1.31.1 neu erzeugen (Release-Binary unter Windows, CI prüft Aktualität).
- `internal/adapter/admin/locations.go`, `handler.go`, `templates.go`, `templates/*.html` -- Muster für Use-Case-Interface, Routen, Formular-Seite, `fieldErrorMessages` (auf eine Meldungs-Map je Seite umstellen, Feld `note` kollidiert), `notFoundPage`; `home.html` „Events folgen.“ durch Link ersetzen.
- `cmd/eventstore/main.go` -- `run` verdrahtet Repos und Services; `RecomputeDerived` nach `Migrate`, vor `net.Listen`; Admin-`Config` um `Events` und `Clock` ergänzen (`Now` bleibt für Sessions).
- `internal/adapter/postgres/locations_test.go`, `cmd/eventstore/main_test.go` -- Muster für Postgres-Tests (`EVENTSTORE_TEST_DATABASE_URL`, `-p 1`).

## Tasks & Acceptance

**Execution:**
- [x] `internal/core/errors.go`, `event.go` (+Tests) -- `EventType`-Konstanten und `EventTypes()`, Feldkonstanten, `EventSource`, `EventInput`, `Event{ID, Title, Type, LocationID, Times EventTimes, Source, Note, Period}`, `Canonicalize`, Text-Parser für Datum/Uhrzeit, `newEvent` mit gesammelter Prüfung.
- [x] `internal/core/event_service.go` (+Test) -- Port `EventRepo` (`List`, `Get`, `Create`, `Update`, `UpdatePeriod`), `EventService` mit `SaveEvent(ctx, id, EventInput)`, `GetEvent`, `ListEvents` (liefert Event, Ort, archiviert, prüfen; sortiert im Kern nach der Entscheidung oben), `RecomputeDerived` (liefert Fehlschläge mit ID, schreibt nur geänderte Werte), „prüfen“-Menge mit Mutex.
- [x] `internal/adapter/postgres/migrations/00004_events.sql`, `queries/events.sql`, `events.go` (+Test), sqlc neu erzeugen -- Tabelle laut Datenmodell der Story, FK `ON DELETE RESTRICT`, Umrechnung `pgtype.Date/Time/Timestamptz` ↔ Kern-Typen.
- [x] `internal/adapter/admin/events.go` (+Test), `templates/events.html`, `event_form.html`, `templates.go`, `handler.go`, `home.html` -- Liste, Neu, Bearbeiten, Speichern mit Fehlerübersetzung (422/404/500), deutsche Typ-Labels laut FR-5.
- [x] `cmd/eventstore/main.go`, `clock.go` (+Tests) -- System-`Clock`, Verdrahtung, Neuberechnung beim Start mit Logging je Fehlschlag.

**Acceptance Criteria:**
- Given ein gespeichertes Event, when ich die Liste öffne, then stehen Titel, Typ, Ort, Beginn, Ende und Status „aktiv“/„archiviert“ (ggf. „prüfen“) dort, und das Formular zeigt beim Bearbeiten die gespeicherten Werte.
- Given ein Ort mit Event, when er per SQL gelöscht werden soll, then verhindert der Fremdschlüssel das.
- Given `bash scripts/check-coverage.sh`, Architekturtest, Postgres-Tests und golangci-lint, when sie laufen, then grün und 100 %.

## Implementation Notes

- `ListEvents(ctx, clock)` bekommt die `Clock` als Parameter; der Admin reicht `Config.Clock` durch, `cmd/eventstore` setzt `systemClock`.
- `parseID` im Postgres-Adapter nimmt die Art der ID; `fieldErrorMessages` nimmt die Meldungs-Map je Formular (neu `forms.go` mit `selectOption`, `notFoundPage.BackLabel`).
- Migration 00004 mit Index auf `location_id`; ein SQL-Löschen eines Orts mit Event scheitert mit SQLSTATE 23001 (`RESTRICT`). `locations_test.go` leert jetzt `events, locations`.
- Lokal verifiziert (2026-10-03): `CI= go test ./...`, Coverage-Gate 726/726, `go vet`, golangci-lint v2.14.0 ohne Befund, Postgres-Tests gegen lokales PostgreSQL 18 (ohne Docker), `sqlc generate` (1.31.1) ohne Diff. `-race` nur in der CI (kein cgo lokal).

## Spec Change Log

## Review Triage Log

| # | Quelle | Befund | Verdikt | Evidenz | Route |
|---|--------|--------|---------|---------|-------|
| 1 | gap + blind | „prüfen“ nicht über die Verdrahtung in `run` getestet | low | Kein Test fordert `/admin/events` nach `run` mit kaputtem Event an; eine zweite `EventService`-Instanz bliebe unentdeckt | patch |
| 2 | gap | Linktext der Orts-404-Seite nicht geprüft | low | Test prüft nur `href`; ohne `BackLabel` entstünde ein leerer Link | patch |
| 3 | blind | `insertBrokenEvent` nicht wiederholbar | low | Fester `name_key` ohne Aufräumen vorher; abgebrochener Lauf blockiert spätere | patch |
| 4 | blind | Listentest prüft Status und leere Zelle nur als Teilstrings | low | `<td></td>`, „aktiv“, „archiviert“ binden nicht an die Zeile | patch |
| 5 | blind | Keine Längengrenzen für Texte | low | Weder AC noch Ortsmuster verlangen das; neue Regel statt Korrektur | reject |
| 6 | blind | Zu großes Formular → englisches „Bad Request“ | low | Gleiches Verhalten wie beim Ortsformular seit 1.4; 16 KiB erreicht man kaum | reject |
| 7 | blind | „prüfen“ nennt keinen Grund | low | AC verlangt nur die Markierung; Grund steht im Log | reject |
| 8 | blind + edge | Schreibfehler/abgebrochener Kontext markiert gültige Events | low | ENT-5 behandelt jedes Scheitern der Neuberechnung gleich; Abbruch beim Start heißt Shutdown | reject |
| 9 | blind | Start ohne Zeitlimit für die Neuberechnung | low | Hobby-Bestand, nur geänderte Werte werden geschrieben | reject |
| 10 | blind | Admin-Pfade als Literale in Templates | low | Bestehendes Muster der Orts-Templates; keine benannte Abweichung | reject |
| 11 | blind | Zwei Zeitquellen im Admin-`Config` | false | Spec legt fest, dass `Now` für Sessions bleibt; beide nutzen dieselbe Systemuhr | reject |
| 12 | blind | Liste ohne Blättern/Filter | false | AC verlangt alle Events | reject |
| 13 | blind | Fehlermeldungen nicht per ARIA verknüpft | low | Bestehendes Formularmuster seit 1.4; eigenes Thema | reject |
| 14 | blind + edge | Unbekannter Typ-Code zeigt leere Zelle; `allDay`-Fehlerslot ungenutzt | low | Codes kommen nur über die Kernprüfung in die DB | reject |
| 15 | blind | Kein Schutz gegen verlorene Updates | low | Genau ein Admin; Import kommt erst in Epic 3 | reject |
| 16 | edge | `EffectivePeriod`-Fehler ohne `*ValidationError` wird verschluckt | false | `EffectivePeriod` liefert ausschließlich `*ValidationError` | reject |
| 17 | edge | 9999-12-31 → `invalidFormat` | low | Schon in 1.6 abgelehnt (Triage #2) | reject |
| 18 | edge | Ort zwischen Prüfung und Insert gelöscht → 500 | false | Es gibt keinen Löschpfad für Orte (Story 1.11) | reject |
| 19 | edge | Fehlender Ort in der Map → ganze Liste 500 | false | FK `ON DELETE RESTRICT` verhindert verwaiste Events | reject |
| 20 | edge | Gespeicherte Zeit mit Sekunden oder 24:00 | false | Nur der Kern schreibt, immer aus `HH:MM` | reject |

## Design Notes

Entscheidungen ohne Rückfrage: Anzeige der Zeiten im Admin als `16.10.2026 19:00`, ohne Uhrzeit nur Datum, bei `allDay` mit Zusatz „ganztägig“. `RecomputeDerived` deckt in 1.7 nur `effective*` ab; `title_key` folgt mit 1.10, `name_key` wird nach `deferred-work.md` verschoben. Scheitert `ListEvents`/`List` beim Start, bricht der Start ab (Datenbank kaputt); nur Fehler je Event lassen ihn weiterlaufen. Event-Typ-Labels: Fest/Kirchweih, Markt, Kultur/Bühne, Politik/Sitzung, Verein/Treff, Sport, Sonstiges. Event-Formular hat Checkbox „Ganztägig“; Uhrzeitfelder `type=time`, Datumsfelder `type=date`.

## Verification

**Commands:**
- `CI= go test ./...` -- grün
- `bash scripts/check-coverage.sh` -- 100 %
- `go vet ./...`, golangci-lint v2.14.0 -- ohne Befund
- Postgres-Tests mit `EVENTSTORE_TEST_DATABASE_URL` und `-p 1` (lokal, falls Docker verfügbar; sonst CI) -- grün
