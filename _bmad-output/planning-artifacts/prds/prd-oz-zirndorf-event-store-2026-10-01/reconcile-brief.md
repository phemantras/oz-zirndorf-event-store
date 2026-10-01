---
title: "Abgleich Brief -> PRD: OZ Zirndorf Event Store"
created: 2026-10-01
input: briefs/brief-oz-zirndorf-event-store-2026-10-01/ (brief.md, addendum.md)
target: prds/prd-oz-zirndorf-event-store-2026-10-01/ (prd.md, addendum.md, .memlog.md)
---

# Abgleich Brief -> PRD

Geprüft wurde, was aus Brief und Brief-Addendum im PRD bzw. PRD-Addendum fehlt, verfälscht ist oder ohne Begründung abweicht. Abweichungen, die im `.memlog.md` als `(decision)` stehen, gelten nicht als Lücke. Das sind: Ort-Objekt statt Adresse + Stammkoordinaten, jeder wiederkehrende Termin als eigenes Event ohne Serienregel, Import-Schlüssel plus Duplikatverdacht über Titel + Beginn-Datum + Ort, feste Typliste, kein läuft/bald-Status, sofortiges Ausblenden vergangener Events, Lizenz als offene Frage, die NFR-Auswahl und kein Export.

Prioritäten: **hoch** = verändert Erfolgsmessung, Haltung oder Scope · **mittel** = Inhalt geht verloren oder ist unscharf · **niedrig** = Kleinigkeit oder Formalie.

---

## A. Erfolgskriterien

### A1. Weihnachtsmarkt ist nicht mehr „das wichtigste Kriterium“ und lässt sich im Projekt nicht prüfen (hoch)
- **Brief (Erfolgskriterien):** „**Das wichtigste Kriterium:** Andreas öffnet die Karte und sieht, wann und wo der Weihnachtsmarkt stattfindet.“
- **PRD (§9):** SM-2 steht als zweites von zwei primären Kriterien hinter SM-1 (Import). Es hängt ausdrücklich vom OZ-Frontend ab, das nicht Teil des Projekts ist. Im PRD gibt es kein API-seitiges Ersatzkriterium und keine User Journey dazu. UJ-2 zeigt nur die Abfrage „heute“, aber nicht die Frage „wann ist der Weihnachtsmarkt?“ (Zeitraumfilter, Typ Markt).
- **Vorschlag:** SM-2 als Leitkriterium an die erste Stelle setzen oder als solches kennzeichnen. Ein im Projekt prüfbares Ersatzkriterium ergänzen, z. B. „Eine Zeitraumabfrage über Dezember mit Typ Markt liefert den Weihnachtsmarkt mit Zeitraum, Ort, Zeit- und Ortsgenauigkeit.“ Optional eine UJ aus Sicht der Menschen in Zirndorf ergänzen: „Wann ist der Weihnachtsmarkt?“ über den Zeitraumfilter.

### A2. Abdeckungskriterium wurde enger gefasst (hoch)
- **Brief (Erfolgskriterien):** „z. B. ‚alle bekannten Zirndorfer Events der nächsten drei Monate sind erfasst‘“
- **PRD (SM-5):** „Alle **Andreas bekannten** Zirndorfer Events …“. Mit dieser Formulierung ist das Kriterium per Definition fast immer erfüllt, denn was Andreas kennt, trägt er ein. Der Brief misst Vollständigkeit gegenüber der Wirklichkeit in Zirndorf, das PRD nur gegenüber Andreas' Wissen. Die Messfrage steht zwar als Offene Frage 4, die Einengung ist aber im memlog nicht begründet.
- **Vorschlag:** Die Brief-Formulierung übernehmen und als Messverfahren eine Referenzliste vorschlagen, z. B. den städtischen Kalender, die Kirchweih-Seite und die Ratssitzungen der nächsten drei Monate plus die bekannten Nicht-Kalender-Events. Gegen diese Liste wird stichprobenartig geprüft.

### A3. Dokumentationskriterium wurde zu einem Adoptionskriterium (mittel)
- **Brief (Erfolgskriterien):** „Die API ist mit OpenAPI **so gut dokumentiert, dass** andere OZ-Mitglieder sie als Vorlage nehmen **können**.“
- **PRD (SM-4):** „Mindestens ein weiteres OZ-Backend **orientiert sich** an der API-Dokumentation“ `[ASSUMPTION: Zielwert]`. Der Brief misst damit, ob die Dokumentation taugt, und das hat Andreas selbst in der Hand. Das PRD misst, ob andere sie tatsächlich übernehmen, und darauf hat Andreas keinen Einfluss. Das passt auch schlecht zur eigenen Haltung „Angebot, keine Vorgabe“.
- **Vorschlag:** Das Brief-Kriterium als primäres, prüfbares Kriterium führen, z. B. „Ein OZ-Mitglied kann ohne Rückfrage anhand der OpenAPI-Doku Versionierung, Fehlerformat und Filterkonventionen nachbauen“ (Review durch eine Person). Adoption höchstens als nachgelagerten Indikator führen.

### A4. Ortskenntnis taucht in keinem Kriterium auf (mittel)
- **Brief (Was das Projekt besonders macht):** „**Ortskenntnis:** … weiß, dass es die Lindner Kärwa gibt, obwohl sie in keinem städtischen Kalender steht.“ Und unter „Problem“: „Ortsteil-Kirchweihen oder das Fischerfest stehen im städtischen Kalender gar nicht.“
- **PRD:** Ortskenntnis wird in der Vision genannt. Die Lindner Kärwa dient in UJ-2 nur als Beispiel für Ortsgenauigkeit. Kein Kriterium und keine Anforderung prüft, ob Events außerhalb der offiziellen Kalender erfasst sind, obwohl das laut Brief einer der drei Werttreiber ist.
- **Vorschlag:** SM-5 ergänzen: Die Referenzliste enthält ausdrücklich Events, die nicht im städtischen Kalender stehen (Ortsteil-Kirchweihen, Fischerfest, Lindner Kärwa).

---

## B. Haltung, Ton, Werte

### B1. Die Vision klingt absoluter als der Brief und widerspricht dem Non-Goal (hoch)
- **Brief (Ausblick):** „**Auf lange Sicht** wird der Event Store zur **verlässlichen** Terminquelle für Zirndorf“.
- **PRD (§1):** „… ist **die Terminquelle für alles, was in Zirndorf stattfindet**“. Dagegen steht §7: „erhebt **keinen Vollständigkeitsanspruch** … Kuratierung geht vor Masse“. Die Vision verliert den Zeithorizont („auf lange Sicht“) und tauscht „verlässlich“ gegen „alles“. Damit widerspricht sie dem eigenen Non-Goal und der bescheidenen Haltung des Briefs („keinen technischen Vorsprung, und das ist in Ordnung“).
- **Vorschlag:** Die Vision an den Brief angleichen, z. B. „soll auf lange Sicht zur verlässlichen Terminquelle für Zirndorf werden; die OZ-Karte ist ihr sichtbarstes Gesicht“. „Verlässlich“ statt „vollständig“ betonen.

### B2. Der Wert „Pflege: hält sie aktuell“ hat keine Entsprechung (mittel)
- **Brief (Was das Projekt besonders macht):** „**Pflege:** Jemand trägt die verstreuten Termine tatsächlich zusammen **und hält sie aktuell**.“ Im Problem steht außerdem, dass das Kulturamt manche Monate erst spät veröffentlicht.
- **PRD:** Pflege wird in der Vision genannt. Für Aktualität gibt es aber weder eine Anforderung noch ein Kriterium, etwa einen Zeitstempel „zuletzt geändert/geprüft“ pro Event, mit dem Abnehmer und Andreas veraltete Einträge erkennen könnten. Dabei passt Aktualität direkt zum Leitwert „ehrliche Angaben“.
- **Vorschlag:** Entweder ein optionales, ausgeliefertes Feld „zuletzt aktualisiert“ pro Event aufnehmen (FR-1/FR-10), oder bewusst als Non-Goal bzw. Annahme vermerken. Optional ein Sekundärkriterium zur Aktualität ergänzen, z. B. Nachpflege nach einem Monats-Release des Kulturamts.

### B3. Die OZ-Standard-Frage ist im PRD faktisch schon entschieden (hoch)
- **Brief (Offene Fragen):** „Soll das API-Format mit der OZ-Community abgestimmt werden? **Oder setzt Andreas es fest**, und andere übernehmen es, wenn es ihnen passt?“ In „Was das Projekt besonders macht“ heißt es: „Das Projekt **kann festlegen**, wie OZ-Backends Daten bereitstellen“. Im Ausblick: „**prägt** das Projekt, wie OZ-Backends ihre Daten bereitstellen“.
- **PRD:** §1 sagt „ein Angebot an die Community, **keine Vorgabe**“, §7 sagt „setzt **keinen verbindlichen OZ-Standard**“. Gleichzeitig ist die Frage als Offene Frage 2 weiter offen. Im memlog gibt es dazu keine decision. Das PRD beantwortet die Frage also stillschweigend, und der Ton wird schwächer: Aus „kann festlegen/prägt“ wird „Angebot“.
- **Vorschlag:** Andreas entscheiden lassen und das im memlog festhalten. Bis dahin §1 und §7 neutral formulieren („ob das Muster verbindlich wird, ist Offene Frage 2“) oder die Offene Frage schließen. Den Anspruch „Format, Versionierung, Dokumentation, CORS“ als Liste der Dinge beibehalten, die das Muster abdeckt.

### B4. Eigentum und Verantwortung fehlen (niedrig)
- **Brief (Offene Fragen, Lizenz):** „Die Software gehört Andreas, und er trägt die Verantwortung. Trotzdem soll sie sich in den öffentlichen OZ-Bestand einfügen.“
- **PRD:** Offene Frage 1 nennt nur die Lizenz. Die Haltung, dass der Code Andreas gehört und trotzdem Teil des OZ-Bestands sein soll, fehlt.
- **Vorschlag:** Diesen Satz in §6 „Rechte und Quellen“ übernehmen.

---

## C. Fachliche Inhalte

### C1. Ortsgenauigkeit deckt nicht alle Brief-Beispiele ab, die Ort-Notiz fehlt (mittel)
- **Brief-Addendum (Beobachtungen):** Beispiele sind „Ortsteil-Zentrum, kein exakter Festplatz bekannt“, „**Festmeile, kein Einzelpunkt**“ und „**nächstgelegener bekannter Punkt**“. Die Datenstruktur hat `location { …, note? }`.
- **PRD (FR-6):** Ortsgenauigkeit kennt Gebäude / Platz oder Straße / nur Ortsteil `[ASSUMPTION]`. Damit fehlen zwei Fälle: eine Fläche bzw. ein Streckenverlauf (Festmeile) und eine Näherung (nächstgelegener Punkt). Der Ort hat außerdem keine Notiz mehr, und der Freitext aus `note` hat kein Ziel.
- **Vorschlag:** Die Werteliste um „Fläche/Strecke“ und „Näherung“ erweitern oder die vorhandenen Werte ausdrücklich darauf abbilden. Am Ort eine optionale Notiz für Präzisierungen ergänzen.

### C2. „Duplikate korrigieren“ ist zu „überspringen/neu/überschreiben“ geworden (mittel)
- **Brief (Lösung, Scope):** „warnt davor und **lässt sie korrigieren**“. Im Addendum: „Andreas kann sie korrigieren“.
- **PRD (FR-18):** Es gibt die Optionen überspringen, neu anlegen und vorhandenes überschreiben. Eine Korrektur des Eintrags im Import selbst ist nicht vorgesehen, etwa Titel oder Datum anpassen oder beide Datensätze zusammenführen. Bei Fehlern sieht UJ-1 vor, dass Andreas die Datei korrigiert und neu hochlädt.
- **Vorschlag:** Klären, ob „korrigieren“ mit den drei Optionen plus nachträglicher Bearbeitung (FR-15) ausreichend abgedeckt ist. Wenn ja, das in FR-18 ausdrücklich schreiben. Wenn nein, eine Option „bearbeiten und übernehmen“ ergänzen.

### C3. Zeitgenauigkeit für Ablaufplan-Einträge ist nicht geregelt (niedrig)
- **Brief (Scope):** „Für Zeit und Ort wird jeweils gespeichert, wie sicher die Angabe ist.“ Laut Addendum haben Timetable-Einträge `startTime`/`endTime`.
- **PRD (FR-4):** Programmpunkte haben Beginn und optional Ende, aber keine Zeitgenauigkeit. FR-2 gilt nur für das Event. Der `00:00`-Fehler aus dem Entwurf kann bei Programmpunkten also wieder auftreten.
- **Vorschlag:** In FR-4 festlegen, dass der Beginn eines Programmpunkts eine Uhrzeit braucht, oder Zeitgenauigkeit auch für Programmpunkte zulassen.

### C4. Die Umstellung der Testsammlung ist eine stille Voraussetzung für SM-1 (niedrig)
- **Brief (Erfolgskriterien):** „Die Testsammlung mit 42 Events lässt sich vollständig importieren.“ Im Scope: „Andreas definiert [das Format] auf Basis seiner Testsammlung.“
- **PRD:** SM-1 und FR-18 setzen den Import der 42 Events voraus. Laut PRD-Addendum §2 fehlen der Datei aber Typ, Zeit- und Ortsgenauigkeit, Quelle und Schlüssel. Wer die Datei auf Format v1 umstellt und wann, steht nirgends. Ebenso fehlt die Erwartung, dass die rund 10 bereits vergangenen Events direkt im Archiv landen.
- **Vorschlag:** In SM-1 ergänzen: „nach Überführung in Import-Format v1 (Aufgabe des Admins)“. Außerdem: Vergangene Events der Testsammlung landen direkt im Archiv und zählen trotzdem als importiert.

---

## D. Scope-Grenzen

### D1. Betrieb auf railway.app fehlt im MVP-Scope (niedrig)
- **Brief (In v1):** „Betrieb auf railway.app“
- **PRD:** Das steht nur im PRD-Addendum als technische Vorgabe, nicht in §8.1. Damit ist unklar, ob ein öffentlich erreichbarer Betrieb Teil von v1 ist. Der Brief zählt ihn zum Umfang, und SM-2 setzt ihn voraus. Wegen der offenen Lizenz ist der öffentliche Start allerdings blockiert (§6).
- **Vorschlag:** In §8.1 „Betrieb (Hosting, z. B. railway.app) inklusive täglichem Archiv-Lauf“ aufnehmen. Klarstellen, wie sich der Betrieb vor der Lizenzentscheidung vom öffentlichen Start unterscheidet.

### D2. „In ihrer Nähe“ ist für Endnutzer als Annahme gestrichen (niedrig, Hinweis)
- **Brief (Für wen):** „Sie wollen wissen, was heute oder bald **in ihrer Nähe** passiert“.
- **PRD:** JTBD übernommen. Die Umkreissuche ist als `[ASSUMPTION]` out of scope, weil bei ein paar hundert Events das Frontend selbst filtern kann. Das ist schlüssig, aber nicht im memlog bestätigt.
- **Vorschlag:** Die Annahme von Andreas bestätigen lassen und im memlog vermerken.

---

## E. Offene Fragen

- **Lizenz (Brief, Offene Frage 1):** übernommen (PRD OQ 1, §6). Vollständig, bis auf B4.
- **OZ-Standard (Brief, Offene Frage 2):** übernommen (PRD OQ 2), aber durch §1 und §7 vorweggenommen. Siehe B3.
- **OParl (Brief, Offene Frage 3):** übernommen (PRD OQ 3). Vollständig.
- **Abdeckungs-Zielwert (Brief, Erfolgskriterien):** übernommen (PRD OQ 4). Siehe A2 zur Einengung.
- **Cron-Lücke (Brief-Addendum, Archiv):** per memlog-decision gelöst (FR-8). Vollständig.
- **Duplikat-Schlüssel (Brief-Addendum):** per memlog-decision gelöst (FR-17). Vollständig.

---

## Vollständig und korrekt übernommen (Stichprobe)

Ehrliche Angaben als Leitwert (§1, §4.1, SM-3, SM-C1), Quelle als eigenes Pflichtfeld, Unterscheidung „Uhrzeit unbekannt“ vs. Mitternacht, Archiv mit dauerhafter Aufbewahrung und Vorbei-Regel inklusive Europe/Berlin, Standardabfrage „heute“, Filter nach Typ und Zeitraum, nur lesende öffentliche API, Single-Admin mit Login, versioniertes Import-Format, keine stillen Dubletten, Non-Goals (Crawling, Frontend, mehrere Admins, Bürger-Einreichungen, Schreibzugriff) sowie der Ausblick (weitere Pfleger, Vereine, OParl) in §8.2.
