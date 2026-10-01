---
title: 'Adversarial Review — Architecture Spine OZ Zirndorf Event Store'
reviewed: ARCHITECTURE-SPINE.md (draft, 2026-10-01)
against: prd.md (final, 2026-10-01), addendum.md
date: 2026-10-01
method: 'Paar-Konstruktion: zwei Einheiten eine Ebene tiefer (Epics/Stories), die jede AD wörtlich einhalten und trotzdem inkompatibel bauen'
verdict: 'NICHT BAUREIF — 4 kritische, 7 hohe Löcher; nach Schließen der vorgeschlagenen Regeln baureif'
---

# Adversarial Review — Architecture Spine

## Urteil

**Nicht baureif.** Der Spine ist in Schichtung (AD-1, AD-2), Archiv-Logik (AD-4, AD-5, AD-13) und Oberflächentrennung (AD-12) solide. Er legt aber die **gemeinsamen Datenformen** zwischen den Epics nicht fest: das Schreib-DTO (Import vs. Admin vs. API), den Ablaufplan, die Update-Semantik, das Verhalten von `SaveEvent` bei Duplikaten und die Zeitrechnung an DST-Grenzen. An diesen Stellen können zwei Teams jede AD wörtlich befolgen und trotzdem Code bauen, der nicht zusammenpasst. Die schwersten Löcher sind F-1 (Feldform Import vs. API widerspricht AD-9 selbst), F-2 (`SaveEvent` und Duplikatentscheidung), F-3 (Ablaufplan ohne Zeitmodell und Eigentümer) und F-4 (veraltete Import-Entscheidungen beim Commit).

Schweregrade:

- **Kritisch** — Epics werden mit hoher Wahrscheinlichkeit inkompatibel gebaut, oder eine PRD-Konsequenz (FR/SM) ist dadurch verletzt.
- **Hoch** — Inkompatibilität oder stiller Datenfehler wahrscheinlich, aber lokal reparierbar.
- **Mittel** — Randfall, der zu abweichendem Verhalten zwischen Einheiten führt.
- **Niedrig** — Klarstellung, Unschärfe ohne akute Bruchgefahr.

Referenz-Einheiten (Epics) in diesem Review: **E-API** (Public API), **E-ADM** (Admin-Pflege), **E-IMP** (JSON-Import), **E-ARC** (Archiv/Bereinigung), **E-ORT** (Orte).

## Übersicht

| # | Thema | Paar | Schwere | Neue/verschärfte AD |
| --- | --- | --- | --- | --- |
| F-1 | Feldform Import ≠ API-Ausgabe, AD-9 widerspricht sich | E-IMP × E-API | Kritisch | AD-9 verschärft, AD-14 neu |
| F-2 | `SaveEvent` und Duplikatprüfung | E-ADM × E-IMP | Kritisch | AD-11 verschärft |
| F-3 | Ablaufplan ohne Zeitmodell und Eigentümer | E-ADM × E-IMP × E-API | Kritisch | AD-15 neu |
| F-4 | Veraltete Entscheidungen beim Import-Commit | E-IMP × E-ADM | Kritisch | AD-10 verschärft |
| F-5 | Update-Semantik (Ersetzen vs. Zusammenführen) | E-IMP × E-ADM | Hoch | AD-16 neu |
| F-6 | DST: nicht existierende / doppelte Ortszeiten, „Ende des Tages“ | E-ADM × E-API × E-ARC | Hoch | AD-4 verschärft |
| F-7 | Ungültige Kombinationen im Zeitmodell, Ende ohne Enddatum, Über-Mitternacht | E-ADM × E-IMP | Hoch | AD-3 verschärft |
| F-8 | `from`/`to`, „heute“ und Überschneidung: Grenzen | E-API (aktiv) × E-API (Archiv) | Hoch | AD-17 neu |
| F-9 | Zwei Uhren: Go `now` vs. SQL `now()` | E-API × E-ARC | Hoch | AD-2 verschärft |
| F-10 | Normalisierung von Ortsnamen/Titeln vs. „keine Regeln in SQL“ | E-ORT × E-IMP | Hoch | AD-11 verschärft, AD-2 klargestellt |
| F-11 | Enum-Drift OpenAPI ↔ Import-Schema ↔ Kern ↔ Admin-Labels | E-API × E-IMP × E-ADM | Hoch | AD-9 verschärft |
| F-12 | Neuberechnung von `effective*` bei Regeländerung / tzdata | E-ARC × Migrationen | Mittel | AD-4 verschärft |
| F-13 | `archivedAt` nach Reaktivierung, „archiviert erkennbar“ | E-ADM × E-ARC × E-API | Mittel | AD-5 verschärft |
| F-14 | `importKey`-Lebenszyklus (Admin-Bearbeitung, Überschreiben, Duplikate in Datei) | E-ADM × E-IMP | Mittel | AD-11 verschärft |
| F-15 | Neue Orte im Import: mehrfach in einer Datei, Konflikt mit Bestand | E-IMP × E-ORT | Mittel | AD-10/AD-11 verschärft |
| F-16 | Löschschutz-Race und DB-Integritätsregeln | E-ORT × E-IMP | Mittel | AD-6 verschärft |
| F-17 | Transaktion vs. „Fehler verhindern nicht den Rest“ | E-IMP intern | Mittel | AD-10 verschärft |
| F-18 | Lost Update bei zwei Admin-Tabs | E-ADM × E-ADM / E-IMP | Mittel | AD-16 (Version) |
| F-19 | Sortierung ohne Tiebreak, Sortierschlüssel | E-API × E-ADM | Niedrig | AD-7 verschärft |
| F-20 | Ort-Liste, Event-Typ-Liste: Inhalt und Labels | E-API × E-ADM | Niedrig | AD-7 klargestellt |
| F-21 | Löschen von Events, Kaskade auf Ablaufplan | E-ADM × E-ARC | Niedrig | AD-6 klargestellt |
| F-22 | Start-Job während Rolling Deploy / Migration | E-ARC × Betrieb | Niedrig | AD-13 klargestellt |

---

## Kritisch

### F-1 — Feldform Import ≠ API-Ausgabe; AD-9 widerspricht sich selbst

**Paar:** E-IMP × E-API.

**Konstruktion.** AD-3 speichert `startDate`, `startTime`, `endDate`, `endTime`, `allDay`. Die Konventionen sagen für die API: „Ist die Uhrzeit bekannt, Zeitpunkt mit Offset Europe/Berlin. Bei `dateOnly`/`allDay` nur `YYYY-MM-DD`“, und Genauigkeit wird abgeleitet (AD-3, AD-7).

- E-API baut regelkonform `start: "2026-12-04T18:00:00+01:00"` bzw. `start: "2026-12-04"`, dazu `startPrecision`, `endPrecision` (abgeleitet, AD-7).
- E-IMP baut regelkonform das Import-Schema mit den gespeicherten Feldern `startDate`, `startTime`, `allDay` (AD-3, Feldname-Beispiele in den Konventionen). AD-9 verlangt aber „Feldnamen identisch mit der OpenAPI-Spec“. Das ist nicht erfüllbar: Die API hat `start` + `startPrecision`, das Import-Format braucht die Eingabeform. Ein drittes Team liest AD-9 wörtlich und baut Import mit `start` + `startPrecision` als Eingabe. Dann muss der Kern aus `startPrecision: allDay` + `start: "2026-12-04"` auf `allDay=true` zurückrechnen, und `startPrecision: exact` mit `start: "2026-12-04"` ist ein Widerspruch, den niemand definiert hat.
- Zusätzlich: Die Testsammlung (`addendum.md`) nutzt `name`, `startTime` als kombinierten Zeitpunkt und `location { address, latitude, longitude }`. Ob `latitude/longitude` oder `lat/lon` oder GeoJSON `coordinates`, sagt niemand. E-ORT (Admin-Kartenpicker) und E-API wählen unabhängig.

**Folge.** Zwei inkompatible Event-Formen; SM-2 (Testsammlung importieren) und SM-3 sind gefährdet. AD-9 kann nicht wörtlich erfüllt werden.

**Vorgeschlagene Regel — AD-9 (verschärft) + AD-14 (neu): Kanonische Lese- und Schreibform**

> **AD-14 — Eine Schreibform, eine Leseform, beide in `openapi.yaml`.** `api/openapi.yaml` definiert unter `components/schemas` zwei Event-Formen: `EventInput` (Schreibform: `title`, `type`, `startDate`, `startTime?`, `endDate?`, `endTime?`, `allDay`, `source{description, url?}`, `note?`, `timetable[]`, `locationId` **oder** `location`(`LocationInput`), `importKey?`) und `Event` (Leseform: alle Felder aus `EventInput` außer `location`/`importKey`, plus `id`, `start`, `end?`, `startPrecision`, `endPrecision?`, `archived`, `location`(`Location`), ohne `effectiveStart/End`). Die Leseform enthält die lokalen Felder `startDate`/`startTime`/… **nicht**; sie enthält nur `start`/`end` im KON-4-Format. Koordinaten heißen überall `latitude`/`longitude` (WGS84, Dezimalgrad, number). Der Kern hat genau einen Eingabetyp `core.EventInput`, den Admin-Formular und Import befüllen.
>
> **AD-9 (verschärft).** `api/import-v1.schema.json` beschreibt `{ formatVersion: "1", events: EventInput[] }` und bezieht `EventInput`, `LocationInput` und alle Enums per `$ref` aus `openapi.yaml` (oder wird in CI daraus erzeugt). „Identische Feldnamen“ heißt: identisch mit `EventInput`, nicht mit `Event`.

Offen zu entscheiden: ob `effectiveStart/End` öffentlich ausgeliefert werden (Konventionen nennen `effectiveEnd` als JSON-Beispiel). Empfehlung: **nicht** ausliefern, sonst sind sie Teil des Vertrags und F-12 wird ein API-Bruch.

---

### F-2 — `SaveEvent` und die Duplikatprüfung

**Paar:** E-ADM × E-IMP.

**Konstruktion.** AD-6: Admin und Import rufen **dieselben** Anwendungsfälle auf. AD-11: „Ein Duplikatverdacht liegt vor, wenn `title`, `startDate` und `locationId` gleich sind. Die Prüfung macht der Kern.“ AD-11 nennt als Zweck: „Import und Admin erkennen dasselbe Event unterschiedlich“ verhindern.

- E-ADM liest AD-11 so: Der Kern prüft Duplikate in `SaveEvent`, damit das Admin-Formular dasselbe erkennt wie der Import. `SaveEvent` liefert `ErrConflict` bei Duplikat.
- E-IMP liest AD-10/FR-18: Die Entscheidung „als neues Event anlegen“ muss ein Duplikat bewusst anlegen. `CommitImport` ruft `SaveEvent` (AD-6) und bekommt `ErrConflict`. UJ-1 (zweite Vorstellung am selben Tag) ist unmöglich.
- Umgekehrt: E-ADM baut keine Prüfung, E-IMP prüft nur in der Klassifizierung. Dann legt der Admin stille Duplikate an — genau, was AD-11 verhindern sollte.

**Vorgeschlagene Regel — AD-11 (verschärft)**

> Die Duplikatprüfung ist eine reine Kernfunktion `FindDuplicateCandidates(input) []EventID` und **kein** Ablehnungsgrund in `SaveEvent`. `SaveEvent` nimmt ein Argument `DuplicatePolicy` (`reject` | `allow`). Das Admin-Formular ruft zuerst mit `reject`; bei Treffern zeigt es die Kandidaten und bietet „trotzdem anlegen“ (`allow`). `CommitImport` ruft mit `allow` nur für Einträge, deren Entscheidung `createNew` ist, sonst mit `reject`. Beim Bearbeiten eines vorhandenen Events zählt das Event selbst nicht als Kandidat.

---

### F-3 — Ablaufplan ohne Zeitmodell und ohne Eigentümer

**Paar:** E-ADM × E-IMP × E-API.

**Konstruktion.** FR-4: Programmpunkte mit Beschreibung, Beginn, optional Ende, chronologisch sortiert. Der Spine erwähnt `TIMETABLE_ENTRIES` nur im ER-Seed (`id`, `event_id`). AD-3 gilt ausdrücklich für Events, nicht für Programmpunkte.

- E-ADM baut Programmpunkte als reine Uhrzeiten (`time`), weil ein Formular unter einem Event das nahelegt. Mehrtägige Kirchweih (Fr–Mo) ist damit nicht abbildbar; Sortierung über Tage falsch.
- E-IMP übernimmt aus der Testsammlung `timetable[].startTime` als vollständigen Zeitpunkt (Datum + Uhrzeit, ggf. `00:00` als „unbekannt“ — genau der Platzhalter, den AD-3 verbietet).
- E-API gibt Programmpunkte mit Offset aus (KON-4) oder ohne — keine Regel.
- Eigentum: Ist der Programmpunkt eine eigene Entität mit eigenen Endpunkten (`SaveTimetableEntry`) oder ein Wertobjekt des Events? E-ADM baut eigene CRUD-Routen pro Punkt (UUIDs stabil), E-IMP ersetzt beim Update die ganze Liste (neue UUIDs). Beide AD-konform.
- Darf ein Programmpunkt außerhalb des Event-Zeitraums liegen? Verlängert er `effectiveEnd`? Nicht geregelt.

**Vorgeschlagene Regel — AD-15 (neu): Ablaufplan als Teil des Event-Aggregats**

> Der Ablaufplan ist ein Wertobjekt des Events, kein eigenständiges Aggregat. Er wird nur über `SaveEvent` geschrieben, immer als **vollständige Liste** (Ersetzen; Zeilen-IDs sind intern und nicht Teil des Vertrags, die API liefert keine IDs für Programmpunkte). Jeder Programmpunkt hat `description`, `startDate`, `startTime?`, `endDate?`, `endTime?` mit denselben Regeln wie AD-3 (leere Uhrzeit = unbekannt). Validierung im Kern: Beginn des Punkts ≥ `startDate` des Events, Ende ≤ effektives Ende des Events; sonst `ErrValidation`. Programmpunkte beeinflussen `effectiveStart/End` nicht. Sortierung: nach (Beginn-Datum, Beginn-Uhrzeit mit „unbekannt“ zuerst, Eingabereihenfolge); `timetable_entries` hat eine Spalte `position`, vom Kern gesetzt. Ausgabe wie KON-4 (`start`/`end` + `startPrecision`).

---

### F-4 — Veraltete Entscheidungen beim Import-Commit

**Paar:** E-IMP × E-ADM (gleichzeitige Bearbeitung).

**Konstruktion.** AD-10: Der Zwischenstand liegt im Browser; beim Speichern klassifiziert der Kern **erneut** gegen den aktuellen Bestand. Was mit den mitgeschickten Entscheidungen passiert, wenn die neue Klassifizierung von der Vorschau abweicht, regelt AD-10 nicht.

Szenarien zwischen Vorschau und Commit (Admin bearbeitet im zweiten Tab):

1. Eintrag war `duplicateSuspect` gegen Event X, Entscheidung „überschreiben“. X wurde gelöscht → jetzt `new`. E-IMP-Team A legt neu an (Entscheidung ignoriert), Team B verwirft (Entscheidung passt nicht).
2. Eintrag war `duplicateSuspect` gegen X, jetzt gegen X **und** Y (Admin hat Y angelegt). „Überschreiben“ welches?
3. Eintrag war `new`, jetzt `duplicateSuspect` (Admin hat das Event in der Zwischenzeit von Hand angelegt). Ohne Entscheidung → nicht übernommen (AD-10) — die Zusammenfassung muss das melden, sonst ist es „still verworfen“.
4. Eintrag war `update` auf X, X wurde inzwischen vom Admin korrigiert. Überschreibt der Import die Korrektur still?
5. Wie wird eine Entscheidung überhaupt einem Eintrag zugeordnet? Index in der Datei, `importKey`, Hash? Die Datei liegt nach AD-10 nicht auf dem Server; das Formular schickt „den vollständigen Satz“ — aus dem Browser, also manipulierbar/editierbar.

Beide Teams folgen AD-10 wörtlich und erzeugen unterschiedliche Bestände.

**Vorgeschlagene Regel — AD-10 (verschärft)**

> Jede Entscheidung ist an `(entryIndex, entryHash, targetEventId?, targetVersion?)` gebunden. `entryHash` ist ein Kern-Hash über die kanonisierte `EventInput` des Eintrags; `targetVersion` ist die `version` (AD-16) des Ziel-Events zum Vorschauzeitpunkt. Beim Commit klassifiziert der Kern neu. Stimmen Klasse, Ziel-ID und Ziel-Version mit der Vorschau überein, wird die Entscheidung angewandt. Andernfalls wird der Eintrag **nicht** übernommen und in der Zusammenfassung als `stale` mit Grund gemeldet (eigene Zählkategorie neben neu/aktualisiert/übersprungen/fehlerhaft). Auch `update` per `importKey` trägt `targetVersion`. Entscheidungen sind `skip` | `createNew` | `overwrite(targetEventId)`; `overwrite` ohne explizite Ziel-ID ist ungültig.

---

## Hoch

### F-5 — Update-Semantik: Ersetzen oder Zusammenführen?

**Paar:** E-IMP × E-ADM.

**Konstruktion.** FR-17 „Aktualisierung“, FR-18 „überschreiben“ — nirgends definiert, ob fehlende Felder in der Import-Datei vorhandene Werte löschen.

- E-IMP baut „überschreiben“ als vollständiges Ersetzen: `note` fehlt in der Datei → Notiz, die der Admin von Hand ergänzt hat, ist weg; Ablaufplan fehlt → gelöscht.
- E-ADM-Formular sendet immer alle Felder (vollständiges Ersetzen).
- Ein drittes Team baut `SaveEvent` als Patch (nur gesetzte Felder). Dann kann der Admin eine Notiz nie löschen (leer = „nicht geändert“).

**Vorgeschlagene Regel — AD-16 (neu): Ersetzen und Versionierung**

> `SaveEvent` und `SaveLocation` ersetzen den Datensatz **vollständig** mit der übergebenen Eingabe (PUT-Semantik, kein Patch). Ein fehlendes optionales Feld bedeutet „leer“. Import-`update` und `overwrite` folgen derselben Semantik. Die Vorschau zeigt für `update`/`overwrite` einen Feld-Diff, damit nichts still verloren geht. Jede Zeile in `events` und `locations` hat eine vom Kern hochgezählte `version` (int). `SaveEvent`/`SaveLocation` beim Ändern verlangen die erwartete Version; Abweichung → `ErrConflict` (siehe F-18). Der Import ändert `importKey` nur nach F-14.

### F-6 — DST: nicht existierende und doppelte Ortszeiten, „Ende des Tages“

**Paar:** E-ADM (Eingabe) × E-API (Ausgabe) × E-ARC (Vorbei-Regel).

**Konstruktion.** AD-4 rechnet lokale Werte in `timestamptz` um. Nicht geregelt:

- **Lücke (Frühjahr, 2027-03-28 02:00–03:00 gibt es nicht):** Ein Event „02:30“ — Go `time.Date` normalisiert stillschweigend auf 03:30 (bzw. 01:30 je nach Implementierung/Version-Semantik), PostgreSQL `timestamp AT TIME ZONE` auf 03:30. Die API (E-API) formatiert aus `startDate`+`startTime` lokal → `02:30+01:00` oder `02:30+02:00`, während `effectiveStart` 03:30 ist. Zwei Wahrheiten.
- **Doppelung (Herbst, 2026-10-25 02:00–03:00 zweimal):** Welcher Offset bei Ausgabe von „02:30“? E-API wählt `+02:00`, ein anderer Pfad `+01:00`.
- **„Ende des Tages“:** E-ARC-Team implementiert `23:59:59`, E-API-Team `23:59:59.999999`, ein drittes `nächster Tag 00:00` mit `>`-Vergleich. Mit AD-4 (`effectiveEnd > now`) ergibt `23:59:59` eine Lücke von einer Sekunde, in der ein Event in **keinem** Zugriff ist — Verstoß gegen FR-12 („zu jedem Zeitpunkt in genau einem“) nur, wenn ein anderer Pfad anders rechnet; aber die Filter-Überschneidung (F-8) kippt sicher. Tageslänge ist an DST-Tagen 23 bzw. 25 h — wer `start + 24h` rechnet, liegt falsch.
- `tzdata`: Im Multi-Stage-Dockerfile (distroless/scratch) fehlt `/usr/share/zoneinfo`; `time.LoadLocation("Europe/Berlin")` schlägt fehl oder fällt auf UTC zurück.

**Vorgeschlagene Regel — AD-4 (verschärft)**

> Es gibt genau eine Kernfunktion `ToInstant(date, time?, boundary)`, die alle lokalen Werte in Instants umrechnet; kein Adapter rechnet selbst. Tagesgrenzen sind halboffen: Tagesbeginn = lokales `00:00` des Datums, **Tagesende = lokales `00:00` des Folgetags** (per `time.Date(y, m, d+1, 0,0,0,0, berlin)`, nie `+24h`, nie `23:59:59`). Eine Uhrzeit in der Frühjahrslücke wird beim Schreiben mit `ErrValidation` abgelehnt („Uhrzeit existiert in Europe/Berlin nicht“). Eine doppelte Herbst-Uhrzeit wird auf das **frühere** Vorkommen (Sommerzeit, `+02:00`) festgelegt. Die API-Ausgabe von `start`/`end` mit Uhrzeit wird aus `effectiveStart`/`effectiveEnd` formatiert (in Europe/Berlin), nicht aus den lokalen Feldern neu berechnet. `cmd/eventstore` importiert `_ "time/tzdata"`; beim Start wird `Europe/Berlin` geladen, Fehler = Abbruch. Unit-Tests decken 2026-10-25 und 2027-03-28 ab.

### F-7 — Ungültige Kombinationen im Zeitmodell

**Paar:** E-ADM × E-IMP (beide befüllen `EventInput`).

**Konstruktion.** AD-3 erlaubt fünf unabhängige Felder; die Ableitung der Genauigkeit „getrennt für Beginn und Ende“ ist nicht spezifiziert.

- `allDay=true` mit `startTime=18:00`: E-ADM blendet die Uhrzeit aus und speichert sie trotzdem; E-IMP lehnt ab. Abgeleitete Genauigkeit `allDay` oder `exact`?
- `allDay` gilt für Beginn und Ende gleichzeitig? Ein Event „Fr ganztägig bis So 18:00“ ist nicht darstellbar — oder doch, wenn ein Team `allDay` nur auf den Beginn bezieht.
- `endTime` ohne `endDate`: E-ADM interpretiert als „am Beginn-Tag“, E-IMP lehnt ab. Mit `startTime=20:00, endTime=02:00` (Party über Mitternacht): E-ADM lehnt ab (FR-1 „Ende vor Beginn“), E-IMP setzt implizit Folgetag.
- Unterschied `dateOnly` vs. `allDay` für die Vorbei-Regel: beide → Tagesende; für die Ausgabe identisch (`YYYY-MM-DD`). Abnehmer unterscheiden sie nur über die Genauigkeit — gut, aber nur wenn beide Pfade sie gleich ableiten.
- „Ende vor Beginn“ bei `startDate=endDate`, `startTime` gesetzt, `endTime` leer: gültig? (Ende = Tagesende, also ja — aber nur, wenn der Vergleich auf Instants und nicht auf Feldern läuft.)

**Vorgeschlagene Regel — AD-3 (verschärft)**

> Gültige Kombinationen, geprüft im Kern (`ValidateTiming`), sonst `ErrValidation`:
> 1. `allDay=true` ⇒ `startTime` und `endTime` leer; `endDate` optional. Genauigkeit Beginn = `allDay`, Ende = `allDay` falls `endDate` gesetzt, sonst kein Ende.
> 2. `allDay=false`: Beginn-Genauigkeit = `exact` wenn `startTime` gesetzt, sonst `dateOnly`. Ende: `endTime` gesetzt ⇒ `endDate` **Pflicht**; Genauigkeit `exact`. Nur `endDate` ⇒ `dateOnly`. Nichts ⇒ kein Ende (`endPrecision` fehlt in der Ausgabe).
> 3. Ende vor Beginn wird auf den von `ToInstant` berechneten `effectiveEnd < effectiveStart` geprüft (und `effectiveEnd == effectiveStart` ist bei `exact`/`exact` erlaubt).
> 4. Über-Mitternacht verlangt ein explizites `endDate`; es gibt keine implizite Folgetag-Logik.
>
> Gemischte Genauigkeit „ganztägiger Beginn, exaktes Ende“ ist in v1 nicht darstellbar (bewusst).

### F-8 — `from`/`to`, „heute“ und Überschneidung: Grenzen

**Paar:** E-API (aktive Liste) × E-API (Archiv) — zwei Stories, ein Adapter.

**Konstruktion.** KON-5: `from`/`to` akzeptieren Datum **oder** Zeitpunkt, beide inklusive. AD-4 sagt nur „filtern über die beiden Spalten“.

- Story A: `to=2026-12-24` → `effectiveStart <= '2026-12-24T00:00+01:00'`; ein Event am 24.12. um 18:00 fehlt. Story B: `to` → Tagesende. Beide AD-4-konform.
- `to` als Zeitpunkt ohne Offset (`2026-12-24T18:00`)? Lokal oder UTC?
- `from > to`? Fehler oder leer?
- Überschneidung: `effectiveStart <= to AND effectiveEnd >= from` vs. `<`/`>` — ein Event, das genau um `from` endet, ist bei der einen Story drin, bei der anderen nicht.
- „Heute“ (FR-8): Archiv-Story und Aktiv-Story nutzen dieselbe Überschneidungslogik? Aktiv = `effectiveEnd > now` **und** überschneidet [heute 00:00, morgen 00:00). Bei Zeitraumfilter: wird „heute“ ersetzt oder zusätzlich angewandt? Ohne `from`, aber mit `to` — ab wann?
- Archiv mit Zeitraumfilter: Überschneidung des Event-Zeitraums mit dem Filter, oder nur `effectiveEnd` im Filter?

**Vorgeschlagene Regel — AD-17 (neu): Zeitraum-Normalisierung im Kern**

> Der Kern wandelt jeden Filter in ein halboffenes Intervall `[lo, hi)` aus Instants um, bevor ein Repository aufgerufen wird. `from` als Datum ⇒ `lo` = Tagesbeginn; als Zeitpunkt ⇒ dieser Instant. `to` als Datum ⇒ `hi` = Tagesende (Folgetag 00:00, AD-4); als Zeitpunkt ⇒ `hi = to + 1 µs` (Inklusivität von KON-5, auf die DB-Auflösung von `timestamptz` abgebildet). Zeitpunkte ohne Offset werden mit `ErrValidation` abgelehnt. `from > to` ⇒ `ErrValidation`. Überschneidung ist immer genau `effectiveStart < hi AND effectiveEnd > lo` — ein Prädikat, keine Variante. Aktive Liste: ohne Filter `lo = now`, `hi = morgen 00:00` (Europe/Berlin); mit Filter `lo = from`, `hi` wie oben, zusätzlich immer `effectiveEnd > now`. Fehlt `from` bei gesetztem `to`, ist `lo = now`; fehlt `to`, ist `hi = +∞`. Archiv: dieselbe Überschneidung, zusätzlich `effectiveEnd <= now`. Repositories bekommen nur `(lo, hi, now, types)` als Parameter.

*(Hinweis: Einfacher wäre, KON-5 für Zeitpunkt-`to` auf „exklusiv“ zu ändern. Das ist eine Vertragsentscheidung; die Regel verlangt nur, dass genau eine Kernstelle sie trifft.)*

### F-9 — Zwei Uhren: Go `now` vs. SQL `now()`

**Paar:** E-API × E-ARC.

**Konstruktion.** AD-2 erlaubt „einfache Vergleiche auf gespeicherten Spalten“. AD-4: aktiv heißt `effectiveEnd > now`.

- E-API schreibt in sqlc `WHERE effective_end > now()` (einfacher Vergleich, AD-2-konform).
- Der Kern berechnet „heute“ (AD-7) mit `time.Now()` im Container.
- E-ARC setzt `archived_at = now()` per SQL.
- Bei Uhrabweichung zwischen App und DB (verschiedene Railway-Hosts) oder in Tests mit fester Uhr laufen „heute“-Grenze und Aktiv-Grenze auseinander; Tests für FR-8 („ab 14:00 nicht mehr“) sind ohne injizierbare Uhr nicht deterministisch. `now()` ist zudem transaktionsstabil, `time.Now()` nicht.

**Vorgeschlagene Regel — AD-2 (verschärft)**

> SQL ruft nie `now()`, `current_date`, `current_timestamp` oder `AT TIME ZONE` auf. Der aktuelle Zeitpunkt kommt aus einem Kern-Port `Clock` und wird jeder Abfrage als Parameter übergeben. Pro Request wird `now` genau einmal gelesen und für alle Berechnungen dieses Requests verwendet.

### F-10 — Normalisierung von Namen/Titeln vs. „keine Regeln in SQL“

**Paar:** E-ORT × E-IMP.

**Konstruktion.** AD-11: Ortsnamen eindeutig, verglichen nach Trimmen und ohne Groß-/Kleinschreibung; Duplikat bei gleichem `title`.

- E-ORT sichert die Eindeutigkeit mit `CREATE UNIQUE INDEX … ON locations (lower(btrim(name)))`. Das ist eine Fachregel in SQL — verboten nach AD-2 („keine Funktionen mit Fachbedeutung“). Ohne Index gibt es aber bei gleichzeitigem Import und Admin-Anlage zwei „Paul-Metz-Halle“.
- E-IMP vergleicht in Go mit `strings.EqualFold(strings.TrimSpace(a), …)`; PostgreSQL `lower()` und Go `EqualFold` unterscheiden sich bei Unicode (ß/ẞ, türkisches I, Kombinationszeichen „Fürth“ als NFC vs. NFD aus einer kopierten Webseite). Zwei Pfade, zwei Ergebnisse.
- Titel beim Duplikatvergleich: exakt, getrimmt, case-insensitiv? AD-11 sagt nur „gleich“. E-ADM (F-2) und E-IMP wählen verschieden. „Kirchweih Weinzierlein “ mit Leerzeichen am Ende wird so zum Nicht-Duplikat.

**Vorgeschlagene Regel — AD-11 (verschärft), AD-2 (klargestellt)**

> Der Kern hat genau eine Funktion `NormalizeKey(s string) string`: Unicode-NFC, Trimmen, Mehrfach-Leerraum zu einem Leerzeichen, `strings.ToLower` (Go-Semantik, kein Case-Folding in SQL). Sie gilt für Ortsnamen und für Event-Titel im Duplikatvergleich. Der Kern speichert das Ergebnis in eigenen Spalten `locations.name_key` (UNIQUE) und `events.title_key` (Index). SQL vergleicht nur diese Spalten mit `=`. **AD-2 Klarstellung:** Integritäts-Constraints auf gespeicherten Spalten (PK, FK, UNIQUE, NOT NULL, CHECK auf reine Wertebereiche) sind erlaubt und erwünscht; Ausdrucksindizes und Funktionen mit Fachbedeutung nicht. Constraint-Verletzungen übersetzt der Postgres-Adapter in `ErrConflict`.

### F-11 — Enum-Drift: OpenAPI ↔ Import-Schema ↔ Kern ↔ Admin-Labels

**Paar:** E-API × E-IMP × E-ADM.

**Konstruktion.** AD-8: OpenAPI ist einzige Quelle für Enum-Codes. AD-9: Import-Schema „identisch“. AD-1/AD-2: Der Kern validiert (FR-5 „unbekannter Typ wird abgelehnt“) und darf `publicapi` (generierte Typen) nicht importieren.

- Also braucht der Kern eine eigene Go-Liste der Codes. Das ist eine **zweite** Quelle neben OpenAPI — von AD-8 nicht erlaubt, von AD-1 erzwungen.
- `import-v1.schema.json` ist eine dritte Liste, die Admin-Auswahlfelder mit deutschen Labels eine vierte.
- `ListEventTypes` liefert Codes — und Labels? Deutsch (Inhalt) oder Englisch (technisch)? Wer besitzt die Labels?
- Ein neuer Typ wird in OpenAPI ergänzt (rückwärtskompatibel? NFR-2 sagt: Änderung an der Liste ist inkompatibel), Import-Schema vergessen → Import lehnt ab, API liefert.

**Vorgeschlagene Regel — AD-9 (verschärft)**

> Der Kern hält die Enum-Listen (`EventType`, `Precision`, `LocationPrecision`) als Go-Konstanten mit deutschem Label (`core.EventTypes() []EventTypeInfo{Code, LabelDE}`). OpenAPI bleibt Vertragsquelle; ein CI-Test liest `openapi.yaml` und `import-v1.schema.json` und schlägt fehl, wenn die Code-Mengen nicht exakt mit dem Kern übereinstimmen. `ListEventTypes` liefert `{code, label}` mit deutschem `label` (Inhalt, KON-7). Das Admin-UI nutzt nur `core.EventTypes()`. Der Import-Validator ist **der Kern**, nicht der JSON-Schema-Validator; das Schema dient der Doku und optional einer Vorprüfung, beide müssen über denselben CI-Test gleich bleiben.

---

## Mittel

### F-12 — Neuberechnung von `effective*` bei Regeländerung oder tzdata-Update

**Paar:** E-ARC × Migrationen.

**Konstruktion.** AD-4 berechnet `effective*` nur „bei jedem Schreiben“. Ändert sich die Vorbei-Regel (z. B. F-6 „Tagesende“ wird korrigiert) oder die Zeitzonendatenbank (EU schafft Zeitumstellung ab), sind alle gespeicherten Werte veraltet. Eine goose-SQL-Migration darf sie nach AD-2 nicht neu berechnen (Regel in SQL). Team A schreibt trotzdem eine SQL-Migration mit `AT TIME ZONE`; Team B wartet, bis jedes Event erneut gespeichert wird. Der Ort ist **kein** Eingang der Berechnung — das sollte festgehalten werden, damit niemand bei Orts-Änderungen neu berechnet.

**Vorgeschlagene Regel — AD-4 (verschärft)**

> `effectiveStart/End` hängen nur von den Zeitfeldern des Events und der Zone Europe/Berlin ab, nie vom Ort. Die Tabelle `events` hat `effective_rule_version int`. Der Kern führt eine Konstante `EffectiveRuleVersion`. Beim Start nach den Migrationen und vor dem HTTP-Server läuft der Kern-Anwendungsfall `RecomputeEffective`, der alle Events mit kleinerer Version neu berechnet und speichert (idempotent, in Batches). Jede Änderung an `ToInstant` oder der Vorbei-Regel erhöht die Konstante. Die tzdata-Version ist über `time/tzdata` an die Go-Version gebunden; ein Go-Upgrade erhöht die Konstante, wenn sich Europe/Berlin ändert.

### F-13 — `archivedAt` nach Reaktivierung; „als archiviert erkennbar“

**Paar:** E-ADM × E-ARC × E-API.

**Konstruktion.** AD-13: Der Job setzt `archivedAt` nur. FR-15: Ein korrigiertes Event wird wieder aktiv. E-ADM setzt `archivedAt` nicht zurück (keine Regel) → Statistik zählt aktive Events als archiviert, und ein Team, das in der Admin-Liste nach `archivedAt` gruppiert, zeigt sie falsch. FR-11: Einzelabruf „als archiviert erkennbar“ — E-API-Story leitet `archived` aus `archivedAt != null` ab (bequem, ist ja im Datensatz), was AD-5 verbietet, aber nur für „Lese-Abfragen“ formuliert ist, nicht für die Abbildung.

**Vorgeschlagene Regel — AD-5 (verschärft)**

> `SaveEvent` setzt `archivedAt = NULL`, wenn der neue `effectiveEnd > now`. Das Feld `archived` in API und Admin wird ausschließlich im Kern als `effectiveEnd <= now` berechnet; `archivedAt` wird in keinem Kern-Objekt für Leser exponiert und nirgends ausgeliefert.

### F-14 — Lebenszyklus des `importKey`

**Paar:** E-ADM × E-IMP.

**Konstruktion.** Nicht geregelt: Zeigt/ändert das Admin-Formular `importKey`? Wenn der Admin ein von Hand angelegtes Event per Import „überschreibt“ (FR-18), bekommt es den `importKey` aus der Datei? Wenn ja und das vorhandene Event hat schon einen anderen Key — Fehler oder Ersetzen? Zwei Einträge derselben Datei mit gleichem Key? SM-2 („zweiter Import erzeugt 0 neue“) hängt daran: Wird bei `overwrite` der Key nicht übernommen, wird ein Event ohne Key beim zweiten Import erneut `duplicateSuspect` und kann bei falscher Entscheidung doppelt angelegt werden. Ein Team setzt den Key, eines nicht — beide AD-konform.

**Vorgeschlagene Regel — AD-11 (verschärft)**

> `importKey` ist im Admin-Formular sichtbar, aber nur leerbar, nicht frei editierbar. `overwrite` und `createNew` aus dem Import setzen den `importKey` des Eintrags. Hat das Ziel bereits einen **anderen** Key, ist der Eintrag `error` („importKey-Konflikt“). Doppelte Keys innerhalb einer Datei machen alle betroffenen Einträge zu `error`. Ein Eintrag mit Key, der im Bestand fehlt, wird trotzdem auf Duplikatverdacht geprüft (der Key allein macht ihn nicht zu `new`). `importKey` ist case-sensitiv und getrimmt.

### F-15 — Neue Orte im Import: mehrfach in einer Datei, Abweichung vom Bestand

**Paar:** E-IMP × E-ORT.

**Konstruktion.** FR-16: Mitgebrachter Ort mit vorhandenem Namen wird dem vorhandenen zugeordnet. Offen: (a) Zehn Events in einer Datei bringen denselben neuen Ort mit — E-IMP legt ihn zehnmal an (UNIQUE-Verletzung → ganze Transaktion bricht ab, F-17) oder einmal. (b) Der mitgebrachte Ort hat andere Koordinaten als der vorhandene gleichen Namens — still ignoriert (FR-16 wörtlich) oder Vorschau-Hinweis? E-ORT erwartet, dass Ortsänderungen nur über `SaveLocation` laufen; E-IMP aktualisiert Koordinaten „nebenbei“. (c) Verweis auf vorhandenen Ort: per `locationId` (UUID, die die Datei nicht kennen kann, solange es keinen Export gibt) oder per Name? (d) `newLocation` ist in AD-10 eine Klasse neben `new`/`update` — aber ein Event kann gleichzeitig `new` sein **und** einen neuen Ort brauchen. Klasse pro Event oder pro Ort?

**Vorgeschlagene Regel — AD-10/AD-11 (verschärft)**

> Die Klassifizierung hat zwei Achsen: pro Event `new | update | duplicateSuspect | error` und pro mitgebrachtem Ort (dedupliziert über `NormalizeKey(name)` innerhalb der Datei) `existing | create | error`. Ein Ort wird pro Commit höchstens einmal angelegt, vor den Events, in derselben Transaktion. Der Import ändert **nie** vorhandene Orte; weichen Adresse/Koordinaten eines mitgebrachten Orts vom vorhandenen ab, zeigt die Vorschau einen Hinweis, übernommen werden die vorhandenen Werte. Widersprüchliche Angaben zum selben neuen Ort innerhalb der Datei ⇒ alle betroffenen Einträge `error`. In der Datei wird ein Ort entweder per `locationId` oder per `location` (mit `name`) referenziert; nur-Name ohne Adresse/Koordinaten ist erlaubt, wenn der Name existiert.

### F-16 — Löschschutz-Race und DB-Integrität

**Paar:** E-ORT × E-IMP.

**Konstruktion.** AD-6: Löschschutz liegt im Kern. `DeleteLocation` zählt Events (0) und löscht; parallel committet ein Import ein Event auf diesen Ort. Ohne FK `ON DELETE RESTRICT` entsteht ein verwaister Verweis. E-ORT liest AD-2 so, dass FK-Verhalten „Regel in SQL“ ist, und lässt ihn weg.

**Vorgeschlagene Regel — AD-6 (verschärft)**

> Der Kern prüft den Löschschutz für die verständliche Meldung; die Datenbank sichert ihn mit `events.location_id … REFERENCES locations ON DELETE RESTRICT`. Der Postgres-Adapter übersetzt die FK-Verletzung in `ErrConflict`. `timetable_entries.event_id` hat `ON DELETE CASCADE` (Wertobjekt, AD-15).

### F-17 — Eine Transaktion vs. „Fehler verhindern nicht den Rest“

**Paar:** E-IMP intern (Klassifizierung × Persistenz).

**Konstruktion.** AD-10: alles in **einer** Transaktion. FR-17: fehlerhafte Einträge verhindern nicht den Import der übrigen. Team A validiert alles vorab und bricht bei einem DB-Fehler (UNIQUE auf `importKey`, F-10, F-15) die gesamte Transaktion ab. Team B nutzt Savepoints pro Eintrag und übernimmt den Rest. Beide AD-konform, verschiedenes Ergebnis.

**Vorgeschlagene Regel — AD-10 (verschärft)**

> Alle fachlichen Fehler werden in der Neu-Klassifizierung **innerhalb** der Commit-Transaktion erkannt (Lesen und Schreiben in derselben Transaktion, Isolationsstufe `REPEATABLE READ`; zusätzlich `pg_advisory_xact_lock` auf eine feste Import-Sperre, damit zwei Commits nicht parallel laufen). Einträge mit Fehler werden übersprungen und gemeldet. Ein unerwarteter DB-Fehler bricht den gesamten Commit ab; dann wird nichts übernommen und die Meldung sagt das. Keine Savepoints pro Eintrag.

### F-18 — Lost Update bei zwei Admin-Tabs

**Paar:** E-ADM × E-ADM (bzw. E-ADM × E-IMP).

**Konstruktion.** Ein Admin, aber mehrere Tabs/Geräte. Tab 1 öffnet Event, Tab 2 korrigiert das Datum, Tab 1 speichert mit altem Datum → Korrektur still verloren. Ohne Regel baut ein Team Optimistic Locking, ein anderes nicht.

**Vorgeschlagene Regel** — abgedeckt durch AD-16 (`version` + `ErrConflict`); das Formular trägt die Version als verstecktes Feld, bei Konflikt zeigt es den aktuellen Stand.

---

## Niedrig

### F-19 — Sortierung ohne Tiebreak

KON-6: nach Beginn. AD-7: Kern legt Sortierung fest — aber nach `effectiveStart` oder nach (`startDate`, `startTime`)? Bei gleichem Beginn (drei Events in der Paul-Metz-Halle, alle `dateOnly`) ist die Reihenfolge nicht deterministisch; E-API-Tests flackern, Abnehmer sehen springende Listen.

**Regel — AD-7 (verschärft):** Sortierung aktiv: `effectiveStart ASC, title_key ASC, id ASC`. Archiv: `effectiveStart DESC, title_key ASC, id ASC`. Der Kern übergibt die Richtung; SQL setzt sie als feste `ORDER BY` pro Abfrage um.

### F-20 — Ort-Liste und Event-Typ-Liste

`ListLocations`: alle Orte oder nur solche mit aktiven Events? Mit oder ohne Notiz? Sortierung? `ListEventTypes`: siehe F-11.

**Regel — AD-7 (klargestellt):** `ListLocations` liefert alle Orte vollständig (Form `Location` wie in `Event.location`), sortiert nach `name_key`. `ListEventTypes` liefert `{code, label}` in der Reihenfolge der Kernliste.

### F-21 — Löschen von Events

FR-15 erlaubt Löschen, auch archivierter Events. Hart oder weich? Kaskade auf Ablaufplan? Abnehmer, die eine ID gecacht haben, bekommen 404 — gewollt?

**Regel — AD-6 (klargestellt):** Löschen ist hart (`DeleteEvent`), Ablaufplan per Kaskade. Danach liefert `GetEvent` `404` Problem Details. Kein Tombstone in v1.

### F-22 — Start-Job während Rolling Deploy

Railway startet die neue Instanz, während die alte noch läuft: Migrationen laufen gegen eine DB, die die alte Version noch liest; der Bereinigungsjob läuft in beiden Instanzen. Durch Idempotenz (AD-13) unkritisch; Migrationen müssen aber rückwärtsverträglich mit der laufenden Version sein (expand/contract), sonst bricht die alte Instanz kurz.

**Regel — AD-13 (klargestellt) / Konvention Migrationen:** Migrationen sind für mindestens eine Vorgängerversion des Codes verträglich (nur additive Änderungen in einem Release; Entfernen erst im nächsten). Bereinigungsjob und `RecomputeEffective` (F-12) laufen unter `pg_try_advisory_lock`; ist die Sperre belegt, wird übersprungen.

---

## Zusammenfassung der neuen/verschärften ADs

| AD | Änderung | Schließt |
| --- | --- | --- |
| AD-2 | `now` nur aus Kern-`Clock`; Integritäts-Constraints erlaubt, Ausdrucksindizes nicht | F-9, F-10, F-16 |
| AD-3 | Gültige Kombinationen, `endTime` ⇒ `endDate`, kein impliziter Folgetag | F-7 |
| AD-4 | Eine `ToInstant`-Funktion, halboffene Tagesgrenzen, DST-Lücke abgelehnt, Herbst früher Offset, `time/tzdata`, `effective_rule_version` + `RecomputeEffective` | F-6, F-12 |
| AD-5 | `archivedAt` wird bei Reaktivierung geleert, `archived` nur aus `effectiveEnd` | F-13 |
| AD-6 | FK `RESTRICT`/`CASCADE`, hartes Löschen | F-16, F-21 |
| AD-7 | Deterministische Sortierung, Inhalt der Nachschlagelisten | F-19, F-20 |
| AD-9 | Import-Schema = `EventInput` per `$ref`; Enum-Gleichheit Kern/OpenAPI/Schema per CI | F-1, F-11 |
| AD-10 | Entscheidungen an Ziel-ID + Version gebunden, `stale`-Kategorie, zwei Klassifizierungsachsen, Advisory Lock, kein Savepoint | F-4, F-15, F-17 |
| AD-11 | Duplikatprüfung nicht in `SaveEvent` (Policy), `NormalizeKey` + `*_key`-Spalten, `importKey`-Lebenszyklus | F-2, F-10, F-14 |
| AD-13 | Advisory Lock, expand/contract-Migrationen | F-22 |
| **AD-14 (neu)** | `EventInput` vs. `Event`, Koordinatennamen | F-1 |
| **AD-15 (neu)** | Ablaufplan als Wertobjekt mit AD-3-Zeitmodell, Ersetzen, Validierung | F-3 |
| **AD-16 (neu)** | PUT-Semantik, `version` für Optimistic Locking, Diff in der Vorschau | F-5, F-18 |
| **AD-17 (neu)** | Zeitraum-Normalisierung `[lo, hi)` im Kern, einheitliches Überschneidungsprädikat | F-8 |

Außerdem: Die `[ASSUMPTION]`-Marker in AD-11 und bei den Enum-Codes sollten vor dem Bau bestätigt werden. Die Codes sind Vertragsbestandteil (NFR-2) und später nur mit einer neuen Hauptversion änderbar.
