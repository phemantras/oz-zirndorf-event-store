---
title: 'C2: Import-Absicherung (Retro Epic 3)'
type: 'chore'
created: '2026-10-07'
status: 'done'
route: 'oneshot'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-3-retro-2026-10-06.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Drei Testlücken aus der Retro Epic 3 (R5–R7) lassen Fehler bei grüner CI durch: (R5) Der AD-9-Vertragstest prüft die kerninternen Schlüssel `sourceFieldDescription`/`sourceFieldURL` nicht, die der Import tatsächlich liest. (R6) Kein Test sichert, dass der Commit einer Datei mit genau `MaxImportFileBytes` und 150 Entscheidungen unter `maxImportCommitBytes` bleibt. (R7) Der Abnahmetest 3.5 ruft nach dem Import weder `RecomputeNameKeys` noch `RecomputeDerived` auf, wie es jeder Start tut.

**Approach:** Nur Tests, kein Produktionscode: (a) `contract_test.go` sammelt zusätzlich Konstanten mit Präfix `sourceField` und prüft sie gegen den generierten Typ `Source`; (b) Admin-Test committet eine auf genau `MaxImportFileBytes` aufgefüllte Datei mit 150 Verdachtsfall-Entscheidungen mit UUID-langen Kandidaten-IDs und 64-stelligen Fingerabdrücken und erwartet 200 statt 413; (c) der Abnahmetest ruft nach dem ersten Import `RecomputeNameKeys` und `RecomputeDerived` auf, erwartet keine Fehlschläge, unveränderte abgeleitete Werte in der Datenbank und weiterhin 15 Archiv- und 24 aktive Events; der zweite Import bleibt `unchanged`.

</frozen-after-approval>

## Implementation Notes

- Deferred-Eintrag „Retro Epic 3, C2“ ist der Umfang dieser Spec und steht mit ihr auf `done`. Weitere offene Einträge mit Ziel C2 oder Epic 3 gibt es nicht („Browser-Tests“ zielt auf C3). Das Aktions-Item C2 in `sprint-status.yaml` geht wie bei C1 nach dem Merge per eigenem Chore-PR auf `done`.
- Der Präfix-Weg statt Ableitung: Eine Konstante wie `EventFieldSource + "." + sourceFieldDescription` wäre kein String-Literal mehr, und `stringConstants` übersähe dann `EventFieldSourceDescription`.
- Geändert: `internal/adapter/publicapi/v1/contract_test.go` (Präfix `sourceField` gegen `Source`), `internal/core/event_test.go` (Kopplung `EventFieldSource*` = `source.` + `sourceField*`), `internal/adapter/admin/import_commit_test.go` (`TestImportCommitReadsTheFormOfTheMostEntries` zu `TestImportCommitReadsTheLargestFileWithTheMostEntries` erweitert: Datei genau 2 MiB, UUID-lange IDs, 64-stellige Fingerabdrücke), `cmd/eventstore/testcollection_test.go` (nach dem ersten Import `recomputeDerived` wie beim Start, keine ERROR-Logs, abgeleitete Werte und öffentliche Listen unverändert).
- Mutationsproben: `sourceFieldURL = "link"` → Vertragstest rot; `maxImportCommitBytes = MaxImportFileBytes + Overhead` → 413, Test rot; abweichender Titelschlüssel in `recomputeEvent` → Abnahmetest rot.
- Postgres-Tests lokal gegen natives PostgreSQL 18 grün.

## Review Triage Log

| # | Layer | Befund | Verdikt | Begründung | Route |
|---|-------|--------|---------|------------|-------|
| 1 | blind | Abnahmetest kopiert die Startsequenz statt `recomputeDerived` aufzurufen | medium | Stimmt; eine geänderte Startsequenz bliebe ungeprüft. Test ruft jetzt `recomputeDerived` und prüft auf ERROR-Logs. | patch |
| 2 | blind | Polsterung mit Leerzeichen trifft nicht die LF→CRLF-Wandlung des Browsers | maybe-false | Betrifft das Produktverhalten (Größenprüfung im Kern beim Commit), nicht nur den Test; braucht Browserprüfung. | defer |
| 3 | blind | Commit-Test prüft nur Status und Überschrift | low | Stimmt; Prüfung auf Zeile der Position 150 ergänzt. | patch |
| 4 | blind | `archived_at` fehlt im Schnappschuss, IDs von Events und Orten in einer Map | false | Die Neuberechnung schreibt `archived_at` nicht; die öffentlichen Listen decken das Archiv ab. UUIDv7-Kollision zwischen Tabellen ist praktisch ausgeschlossen. | – |
| 5 | blind | Kopplung `EventFieldSourceDescription` = `source.` + `sourceFieldDescription` ungeprüft | low | Stimmt; ein Kern-Test mit einer Schleife. | patch |
| 6 | blind | Tracking-Dateien und Spec-Status nicht aktualisiert | false | Werden am Ende des Workflows synchronisiert; Deferred-Eintrag jetzt `done`. | – |
| 7 | blind | Konstantenblock `uuidFormat` mitten in der Datei, Name generisch | low | Kosmetik; der Block steht direkt am einzigen Nutzer. | reject |
| 8 | blind | Kommentar zum Präfix erklärt nicht, warum gegen `Source` | low | Stimmt; Kommentar präzisiert. | patch |
