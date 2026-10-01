---
title: "Product Brief: OZ Zirndorf Event Store"
status: final
created: 2026-10-01
updated: 2026-10-01
---

# Product Brief: OZ Zirndorf Event Store

## Zusammenfassung

Der **OZ Zirndorf Event Store** sammelt alle Veranstaltungen in Zirndorf an einem Ort: Feste und Kirchweihen, Vorstellungen in der Paul-Metz-Halle, Bürgerversammlungen und Stadtratssitzungen. Er speichert jedes Event mit Ort, Zeitraum und optionalem Ablaufplan und stellt die Daten über eine öffentliche REST-API bereit. Gepflegt werden sie in einer schlanken Admin-Oberfläche, entweder einzeln von Hand oder per JSON-Import.

Das Projekt ist ein Backend von **OpenZirndorf (OZ)**, einer Open-Source-Community in Zirndorf. Bei OZ baut jedes Mitglied sein Backend selbst und wählt die Technologie frei. Ein gemeinsames Frontend bindet die Backends ein. Der erste Abnehmer ist eine Karten-App im Stil von Google Maps, die Events als eigene Ebene zeigt.

Für OZ gibt es noch keine gemeinsamen Regeln, wie Backends ihre Schnittstellen gestalten. Dieser Event Store kann deshalb **zur Vorlage für weitere OZ-Backends werden**.

## Das Problem

Wer wissen will, was in Zirndorf los ist, muss viele Quellen absuchen:
- den städtischen Veranstaltungskalender, der Lücken hat, weil das Kulturamt manche Monate erst spät veröffentlicht
- das Ratsinformationssystem, das sich schlecht automatisch abrufen lässt
- die Kirchweih-Seite der Stadt
- Vereinsseiten, Ticketportale und Aushänge auf Papier

Ortsteil-Kirchweihen oder das Fischerfest stehen im städtischen Kalender gar nicht. Eine Gesamtübersicht gibt es nicht, schon gar nicht auf einer Karte.

Andreas' Testsammlung zeigt, wie unscharf diese Daten sind. Für 42 Events aus mindestens sechs Quellen gilt:
- Oft fehlt die Uhrzeit oder das Ende.
- Für Feste ist häufig nur der Ortsteil bekannt, nicht der Festplatz.
- Die Herkunft eines Termins steht nur als Freitext im Namen.

Ein Datenbestand, der diese Unschärfe verschweigt, setzt auf der Karte Pins, die exakter wirken, als sie sind.

## Die Lösung

Ein Backend-Service mit drei Teilen:

1. **Datenbestand für Events.** Jedes Event hat:
   - eine Bezeichnung
   - einen Typ
   - einen Zeitraum
   - eine Adresse und Koordinaten
   - eine Quelle
   - optional einen Ablaufplan (Timetable) mit mehreren Einträgen und eine Notiz

   Wie sicher eine Angabe ist, wird ausdrücklich gespeichert, zum Beispiel „Uhrzeit unbekannt“, „ganztägig“ oder „nur Ortsteil-Zentrum“. Wiederkehrende Termine wie Stadtratssitzungen oder das DigitalCafé sind fest vorgesehen.
2. **Öffentliche REST-API.** Jeder darf sie nutzen. Sie filtert nach Typ und Zeitraum und liefert ohne Filter die Events des heutigen Tages. Die Antworten enthalten alles, was eine Karte braucht:
   - Events nach Standort gruppieren
   - laufende und demnächst beginnende Events von anderen unterscheiden

   Vergangene Events wandern in ein Archiv, das über einen eigenen Endpunkt abrufbar ist.
3. **Admin-Oberfläche.** Events lassen sich von Hand anlegen und bearbeiten oder als JSON importieren. Beim Import erkennt das System mögliche Duplikate, warnt davor und lässt sie korrigieren. Wiederkehrende Events wie die monatliche Stadtratssitzung zählen dabei nicht als Duplikat.

   Für häufige Orte wie Marktplatz oder Paul-Metz-Halle gibt es **Stammkoordinaten**. Derselbe Ort bekommt dadurch immer denselben Punkt auf der Karte. Ein eigenes Objekt „Veranstaltungsort“ ist dafür nicht nötig, die Adresse genügt.

## Was das Projekt besonders macht

Das Projekt hat keinen technischen Vorsprung, und das ist in Ordnung. Sein Wert entsteht auf drei Wegen:

- **Pflege:** Jemand trägt die verstreuten Termine tatsächlich zusammen und hält sie aktuell.
- **Ortskenntnis:** Wer sich vor Ort auskennt, weiß, dass es die Lindner Kärwa gibt, obwohl sie in keinem städtischen Kalender steht.
- **Ehrliche Angaben:** Der Datenbestand zeigt, wie genau Zeit und Ort eines Events bekannt sind.

Dazu kommt ein Nutzen für die ganze Community: Das Projekt kann festlegen, wie OZ-Backends Daten bereitstellen, also Format, Versionierung, Dokumentation und CORS.

## Für wen

- **Andreas als Pfleger:** In v1 der einzige Admin. Er braucht einen schnellen Weg, recherchierte Termine aus einer JSON-Datei zu übernehmen, ohne Dubletten anzulegen.
- **Das OZ-Frontend:** Die Karten-App ist der erste Abnehmer der API. Sie braucht verlässliche, filterbare Daten mit ehrlichen Angaben zur Genauigkeit.
- **Menschen in Zirndorf:** Sie nutzen den Service nur indirekt über die Karte. Sie wollen wissen, was heute oder bald in ihrer Nähe passiert und wann zum Beispiel der Weihnachtsmarkt ist.
- **Andere Entwickler:** Sie dürfen die API frei nutzen, zum Beispiel für weitere OZ-Projekte oder eigene Anwendungen.

## Erfolgskriterien

- **Das wichtigste Kriterium:** Andreas öffnet die Karte und sieht, wann und wo der Weihnachtsmarkt stattfindet.
- Die Testsammlung mit 42 Events lässt sich vollständig importieren. Ein zweiter Import erzeugt keine stillen Dubletten.
- Bei jedem Event in der API ist erkennbar, wie sicher Zeit und Ort sind und woher es stammt.
- Die API ist mit OpenAPI so gut dokumentiert, dass andere OZ-Mitglieder sie als Vorlage für ihre Backends nehmen können.
- Es gibt einen Gradmesser für Vollständigkeit, zum Beispiel „alle bekannten Zirndorfer Events der nächsten drei Monate sind erfasst“. Einen Zielwert gibt es noch nicht.

## Umfang

**In v1:**
- Datenmodell für Events mit Typ, Zeitraum, Ort (Adresse und Koordinaten), Quelle, optionalem Ablaufplan und Notiz. Für Zeit und Ort wird jeweils gespeichert, wie sicher die Angabe ist.
- Wiederkehrende Events
- Stammkoordinaten für häufige Orte
- Öffentliche REST-API, die nur Daten ausgibt. Sie filtert nach Typ und Zeitraum und zeigt ohne Filter den heutigen Tag.
- Archiv-Endpunkt für vergangene Events
- Admin-Oberfläche für einen einzelnen Nutzer mit Login, Anlegen und Bearbeiten von Events sowie JSON-Import. Mögliche Duplikate werden gemeldet und lassen sich korrigieren.
- Ein festgelegtes, versioniertes JSON-Format für den Import. Andreas definiert es auf Basis seiner Testsammlung.
- Betrieb auf railway.app

**Nicht in v1:**
- Automatisches Einsammeln von Events aus Websites (Crawling)
- Das OZ-Frontend und die Karten-App selbst
- Mehrere Admins, Rollen, Einreichungen durch Bürger
- Schreibender Zugriff über die öffentliche API

## Offene Fragen

- **Lizenz:** Die Software gehört Andreas, und er trägt die Verantwortung. Trotzdem soll sie sich in den öffentlichen OZ-Bestand einfügen. Eine Open-Source-Lizenz ändert nichts daran, wem der Code gehört. Sie legt nur fest, was andere damit tun dürfen. Vor dem öffentlichen Start braucht es zwei Entscheidungen:
  - eine Lizenz für den Code
  - eine Lizenz für die Event-Daten

  Dazu kommt eine Quellenangabe pro Event, weil ein Teil der Daten aus fremden Seiten nachformuliert ist. Optionen stehen im Addendum.
- **Gemeinsamer Standard für OZ:** Soll das API-Format mit der OZ-Community abgestimmt werden? Oder setzt Andreas es fest, und andere übernehmen es, wenn es ihnen passt?
- **Ratsinformationssystem:** Prüfen, ob sitzung.zirndorf.de eine OParl-Schnittstelle anbietet. Das ist für später interessant, nicht für v1.

## Ausblick

Auf lange Sicht wird der Event Store zur **verlässlichen Terminquelle für Zirndorf**, und die OZ-Karte ist ihr sichtbarstes Gesicht. Mögliche nächste Schritte:
- weitere Pfleger aus der Community
- Einreichungen von Vereinen
- halbautomatische Übernahme aus Quellen mit Schnittstellen (z. B. OParl)

Unabhängig davon prägt das Projekt, wie OZ-Backends ihre Daten bereitstellen.
