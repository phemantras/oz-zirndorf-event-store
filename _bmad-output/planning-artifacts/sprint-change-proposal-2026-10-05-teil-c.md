---
title: "Sprint Change Proposal: Abgleich vor Epic 3 (B4) und Story 2.7 „Events mit prüfen öffentlich ausblenden“ (B5)"
status: approved
created: 2026-10-05
author: Andreas (mit Developer-Agent, bmad-correct-course)
scope: moderate
---

# Sprint Change Proposal: Abgleich vor Epic 3 und Story 2.7

## 1. Problem

**Auslöser:** Retrospektive Epic 2 (`_bmad-output/implementation-artifacts/epic-2-retro-2026-10-05.md`), Maßnahmen B4 und B5.

**Problem:**

1. **B5 / R1:** Ein Event, dessen Neuberechnung beim Start scheitert (Markierung „prüfen“), erscheint in `/v1/events` und `/v1/archive/events` ohne Kennzeichen mit seinem **alten** `effective*`-Zeitraum. Auch `archived` richtet sich nach diesem Zeitraum. Die Markierung wirkt bisher nur im Admin. Andreas hat am 2026-10-05 entschieden, solche Events auszublenden: Sie erscheinen erst wieder, wenn sie erfolgreich gespeichert sind. Das widerspricht FR-12 („jedes Event in genau einem der beiden Zugriffe“) und AD-5 („keine Lücke“), deshalb braucht es eine neue Story und Änderungen an PRD und Spine.
2. **B4 / S3:** `github.com/oapi-codegen/runtime` v1.7.0 ist seit Story 2.3 eine Modulabhängigkeit, steht aber nicht in der Stack-Tabelle. AGENTS.md verlangt Versionen aus dieser Tabelle.
3. **B4 / S6:** `archived_at` bedeutet so, wie es gebaut ist, „zuletzt markiert“, nicht „archiviert seit“. `UpdateEvent` leert es bei jedem Speichern, `UpdateEventDerived` bei jeder geänderten Neuberechnung, und der nächste Lauf von `MarkArchived` setzt einen neuen Zeitpunkt. Der Spine sagt nur „`SaveEvent` leert `archivedAt`, wenn das Event danach wieder aktiv ist“.
4. **B4 / S4:** AD-9 soll unterschiedliche **Feldnamen** zwischen Spec und Kern verhindern. Der CI-Test prüft aber nur Enum-Codes und Längengrenzen. Die Kern-Konstanten `EventField*` und `FilterField*` prüft kein Test. Relevant wird das mit Story 3.1, wenn das Import-Schema `EventInput` per `$ref` einbindet und der Kern Fehler je Feld meldet.
5. **B4 / R3 (mit R9 aus Epic 1):** Überlappen sich beim Deploy alte und neue Instanz, kann `RecomputeDerived` der neuen Instanz Werte überschreiben, die die alte gerade gespeichert hat (`UpdateEventDerived … WHERE id = $1`, ohne Vergleich der gelesenen Werte). Dasselbe gilt für `UpdateNameKey`. Die Sperre umfasst heute nur `provider.Up`. Laut Retro soll das zusammen mit der Advisory-Sperre in Story 3.3 entschieden werden.

**Kategorie:** Neue Anforderung des Stakeholders (1). Die übrigen Punkte (2–5) gleichen Spec und Code ab.

**Belege:** Retro Epic 2, Befunde R1, R3, S3, S4, S6 und Abschnitt „Offene Fragen“. Verhaltensprüfung „Herbstfest“: Das Event liegt am 10. und 11.10., der gespeicherte Zeitraum endet aber am 16.10. Weitere Belege: `internal/core/event_query.go:154-177`, `internal/core/event_service.go:372-417`, `internal/adapter/postgres/queries/events.sql:32-51`, `go.mod:7` und `internal/adapter/publicapi/v1/enum_test.go`. In `api/v1/openapi.yaml:262` steht „every event is in exactly one of `/v1/events` and …“.

## 2. Auswirkungen

### Checkliste (Kurzfassung)

| # | Punkt | Status | Ergebnis |
| --- | --- | --- | --- |
| 1.1–1.3 | Auslöser, Problem, Belege | [x] | Retro Epic 2, B4/B5 |
| 2.1 | Aktuelles Epic (2) | [!] | Neue Story 2.7. `epic-2` geht zurück auf `in-progress`. |
| 2.2 | Änderungen auf Epic-Ebene | [x] | Kein neues Epic. Epic 2 bekommt Story 2.7, Story 2.4 bekommt einen Verweis. |
| 2.3 | Spätere Epics | [!] | Epic 3: neues AC in 3.1 (AD-9-Feldnamen) und in 3.3 (Neuberechnung unter der Sperre) |
| 2.4 | Epics ungültig oder neu nötig | [N/A] | nein |
| 2.5 | Reihenfolge | [x] | 2.7 vor Epic 3; 2.7 ist unabhängig von B1 |
| 3.1 | PRD | [!] | FR-12, Ausnahme für „prüfen“ |
| 3.2 | Architektur | [!] | AD-5, AD-9, AD-16, Stack-Tabelle |
| 3.3 | UX | [N/A] | Es gibt kein UX-Dokument. Die Admin-Anzeige „prüfen“ bleibt unverändert. |
| 3.4 | Weitere Artefakte | [!] | `openapi.yaml` (in Story 2.7), `sprint-status.yaml` |
| 4.1 | Direkte Anpassung | machbar | Aufwand gering, Risiko gering |
| 4.2 | Rollback | nicht machbar | Es gibt nichts zurückzunehmen. |
| 4.3 | MVP prüfen | [N/A] | Das MVP-Ziel bleibt. |

### Epics

- **Epic 2:** Neue Story 2.7. Sie ändert nur die Auswahl in den beiden öffentlichen Listen. Es gibt kein neues Feld und keine Vertragsänderung (NFR-2): Die Leseform bleibt gleich, nur die Beschreibung im `info`-Abschnitt kommt dazu. Der Partitionstest aus Story 2.4 bekommt die Ausnahme.
- **Epic 3:** Story 3.1 bekommt ein AC zum AD-9-Abgleich der Feldnamen, Story 3.3 eines zur Neuberechnung unter der Sperre aus AD-6. Die übrigen Stories bleiben unverändert.

### Artefakte

| Artefakt | Auswirkung |
| --- | --- |
| PRD | FR-12: Ausnahme für Events mit „prüfen“ |
| Architektur-Spine | AD-5 (Ausnahme, Bedeutung von `archivedAt`), AD-9 (Feldnamen), AD-16 (Ausblenden, Neuberechnung unter Sperre), Stack (`oapi-codegen/runtime`) |
| `epics.md` | neue Story 2.7, Verweis in 2.4, ein AC in 3.1, ein AC in 3.3, Wortlaut von AD-5/AD-16 unter „Additional Requirements“, ENT-5 |
| `sprint-status.yaml` | `epic-2: in-progress`, neue Zeile `2-7-…: backlog`, B4 nach dem Merge auf `done` |
| `api/v1/openapi.yaml` | Beschreibung im `info`-Abschnitt; Umsetzung in Story 2.7, nicht in diesem Proposal |
| UX | keine |
| `epic-2-context.md` | keine. Story 2.7 steht vollständig in `epics.md`. |

### Technik

Das Proposal ändert keinen Code. Nahtstellen für Story 2.7:

- Die Menge „prüfen“ liegt bereits am `EventService` (`needsReview`, `event_service.go:450`). Dieselbe Methode liefert `ListActiveEvents` und `ListArchivedEvents` über `listMatchingEvents`. Die Auswahl passt deshalb in den Kern, ohne neuen Port, ohne SQL und ohne neue Spalte.
- `RecomputeDerived` läuft vor dem HTTP-Server (AD-16). Deshalb wird nie ein markiertes Event ausgeliefert, bevor die Menge aufgebaut ist.
- Folgende Tests sind betroffen: der Partitionstest im Kern (2.4), der Vergleich über die API in `cmd/eventstore` sowie die Tests aus 2.5b zum Leeren der Menge nach dem Commit.

## 3. Empfohlenes Vorgehen

**Direkte Anpassung:** eine neue Story in Epic 2, Ergänzungen in PRD, Spine und `epics.md`, zwei zusätzliche ACs in Epic 3.

- **Aufwand:** gering. Story 2.7 ist eine Filterbedingung im Kern mit Tests und einem Absatz in der Spec. Die übrigen Punkte sind Text.
- **Risiko:** gering. Die Antwortform ändert sich nicht. Nur Events, die ohnehin falsch ausgeliefert würden, fallen weg.
- **Zeitplan:** Story 2.7 kommt vor Story 3.1 dazu.
- **Abwägung:**
  - Andreas hat am 2026-10-05 „Kennzeichnen“ (neues Feld, Vertragsänderung) und „Ausliefern und dokumentieren“ verworfen.
  - Story 3.0 statt 2.7 ist verworfen, weil die Story fachlich zur Lese-API gehört (Entscheidung Andreas, 2026-10-05).
  - Für R3 empfiehlt das Proposal, die Neuberechnung in **einer** Transaktion unter der Sperre aus AD-6 laufen zu lassen. Eine Prüfung der gelesenen Werte je `UPDATE` (Compare-and-Set) ist verworfen: Sie bräuchte breitere SQL-Bedingungen über alle Rohfelder und verhindert trotzdem nicht, dass die alte Instanz nach der Neuberechnung mit alten Regeln schreibt. Dieses Restrisiko ist hingenommen: Es gibt eine Replika, die Überlappung dauert Sekunden, und der nächste Start rechnet neu.

## 4. Die Änderungen im Einzelnen

### 4.1 PRD, FR-12, Konsequenzen

```
OLD:
- Jedes Event ist zu jedem Zeitpunkt in genau einem der beiden Zugriffe enthalten: in der regulären Abfrage oder im Archiv-Zugriff.

NEW:
- Jedes Event ist zu jedem Zeitpunkt in genau einem der beiden Zugriffe enthalten: in der regulären Abfrage oder im Archiv-Zugriff.
  Ausgenommen sind Events, deren gespeicherte Angaben nach einer Regeländerung nicht neu berechnet werden konnten
  (im Admin mit „prüfen“ markiert). Sie erscheinen in keinem der beiden Zugriffe, bis der Admin sie erfolgreich gespeichert hat.
```

Dazu im Frontmatter `updated: 2026-10-05`.

**Begründung:** Entscheidung B5. Ein Zeitraum, der nach den aktuellen Regeln falsch ist, ist keine ehrliche Angabe (Vision, SM-C1). Fehlen ist besser als falsch.

### 4.2 Spine, AD-5, Regel

```
OLD:
… Der Bereinigungsjob setzt nur `archivedAt` als Markierung für Statistik. Keine Lese-Abfrage und keine Ausgabe wertet `archivedAt` aus. `SaveEvent` leert `archivedAt`, wenn das Event danach wieder aktiv ist.

NEW:
… Der Bereinigungsjob setzt nur `archivedAt` als Markierung für Statistik. Keine Lese-Abfrage und keine Ausgabe wertet `archivedAt` aus.
`archivedAt` bedeutet „zuletzt als archiviert markiert“, nicht „archiviert seit“: Jedes Speichern eines Events (`SaveEvent`, `CommitImport`)
und jede Neuberechnung, die seine abgeleiteten Werte ändert (`RecomputeDerived`), leert es; der nächste Lauf von `MarkArchived` setzt es neu,
wenn das Event dann vorbei ist. Den ersten Archivierungszeitpunkt hält v1 nicht fest.
Einzige Ausnahme von „genau eine der beiden Listen“: Events mit „prüfen“ (AD-16) fehlen in beiden.
```

**Begründung:** S6 hält die gebaute Bedeutung fest, ohne den Code zu ändern. Ob später der erste Archivierungszeitpunkt gebraucht wird, bleibt eine offene Frage der Retro. Dann gäbe es eine eigene Spalte (expand nach AD-17). Die Ausnahme folgt aus 4.1.

### 4.3 Spine, AD-9, Regel

```
OLD:
… Die Konstanten im Kern spiegeln die Codes. Ein Test in CI prüft, dass Kern-Konstanten und Spec übereinstimmen. …

NEW:
… Die Konstanten im Kern spiegeln die Codes und die Feldnamen, die der Kern in Fehlern oder Filtern nennt (`EventField*`, `FilterField*`).
Ein Test in CI prüft, dass Kern-Konstanten und Spec übereinstimmen; Feldnamen, die nur das Admin-Formular kennt (z. B. `locationId`),
sind ausdrücklich ausgenommen. …
```

**Begründung:** S4. `EventFieldLocationID = "locationId"` existiert in der Spec bewusst nicht (ENT-18). Ohne diese Ausnahme schlüge der Test zu Recht fehl.

### 4.4 Spine, AD-16, Punkt „Neuberechnung“

```
OLD:
… Event- und Ortsliste im Admin lesen die Menge über eine Kern-Abfrage und markieren die Einträge mit „prüfen“. Nach einem Neustart baut die Neuberechnung die Menge neu auf.

NEW:
… Event- und Ortsliste im Admin lesen die Menge über eine Kern-Abfrage und markieren die Einträge mit „prüfen“.
`ListActiveEvents` und `ListArchivedEvents` lassen Events aus dieser Menge aus, bis die ID entfernt ist; der Adapter filtert nichts selbst (AD-7).
Nach einem Neustart baut die Neuberechnung die Menge neu auf, bevor der HTTP-Server startet.
Die Neuberechnung (Events und Orte) läuft in einer Transaktion unter der Sperre aus AD-6, damit sie bei einer Deploy-Überlappung
keine gleichzeitig gespeicherten Werte der alten Instanz überschreibt. Schreibt die alte Instanz danach noch mit alten Regeln,
ist das hingenommen; der nächste Start rechnet neu.
```

**Begründung:** B5 (Ausblenden). R3/R9: Die Entscheidung steht im Spine, umgesetzt wird sie in Story 3.3, die die Sperre einführt.

### 4.5 Spine, Stack-Tabelle

```
OLD:
| oapi-codegen | v2.8.0 |

NEW:
| oapi-codegen | v2.8.0 |
| oapi-codegen/runtime (Laufzeit des erzeugten Gerüsts) | v1.7.0 |
```

Dazu im Frontmatter `updated: 2026-10-05` (bleibt).

**Begründung:** S3. Die Version ist seit 2.3 in `go.mod` und wird damit verbindlich.

### 4.6 `epics.md`, neue Story 2.7 (nach Story 2.6)

```
NEW:
### Story 2.7: Events mit „prüfen“ öffentlich ausblenden

Als Karten-App,
möchte ich keine Events bekommen, deren Zeitraum nach einer Regeländerung nicht neu berechnet werden konnte,
damit ich keine Termine mit veraltetem Zeitraum oder falschem `archived` zeige.

**Deckt ab:** FR-8, FR-12, AD-5, AD-7, AD-16, ENT-5

**Acceptance Criteria:**

**Angenommen** ein Event, dessen Neuberechnung beim Start gescheitert ist und das deshalb „prüfen“ trägt (ENT-5)
**Wenn** ein Abnehmer `GET /v1/events` oder `GET /v1/archive/events` aufruft, mit oder ohne Filter
**Dann** fehlt das Event in beiden Antworten
**Und** die Event-Liste im Admin zeigt es weiter mit „prüfen“

**Angenommen** dieses Event
**Wenn** der Admin es erfolgreich speichert
**Dann** erscheint es mit den neu berechneten Werten wieder in genau einer der beiden Listen
**Und** wird es gelöscht, erscheint es in keiner

**Angenommen** eine feste `Clock` und ein Bestand mit markierten und nicht markierten Events
**Wenn** `ListActiveEvents` ohne Untergrenze und `ListArchivedEvents` ohne Begrenzung verglichen werden (Kern-Test)
**Dann** ist jedes nicht markierte Event in genau einer der beiden Listen und kein markiertes in einer von ihnen

**Angenommen** die Spec
**Wenn** ich den Abschnitt „Time model“ und die Beschreibung von `/v1/archive/events` lese
**Dann** steht dort, dass Events, deren gespeicherte Angaben nach einer Regeländerung nicht neu berechnet werden konnten, in keiner der beiden Listen erscheinen, bis sie korrigiert sind
**Und** die Aussage „every event is in exactly one of `/v1/events` and `/v1/archive/events`“ nennt diese Ausnahme
**Und** das Schema `Event` bleibt unverändert, ohne neues Feld (NFR-2)

**Außerdem gilt:**
- Die Auswahl trifft der Kern in den Abfragen über dieselbe Menge, die die Admin-Liste liest. Es gibt keine neue Spalte, keine SQL-Änderung, und der Handler filtert nichts (AD-2, AD-7).
- Nahtstellen: `listMatchingEvents` (2.3, 2.4), der Partitionstest im Kern und der API-Vergleich in `cmd/eventstore` (2.4), die Menge „prüfen“ und ihr Leeren nach dem Commit (2.5b). Das AC aus 2.5 „Antworten vor und nach dem Job identisch“ gilt unverändert.
- Kern-Tests mit fester `Clock` und Fake-Repos decken beide Listen ab, mit und ohne Filter.
```

**Begründung:** Entscheidung B5. Gegenüber der Retro ergänzt die Story den Fall „gelöscht“ und den Spec-Satz in Zeile 262, weil beide sonst widersprüchlich blieben.

### 4.7 `epics.md`, Story 2.4, AC zur Partition

```
OLD:
**Dann** ist jedes Event in genau einer der beiden Listen enthalten

NEW:
**Dann** ist jedes Event in genau einer der beiden Listen enthalten (seit Story 2.7: jedes Event ohne „prüfen“)
```

**Begründung:** Story 2.4 ist `done`, ihr Text soll aber nicht im Widerspruch zu 2.7 stehen.

### 4.8 `epics.md`, Story 3.1, neues AC (nach dem Schema-Szenario)

```
NEW:
**Angenommen** die Kern-Konstanten für Feldnamen (`EventField*`, `FilterField*`)
**Wenn** die CI läuft
**Dann** schlägt ein Test fehl, wenn ein Feldname, den der Kern in Fehlern oder Filtern nennt, nicht als Feld von `EventInput`/`Event` oder als Parameter in `openapi.yaml` vorkommt (AD-9)
**Und** Feldnamen, die nur das Admin-Formular kennt (z. B. `locationId`), sind im Test ausdrücklich als Ausnahme gelistet
```

Dazu in „Deckt ab“ von 3.1 nichts Neues (AD-9 steht schon dort).

**Begründung:** S4. Mit 3.1 meldet der Import Fehler je Feld über diese Konstanten. Weichen sie von der Spec ab, zeigt die Vorschau Felder an, die es in der Datei nicht gibt.

### 4.9 `epics.md`, Story 3.3, neues AC (nach dem Szenario „zwei Schreib-Transaktionen“)

```
NEW:
**Angenommen** eine Deploy-Überlappung, in der die alte Instanz ein Event speichert, während die neue startet
**Wenn** `RecomputeDerived` läuft
**Dann** läuft die Neuberechnung von Events und Orten in einer Transaktion unter derselben Sperre wie `TxRunner` (AD-6, AD-16)
**Und** sie liest erst nach Erhalt der Sperre, sodass sie keine gleichzeitig gespeicherten Werte überschreibt
**Und** ein Postgres-Test belegt, dass eine parallele Schreib-Transaktion auf die Neuberechnung wartet
```

Dazu in „Außerdem gilt“ von 3.3:

```
OLD:
- Nahtstellen: `SaveLocation` (Story 1.4, 1.8) und alle Hüllen über `TxRunner` (`SaveEvent`, `DeleteEvent`, `DeleteLocation`, `MarkArchived`). …

NEW:
- Nahtstellen: `SaveLocation` (Story 1.4, 1.8), alle Hüllen über `TxRunner` (`SaveEvent`, `DeleteEvent`, `DeleteLocation`, `MarkArchived`)
  und `RecomputeDerived` (Story 2.5, 2.5b); die Ausnahme „ohne `TxRunner`“ aus Story 2.5 entfällt. …
```

Und in Story 2.5, „Außerdem gilt“:

```
OLD:
- `RecomputeDerived` läuft vor dem HTTP-Server und schreibt deshalb ohne `TxRunner`; das ist die einzige Ausnahme von der Hülle nach AD-6.

NEW:
- `RecomputeDerived` läuft vor dem HTTP-Server und schreibt deshalb ohne `TxRunner`; das ist die einzige Ausnahme von der Hülle nach AD-6.
  Seit Story 3.3 läuft sie in einer Transaktion unter der Sperre aus AD-6 (R3 der Retro Epic 2).
```

**Begründung:** R3/R9. Die Sperre kommt mit 3.3, deshalb gehört die Umsetzung dorthin. Die Entscheidung steht in 4.4.

### 4.10 `epics.md`, Abschnitt „Additional Requirements“ und ENT-5

Der Wortlaut von AD-5 und AD-16 in der Kurzfassung wird an 4.2 und 4.4 angeglichen:

```
OLD (AD-5):
… `archivedAt` ist nur eine Markierung für Statistik, die keine Abfrage auswertet. `SaveEvent` leert `archivedAt`, wenn das Event wieder aktiv ist.

NEW (AD-5):
… `archivedAt` ist nur eine Markierung für Statistik, die keine Abfrage auswertet. Sie bedeutet „zuletzt als archiviert markiert“: Jedes Speichern und jede ändernde Neuberechnung leert sie, `MarkArchived` setzt sie neu. Events mit „prüfen“ fehlen in beiden Listen.

OLD (ENT-5, Ende):
… Für Orte mit kollidierendem neuen `name_key` gilt dasselbe; die Markierung „prüfen“ erscheint in der Ortsliste und verschwindet nach `SaveLocation` oder `DeleteLocation`.

NEW (ENT-5, Ende):
… Für Orte mit kollidierendem neuen `name_key` gilt dasselbe; die Markierung „prüfen“ erscheint in der Ortsliste und verschwindet nach `SaveLocation` oder `DeleteLocation`.
Events mit „prüfen“ erscheinen in keiner öffentlichen Liste (Story 2.7).
```

**Begründung:** `epics.md` wiederholt den Spine in einer Kurzfassung. Ohne diese Anpassung widerspräche die Kurzfassung dem Spine.

### 4.11 `sprint-status.yaml`

```
OLD:
  epic-2: done
  …
  2-6-lesbare-api-dokumentation-und-abnahme: done
  epic-2-retrospective: done

NEW:
  epic-2: in-progress
  …
  2-6-lesbare-api-dokumentation-und-abnahme: done
  2-7-events-mit-prüfen-öffentlich-ausblenden: backlog
  epic-2-retrospective: done
```

Nach dem Merge dieses Proposals wird `epic-2-retro-item-11-b4-…` auf `done` gesetzt.

**Begründung:** Die Story muss für `bmad-build` sichtbar sein. Die Retrospektive zu Epic 2 bleibt gültig, weil 2.7 eine ihrer Maßnahmen umsetzt. Eine zweite Retro ist nicht nötig.

## 5. Übergabe

- **Umfang:** Moderate. Der Backlog bekommt eine Story, und die Planungsartefakte (PRD, Spine, `epics.md`) ändern sich. Die Änderungen sind Text, eine neue Planung ist nicht nötig.
- **Umsetzung:**
  1. Der Developer-Agent setzt 4.1 bis 4.11 in einem PR um (`chore/b4-b5-correct-course`) und setzt danach B4 auf `done`.
  2. Story 2.7 startet per `bmad-build` mit eigener Spec (`spec-2-7-events-mit-pruefen-oeffentlich-ausblenden.md`), test-first. Am Ende wird `epic-2` wieder auf `done` gesetzt.
  3. Die ACs aus 4.8 und 4.9 setzen die Stories 3.1 und 3.3 um.
- **Erfolgskriterien:**
  - PRD, Spine und `epics.md` widersprechen sich bei „genau eine Liste“, `archivedAt` und der Neuberechnung nicht mehr.
  - Die Stack-Tabelle enthält `oapi-codegen/runtime` v1.7.0.
  - Story 2.7 ist `done`: Ein markiertes Event fehlt in beiden Listen und erscheint nach dem Speichern wieder. Die Spec beschreibt das. Das Coverage-Gate steht weiter bei 100 %.
  - `epic-2` steht nach 2.7 wieder auf `done`, und B4 steht auf `done`.
