---
title: "Addendum: OZ Zirndorf Event Store"
created: 2026-10-01
updated: 2026-10-01
---

# Addendum: OZ Zirndorf Event Store

Detailwissen für PRD und Architektur, das nicht in den Brief gehört.

## Technischer Rahmen

- **OZ-Architektur:** Bei OpenZirndorf baut jedes Mitglied sein Backend selbst und wählt die Technologie frei. Ein gemeinsames Frontend bindet die Backends ein. Dieses Frontend ist nicht Teil dieses Projekts.
- **Hosting:** Andreas betreibt seine Anwendungen derzeit auf railway.app.

## Was die API für die Karte können muss

Andreas' Vorstellung vom Frontend: Jedes Event ist ein Pin auf der Karte. Pins am selben Standort werden möglichst zusammengefasst, und laufende oder bald beginnende Events sind optisch hervorgehoben. Für die API heißt das:
- Sie liefert stabile Koordinaten, damit sich Events nach Standort gruppieren lassen.
- Sie liefert einen Status oder Zeitbezug, aus dem hervorgeht, ob ein Event gerade läuft, bald beginnt oder vorbei ist.
- Sie filtert nach Typ und Zeitraum und zeigt ohne Filter den heutigen Tag.

## Regeln für Duplikate beim Import

- Gleicher Name an einem anderen Datum ist **kein** Duplikat, wenn es sich um einen wiederkehrenden Termin handelt (Stadtratssitzung, DigitalCafé).
- Ein Kandidat für einen Schlüssel ist die Kombination aus Name, Beginn und Ort. Das muss im PRD bzw. in der Architektur festgelegt werden.
- Duplikate werden nicht stillschweigend verworfen. Das System warnt, und Andreas kann sie korrigieren.

## Lizenzoptionen (Entscheidungshilfe, keine Rechtsberatung)

Wem der Code gehört, ändert sich durch keine dieser Lizenzen. Eine Lizenz regelt nur, was andere mit Code oder Daten tun dürfen.

**Code**
- **MIT** oder **Apache-2.0**: Jeder darf den Code nutzen und verändern, auch für kommerzielle Zwecke. Es muss nur der Lizenzhinweis erhalten bleiben. Apache-2.0 enthält zusätzlich eine Patentklausel.
- **AGPL-3.0**: Wer den Code verändert und als Dienst im Netz betreibt, muss seine Änderungen ebenfalls offenlegen. Das ist sinnvoll, wenn niemand eine geschlossene Kopie betreiben soll.

**Daten**
- **CC BY 4.0** oder **DL-DE/BY-2.0** (die Datenlizenz Deutschland, die im öffentlichen Sektor üblich ist): Jeder darf die Daten nutzen, muss aber die Quelle nennen.
- **ODbL**: Wer die Daten verändert und weitergibt, muss sie unter derselben Lizenz weitergeben. OpenStreetMap nutzt diese Lizenz.
- Termine, die aus Dritt-Websites nachformuliert sind, sollten eine Quellenangabe pro Event tragen. Ob Andreas sie unter eigener Lizenz weitergeben darf, muss er selbst klären.

Die Entscheidung trifft Andreas vor dem öffentlichen Start. Zuständig ist das PRD bzw. eine eigene Entscheidung.

## Archiv

Vergangene Events werden aus der Standardabfrage herausgenommen und sind über einen eigenen Archiv-Endpunkt abrufbar. Entschieden am 2026-10-01:
- **Wann ein Event vergangen ist:** wenn sein Ende überschritten ist. Fehlt das Ende, gilt das Ende des Starttags (23:59:59) als Ende. [ASSUMPTION: gemeint ist die Ortszeit Europe/Berlin]
- **Wie archiviert wird:** Ein Cron-Job verschiebt vergangene Events einmal täglich ins Archiv.
- **Wie lange Events bleiben:** Vorerst dauerhaft. Es gibt keine Löschfrist.

Folge für das PRD: Zwischen zwei Cron-Läufen kann ein Event noch in der Standardabfrage auftauchen, obwohl es schon vorbei ist. Es muss festgelegt werden, ob das in Ordnung ist oder ob die Standardabfrage solche Events zusätzlich herausfiltert.

## Erster Datenentwurf (zirndorf_events.json, Stand 2026-09-18)

Struktur pro Event: `name`, `startTime`, `endTime` (oft `null`), `location { address, latitude, longitude, note? }`, optional `timetable[] { description, startTime, endTime }`. Die Datei hat Metadaten auf oberster Ebene: `generated`, `source_note` und `limitations_note`. Der Entwurf enthält 42 Events von Mai 2026 bis Dezember 2027.

Quellen im Entwurf:
- städtischer Veranstaltungskalender (zirndorf.de)
- Ratsinformationssystem (sitzung.zirndorf.de, eine zustandsbehaftete ASP-Anwendung, die sich schlecht automatisch abrufen lässt)
- die Kirchweih-Seite der Stadt
- festivalsindeutschland.de
- eventim/songkick
- Koordinaten aus Google Places

### Beobachtungen am Entwurf (offen für PRD/Architektur)

- **Unschärfe bei der Zeit:** Fehlt die Uhrzeit, steht im Entwurf `00:00`. Ganztägige Mehrtagesfeste sind als `00:00`–`23:59` kodiert, und ein fehlendes Ende als `endTime: null`. Damit lässt sich „unbekannte Uhrzeit“ nicht von „Mitternacht“ unterscheiden.
- **Unschärfe beim Ort:** Ob eine Koordinate genau ist, steht nur als Freitext in `address` oder `note`, zum Beispiel „Ortsteil-Zentrum, kein exakter Festplatz bekannt“, „Festmeile, kein Einzelpunkt“ oder „nächstgelegener bekannter Punkt“.
- **Herkunft steckt im Namen:** Hinweise wie `[nicht im städt. Kalender, Quelle: eventim/songkick]` stehen in `name` statt in einem eigenen Feld.
- **Keine IDs:** Wird die Datei erneut importiert, lässt sich nicht erkennen, ob ein Event neu ist, aktualisiert wurde oder doppelt vorkommt.
- **Wiederkehrende Termine:** Formate wie DigitalCafé, Seniorentanznachmittag und Stadtratssitzungen stehen als Einzeleinträge in der Datei.
- **Wiederholte Orte:** Die Paul-Metz-Halle kommt etwa 20-mal mit identischer Adresse und Koordinate vor. Das spricht dafür, Veranstaltungsorte als eigenes Objekt zu führen.
- **Keine Kategorie:** Feste, Kultur und politische Sitzungen sind nicht unterscheidbar. Für Filter in einer Karten-App wäre das relevant.
- **Vergangene Events:** Die Datei enthält auch Events aus Mai bis September 2026, die bereits vorbei sind.
- **Urheberrecht:** Die Timetables sind von Dritt-Websites paraphrasiert, und das Kirchweih-PDF ist per robots.txt für automatisierten Abruf gesperrt. Das ist relevant, sobald die Daten öffentlich und offen lizenziert ausgeliefert werden.
