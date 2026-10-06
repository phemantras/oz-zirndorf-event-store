---
title: "Sprint Change Proposal: `archived` aus der Leseform entfernen"
status: approved
created: 2026-10-06
author: Andreas (mit Developer-Agent, bmad-correct-course)
scope: minor
---

# Sprint Change Proposal: `archived` aus der Leseform entfernen

## 1. Problem

**Auslöser:** Entscheidung von Andreas am 2026-10-06, während Story 3.5 (Abnahme der Testsammlung) in Review ist.

**Problem:** Die Leseform `Event` hat das Pflichtfeld `archived` (AD-14, `api/v1/openapi.yaml`, Schema `Event`). Sein Wert hängt nur vom Endpunkt ab:

- In `GET /v1/events` ist es immer `false`, in `GET /v1/archive/events` immer `true`. Das sagt die Spec selbst („It is `false` for every event in `/v1/events` and `true` for every event in `/v1/archive/events`“).
- Der Wert gilt nur im Moment der Abfrage. Wer Events zwischenspeichert, hat nach `effectiveEnd` einen falschen Wert. `effectiveEnd` dagegen bleibt richtig und reicht, um „vorbei“ selbst zu prüfen.

Das Feld liefert dem Abnehmer also keine Information, die er nicht schon hat, und verleitet zum Speichern eines Werts, der veraltet. Für eine Vorlage (SM-4) ist das ein schlechtes Muster.

**Kategorie:** Neue Anforderung des Stakeholders (Vereinfachung des API-Vertrags).

**Konflikt mit NFR-2:** Ein Pflichtfeld zu streichen ist eine inkompatible Änderung und gehört nach NFR-2 in eine neue Hauptversion. v1 läuft auf Railway, hat aber noch keinen Abnehmer: Die Karten-App gibt es noch nicht, SM-4 ist nicht durchgeführt. Andreas entscheidet: `archived` wird in v1 gestrichen, als einmalige, dokumentierte Ausnahme von NFR-2. Eine v2 mit 6 Monaten Parallelbetrieb (KON-2) wäre für ein Feld ohne Abnehmer unverhältnismäßig.

## 2. Auswirkungen

| # | Punkt | Status | Ergebnis |
| --- | --- | --- | --- |
| 1 | Auslöser und Belege | [x] | Entscheidung Andreas; Spec-Text des Felds belegt die Redundanz |
| 2.1 | Aktuelles Epic (3) | [N/A] | Story 3.5 bleibt unverändert; ihr Abnahmetest folgt in Story 2.8 der Spec |
| 2.2 | Epic 2 | [!] | Neue Story 2.8; epic-2 geht von `done` auf `in-progress` |
| 2.3–2.5 | Weitere Epics, Reihenfolge | [N/A] | Kein weiteres Epic betroffen; 2.8 läuft nach dem Merge von 3.5 |
| 3.1 | PRD | [!] | FR-11 (Konsequenz zu `archived`), NFR-2 (Ausnahme festhalten) |
| 3.2 | Architektur | [!] | Spine AD-5, AD-7, AD-14 |
| 3.3 | UX | [N/A] | Kein UX-Dokument; die Admin-Liste zeigt „archiviert“ weiter |
| 3.4 | Sonstige Artefakte | [x] | `epics.md` (Inventar, Stories 2.3, 2.4, 2.6, 2.7, neue 2.8), `sprint-status.yaml`. Spec, README und Tests ändert Story 2.8. Abgeschlossene Story-Specs, Retros, Reviews und `.memlog`-Dateien bleiben als datierte Stände unverändert. |
| 4 | Weg | [x] | Direkte Anpassung plus eine kleine Story |

**Technische Auswirkung (Story 2.8):**

- `api/v1/openapi.yaml`: Feld `archived` aus `Event` (`properties`, `required`), aus den Beispielen und aus den Texten („Time model“, `/v1/archive/events`); `api.gen.go` neu erzeugen.
- `internal/adapter/publicapi/v1/events.go`: Abbildung von `Archived` entfällt; Leseform-Test anpassen.
- `cmd/eventstore`: Abnahme- und Vergleichstests ohne `archived`.
- `README.md`: Endpunkt-Tabelle und Beispielantwort.
- Keine Änderung an Kern, Datenbankschema oder Migrationen. `ListedEvent.Archived` wählt im Kern weiter das Archiv aus, die Admin-Liste zeigt weiter „archiviert“, `archived_at` und `MarkArchived` bleiben.

## 3. Empfohlener Weg

Direkte Anpassung von PRD, Spine und `epics.md`, dazu Story 2.8. Aufwand gering (ein Feld, generierter Code, Tests, Doku), Risiko gering, solange es keinen Abnehmer gibt. Story 2.8 startet nach dem Merge von Story 3.5, weil beide den Abnahmetest in `cmd/eventstore` berühren.

## 4. Änderungen

### PRD (`prds/prd-oz-zirndorf-event-store-2026-10-01/prd.md`)

#### V1 – FR-11, Konsequenzen

ALT:
> - Ob ein Event archiviert ist, erkennt der Abnehmer an `archived`. Maßgeblich ist die Vorbei-Regel, nicht der Zeitpunkt der täglichen Bereinigung.

NEU:
> - Ob ein Event archiviert ist, ergibt sich aus der Liste, in der es steht: reguläre Abfrage (FR-8) oder Archiv-Zugriff (FR-12). Ein eigenes Feld dafür gibt es nicht; wer Events zwischenspeichert, prüft `effectiveEnd`. Maßgeblich ist die Vorbei-Regel, nicht der Zeitpunkt der täglichen Bereinigung.

Begründung: Das Feld wiederholt nur den Endpunkt und veraltet beim Zwischenspeichern.

#### V2 – NFR-2, Ausnahme

ALT:
> - **NFR-2 Versionierung:** Die öffentliche API ist versioniert (Umsetzung: KON-1, KON-2). Inkompatible Änderungen, z. B. an der Event-Typen-Liste oder an Feldnamen, gibt es nur in einer neuen Version. Rückwärtskompatible Ergänzungen (neue optionale Felder) dürfen in der bestehenden Version erfolgen.

NEU:
> - **NFR-2 Versionierung:** Die öffentliche API ist versioniert (Umsetzung: KON-1, KON-2). Inkompatible Änderungen, z. B. an der Event-Typen-Liste oder an Feldnamen, gibt es nur in einer neuen Version. Rückwärtskompatible Ergänzungen (neue optionale Felder) dürfen in der bestehenden Version erfolgen. Einmalige Ausnahme: Das Feld `archived` wurde aus v1 entfernt, bevor die API einen Abnehmer hatte (Sprint Change Proposal 2026-10-06 Teil B, Story 2.8).

Begründung: Die Regel bleibt gültig; die Ausnahme ist datiert und begründet, damit niemand sie als Präzedenzfall liest.

### Architektur (`ARCHITECTURE-SPINE.md`, Frontmatter `updated: '2026-10-06'`)

#### V3 – AD-5, Rule

ALT:
> Ob ein Event archiviert ist, entscheidet ausschließlich AD-4. Das API-Feld `archived` wird daraus abgeleitet. Der Bereinigungsjob …

NEU:
> Ob ein Event archiviert ist, entscheidet ausschließlich AD-4. Die API gibt das nicht als Feld aus: Es ergibt sich aus der Liste, in der ein Event steht (AD-7). Der Bereinigungsjob …

#### V4 – AD-7, Rule

ALT:
> Sie liefern Kern-Objekte mit abgeleiteter Genauigkeit und `archived`. Der Adapter bildet sie nur auf die generierten Typen ab.

NEU:
> Sie liefern Kern-Objekte mit abgeleiteter Genauigkeit. Den Archivstatus nutzt der Kern, um das Archiv auszuwählen, und die Admin-Liste, um „archiviert“ anzuzeigen; die Public API gibt ihn nicht aus. Der Adapter bildet sie nur auf die generierten Typen ab.

#### V5 – AD-14, Rule

ALT:
> … **plus** abgeleitete Felder: `startPrecision`, `endPrecision`, `effectiveStart`, `effectiveEnd` und `archived`. **Keine Form nach außen enthält interne Werte:** …

NEU:
> … **plus** abgeleitete Felder: `startPrecision`, `endPrecision`, `effectiveStart` und `effectiveEnd`. Einen Archivstatus enthält sie nicht (AD-5). **Keine Form nach außen enthält interne Werte:** …

### Epics (`epics.md`)

#### V6 – Requirements Inventory

FR-11 ALT:
> Ob ein Event archiviert ist, zeigt `archived`. Maßgeblich ist die Vorbei-Regel …

FR-11 NEU:
> Ob ein Event archiviert ist, ergibt sich aus der Liste, in der es steht; ein eigenes Feld gibt es nicht. Maßgeblich ist die Vorbei-Regel …

NFR-2 ALT:
> NFR-2: Versionierung: Inkompatible Änderungen (z. B. an der Event-Typen-Liste oder an Feldnamen) gibt es nur in einer neuen Hauptversion. Neue optionale Felder dürfen in der bestehenden Version ergänzt werden.

NFR-2 NEU:
> NFR-2: Versionierung: Inkompatible Änderungen (z. B. an der Event-Typen-Liste oder an Feldnamen) gibt es nur in einer neuen Hauptversion. Neue optionale Felder dürfen in der bestehenden Version ergänzt werden. Einmalige Ausnahme: `archived` wurde vor dem ersten Abnehmer aus v1 entfernt (Story 2.8).

AD-5 ALT: „Das API-Feld `archived` ist abgeleitet.“ → NEU: „Die API gibt ihn nicht als Feld aus; er ergibt sich aus der Liste.“

AD-7 ALT (Ende): „… Orte nach Namen im Kern (ENT-8).“ → NEU: „… Orte nach Namen im Kern (ENT-8). Den Archivstatus gibt die Public API nicht aus.“

AD-14 ALT: „… `effectiveStart`, `effectiveEnd` (lokaler Offset Europe/Berlin) und `archived`.“ → NEU: „… `effectiveStart` und `effectiveEnd` (lokaler Offset Europe/Berlin), ohne Archivstatus.“

#### V7 – Abgeschlossene Stories 2.3, 2.4, 2.6, 2.7

Wie bei Story 2.7 bleibt der Wortlaut erhalten und bekommt einen Verweis:

- 2.3, erstes AC: „… `effectiveStart`, `effectiveEnd`, `archived`“ → „… `effectiveStart`, `effectiveEnd`, `archived` (seit Story 2.8 ohne `archived`)“
- 2.4, Außerdem gilt: „Archivierte Events haben dieselbe Struktur wie aktive, mit `archived: true`.“ → „Archivierte Events haben dieselbe Struktur wie aktive, mit `archived: true` (seit Story 2.8 ohne `archived`).“
- 2.6, zweites AC: „… `effective*`, `archived`, die Standardwerte …“ → „… `effective*`, `archived` (seit Story 2.8 entfallen), die Standardwerte …“
- 2.7, Nutzen: „… veraltetem Zeitraum oder falschem `archived` zeige.“ → „… veraltetem Zeitraum oder in der falschen Liste zeige.“

#### V8 – Neue Story 2.8 (nach Story 2.7)

> ### Story 2.8: `archived` aus der Leseform entfernen
>
> Als Karten-App,
> möchte ich Events ohne ein Feld bekommen, das nur wiederholt, welche Liste ich abgefragt habe,
> damit ich keinen Wert zwischenspeichere, der mit der Zeit falsch wird.
>
> **Deckt ab:** FR-11, FR-12, NFR-2, AD-5, AD-7, AD-14
>
> **Hinweis:** Sprint Change Proposal 2026-10-06 Teil B. Einmalige Ausnahme von NFR-2, weil v1 noch keinen Abnehmer hat. Die Story startet nach dem Merge von Story 3.5.
>
> **Acceptance Criteria:**
>
> **Angenommen** die Spec
> **Wenn** ich das Schema `Event` lese
> **Dann** enthält es kein Feld `archived`, weder in `properties` noch in `required`
> **Und** die Beispiele von `GET /v1/events` und `GET /v1/archive/events` enthalten es nicht
> **Und** der Abschnitt „Time model“ und die Beschreibung von `/v1/archive/events` erklären, dass die Liste angibt, ob ein Event vorbei ist, und dass Abnehmer, die Events zwischenspeichern, `effectiveEnd` gegen die aktuelle Zeit prüfen
>
> **Angenommen** ein aktives und ein vergangenes Event
> **Wenn** ein Abnehmer `GET /v1/events` und `GET /v1/archive/events` aufruft
> **Dann** enthält kein Event der beiden Antworten ein Feld `archived`
> **Und** der Leseform-Test aus Story 2.3 prüft die Schlüsselmenge jeder Ebene ohne `archived`
>
> **Angenommen** eine feste `Clock` und ein gemischter Bestand
> **Wenn** beide Listen verglichen werden (Kern-Test und API-Vergleich aus Story 2.4)
> **Dann** ist jedes Event ohne „prüfen“ weiter in genau einer der beiden Listen
>
> **Außerdem gilt:**
> - `api.gen.go` wird mit `go generate ./...` neu erzeugt, nicht von Hand geändert (AD-8).
> - Der Kern bleibt unverändert: `ListedEvent.Archived` wählt weiter das Archiv aus, die Admin-Liste zeigt weiter „archiviert“. `archived_at` und `MarkArchived` bleiben (AD-5, AD-13).
> - README (Endpunkt-Tabelle und Beispielantwort) und die Abnahmetests in `cmd/eventstore` folgen der Spec.
> - Nahtstellen: Leseform-Test und Abbildung in `publicapi/v1` (2.3), Beschreibung des Archivs und API-Vergleich (2.4), „Time model“ in der Spec (2.6, 2.7), Abnahme der Testsammlung in `cmd/eventstore` (3.5).

### Sprint-Status (`implementation-artifacts/sprint-status.yaml`)

#### V9

- `epic-2: done` → `epic-2: in-progress`
- neu nach `2-7-events-mit-prüfen-öffentlich-ausblenden`: `2-8-archived-aus-der-leseform-entfernen: backlog`
- `epic-2-retrospective: done` bleibt; 2.8 braucht keine eigene Retro.

## 5. Übergabe

**Umfang:** Minor. Direkte Umsetzung durch den Developer-Agent.

| Wer | Aufgabe |
| --- | --- |
| Developer-Agent (dieser Lauf) | V1 bis V9 in die Planungsartefakte übernehmen, Commit auf `chore/correct-course-archived`, PR |
| Andreas | PR prüfen und mergen; Story 3.5 abschließen |
| Developer-Agent (`bmad-build`) | Story 2.8 nach dem Merge von 3.5 test-first umsetzen |

**Erfolgskriterien:**

- PRD, Spine und `epics.md` nennen `archived` nirgends mehr als Teil der Leseform; NFR-2 nennt die Ausnahme.
- Nach Story 2.8 liefern `/v1/events` und `/v1/archive/events` kein `archived`, `api.gen.go` ist neu erzeugt, die CI (Abdeckung, Lint, Generator-Diff) ist grün.
