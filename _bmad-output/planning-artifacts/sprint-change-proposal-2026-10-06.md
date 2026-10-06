---
title: "Sprint Change Proposal: Testsammlung an Story 3.4 angleichen (39 Events, Weihnachtsmarkt im Admin)"
status: approved
created: 2026-10-06
author: Andreas (mit Developer-Agent, bmad-correct-course)
scope: minor
---

# Sprint Change Proposal: Testsammlung an Story 3.4 angleichen

## 1. Problem

**Auslöser:** Offener Eintrag in `_bmad-output/implementation-artifacts/deferred-work.md` aus Story 3.4 (Review-Befund 4, `spec-3-4-testsammlung-ins-format-v1-ueberfuehren.md`).

**Problem:** Mit Story 3.4 hat Andreas am 2026-10-06 entschieden:

- Die Testsammlung im Format v1 (`testdata/zirndorf_events.v1.json`) enthält **39 Events**. Die Quelldatei `testdata/zirndorf_events.json` enthält 41; Stadtführung, Museums-Herbstmarkt und Innenstadt-Herbstmarkt sind zu „Zirndorfer Herbstmarkt“ zusammengelegt. Die 42 aus Brief und PRD waren ein Zählfehler.
- Der **Weihnachtsmarkt ist nicht in der Datei**. Andreas hat ihn in Produktion über die Admin-Oberfläche angelegt.
- Beide Dateien liegen unter `testdata/`, nicht mehr im Repo-Root.
- Die v1-Datei ist in Produktion bereits importiert.

PRD, Addendum und `epics.md` nennen weiter 42 Events, und Story 3.5 erwartet den Weihnachtsmarkt aus dem Import in Produktion. Das dritte Kriterium von Story 3.5 ist so nicht erfüllbar.

**Kategorie:** Missverständnis der ursprünglichen Anforderung (Zählfehler) und Entscheidung des Stakeholders während der Umsetzung.

## 2. Auswirkungen

| # | Punkt | Status | Ergebnis |
| --- | --- | --- | --- |
| 1 | Auslöser und Belege | [x] | Story 3.4, Review-Befund 4 |
| 2.1 | Aktuelles Epic (3) | [!] | Story 3.4 (Notiz) und Story 3.5 (Kriterien) anpassen |
| 2.2–2.5 | Weitere Epics, Reihenfolge | [N/A] | Keine neuen oder entfallenden Stories, Reihenfolge bleibt |
| 3.1 | PRD | [!] | FR-18 und SM-2: 39 statt 42; Addendum §2: 41 Events in der Quelle |
| 3.2 | Architektur | [N/A] | Spine nennt keine Zahl und keine Fixture |
| 3.3 | UX | [N/A] | Kein UX-Dokument |
| 3.4 | Sonstige Artefakte | [x] | Brief, `extract-brief.md`, Architektur-Reviews und `.memlog`-Dateien bleiben als datierte Stände unverändert. Story 2.6 prüft SM-1 gegen eine eigene Fixture und ist nicht betroffen. |
| 4 | Weg | [x] | Direkte Anpassung |

**Technische Auswirkung:** keine. Kein Code, kein Schema, keine Migration.

## 3. Empfohlener Weg

Direkte Anpassung der Planungsartefakte. Aufwand gering, Risiko gering, kein Einfluss auf den Zeitplan. Story 3.5 bleibt der nächste Schritt.

SM-1 wird in Produktion nur über die Zeitraumabfrage geprüft. Einen zweiten Import in Produktion gibt es nicht; den zweiten Import deckt der Integrationstest von Story 3.5 ab.

## 4. Änderungen

### V1 – SM-2 (`epics.md` Success Metrics, `prd.md` §8)

`epics.md` ALT:
> SM-2: Die 42 Events werden vollständig importiert, ein zweiter Import erzeugt 0 zusätzliche Events. (Die Datei vom 2026-09-18 enthält 41 Events; die Zahl klärt Story 3.4.)

`epics.md` NEU:
> SM-2: Die Testsammlung im Format v1 (`testdata/zirndorf_events.v1.json`, 39 Events) wird vollständig importiert, ein zweiter Import erzeugt 0 zusätzliche Events.

`prd.md` ALT:
> **SM-2 Import-Tauglichkeit:** Die Testsammlung mit 42 Events, umgestellt auf das Import-Format v1, wird vollständig importiert. …

`prd.md` NEU:
> **SM-2 Import-Tauglichkeit:** Die Testsammlung mit 39 Events, umgestellt auf das Import-Format v1, wird vollständig importiert. … (Die frühere Angabe 42 war ein Zählfehler; siehe Story 3.4 und Sprint Change Proposal 2026-10-06.)

### V2 – FR-18 (`epics.md` Requirements Inventory, `prd.md` §4)

ALT: „Die Testsammlung (42 Events) lässt sich vollständig importieren …“
NEU: „Die Testsammlung (39 Events) lässt sich vollständig importieren …“

### V3 – Absatz „Testsammlung“ (`epics.md` Success Metrics)

ALT:
> **Testsammlung:** `zirndorf_events.json` (Stand 2026-09-18, Mai 2026 bis Dezember 2027) wird in das Import-Format v1 überführt. … Die Datei liegt derzeit im Repo-Root, Story 3.4 verschiebt sie nach `testdata/`.

NEU:
> **Testsammlung:** `testdata/zirndorf_events.json` (Stand 2026-09-18, Mai 2026 bis Dezember 2027, 41 Events) ist mit Story 3.4 in das Import-Format v1 überführt: `testdata/zirndorf_events.v1.json` mit 39 Events (Herbstmarkt aus drei Einträgen zusammengelegt). … Der Weihnachtsmarkt ist nicht in der Datei; er ist in Produktion über die Admin-Oberfläche angelegt.

### V4 – PRD-Addendum §2

ALT: „42 Events von Mai 2026 bis Dezember 2027, ein Teil davon schon vorbei. …“
NEU: „41 Events von Mai 2026 bis Dezember 2027 (nach der Überführung in v1: 39, siehe Story 3.4), ein Teil davon schon vorbei. …“

### V5 – Story 3.4, „Außerdem gilt“

ALT:
> - Die Quelldatei enthält 41 Events, PRD und SM-2 nennen 42. Die Story klärt die Abweichung und hält die gültige Zahl fest; die folgenden Kriterien sprechen von „allen Events der Datei“.

NEU:
> - Geklärt (2026-10-06): Die v1-Datei enthält 39 Events; die 42 aus PRD und SM-2 waren ein Zählfehler. Der Weihnachtsmarkt ist nicht in der Datei, sondern im Admin angelegt. Die folgenden Kriterien sprechen von „allen Events der Datei“.

### V6 – Story 3.5, drittes Kriterium und „Außerdem gilt“

ALT:
> **Angenommen** grüne Integrationstests
> **Wenn** Andreas die Datei in Produktion importiert
> **Dann** liefert `GET /v1/events` mit einem Zeitraum über die Adventszeit den Weihnachtsmarkt mit Zeitraum, Ort, Zeitgenauigkeit, Ortsgenauigkeit und Quelle (SM-1)
>
> - Der Import in Produktion und die SM-1-Prüfung sind als manuelle Abschlussschritte vermerkt.

NEU:
> **Angenommen** grüne Integrationstests, die importierte Testsammlung und der im Admin angelegte Weihnachtsmarkt in Produktion
> **Wenn** `GET /v1/events` mit einem Zeitraum über die Adventszeit abgefragt wird
> **Dann** liefert die Antwort die Termine des Weihnachtsmarkts mit Zeitraum, Ort, Zeitgenauigkeit, Ortsgenauigkeit und Quelle (SM-1)
>
> - Die SM-1-Prüfung in Produktion ist ein manueller Abschlussschritt. Die Testsammlung ist dort seit 2026-10-06 importiert; ein weiterer Import in Produktion ist nicht nötig.

### V7 – Nacharbeit

- `prd.md`: Kopfzeile `updated: 2026-10-06`.
- `deferred-work.md`: Eintrag aus Story 3.4 auf `status: done`.

## 5. Übergabe

**Umfang:** minor. Die Änderungen setzt der Developer-Agent direkt in den Planungsartefakten um, per Pull Request.

**Erfolgskriterien:**
- Kein Planungsartefakt außer den datierten Ständen (Brief, Extrakt, Reviews, `.memlog`) nennt noch 42 Events.
- Story 3.5 ist ohne Weihnachtsmarkt in der Datei vollständig erfüllbar.
- Der Eintrag in `deferred-work.md` ist geschlossen.
