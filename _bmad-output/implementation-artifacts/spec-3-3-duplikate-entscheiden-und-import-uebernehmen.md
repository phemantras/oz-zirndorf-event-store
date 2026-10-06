---
title: 'Story 3.3: Duplikate entscheiden und Import übernehmen'
type: 'feature'
created: '2026-10-06'
status: 'done'
baseline_commit: 'a09afeb3d3fdbbf85f84a411a826978842399808'
route: 'dispatch'
review_loop_iteration: 1
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
- `stale`: Klasse, Ziel-ID oder „Ort neu“ weicht von der Formularangabe ab, oder das gewählte Überschreib-Ziel ist kein aktueller Bestands-Kandidat mehr, oder ein `duplicateSuspect` hat jetzt einen Bestands-Kandidaten, den die Vorschau nicht zeigte. Dafür schickt das Formular die Bestands-Kandidaten der Vorschau mit (Entscheidung nach Review 1).
- Eine Datei hat höchstens `MaxImportEntries` = 150 Einträge, sonst wird sie als Ganzes abgelehnt: „Die Datei enthält mehr als 150 Events.“ Das Import-Schema setzt `maxItems: 150`. So bleibt das Formular unter dem Multipart-Limit von 1000 Teilen (Entscheidung nach Review 1).
- `error`: Klasse `error`; zwei Einträge mit demselben Ziel (`update` oder Überschreiben), dann beide; beim Überschreiben ein anderer, nicht leerer `importKey` am Ziel.
- Schreiben in einer Transaktion: Neue Orte werden einmal je `NormalizeKey` angelegt (Angaben des ersten Eintrags), und nur solche, die ein übernommener Eintrag braucht. Danach Events über den Tx-Kern von `saveEvent` mit `AllowDuplicates`. Überschreiben und `update` ersetzen das Ziel samt Ablaufplan und löschen `archived_at`. `import_key` wird normalisiert gesetzt; ohne Schlüssel im Eintrag bleibt der des Ziels.
- Nach dem Commit werden die Prüf-Markierungen der geschriebenen Events gelöscht.
- Ein unerwarteter DB-Fehler rollt alles zurück. Admin: Seite mit „Der Import ist fehlgeschlagen; es wurde nichts übernommen.“ (500).
- Die Ergebnisseite zeigt die 7 Zahlen und die Zahl neu angelegter Orte. Die Summe der Einträge ist die Zahl der Einträge in der Datei. Darunter listet sie die Einträge mit `stale`, `undecided` und `error` mit Position, Titel und Ergebnis (Entscheidung nach Review 1).
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
| Doppelklick | zweiter Commit derselben Form, auch bei Wahl „als neues Event anlegen“ ohne `importKey` | `stale`/`unchanged`, kein Duplikat |
| Neuer Kandidat | Verdacht, inzwischen kam ein weiterer Bestands-Kandidat hinzu | `stale` |
| Zu viele Einträge | Datei mit 151 Einträgen | Dateifehler bei Vorschau und Commit |
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
- Review 1: `internal/core/import.go` -- `MaxImportEntries = 150` neben `MaxImportFileBytes`, `ImportProblemTooManyEntries` (`"tooManyEntries"`) in `readImportFile` nach der Leer-Prüfung; `api/v1/import-v1.schema.json` `events.maxItems: 150`, Gleichheit per Test in `internal/adapter/publicapi/v1/contract_test.go` (wie die übrigen Grenzen, AD-9).
- Review 1: `internal/core/import_commit.go` -- `ImportDecision.CandidateIDs []string`; `isStale`: bei `duplicateSuspect` ist ein aktueller Kandidat mit `EventID` außerhalb von `CandidateIDs` veraltet (unabhängig von der Wahl).
- Review 1: `internal/adapter/admin/import.go`, `import.html` -- verstecktes Feld `candidates-<pos>` (IDs der Bestands-Kandidaten, durch Leerzeichen getrennt, nur bei `duplicateSuspect`); Meldung für `tooManyEntries`; Ergebnisliste aus `ImportSummary.Results`; Überschrift neutral „Ergebnis des Imports“; Kopfhinweis „nichts gespeichert“ nur beim Prüfen, nicht auf der Ergebnisseite, Text: „Beim Prüfen wird nichts gespeichert.“; Entscheidungen in `<fieldset>` mit `<legend>`.

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
- [x] `internal/core/import.go`, `import-v1.schema.json`, `contract_test.go` (+ Tests) -- `MaxImportEntries`, Dateifehler, `maxItems`.
- [x] `internal/core/import_commit.go` (+ Test) -- `CandidateIDs`, neuer Kandidat → `stale`; zweiter Commit mit Wahl „create“ ohne Schlüssel → `stale`.
- [x] `internal/adapter/admin/import.go`, `import.html` (+ Tests) -- Feld `candidates-<pos>`, Meldung zu vielen Einträgen, Ergebnisliste, neutrale Überschrift, Kopfhinweis nur beim Prüfen, `fieldset`/`legend`.
- [x] `cmd/eventstore/import_test.go` -- Doppel-Commit mit Duplikatverdacht und Wahl „create“ ohne Schlüssel speichert einmal; Commit, der ein beim Start markiertes Event per Schlüssel repariert, entfernt „prüfen“ im Admin und zeigt es in der öffentlichen Liste.
- [x] `internal/adapter/postgres/tx_test.go` -- `waitingForAdvisoryLock` nur für die Schreibsperre (`classid`/`objid` aus `writeLockID`).
- [x] `internal/core/event_service.go` -- Doc-Kommentar von `EventRepo` umbrechen; `sprint-status.yaml` auf `review`.

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

- Review 1 (Iteration 1): Befunde 1–3 (intent_gap) vom Menschen entschieden: Kandidaten der Vorschau im Formular, `stale` bei neuem Kandidaten; `MaxImportEntries` = 150 mit Dateifehler und `maxItems`; Ergebnisliste für `stale`/`undecided`/`error`. Ergänzt: eingefrorener Block, Matrix, Code Map, Aufgaben. Vermiedener Zustand: zweiter Klick legt ein zweites Duplikat an; große Dateien scheitern mit nacktem 400. Der Mensch hat entschieden, den Code zu behalten und gezielt nachzuziehen statt zurückzusetzen. KEEP: Sperre in `InTx`, Tx-Kerne `saveLocation`/`saveEvent`, `planImport`-Reihenfolge, alle bestehenden Tests und die Formularfelder aus den Implementation Notes.

## Review Triage Log

| # | Quelle | Befund | Urteil | Begründung | Route |
|---|---|---|---|---|---|
| 1 | edge-case | Doppelklick mit „Als neues Event anlegen“ legt ein zweites Duplikat an | high | Ohne `importKey` bleibt der Eintrag beim zweiten Commit `duplicateSuspect` (jetzt auch gegen das gerade angelegte Event); `isStale` prüft nur Klasse, Ziel, „Ort neu“ und das Überschreib-Ziel, `planEntry` liefert erneut `created` (`import_commit.go:221`). Widerspricht der Matrixzeile „Doppelklick … kein Duplikat“; die eingefrorene `stale`-Definition deckt den Fall nicht ab. | intent_gap |
| 2 | edge-case | Große Dateien scheitern am Multipart-Limit von 1000 Teilen | medium | 3–5 Formularteile je Eintrag (`importHiddenFieldsOf` plus Wahl), `ReadForm` begrenzt auf 1000 Teile; ab etwa 200–330 Einträgen liefert `parseImportForm` `rejectNoForm` (nacktes 400). Die Datei darf 2 MiB groß sein, eine Höchstzahl an Einträgen gibt es nicht. Das eingefrorene Formular legt Multipart mit Feldern je Position fest; Abhilfe (GODEBUG, andere Kodierung, Höchstzahl) ist nicht geklärt. | intent_gap |
| 3 | blind | Ergebnisseite nennt die betroffenen Einträge nicht | low | `importSummaryViewOf` zeigt nur Zahlen, `Results` mit Position und Titel bleiben ungenutzt; bei `stale`/`undecided` weiß die Person nicht, welche Einträge betroffen sind. Der Inhalt der Ergebnisseite steht in der eingefrorenen Absicht. | intent_gap |
| 4 | verification-gap | Gemeinsame `EventService`-Instanz in `main.go` für `NewImportService` ungetestet | medium | Vorgeprüft: Kein cmd-Test prüft, dass ein Commit die Prüf-Markierung in Admin und öffentlicher API löscht; eine eigene Instanz in `main.go:103` bliebe unbemerkt. | patch |
| 5 | seam | Kopfhinweis „es wird nichts gespeichert“ auf Vorschau mit Übernehmen-Knopf und auf der Ergebnisseite | medium | `newImportPage()` setzt immer `NothingSaved`; das Template zeigt ihn in Zeile 6 auch über der Zusammenfassung „neu angelegt: 3“. | patch |
| 6 | blind | Überschrift „Import übernommen“ auch, wenn nichts geschrieben wurde | low | `importSummaryViewOf` setzt immer `msgImportCommitted`; der zweite Doppelklick-Commit zeigt sie bei „veraltet: 1“. Direkte Textkorrektur. | patch |
| 7 | blind | Entscheidungs-Radios ohne `fieldset`/`legend` | low | `import.html` rendert `ChoiceLegend` als `<p>`; Screenreader nennen die Gruppe nicht. Direkte Template-Korrektur. | patch |
| 8 | blind | `waitingForAdvisoryLock` filtert nicht auf die Schreibsperre | low | `tx_test.go:18` zählt jede wartende Advisory-Sperre; ein Warter auf die goose-Sperre ließe `awaitLockWaiter` zu früh zurückkehren. Direkte Korrektur der Abfrage (`classid`/`objid`). | patch |
| 9 | blind | `sprint-status.yaml` steht auf `in-progress`, Spec auf `in-review` | low | Der Kopf der Datei sieht `review` vor dem Code-Review vor. | patch |
| 10 | blind | Doc-Kommentar von `EventRepo` nicht neu umbrochen | low | Eine Zeile deutlich länger als die Nachbarn. | patch |
| 11 | blind | `unchanged` und Überschreiben auf dasselbe Ziel ergeben widersprüchliche Zahlen | low | Selten (zweiter Eintrag ohne Schlüssel überschreibt ein per Schlüssel unverändertes Event); die Abhilfe bräuchte einen weiteren Zweig. | reject |
| 12 | blind | Rückrollen bei DB-Fehler nicht gegen PostgreSQL geprüft | low | `CommitImport` schreibt vollständig in einem `InTx`; das Rückrollen leistet der `TxRunner`. Die Abhilfe wäre ein zusätzlicher Fehlerinjektions-Test. | reject |
| 13 | blind | Löschen von `archived_at` beim Import-Update ungetestet | low | `UpdateEvent` setzt `archived_at = NULL` (`queries/events.sql:37`), der Import schreibt über denselben `saveEvent`-Pfad. | reject |
| 14 | blind | Doppelklick-Test beweist keine Überlappung | low | Das Warten an der Sperre beweisen die Tests in `tx_test.go`; der cmd-Test prüft das Ergebnis. | reject |
| 15 | blind | Keine Zeitgrenze beim Warten auf die Sperre | false | Warten ist die verlangte Wirkung (AC „wartet das Speichern bis zum Commit“); abgebrochene Anfragen beenden die Tx über den Kontext. | reject |
| 16 | blind | `ImportService` greift auf private Felder von `EventService` zu | false | So in der Code Map festgelegt (`NewImportService(events *EventService)`), gleiches Paket; kein benannter Aufrufer weicht ab. | reject |
| 17 | blind | `locationID` kann bei verletzter Invariante nil dereferenzieren | maybe-false | Nur wenn `NewLocation` und `isNewLocation()` auseinanderlaufen; `isStale` erzwingt die Übereinstimmung. Wäre höchstens low. | reject |
| 18 | edge-case | Typisierte Kernfehler beim Schreiben ergeben 500 | false | Die Neuklassifizierung in derselben Tx validiert dieselbe Eingabe; ein inzwischen vorhandener Ort macht den Eintrag `stale`; `AllowDuplicates` schließt `ErrConflict` aus. | reject |
| 19 | edge-case | Orte werden verzahnt statt vor den Events angelegt | false | Eine Transaktion, gleiches Ergebnis; nur Orte, die ein geschriebener Eintrag braucht, wie verlangt. | reject |
| 20 | verification-gap | Test zur Savepoint-Konfliktbehandlung hängt von der Reihenfolge von `ListLocations` ab | false | `RecomputeNameKeys` sortiert über `SortLocations`; die Reihenfolge ist deterministisch. | reject |
| 21 | verification-gap (R2) | Kein Commit-Test mit `MaxImportEntries` Einträgen prüft das Multipart-Limit | medium | Vorgeprüft: alle Commit-Tests senden 1–4 Einträge; im schlimmsten Fall 150 × 6 + 1 = 901 Teile, ein Anheben fiele unbemerkt durch. | patch |
| 22 | blind (R2) | Ergebnisliste ohne Erklärung, was mit den Einträgen zu tun ist | low | Die Tabelle steht ohne Einleitung da; eine Zeile Hinweistext reicht. | patch |
| 23 | blind (R2) | `TestEventRepoSetsTheImportKey` nutzt `unknownLocationID` als Event-ID | low | `import_key_test.go:149`; `unknownEventID` existiert in `events_test.go:18`. | patch |
| 24 | blind/edge (R2) | LF→CRLF beim Zurücksenden des versteckten Dateiinhalts vergrößert die Datei | low | Browser normalisieren Zeilenenden in multipart; nur Dateien knapp unter 2 MiB mit vielen Zeilen scheitern dann beim Commit mit „größer als 2 MiB“. Bei höchstens 150 Einträgen praktisch nicht erreichbar; Abhilfe (Base64, Größenvergleich) braucht neue Kodierung. | reject |
| 25 | blind (R2) | Kein Post/Redirect/Get nach dem Commit | low | Neu laden zeigt alles als „veraltet“, schreibt aber nichts; PRG bräuchte Zwischenstand, den die Absicht verbietet. | reject |
| 26 | blind (R2) | Irreführende Meldungen bei handgebauten Commit-Anfragen (zu groß, ohne `content`) | low | Nur bei manipulierten Formularen erreichbar. | reject |
| 27 | blind (R2) | Erfolgreicher Commit wird nicht geloggt | low | Kein anderer Schreib-Anwendungsfall loggt Erfolge; keine Anforderung. | reject |
| 28 | blind (R2) | Kommentare „deleted meanwhile“ in den Neuberechnungen veraltet | false | Schreibvorgänge außerhalb des `TxRunner` (z. B. per SQL) bleiben möglich; der Zweig und sein Kommentar beschreiben weiter zutreffendes Verhalten. | reject |
| 29 | blind (R2) | `UpdateNameKey` auf dem Pool umgeht die Sperre | false | Einziger Aufrufer ist `recomputeNameKeys` über `repos.Locations` in `InTx`; `s.repo` ruft es nicht auf. | reject |
| 30 | blind (R2) | Doppelte oder unbekannte `position`-Werte werden still übergangen | low | Nur bei manipulierten Formularen; fehlende Entscheidung ergibt `stale`. | reject |
| 31 | blind (R2) | `export_test.go` ist nicht versioniert | false | Neue Datei des Arbeitsstands; sie wird mit der Story committet. | reject |
| 32 | blind (R2) | Kein Test sendet das gerenderte Vorschau-Formular unverändert zurück | low | Schreiber und Leser teilen dieselben Feldkonstanten; ein HTML-Parsing-Test wäre zusätzliche Komplexität. | reject |
| 33 | edge (R2) | Veralteter `name_key` im Bestand lässt das Anlegen eines „neuen“ Orts mit 500 scheitern | maybe-false | Nur wenn `RecomputeNameKeys` einen Ort wegen Kollision nicht umschlüsseln konnte und die Datei genau diesen Namen bringt; der Ort ist dann zur Prüfung markiert. Höchstens low. | reject |
| 34 | edge (R2) | Fehlschlag in `RecomputeDerived` rollt alle Neuberechnungen zurück | false | So verlangt („laufen je in einer Transaktion“); ein Fehler bricht den Start ohnehin ab. | reject |
