# Epic 3 Context: Andreas importiert Recherchen per JSON-Datei

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

Andreas lädt eine dokumentierte, versionierte JSON-Import-Datei im Admin hoch, sieht eine Vorschau mit Klassifizierung je Eintrag, entscheidet jeden Duplikatverdacht und übernimmt alles in einer Transaktion mit Zusammenfassung. So landen Recherchen vollständig, ehrlich (keine geratenen Genauigkeiten) und ohne stille Duplikate im Bestand. Abschluss: Die Testsammlung im Format v1 (39 Events) ist vollständig und wiederholbar importierbar (zweiter Import erzeugt 0 neue Events), und der Weihnachtsmarkt ist in Produktion über eine Zeitraumabfrage für die Adventszeit abrufbar.

## Stories

- Story 3.1: Import-Format v1 und Prüfung der Datei
- Story 3.2: Import-Vorschau mit Klassifizierung
- Story 3.3: Duplikate entscheiden und Import übernehmen
- Story 3.4: Testsammlung ins Format v1 überführen
- Story 3.5: Testsammlung importieren und Abnahme

## Requirements & Constraints

- **Format:** Pflichtfeld `formatVersion`; fehlende oder unbekannte Version, ungültiges JSON, Datei über 2 MB oder ohne Einträge: ganze Datei mit klarer deutscher Meldung ablehnen. Das Format bildet alle Event- und Ortsangaben ab, inklusive optionalem `importKey`, enthält aber keine internen Kennungen.
- **Mitgebrachter Ort:** Jedes Event bringt `location` mit, mindestens `name`. Ein vorhandener Name (über `NormalizeKey`) ordnet dem vorhandenen Ort zu, der unverändert bleibt; Abweichungen bei Adresse/Koordinaten nur als Hinweis. Ein neuer Ort braucht vollständige Adresse (Straße, fünfstellige PLZ, Ort), Koordinaten und Ortsgenauigkeit, sonst ist der Eintrag fehlerhaft.
- **Validierung:** Dieselben Regeln wie beim Speichern im Admin, über denselben Eingabetyp. Fehlerhafte Einträge werden einzeln mit Position, Titel und Grund gemeldet und blockieren die übrigen nicht. Textgrenzen in Zeichen nach NFC: `title`, `location.name`, `street` 200; `city` 100; `source.description` und Programmpunkt-Beschreibung 500; `note` und `source.url` 2000; `importKey` 200; höchstens 100 Programmpunkte.
- **Klassen:** `new`, `update`, `unchanged`, `duplicateSuspect`, `error`, dazu pro Ort `newLocation`. Klassifiziert wird gegen den gesamten Bestand inklusive Archiv. Vorrang: `error` vor `update`/`unchanged` vor `duplicateSuspect`. Bekannter `importKey` heißt `update` (Vorschau zeigt geänderte Felder alt/neu) oder `unchanged`. Duplikatverdacht: gleicher Titelschlüssel, gleiches Beginn-Datum (nicht Uhrzeit), gleicher Ort; gleicher Titel an anderem Datum ist `new`.
- **Konflikte in der Datei:** Doppelter `importKey` oder gemeinsames Ziel-Event macht beide Einträge zu `error`; untereinander gleiche Einträge werden `duplicateSuspect`; ein mehrfach mitgebrachter neuer Ort wird nur einmal angelegt. Ein Ziel-Event mit anderem `importKey` als der Eintrag ist `error`.
- **Entscheidungen:** Verdacht gegen den Bestand: überspringen, als neu anlegen oder vorhandenes überschreiben. Verdacht nur innerhalb der Datei: nur überspringen oder als neu anlegen. Ohne Entscheidung wird der Eintrag nicht übernommen.
- **Zusammenfassung:** neu, aktualisiert, unverändert, übersprungen, ohne Entscheidung, fehlerhaft, veraltet (`stale`) sowie Zahl neuer Orte; die Summe entspricht der Zahl der Einträge.
- **Datenschutz:** Die Vorschau erinnert daran, Titel, Notizen und Quellen auf Personendaten zu prüfen.
- **Ehrlichkeit:** Bei der Überführung der Testsammlung wird bei Unsicherheit (Genauigkeit, Typ) immer der vorsichtigere Wert gewählt; `00:00` als Platzhalter wird zur leeren Uhrzeit.

## Technical Decisions

- **Spec-first, eine Quelle:** Feldnamen, Enum-Codes und `EventInput` (inklusive `maxLength`/`maxItems`) stehen nur in `api/v1/openapi.yaml`. `api/v1/import-v1.schema.json` bindet `EventInput`, Enums und `address` per `$ref` ein, kopiert nichts. Kern-Konstanten spiegeln Codes und Grenzen; ein CI-Test prüft die Übereinstimmung, auch die Feldnamen in Fehlern/Filtern (admin-only Felder wie `locationId` als ausdrückliche Ausnahme). Das Schema wird statisch unter `GET /v1/import-v1.schema.json` mit offenem CORS ausgeliefert.
- **Datenform:** Der mitgebrachte Ort heißt `location` (kein `locationId`, kein `importLocation`). `source` ist ein Objekt `{description, url|null}`. `Canonicalize` (getrimmt, `""` als `null`, Ablaufplan sortiert) läuft vor dem Speichern und beim Vergleich für `unchanged`.
- **Zustandsloser Import:** Kein Zwischenstand auf dem Server; er liegt nur im Browser-Formular. Beim Speichern werden Originaldatei (verstecktes Feld) und je Position Entscheidung, ursprüngliche Klasse und Ziel-ID gesendet. `CommitImport` klassifiziert neu; Abweichung von Klasse oder Ziel oder ein inzwischen existierender „neuer“ Ort ergibt `stale`.
- **Schreiben:** Nur über Kern-Anwendungsfälle. `CommitImport` sortiert `error`, `stale`, `unchanged` und unentschiedene Verdachtsfälle vor der Transaktion aus und ruft dann nur die transaktionsgebundenen Kerne (`SaveLocation`, `SaveEvent`) in einer Transaktion über `TxRunner` auf, immer mit `allowDuplicates`. `update`/„überschreiben“ ersetzen das Ziel-Event vollständig samt Ablaufplan. Beim Überschreiben erhält das Ziel den `importKey` des Eintrags; ohne Schlüssel im Eintrag bleibt der des Ziels. Nur `CommitImport` setzt `importKey`, `SaveEvent` aus dem Admin nie. Wieder aktive Events verlieren `archived_at`. Unerwarteter DB-Fehler: nichts übernommen, klare Meldung.
- **Serialisierung:** `TxRunner` nimmt eine transaktionsgebundene Advisory-Sperre; alle Schreib-Hüllen laufen darüber, ebenso `RecomputeDerived` beim Start (liest erst nach Erhalt der Sperre). Parallele Importe/Speichervorgänge klassifizieren nach dem Warten neu und erzeugen kein Duplikat.
- **Datenmodell:** Spalte `import_key` in `events`, optional, eindeutig wenn gesetzt. Duplikatprüfung über gespeicherte `title_key`, `startDate`, `locationId` (`FindDuplicateCandidates`); Schlüssel aus `NormalizeKey` (trim, Leerraum zusammenfassen, lowercase) nach NFC.
- **Upload-Grenzen:** Import-Datei höchstens 2 MB; Event-Formulargrenze auf 256 KiB angehoben, damit ein Event mit allen Feldern an der Grenze passt.
- **Tests:** Parser, Validierung und Klassifizierung (alle dateiinternen Konflikte, Vorrang) als Unit-Tests ohne Datenbank mit fester `Clock`. Nebenläufigkeit (zweite Transaktion wartet und sieht das Ergebnis der ersten) und beide Importläufe der Testsammlung als Integrationstests gegen PostgreSQL 18 in der CI.

## UX & Interaction Patterns

- Admin-Oberfläche deutsch (`html/template`, htmx), Meldungen deutsch, alles Technische englisch.
- Vorschau: Anzahl je Klasse, jeder Eintrag mit Klasse, Grund oder verlinktem Ziel-Event, neue Orte gesondert gelistet, Hinweis auf Personendaten; bei `update` geänderte Felder alt/neu; Zusatzhinweis, wenn ein `update` auch nach Titel/Datum/Ort zu einem anderen Event passt.
- Je Verdachtsfall eine Auswahl der erlaubten Entscheidungen; danach ein Schritt „Import übernehmen“ und eine Ergebnisseite mit Zusammenfassung.
- Formulare (Event, Ort) zeigen Grenzüberschreitungen als deutsche Meldung am Feld, ohne Eingaben zu verlieren.

## Cross-Story Dependencies

- 3.1 liefert Format, Schema und Validierung; 3.2 baut die Klassifizierung darauf; 3.3 nutzt sie erneut für `stale`-Erkennung und Übernahme. 3.1 bis 3.4 sind erledigt, 3.5 ist offen.
- 3.3 vereinheitlicht alle Schreib-Hüllen auf `TxRunner` und berührt dadurch Ortsverwaltung und „Neuer Ort“ im Event-Formular (Epic 1) sowie `RecomputeDerived`/`MarkArchived` (Epic 2); deren Verhalten bleibt unverändert.
- 3.5 nutzt `testdata/zirndorf_events.v1.json` (39 Events, aus 3.4; Quelle `testdata/zirndorf_events.json` mit 41 Events, Herbstmarkt zusammengelegt) mit fester `Clock` 2026-10-01 12:00 Europe/Berlin: erster Import legt alle Events an, vergangene sofort im Archiv; zweiter Import ergibt nur `unchanged`, 0 neue Events.
- Der Weihnachtsmarkt ist nicht in der Datei, sondern in Produktion im Admin angelegt. Die Testsammlung ist in Produktion bereits importiert; die SM-1-Prüfung dort ist ein manueller Schritt per `GET /v1/events` mit Zeitraum über die Adventszeit, ohne erneuten Import.
- Archiv-Zugriff, Vorbei-Regel und Ausblenden von „prüfen“-Events stammen aus Epic 2 und gelten für importierte Events unverändert.
