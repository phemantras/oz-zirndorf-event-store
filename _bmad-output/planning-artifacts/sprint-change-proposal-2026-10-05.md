---
title: "Sprint Change Proposal: Offene Lücken aus der Retro von Epic 1 zuordnen (A5)"
status: approved
created: 2026-10-05
author: Andreas (mit Developer-Agent, bmad-correct-course)
scope: moderate
---

# Sprint Change Proposal: Offene Lücken aus der Retro von Epic 1 zuordnen (A5)

## 1. Problem

**Problem:** Die Retrospektive von Epic 1 (`epic-1-retro-2026-10-03.md`) hat sechs Lücken zwischen Spine, Stories und Code gefunden, die keiner Story zugeordnet sind. Action Item A5 verlangt, sie per Correct Course einer Story zuzuordnen oder zu entscheiden.

| Befund | Inhalt | Beleg |
| --- | --- | --- |
| S2 | `RecomputeDerived` berechnet `name_key` der Orte nicht neu, obwohl AD-16 das verlangt. | `internal/core/event_service.go:336`; Spine AD-16; `deferred-work.md` |
| R5 | `RecomputeDerived` prüft nach einer geänderten Periode nicht, ob der Ablaufplan noch hineinpasst (AD-15). | `internal/core/event_service.go:361` |
| B5 | Jedes Speichern erzeugt neue IDs für alle Programmpunkte; offen war, ob sie öffentlich stabil sein müssen. | `internal/adapter/postgres/events.go:123-146` |
| G1 | `SaveLocation` hat keinen transaktionsgebundenen Kern und läuft nicht über `TxRunner`, wie AD-6 es verlangt; `CommitImport` braucht ihn. | `internal/core/location_service.go:81` |
| B3 | Der Kern begrenzt weder Textlängen noch die Zahl der Programmpunkte; nur das Admin-Formular ist auf 64 KiB gedeckelt. | `internal/core/event.go`, `internal/core/timetable.go` |
| R8 | Zwei gleichzeitige Saves mit demselben Duplikatschlüssel sehen beide keinen Kandidaten. | `internal/core/event_service.go:140-144`; `migrations/00006_event_title_key.sql` |

**Wann es aufgefallen ist:** In der Retrospektive von Epic 1 am 2026-10-03, nach Abschluss aller zwölf Stories.

**Kategorie:** Lücken in Architektur und Spec, die bei der Umsetzung entdeckt wurden.

## 2. Auswirkungen

### Epics

- **Epic 1:** abgeschlossen, bleibt es. Die Lücken werden in späteren Stories geschlossen.
- **Epic 2:** Story 2.5 wächst um die vollständige Neuberechnung beim Start (S2, R5). B5 ist mit dem Sprint Change Proposal vom 2026-10-04 bereits erledigt (Story 2.2 entfällt, keine Programmpunkt-Kennungen nach außen); es bleibt nur eine Klarstellung in AD-15.
- **Epic 3:** Story 3.1 bekommt die Längengrenzen (B3), Story 3.3 den `SaveLocation`-Kern und die Schreibsperre (G1, R8).
- Kein Epic kommt dazu oder entfällt, die Reihenfolge bleibt.

### Artefakte

| Artefakt | Auswirkung |
| --- | --- |
| PRD | keine |
| Architektur-Spine | AD-6 (Advisory-Sperre in `TxRunner`), AD-9 (Längengrenzen nur in der Spec), AD-15 (Programmpunkt-IDs intern und nicht stabil), AD-16 (Neuberechnung: Ablaufplan-Grenze, `name_key`-Kollisionen, „prüfen“ für Orte), `updated` |
| UX | Es gibt kein UX-Dokument. |
| `epics.md` | Inventory: ENT-5, ENT-15, AD-15-Zeile, neue ENT-24; Stories 2.5, 3.1, 3.3 |
| `epic-2-context.md` | Story 2.5: Titel, Neuberechnung, Abhängigkeiten |
| `deferred-work.md` | Ziel des `name_key`-Eintrags: Story 2.5 |
| `sprint-status.yaml` | A5 auf `done` |

### Technik

- Keine Migration aus diesem Proposal. `name_key` existiert bereits.
- `TxRunner` (Postgres-Adapter) nimmt künftig `pg_advisory_xact_lock` mit fester Kennung. Alle Schreib-Transaktionen laufen nacheinander; bei einem Admin ohne spürbare Kosten.
- `openapi.yaml` bekommt `maxLength`/`maxItems` in `EventInput` und den eingebundenen Schemas. Für v1 zulässig, weil die Schreibform erst mit Epic 3 benutzt wird (NFR-2).
- Die Formulargrenze des Event-Formulars steigt von 64 KiB auf 256 KiB.

## 3. Empfohlenes Vorgehen

**Direkte Anpassung** der Planungsartefakte, ohne Rollback und ohne Änderung am MVP-Ziel.

- **Aufwand:** gering bis mittel. Nur Dokumente in diesem PR; Story 2.5 wächst um einen Kernteil, Stories 3.1 und 3.3 um je zwei Szenarien.
- **Risiko:** gering. Die Advisory-Sperre ändert das Verhalten nur bei gleichzeitigen Schreibzugriffen. Die Längengrenzen sind großzügig gewählt.
- **Zeitplan:** keine Verzögerung. Story 2.5 ist die nächste Story, die Lücke `name_key` ist geschlossen, bevor der Import Orte darüber zuordnet.
- **Abwägungen:**
  - R5 braucht keine neue Regel: Ein unpassender Ablaufplan zählt als Regelfehler nach ENT-5. Das Event behält seine gespeicherten, zum Plan passenden Werte und wird mit „prüfen“ markiert.
  - R8: verworfen wurden „Risiko hinnehmen“ (ein Doppelklick beim Import könnte Einträge ohne `importKey` verdoppeln) und `SERIALIZABLE` (braucht eine Wiederholungsschleife).
  - `name_key`-Kollisionen: verworfen wurde „nur loggen“, weil der Konflikt dann nur im Log auffiele.
  - `RecomputeDerived` schreibt weiter ohne `TxRunner`, weil es vor dem HTTP-Server läuft. Das wird als einzige Ausnahme von der Hülle nach AD-6 in Story 2.5 vermerkt.

## 4. Die Änderungen im Einzelnen

### 4.1 Architecture Spine (`ARCHITECTURE-SPINE.md`)

**AD-6, Rule: nach dem Satz zu `CommitImport` angehängt (G1, R8)**

```
NEW:
`TxRunner` nimmt zu Beginn jeder Transaktion eine transaktionsgebundene Sperre
(`pg_advisory_xact_lock` mit einer festen Kennung), sodass Schreib-Transaktionen
nacheinander laufen: Duplikat- und Namensprüfung sehen immer den Stand der vorigen
Transaktion. Bei einem Admin kostet das nichts; es ist keine Fachlogik in SQL (AD-2),
sondern Nebenläufigkeitsschutz.
```

**AD-9, Rule: am Ende angehängt (B3)**

```
NEW:
Gleiches gilt für Längengrenzen (`maxLength`, `maxItems`): Sie stehen nur in der Spec,
der Kern spiegelt sie, und derselbe Test prüft die Übereinstimmung.
```

**AD-15, Rule: am Ende angehängt (B5)**

```
NEW:
Programmpunkte haben nur interne Kennungen. Weil jedes Speichern die ganze Liste ersetzt,
sind diese Kennungen nicht stabil, und nichts außerhalb des Kerns darf sie speichern oder
sich auf sie beziehen (AD-11, AD-14).
```

**AD-16, Punkt „Neuberechnung“ (S2, R5)**

```
OLD:
… Schlägt die Neuberechnung für ein Event fehl, behält es seine gespeicherten Werte, der
Fehler wird mit der Event-Kennung geloggt, und das Programm startet trotzdem. Der Kern
merkt sich die betroffenen Event-IDs **nur im Speicher**, geschützt per Mutex. Nach
erfolgreichem Commit von `SaveEvent`, `CommitImport` oder `DeleteEvent` wird die ID
entfernt. Die Admin-Liste liest die Menge über eine Kern-Abfrage und markiert diese
Events mit „prüfen“. Nach einem Neustart baut die Neuberechnung die Menge neu auf.

NEW:
… Als fehlgeschlagen gilt ein Event, wenn die aktuellen Regeln seine Zeitangaben ablehnen
oder sein Ablaufplan nicht mehr in den neu berechneten Zeitraum passt (AD-15). Es behält
dann seine gespeicherten Werte, der Fehler wird mit der Event-Kennung geloggt, und das
Programm startet trotzdem. Für Orte gilt dasselbe: Ergäben zwei Orte denselben neuen
`name_key`, behalten beide ihren gespeicherten Schlüssel, der Fehler wird mit den
Ort-Kennungen geloggt. Der Kern merkt sich die betroffenen Event- und Ort-IDs **nur im
Speicher**, geschützt per Mutex. Nach erfolgreichem Commit von `SaveEvent`,
`CommitImport` oder `DeleteEvent` (Event) bzw. `SaveLocation` oder `DeleteLocation` (Ort)
wird die ID entfernt. Event- und Ortsliste im Admin lesen die Menge über eine
Kern-Abfrage und markieren die Einträge mit „prüfen“. Nach einem Neustart baut die
Neuberechnung die Menge neu auf.
```

**Kopf:** `updated: '2026-10-05'`

### 4.2 Epics: Requirements Inventory (`epics.md`)

- **AD-9-Zeile:** „… Ein CI-Test prüft die Übereinstimmung, auch für Längengrenzen (ENT-24). …“
- **AD-15-Zeile:** angehängt: „Programmpunkt-IDs sind intern und nicht stabil (jedes Speichern ersetzt die Liste).“
- **ENT-5:** „Neuberechnung beim Start: Schlägt sie für ein Event fehl (Zeitangaben abgelehnt oder Ablaufplan außerhalb des neuen Zeitraums), behält es seine gespeicherten Werte. … Für Orte mit kollidierendem neuen `name_key` gilt dasselbe; die Markierung „prüfen“ erscheint in der Ortsliste und verschwindet nach `SaveLocation` oder `DeleteLocation`.“
- **ENT-15:** angehängt: „`TxRunner` serialisiert alle Schreib-Transaktionen über eine transaktionsgebundene Advisory-Sperre (AD-6).“
- **Neu ENT-24 (→ AD-9, AD-11) Grenzen:**

```
ENT-24 (→ AD-9, AD-11) Grenzen: Der Kern begrenzt Texte nach der NFC-Normalisierung in
Zeichen (Unicode-Codepoints), für Admin und Import gleich: `title`, `location.name` und
`street` höchstens 200; `city` höchstens 100; `source.description` und die Beschreibung
eines Programmpunkts höchstens 500; `note` (Event, Ort) und `source.url` höchstens 2000;
`importKey` höchstens 200; höchstens 100 Programmpunkte je Event. Die Grenzen stehen als
`maxLength`/`maxItems` in `EventInput` und den eingebundenen Schemas von `openapi.yaml`;
die Kern-Konstanten spiegeln sie, und der Test aus AD-9 prüft die Übereinstimmung.
```

- Der Einleitungssatz zu den Ergänzenden Festlegungen nennt künftig ENT-1 bis ENT-24; ENT-24 kam mit dem Sprint Change Proposal vom 2026-10-05 dazu.

### 4.3 Epics: Story 2.5 (S2, R5)

**Titel und User Story**

```
OLD: ### Story 2.5: Tägliche Bereinigung
NEW: ### Story 2.5: Tägliche Bereinigung und vollständige Neuberechnung beim Start

Als Admin,
möchte ich, dass vergangene Events täglich als archiviert markiert werden, ohne dass sich
die API-Antworten ändern, und dass der Start alle abgeleiteten Werte nachzieht,
damit ich den Bestand auswerten kann, nichts verloren geht und Regeländerungen überall
greifen.
```

**Deckt ab**

```
OLD: **Deckt ab:** FR-13, AD-5, AD-13, ENT-17
NEW: **Deckt ab:** FR-13, AD-5, AD-13, AD-15, AD-16, ENT-5, ENT-17
```

**Neue Szenarien nach dem letzten bisherigen**

```
**Angenommen** ein Ort, dessen gespeicherter `name_key` nicht `NormalizeKey(name)` entspricht
**Wenn** das Programm startet
**Dann** berechnet `RecomputeDerived` den Schlüssel neu und speichert ihn, bevor
`MarkArchived` und der HTTP-Server starten

**Angenommen** zwei Orte, deren neu berechnete `name_key` gleich wären
**Wenn** das Programm startet
**Dann** behalten beide ihren gespeicherten Schlüssel, der Fehler wird mit beiden
Ort-Kennungen geloggt, und das Programm startet trotzdem
**Und** die Ortsliste im Admin markiert beide mit „prüfen“, bis einer von ihnen
erfolgreich gespeichert oder gelöscht ist
**Und** der Kern erkennt solche Kollisionen vorab über alle Orte; ein trotzdem
gemeldetes `ErrConflict` der Datenbank wird genauso behandelt

**Angenommen** ein Event mit Ablaufplan, dessen neu berechneter Zeitraum (z. B. nach
einer Regel- oder tzdata-Änderung) einen Programmpunkt ausschließt (AD-15, ENT-20)
**Wenn** das Programm startet
**Dann** behält das Event seine gespeicherten Werte, der Fehler nennt die Event-Kennung
und den betroffenen Punkt, und die Event-Liste markiert es mit „prüfen“ (ENT-5)
```

**Außerdem gilt (neu)**

```
- `RecomputeDerived` läuft vor dem HTTP-Server und schreibt deshalb ohne `TxRunner`;
  das ist die einzige Ausnahme von der Hülle nach AD-6.
- Die Fälle sind mit Kern-Tests ohne Datenbank abgedeckt (fester Bestand, Fake-Repos);
  ein Postgres-Test belegt die Neuberechnung von `name_key`.
```

### 4.4 Epics: Story 3.1 (B3)

**Deckt ab**

```
OLD: **Deckt ab:** FR-16, AD-9, ENT-13, ENT-18
NEW: **Deckt ab:** FR-16, AD-9, ENT-13, ENT-18, ENT-24
```

**Neue Szenarien nach „einzelne Einträge fehlerhaft“**

```
**Angenommen** ein Eintrag mit einem Text über der Grenze aus ENT-24 oder mehr als 100
Programmpunkten
**Wenn** ich die Datei hochlade
**Dann** ist genau dieser Eintrag fehlerhaft, und der Grund nennt Feld und Grenze

**Angenommen** das Event- oder Ortsformular im Admin
**Wenn** ich einen Text über der Grenze speichere
**Dann** lehnt der Kern mit `ErrValidation` ab, und das Formular zeigt die deutsche
Meldung am Feld, ohne die Eingaben zu verlieren
```

**Außerdem gilt (neu)**

```
- Die Formulargrenze des Event-Formulars (`maxEventFormBytes`, heute 64 KiB) wird so
  angehoben, dass ein Event mit allen Feldern an der Grenze aus ENT-24 hineinpasst
  (256 KiB); maßgeblich sind die Grenzen des Kerns.
- Das Hinzufügen von `maxLength`/`maxItems` zu `EventInput` ist in v1 zulässig, weil die
  Schreibform erst mit Epic 3 benutzt wird (NFR-2).
```

### 4.5 Epics: Story 3.3 (G1, R8)

**Deckt ab**

```
OLD: **Deckt ab:** FR-18, AD-10, ENT-11, ENT-12, ENT-15
NEW: **Deckt ab:** FR-18, AD-6, AD-10, ENT-11, ENT-12, ENT-15
```

**Neue Szenarien vor „Angenommen der Import ist abgeschlossen“**

```
**Angenommen** `CommitImport` legt neue Orte an
**Wenn** übernommen wird
**Dann** nutzt es den transaktionsgebundenen Kern von `SaveLocation` in derselben
Transaktion wie die Events (AD-6)
**Und** Ortsverwaltung und „Neuer Ort“ im Event-Formular nutzen dieselbe Hülle über
`TxRunner`, mit unverändertem Verhalten aus Story 1.4 und 1.8

**Angenommen** zwei Schreib-Transaktionen laufen gleichzeitig, z. B. ein Doppelklick
auf „Import übernehmen“ oder ein Import neben einem Speichern im Event-Formular
**Wenn** beide dasselbe Event oder denselben neuen Ort betreffen
**Dann** läuft die zweite erst nach dem Commit der ersten, weil `TxRunner` die Sperre
aus AD-6 nimmt
**Und** sie klassifiziert neu: Die Einträge des zweiten Imports sind `stale` oder
`unchanged`, und es entsteht kein Duplikat
```

**Außerdem gilt (neu)**

```
- Nahtstellen: `SaveLocation` (Story 1.4, 1.8) und alle Hüllen über `TxRunner`
  (`SaveEvent`, `DeleteEvent`, `DeleteLocation`, `MarkArchived`). Der Kommentar zur
  nicht atomaren Namensprüfung in `location_service.go` entfällt.
- Ein Postgres-Test belegt mit zwei parallelen Transaktionen, dass die zweite auf die
  erste wartet und deren Ergebnis sieht.
```

### 4.6 `epic-2-context.md`

- Story-Liste: „Story 2.5: Tägliche Bereinigung und vollständige Neuberechnung beim Start“.
- Neuer Punkt nach „Bereinigung“: „**Neuberechnung (2.5):** `RecomputeDerived` zieht auch `name_key` der Orte nach. Kollisionen und Ablaufpläne außerhalb des neuen Zeitraums behandelt es nach ENT-5 (gespeicherte Werte bleiben, Log, „prüfen“ in Event- bzw. Ortsliste, Menge nur im Speicher). Läuft vor dem HTTP-Server ohne `TxRunner`.“
- Abhängigkeiten: „2.5 erweitert `RecomputeDerived` und die Ortsliste im Admin (Markierung „prüfen“).“

### 4.7 `deferred-work.md`

Eintrag „`RecomputeDerived` auch `name_key` der Orte neu berechnen lassen“: `target: Story 2.5 (Sprint Change Proposal 2026-10-05)`, `status` bleibt `open`.

### 4.8 `sprint-status.yaml`

Action Item `epic-1-retro-item-5-a5-…`: `status: done`. Der Schlüssel `2-5-tägliche-bereinigung` bleibt trotz des längeren Titels.

Die Retro-Datei bleibt unverändert, sie ist ein Protokoll.

## 5. Übergabe an die Umsetzung

**Umfang:** Moderate. Der Backlog ändert sich innerhalb von Epic 2 und Epic 3 (drei Stories wachsen), der Spine bekommt Ergänzungen in vier ADs. Kein Replan, kein neues Epic.

**Zuständig:**

- **Developer-Agent (jetzt):** setzt 4.1 bis 4.8 auf einem Branch `chore/correct-course-a5` um, als ein PR auf `main`. Es ändert sich kein Code.
- **Andreas:** prüft und merged den PR.
- **Developer-Agent (danach):** setzt Story 2.5 in der neuen Fassung um; 3.1 und 3.3 folgen mit Epic 3.

**Erfolgskriterien:**

- Jeder der Befunde S2, R5, B5, G1, B3 und R8 ist entweder einer Story mit Akzeptanzkriterien zugeordnet oder im Spine als erledigt festgehalten.
- Spine (AD-6, AD-9, AD-15, AD-16), Inventory (ENT-5, ENT-15, ENT-24) und Stories (2.5, 3.1, 3.3) widersprechen einander nicht.
- `deferred-work.md` hat für den `name_key`-Eintrag ein Story-Ziel; A5 steht in `sprint-status.yaml` auf `done`.
