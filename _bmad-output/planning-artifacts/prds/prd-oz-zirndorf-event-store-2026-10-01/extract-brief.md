---
title: "Extraktion aus Brief + Addendum: OZ Zirndorf Event Store"
created: 2026-10-01
sources:
  - _bmad-output/planning-artifacts/briefs/brief-oz-zirndorf-event-store-2026-10-01/brief.md
  - _bmad-output/planning-artifacts/briefs/brief-oz-zirndorf-event-store-2026-10-01/addendum.md
---

# Extraktion für das PRD: OZ Zirndorf Event Store

Kürzel: [B] = brief.md, [A] = addendum.md. Nur Inhalte aus den Quellen, nichts ergänzt.

## 1. Vision / Problem

- Vision: Alle Veranstaltungen in Zirndorf an einem Ort; langfristig „verlässliche Terminquelle für Zirndorf“, die OZ-Karte ist „ihr sichtbarstes Gesicht“ [B].
- Problem: Termine verstreut über städtischen Kalender (lückenhaft, Kulturamt veröffentlicht spät), Ratsinformationssystem (schlecht automatisch abrufbar), Kirchweih-Seite, Vereinsseiten, Ticketportale, Papieraushänge [B].
- Ortsteil-Kirchweihen und Fischerfest fehlen im städtischen Kalender; keine Gesamtübersicht, „schon gar nicht auf einer Karte“ [B].
- Datenunschärfe (Testsammlung, 42 Events, ≥6 Quellen): oft fehlt Uhrzeit oder Ende; bei Festen oft nur Ortsteil bekannt; Herkunft nur als Freitext im Namen [B].
- Kernrisiko: „Ein Datenbestand, der diese Unschärfe verschweigt, setzt auf der Karte Pins, die exakter wirken, als sie sind.“ [B]
- Nebenziel: Event Store kann „zur Vorlage für weitere OZ-Backends werden“ (Format, Versionierung, Doku, CORS) [B].

## 2. Zielgruppen & Abnehmer

- **Andreas (Pfleger/Admin):** einziger Admin in v1; braucht schnellen JSON-Import recherchierter Termine ohne Dubletten [B].
- **OZ-Frontend / Karten-App** (Google-Maps-Stil, Events als eigene Ebene): erster API-Abnehmer; braucht verlässliche, filterbare Daten mit ehrlichen Genauigkeitsangaben [B].
- **Menschen in Zirndorf:** nur indirekt über die Karte; wollen wissen, was heute/bald in der Nähe passiert, z. B. wann der Weihnachtsmarkt ist [B].
- **Andere Entwickler / OZ-Mitglieder:** freie API-Nutzung; API als Vorlage für eigene Backends [B].

## 3. Stakes / Kontext

- Backend innerhalb von **OpenZirndorf (OZ)**, Open-Source-Community; jedes Mitglied baut eigenes Backend mit frei gewählter Technologie, gemeinsames Frontend bindet ein [B][A].
- Ein-Personen-Projekt (Andreas), Software gehört Andreas, er „trägt die Verantwortung“ [B].
- Ausdrücklich kein technischer Vorsprung: „und das ist in Ordnung“ [B].
- Es gibt einen „öffentlichen Start“ (Lizenzentscheidung muss davor fallen) [B][A] — Termin nicht genannt.
- Für OZ gibt es noch keine gemeinsamen Schnittstellenregeln [B].

## 4. Kernfähigkeiten (FR-Kandidaten)

**Datenbestand**
- Event mit Bezeichnung, Typ, Zeitraum, Adresse + Koordinaten, Quelle, optional Ablaufplan (mehrere Einträge) und Notiz [B].
- Explizite Speicherung der Sicherheit von Zeit- und Ortsangaben (z. B. „Uhrzeit unbekannt“, „ganztägig“, „nur Ortsteil-Zentrum“) [B].
- Wiederkehrende Termine „fest vorgesehen“ (Stadtratssitzung, DigitalCafé) [B].
- Stammkoordinaten für häufige Orte (Marktplatz, Paul-Metz-Halle) → gleicher Ort = gleicher Kartenpunkt [B].

**Öffentliche API (nur lesend)**
- Filter nach Typ und Zeitraum; ohne Filter: Events des heutigen Tages [B][A].
- Antworten ermöglichen Gruppierung nach Standort (stabile Koordinaten) [B][A].
- Status/Zeitbezug: läuft gerade / beginnt bald / vorbei [A].
- Archiv-Endpunkt für vergangene Events [B].
- Pro Event erkennbar: Sicherheit von Zeit und Ort, Herkunft [B].

**Admin**
- Login (ein Nutzer), Anlegen und Bearbeiten von Events [B].
- JSON-Import mit Duplikaterkennung, Warnung und Korrekturmöglichkeit [B].
- Wiederkehrende Events gelten nicht als Duplikat [B][A].

**Archivierung**
- Täglicher Cron-Job verschiebt vergangene Events ins Archiv [A].

## 5. Datenmodell-Hinweise

- **Event:** Bezeichnung, Typ, Zeitraum (Start/Ende), Adresse, Koordinaten, Quelle, Notiz, optional Timetable [B].
- **Timetable-Eintrag** (Entwurf): `description`, `startTime`, `endTime` [A].
- **Ort:** Brief: kein eigenes Objekt „Veranstaltungsort“ nötig, Adresse genügt, plus Stammkoordinaten [B]. Addendum-Beobachtung dagegen: Paul-Metz-Halle ~20× identisch → „spricht dafür, Veranstaltungsorte als eigenes Objekt zu führen“ [A]. **Widerspruch, im PRD auflösen.**
- **Genauigkeit:** Zeit- und Ortssicherheit als eigene Angaben, nicht Freitext [B]; heute im Entwurf: `00:00` = unbekannte Uhrzeit, ganztägig = `00:00–23:59`, fehlendes Ende = `null` → „unbekannte Uhrzeit“ nicht von „Mitternacht“ unterscheidbar [A]. Ortsgenauigkeit nur als Freitext in `address`/`note` [A].
- **Quelle:** eigenes Feld statt Text im Namen (z. B. `[nicht im städt. Kalender, Quelle: eventim/songkick]`) [A]; Quellenangabe pro Event auch lizenzrechtlich nötig [B][A].
- **Typ/Kategorie:** fehlt im Entwurf; Feste, Kultur, politische Sitzungen nicht unterscheidbar [A]. Typenliste nicht definiert.
- **IDs:** fehlen im Entwurf → Re-Import kann neu/aktualisiert/doppelt nicht unterscheiden [A].
- **Wiederkehrend:** im Entwurf als Einzeleinträge (DigitalCafé, Seniorentanznachmittag, Stadtratssitzungen) [A]; Modellierung (Serie vs. Einzeltermine) offen.
- **Archivstatus:** vergangen = Ende überschritten; ohne Ende gilt Starttag 23:59:59 [A] (Annahme: Europe/Berlin).
- **Entwurfsdatei** `zirndorf_events.json` (Stand 2026-09-18): Felder `name`, `startTime`, `endTime`, `location { address, latitude, longitude, note? }`, `timetable[]`; Top-Level-Metadaten `generated`, `source_note`, `limitations_note`; 42 Events Mai 2026–Dez 2027, inkl. bereits vergangener [A].

## 6. API-/Schnittstellen-Aussagen

- Öffentlich, jeder darf sie nutzen; REST; nur lesend [B].
- Standardabfrage = heute; Filter Typ + Zeitraum [B][A].
- Eigener Archiv-Endpunkt [B][A].
- Kartengerechte Antworten: stabile Koordinaten für Gruppierung, Status läuft/bald/vorbei [A].
- OpenAPI-Dokumentation, gut genug als Vorlage für andere OZ-Backends [B].
- Projekt kann OZ-Konventionen prägen: Format, Versionierung, Dokumentation, CORS [B].
- Import-Schnittstelle: festgelegtes, **versioniertes JSON-Format**, von Andreas auf Basis der Testsammlung definiert [B].
- Später: OParl beim Ratsinformationssystem prüfen (nicht v1) [B].

## 7. Admin / Pflege / Import

- Ein Admin (Andreas), Login [B].
- Manuelles Anlegen/Bearbeiten [B].
- JSON-Import; Testsammlung (42 Events) muss vollständig importierbar sein; zweiter Import erzeugt keine stillen Dubletten [B].
- Duplikatregeln [A]: gleicher Name an anderem Datum ≠ Duplikat bei wiederkehrenden Terminen; Schlüsselkandidat Name + Beginn + Ort (im PRD/Architektur festzulegen); Duplikate nicht stillschweigend verwerfen, sondern warnen + korrigierbar.
- Stammkoordinaten-Pflege für häufige Orte [B] (Pflegeweg nicht beschrieben).

## 8. Nicht-funktionale Hinweise

- **Betrieb:** railway.app [B][A]; täglicher Cron für Archivierung [A].
- **Aufbewahrung:** Archiv vorerst dauerhaft, keine Löschfrist [A].
- **Zeitzone:** Annahme Europe/Berlin [A].
- **Sicherheit:** Admin-Login; öffentliche API ohne Schreibzugriff [B]. Weiteres nicht spezifiziert.
- **Datenschutz:** in den Quellen nicht behandelt.
- **Performance/Verfügbarkeit:** in den Quellen nicht behandelt.
- **CORS:** als Teil der OZ-Konvention genannt [B].
- **Lizenz/Urheberrecht:** Code- und Datenlizenz vor öffentlichem Start zu entscheiden [B]; Optionen Code: MIT, Apache-2.0, AGPL-3.0; Daten: CC BY 4.0, DL-DE/BY-2.0, ODbL [A]. Timetables sind paraphrasiert aus Dritt-Websites; Kirchweih-PDF per robots.txt für automatisierten Abruf gesperrt [A]. „Entscheidungshilfe, keine Rechtsberatung“ [A].

## 9. Scope / MVP-Schnitt

**In v1** [B]: Datenmodell inkl. Genauigkeitsangaben; wiederkehrende Events; Stammkoordinaten; öffentliche lesende REST-API (Typ/Zeitraum, Default heute); Archiv-Endpunkt; Admin für einen Nutzer mit Login, CRUD, JSON-Import mit Duplikatmeldung; versioniertes Import-Format; Betrieb auf railway.app.

**Nicht in v1** [B]: Crawling; OZ-Frontend/Karten-App; mehrere Admins, Rollen, Bürger-Einreichungen; Schreibzugriff über öffentliche API.

**Ausblick** [B]: weitere Pfleger, Vereins-Einreichungen, halbautomatische Übernahme (z. B. OParl).

## 10. Erfolgskriterien

- Nordstern: „Andreas öffnet die Karte und sieht, wann und wo der Weihnachtsmarkt stattfindet.“ [B] (hängt vom Frontend ab, das out of scope ist)
- 42-Event-Testsammlung vollständig importierbar; Zweitimport ohne stille Dubletten [B].
- Bei jedem Event in der API: Sicherheit von Zeit und Ort sowie Herkunft erkennbar [B].
- OpenAPI-Doku taugt als Vorlage für andere OZ-Backends [B].
- Vollständigkeits-Gradmesser, z. B. „alle bekannten Zirndorfer Events der nächsten drei Monate sind erfasst“ — Zielwert fehlt [B].

## 11. Offene Fragen & Annahmen

- Lizenz für Code und für Daten; Rechte an paraphrasierten Dritt-Inhalten [B][A].
- OZ-Standard: mit Community abstimmen oder von Andreas setzen und übernehmen lassen? [B]
- OParl bei sitzung.zirndorf.de prüfen (später) [B].
- Duplikat-Schlüssel (Name + Beginn + Ort?) [A].
- Cron-Lücke: Soll die Standardabfrage bereits vergangene, noch nicht archivierte Events zusätzlich herausfiltern? [A]
- Ort als eigenes Objekt vs. Adresse + Stammkoordinaten (Widerspruch Brief/Addendum) [B][A].
- Modellierung wiederkehrender Termine (Regel vs. Einzeleinträge) — nicht festgelegt.
- Typ-/Kategorienliste — nicht festgelegt [A].
- Definition „bald beginnend“ (Zeitfenster) — nicht festgelegt.
- Stabile IDs im Import-Format / Update-Semantik bei Re-Import [A].
- Vollständigkeits-Zielwert [B].
- Annahme: Zeitzone Europe/Berlin [A].
- Annahme: Archivierung ohne Löschfrist „vorerst“ [A].

## 12. Technische Entscheidungen (→ PRD-Addendum/Architektur, nicht ins PRD)

- Hosting railway.app [B][A].
- Archivierung per täglichem Cron-Job, Verschieben ins Archiv [A].
- Vergangen-Regel: Ende überschritten bzw. Starttag 23:59:59 [A].
- REST + OpenAPI [B].
- Technologiewahl frei (OZ-Prinzip), im Brief nicht festgelegt [B][A].
- Struktur des Datenentwurfs `zirndorf_events.json` als Ausgangspunkt fürs Import-Format [A].

## 13. Qualitative Aspekte (leicht verlierbar)

- **Ehrlichkeit vor Scheingenauigkeit:** Kernwert; „Ehrliche Angaben“ ist eines von drei Alleinstellungsmerkmalen. Unschärfe zeigen statt verstecken [B].
- **Wert durch Pflege und Ortskenntnis**, nicht durch Technik: „Wer sich vor Ort auskennt, weiß, dass es die Lindner Kärwa gibt“ [B].
- **Bescheidene Haltung:** kein technischer Vorsprung, „und das ist in Ordnung“ [B].
- **Community-Beitrag:** Vorlagencharakter für OZ; offene Frage, ob Standard gemeinsam abgestimmt oder vorgelebt wird — Ton von Angebot, nicht Vorgabe [B].
- **Offenheit:** „Jeder darf sie nutzen“; API für andere Entwickler frei [B].
- **Verantwortung & Eigentum:** Code gehört Andreas, trotzdem Einfügen in öffentlichen OZ-Bestand; Respekt vor Rechten Dritter (Quellenangabe, robots.txt) [B][A].
- **Konkrete, lokale Nutzerperspektive:** Weihnachtsmarkt, Kirchweih, Fischerfest — Alltagsfragen „was ist heute/bald in meiner Nähe“ [B].
- **Keine stillen Eingriffe:** Duplikate nie stillschweigend verwerfen; Pfleger behält Kontrolle [A].
