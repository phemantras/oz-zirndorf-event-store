---
title: "Sprint Change Proposal: Import vor verlorenen Änderungen schützen, Import-Schlüssel im Admin"
status: approved
created: 2026-10-06
author: Andreas (mit Developer-Agent, bmad-correct-course)
scope: minor
---

# Sprint Change Proposal: Import vor verlorenen Änderungen schützen, Import-Schlüssel im Admin

## 1. Problem

**Auslöser:** Retrospektive Epic 3 vom 2026-10-06 (`implementation-artifacts/epic-3-retro-2026-10-06.md`), Befunde R8, R9 und R10, Maßnahme C4. Andreas hat R8 und R9 am 2026-10-06 entschieden: umsetzen.

**Probleme:**

- **R8, verlorene Änderung:** `CommitImport` meldet einen Eintrag als `stale`, wenn Klasse, Ziel-ID, „Ort neu“ oder die Bestands-Kandidaten von der Vorschau abweichen (`internal/core/import_commit.go:242-256`). Ob sich das Ziel-Event selbst seit der Vorschau geändert hat, prüft der Commit nicht. Wird das Ziel zwischen Vorschau und Commit im Admin bearbeitet, überschreibt `update` bzw. „überschreiben“ diese Änderung, ohne dass die Person sie gesehen hat.
- **R9, unsichtbarer Import-Schlüssel:** „Überschreiben“ hängt den `importKey` des Eintrags dauerhaft an das gewählte Event (`import_commit.go:306-310`). Der Admin zeigt den Schlüssel nirgends an, und es gibt keinen Anwendungsfall, der ihn entfernt. Wurde das falsche Event überschrieben, aktualisiert jeder spätere Import dieses Event. Der einzige Ausweg wäre SQL, und AD-6 verbietet Schreiben am Kern vorbei.
- **R10, Bedeutung des Schlüssels:** Ein Eintrag mit Schlüssel-Treffer wird immer geschrieben, auch wenn das Ziel im Archiv liegt. Nutzt eine Quelle denselben Schlüssel jedes Jahr, ersetzt der Import das Vorjahres-Event. Das ist gewollt, weil ein Schlüssel ein Event identifiziert, steht aber nirgends.

**Kategorie:** Lücke in den ursprünglichen Anforderungen, aufgedeckt durch das Review des fertigen Epics.

## 2. Auswirkungen

| # | Punkt | Status | Ergebnis |
| --- | --- | --- | --- |
| 1 | Auslöser und Belege | [x] | Retro Epic 3, R8–R10, mit Code-Stellen |
| 2.1 | Aktuelles Epic (3) | [!] | Neue Story 3.6; `epic-3` geht von `done` auf `in-progress` |
| 2.2–2.5 | Weitere Epics, Reihenfolge | [N/A] | Epic 1 und 2 sind fertig, weitere Epics gibt es nicht |
| 3.1 | PRD | [!] | FR-16 (Bedeutung des Schlüssels, Pflege im Admin), FR-18 (`stale` bei geändertem Ziel) |
| 3.2 | Architektur | [!] | Spine AD-6 (neuer Schreib-Anwendungsfall `RemoveImportKey`), AD-10 Schritt 4 (Fingerabdruck) |
| 3.3 | UX | [N/A] | Kein UX-Dokument; die Bearbeitungsseite eines Events bekommt eine Zeile und einen Knopf |
| 3.4 | Sonstige Artefakte | [x] | `epics.md` (Inventar AD-6, AD-10, ENT-16, neue Story 3.6), `sprint-status.yaml`. `openapi.yaml` und README ändert Story 3.6. Abgeschlossene Specs und Retros bleiben als datierte Stände unverändert. |
| 4 | Weg | [x] | Direkte Anpassung plus eine kleine Story |

**Technische Auswirkung (Story 3.6):**

- **Kern:** Ein Fingerabdruck eines gespeicherten Events, abgeleitet aus seiner kanonischen Form samt Ort und `importKey`. Die Vorschau liefert ihn für das Ziel eines `update`/`unchanged` und für jeden Bestands-Kandidaten eines Duplikatverdachts; `ImportDecision` trägt ihn zurück. `isStale` vergleicht ihn für das Event, das tatsächlich geschrieben würde. Neuer Anwendungsfall `RemoveImportKey(ctx, eventID)` über `TxRunner` mit transaktionsgebundenem Kern.
- **Admin:** Je Event versteckte Felder für die Fingerabdrücke im Entscheidungsformular. Die Bearbeitungsseite eines Events zeigt den Import-Schlüssel nur lesend und hat einen Knopf „Import-Schlüssel entfernen“ (POST mit CSRF, Rückfrage wie beim Löschen).
- **Spec:** Beschreibung von `importKey` in `openapi.yaml` ergänzt; `api.gen.go` neu erzeugen.
- **Keine Migration:** `events.import_key` ist schon nullable. `EventRepo.SetImportKey` existiert; das Entfernen setzt den Wert auf `NULL`.

## 3. Empfohlener Weg

Direkte Anpassung von PRD, Spine und `epics.md`, dazu Story 3.6 in Epic 3. Aufwand gering, Risiko gering, weil der Import erst seit heute in Produktion benutzt wird. Die Story kann unabhängig von den Retro-Maßnahmen C1 (Import-Härtung) und C2 (Testlücken) laufen. Beide berühren `import_commit.go` und `admin/import.go`; wer zuerst merged, ist egal, der zweite PR zieht nach.

## 4. Änderungen

### PRD (`prds/prd-oz-zirndorf-event-store-2026-10-01/prd.md`)

#### V1 – FR-16, Konsequenzen (neue Punkte am Ende)

NEU:
> - Ein Import-Schlüssel identifiziert genau ein Event, keine wiederkehrende Reihe. Ein erneuter Import mit demselben Schlüssel aktualisiert dieses Event, auch wenn es im Archiv liegt; für jeden Termin einer Reihe braucht die Quelle einen eigenen Schlüssel.
> - Der Admin sieht den Import-Schlüssel eines Events und kann ihn entfernen. Danach wird das Event bei einem Import wie jedes Event ohne Schlüssel behandelt.

Begründung: R10 dokumentiert die Bedeutung, R9 macht den Schlüssel sichtbar und korrigierbar.

#### V2 – FR-18, Konsequenzen (neuer Punkt nach „Ohne Entscheidung …“)

NEU:
> - Wurde ein Event, das ein Eintrag aktualisieren oder überschreiben soll, seit der Vorschau geändert, wird der Eintrag nicht übernommen und als veraltet gemeldet.

Begründung: R8; keine Änderung geht still verloren.

### Architektur (`ARCHITECTURE-SPINE.md`, Frontmatter `updated: '2026-10-06'` bleibt)

#### V3 – AD-6, Rule

ALT:
> Events und Orte werden ausschließlich über Anwendungsfälle des Kerns geschrieben: `SaveEvent`, `DeleteEvent`, `SaveLocation`, `DeleteLocation`, `CommitImport`, `RecomputeDerived` (Start, AD-16) und `MarkArchived` (Bereinigungsjob, AD-13). … `SaveEvent` aus dem Admin ändert `importKey` nie, nur `CommitImport` setzt ihn.

NEU:
> Events und Orte werden ausschließlich über Anwendungsfälle des Kerns geschrieben: `SaveEvent`, `DeleteEvent`, `SaveLocation`, `DeleteLocation`, `CommitImport`, `RemoveImportKey`, `RecomputeDerived` (Start, AD-16) und `MarkArchived` (Bereinigungsjob, AD-13). … `SaveEvent` aus dem Admin ändert `importKey` nie. Nur `CommitImport` setzt ihn, und nur `RemoveImportKey` entfernt ihn auf ausdrücklichen Wunsch im Admin.

#### V4 – AD-10, Rule, Schritt 4

ALT:
> 4. **Speichern:** Der vollständige Satz wird samt Entscheidungen gesendet. Jede Entscheidung trägt die beim Upload ermittelte Klasse und gegebenenfalls die Ziel-ID. Der Kern klassifiziert erneut. Weicht die Klasse oder die Ziel-ID ab, wird der Eintrag **nicht** übernommen und als `stale` gemeldet.

NEU:
> 4. **Speichern:** Der vollständige Satz wird samt Entscheidungen gesendet. Jede Entscheidung trägt die beim Upload ermittelte Klasse, gegebenenfalls die Ziel-ID und den Fingerabdruck jedes Bestands-Events, das sie schreiben könnte. Der Fingerabdruck leitet der Kern aus der kanonischen Form des gespeicherten Events samt Ort und `importKey` ab. Der Kern klassifiziert erneut. Weicht die Klasse oder die Ziel-ID ab, oder hat sich das Event, das geschrieben würde, seit dem Upload geändert, wird der Eintrag **nicht** übernommen und als `stale` gemeldet.

Begründung: Erweitert „Prevents: Entscheidungen, die auf einem veralteten Stand beruhen“ auf den Inhalt des Ziels. Kein Zwischenstand auf dem Server: Der Fingerabdruck reist im Formular.

### Epics (`epics.md`)

#### V5 – Requirements Inventory

AD-6 ALT:
> … `SaveEvent` aus dem Admin ändert `importKey` nie. Admin und Import nutzen …

AD-6 NEU:
> … `SaveEvent` aus dem Admin ändert `importKey` nie; `CommitImport` setzt ihn, `RemoveImportKey` entfernt ihn. Admin und Import nutzen …

(Die Aufzählung der Anwendungsfälle am Anfang von AD-6 bekommt `RemoveImportKey` nach `CommitImport`.)

AD-10 ALT (4):
> (4) Beim Speichern wird der vollständige Satz samt Entscheidungen, ursprünglicher Klasse und Ziel-ID gesendet und neu klassifiziert. Bei einer Abweichung wird der Eintrag als `stale` gemeldet.

AD-10 NEU (4):
> (4) Beim Speichern wird der vollständige Satz samt Entscheidungen, ursprünglicher Klasse, Ziel-ID und den Fingerabdrücken der Bestands-Events, die geschrieben werden könnten, gesendet und neu klassifiziert. Bei einer Abweichung, auch bei einem seit dem Upload geänderten Ziel, wird der Eintrag als `stale` gemeldet.

ENT-16 ALT:
> ENT-16 (→ AD-6) Import-Schlüssel: `SaveEvent` aus dem Admin ändert `importKey` nie. Nur `CommitImport` setzt ihn.

ENT-16 NEU:
> ENT-16 (→ AD-6) Import-Schlüssel: `SaveEvent` aus dem Admin ändert `importKey` nie. Nur `CommitImport` setzt ihn. Der Admin zeigt ihn nur lesend an und kann ihn über `RemoveImportKey` entfernen. Ein Schlüssel identifiziert genau ein Event, keine wiederkehrende Reihe.

#### V6 – Neue Story 3.6 (nach Story 3.5)

> ### Story 3.6: Import vor verlorenen Änderungen schützen und Import-Schlüssel pflegen
>
> Als Admin,
> möchte ich, dass ein Import keine Änderung überschreibt, die ich nach der Vorschau gemacht habe, und dass ich den Import-Schlüssel eines Events sehe und entfernen kann,
> damit kein Import still meine Arbeit verwirft oder dauerhaft am falschen Event hängt.
>
> **Deckt ab:** FR-16, FR-18, AD-6, AD-10, ENT-16
>
> **Hinweis:** Sprint Change Proposal 2026-10-06 Teil C (Retro Epic 3, R8–R10).
>
> **Acceptance Criteria:**
>
> **Angenommen** ein Eintrag ist in der Vorschau `update` auf Event X
> **Wenn** X vor dem Commit im Admin geändert wird (z. B. Beginn-Uhrzeit oder Notiz)
> **Dann** ist der Eintrag beim Commit `stale`, und X behält die Änderung
>
> **Angenommen** ein Duplikatverdacht mit der Wahl „überschreiben“ von Y
> **Wenn** Y vor dem Commit geändert wird
> **Dann** ist der Eintrag `stale`
> **Und** wird stattdessen ein anderer Kandidat geändert, den die Wahl nicht schreibt, bleibt die Entscheidung gültig
>
> **Angenommen** das Ziel ist seit der Vorschau unverändert
> **Wenn** übernommen wird
> **Dann** verhält sich der Commit wie in Story 3.3, und der zweite Import der Testsammlung bleibt vollständig `unchanged` (Story 3.5)
>
> **Angenommen** ein Event mit `importKey`
> **Wenn** ich es im Admin bearbeite
> **Dann** sehe ich den Import-Schlüssel nur lesend
> **Und** ein Event ohne Schlüssel zeigt keine solche Zeile
>
> **Angenommen** ein Event mit `importKey`
> **Wenn** ich „Import-Schlüssel entfernen“ wähle und bestätige
> **Dann** entfernt der Kern-Anwendungsfall `RemoveImportKey` den Schlüssel über `TxRunner`, die übrigen Felder bleiben unverändert
> **Und** ein späterer Import mit diesem Schlüssel klassifiziert den Eintrag nicht mehr als `update` dieses Events
> **Und** für ein unbekanntes Event liefert der Kern `ErrNotFound`, der Admin zeigt die übliche Meldung
>
> **Angenommen** die Spec
> **Wenn** ich die Beschreibung von `importKey` in `EventInput` lese
> **Dann** steht dort, dass ein Schlüssel genau ein Event identifiziert, ein erneuter Import es auch im Archiv aktualisiert und jeder Termin einer Reihe einen eigenen Schlüssel braucht
>
> **Außerdem gilt:**
> - Kein Zwischenstand auf dem Server; der Fingerabdruck reist im Formular (AD-10).
> - `api.gen.go` wird mit `go generate ./...` neu erzeugt, nicht von Hand geändert.
> - Keine Migration; `RemoveImportKey` schreibt über einen Repository-Port, der nur vom Kern aufgerufen wird (AD-6).
> - Nahtstellen: `isStale` und das Entscheidungsformular (3.3), Klassifizierung und `unchanged`-Vergleich (3.2), Abnahme der Testsammlung (3.5), Bearbeitungsseite und Löschrückfrage eines Events (1.7, 1.11), `SaveEvent` lässt `importKey` stehen (3.2).

### Sprint-Status (`implementation-artifacts/sprint-status.yaml`)

#### V7

- `epic-3: done` → `epic-3: in-progress`
- neu nach `3-5-testsammlung-importieren-und-abnahme`: `3-6-import-vor-verlorenen-änderungen-schützen-und-import-schlüssel-pflegen: backlog`
- `epic-3-retrospective: done` bleibt; 3.6 braucht keine eigene Retro.
- Die Maßnahme `epic-3-retro-item-…-c4-…` steht auf `done`, sobald dieser Plan gemergt ist (Umsetzung folgt in 3.6).

## 5. Übergabe

**Umfang:** Minor. Direkte Umsetzung durch den Developer-Agent.

| Wer | Aufgabe |
| --- | --- |
| Developer-Agent (dieser Lauf) | V1 bis V7 in die Planungsartefakte übernehmen, Commit auf `chore/b3-c4-build-und-correct-course` zusammen mit B3, PR |
| Andreas | PR prüfen und mergen |
| Developer-Agent (`bmad-build`) | Story 3.6 test-first umsetzen |

**Erfolgskriterien:**

- PRD, Spine und `epics.md` nennen `RemoveImportKey`, den Fingerabdruck in AD-10 und die Bedeutung des Schlüssels.
- Nach Story 3.6: Ein seit der Vorschau geändertes Ziel ergibt `stale`. Der Admin zeigt den Schlüssel an und entfernt ihn über den Kern. Die CI (Abdeckung, Lint, Generator-Diff, Postgres-Tests) ist grün.
