---
title: "Sprint Change Proposal: Keine Kennungen nach außen, Events nur als Listen"
status: approved
created: 2026-10-04
author: Andreas (mit Developer-Agent, bmad-correct-course)
scope: moderate
---

# Sprint Change Proposal: Keine Kennungen nach außen, Events nur als Listen

## 1. Problem

**Problem:** Die Planung sieht vor, dass die öffentliche API interne Werte ausgibt: die Datenbank-Kennung (UUIDv7) als `id` jedes Events und jedes Orts, einen Einzelabruf `GET /v1/events/{id}` und eine Ortsliste `GET /v1/locations`, deren Zweck die Ort-ID ist. Auch das öffentliche Import-Schema verweist über `locationId` auf interne Kennungen. Eine UUIDv7 verrät den Anlagezeitpunkt, vor allem aber zwingt ein Einzelabruf Abnehmer dazu, Kennungen zu speichern. Das ist nicht gewollt: Abnehmer sollen Events nur als gefilterte Listen abfragen und nichts speichern müssen.

**Wann es aufgefallen ist:** Nach Abschluss von Story 2.1, vor Story 2.2. Der Punkt stand als offener Punkt im Sprint Change Proposal vom 2026-10-02 und im Epic-2-Kontext („vor Story 2.2 klären“).

**Belege:**

- README, Beispiel-JSON: `"id": "0192f0c4-…"` beim Event und `"id": "0192f0b1-…"` beim Ort.
- README, Ressourcen: `GET /v1/events/{id}` und `GET /v1/locations` („Events am selben Ort haben dieselbe Ort-ID“).
- Story 2.2 und AD-14: `id` und `location.id` in der Leseform `Event`.
- AD-14, ENT-18 und Story 3.1: `locationId` (`oneOf` mit `importLocation`) im öffentlichen Import-Schema.

**Kategorie:** Missverständnis der ursprünglichen Anforderung.

## 2. Auswirkungen

### Epics

- **Epic 1:** keine Auswirkung. Die Kennungen bleiben intern (Datenbank, Kern, Admin).
- **Epic 2:** Story 2.2 entfällt, weil ihre beiden Endpunkte entfallen. Story 2.3 übernimmt die Leseform `Event`, die Schreibform `EventInput` und FR-10. Story 2.1 ist nicht betroffen, die Stories 2.4 bis 2.6 bleiben unverändert.
- **Epic 3:** Der mitgebrachte Ort heißt `location`, `locationId` entfällt (Stories 3.1, 3.2, 3.4). Das Verhalten ändert sich kaum, weil FR-16 einen mitgebrachten Ort schon heute über den Namen zuordnet.
- Kein Epic kommt dazu oder entfällt, die Reihenfolge bleibt.

### Artefakte

| Artefakt | Auswirkung |
| --- | --- |
| PRD | FR-10 (Ort ohne Kennung, gleicher Ortsname, keine internen Werte), FR-11 (nur Listen, kein Einzelabruf, keine Ortsliste), FR-16 (Ort immer mitgebracht), Umfangszeile; Addendum: Ressourcenliste |
| Architektur-Spine | AD-7 (Public API ohne `GetEvent`/`ListLocations`), AD-11 (IDs rein intern), AD-14 (Formen ohne interne Werte, `location` statt `locationId`/`importLocation`), Stack-Tabelle „IDs“, `updated` |
| UX | Es gibt kein UX-Dokument. |
| `epics.md` | Inventory (FR-10, FR-11, FR-16, AD-7, AD-11, AD-14, Ressourcen, ENT-18, Testsammlung), Coverage Map (SM-3), Story 2.2 entfällt, Stories 2.3, 3.1, 3.2, 3.4 |
| README | Konventionen, Ressourcentabelle, Beispiel-JSON ohne `id` |
| `epic-2-context.md` | offener Punkt erledigt, Ressourcen, Schemas, Kern-Abfragen, Story-Liste |
| `sprint-status.yaml` | Eintrag Story 2.2 entfernt |

### Technik

- Keine Migration. UUIDv7 bleibt als interner Schlüssel und als Gleichstands-Sortierschlüssel (AD-7).
- Die Kern-Abfragen `GetEvent` und `ListLocations` bleiben für den Admin bestehen.
- Story 2.3 belegt mit einem Test, dass die JSON-Antwort nur die Felder der Leseform enthält.

### Verworfene Zwischenlösung

Zunächst war erwogen, die Kennung beizubehalten und auf zufällige UUIDv4 umzustellen (samt Neuvergabe vorhandener Kennungen per Migration). Das ist hinfällig: Ohne Einzelabruf braucht die API gar keine Kennung.

## 3. Empfohlenes Vorgehen

**Direkte Anpassung** der Planungsartefakte, ohne Rollback und ohne Änderung am MVP-Ziel.

- **Aufwand:** gering. Nur Dokumente; Story 2.3 wird etwas größer, Story 2.2 entfällt ganz, insgesamt sinkt der Umfang von Epic 2.
- **Risiko:** gering. Noch kein Code nutzt Event- oder Ort-Kennungen in der öffentlichen API.
- **Zeitplan:** keine Verzögerung. Story 2.3 ist die nächste Story.
- **Abwägung:** Abnehmer können ein bestimmtes Event nicht mehr direkt verlinken. Das ist gewollt: Sie fragen über Filter erneut ab. Ortsnamen als Gruppierungsmerkmal sind eindeutig (`name_key`); eine Umbenennung ändert die Gruppierung, was ohne gespeicherte Kennungen unkritisch ist.

## 4. Die Änderungen im Einzelnen

### 4.1 PRD (`prds/prd-oz-zirndorf-event-store-2026-10-01/prd.md`)

**FR-10**

```
OLD:
Jedes ausgelieferte Event enthält alle Angaben aus FR-1 und den vollständigen Ort (Kennung, Name, Adresse, Koordinaten, Ortsgenauigkeit, Notiz).

**Konsequenzen (testbar):**
- Zwei Events am selben Ort liefern dieselbe Ort-Kennung und identische Koordinaten.
- Zeitgenauigkeit, Ortsgenauigkeit und Quelle sind bei jedem Event in der Antwort enthalten.

NEW:
Jedes ausgelieferte Event enthält alle Angaben aus FR-1 und den vollständigen Ort (Name, Adresse, Koordinaten, Ortsgenauigkeit, Notiz).

**Konsequenzen (testbar):**
- Zwei Events am selben Ort liefern denselben Ortsnamen und identische Koordinaten. Der Ortsname ist eindeutig und kennzeichnet den Ort.
- Zeitgenauigkeit, Ortsgenauigkeit und Quelle sind bei jedem Event in der Antwort enthalten.
- Die Antwort enthält keine internen Werte: keine Kennungen von Events, Orten oder Programmpunkten, keinen Import-Schlüssel und keine Hilfswerte wie Archivierungszeitpunkt oder Vergleichsschlüssel.
```

**FR-11**

```
OLD:
#### FR-11: Einzelabruf und Nachschlagelisten

Ein Abnehmer kann ein einzelnes Event über seine Kennung abrufen sowie die Liste aller Orte und aller Event-Typen abfragen.

**Konsequenzen (testbar):**
- Der Abruf einer unbekannten Event-Kennung liefert „nicht gefunden“ im einheitlichen Fehlerformat.
- Ein archiviertes Event ist über seine Kennung weiterhin abrufbar und als archiviert erkennbar. Maßgeblich ist die Vorbei-Regel, nicht der Zeitpunkt der täglichen Bereinigung.

**Out of Scope (gesamte Lese-API):**
- Schreibzugriff jeder Art über die öffentliche API.
- Volltextsuche, Umkreissuche, Paginierung. Bei ein paar hundert Events sind sie nicht nötig.

NEW:
#### FR-11: Nur Listen, Liste der Event-Typen

Die öffentliche API liefert Events ausschließlich als Liste (FR-8, FR-9, FR-12). Zusätzlich kann ein Abnehmer die Liste aller Event-Typen abfragen. Abnehmer müssen sich keine Kennungen merken: Was sie brauchen, erfragen sie erneut über Filter.

**Konsequenzen (testbar):**
- Es gibt keinen Abruf eines einzelnen Events und keine eigene Liste der Orte. Orte erscheinen nur eingebettet im Event.
- Ob ein Event archiviert ist, erkennt der Abnehmer an `archived`. Maßgeblich ist die Vorbei-Regel, nicht der Zeitpunkt der täglichen Bereinigung.

**Out of Scope (gesamte Lese-API):**
- Schreibzugriff jeder Art über die öffentliche API.
- Einzelabruf über eine Kennung, eigene Ortsliste.
- Volltextsuche, Umkreissuche, Paginierung. Bei ein paar hundert Events sind sie nicht nötig.
```

**FR-16, dritter Punkt**

```
OLD: - Ein Event in der Import-Datei kann auf einen vorhandenen Ort verweisen oder einen Ort mitbringen. Ein mitgebrachter Ort mit demselben Namen wie ein vorhandener wird dem vorhandenen zugeordnet.
NEW: - Jedes Event in der Import-Datei bringt seinen Ort mit, mindestens mit Namen. Ein mitgebrachter Ort mit demselben Namen wie ein vorhandener wird dem vorhandenen zugeordnet. Die Import-Datei enthält keine internen Kennungen.
```

**Umfangszeile**

```
OLD: - Öffentliche, versionierte, nur lesende API mit Standardabfrage „heute“, Filtern, Einzelabruf und Nachschlagelisten (FR-8 bis FR-11)
NEW: - Öffentliche, versionierte, nur lesende API mit Standardabfrage „heute“, Filtern, Archiv und Liste der Event-Typen, ausschließlich als Listen (FR-8 bis FR-11)
```

FR-6 („stabile Kennung“) bleibt: Die Kennung existiert weiter, nur nicht öffentlich.

**Addendum (`addendum.md`), mögliche Ressourcen**

```
OLD: `GET /v1/events` (Filter `from`, `to`, `type`), `GET /v1/events/{id}`, `GET /v1/locations`, `GET /v1/event-types`, `GET /v1/archive/events`.
NEW: `GET /v1/events` (Filter `from`, `to`, `type`), `GET /v1/event-types`, `GET /v1/archive/events`. Einzelabruf und Ortsliste sind verworfen (Sprint Change Proposal 2026-10-04): Abnehmer arbeiten nur mit Listen und speichern keine Kennungen.
```

### 4.2 Architecture Spine (`ARCHITECTURE-SPINE.md`)

**AD-7, erster Satz der Rule**

```
OLD: Die Public API ruft Abfrage-Anwendungsfälle des Kerns auf: `ListActiveEvents`, `ListArchivedEvents`, `GetEvent`, `ListLocations`, `ListEventTypes`.
NEW: Die Public API ruft Abfrage-Anwendungsfälle des Kerns auf: `ListActiveEvents`, `ListArchivedEvents`, `ListEventTypes`. Sie liefert Events nur als Listen; einen Einzelabruf und eine Ortsliste gibt es nicht (FR-11). `GetEvent` und `ListLocations` dienen nur dem Admin.
```

**AD-11, IDs**

```
OLD: - **IDs:** Events, Orte und Ablaufplan-Einträge haben UUIDv7-Kennungen, erzeugt per `DEFAULT uuidv7()` in PostgreSQL 18.
NEW: - **IDs:** Events, Orte und Ablaufplan-Einträge haben UUIDv7-Kennungen, erzeugt per `DEFAULT uuidv7()` in PostgreSQL 18. Sie sind **rein intern** (Datenbank, Kern, Admin-Oberfläche) und erscheinen weder in der öffentlichen API noch im Import-Schema. Nach außen kennzeichnet der eindeutige Ortsname einen Ort (`name_key`); Events haben nach außen keine Kennung.
```

**AD-14, erste zwei Sätze der Rule (Rest unverändert)**

```
OLD: Die Schreibform `EventInput` enthält `title`, `type`, `locationId` oder `importLocation` (mitgebrachter Ort, nur Import, `oneOf`), `startDate`, `startTime`, `endDate`, `endTime`, `allDay`, `source`, `note`, `timetable` und `importKey` (nur Import). Die Leseform `Event` ist ein eigenes Schema mit denselben Feldnamen wie `EventInput` (ohne `importKey` und `importLocation`) **plus** abgeleitete Felder: `id`, `location` (vollständig), `startPrecision`, `endPrecision`, `effectiveStart`, `effectiveEnd` und `archived`.
NEW: Die Schreibform `EventInput` (Import) enthält `title`, `type`, `location` (mitgebrachter Ort: `name` Pflicht; `address`, `latitude`, `longitude`, `precision`, `note` optional, weil ein vorhandener Name sie nicht braucht), `startDate`, `startTime`, `endDate`, `endTime`, `allDay`, `source`, `note`, `timetable` und `importKey`. Die Leseform `Event` ist ein eigenes Schema mit denselben Feldnamen wie `EventInput` (ohne `importKey`), wobei `location` vollständig ist, **plus** abgeleitete Felder: `startPrecision`, `endPrecision`, `effectiveStart`, `effectiveEnd` und `archived`. **Keine Form nach außen enthält interne Werte:** keine Kennungen (Event, Ort, Programmpunkt), kein `importKey` in der Leseform, kein `archivedAt`, `title_key` oder `name_key`. Im Kern trägt `core.EventInput` entweder die interne Ortskennung (Admin-Formular) oder den mitgebrachten Ort (Import); die Spec kennt nur `location`.
```

**Stack-Tabelle**

```
OLD: | IDs | UUIDv7 als String in JSON |
NEW: | IDs | UUIDv7, nur intern. Öffentliche API und Import-Schema enthalten keine Kennungen (AD-11, AD-14) |
```

**Kopf:** `updated: '2026-10-04'`

### 4.3 Epics: Requirements Inventory (`epics.md`)

- **FR-10:** „… den vollständigen Ort (Name, Adresse, Koordinaten, Ortsgenauigkeit, Notiz). Zwei Events am selben Ort liefern denselben, eindeutigen Ortsnamen und identische Koordinaten. Zeitgenauigkeit, Ortsgenauigkeit und Quelle sind immer enthalten. Die Antwort enthält keine internen Werte (Kennungen, Import-Schlüssel, Archivierungszeitpunkt, Vergleichsschlüssel).“
- **FR-11:** „Events gibt es nur als Listen (FR-8, FR-9, FR-12), dazu die Liste aller Event-Typen. Es gibt keinen Einzelabruf und keine eigene Ortsliste; Abnehmer speichern keine Kennungen. Ob ein Event archiviert ist, zeigt `archived`. Maßgeblich ist die Vorbei-Regel: … Außerhalb des Umfangs: Schreibzugriff, Einzelabruf, Ortsliste, Volltextsuche, Umkreissuche, Paginierung.“
- **FR-16:** „Jedes Event bringt seinen Ort mit, mindestens mit Namen; ein Ort mit vorhandenem Namen wird dem vorhandenen Ort zugeordnet. Die Datei enthält keine internen Kennungen.“ (ersetzt „Ein Event verweist auf einen vorhandenen Ort oder bringt einen mit. Ein mitgebrachter Ort mit vorhandenem Namen wird dem vorhandenen Ort zugeordnet.“)
- **AD-7:** „Lesen über die Kern-Abfragen `ListActiveEvents`, `ListArchivedEvents` und `ListEventTypes`, nur Listen. `GetEvent` und `ListLocations` nur für den Admin. Sortierung: … bei Gleichstand nach der internen `id`. Orte nach Namen im Kern (ENT-8).“
- **AD-11:** „IDs sind UUIDv7 per `DEFAULT uuidv7()` (PostgreSQL 18), rein intern; sie erscheinen weder in der API noch im Import-Schema. …“
- **AD-14:** „Die Schreibform `EventInput` hat die Felder `title`, `type`, `location` (mitgebrachter Ort, `name` Pflicht, Rest optional), `startDate`, `startTime`, `endDate`, `endTime`, `allDay`, `source` (Objekt, ENT-10), `note`, `timetable` und `importKey`. Die Leseform `Event` ist ein eigenes Schema mit denselben Feldnamen (ohne `importKey`, `location` vollständig), plus `startPrecision`, `endPrecision`, `effectiveStart`, `effectiveEnd` (lokaler Offset Europe/Berlin) und `archived`. Keine Form nach außen enthält interne Werte. `Canonicalize` bringt Eingaben in eine kanonische Form.“
- **Ressourcen:** „`GET /v1/events` (`from`, `to`, `type`), `GET /v1/event-types`, `GET /v1/archive/events`.“
- **ENT-18:** „Datenformen: Der mitgebrachte Ort heißt `location` wie in der Leseform; `locationId` und `importLocation` gibt es nicht (Sprint Change Proposal 2026-10-04). `Event` und `EventInput` sind eigene Schemas mit denselben Feldnamen. …“
- **Testsammlung:** „Orte werden mitgebracht (es gibt keine Referenz per Kennung), damit die Datei in jeder Umgebung importierbar ist.“
- **FR Coverage Map:** `SM-3: Epic 2 (Story 2.3)` statt `(Story 2.2)`.

### 4.4 Epics: Story 2.2 entfällt, Story 2.3 übernimmt die Leseform

**Story 2.2** wird ersetzt durch:

```
### Story 2.2: entfällt

Gestrichen mit dem Sprint Change Proposal vom 2026-10-04: Die öffentliche API liefert Events nur als Listen und gibt keine Kennungen aus (FR-11). Die Leseform `Event`, die Schreibform `EventInput` und FR-10 setzt Story 2.3 um.
```

Die Nummern 2.3 bis 2.6 bleiben, 2.2 wird nicht neu vergeben.

**Story 2.3, Deckt ab**

```
OLD: **Deckt ab:** FR-8, FR-9, KON-5, KON-6, AD-16, ENT-4
NEW: **Deckt ab:** FR-8, FR-9, FR-10, FR-11, KON-4, KON-5, KON-6, AD-7, AD-14, AD-16, SM-3, ENT-4, ENT-10, ENT-18
```

**Story 2.3, neue Szenarien vor dem bisherigen ersten**

```
**Angenommen** die Spec ist um das Schema `Event` erweitert
**Wenn** ein Abnehmer `GET /v1/events` aufruft
**Dann** hat jedes Event der Liste die Leseform nach AD-14: `title`, `type`, `location` (vollständig: `name`, `address` als Objekt aus `street`, `postalCode` und `city`, `latitude`, `longitude`, `precision`, `note`), `startDate`, `startTime`, `endDate`, `endTime`, `allDay`, `startPrecision`, `endPrecision`, `source` (Objekt aus `description` und `url`, ENT-10), `note`, `timetable` (Einträge aus `description`, `date`, `startTime`, `endTime`), `effectiveStart`, `effectiveEnd`, `archived`
**Und** Uhrzeiten sind `HH:MM` oder `null`, Datumswerte `YYYY-MM-DD`, `effective*` ISO 8601 mit Offset
**Und** der Ablaufplan ist chronologisch sortiert
**Und** kein Event, kein Ort und kein Programmpunkt enthält eine Kennung, einen `importKey`, `archivedAt`, `title_key` oder `name_key`

**Angenommen** zwei Events am selben Ort
**Wenn** beide in der Liste stehen
**Dann** tragen sie denselben Ortsnamen und identische Koordinaten
**Und** nach einer Änderung der Koordinaten im Admin liefern beide die neuen Koordinaten (FR-7)
```

**Story 2.3, bisheriges erstes Szenario**

```
OLD: **Angenommen** die Spec ist um `GET /v1/events` mit den Parametern …
NEW: **Angenommen** die Spec enthält `GET /v1/events` mit den Parametern …
```

**Story 2.3, Außerdem gilt (neu)**

```
- `components/schemas` enthält zusätzlich die Schreibform `EventInput` nach AD-14 (mit `location` als mitgebrachtem Ort und `importKey`, ENT-18), mit denselben Feldnamen und Formaten wie `Event`, damit das Import-Schema in Epic 3 sie per `$ref` einbinden kann (AD-9). Sie enthält keine Kennungen.
- Die Spec beschreibt bei `location.name`, dass der Name einen Ort eindeutig kennzeichnet und zum Gruppieren dient, und im `info`-Abschnitt, dass die API nur Listen liefert und keine Kennungen ausgibt.
- Der Handler bildet nur Kern-Objekte auf die erzeugten Typen ab und leitet nichts selbst ab (AD-7). Ein Test belegt, dass die JSON-Antwort keine Felder außer denen der Leseform enthält.
```

### 4.5 Epics: Stories 3.1, 3.2 und 3.4

**Story 3.1, Schema**

```
OLD:
**Und** jedes Event hat genau eines von beidem (`oneOf`): `locationId` für einen vorhandenen Ort oder `importLocation` für einen mitgebrachten Ort (ENT-18)
**Und** ein mitgebrachter Ort hat immer `name`; `address`, `latitude`, `longitude` und `precision` sind im Schema optional, weil ein Ort mit vorhandenem Namen sie nicht braucht; `note` ist optional

NEW:
**Und** jedes Event bringt seinen Ort als `location` mit (ENT-18); eine Kennung (`locationId`) gibt es nicht
**Und** `location` hat immer `name`; `address`, `latitude`, `longitude` und `precision` sind im Schema optional, weil ein Ort mit vorhandenem Namen sie nicht braucht; `note` ist optional
**Und** das Schema enthält keine Kennungen
```

**Story 3.1, fehlerhafte Einträge**

```
OLD: … unbekannter Typ, `locationId` und mitgebrachter Ort zugleich oder keines von beiden, eine syntaktisch ungültige oder unbekannte `locationId`, ein mitgebrachter neuer Ort ohne Adresse, …
NEW: … unbekannter Typ, ein Event ohne `location` oder ohne Ortsnamen, ein mitgebrachter neuer Ort ohne Adresse, …
```

**Story 3.2, Konflikte innerhalb der Datei**

```
OLD: **Und** untereinander gleiche Einträge werden zu `duplicateSuspect`; bei einem neuen Ort zählt dafür sein `NormalizeKey`, weil er noch keine Kennung hat
NEW: **Und** untereinander gleiche Einträge werden zu `duplicateSuspect`; als gleicher Ort zählt derselbe `NormalizeKey` des Ortsnamens
```

**Story 3.4**

```
OLD: **Und** Orte werden mitgebracht, nicht per `locationId` referenziert; gleiche Orte sind zusammengeführt …
NEW: **Und** jedes Event bringt seinen Ort als `location` mit; gleiche Orte sind zusammengeführt …
```

Die Stories 2.4, 2.5, 2.6 und 3.3 bleiben unverändert. Die Verweise auf `id` dort betreffen nur die interne Sortierung oder das Admin-Formular.

### 4.6 README

- Konventionen, Zeile „Namen“: Beispiel `locationId` durch `effectiveEnd` ersetzen.
- Neue Zeile „Keine Kennungen“: „Events gibt es nur als gefilterte Listen, nie einzeln. Die API gibt keine internen IDs aus; Abnehmer müssen sich nichts merken. Ein Ort ist an seinem eindeutigen Namen erkennbar.“
- Ressourcen: `GET /v1/events/{id}` und `GET /v1/locations` streichen; bei `GET /v1/events` ergänzen: „Events am selben Ort tragen denselben Ortsnamen und dieselben Koordinaten und lassen sich so auf der Karte gruppieren.“
- Beispiel-JSON: `"id"` bei Event und Ort entfernen.

### 4.7 `epic-2-context.md`

- Offener Punkt und „Vor 2.2“ ersetzen durch: „Erledigt mit Sprint Change Proposal 2026-10-04: keine Kennungen nach außen, nur Listen.“
- Story-Liste: „Story 2.2: entfällt“.
- Ressourcen, Antwortinhalt, Schemas und Kern-Abfragen nach 4.2 bis 4.4; Absatz „Einzelabruf“ entfällt.
- Abhängigkeiten: „2.3 liefert `Event` und `EventInput`; 2.4 nutzt die Leseform …“

### 4.8 `sprint-status.yaml`

Zeile `2-2-einzelnes-event-und-orte-abrufen: backlog` entfernen.

## 5. Übergabe an die Umsetzung

**Umfang:** Moderate. Der Backlog ändert sich (eine Story entfällt, eine wächst), aber nur innerhalb von Epic 2 und mit kleinen Folgen in Epic 3. Kein Replan.

**Zuständig:**

- **Developer-Agent (jetzt):** setzt 4.1 bis 4.8 auf einem Branch `chore/correct-course-keine-kennungen` um, als ein PR auf `main`. Es ändert sich kein Code.
- **Andreas:** prüft und merged den PR.
- **Developer-Agent (danach):** setzt Story 2.3 in der neuen Fassung um.

**Erfolgskriterien:**

- In PRD, Spine, Epics, Epic-Kontext und README kommen `GET /v1/events/{id}`, `GET /v1/locations`, `locationId`, `importLocation` und eine öffentliche `id` nicht mehr vor (außer als „verworfen“).
- Story 2.3 nennt ausdrücklich, dass die Antwort keine internen Werte enthält, und verlangt einen Test dafür.
- Sprint-Status und Epics stimmen überein.
