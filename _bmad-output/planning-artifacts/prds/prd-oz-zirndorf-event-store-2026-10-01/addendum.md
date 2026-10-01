---
title: "Addendum zum PRD: OZ Zirndorf Event Store"
created: 2026-10-01
updated: 2026-10-01
---

# Addendum zum PRD: OZ Zirndorf Event Store

Dieses Addendum enthält Details, die nicht ins PRD gehören, aber für Architektur und Umsetzung wichtig sind: technische Vorgaben, verworfene Alternativen und Hinweise zum Ausgangsmaterial.

## 1. Technische Vorgaben und Annahmen

- **Hosting:** railway.app (aktueller Hosting-Anbieter von Andreas).
- **Technologie:** frei wählbar (OZ-Prinzip). Im PRD ist nichts festgelegt.
- **Archivierung:** täglicher Cron-Job, der Vorbei-Events in den Archiv-Bestand verschiebt. Ob das Archiv eine eigene Tabelle oder ein Statusfeld ist, entscheidet die Architektur. Fachlich gilt nur: getrennter Zugriff, dauerhafte Aufbewahrung.
- **Vorbei-Regel:** Ein Event ist vorbei, wenn sein Ende überschritten ist. Ist beim Ende nur das Datum bekannt, gilt 23:59:59 des End-Tages. Ohne Ende gilt 23:59:59 des Beginn-Tages (Europe/Berlin). Reguläre Abfrage und Archiv-Zugriff wenden dieselbe Regel bei jeder Abfrage an. Der Cron-Lauf ist nur Aufräumen und darf das Ergebnis nicht verändern.
- **Mögliche Ressourcen (Vorschlag):** `GET /v1/events` (Filter `from`, `to`, `type`), `GET /v1/events/{id}`, `GET /v1/locations`, `GET /v1/event-types`, `GET /v1/archive/events`. Die Namen sind englisch gemäß PRD KON-7.
- **Koordinaten-Auswahl im Admin:** z. B. ein Kartenpicker auf Basis von OpenStreetMap (Andreas' Idee). Die konkrete Bibliothek wählt die Architektur.
- **API-Konventionen:** Versionierung, Fehlerformat, Zeit- und Filterformat sind im PRD (§6) festgelegt. RFC 9457 wurde gewählt, weil es ein verbreiteter Standard und für andere OZ-Backends leicht übernehmbar ist.

## 2. Ausgangsmaterial: `zirndorf_events.json` (Stand 2026-09-18)

- Felder heute: `name`, `startTime`, `endTime`, `location { address, latitude, longitude, note? }`, `timetable[] { description, startTime, endTime }`. Metadaten: `generated`, `source_note`, `limitations_note`.
- 42 Events von Mai 2026 bis Dezember 2027, ein Teil davon schon vorbei. Beim Import der Testsammlung gehören diese also sofort zum Archiv.
- Lücken gegenüber dem PRD, die das Import-Format v1 schließen muss: Event-Typ, Zeitgenauigkeit (heute `00:00` für „unbekannt“), Ortsgenauigkeit (heute Freitext in `note`), Quelle (heute im Namen), Import-Schlüssel, Ort als Referenz.
- Die Paul-Metz-Halle kommt rund 20-mal mit identischen Daten vor. Das war der Anlass für das eigene Ort-Objekt.

## 3. Verworfene Alternativen

- **Adresse + Stammkoordinaten statt Ort-Objekt** (ursprüngliche Brief-Entscheidung): verworfen, weil die Daten doppelt vorliegen und Gruppierung nach Ort nur über Koordinatenvergleich möglich wäre.
- **Serienregeln für wiederkehrende Termine:** verworfen, weil Stadtratssitzungen unregelmäßig sind, einzelne Termine leicht verschoben werden und Regeln Import und Duplikaterkennung verkomplizieren. Jeder Termin ist ein eigenes Event. Eine Serien-Kennung gibt es in v1 ebenfalls nicht.
- **Status „läuft gerade / beginnt bald“ in der API:** verworfen. Abnehmer berechnen ihn selbst aus Beginn, Ende und Zeitgenauigkeit. Damit entfällt auch die Festlegung eines „bald“-Zeitfensters.
- **Freie Tags statt fester Event-Typen:** verworfen, weil Kartenfilter und Icons eine feste Liste brauchen.
- **Duplikaterkennung nur über Titel + Beginn + Ort, ohne Import-Schlüssel:** verworfen, weil dann ein erneuter Import ein Event nicht aktualisieren kann.

## 4. Lizenz

Die Lizenzoptionen (Code: MIT, Apache-2.0, AGPL-3.0; Daten: CC BY 4.0, DL-DE/BY-2.0, ODbL) und die Hinweise zu Drittquellen (umformulierte Ablaufpläne, `robots.txt` beim Kirchweih-PDF) stehen im Addendum des Briefs. Das ist eine Entscheidungshilfe, keine Rechtsberatung.
