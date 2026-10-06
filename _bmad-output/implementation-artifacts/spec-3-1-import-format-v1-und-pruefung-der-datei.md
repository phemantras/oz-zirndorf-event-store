---
title: 'Story 3.1: Import-Format v1 und Prüfung der Datei'
type: 'feature'
created: '2026-10-05'
status: 'done'
route: 'dispatch'
baseline_commit: '94eef68df43c095bb4334e28873e48685286f31b'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-3-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Recherchen kommen bisher nur Event für Event über das Admin-Formular in den Bestand. Es fehlen ein dokumentiertes, versioniertes Import-Format und eine Prüfung, die eine Datei liest und Fehler je Eintrag meldet. Außerdem fehlen die Längengrenzen aus ENT-24, die für Formular und Import gleich gelten sollen.

**Approach:** `api/v1/import-v1.schema.json` bindet `EventInput` per `$ref` aus `openapi.yaml` ein und wird öffentlich ausgeliefert. Die Grenzen stehen als `maxLength`/`maxItems` in der Spec, und der Kern spiegelt sie. Ein Kern-Anwendungsfall `PreviewImport` parst die Datei und prüft jeden Eintrag mit denselben Regeln wie `SaveEvent`, über `core.EventInput` mit mitgebrachtem Ort. Eine Admin-Seite `/admin/import` lädt die Datei hoch und zeigt das Ergebnis. Es wird nichts geschrieben.

## Boundaries & Constraints

**Always:**
- Grenzen nach `normalizeText`, gezählt in Codepoints, im Kern (`newEvent`, `newLocation`, Import). Dabei gelten:
  - `title`, `location.name`, `street`, `importKey` höchstens 200;
  - `city` höchstens 100;
  - `source.description` und die Beschreibung eines Programmpunkts höchstens 500;
  - `note` (Event und Ort) und `source.url` höchstens 2000;
  - höchstens 100 Programmpunkte.
- Neue Probleme `tooLong` und `tooMany`. `FieldError` bekommt `Limit int` (0, wenn ohne Grenze), damit Meldungen Feld und Grenze nennen.
- Datei-Format: Objekt mit `formatVersion` (Integer, nur `1`) und `events` (Array mit mindestens einem Eintrag). Fehler je Eintrag nennen die Position (1-basiert), den Titel (falls lesbar) und die Feldpfade wie im Schema: `location.name`, `location.address.street`, `timetable[2].date`.
- Ort im Import: `location` fehlt → `location` missing. Der Name ist Pflicht und wird gegen den Bestand per `NormalizeKey` abgeglichen. Bei einem vorhandenen Namen werden die übrigen Ortsfelder ignoriert. Ein neuer Name wird vollständig wie bei `SaveLocation` geprüft (`newLocation`), Feldpfade mit Präfix `location.` bzw. `location.address.`.
- Unbekannte JSON-Felder werden ignoriert, wie die Spec es für v1 vorsieht. Ein JSON-Typfehler in einem Eintrag macht nur diesen Eintrag fehlerhaft (`invalidFormat` am Feld).
- Admin-Formulare zeigen `tooLong`/`tooMany` als deutsche Meldung am Feld (Programmpunkte über `TimetableError`), und die Eingaben bleiben erhalten. `maxEventFormBytes` steigt auf 256 KiB, `maxLocationFormBytes` auf 64 KiB, weil eine Ortsnotiz mit 2000 Zeichen URL-kodiert sonst nicht hineinpasst.

**Never:** Nichts in den Bestand schreiben; keine Klassifizierung (3.2); keine Migration, kein `import_key`; keine neue Modulabhängigkeit (YAML wird im Test mit einem kleinen eigenen Leser gelesen); `api.gen.go` nicht von Hand ändern.

## I/O & Edge-Case Matrix

| Szenario | Eingabe | Ergebnis |
|---|---|---|
| Gültige Datei | 2 Einträge, einer mit vorhandenem Ortsnamen, einer mit vollständigem neuen Ort | beide gültig, Seite nennt 2 gültig, 0 fehlerhaft |
| Ganze Datei abgelehnt | kein JSON / keine `formatVersion` / Version ≠ 1 / `events` fehlt oder leer / > 2 MiB | `*ImportFileError` mit Grund, eine deutsche Meldung, keine Eintragsliste |
| Fehler je Eintrag | ungültiges Datum, unbekannter Typ, ohne `location`, ohne Ortsnamen, neuer Ort ohne Adresse/Koordinaten/Genauigkeit, PLZ `9051` | nur dieser Eintrag fehlerhaft, mit Position, Titel und Grund je Feld |
| Grenze | `title` mit 201 Zeichen; 101 Programmpunkte | `title tooLong 200` bzw. `timetable tooMany 100` |
| Formular | Event-Titel oder Ortsnotiz über der Grenze speichern | 422, Meldung „Höchstens 200 Zeichen.“ am Feld, Eingaben erhalten |

</frozen-after-approval>

## Code Map

- `api/v1/openapi.yaml` -- `EventInput`, `EventInputLocation`, `Address`, `Source`, `TimetableEntry`: `maxLength`/`maxItems` ergänzen; `importKey` bekommt `maxLength: 200`.
- `api/v1/spec.go` -- `OpenAPISpec` einbetten; daneben `ImportSchemaV1` per `//go:embed import-v1.schema.json`.
- `internal/adapter/publicapi/v1/handler.go` -- statische Pfade nach dem Muster von `specPath`; `/v1/import-v1.schema.json` mit `application/schema+json`.
- `internal/adapter/publicapi/v1/enum_test.go` -- liest `api.gen.go` per `go/parser`; als Muster für den Feldnamen-Test (Struct-Tags walken).
- `internal/core/event.go` -- `EventInput`, `normalized`, `Canonicalize`, `newEvent` (meldet heute `locationId` missing), `EventField*`.
- `internal/core/location.go` -- `LocationInput`, `newLocation`, `LocationField*`.
- `internal/core/timetable.go` -- `parseTimetable`, `TimetableField`, `SplitTimetableField`.
- `internal/core/errors.go` -- `FieldProblem`, `FieldError`, `ValidationError`.
- `internal/core/location_service.go` -- `LocationRepo.List` liefert die Orte für den Namensabgleich.
- `internal/adapter/admin/forms.go` -- `fieldErrorMessages` sucht per `map[core.FieldError]string`: Den Schlüssel ohne `Limit` bilden.
- `internal/adapter/admin/events.go`, `locations.go`, `new_location.go`, `timetable.go` -- Meldungs-Maps, `maxEventFormBytes`, `maxLocationFormBytes`, `TimetableError`.
- `internal/adapter/admin/handler.go` -- `Config`, Routen unter `protected`; `templates/home.html` mit Navigationslinks.
- `cmd/eventstore/main.go` -- Verdrahtung der Services.

## Nahtstellen

- `newEvent` / `SaveEvent` -- 1.7/1.9 -- das Admin-Formular meldet weiter `locationId` missing, wenn weder `LocationID` noch `Location` gesetzt ist; neu kommen Grenzen hinzu -- bestehende `event_service_test.go` plus neue Grenz-Tests.
- `newLocation` / `SaveLocation` und „Neuer Ort“ im Event-Formular -- 1.4/1.8/1.12 -- neue Grenzen; Fehler werden in beiden Formularen am Feld gezeigt, Eingaben bleiben erhalten -- Tests in `locations_test.go`, `new_location_test.go`.
- `fieldErrorMessages` -- 1.4/1.7 -- die Suche ignoriert `Limit`, bestehende Meldungen bleiben unverändert -- vorhandene Admin-Tests.
- `parseTimetable` / `TimetableError` -- 1.9 -- mehr als 100 Programmpunkte ergeben eine Meldung im Ablaufplan-Block, die Einträge bleiben stehen -- neuer Test in `timetable_test.go` (admin).
- Formulargrenzen -- 1.7/1.4 -- `events_test.go:614`, `locations_test.go:783`, `delete_test.go:343` und `new_location_test.go:298` nutzen die Konstanten und bleiben gültig.
- `RecomputeDerived` -- 2.5b -- ruft `newEvent`/`newLocation` nicht auf, deshalb fallen Altdaten über der Grenze nicht unter „prüfen“; erst beim nächsten Speichern greift die Grenze -- kein Code nötig, steht in den Implementation Notes.

## Tasks & Acceptance

**Execution:**
- [x] `internal/core/errors.go` -- `ProblemTooLong`, `ProblemTooMany`, `FieldError.Limit` -- Grund nennt Grenze.
- [x] `internal/core/limits.go` (+ Test) -- exportierte Grenz-Konstanten und `checkLength(field, text, limit)` -- eine Stelle, test-first.
- [x] `internal/core/event.go`, `location.go`, `timetable.go` (+ Tests) -- Grenzen prüfen; `EventInput` um `Location *LocationInput` und `ImportKey` erweitern; `EventFieldLocation`, `EventFieldImportKey`, `LocationFieldAddress` ergänzen; die Prüfung auf `locationId` nur, wenn `Location == nil`.
- [x] `internal/core/import.go` (+ `import_test.go`) -- `MaxImportFileBytes = 2 << 20`, `ImportFormatVersion = 1`, `ImportFileError{Problem}` (Wraps `ErrValidation`), `ImportService.PreviewImport(ctx, data) (ImportPreview, error)` mit `ImportEntry{Position, Title, Input, Problems}`. Unit-Tests ohne DB mit Fake-`LocationRepo` für jede Zeile der Matrix.
- [x] `api/v1/openapi.yaml`, `api/v1/import-v1.schema.json`, `api/v1/spec.go` -- Grenzen, neues Schema (JSON Schema 2020-12, `$ref: openapi.yaml#/components/schemas/EventInput`, `formatVersion` `const: 1`, `events` `minItems: 1`), eingebettet; danach `go generate ./...`.
- [x] `internal/adapter/publicapi/v1/handler.go` (+ Test) -- `GET /v1/import-v1.schema.json`, CORS wie die übrigen statischen Pfade.
- [x] `internal/adapter/publicapi/v1/contract_test.go` -- (a) jede Kern-Konstante `EventField*`, `LocationField*`, `TimetableField*`, `FilterField*` kommt als Feld oder Pfad in `EventInput`/`Event`/`EventInputLocation`/`EventLocation`/`Address`/`TimetableEntry` bzw. als Parameter in `List*Params` aus `api.gen.go` vor; Ausnahmeliste `{EventFieldLocationID}`. (b) Grenz-Konstanten gleich `maxLength`/`maxItems` in `openapi.yaml`, gelesen mit einem kleinen Leser für Einrückungspfade, mit Fixture-Test. (c) Das Import-Schema ist gültiges JSON, alle `$ref` zeigen auf vorhandene Schemas, und es enthält kein `locationId` und kein `id`.
- [x] `internal/adapter/admin/import.go`, `templates/import.html` (+ Test) -- `GET`/`POST /admin/import` (multipart, Feld `file`, `MaxBytesReader` 2 MiB + 64 KiB) mit dem Ergebnis: Anzahl gültig/fehlerhaft, Tabelle Position | Titel | Status | Gründe. Gründe als „<Feldname deutsch>: <Meldung>“. Ganz abgelehnte Dateien zeigen eine Meldung je Grund. Dazu der Personendaten-Hinweis und ein Link auf `home.html`.
- [x] `internal/adapter/admin/forms.go`, `events.go`, `locations.go`, `timetable.go` (+ Tests) -- Meldungen „Höchstens N Zeichen.“ / „Höchstens N Programmpunkte.“, Formulargrenzen anheben.
- [x] `internal/adapter/admin/handler.go`, `cmd/eventstore/main.go` -- `Config.Imports`, Route, Verdrahtung.

**Acceptance Criteria:**
- Given `GET /v1/import-v1.schema.json`, when abgerufen, then 200 mit dem eingebetteten Schema, `Access-Control-Allow-Origin: *`.
- Given eine Kern-Feldkonstante, die in `openapi.yaml` fehlt, oder eine abweichende Grenze, when die CI läuft, then schlägt der Vertragstest fehl.
- Given eine angemeldete Sitzung ohne gültiges CSRF-Muster bzw. ohne Sitzung, when `POST /admin/import`, then 403 bzw. Weiterleitung zum Login wie bei den übrigen Admin-Routen.
- Given eine geprüfte Datei, when das Ergebnis erscheint, then ist der Bestand unverändert.

## Design Notes

Die Datei wird in zwei Stufen gelesen: Zuerst wird die oberste Ebene als `struct{FormatVersion, Events json.RawMessage}` gelesen. Daraus ergeben sich die Gründe für eine ganz abgelehnte Datei: `invalidJson`, `missingFormatVersion`, `unknownFormatVersion`, `missingEvents`, `noEntries`, `tooLarge`. Danach wird jeder Eintrag einzeln in ein internes Struct mit Zeigerfeldern gelesen. `null` heißt fehlend, Koordinaten werden per `strconv.FormatFloat(v, 'f', -1, 64)` zu Text und gehen in `LocationInput`. So bleiben alle Regeln in `newEvent`/`newLocation`.

## Verification

**Commands:**
- `go test ./...` (mit `CI=`) -- grün
- `bash scripts/check-coverage.sh` -- 100 % für core, publicapi/v1, admin
- `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./...` -- keine Befunde
- `go generate ./...` -- `git status` zeigt nur die erwartete Änderung in `api.gen.go`

## Implementation Notes

- `RecomputeDerived` ruft `newEvent`/`newLocation` nicht auf. Altdaten über einer Grenze fallen deshalb nicht unter „prüfen“; die Grenze greift erst beim nächsten Speichern im Formular.
- `newEvent` verlangt `locationId` nur, wenn `Location == nil`. Der Import setzt für einen Eintrag ohne `location` intern einen leeren Ort, damit statt `locationId` nur `location` missing gemeldet wird.
- JSON-Typfehler werden am Feld als `invalidFormat` gemeldet; Regelverstöße desselben Felds oder darin liegender Felder entfallen dann, damit ein Feld nicht doppelt erscheint. Ein Typfehler in einem Ortsfeld zählt auch bei vorhandenem Ortsnamen, wie das Schema es verlangt.
- Ein Eintrag, der kein JSON-Objekt ist, wird mit dem Feldpfad `""` (`ImportFieldEntry`, deutsch „Eintrag“) gemeldet.
- Die Admin-Seite antwortet bei ganz abgelehnter Datei mit 422, bei einer Anfrage über 2 MiB + 64 KiB mit 413, ohne Multipart-Formular mit 400.

## Spec Change Log

## Review Triage Log

| # | Quelle | Befund | Urteil | Weg | Beleg |
|---|---|---|---|---|---|
| 1 | verification-gap | Verdrahtung `imports` in `run()` von keinem Test über `/admin/import` geprüft | medium | patch | `startRun`-Tests rufen nur Orte/Events ab; `emptyImports` ersetzt den echten Dienst; fehlt die Zeile in `main.go`, bleibt alles grün und der Upload in Produktion scheitert an nil. |
| 2 | verification-gap, blind | Feldnamen-Vertragstest prüft gegen die Vereinigung aller Schemas | medium | patch | `note`, `startTime`, `endTime`, `description` kommen in mehreren Schemas vor; eine Umbenennung in `EventInput` bliebe unbemerkt, der Import läse dann einen veralteten Schlüssel. |
| 3 | verification-gap | Kein Test für Typfehler in einem Ortsfeld bei vorhandenem Ortsnamen | low | patch | Alle Typfehler-Fälle nutzen den neuen Ort „Alte Veste“; das in den Implementation Notes festgelegte Verhalten ist ungetestet. Fix ist ein Testfall. |
| 4 | blind, edge | UTF-8-BOM lässt die ganze Datei als ungültiges JSON scheitern | medium | patch | `encoding/json` lehnt ein führendes BOM ab; Windows-Werkzeuge (PowerShell 5.1 `Out-File`) schreiben es. |
| 5 | verification-gap, blind, edge | `formatVersion: 1.0` wird als unbekannte Version abgelehnt, obwohl das Schema es annimmt | low | patch | `json.Unmarshal` in `int` scheitert an `1.0`; JSON Schema 2020-12 wertet `1.0` als Integer 1. |
| 6 | blind, edge | Kommentar zu `maxEventFormBytes` verspricht Platz für jede Grenze, gilt aber nur für ASCII | low | patch | 100 × 500 `ü` sind URL-kodiert 300 000 Bytes > 256 KiB; der Wert ist im Intent fest, nur der Kommentar ist falsch. |
| 7 | seam | `TestEventTextsOverTheirLimitsShowTheLimitAndKeepInput` belegt die Notiz-Meldung nicht und prüft `source.description` nicht | low | patch | `source.url` und `note` liefern dieselbe Meldung; ein einmaliges Vorkommen genügt der Assertion. |
| 8 | verification-gap, blind | `last_updated` in `sprint-status.yaml` läuft rückwärts | low | patch | 17:00 → 15:55; wird beim Status-Sync korrigiert. |
| 9 | blind, seam | `maxLength` an `Address`/`Source`/`TimetableEntry` gilt auch für die Leseform | low | reject | Intent und Epic-Kontext legen die Grenzen ausdrücklich in die eingebundenen Schemas; Altdaten über der Grenze sind bei wenigen handgepflegten Events unwahrscheinlich, eigene Schreib-Schemas wären mehr als eine direkte Korrektur. |
| 10 | blind, edge | Import-Schema ohne `$id`, `$ref` zeigt auf YAML | low | reject | Der Intent verlangt genau `$ref: openapi.yaml#/components/schemas/EventInput` (AD-9); die Prüfung gegen das Schema ist Thema von 3.4. |
| 11 | blind | Kein gemeinsamer Test Schema gegen Kern; `null` bei nicht-nullbaren Feldern | low | reject | Der Kern ist bei `null` nur nachsichtiger als das Schema, ohne Schaden; der Fall `1.0` ist in #5 erfasst. |
| 12 | blind, edge | Koordinate `1e400` als Typfehler statt `outOfRange` | low | reject | Unwahrscheinlich; der Fix bräuchte einen eigenen Zweig für Zahlenfehler. |
| 13 | blind, seam | `tooMany` verdrängt die allgemeine Ablaufplan-Meldung | low | reject | Die Meldungen je Eintrag bleiben sichtbar; mehr als 100 Programmpunkte und zugleich fehlerhafte Einträge sind unwahrscheinlich, beide Meldungen zu verbinden wäre zusätzliche Logik. |
| 14 | blind, edge | Zwei Probleme an `source.url`, das Formular zeigt nur eines | low | reject | Nur bei einer URL, die zugleich kein http(s) und über 2000 Zeichen ist; das zweite Problem erscheint beim nächsten Speichern. |
| 15 | blind | Tests, die nur Konstanten wiederholen | low | reject | Sie halten die im Intent festgelegten Werte (256 KiB, 64 KiB, 2 MiB, Pfad) fest; ohne Schaden. |
| 16 | blind | Leere Titelzelle bei unlesbarem Titel, Personendaten-Hinweis doppelt | low | reject | Kosmetisch; die Position identifiziert die Zeile, und der Hinweis ist im Intent verlangt. |
| 17 | blind | Feldpfade nicht gegen das Schema getestet, unbekannter Pfad roh angezeigt | low | reject | `TestImportLabelsEveryFieldOfTheFormatInGerman` deckt alle heutigen Pfade ab; ein Rohpfad als Rückfall ist harmlos. |
| 18 | edge | 2 MiB aus winzigen Einträgen ergeben eine riesige Ergebnistabelle | low | reject | Nur angemeldeter Admin, selbst verursacht; eine Eintragsgrenze wäre neue öffentliche Fläche. |
| 19 | edge | Typfehler an `startDate` zeigt die Formatmeldung des Formulars statt „falscher Typ“ | low | reject | Die Meldung „kein gültiges Datum (JJJJ-MM-TT)“ führt trotzdem zur richtigen Korrektur. |
| 20 | edge | Typfehler in Ortsfeldern bei vorhandenem Namen machen den Eintrag fehlerhaft, obwohl die übrigen Felder ignoriert werden | low | reject | Bewusste, dokumentierte Auslegung (Implementation Notes): Typfehler gelten wie im Schema; selten, und beide Lesarten sind schemakonform vertretbar. |
| 21 | edge | Nur ein Grund je ganz abgelehnter Datei | false | reject | Design Notes legen die Gründe als Stufenfolge fest; „eine Meldung je Grund“ heißt eine eigene Meldung für jeden Grund, und jede Stufe hat ihre. |
| 22 | seam | `SaveEvent` mit `Location` und leerer `LocationID` schreibt ohne Ort | low | reject | Heute kein Aufrufer setzt `Location`; der Fall endet laut im FK-Fehler (500), nicht still. Für 3.3 relevant, dort löst `CommitImport` den Ort vor `SaveEvent` auf. |
| 23 | seam | Bearbeiten mit Grenzverletzung nicht getestet | low | reject | Bearbeiten und Anlegen teilen `renderEventFormAgain`/`locationFormPageAgain`; die Anlage-Tests belegen das Verhalten. |
