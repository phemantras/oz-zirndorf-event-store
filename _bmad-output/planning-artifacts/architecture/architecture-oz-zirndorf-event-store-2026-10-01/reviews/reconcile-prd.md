---
title: "Abgleich PRD ↔ Architecture Spine"
created: 2026-10-01
inputs:
  - prds/prd-oz-zirndorf-event-store-2026-10-01/prd.md
  - prds/prd-oz-zirndorf-event-store-2026-10-01/addendum.md
  - architecture/architecture-oz-zirndorf-event-store-2026-10-01/ARCHITECTURE-SPINE.md
  - architecture/architecture-oz-zirndorf-event-store-2026-10-01/.memlog.md
---

# Abgleich PRD ↔ Architecture Spine

Maßstab: Gemeldet wird nur, wo der Spine dem PRD widerspricht, eine Anforderung falsch überträgt oder wo zwei unabhängig gebaute Teile (Public API, Admin, Import, Postgres, Cleanup, OpenAPI-Spec, Import-Schema) auseinanderlaufen könnten. FRs, die ein einzelner Baustein im Kern ohne Abstimmung erfüllt, brauchen kein AD.

Schwere: **H** = Widerspruch oder wahrscheinlicher Drift zwischen Bausteinen bzw. gefährdetes Erfolgskriterium · **M** = Lücke mit realem Drift-Risiko · **N** = Hinweis, kleine Präzisierung.

## Übersicht

| # | Schwere | Thema | PRD | Spine |
| --- | --- | --- | --- | --- |
| 1 | H | Zeitfelder: Ein- und Ausgabemodell mehrdeutig | FR-1, FR-2, FR-4, FR-10, KON-4, KON-7, FR-16 | AD-3, AD-8, AD-9, Conventions |
| 2 | H | Import: eine Transaktion vs. „Fehler blockieren Rest nicht“, Konflikte innerhalb der Datei | FR-16, FR-17, FR-18, SM-2 | AD-10, AD-11 |
| 3 | H | Import-Abgleich muss archivierte Events einschließen | FR-17, FR-18, SM-2, Addendum §2 | AD-10, AD-11, AD-7 |
| 4 | M | „Als archiviert erkennbar“ ohne festgelegtes Feld; `archivedAt` nach Reaktivierung veraltet | FR-11, FR-15, FR-13 | AD-5, AD-13 |
| 5 | M | KON-2 (Parallelbetrieb, Abschaltdatum) ohne Regel; einzige Spec-Datei | KON-2, NFR-2 | AD-8 |
| 6 | M | Mitgebrachter Ort mit bekanntem Namen: Feldabweichungen ungeregelt; `newLocation` als Event-Klasse | FR-16, FR-17, FR-6 | AD-10, AD-11 |
| 7 | M | Ablaufplan-Zeiten ohne Zeitmodell | FR-4, KON-4 | AD-3 (nur Event) |
| 8 | M | Antwortform „vollständiger Ort“ nicht festgelegt | FR-10 | AD-7, AD-8, Conventions |
| 9 | N | Öffentliche Doku: Import-Format, Lesbarkeit | NFR-3, FR-16, SM-4 | AD-8, AD-9 |
| 10 | N | Import-Zusammenfassung fehlt im Ablauf | FR-18 | AD-10 |
| 11 | N | Duplikat-Entscheidung „überschreiben“/„neu“ und `importKey` | FR-18, SM-2 | AD-11 |
| 12 | N | `to` als Datum inklusive, Vorbei-Grenze 23:59:59 | KON-5, Addendum §1 | AD-4 |
| 13 | N | NFR-4 nur als Datenmodell-Verweis | NFR-4 | Capability Map |
| 14 | N | Lizenz als Start-Blocker fehlt in Deferred | §7, Offene Frage 1 | Deferred |
| 15 | N | OSM-Kachelnutzung im Kartenpicker | FR-15, Addendum §1 | Stack |

## Befunde im Detail

### 1 (H) Zeitfelder: Ein- und Ausgabemodell mehrdeutig

- **PRD:** KON-4: Zeitpunkte als ISO 8601 mit Offset, bei nur bekanntem Datum nur `JJJJ-MM-TT`. KON-7: Feldnamen „wie im Entwurf `zirndorf_events.json` (z. B. `startTime`)“. Im Entwurf ist `startTime` ein vollständiger Zeitpunkt. FR-2: Zeitgenauigkeit getrennt für Beginn und Ende. FR-10: Zeitgenauigkeit steht in jeder Antwort.
- **Spine:** AD-3 speichert `startDate`, `startTime` (nur Uhrzeit), `endDate`, `endTime`, `allDay`. Die Conventions nennen `startDate`, `startTime`, `effectiveEnd` als JSON-Feldnamen. Gleichzeitig sagen sie unter „Zeiten (API)“: ein Zeitpunkt mit Offset oder nur `YYYY-MM-DD`, also **ein** kombiniertes Feld. AD-9 verlangt, dass das Import-Schema dieselben Feldnamen wie die OpenAPI-Spec nutzt.
- **Drift-Risiko:**
  - OpenAPI-Autor und Import-Schema-Autor können unterschiedlich entscheiden: getrennte `startDate`/`startTime` oder ein kombiniertes `start`. Hat `startTime` dieselbe Bedeutung wie im Entwurf (Zeitpunkt) oder eine andere (Uhrzeit)? Gleicher Name mit anderer Bedeutung widerspricht dem Geist von KON-7.
  - Die API **gibt** abgeleitete Genauigkeiten aus (`exact`/`dateOnly`/`allDay`). Der Import **nimmt** Speicherfelder entgegen (`allDay`-Bool). „Identische Feldnamen“ (AD-9) ist deshalb nicht erfüllbar, ohne festzulegen, welche Felder nur in einer Richtung existieren.
  - Ein einziges `allDay` gilt für das ganze Event, das PRD verlangt aber Genauigkeit „je für Beginn und Ende“. Unklar: Ist `allDay=true` mit gesetzter `startTime` ungültig? Ist ein leeres `endTime` bei `allDay=false` immer `dateOnly`? Kombinationen wie „Beginn nur Datum, Ende ganztägig“ sind nicht darstellbar. Das ist vertretbar, muss aber als Regel stehen.
  - Ob `effectiveStart`/`effectiveEnd` in der öffentlichen API erscheinen, ist offen. Die Conventions nennen `effectiveEnd` als JSON-Feld, und das wäre eine Vertragsentscheidung (NFR-2).
- **Empfehlung:** AD-3 ergänzen, und zwar um (a) das API-Ausgabeformat je Ende, z. B. `start`/`end` als ISO-Wert plus `startPrecision`/`endPrecision`, (b) das Import-Eingabeformat und (c) die Validierungsregeln für `allDay` mit Uhrzeit. AD-9 präzisieren: „identische Namen und Codes **für gleichbedeutende Felder**“. Klären, ob `effective*` öffentlich ist.

### 2 (H) Import: eine Transaktion vs. „Fehler blockieren Rest nicht“, Konflikte innerhalb der Datei

- **PRD:** FR-17: Fehlerhafte Einträge werden einzeln gemeldet und blockieren die übrigen nicht. FR-16: Ein mitgebrachter neuer Ort wird angelegt. SM-2: Die 42 Events lassen sich vollständig importieren.
- **Spine:** AD-10 klassifiziert beim Speichern erneut und schreibt **alle** übernommenen Einträge in **einer** Transaktion. AD-11: Ortsnamen und `importKey` sind eindeutig.
- **Lücke:** Die Klassifizierung erfolgt nur „gegen den aktuellen Bestand“. Konflikte **innerhalb der Datei** sind nicht geregelt:
  - Mehrere Events bringen denselben neuen Ort mit. Das ist beim Material mit rund 20× Paul-Metz-Halle (Addendum §2) wahrscheinlich. Jeder Eintrag wird als `newLocation` klassifiziert, und das zweite Anlegen verletzt die Eindeutigkeit des Namens.
  - Derselbe `importKey` steht zweimal in der Datei.
  - Zwei Einträge der Datei erfüllen untereinander die Duplikatbedingung. UJ-1 nennt genau diesen Fall: Nachmittags- und Abendvorstellung mit gleichem Titel am selben Tag und Ort.
  - Wenn der Bestand sich zwischen Vorschau und Speichern ändert, wird ein Eintrag beim erneuten Klassifizieren zum Fehler. Fällt dann die ganze Transaktion oder nur der Eintrag?
  Fällt eine Datenbank-Constraint-Verletzung in der Transaktion an, rollt sie **alles** zurück. Das widerspricht FR-17.
- **Empfehlung:** In AD-10 festlegen: (a) Die Klassifizierung behandelt die Datei als Ganzes, ein neuer Ort wird pro normalisiertem Namen einmal angelegt, und ein doppelter `importKey` in der Datei ist ein Fehler für die betroffenen Einträge. (b) Ein beim erneuten Klassifizieren entdeckter Fehler schließt nur den Eintrag aus und landet in der Zusammenfassung. Die Transaktion umfasst nur fehlerfreie Einträge. (c) Duplikate innerhalb der Datei sind entweder ein Duplikatverdacht mit Admin-Entscheidung oder ausdrücklich keiner. Eine dieser beiden Regeln festlegen.

### 3 (H) Import-Abgleich muss archivierte Events einschließen

- **PRD:** Die Testsammlung enthält bereits vergangene Events (Addendum §2). Ein zweiter Import erzeugt 0 neue Events (FR-18, SM-2). Der Duplikatverdacht bezieht sich auf „ein vorhandenes Event“.
- **Spine:** AD-7 bietet nur `ListActiveEvents` und `ListArchivedEvents`. AD-10 und AD-11 sagen nicht, dass die Suche nach `importKey` und nach Duplikaten **über den ganzen Bestand** läuft, also unabhängig von der Vorbei-Regel.
- **Risiko:** Baut jemand den Abgleich auf `ListActiveEvents` auf, werden vergangene Events beim zweiten Import neu angelegt. SM-2 schlägt dann fehl. Wird ein archiviertes Event per Import korrigiert (Datum in der Zukunft), muss es wieder aktiv werden. Das gilt laut FR-15 analog.
- **Empfehlung:** In AD-11 ergänzen: „Die Suche nach `importKey`, Duplikaten und Ortsnamen läuft über alle Events, aktive und archivierte. Dafür gibt es eigene Repository-Abfragen, keine Lese-Anwendungsfälle aus AD-7.“

### 4 (M) „Als archiviert erkennbar“ ohne festgelegtes Feld; `archivedAt` nach Reaktivierung veraltet

- **PRD:** FR-11: Ein archiviertes Event ist über seine Kennung abrufbar und **als archiviert erkennbar**. Maßgeblich ist die Vorbei-Regel. FR-15: Ein korrigiertes Archiv-Event wird wieder aktiv.
- **Spine:** AD-5 sagt: Keine Lese-Abfrage wertet `archivedAt` aus. Ein Ausgabefeld für „archiviert“ ist nicht definiert. AD-13 setzt `archivedAt` nur dort, wo es `NULL` ist. Niemand setzt es zurück.
- **Risiko:** Der Entwickler der Public API gibt naheliegend `archivedAt` aus, das ja als Spalte existiert. Das widerspricht FR-11 (vom Job abhängig), FR-13 (Antworten vor und nach der Bereinigung identisch) und AD-5. Ein reaktiviertes Event behält einen veralteten `archivedAt`-Wert, und Statistik und Anzeige im Admin werden falsch.
- **Empfehlung:** In AD-7 festlegen: Der Kern liefert ein abgeleitetes `archived` (bool), berechnet aus `effectiveEnd <= now`, und `archivedAt` erscheint nie in der API. In AD-6 oder AD-13 festlegen: `SaveEvent` setzt `archivedAt` auf `NULL`, wenn das neue `effectiveEnd > now` ist. Funktional gilt die Reaktivierung dank AD-4/AD-5 bereits, offen ist nur die Markierung.

### 5 (M) KON-2: Parallelbetrieb, Abschaltdatum ohne Regel; einzige Spec-Datei

- **PRD:** KON-2: Die alte Hauptversion bleibt mindestens 6 Monate erreichbar, das Abschaltdatum steht in der Doku. NFR-2: Inkompatible Änderungen gibt es nur in einer neuen Version.
- **Spine:** Bindet KON-2 im Frontmatter, enthält aber keine Regel dazu. AD-8 nennt **eine** Datei `api/openapi.yaml` als „einzige Quelle“. AD-7 sagt „Ausgabeformat aus dem Kern“, die Kern-Objekte sind also die gemeinsame Basis aller Versionen.
- **Risiko:** Bei v2 ist nicht vorgezeichnet, wie zwei Specs, zwei generierte Gerüste und zwei Mapper nebeneinander leben. Ändert jemand die Event-Typen im Kern (FR-5-Hinweis), bricht v1 still, weil AD-7 die Abbildung nur „noch mappen“ lässt. Für das Abschaltdatum gibt es keinen Ort in der Spec und keinen Mechanismus wie `Deprecation`/`Sunset`-Header.
- **Empfehlung:** Ein kurzes AD oder einen Conventions-Eintrag ergänzen: eine Spec-Datei pro Hauptversion (`api/v1/openapi.yaml`), ein eigenes Adapter-Paket pro Version, das Mapping je Version vom Kern-Modell entkoppelt, und das Abschaltdatum in `info` bzw. als `Sunset`-Header. Enum-Änderungen im Kern brauchen eine Abbildungstabelle pro Version.

### 6 (M) Mitgebrachter Ort mit bekanntem Namen; `newLocation` als Event-Klasse

- **PRD:** FR-16: Ein mitgebrachter Ort mit vorhandenem Namen wird dem vorhandenen **zugeordnet**. Ein neuer Ort braucht Adresse **und** Koordinaten, sonst ist der Eintrag fehlerhaft. Die Vorschau zeigt neue Orte gesondert. FR-6: Ortsgenauigkeit ist Pflicht.
- **Spine:** AD-10 listet `newLocation` neben `new`/`update`/`duplicateSuspect`/`error` als Klasse. Ein Event kann aber gleichzeitig `new` sein und einen neuen Ort mitbringen, das sind zwei Dimensionen. Offen ist auch, was passiert, wenn der mitgebrachte Ort mit bekanntem Namen andere Adresse oder Koordinaten hat: ignorieren, warnen oder überschreiben?
- **Risiko:** Viewer und Kern-Klassifizierung bilden die Zustände verschieden ab. Ein stilles Überschreiben von Ortskoordinaten per Import würde nach FR-7 alle Events des Orts verschieben.
- **Empfehlung:** Die Event-Klasse und eine Orts-Annotation (`existing`/`new`/`error`) trennen. Festlegen, dass mitgebrachte Felder eines bekannten Orts ignoriert werden. Optional zeigt der Viewer eine Abweichung als Hinweis an. Ein neuer Ort ohne Adresse, Koordinaten **oder Ortsgenauigkeit** macht den Eintrag zum Fehler. Die Ortsgenauigkeit ist laut FR-6 Pflicht und in FR-16 nicht genannt, gilt aber über FR-15/FR-6.

### 7 (M) Ablaufplan-Zeiten ohne Zeitmodell

- **PRD:** FR-4: Programmpunkte mit Beginn und optionalem Ende, chronologisch sortiert. KON-4 gilt für alle Zeitpunkte.
- **Spine:** AD-3 regelt nur Event-Zeiten. Das Schema von `timetable_entries` hat keine Zeitfelder. Im Entwurf haben Ablaufpunkte vollständige `startTime`/`endTime`.
- **Risiko:** Import und Admin speichern Programmpunkte unterschiedlich (Zeitpunkt, nur Uhrzeit oder Datum + Uhrzeit). Programmpunkte mehrtägiger Events (Kirchweih) brauchen ein Datum. Unbekannte Uhrzeiten wären wieder `00:00`.
- **Empfehlung:** AD-3 auf Programmpunkte ausdehnen: `date` + optionale `time`, gleiche Ausgaberegel nach KON-4. Der Kern sortiert.

### 8 (M) Antwortform „vollständiger Ort“ nicht festgelegt

- **PRD:** FR-10: Jedes Event enthält den **vollständigen** Ort (Kennung, Name, Adresse, Koordinaten, Ortsgenauigkeit, Notiz).
- **Spine:** Die Conventions nennen `locationId` als Beispiel-Feldnamen, eine Einbettung wird nicht erwähnt. Die Capability Map verweist nur pauschal auf AD-8.
- **Risiko:** Die Spec wird mit `locationId` plus separatem `/v1/locations` gebaut, und die Karte müsste nachschlagen. Das verletzt FR-10 und SM-1/SM-3.
- **Empfehlung:** In den Conventions festlegen: Event-Antworten betten `location { id, name, address, latitude, longitude, precision, note }` ein. Ob die Koordinaten als `latitude`/`longitude` wie im Entwurf geführt werden, ebenfalls festlegen.

### 9 (N) Öffentliche Doku: Import-Format, Lesbarkeit

- NFR-3 (Doku öffentlich abrufbar) ist mit `/v1/openapi.yaml` formal erfüllt. FR-16 verlangt ein **dokumentiertes** Import-Format. AD-9 legt `api/import-v1.schema.json` an, sagt aber nicht, dass es öffentlich ausgeliefert wird. Es ist nur im Repo, und das Repo ist bis zur Lizenzklärung (§7) womöglich nicht öffentlich.
- Für SM-4 (Mitglied übernimmt ohne Rückfrage) ist eine lesbare Darstellung hilfreich, etwa eine statische Redoc/Swagger-UI-Seite unter `/v1/docs`. Die Feldbedeutungen der Genauigkeiten und die §6-Konventionen müssen als `description` in der Spec stehen. Das ist bisher nicht als Regel formuliert.
- **Empfehlung:** In AD-8 ergänzen: Die Spec trägt die §6-Konventionen und Feldbedeutungen als Beschreibungen. Optional eine HTML-Doku-Seite. AD-9: Das Schema wird ebenfalls ausgeliefert, z. B. unter `/v1/import-v1.schema.json` oder unter `/admin`, falls es bewusst nicht öffentlich sein soll.

### 10 (N) Import-Zusammenfassung fehlt im Ablauf

- FR-18: Nach dem Import gibt es eine Zusammenfassung mit den Zahlen neu, aktualisiert, übersprungen und fehlerhaft. AD-10 endet mit dem Schreiben. `CommitImport` sollte ein Ergebnis mit diesen Zahlen liefern. Das gilt besonders, weil Einträge beim erneuten Klassifizieren ihre Klasse wechseln können (siehe #2) und die Vorschau-Zahlen dann nicht mehr stimmen.

### 11 (N) Duplikat-Entscheidung und `importKey`

- Nicht geregelt ist, ob bei „überschreiben“ der `importKey` des importierten Eintrags auf das vorhandene Event übertragen wird. Wenn nicht, erscheint derselbe Eintrag bei jedem weiteren Import erneut als Duplikatverdacht. SM-2 ist zwar formal erfüllt (0 neue Events), aber unnötig mühsam. Bei „überspringen“ bleibt der Verdacht bestehen. Das ist gewollt, sollte aber bewusst entschieden werden.
- Offen ist auch, ob „überschreiben“ den Ablaufplan ersetzt und ob der Admin den `importKey` im Formular sieht oder ändern kann.
- Der Titelvergleich für den Duplikatverdacht hat keine Normalisierung. Für Ortsnamen gibt es Trim und Groß-/Kleinschreibung, für Titel nichts. Die Regel liegt nur im Kern, das Drift-Risiko ist also gering, sie sollte aber einheitlich sein.

### 12 (N) `to` als Datum inklusive, Vorbei-Grenze

- KON-5: `to` als **Datum** ist inklusive, gemeint ist also das Ende dieses Tages. AD-4 sagt nur „filtern über `effectiveStart`/`effectiveEnd`“. Die Umrechnung eines Datums-Parameters in eine Grenze (Ende des Tages Europe/Berlin) sollte im Kern festgelegt sein, nicht im Handler.
- Das Addendum nennt 23:59:59 als Ende des Tages, AD-4 „Ende des End-Tages“. Kombiniert mit `effectiveEnd > now` sind beide Varianten möglich. Entweder 23:59:59 oder der Beginn des Folgetags, festgelegt im Kern. Eine Variante wählen und in Tests absichern.

### 13 (N) NFR-4 nur als Datenmodell-Verweis

- Die Capability Map verweist für NFR-4 auf „core (Datenmodell), AD-6“, eine Regel fehlt. Technisch durchsetzbar ist: keine Felder für Kontaktperson, Telefon oder E-Mail an Event und Ort, weder in der API noch im Import-Schema. Freitext (Notiz, Quelle, Ablaufplan) bleibt Verantwortung der Redaktion. Ein Satz in den Conventions genügt, damit niemand ein `contact`-Feld aus dem Entwurf übernimmt. Das Logging-Verbot für Formularinhalte ist bereits vorhanden.

### 14 (N) Lizenz als Start-Blocker fehlt in Deferred

- §7 und Offene Frage 1: Die Lizenz blockiert den öffentlichen Start. Unter Deferred stehen Backup und Domain „vor dem öffentlichen Start“, die Lizenz fehlt. Architektonisch relevant ist das wenig, KON-8 sieht einen Lizenzhinweis in der Listenhülle aber bereits vor. Die Lizenz gehört als Start-Gate in die Deferred-Liste.

### 15 (N) OSM-Kachelnutzung im Kartenpicker

- Der Kartenpicker (FR-15) ist mit Leaflet und OSM-Kacheln abgedeckt. Die Nutzungsrichtlinie von `tile.openstreetmap.org` verlangt Attribution und eine geringe Last. Bei einem Admin ist das unkritisch, die Attribution muss aber in die Story-Akzeptanz.

## Ausreichend abgedeckt (geprüft, kein Befund)

- **FR-3 Quelle Pflicht:** über AD-6 (eine Validierung für Admin und Import) abgedeckt. Die Struktur (Beschreibung + optionaler Link) ist ein Spec-Detail.
- **FR-7 Ort-Löschschutz inkl. archivierter Events:** Dank AD-5 (eine Tabelle) zählen archivierte Events automatisch mit. AD-6 legt den Schutz in den Kern. Die Fremdschlüssel-Restriktion in Postgres ist als Absicherung zulässig, sie ist eine Integritätsregel und kein Verstoß gegen AD-2.
- **FR-15 korrigiertes Archiv-Event wird wieder aktiv:** funktional erfüllt, weil `effectiveEnd` bei jedem Schreiben neu berechnet wird (AD-4) und keine Abfrage `archivedAt` nutzt (AD-5). Siehe nur #4 zur Markierung.
- **FR-15 Kartenpicker:** im Stack (Leaflet + OSM) und in der Capability Map.
- **FR-16 neue Orte brauchen Adresse + Koordinaten, Vorschau zeigt neue Orte gesondert:** im Grundsatz über AD-6 und `newLocation` abgedeckt, Präzisierung in #6.
- **FR-8, FR-12, FR-13 (Vorbei-Regel, keine Lücke, Job ändert nichts):** sauber über AD-4, AD-5 und AD-13 gelöst.
- **FR-14, NFR-1:** AD-12. Die Spec unter `/v1/openapi.yaml` ist damit ebenfalls per CORS abrufbar.
- **NFR-6, KON-1, KON-3, KON-6, KON-7, KON-8:** in den Conventions abgebildet.
- **Memlog-Konflikt D** (00:00 für „nur Datum“): im Sinne des PRD aufgelöst.
