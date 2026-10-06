---
title: 'Story 3.3: Duplikate entscheiden und Import übernehmen'
type: 'feature'
created: '2026-10-06'
status: 'in-review'
baseline_commit: 'a09afeb3d3fdbbf85f84a411a826978842399808'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-3-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Die Vorschau aus 3.2 zeigt Klassen, aber nichts lässt sich übernehmen. Schreib-Transaktionen laufen ungesperrt nebeneinander, `SaveLocation` und die Neuberechnung beim Start schreiben ohne `TxRunner`.

**Approach:** `TxRunner` nimmt zu Beginn jeder Transaktion eine feste Advisory-Sperre. `SaveLocation`, `RecomputeNameKeys` und `RecomputeDerived` laufen über `TxRunner`. `CommitImport` klassifiziert in einer Transaktion neu, sortiert aus und schreibt den Rest. Die Vorschau bekommt ein Formular mit Entscheidungen, danach folgt eine Ergebnisseite mit Zusammenfassung.

## Boundaries & Constraints

**Always:**
- Formular (multipart): Dateiinhalt als verstecktes Feld; je Position die Upload-Klasse, Ziel-ID (bei `update`/`unchanged`), ob der Ort beim Upload neu war, und bei `duplicateSuspect` die Wahl: „überspringen“, „als neues Event anlegen“ und je Kandidat aus dem Bestand „überschreiben“. Ohne Bestands-Kandidat gibt es kein Überschreiben. Voreingestellt ist nichts.
- `CommitImport` liest die Datei, klassifiziert unter der Sperre neu und ordnet jedem Eintrag genau ein Ergebnis zu: `created`, `updated`, `unchanged`, `skipped`, `undecided`, `error`, `stale`.
- `stale`: Klasse, Ziel-ID oder „Ort neu“ weicht von der Formularangabe ab, oder das gewählte Überschreib-Ziel ist kein aktueller Bestands-Kandidat mehr.
- `error`: Klasse `error`; zwei Einträge mit demselben Ziel (`update` oder Überschreiben), dann beide; beim Überschreiben ein anderer, nicht leerer `importKey` am Ziel.
- Schreiben in einer Transaktion: Neue Orte werden einmal je `NormalizeKey` angelegt (Angaben des ersten Eintrags), und nur solche, die ein übernommener Eintrag braucht. Danach Events über den Tx-Kern von `saveEvent` mit `AllowDuplicates`. Überschreiben und `update` ersetzen das Ziel samt Ablaufplan und löschen `archived_at`. `import_key` wird normalisiert gesetzt; ohne Schlüssel im Eintrag bleibt der des Ziels.
- Nach dem Commit werden die Prüf-Markierungen der geschriebenen Events gelöscht.
- Ein unerwarteter DB-Fehler rollt alles zurück. Admin: Seite mit „Der Import ist fehlgeschlagen; es wurde nichts übernommen.“ (500).
- Die Ergebnisseite zeigt die 7 Zahlen und die Zahl neu angelegter Orte. Die Summe der Einträge ist die Zahl der Einträge in der Datei.
- `TxRunner.InTx` beginnt mit `pg_advisory_xact_lock` und einer eigenen festen Kennung (nicht der von goose).
- `SaveLocation` (Ortsverwaltung, „Neuer Ort“) läuft über `TxRunner`, Verhalten wie in 1.4/1.8.
- `RecomputeNameKeys` und `RecomputeDerived` laufen je in einer Transaktion unter der Sperre und lesen erst danach.

**Never:** Kein Zwischenstand auf dem Server. Keine Schreibmethode am Kern vorbei. `SaveEvent` aus dem Admin setzt `import_key` nie. Keine Änderung angewendeter Migrationen.

## I/O & Edge-Case Matrix

| Szenario | Eingabe / Bestand | Ergebnis |
|---|---|---|
| Neu | `new`, Formular `new` | `created` |
| Update | `update` auf X, Formular gleiches Ziel | `updated`, X ersetzt, `archived_at` leer |
| Verdacht ohne Wahl | `duplicateSuspect` | `undecided` |
| Überschreiben | Verdacht gegen Y, Wahl Y | `updated` Y; `import_key` aus dem Eintrag oder der von Y |
| Ziel hat anderen Schlüssel | Überschreiben Y, Y hat „b“, Eintrag „a“ | `error` |
| Gleiches Ziel | zwei Einträge überschreiben Y | beide `error` |
| Veraltet | Formular `new`, jetzt `update` | `stale` |
| Ort inzwischen da | Formular „Ort neu“, Ort existiert jetzt | `stale`, Ort nicht angelegt |
| Doppelklick | zweiter Commit derselben Form | `stale`/`unchanged`, kein Duplikat |
| DB-Fehler | Schreiben scheitert | nichts übernommen, Fehler |

</frozen-after-approval>

## Code Map

- `internal/adapter/postgres/tx.go` -- `InTx`: Sperre als erste Anweisung, Konstante `writeLockID`.
- `internal/adapter/postgres/locations.go` -- `UpdateNameKey` in eigenem Savepoint/Tx (`Begin` der Verbindung), damit ein Konflikt die Neuberechnungs-Tx nicht abbricht.
- `internal/adapter/postgres/queries/events.sql`, `events.go` -- neue Query `UpdateEventImportKey`, `EventRepo.SetImportKey`; sqlc-Binary 1.31.1.
- `internal/core/location_service.go` -- `saveLocation(ctx, repos, id, in)` als Tx-Kern; `SaveLocation` über `InTx`, `ErrConflict` danach über `conflictWith` außerhalb der Tx; `RecomputeNameKeys` über `InTx` mit `repos.Locations`; Kommentare zur Ausnahme und zur nicht atomaren Prüfung entfernen.
- `internal/core/event_service.go` -- `RecomputeDerived` über `InTx`; `EventRepo.SetImportKey`.
- `internal/core/import_classify.go` -- `newImportClassifier(ctx, repos Repos)`.
- `internal/core/import_commit.go` (neu) -- `ImportDecision`, `ImportChoice`, `ImportOutcome`, `ImportSummary`, `CommitImport`.
- `internal/core/import.go` -- `NewImportService(events *EventService)` (Tx, Repos, Prüf-Markierungen daraus).
- `internal/adapter/admin/import.go`, `templates/import.html` -- Entscheidungsformular, `POST /admin/import/commit`, Ergebnisseite.
- `cmd/eventstore/main.go` -- Verdrahtung.

## Nahtstellen

- `SaveLocation` -- 1.4/1.8 -- Konflikt-, Validierungs- und Nicht-gefunden-Fälle unverändert -- bestehende core-/admin-Tests.
- `RecomputeNameKeys`/`RecomputeDerived` -- 2.5/2.5b -- Fehlschläge, Markierungen und Kollisionsgruppen inklusive DB-Konflikt unverändert -- bestehende Tests plus Postgres-Sperrtest.
- `PreviewImport` -- 3.2 -- gleiche Klassen; liest über `Repos` -- bestehende Tests.
- `SaveEvent`/`DeleteEvent`/`DeleteLocation`/`MarkArchived` -- laufen jetzt unter der Sperre -- Postgres-Test „zweite Tx wartet“.

## Tasks & Acceptance

**Execution:**
- [x] `internal/adapter/postgres/tx.go` (+ Test) -- Sperre; Test: zweite Tx wartet und sieht das Ergebnis der ersten.
- [x] `internal/core/location_service.go` (+ Tests) -- `SaveLocation` und `RecomputeNameKeys` über `TxRunner`.
- [x] `internal/adapter/postgres/locations.go` -- Savepoint für `UpdateNameKey`.
- [x] `internal/core/event_service.go` (+ Tests), Postgres-Test -- `RecomputeDerived` in Tx; paralleles Speichern wartet.
- [x] `queries/events.sql`, `events.go`, `db/*` -- `SetImportKey`.
- [x] `internal/core/import_commit.go` (+ Test) -- jede Zeile der Matrix test-first mit Fakes.
- [x] `internal/adapter/admin/import.go`, `import.html` (+ Test) -- Formular, Commit, Ergebnisseite, Fehlermeldung.
- [x] `cmd/eventstore/main.go`, Postgres-Test -- Verdrahtung; Doppel-Commit gegen PostgreSQL ohne Duplikat.

**Acceptance Criteria:**
- Given übernehmbare Einträge, when übernommen wird, then entspricht die Summe der Ergebnisse der Zahl der Einträge.
- Given ein laufender Import, when parallel im Event-Formular gespeichert wird, then wartet das Speichern bis zum Commit.

## Design Notes

Ein Ergebnis je Eintrag, ermittelt in fester Reihenfolge: zuerst `stale`, dann `error`, dann gleiches Ziel → `error`, dann `unchanged`/`skipped`/`undecided`. Der Rest wird geschrieben. Alles läuft innerhalb von `InTx`, damit die Neuklassifizierung den Stand unter der Sperre sieht. Ein Fehler beim Aussortieren betrifft nur den Eintrag; ein Fehler beim Schreiben bricht die ganze Transaktion ab.

## Verification

**Commands:**
- `go test ./...` (mit `CI=`) und Postgres-Tests mit `-p 1` -- grün
- `bash scripts/check-coverage.sh` -- 100 %
- golangci-lint v2.14.0 -- keine Befunde
- sqlc generate -- nur erwartete Änderungen

## Implementation Notes

- Sperre als sqlc-Query `TakeWriteLock` (`queries/locks.sql`), Kennung `writeLockID = 0x6f7a2d6576656e74` („oz-event“).
- `ImportEntry.NewLocation` neu: das Formular schickt es als `newLocation-<pos>` zurück.
- Gleiches Ziel zählt nur `update` und Überschreiben (wie in der Spec); ein `unchanged`-Eintrag auf dasselbe Ziel bleibt `unchanged`.
- Neue Orte nehmen die Angaben des ersten gültigen Eintrags der Datei, der sie mitbringt, auch wenn dieser Eintrag selbst nicht geschrieben wird.
- Formularfelder: `content`, je Eintrag `position`, `class-<pos>`, `target-<pos>`, `newLocation-<pos>`, `choice-<pos>` (`skip`, `create`, `overwrite:<id>`).

## Spec Change Log

## Review Triage Log
