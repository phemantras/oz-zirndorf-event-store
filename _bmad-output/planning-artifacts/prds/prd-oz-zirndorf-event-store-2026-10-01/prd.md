---
title: "PRD: OZ Zirndorf Event Store"
status: final
created: 2026-10-01
updated: 2026-10-02
---

# PRD: OZ Zirndorf Event Store

## 0. Zweck des Dokuments

Dieses PRD beschreibt, **was** der OZ Zirndorf Event Store in Version 1 können muss. Es richtet sich an Andreas als Entwickler und Pfleger, an spätere Architektur- und Story-Arbeit und an andere OpenZirndorf-Mitglieder, die die API nutzen oder als Vorlage nehmen wollen. Es baut auf dem Product Brief vom 2026-10-01 auf (`_bmad-output/planning-artifacts/briefs/brief-oz-zirndorf-event-store-2026-10-01/`) und wiederholt dessen Problembeschreibung nicht. Begriffe sind im Glossar (§3) festgelegt und werden überall genau so verwendet. Anforderungen sind nach Features gruppiert und durchgehend nummeriert (FR-1 …). Technische Umsetzungsfragen stehen im `addendum.md`.

## 1. Vision

Der OZ Zirndorf Event Store sammelt, was in Zirndorf stattfindet: Kirchweihen und Feste, Märkte, Vorstellungen in der Paul-Metz-Halle, Vereinstreffen, Stadtratssitzungen. Auf lange Sicht soll er eine verlässliche Terminquelle für Zirndorf werden. Andreas pflegt die Events von Hand oder per JSON-Import. Eine öffentliche, nur lesende REST-API stellt sie jedem bereit, allen voran der OZ-Karten-App, die Events als eigene Ebene zeigt.

Der Wert entsteht nicht durch Technik, sondern durch Ortskenntnis, Pflege und **ehrliche Angaben**. Viele Zirndorfer Termine sind unscharf: Die Uhrzeit fehlt, das Ende ist offen, bei einem Fest ist nur der Ortsteil bekannt. Der Event Store verschweigt das nicht. Er speichert ausdrücklich, wie genau Zeit und Ort sind und woher ein Event stammt, damit die Karte keine Pins setzt, die genauer wirken, als sie sind.

Nebenbei soll das Projekt zeigen, wie ein OZ-Backend seine Schnittstelle gestalten kann: versioniert, dokumentiert mit OpenAPI, offen per CORS, mit klaren Konventionen (§6). Das ist ein Angebot an die Community, keine Vorgabe: Andreas lebt das Muster vor, andere können es übernehmen.

## 2. Zielnutzer

### 2.1 Jobs To Be Done

- **Andreas (Admin):** „Wenn ich Termine recherchiert habe, will ich sie schnell und ohne Duplikate in den Bestand bringen und Fehler einzeln korrigieren können.“
- **OZ-Frontend / Karten-App (Abnehmer):** „Ich will mit einer einfachen Abfrage die Events von heute oder einem Zeitraum bekommen, nach Ort gruppierbar, mit der Angabe, wie genau Zeit und Ort sind.“
- **Menschen in Zirndorf (indirekt, über die Karte):** „Ich will sehen, was heute oder bald in meiner Nähe los ist, und wann zum Beispiel der Weihnachtsmarkt stattfindet.“
- **OZ-Mitglieder und andere Entwickler:** „Ich will die API frei nutzen und mir für mein eigenes Backend abschauen, wie man so eine Schnittstelle sauber aufbaut.“

### 2.2 Nicht-Nutzer (v1)

- Vereine, Veranstalter oder Bürger, die selbst Events eintragen wollen. In v1 gibt es keine Einreichung und nur einen Admin.
- Endnutzer, die direkt mit dem Event Store arbeiten. Sie sehen Events nur über ein Frontend.

### 2.3 Zentrale User Journeys

- **UJ-1. Andreas trägt eine neue Recherche ein.** Andreas hat 15 neue Termine aus Vereinsseiten und dem Ratsinformationssystem zusammengetragen. Er lädt die JSON-Datei im Admin hoch. Der Import zeigt ihm: 12 neu, 1 aktualisiert, zweimal Duplikatverdacht. Er sieht, dass ein Duplikatverdacht tatsächlich derselbe Termin ist, und überspringt ihn. Der andere ist eine zweite Vorstellung mit gleichem Titel am selben Tag in der Paul-Metz-Halle (Nachmittag und Abend). Die legt er als neues Event an. Danach stehen alle Termine im Bestand, ohne stille Duplikate. **Sonderfall:** Ein Eintrag hat kein gültiges Datum. Der Import meldet genau diesen Eintrag als Fehler, Andreas korrigiert ihn und lädt die Datei erneut hoch.
- **UJ-2. Die Karten-App zeigt den heutigen Tag.** Die Karten-App ruft die Events ohne Filter ab und bekommt alle Events von heute, die noch nicht vorbei sind. Drei Events in der Paul-Metz-Halle haben denselben Ort und landen als ein gruppierter Pin auf der Karte. Bei der Lindner Kärwa ist nur der Ortsteil bekannt. Die Karte kann das kenntlich machen, weil die API es mitliefert.
- **UJ-3. Eine Entwicklerin baut ein neues OZ-Backend.** Sie öffnet die OpenAPI-Dokumentation des Event Stores, sieht Versionierung, Fehlerformat und Filter-Konventionen und übernimmt das Muster für ihren eigenen Dienst.

## 3. Glossar

- **Event** — Eine Veranstaltung an einem Termin. Hat genau einen Ort, genau einen Event-Typ, einen Beginn, optional ein Ende, je eine Zeitgenauigkeit für Beginn und Ende, eine Quelle und optional eine Notiz und einen Ablaufplan. Wiederkehrende Termine sind jeweils eigene Events.
- **Ort** — Ein Veranstaltungsort mit Name, Adresse, Koordinaten, Ortsgenauigkeit und optionaler Notiz (z. B. „Paul-Metz-Halle“, „Marktplatz“, „Ortsteil Weinzierlein“). Ein Ort gehört zu beliebig vielen Events.
- **Adresse** — Teil eines Orts, aufgeteilt in Straße (mit Hausnummer, falls vorhanden), Postleitzahl und Ort (Gemeinde oder Ortsteil, z. B. „Zirndorf“).
- **Event-Typ** — Kategorie eines Events aus einer festen Liste (§4.1).
- **Zeitgenauigkeit** — Angabe, wie genau ein Zeitpunkt bekannt ist (z. B. „exakt“, „nur Datum“, „ganztägig“). Beginn und Ende eines Events haben je eine eigene Zeitgenauigkeit.
- **Ortsgenauigkeit** — Angabe, wie genau die Koordinaten eines Orts den tatsächlichen Veranstaltungsplatz treffen (z. B. „Gebäude“, „Platz/Straße“, „Bereich“, „nur Ortsteil“).
- **Quelle** — Herkunft der Angaben zu einem Event: Beschreibung und optional ein Link.
- **Ablaufplan** — Geordnete Liste von Programmpunkten eines Events, jeder mit Beschreibung, Beginn und optional Ende.
- **Aktives Event** — Ein Event, das noch nicht vorbei ist. Ein Event ist **vorbei**, wenn sein Ende überschritten ist. Ist beim Ende nur das Datum bekannt, gilt das Ende des End-Tages. Ohne Ende gilt das Ende des Beginn-Tages.
- **Archiv** — Alle Events, die vorbei sind. Ob ein Event zum Archiv gehört, ergibt sich allein aus der Vorbei-Regel, nicht aus dem Zeitpunkt der täglichen Bereinigung. Archivierte Events erscheinen nicht in den regulären Abfragen, nur im Archiv-Zugriff.
- **Import-Datei** — JSON-Datei im versionierten Import-Format, mit der Andreas Events und Orte gesammelt einspielt.
- **Import-Schlüssel** — Optionale, vom Ersteller der Import-Datei frei gewählte, stabile Kennung eines Events. Erkennt beim erneuten Import dasselbe Event wieder.
- **Duplikatverdacht** — Ein importiertes Event ohne passenden Import-Schlüssel, das in Titel, Beginn-Datum und Ort mit einem vorhandenen Event übereinstimmt.
- **Admin** — Die einzige Person mit Schreibrechten in v1 (Andreas).
- **Abnehmer** — Jede Anwendung, die die öffentliche API liest, allen voran die OZ-Karten-App.

## 4. Features

### 4.1 Event-Bestand mit ehrlichen Angaben

**Beschreibung:** Der Kern des Produkts. Jedes Event hält fest, was bekannt ist, und auch, was nicht bekannt ist. Unscharfe Angaben werden nicht durch Platzhalter wie `00:00` versteckt, sondern über Zeitgenauigkeit und Ortsgenauigkeit ausdrücklich gekennzeichnet. Die Herkunft steht in einem eigenen Feld statt im Titel. Wiederkehrende Termine wie Stadtratssitzungen oder das DigitalCafé sind jeweils eigene Events.

#### FR-1: Event-Daten

Ein Event besteht aus Titel, Event-Typ, Ort, Beginn, optionalem Ende, Zeitgenauigkeit für Beginn und Ende, Quelle, optionaler Notiz und optionalem Ablaufplan. Realisiert UJ-1, UJ-2.

**Konsequenzen (testbar):**
- Titel, Event-Typ, Ort, Beginn (mindestens das Datum), Zeitgenauigkeit und Quelle sind Pflichtangaben. Fehlt eine davon, wird das Event abgelehnt.
- Ein Ende vor dem Beginn wird abgelehnt.
- Ein Event enthält keine personenbezogenen Daten (siehe NFR-4).

#### FR-2: Zeitgenauigkeit

Jedes Event kennzeichnet für Beginn und Ende getrennt, wie genau die Zeitangabe ist. Realisiert UJ-2.

**Konsequenzen (testbar):**
- Unterscheidbar sind: Uhrzeit exakt bekannt / nur Datum bekannt (Uhrzeit unbekannt) / ganztägig. Ein unbekanntes Ende wird durch ein leeres Ende ausgedrückt, nicht über einen eigenen Wert.
- „Uhrzeit unbekannt“ ist von „beginnt um Mitternacht“ unterscheidbar.
- Ein Event mit exakt bekanntem Beginn und einem Ende, von dem nur das Datum bekannt ist, ist darstellbar und gilt erst nach Ablauf des End-Tages als vorbei.

#### FR-3: Quelle

Jedes Event nennt seine Quelle als Beschreibung, optional mit Link. Realisiert UJ-2.

**Konsequenzen (testbar):**
- Ein Event ohne Quelle wird abgelehnt. Für eigenes Wissen genügt z. B. „eigene Kenntnis“.
- Die Quelle steht nicht im Titel.

#### FR-4: Ablaufplan

Ein Event kann einen Ablaufplan mit beliebig vielen Programmpunkten haben, jeder mit Beschreibung, Beginn und optionalem Ende.

**Konsequenzen (testbar):**
- Programmpunkte werden chronologisch nach Beginn sortiert ausgeliefert.

#### FR-5: Feste Liste der Event-Typen

Jedes Event hat genau einen Event-Typ aus einer festen Liste. Abnehmer können die Liste abfragen.

**Konsequenzen (testbar):**
- Ein Event mit einem unbekannten Event-Typ wird abgelehnt.
- Die Liste lautet: Fest/Kirchweih, Markt, Kultur/Bühne, Politik/Sitzung, Verein/Treff, Sport, Sonstiges.

**Hinweis:** Eine Änderung der Liste ist eine Änderung am API-Vertrag und folgt der Versionierungsregel (NFR-2).

### 4.2 Orte

**Beschreibung:** Orte werden einmal gepflegt und von Events referenziert. So bekommen alle Events in der Paul-Metz-Halle exakt denselben Kartenpunkt, und Abnehmer können Events nach Ort gruppieren. Die Ortsgenauigkeit gehört zum Ort: „Ortsteil Weinzierlein“ ist als nur ortsteilgenau gekennzeichnet.

#### FR-6: Ort-Daten

Ein Ort besteht aus Name, Adresse, Koordinaten, Ortsgenauigkeit und optionaler Notiz. Realisiert UJ-2.

**Konsequenzen (testbar):**
- Name, Adresse, Koordinaten und Ortsgenauigkeit sind Pflichtangaben. Die Adresse besteht aus Straße, Postleitzahl und Ort. Alle drei sind Pflicht, die Postleitzahl hat genau fünf Ziffern. Fehlt einer der Teile oder die Koordinaten, wird der Ort abgelehnt. Auch ein nur ortsteilgenauer Ort und ein Bereich haben eine vollständige Adresse. Als Straße gilt dann die Straße der Ortsmitte oder eine Bereichsangabe (z. B. „Marktplatz bis Schulsportplatz“). Die Ortsgenauigkeit zeigt, dass die Angabe ungenau ist.
- Die Ortsgenauigkeit kennt die Werte Gebäude / Platz/Straße / Bereich (z. B. Festmeile über mehrere Straßen; die Koordinaten markieren den Mittelpunkt) / nur Ortsteil.
- Die Notiz erklärt Besonderheiten des Orts, z. B. „Eingang über den Hof“ oder „Koordinaten = Mitte der Festmeile“.
- Jeder Ort hat eine stabile Kennung, die sich nicht ändert, wenn Name oder Adresse bearbeitet werden.

#### FR-7: Ort-Referenz statt Kopie

Events verweisen auf einen Ort, statt Adresse und Koordinaten selbst zu tragen.

**Konsequenzen (testbar):**
- Ändert der Admin die Koordinaten eines Orts, liefern danach alle Events dieses Orts die neuen Koordinaten.
- Ein Ort, auf den noch Events (auch archivierte) verweisen, kann nicht gelöscht werden.

### 4.3 Öffentliche Lese-API

**Beschreibung:** Die API ist öffentlich, ohne Anmeldung nutzbar und nur lesend. Sie ist auf die Karte zugeschnitten: Die Standardabfrage liefert „was heute noch los ist“, und Filter erlauben Zeiträume und Event-Typen. Einen berechneten Status wie „läuft gerade“ oder „beginnt bald“ liefert die API nicht. Abnehmer berechnen ihn selbst aus Beginn, Ende und Zeitgenauigkeit. Die API folgt den Konventionen aus §6 und ist Vorlage-tauglich dokumentiert (NFR-3). Realisiert UJ-2, UJ-3.

#### FR-8: Standardabfrage „heute“

Ein Abnehmer, der Events ohne Filter abfragt, erhält alle aktiven Events, deren Zeitraum den heutigen Tag berührt.

**Konsequenzen (testbar):**
- Ein Event, das heute um 14:00 endet, ist ab 14:00 nicht mehr in der Antwort, sondern im Archiv-Zugriff, unabhängig von der täglichen Bereinigung.
- Ein mehrtägiges Event (z. B. Kirchweih Freitag bis Montag) ist an jedem seiner Tage enthalten.
- „Heute“ bezieht sich auf die Zeitzone Europe/Berlin.

#### FR-9: Filter nach Zeitraum und Event-Typ

Ein Abnehmer kann Events nach Zeitraum (von/bis) und nach einem oder mehreren Event-Typen filtern.

**Konsequenzen (testbar):**
- Ein Event ist im Zeitraum enthalten, wenn sich sein Zeitraum mit dem Filterzeitraum überschneidet.
- Mehrere Event-Typen werden mit ODER verknüpft.
- Auch mit Zeitraumfilter liefert die reguläre Abfrage nur aktive Events. Vorbei-Events gibt es nur über den Archiv-Zugriff (FR-12).
- Ungültige Filterwerte werden mit einer verständlichen Fehlermeldung im einheitlichen Fehlerformat abgelehnt (NFR-3).

#### FR-10: Kartengerechte Antwort

Jedes ausgelieferte Event enthält alle Angaben aus FR-1 und den vollständigen Ort (Kennung, Name, Adresse, Koordinaten, Ortsgenauigkeit, Notiz).

**Konsequenzen (testbar):**
- Zwei Events am selben Ort liefern dieselbe Ort-Kennung und identische Koordinaten.
- Zeitgenauigkeit, Ortsgenauigkeit und Quelle sind bei jedem Event in der Antwort enthalten.

#### FR-11: Einzelabruf und Nachschlagelisten

Ein Abnehmer kann ein einzelnes Event über seine Kennung abrufen sowie die Liste aller Orte und aller Event-Typen abfragen.

**Konsequenzen (testbar):**
- Der Abruf einer unbekannten Event-Kennung liefert „nicht gefunden“ im einheitlichen Fehlerformat.
- Ein archiviertes Event ist über seine Kennung weiterhin abrufbar und als archiviert erkennbar. Maßgeblich ist die Vorbei-Regel, nicht der Zeitpunkt der täglichen Bereinigung.

**Out of Scope (gesamte Lese-API):**
- Schreibzugriff jeder Art über die öffentliche API.
- Volltextsuche, Umkreissuche, Paginierung. Bei ein paar hundert Events sind sie nicht nötig.

### 4.4 Archiv

**Beschreibung:** Sobald ein Event vorbei ist, gehört es zum Archiv. Es verschwindet sofort aus den regulären Abfragen und ist sofort über den Archiv-Zugriff abrufbar, es fällt also nie durch eine Lücke. Archivierte Events bleiben dauerhaft erhalten. Eine tägliche Bereinigung räumt den Bestand technisch auf, ändert aber nichts daran, was Abnehmer sehen.

#### FR-12: Archiv-Zugriff

Ein Abnehmer kann archivierte Events abfragen, gefiltert nach Zeitraum und Event-Typ wie in FR-9.

**Konsequenzen (testbar):**
- Archivierte Events haben dieselbe Struktur wie aktive Events (FR-10).
- Ein Event, das seit einer Minute vorbei ist, ist bereits im Archiv-Zugriff enthalten.
- Jedes Event ist zu jedem Zeitpunkt in genau einem der beiden Zugriffe enthalten: in der regulären Abfrage oder im Archiv-Zugriff.

#### FR-13: Tägliche Bereinigung und Aufbewahrung

Das System verschiebt einmal täglich alle Vorbei-Events in den Archiv-Bestand.

**Konsequenzen (testbar):**
- Die Antworten der API sind vor und nach der Bereinigung identisch.
- Archivierte Events werden nicht gelöscht. Es gibt in v1 keine Löschfrist.

### 4.5 Admin-Pflege

**Beschreibung:** Eine schlanke Web-Oberfläche im Browser für genau einen Admin. Hier legt Andreas Events und Orte an, bearbeitet und löscht sie. Realisiert UJ-1.

#### FR-14: Anmeldung

Nur der angemeldete Admin kann Daten ändern.

**Konsequenzen (testbar):**
- Ohne gültige Anmeldung schlägt jeder schreibende Zugriff fehl.
- Es gibt genau ein Admin-Konto. Eine Registrierung gibt es nicht.

#### FR-15: Events und Orte pflegen

Der Admin kann Events und Orte anlegen, bearbeiten und löschen, auch archivierte Events.

**Konsequenzen (testbar):**
- Beim Anlegen und Bearbeiten gelten dieselben Pflicht- und Plausibilitätsregeln wie in FR-1 bis FR-7.
- Beim Anlegen eines Events wählt der Admin einen vorhandenen Ort aus oder legt direkt einen neuen an.
- Beim Anlegen und Bearbeiten eines Orts kann der Admin die Koordinaten auf einer Karte setzen oder verschieben, statt sie von Hand einzutippen.
- Wird ein archiviertes Event so bearbeitet, dass es wieder aktiv ist (z. B. falsches Datum korrigiert), erscheint es wieder in den regulären Abfragen.

### 4.6 JSON-Import

**Beschreibung:** Der schnellste Weg, recherchierte Termine in den Bestand zu bringen. Andreas lädt eine Import-Datei hoch und sieht vor dem Übernehmen, was passieren wird: was neu ist, was aktualisiert wird, was Duplikatverdacht ist und was fehlerhaft ist. Nichts wird stillschweigend verworfen oder verdoppelt. Das Format ist versioniert und von Andreas auf Basis der Testsammlung `zirndorf_events.json` festgelegt. Realisiert UJ-1.

#### FR-16: Versioniertes Import-Format

Die Import-Datei hat eine Formatversion und ist dokumentiert.

**Konsequenzen (testbar):**
- Eine Datei mit unbekannter oder fehlender Formatversion wird mit einer klaren Meldung abgelehnt.
- Das Format kann alle Angaben aus FR-1 bis FR-6 abbilden, einschließlich des optionalen Import-Schlüssels pro Event.
- Ein Event in der Import-Datei kann auf einen vorhandenen Ort verweisen oder einen Ort mitbringen. Ein mitgebrachter Ort mit demselben Namen wie ein vorhandener wird dem vorhandenen zugeordnet.
- Ein mitgebrachter Ort, dessen Name noch nicht existiert, wird als neuer Ort angelegt. Dafür muss er Adresse und Koordinaten enthalten, sonst ist der Eintrag fehlerhaft. Die Import-Vorschau (FR-17) zeigt neu anzulegende Orte gesondert an.

#### FR-17: Import-Vorschau

Vor dem Übernehmen zeigt der Import für jedes Event, was passieren wird: neu, Aktualisierung, Duplikatverdacht oder Fehler.

**Konsequenzen (testbar):**
- Ein Event, dessen Import-Schlüssel schon im Bestand vorkommt, wird als Aktualisierung des vorhandenen Events angezeigt.
- Ein Event ohne passenden Import-Schlüssel, das in Titel, Beginn-Datum und Ort mit einem vorhandenen Event übereinstimmt, wird als Duplikatverdacht angezeigt. Verglichen wird das Beginn-Datum, nicht die Uhrzeit, weil die Uhrzeit oft unbekannt ist.
- Gleicher Titel an einem anderen Datum ist kein Duplikatverdacht (z. B. wiederkehrende Stadtratssitzung).
- Fehlerhafte Einträge werden einzeln mit Grund gemeldet. Sie verhindern nicht den Import der übrigen Einträge.

#### FR-18: Entscheidung bei Duplikatverdacht

Der Admin entscheidet für jeden Duplikatverdacht: überspringen, als neues Event anlegen oder das vorhandene Event überschreiben.

**Konsequenzen (testbar):**
- Ohne Entscheidung des Admins wird kein Duplikatverdacht übernommen.
- Nach dem Import erhält der Admin eine Zusammenfassung mit der Anzahl neuer, aktualisierter, übersprungener und fehlerhafter Einträge.
- Die Testsammlung (42 Events) lässt sich vollständig importieren. Ein zweiter Import derselben Datei erzeugt keine neuen Events.

## 5. Querschnittliche NFRs

- **NFR-1 CORS offen:** Die öffentliche API ist per CORS von jeder Herkunft aus aufrufbar. Die Admin-Funktionen sind es nicht.
- **NFR-2 Versionierung:** Die öffentliche API ist versioniert (Umsetzung: KON-1, KON-2). Inkompatible Änderungen, z. B. an der Event-Typen-Liste oder an Feldnamen, gibt es nur in einer neuen Version. Rückwärtskompatible Ergänzungen (neue optionale Felder) dürfen in der bestehenden Version erfolgen.
- **NFR-3 Vorlage-taugliche Dokumentation:** Die öffentliche API ist vollständig mit OpenAPI beschrieben, einschließlich Filter, Feldbedeutungen (insbesondere Zeit- und Ortsgenauigkeit), Fehlerformat und der Konventionen aus §6. Die Dokumentation ist öffentlich abrufbar.
- **NFR-4 Keine personenbezogenen Daten:** Events und Orte enthalten keine Daten zu natürlichen Personen, also keine Kontaktpersonen, Telefonnummern oder Namen von Privatpersonen. Personenbezogen ist nur das Admin-Konto.
- **NFR-5 Betrieb auf Hobby-Niveau:** Für Antwortzeit und Verfügbarkeit gibt es keine Zielwerte. Es gilt Best Effort.
- **NFR-6 Zeitzone:** Alle Zeitangaben beziehen sich auf Europe/Berlin. Das Format regelt KON-4.

## 6. API-Konventionen

Diese Konventionen sind das Muster, das andere OZ-Backends übernehmen können. Sie gelten für die gesamte öffentliche API und sind Teil der OpenAPI-Dokumentation (NFR-3).

- **KON-1 Version im Pfad:** Jede Ressource liegt unter einer Hauptversion im Pfad, z. B. `/v1/…`.
- **KON-2 Laufzeit alter Versionen:** Erscheint eine neue Hauptversion, bleibt die vorherige mindestens 6 Monate erreichbar. Das Abschaltdatum steht in der Dokumentation.
- **KON-3 Fehlerformat:** Alle Fehler werden einheitlich nach RFC 9457 (Problem Details) ausgeliefert, mit verständlicher Beschreibung auf Englisch.
- **KON-4 Zeitformat:** Beginn und Ende werden als Datum (`JJJJ-MM-TT`) und getrennte, optionale Uhrzeit (`HH:MM`, lokal Europe/Berlin) ausgeliefert. Ist die Uhrzeit unbekannt, ist sie leer (`null`), nie eine erfundene Uhrzeit. Berechnete Zeitpunkte (effektiver Beginn und effektives Ende) werden nach ISO 8601 mit Zeitzonen-Offset ausgeliefert.
- **KON-5 Zeitraumfilter:** Die Parameter `from` und `to` (von/bis, englisch benannt gemäß KON-7) akzeptieren ein Datum oder einen Zeitpunkt und sind beide inklusive. Ein Event ist enthalten, wenn sich sein Zeitraum mit dem Filterzeitraum überschneidet. Fehlt `to`, ist der Zeitraum nach hinten offen.
- **KON-6 Sortierung:** Event-Listen sind nach Beginn aufsteigend sortiert, der Archiv-Zugriff absteigend (neuestes zuerst).
- **KON-7 Technische Sprache Englisch:** Alles Technische ist englisch: Feldnamen in camelCase wie im Entwurf `zirndorf_events.json` (z. B. `startTime`), Parameter, Pfade, Fehlermeldungen und die Codes für Event-Typen, Zeitgenauigkeit und Ortsgenauigkeit (z. B. `market`, `dateOnly`, `district`). Inhalte wie Titel, Notizen und Ortsnamen bleiben deutsch.
- **KON-8 Listenhülle:** Listen werden als Objekt mit einem Datenfeld ausgeliefert, nicht als nacktes Array, damit später Metadaten (z. B. Lizenzhinweis oder Anzahl) ohne Bruch ergänzt werden können. Beispiel: `{ "data": [ … ] }`.

## 7. Rechte und Quellen

- Der Event Store übernimmt Angaben aus fremden Quellen (Stadt, Vereine, Ticketportale). Ablaufpläne sind umformuliert, nicht kopiert. Die Quellenangabe pro Event (FR-3) ist auch aus rechtlicher Sicht nötig.
- Quellen, die einen automatisierten Abruf per `robots.txt` untersagen, werden nicht automatisch abgerufen. In v1 gibt es ohnehin kein Crawling.
- **Die Lizenz für Code und Daten ist offen und blockiert den öffentlichen Start** (siehe Offene Frage 1). Entwickeln und Testen dürfen schon vorher stattfinden.

## 8. Non-Goals

- Der Event Store ist **kein Frontend** und keine Karten-App. Er liefert Daten.
- Er ist **kein Crawler**. Termine kommen von Hand oder per Import-Datei.
- Er ist **keine Community-Plattform**: keine Einreichungen, keine Kommentare, keine Nutzerkonten außer dem Admin.
- Er erhebt **keinen Vollständigkeitsanspruch**. Was nicht gepflegt ist, fehlt. Kuratierung geht vor Masse.
- Er setzt **keinen verbindlichen OZ-Standard**. Er lebt ein Muster vor, das andere übernehmen können.

## 9. MVP-Umfang

### 9.1 In Scope

- Event-Bestand mit Zeitgenauigkeit, Ortsgenauigkeit, Quelle, Ablaufplan und fester Event-Typen-Liste (FR-1 bis FR-5)
- Orte als eigene, referenzierte Objekte (FR-6, FR-7)
- Öffentliche, versionierte, nur lesende API mit Standardabfrage „heute“, Filtern, Einzelabruf und Nachschlagelisten (FR-8 bis FR-11)
- Archiv mit täglicher Archivierung und eigenem Zugriff (FR-12, FR-13)
- Admin-Oberfläche für einen Admin mit Anmeldung, Pflege und JSON-Import mit Vorschau und Duplikatentscheidung (FR-14 bis FR-18)
- API-Konventionen (§6), OpenAPI-Dokumentation, CORS offen
- Betrieb öffentlich erreichbar (Hosting siehe Addendum)

### 9.2 Out of Scope für v1

- Mehrere Admins, Rollen, Einreichungen durch Vereine oder Bürger (Ausblick v2+)
- Halbautomatische Übernahme, z. B. über OParl aus dem Ratsinformationssystem (Ausblick)
- Automatisches Crawlen
- Serienregeln für wiederkehrende Termine („jeden 2. Dienstag“). Jeder Termin ist ein eigenes Event.
- Berechneter Status „läuft gerade“ / „beginnt bald“ in der API
- Export von Events als Import-Datei

## 10. Erfolgskriterien

**Primär**
- **SM-1 Weihnachtsmarkt:** Das wichtigste Kriterium. Der Zirndorfer Weihnachtsmarkt ist im Bestand, und eine Zeitraumabfrage über die Adventszeit liefert ihn mit Zeitraum, Ort, Zeit- und Ortsgenauigkeit und Quelle. Das ist ohne Frontend prüfbar. Das eigentliche Ziel ist erreicht, wenn die OZ-Karten-App ihn anzeigt. Validiert FR-8 bis FR-10.
- **SM-2 Import-Tauglichkeit:** Die Testsammlung mit 42 Events, umgestellt auf das Import-Format v1, wird vollständig importiert. Ein zweiter Import derselben Datei erzeugt 0 zusätzliche Events. Validiert FR-16 bis FR-18.

**Sekundär**
- **SM-3 Ehrliche Angaben:** 100 % der ausgelieferten Events tragen Zeitgenauigkeit für Beginn und Ende, Ortsgenauigkeit und Quelle. Validiert FR-2, FR-3, FR-6, FR-10.
- **SM-4 Vorlage:** Die OpenAPI-Dokumentation taugt als Vorlage. Ein OZ-Mitglied bestätigt nach Durchsicht, dass es Versionierung, Fehlerformat, Zeitformat und Filterkonventionen ohne Rückfrage übernehmen könnte. Validiert NFR-3, §6.
- **SM-5 Abdeckung:** Alle bekannten Zirndorfer Events der nächsten drei Monate sind erfasst. Zielwert und Messweg offen (Offene Frage 3).
- **SM-6 Ortskenntnis:** Der Bestand enthält Events, die im städtischen Kalender fehlen (z. B. Ortsteil-Kirchweihen, Fischerfest). Gemessen wird ihre Anzahl. Zielwert offen (Offene Frage 3).

**Gegenmetriken (nicht optimieren)**
- **SM-C1 Scheinpräzision:** Ein höherer Anteil von Events mit „exakter“ Zeit oder „Gebäude“-Genauigkeit ist kein Ziel an sich. Genauigkeitswerte dürfen nie geraten werden, um die Karte schöner aussehen zu lassen. Gegengewicht zu SM-5 und SM-6.
- **SM-C2 Masse:** Die Anzahl der Events ist kein Erfolgsmaß. Ein ungeprüfter Massenimport ist schlechter als weniger, dafür gepflegte Events. Gegengewicht zu SM-5 und SM-6.

## 11. Offene Fragen

1. **Lizenz für Code und Daten**, einschließlich der Rechte an umformulierten Ablaufplänen aus fremden Quellen. Blockiert den öffentlichen Start, nicht die Entwicklung. Optionen stehen im Addendum des Briefs. *Verantwortlich: Andreas. Wiedervorlage: vor dem öffentlichen Start.*
2. **OParl:** Bietet das Ratsinformationssystem (sitzung.zirndorf.de) eine OParl-Schnittstelle für eine spätere halbautomatische Übernahme? *Verantwortlich: Andreas. Wiedervorlage: nach v1.*
3. **Abdeckungsziel:** Welche Zielwerte gelten für SM-5 und SM-6, und wie wird „alle bekannten Events“ gemessen? *Verantwortlich: Andreas. Wiedervorlage: nach den ersten drei Monaten Betrieb.*

## 12. Annahmen-Index

Alle Annahmen aus dem Entwurf sind bestätigt oder eingearbeitet. Es gibt keine offenen Annahmen.
