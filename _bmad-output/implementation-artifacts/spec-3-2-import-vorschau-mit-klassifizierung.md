---
title: 'Story 3.2: Import-Vorschau mit Klassifizierung'
type: 'feature'
created: '2026-10-06'
status: 'done'
baseline_commit: 'a9d548964011faa08d062611b8a82ed7108f14bc'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-3-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Die Import-Prüfung aus 3.1 sagt nur „gültig“ oder „fehlerhaft“. Andreas sieht nicht, ob ein Eintrag neu ist, ein vorhandenes Event aktualisiert, unverändert ist oder ein Duplikat sein könnte, und auch nicht, welche Orte neu angelegt würden.

**Approach:** Neue Spalte `events.import_key` (optional, eindeutig, wenn gesetzt), nur gelesen. `PreviewImport` klassifiziert jeden Eintrag gegen den ganzen Bestand samt Archiv in genau eine Klasse und führt neue Orte gesondert. Die Admin-Seite `/admin/import` zeigt Anzahl je Klasse, Klasse, Grund, Ziel-Event (verlinkt), Änderungen alt/neu und Hinweise. Es wird nichts geschrieben und nichts auf dem Server zwischengespeichert.

## Boundaries & Constraints

**Always:**
- Klassen `new`, `update`, `unchanged`, `duplicateSuspect`, `error`; Vorrang `error` > `update`/`unchanged` > `duplicateSuspect` > `new`.
- `error`: Problem aus 3.1 oder ein `importKey`, den mehrere Einträge der Datei tragen (alle betroffenen Einträge, neues Problem `duplicateInFile` an `importKey`). Verglichen wird der normalisierte Schlüssel exakt.
- `importKey` im Bestand vorhanden → Ziel ist dieses Event: `unchanged`, wenn die kanonischen Formen gleich sind, sonst `update` mit Liste geänderter Felder (Feldpfad, alt, neu; Ort als Name, Ablaufplan als ein Feld). Weitere Bestands-Events mit gleichem Duplikat-Schlüssel sind nur Hinweis.
- Ohne Treffer per `importKey`: Kandidaten aus `FindDuplicateCandidates` (Titel per `NormalizeKey`, Beginn-Datum, Ort) → `duplicateSuspect`; ebenso, wenn ein anderer gültiger Eintrag der Datei denselben Schlüssel hat (Ort per `NormalizeKey` des Namens). Dann sind beide Einträge `duplicateSuspect`, sofern nicht `update`/`unchanged`. Kandidaten nennen Bestands-Events und/oder Positionen. Sonst `new`.
- Vorhandener Ortsname: Zuordnung zum vorhandenen Ort, der unverändert bleibt; weicht eine angegebene Straße, PLZ, Stadt oder Koordinate ab, gibt es einen Hinweis am Eintrag.
- Neuer Ort: einmal je `NormalizeKey` als `newLocation` geführt, mit den Positionen, die ihn mitbringen. Maßgeblich sind die Angaben des ersten gültigen Eintrags; abweichende Angaben späterer Einträge sind Hinweis. Orte nur fehlerhafter Einträge erscheinen nicht.
- Klassifizierung als reine Kernlogik, Unit-Tests ohne DB mit Fakes, inklusive aller Konflikte innerhalb der Datei und des Vorrangs.

**Never:** Nichts schreiben; keine Entscheidungen, kein `CommitImport`, kein `stale`, keine Advisory-Sperre (3.3); `SaveEvent` und die Create/Update-Queries setzen `import_key` nicht; `importKey` erscheint nicht in der öffentlichen Leseform; angewendete Migrationen nicht ändern; generierten Code nur neu erzeugen.

## I/O & Edge-Case Matrix

| Szenario | Eingabe / Bestand | Ergebnis |
|---|---|---|
| Neu | gültig, kein Schlüssel-Treffer, kein Kandidat | `new` |
| Update | `importKey` = Event X, Titel geändert | `update`, Ziel X, Änderung `title` alt/neu |
| Unverändert | `importKey` = archiviertes X, alles gleich (Leerraum/Reihenfolge des Ablaufplans egal) | `unchanged`, Ziel X |
| Update + Duplikat | `importKey` = X, Schlüssel passt zusätzlich zu Y | `update`, Hinweis auf Y |
| Verdacht Bestand | ohne Schlüssel-Treffer, gleicher Titel (anders geschrieben), Datum, Ort wie Y | `duplicateSuspect`, Kandidat Y |
| Anderes Datum | wie oben, anderes Beginn-Datum | `new` |
| Doppelter Schlüssel | Pos. 1 und 3 mit `importKey` „a“ | beide `error` (`importKey duplicateInFile`) |
| Gleiche Einträge | Pos. 2 und 4 gleicher Titel/Datum/Ortsname | beide `duplicateSuspect`, Kandidat jeweils die andere Position |
| Fehler vor Update | `importKey` = X, Datum ungültig | `error` |
| Neuer Ort doppelt | Pos. 1 und 2 bringen „Neuer Platz“ mit | ein `newLocation` mit Positionen 1, 2 |
| Ort weicht ab | vorhandener Ortsname, andere PLZ | Klasse unverändert, Hinweis zur Adresse |

</frozen-after-approval>

## Code Map

- `internal/core/import.go` -- `PreviewImport`, `readImportEntry` (gibt Eintrag mit Problemen), `importLocationProblems` (Namensabgleich über `storedNames`); `NewImportService(locations)` bekommt das `EventRepo` dazu.
- `internal/core/duplicates.go` -- `FindDuplicateCandidates`, `DuplicateKey`; wiederverwenden, nicht ändern.
- `internal/core/event.go` -- `Event` bekommt `ImportKey`; `newEvent` liefert das geparste Event, `EventInputOf` + `Canonicalize` für den Vergleich (`EventInputOf` setzt `LocationID`, Ortsauflösung vorher).
- `internal/core/errors.go` -- neues `ProblemDuplicateInFile`.
- `internal/adapter/postgres/migrations/00008_event_import_key.sql` -- neu: `text` nullable, eindeutiger Teilindex `WHERE import_key IS NOT NULL`, Kommentarstil wie `00007`.
- `internal/adapter/postgres/queries/events.sql`, `events.go` -- `import_key` in allen `SELECT`/`RETURNING`, `eventFromRow` liest ihn; `CreateEvent`/`UpdateEvent` setzen ihn nicht. Danach sqlc-Release-Binary 1.31.1.
- `internal/adapter/admin/import.go`, `templates/import.html` -- `importResultOf`, `importRow`, Meldungs-Maps; `eventURL(id)` aus `events.go` für Links.
- `cmd/eventstore/main.go:102` -- Verdrahtung; `main_test.go:771` `emptyImports`.

## Nahtstellen

- `PreviewImport` / `ImportEntry` -- 3.1 -- Prüfung der Datei und Probleme je Eintrag bleiben unverändert; ganz abgelehnte Datei weiter `*ImportFileError` -- bestehende `import_test.go`.
- `/admin/import` -- 3.1 -- 422/413/400, CSRF und Personendaten-Hinweis bleiben; Spalte „Status“ zeigt jetzt die Klasse -- bestehende `import_test.go` (admin), angepasst an die Klassen.
- `EventRepo` Lesen / `SaveEvent` -- 1.7/2.x -- Admin-Update lässt `import_key` stehen -- neuer Postgres-Test.
- `run()`-Verdrahtung -- 3.1 -- `main_test.go` Upload-Test bleibt grün mit echtem Dienst.

## Tasks & Acceptance

**Execution:**
- [x] `internal/adapter/postgres/migrations/00008_event_import_key.sql`, `queries/events.sql`, `db/*` (generiert), `events.go` (+ Tests) -- Spalte, Lesen, Eindeutigkeit -- Datenmodell aus der Story.
- [x] `internal/core/event.go`, `errors.go` -- `Event.ImportKey`, `ProblemDuplicateInFile`.
- [x] `internal/core/import.go` (+ `import_test.go`) -- `ImportClass`-Konstanten, `ImportEntry` um `Class`, `TargetID`, `Changes []ImportChange{Field, Old, New}`, `Candidates []ImportCandidate{EventID, Title, StartDate, Position}`, `Hints` erweitern; `ImportPreview.NewLocations []ImportNewLocation{Name, Positions}`, `CountOf(class)`. Lädt `EventRepo.List` einmal (Archiv inklusive), test-first je Zeile der Matrix.
- [x] `internal/adapter/admin/import.go`, `templates/import.html` (+ Test) -- Anzahl je Klasse, deutsche Klassennamen, Ziel/Kandidaten verlinkt, Änderungen alt → neu, Hinweise, Abschnitt „Neue Orte“, Hinweis „wird nichts gespeichert“ bleibt.
- [x] `cmd/eventstore/main.go`, `main_test.go` -- Verdrahtung mit `EventRepo`.

**Acceptance Criteria:**
- Given eine geprüfte Datei, when die Vorschau erscheint, then ist der Bestand unverändert und der Server hält keinen Zwischenstand.
- Given zwei Events mit gleichem gesetztem `import_key`, when gespeichert, then lehnt die Datenbank das zweite ab; `NULL` mehrfach ist erlaubt.
- Given ein Event mit `import_key`, when es im Admin bearbeitet wird, then bleibt `import_key` erhalten.

## Design Notes

Ablauf: Einträge lesen (3.1) → Schlüssel-Konflikte → Bestands-Abgleich je gültigem Eintrag → Datei-Duplikate → neue Orte. Vergleich `unchanged`: `EventInputOf(geparst).Canonicalize()` gegen `EventInputOf(gespeichert).Canonicalize()`, beide mit aufgelöster `LocationID`; ein neuer Ort ist immer eine Änderung. „Ziel-Event mit anderem `importKey`“ (ENT-19) kann beim Upload nicht entstehen, weil das Ziel nur über denselben Schlüssel gefunden wird; es betrifft „überschreiben“ in 3.3.

## Verification

**Commands:**
- `go test ./...` (mit `CI=`) und Postgres-Tests mit `-p 1` -- grün
- `bash scripts/check-coverage.sh` -- 100 % für core, publicapi/v1, admin
- `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./...` -- keine Befunde
- sqlc generate -- `git status` zeigt nur erwartete generierte Änderungen

## Implementation Notes

- `CommitImport` (3.3) muss `import_key` normalisiert (`normalizeText`) speichern: Die Vorschau sucht den normalisierten Schlüssel exakt im gespeicherten Wert, und der eindeutige Index vergleicht den Rohtext.
- Duplikat-Kandidaten aus dem Bestand kommen je gültigem Eintrag an einem vorhandenen Ort über `FindDuplicateCandidates` (eine Abfrage je Eintrag), wie die Story es verlangt; bei rund 40 Einträgen unkritisch.
- Neuer Typ `ImportHint{Kind, Field, Candidate}` mit den Arten `duplicate`, `locationDiffers`, `newLocationDiffers`; `ValidCount`/`ErrorCount` entfallen zugunsten von `CountOf`.
- Das Ziel-Event wird als „vorhandenes Event“ verlinkt; der Kern liefert nur die ID.

## Spec Change Log

## Review Triage Log

| # | Quelle | Befund | Urteil | Weg | Beleg |
|---|---|---|---|---|---|
| 1 | verification-gap, blind | Deutsche Meldung für `importKey duplicateInFile` in keinem Admin-Test geprüft | low | patch | Nur Kern-Tests prüfen den `FieldError`; ein Tippfehler im Map-Schlüssel fiele auf die generische Meldung zurück, ohne dass ein Test scheitert. |
| 2 | blind | Ortshinweis „Straße und Hausnummer weicht … ab“ grammatisch falsch | low | patch | Der Test schreibt genau diesen Text fest; direkte Korrektur der Formatzeichenkette. |
| 3 | blind | `TestNoQueryWritesTheImportKey` scheitert, sobald 3.3 die Commit-Abfrage ergänzt | low | patch | Die Heuristik prüft alle INSERT/UPDATE-Abfragen; die Invariante betrifft nur `CreateEvent`/`UpdateEvent`. |
| 4 | blind | `wantOneValidEntry` prüft jetzt „neu: 1“ | low | patch | Name sagt nicht mehr, was geprüft wird (Clean-Code-Regel). |
| 5 | blind, edge | Gespeicherter `import_key` nicht normalisiert → kein Treffer; zwei nur im Leerraum verschiedene Schlüssel | low | reject | In 3.2 schreibt nichts `import_key`; die Pflicht, normalisiert zu speichern, steht für 3.3 in den Implementation Notes. |
| 6 | blind, edge | N+1-Abfragen und kein gemeinsamer Lese-Snapshot | low | reject | Die Story verlangt `FindDuplicateCandidates`; die Vorschau ist nur Anzeige, 3.3 klassifiziert beim Speichern neu (`stale`). |
| 7 | blind | Kein Klassifizierungstest gegen echtes PostgreSQL | low | reject | Abnahme 3.5 verlangt genau diesen Integrationstest (zweiter Import alle `unchanged`); `EventInputOf` vergleicht Textformen unabhängig von IDs und Zeigern. |
| 8 | blind | Ziel-Link heißt nur „vorhandenes Event“ | low | reject | AC verlangt das Ziel verlinkt, das ist erfüllt; Titel/Datum bräuchten neue Kernfelder. |
| 9 | blind | ISO-Datum in Änderungen, deutsches Datum bei Kandidaten | low | reject | Kosmetisch; Umformatierung je Feldtyp im Adapter wäre zusätzliche Logik. |
| 10 | blind | Keine Vollständigkeitsprüfung der Hinweisarten | low | reject | Alle heutigen Arten haben Texte und sind getestet; erst eine künftige Art könnte fehlen. |
| 11 | blind | Deutsche Texte inline im Template | false | reject | Das Template folgt dem bestehenden Stil (z. B. „Ergebnis“, „Import prüfen“ inline). |
| 12 | blind | Positionen der neuen Orte nicht verlinkt | low | reject | Komfort; die Positionen identifizieren die Zeilen. |
| 13 | blind | Kein Test, dass die öffentliche Leseform `importKey` weglässt | false | reject | Der generierte Typ `Event` hat kein solches Feld; die Abbildung kann es nicht durchreichen. |
| 14 | blind | `unchanged`-Zeile im Admin nicht gerendert getestet | low | reject | Gleicher Template-Pfad wie `update` ohne Änderungen; Kern-Test belegt Klasse und Ziel. |
| 15 | edge | Koordinaten nach Speicherrundung als abweichend gemeldet | false | reject | Spalten sind `double precision`, `parseCoordinate` liest float64; es gibt keine Rundung. |
| 16 | seam | Admin-Testfake `memoryEventRepo.Update` verliert `ImportKey` | low | reject | Nur Testfake; das echte Verhalten belegt `TestSavingAnEventInTheAdminKeepsItsImportKey` gegen PostgreSQL. |
