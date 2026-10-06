---
title: 'Story 3.5: Testsammlung importieren und Abnahme'
type: 'chore'
created: '2026-10-06'
status: 'done'
route: 'oneshot'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-3-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Dass die Testsammlung `testdata/zirndorf_events.v1.json` (39 Events, 19 Orte) vollständig und wiederholbar importierbar ist, belegt bisher nur die Vorschau gegen einen leeren Bestand (Story 3.4). Übernahme, Archiv-Zugriff vergangener Events und ein zweiter Lauf ohne neue Events sind nicht gegen PostgreSQL 18 abgesichert (SM-2).

**Approach:** Ein Integrationstest in `cmd/eventstore` gegen PostgreSQL 18 mit fester `Clock` 2026-10-01 12:00 Europe/Berlin: leerer Bestand → Vorschau und `CommitImport` mit den Entscheidungen, die das Formular aus der Vorschau sendet → alle 39 Events `created`, 19 Orte neu; die vergangenen Events liefert sofort `GET /v1/archive/events` der öffentlichen API, die übrigen `GET /v1/events`. Zweiter Lauf mit derselben Datei: Vorschau nur `unchanged`, Übernahme nur `unchanged`, weiterhin 39 Events. Keine Änderung an Produktionscode. Die SM-1-Prüfung in Produktion (Weihnachtsmarkt per `GET /v1/events` über die Adventszeit, im Admin angelegt) ist ein manueller, nur lesender Abschlussschritt ohne erneuten Import.

</frozen-after-approval>

## Implementation Notes

- Neuer Test `TestTestCollectionImportsCompletelyAndRepeatably` in `cmd/eventstore/testcollection_test.go`, neben dem SM-1-Abnahmetest; nutzt `connectAndMigrate`, `fixedClock`, `publicArchivePath` und `publicAllEventsPath` des Pakets. Kein Produktionscode geändert.
- Leerer Bestand: `TRUNCATE timetable_entries, events, locations` vor und nach dem Test, wie `migratedLocationRepo` im Postgres-Paket. Die Tests in `cmd/eventstore` laufen nicht parallel, und `-p 1` trennt die Pakete.
- Die Entscheidungen bildet der Test aus der Vorschau, wie das Formular sie sendet (Klasse, Ziel, neuer Ort, Kandidaten), ohne eine Wahl bei Verdachtsfällen.
- Archiv-Erwartung als Konstante: 15 Events bis 2026-09-30 sind am 2026-10-01 12:00 vorbei, 24 nicht. Zusätzlich prüft der Test, dass kein archiviertes Event am oder nach dem 2026-10-01 beginnt. Vergleich über Startdaten, weil Titel in der Sammlung mehrfach vorkommen (Stadtratssitzung, DigitalCafé).
- Test-first: Der Abnahmetest war sofort grün, weil er bestehendes Verhalten belegt. Gegenprobe mit falscher Erwartung (14 statt 15 archiviert) schlug wie erwartet fehl.
- SM-1 in Produktion (2026-10-06, nur lesend): `GET https://oz-zirndorf-event-store.up.railway.app/v1/events?from=2026-11-29&to=2026-12-24` liefert 5 Events, darunter „Weihnachtsmarkt 1. Wochenende“ (2026-11-27 15:00 bis 2026-11-29 20:00) und „Weihnachtsmarkt 2. Wochenende“ (2026-12-04 15:00 bis 2026-12-06 20:00), beide mit Zeitgenauigkeit `exact`/`exact`, Ort Zimmermannspark (Koordinaten, Ortsgenauigkeit `area`) und Quelle „Zirndorf Marketing“ mit URL. Freigabe durch Andreas (2026-10-06): Die zwei Weihnachtsmarkt-Termine sind vollständig; am 1. Wochenende findet zusätzlich ein anderes Event statt.

## Verification

- `go test -p 1 ./...` mit `EVENTSTORE_TEST_DATABASE_URL` (PostgreSQL 18) -- grün
- `bash scripts/check-coverage.sh` -- 2108 von 2108 Anweisungen
- golangci-lint v2.14.0 -- 0 Befunde


## Review Triage Log

| # | Quelle | Befund | Urteil | Begründung | Route |
|---|---|---|---|---|---|
| 1 | blind | README: „Die Beispieldaten sind erfunden“ passt nicht mehr zum neuen, echten Beispiel | low | Stimmt; das Beispiel hat Andreas selbst eingesetzt. | an Andreas gemeldet |
| 2 | blind | README-Beispiel einzeilig, unsortiert, „gekürzt“ | low | Andreas' Wunsch: schön formatieren. | patch: formatiert in API-Reihenfolge |
| 3 | blind | README-Beispiel weicht vom SM-1-Testfixture ab; Notiz „1. und 2. Adventswochenende“ verwirrt | false/low | Das README muss das Fixture nicht spiegeln; Notiz sind Andreas' Daten. | an Andreas gemeldet |
| 4 | blind | `last_updated` springt von 17:00 auf 15:24 zurück | false | Der alte Wert lag in der Zukunft (letzter Commit 15:06); eingetragen ist die echte Zeit. | reject |
| 5 | blind | Vorschau-Klassen nicht geprüft (alle `new`, dann alle `unchanged`) | medium | AC nennt „alle Einträge `unchanged`“; einfache Prüfung. | patch: `previewAndCommit` erwartet eine Klasse |
| 6 | blind | Vollständigkeit nur über Zahlen | false | Der zweite Lauf vergleicht jede Eingabe kanonisch mit dem Gespeicherten; ein verlustbehafteter Import ergäbe `update` statt `unchanged`. | reject |
| 7 | blind | Aktive Liste und Ortszahl nach zweitem Lauf ungeprüft | low | Aufteilung 15 + 24 = 39 deckt die aktive Liste ab; Ortszahl fehlte. | patch: Zählung der Orte |
| 8 | blind | `TRUNCATE` weicht von der Paketkonvention ab | low | Leerer Bestand ist Vorbedingung der AC; Begründung fehlte. | patch: Kommentar |
| 9 | blind | `archived_at` nicht geprüft | false | AC meint den Archiv-Zugriff, abgeleitet aus `effective_end` (AD-16), nicht die Markierung. | reject |
| 10 | blind | Zahlen 39/19/15 hart kodiert | low | Gleiches Muster wie der Kern-Test aus 3.4; Kommentar nennt die Herkunft. | reject |
| 11 | blind | Gegenprobe nur für die Archivzahl | low | Abnahmetest über bestehendes Verhalten; Gegenprobe zeigt, dass er prüft. | reject |
| 12 | blind | SM-1-Nachweis nur als Text, Zeitraum ab 2026-11-29 | low | Zeitraum ist der von SM-1 (`christmasMarketQuery`); Überlappung liefert das 1. Wochenende. | reject |
| 13 | blind | Status `in-progress` trotz grüner Prüfungen | false | Wird beim Abschluss auf `done` gesetzt; Freigabe der Termine durch Andreas steht in den Notes. | reject |
| 14 | blind | `epic-3-context.md` verliert Details (Browser-Tests, Restrisiko Sperre, NFC-Ort) | low | Kompilierter Cache; Browser-Tests stehen in `deferred-work.md`, Regeln im Spine. | reject |
