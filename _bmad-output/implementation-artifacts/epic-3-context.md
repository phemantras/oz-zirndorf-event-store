# Epic 3 Context: Andreas importiert Recherchen per JSON-Datei

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

Epic 3 bringt Recherchen per JSON-Datei in den Bestand, statt jedes Event einzeln im Admin anzulegen. Andreas lädt eine Datei in einem dokumentierten, versionierten Format hoch, sieht eine Vorschau mit Klasse je Eintrag (neu, Aktualisierung, unverändert, Duplikatverdacht, fehlerhaft) und die neu anzulegenden Orte, entscheidet jeden Duplikatverdacht und übernimmt alles in einer Transaktion mit Zusammenfassung. Nichts wird stillschweigend verdoppelt oder verworfen, und ein zweiter Import derselben Datei ändert nichts. Dieselben Regeln wie im Admin-Formular gelten, weil Import und Formular denselben Eingabetyp und dieselben Kern-Anwendungsfälle nutzen. Am Ende ist die Testsammlung im Format v1 importiert (SM-2), und der Weihnachtsmarkt ist in Produktion über die öffentliche API abrufbar (SM-1).

## Stories

- Story 3.1: Import-Format v1 und Prüfung der Datei
- Story 3.2: Import-Vorschau mit Klassifizierung
- Story 3.3: Duplikate entscheiden und Import übernehmen
- Story 3.4: Testsammlung ins Format v1 überführen
- Story 3.5: Testsammlung importieren und Abnahme

## Requirements & Constraints

- **Format:** `api/v1/import-v1.schema.json` verlangt `formatVersion` und eine Liste von Events; fehlende oder unbekannte Version, ungültiges JSON, mehr als 2 MB oder keine Einträge → ganze Datei mit klarer deutscher Meldung abgelehnt. Öffentlich unter `GET /v1/import-v1.schema.json`.
- **Keine Kennungen in der Datei.** Jedes Event bringt seinen Ort als `location` mit (`name` Pflicht; `address`, `latitude`, `longitude`, `precision`, `note` optional). Ein vorhandener Name (über `NormalizeKey`) ordnet dem vorhandenen Ort zu, der unverändert bleibt; abweichende Adresse/Koordinaten nur als Hinweis. Ein neuer Ort braucht vollständige Adresse (Straße, PLZ fünfstellig, Ort), Koordinaten und Ortsgenauigkeit, sonst ist der Eintrag `error`.
- **Fehler je Eintrag** mit Position, Titel und Grund; sie blockieren die übrigen nicht. Längengrenzen (ENT-24, nach NFC in Codepoints): `title`, `location.name`, `street`, `importKey` ≤ 200; `city` ≤ 100; `source.description` und Programmpunkt-Beschreibung ≤ 500; `note` (Event, Ort) und `source.url` ≤ 2000; ≤ 100 Programmpunkte. Der Grund nennt Feld und Grenze. Dieselben Grenzen gelten im Admin-Formular (`ErrValidation`, deutsche Meldung am Feld, Eingaben bleiben erhalten); `maxEventFormBytes` steigt auf 256 KiB.
- **Klassen** gegen den gesamten Bestand inkl. Archiv: `new`, `update` (Vorschau zeigt geänderte Felder alt/neu), `unchanged` (Vergleich der kanonischen Formen), `duplicateSuspect` (gleicher `title_key`, Beginn-Datum, Ort; gleicher Titel an anderem Datum ist `new`), `error`, pro Ort `newLocation`. Vorrang `error` vor `update`/`unchanged` vor `duplicateSuspect`. Ein Duplikat-Treffer zusätzlich zu einem `importKey`-Treffer ist nur Hinweis.
- **Konflikte innerhalb der Datei:** doppelter `importKey` oder gemeinsames Ziel-Event → beide `error`; gleiche Einträge untereinander → `duplicateSuspect`; ein mehrfach mitgebrachter neuer Ort wird nur einmal geführt und angelegt. Ziel-Event mit anderem `importKey` → `error` schon beim Upload.
- **Entscheidungen:** Verdacht gegen vorhandenes Event: überspringen, als neu anlegen, überschreiben. Verdacht nur gegen einen anderen Eintrag: überspringen oder als neu anlegen. Ohne Entscheidung wird nichts übernommen.
- **Übernahme:** Formular sendet ursprünglichen Dateiinhalt (verstecktes Feld) und je Position Entscheidung, Upload-Klasse und ggf. Ziel-ID. Erneute Klassifizierung; abweichende Klasse/Ziel-ID oder inzwischen existierender „neuer“ Ort → `stale`. `error`, `stale`, `unchanged` und unentschiedene Verdachtsfälle werden vor der Transaktion aussortiert. Überschreiben/`update` ersetzt das Ziel vollständig samt Ablaufplan; Eintrag ohne `importKey` lässt den Schlüssel des Ziels stehen. Wieder aktive Events verlieren `archived_at`. Unerwarteter DB-Fehler → nichts übernommen, klare Meldung.
- **Zusammenfassung:** neu, aktualisiert, unverändert, übersprungen, ohne Entscheidung, fehlerhaft, veraltet, dazu neu angelegte Orte; Summe = Zahl der Einträge.
- **Vorschau-UI:** Anzahl je Klasse, jeder Eintrag mit Klasse und Grund oder verlinktem Ziel-Event, neue Orte gesondert, Hinweis auf Personendaten in Titel, Notizen, Quellen (NFR-4). Kein Zwischenstand auf dem Server.
- **Testsammlung (3.4/3.5):** Quelldatei unverändert nach `testdata/zirndorf_events.json`, Ergebnis `testdata/zirndorf_events.v1.json`, gültig gegen das Schema. Stabiler `importKey`, Typ, aus `name` herausgelöste Quelle, `00:00` → leere Uhrzeit, Orte zusammengeführt mit Genauigkeit, Adresse aufgeteilt ohne „, Germany“, Klammer-Erläuterungen in die Ortsnotiz. Unsichere Werte: der vorsichtigere (SM-C1), Fälle zur Freigabe durch Andreas gelistet. Die Quelldatei hat 41 Events, PRD/SM-2 nennen 42; 3.4 klärt die Zahl.
- **Abnahme (3.5):** leerer Bestand, feste `Clock` 2026-10-01 12:00 Europe/Berlin: alle Events angelegt, vergangene sofort im Archiv; zweiter Import: 0 neue, alle `unchanged`. Beides als Integrationstest gegen PostgreSQL 18 in der CI. Import und SM-1-Prüfung in Produktion sind manuelle Abschlussschritte.

## Technical Decisions

- **Eine Quelle (AD-9):** Enum-Codes, `EventInput` und das Adress-Objekt nur in `openapi.yaml`; das Import-Schema bindet sie per `$ref` ein. `maxLength`/`maxItems` stehen in `EventInput` und den eingebundenen Schemas (in v1 zulässig, weil die Schreibform erst jetzt benutzt wird). Ein CI-Test prüft Enum-Codes, Grenzen und neu die Kern-Feldnamen `EventField*`/`FilterField*` gegen Felder von `EventInput`/`Event` bzw. Parameter; nur-Admin-Felder (z. B. `locationId`) stehen ausdrücklich als Ausnahme im Test.
- **Ein Eingabetyp:** Parser und Validierung im Kern über `core.EventInput`, gleiche Regeln wie `SaveEvent`, NFC an einer Stelle, `Canonicalize` vor Speichern und für den `unchanged`-Vergleich. `core.EventInput` trägt entweder die interne Ortskennung (Admin) oder den mitgebrachten Ort (Import).
- **Zustandsloser Import (AD-10):** Upload klassifiziert, Viewer hält den Stand nur im Browser-Formular, Speichern klassifiziert neu. Klassifizierung als reine Kernlogik, Unit-Tests ohne DB mit fester `Clock`, inkl. aller Datei-Konflikte und Vorrang.
- **Datenmodell:** neue Spalte `events.import_key` (optional, eindeutig, wenn gesetzt) per neuer goose-Migration, nur vorwärts. `importKey` erscheint nie in der Leseform; `SaveEvent` aus dem Admin ändert ihn nie, nur `CommitImport` setzt ihn.
- **Schreiben (AD-6):** `CommitImport` ruft nur die transaktionsgebundenen Kerne (`SaveEvent`, `SaveLocation`) in **einer** Transaktion über `TxRunner`, immer mit `allowDuplicates`. Ortsverwaltung und „Neuer Ort“ im Event-Formular nutzen dieselbe `SaveLocation`-Hülle, Verhalten unverändert. Nach Commit werden betroffene IDs aus der Menge „prüfen“ entfernt.
- **Advisory-Sperre (neu in 3.3):** `TxRunner` nimmt zu Beginn jeder Transaktion `pg_advisory_xact_lock` mit fester Kennung; Schreib-Transaktionen laufen nacheinander, Duplikat- und Namensprüfung sehen den Stand der vorigen. Doppelklick oder Import neben Formular-Speichern → zweiter wartet und klassifiziert neu (`stale`/`unchanged`, kein Duplikat). `RecomputeDerived` (Events und Orte) läuft ab jetzt in einer Transaktion unter derselben Sperre und liest erst nach Erhalt; die Ausnahme „ohne `TxRunner`“ entfällt, ebenso der Kommentar zur nicht atomaren Namensprüfung in `location_service.go`. Postgres-Tests belegen das Warten. Restrisiko (alte Instanz schreibt danach mit alten Regeln) ist hingenommen.
- **Admin:** Import-Viewer in `internal/adapter/admin` (`html/template`, htmx, deutsche Texte, Session, CSRF-Schutz). Nur der Adapter übersetzt Kern-Fehler. Kein CORS unter `/admin`.
- **Tests:** test-first; 100 % Abdeckung für `internal/core`, `internal/adapter/publicapi/v1`, `internal/adapter/admin`.

## Cross-Story Dependencies

- **Aus Epic 1/2:** `SaveEvent` mit Duplikat-Policy, `FindDuplicateCandidates`, `NormalizeKey`/`title_key`/`name_key`, `Canonicalize`, `SaveLocation`-tx-Kern, Ablaufplan als Wertobjekt, Menge „prüfen“ aus `RecomputeDerived`, statischer Pfad `/v1/import-v1.schema.json`, öffentliche Listen für die Abnahme.
- 3.1 → 3.2 → 3.3 aufeinander aufbauend (Format/Validierung, Klassifizierung + `import_key`, Übernahme + Sperre). 3.4 braucht das Schema aus 3.1; 3.5 braucht 3.3 und 3.4.
- Offen aus früheren Retros, berührt Epic 3: Browser-Tests für das Admin-JavaScript (Deferred, Ziel Epic 3); B1-Fix-PR (Kleinigkeiten der Public API) ist noch offen, aber unabhängig.
