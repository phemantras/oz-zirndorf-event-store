---
stepsCompleted: [1, 2, 3, 4]
inputDocuments:
  - _bmad-output/planning-artifacts/prds/prd-oz-zirndorf-event-store-2026-10-01/prd.md
  - _bmad-output/planning-artifacts/prds/prd-oz-zirndorf-event-store-2026-10-01/addendum.md
  - _bmad-output/planning-artifacts/architecture/architecture-oz-zirndorf-event-store-2026-10-01/ARCHITECTURE-SPINE.md
---

# oz-zirndorf-event-store - Epic Breakdown

## Overview

Dieses Dokument zerlegt die Anforderungen aus PRD und Architecture Spine des OZ Zirndorf Event Store in umsetzbare Epics und Stories.

Das Requirements Inventory fasst PRD und Spine zusammen, damit jede Story ohne Umweg lesbar ist. Bei einer Abweichung gelten PRD und Spine. Die Festlegungen ENT-1 bis ENT-23 sind seit dem Spine-Update vom 2026-10-02 im Spine enthalten, ENT-24 seit dem 2026-10-05. Jede nennt die AD, in der sie steht.

Abnehmer sind Apps, die die öffentliche API lesen, zuerst die Karten-App.

Jede Story nennt nach der User Story unter **Deckt ab** die Anforderungen, die sie umsetzt. Die Akzeptanzkriterien stehen als Szenarien (Angenommen/Wenn/Dann). Regeln, die für die ganze Story gelten, stehen danach unter **Außerdem gilt**. Tabellen und Spalten, die eine Story anlegt, stehen unter **Datenmodell**.

## Requirements Inventory

### Functional Requirements

**Event-Bestand mit ehrlichen Angaben**

FR-1: Ein Event besteht aus Titel, Event-Typ, Ort, Beginn, optionalem Ende, Zeitgenauigkeit für Beginn und Ende, Quelle, optionaler Notiz und optionalem Ablaufplan. Pflicht sind Titel, Event-Typ, Ort, Beginn (mindestens das Datum), Zeitgenauigkeit und Quelle. Fehlt eine davon, wird das Event abgelehnt. Ein Ende vor dem Beginn wird abgelehnt. Ein Event enthält keine personenbezogenen Daten.

FR-2: Zeitgenauigkeit getrennt für Beginn und Ende: Uhrzeit exakt, nur Datum oder ganztägig. Ein unbekanntes Ende ist ein leeres Ende. „Uhrzeit unbekannt“ ist von „Mitternacht“ unterscheidbar. Beginn exakt plus Ende nur als Datum ist darstellbar und erst nach Ablauf des End-Tages vorbei.

FR-3: Jedes Event nennt seine Quelle als Beschreibung, optional mit Link. Ohne Quelle wird es abgelehnt („eigene Kenntnis“ genügt). Die Quelle steht nicht im Titel.

FR-4: Ein Event kann einen Ablaufplan mit beliebig vielen Programmpunkten haben, jeder mit Beschreibung, Beginn und optionalem Ende. Die Programmpunkte werden chronologisch nach Beginn sortiert ausgeliefert.

FR-5: Jedes Event hat genau einen Event-Typ aus der festen Liste Fest/Kirchweih, Markt, Kultur/Bühne, Politik/Sitzung, Verein/Treff, Sport und Sonstiges (Codes `festival`, `market`, `culture`, `politics`, `club`, `sports`, `other`). Ein unbekannter Typ wird abgelehnt. Abnehmer können die Liste abfragen. Eine Änderung der Liste ist eine Vertragsänderung (NFR-2).

**Orte**

FR-6: Ein Ort besteht aus Name, Adresse, Koordinaten, Ortsgenauigkeit und optionaler Notiz. Name, Adresse, Koordinaten und Ortsgenauigkeit sind Pflicht. Die Adresse besteht aus Straße (mit Hausnummer, falls vorhanden), Postleitzahl (fünf Ziffern) und Ort. Alle drei sind Pflicht, auch bei einem nur ortsteilgenauen Ort und bei einem Bereich. Die Ortsgenauigkeit hat die Werte Gebäude, Platz/Straße, Bereich und nur Ortsteil (`building`, `street`, `area`, `district`). Jeder Ort hat eine stabile Kennung, die beim Bearbeiten von Name oder Adresse gleich bleibt.

FR-7: Events verweisen auf einen Ort statt einer Kopie. Werden die Koordinaten eines Orts geändert, liefern alle Events dieses Orts die neuen Koordinaten. Ein Ort, auf den noch Events verweisen (auch archivierte), kann nicht gelöscht werden.

**Öffentliche Lese-API**

FR-8: Eine Abfrage ohne Filter liefert alle aktiven Events, deren Zeitraum den heutigen Tag (Europe/Berlin) berührt. Ein Event, das heute um 14:00 endet, ist ab 14:00 nicht mehr enthalten, sondern im Archiv, unabhängig von der Bereinigung. Ein mehrtägiges Event ist an jedem seiner Tage enthalten.

FR-9: Filter nach Zeitraum (`from`/`to`) und einem oder mehreren Event-Typen (ODER-Verknüpfung). Enthalten ist ein Event, wenn sich sein Zeitraum mit dem Filterzeitraum überschneidet. Auch mit Filter liefert die Abfrage nur aktive Events. Ungültige Filterwerte werden mit einer verständlichen Meldung im einheitlichen Fehlerformat abgelehnt.

FR-10: Jedes ausgelieferte Event enthält alle Angaben aus FR-1 und den vollständigen Ort (Name, Adresse, Koordinaten, Ortsgenauigkeit, Notiz). Zwei Events am selben Ort liefern denselben, eindeutigen Ortsnamen und identische Koordinaten. Zeitgenauigkeit, Ortsgenauigkeit und Quelle sind immer enthalten. Die Antwort enthält keine internen Werte (Kennungen, Import-Schlüssel, Archivierungszeitpunkt, Vergleichsschlüssel).

FR-11: Events gibt es nur als Listen (FR-8, FR-9, FR-12), dazu die Liste aller Event-Typen. Es gibt keinen Einzelabruf und keine eigene Ortsliste; Abnehmer speichern keine Kennungen. Ob ein Event archiviert ist, ergibt sich aus der Liste, in der es steht; ein eigenes Feld gibt es nicht. Maßgeblich ist die Vorbei-Regel: Ein Event ist vorbei, sobald `effectiveEnd <= now` (AD-4). Außerhalb des Umfangs: Schreibzugriff, Einzelabruf, Ortsliste, Volltextsuche, Umkreissuche, Paginierung.

**Archiv**

FR-12: Archivierte Events lassen sich abfragen, gefiltert nach Zeitraum und Event-Typ wie in FR-9. Sie haben dieselbe Struktur wie aktive Events. Ein Event, das seit einer Minute vorbei ist, ist bereits im Archiv. Jedes Event ist zu jedem Zeitpunkt in genau einem der beiden Zugriffe enthalten.

FR-13: Eine tägliche Bereinigung markiert Events, die vorbei sind, als archiviert. Die API-Antworten sind vor und nach der Bereinigung identisch. Archivierte Events werden nie automatisch gelöscht. In v1 gibt es keine Löschfrist.

**Admin-Pflege**

FR-14: Nur der angemeldete Admin kann Daten ändern. Ohne gültige Anmeldung schlägt jeder schreibende Zugriff fehl. Es gibt genau ein Admin-Konto und keine Registrierung.

FR-15: Der Admin kann Events und Orte anlegen, bearbeiten und löschen, auch archivierte Events. Es gelten dieselben Regeln wie in FR-1 bis FR-7. Beim Anlegen eines Events wählt er einen vorhandenen Ort oder legt direkt einen neuen an. Die Koordinaten eines Orts setzt oder verschiebt er auf einer Karte. Ein archiviertes Event, das durch eine Korrektur wieder aktiv wird, erscheint wieder in den regulären Abfragen.

**JSON-Import**

FR-16: Die Import-Datei hat eine Formatversion und ist dokumentiert. Eine unbekannte oder fehlende Version wird mit klarer Meldung abgelehnt. Das Format bildet alle Angaben aus FR-1 bis FR-6 ab, einschließlich des optionalen Import-Schlüssels. Jedes Event bringt seinen Ort mit, mindestens mit Namen; ein Ort mit vorhandenem Namen wird dem vorhandenen Ort zugeordnet. Die Datei enthält keine internen Kennungen. Ein mitgebrachter neuer Ort braucht Adresse und Koordinaten, sonst ist der Eintrag fehlerhaft. Neue Orte zeigt die Vorschau gesondert an.

FR-17: Vor dem Übernehmen zeigt der Import für jedes Event: neu, Aktualisierung, Duplikatverdacht oder Fehler. Ein vorhandener Import-Schlüssel führt zu einer Aktualisierung. Ohne passenden Schlüssel, aber mit gleichem Titel, Beginn-Datum (nicht Uhrzeit) und Ort entsteht ein Duplikatverdacht. Gleicher Titel an einem anderen Datum ist kein Verdacht. Fehlerhafte Einträge werden einzeln mit Grund gemeldet und blockieren die übrigen nicht.

FR-18: Der Admin entscheidet jeden Duplikatverdacht: überspringen, als neues Event anlegen oder das vorhandene überschreiben. Ohne Entscheidung wird nichts übernommen. Nach dem Import gibt es eine Zusammenfassung (neu, aktualisiert, übersprungen, fehlerhaft). Die Testsammlung (39 Events) lässt sich vollständig importieren, ein zweiter Import erzeugt keine neuen Events.

### NonFunctional Requirements

NFR-1: CORS offen: Die öffentliche API ist von jeder Herkunft aufrufbar, die Admin-Funktionen sind es nicht.

NFR-2: Versionierung: Inkompatible Änderungen (z. B. an der Event-Typen-Liste oder an Feldnamen) gibt es nur in einer neuen Hauptversion. Neue optionale Felder dürfen in der bestehenden Version ergänzt werden. Einmalige Ausnahme: `archived` wurde vor dem ersten Abnehmer aus v1 entfernt (Story 2.8).

NFR-3: Vorlage-taugliche Dokumentation: Die öffentliche API ist vollständig mit OpenAPI beschrieben, mit Filtern, Feldbedeutungen (besonders Zeit- und Ortsgenauigkeit), Fehlerformat und den Konventionen KON-1 bis KON-8. Die Dokumentation ist öffentlich abrufbar.

NFR-4: Keine personenbezogenen Daten in Events und Orten (keine Kontaktpersonen, Telefonnummern oder Namen von Privatpersonen). Personenbezogen ist nur das Admin-Konto.

NFR-5: Betrieb auf Hobby-Niveau: keine Zielwerte für Antwortzeit und Verfügbarkeit, Best Effort.

NFR-6: Zeitzone: Alle Zeitangaben beziehen sich auf Europe/Berlin.

**API-Konventionen (Teil des Vertrags, gelten für die gesamte öffentliche API)**

KON-1: Jede Ressource liegt unter einer Hauptversion im Pfad (`/v1/…`).

KON-2: Erscheint eine neue Hauptversion, bleibt die alte mindestens 6 Monate erreichbar. Das Abschaltdatum steht in der Dokumentation.

KON-3: Fehler nach RFC 9457 (Problem Details), Beschreibung auf Englisch.

KON-4: Beginn und Ende als Datum `YYYY-MM-DD` plus getrennte, optionale Uhrzeit `HH:MM` (lokal Europe/Berlin). Eine unbekannte Uhrzeit ist `null`. Berechnete Zeitpunkte nach ISO 8601 mit Offset.

KON-5: `from`/`to` akzeptieren ein Datum oder einen Zeitpunkt und sind beide inklusive. Es zählt die Überschneidung. Ohne `to` ist der Zeitraum nach hinten offen.

KON-6: Event-Listen sind nach Beginn aufsteigend sortiert, das Archiv absteigend.

KON-7: Alles Technische ist englisch (Feldnamen in camelCase, Parameter, Pfade, Fehlermeldungen, Enum-Codes). Inhalte bleiben deutsch.

KON-8: Listen werden in einer Hülle `{ "data": [ … ] }` ausgeliefert.

### Additional Requirements

Die Punkte mit AD-Nummer sind Architekturentscheidungen aus dem Spine, nach Thema gruppiert. Die Punkte ohne Nummer sind Festlegungen aus Stack und Querschnitt des Spine.

**Starter-Template:** Keines. Es ist ein Greenfield-Go-Projekt. Story 1.1 legt das Projektgerüst nach der Struktur im Spine an: `api/v1/`, `cmd/eventstore/`, `internal/core/`, `internal/adapter/{publicapi/v1,admin,postgres,cleanup}/`, `Dockerfile`, `railway.json`, `compose.yaml`, `.github/workflows/ci.yaml`.

**Struktur und Abhängigkeiten**
- AD-1: Hexagonal light, genau eine Replika. `internal/core` importiert nur die Standardbibliothek, einzige Ausnahme ist `golang.org/x/text/unicode/norm` (NFC). Adapter importieren nur `core`, nie einander. Nur `cmd/eventstore` kennt alle. Der Kern definiert die Ports `EventRepo`, `LocationRepo`, `TxRunner` und `Clock`.
- AD-2: Fachlogik nur im Kern. SQL macht nur CRUD und einfache Vergleiche, ohne Views, Trigger, Funktionen, `lower()`/`trim()` oder `now()`. Die aktuelle Zeit kommt als Parameter aus `Clock`. Abgeleitete Werte (`effective*`, `name_key`) berechnet der Kern und speichert sie.
- AD-6: Schreiben nur über die Kern-Anwendungsfälle `SaveEvent`, `DeleteEvent`, `SaveLocation`, `DeleteLocation`, `CommitImport`, `RecomputeDerived` und `MarkArchived`. Jeder Schreib-Anwendungsfall hat einen transaktionsgebundenen Kern und eine Hülle mit `TxRunner`; `CommitImport` ruft nur die Kerne in einer Transaktion auf. `SaveEvent` aus dem Admin ändert `importKey` nie. Admin und Import nutzen denselben Eingabetyp `core.EventInput`. Der Löschschutz für Orte liegt im Kern und zusätzlich im Fremdschlüssel `ON DELETE RESTRICT`.
- AD-7: Lesen über die Kern-Abfragen `ListActiveEvents`, `ListArchivedEvents` und `ListEventTypes`, nur Listen. `GetEvent` und `ListLocations` nur für den Admin. Sortierung: aktive Events nach `effectiveStart` aufsteigend, Archiv absteigend, bei Gleichstand nach der internen `id`. Orte nach Namen im Kern (ENT-8). Den Archivstatus gibt die Public API nicht aus.

**Zeitmodell**
- AD-3: Gespeichert werden `startDate` (Pflicht), `startTime`, `endDate`, `endTime` (optional) und `allDay` (bool), lokal Europe/Berlin. Eine leere Uhrzeit heißt „unbekannt“. `allDay` gilt für Beginn und Ende gemeinsam, gemischte Angaben und `endTime` ohne `endDate` werden abgelehnt. Die Genauigkeit (`exact`/`dateOnly`/`allDay`) leitet der Kern je Beginn und Ende ab und speichert sie nicht.
- AD-4: `effectiveStart`/`effectiveEnd` (timestamptz) werden bei jedem Schreiben berechnet. Ohne Uhrzeit beim Beginn gilt 00:00 des Beginn-Tages. Ohne Uhrzeit beim Ende gilt 00:00 des Tages nach dem End-Tag. Ohne Ende gilt 00:00 des Tages nach dem Beginn-Tag. Intervalle sind halboffen. Ein Ende mit `effectiveEnd <= effectiveStart` wird abgelehnt (ENT-1). Aktiv heißt `effectiveEnd > now`. Alle Lese-Abfragen filtern nur über diese Spalten. (Diese Regel ersetzt die „23:59:59“-Formulierung aus dem PRD-Addendum und ist gleichwertig.)
- AD-16: Die einzige Umrechnung ist die Kernfunktion `ToInstant` (Europe/Berlin). Uhrzeiten in der Frühjahrslücke werden abgelehnt, doppelte Uhrzeiten im Herbst bekommen den früheren Offset. Die Zeit kommt nur aus `Clock`. Filter werden zu `[lo, hi)` normalisiert, ein reines Datum gilt als ganzer Tag inklusive. Das einzige Prädikat lautet `effectiveStart < hi AND effectiveEnd > lo`. Standardwerte von `from`/`to` je Endpunkt nach ENT-4. `time/tzdata` ist eingebettet. Beim Start berechnet `RecomputeDerived` alle abgeleiteten Werte neu, Fehler nach ENT-5; Startreihenfolge: Migrationen, `RecomputeDerived`, `MarkArchived`, HTTP-Server.
- AD-15: Der Ablaufplan ist ein Wertobjekt des Events und wird nur über `SaveEvent` als ganze Liste ersetzt. Jeder Eintrag hat `description`, `date`, `startTime?` und `endTime?`. Einträge liegen in `[effectiveStart, effectiveEnd]`, über Mitternacht nach ENT-3, und ändern `effective*` nicht. Programmpunkt-IDs sind intern und nicht stabil (jedes Speichern ersetzt die Liste).

**Archiv und Bereinigung**
- AD-5: Es gibt eine Tabelle. Der Archivstatus ergibt sich nur aus AD-4. Die API gibt ihn nicht als Feld aus; er ergibt sich aus der Liste. `archivedAt` ist nur eine Markierung für Statistik, die keine Abfrage auswertet. Sie bedeutet „zuletzt als archiviert markiert“: Jedes Speichern und jede ändernde Neuberechnung leert sie, `MarkArchived` setzt sie neu. Events mit „prüfen“ fehlen in beiden Listen.
- AD-13: Der Bereinigungsjob läuft im Programm, einmal beim Start und danach täglich. Über `MarkArchived` setzt er `archivedAt` für `effectiveEnd <= now` und leeres `archivedAt` und ist idempotent.

**API-Vertrag**
- AD-8: Spec-first. `api/v1/openapi.yaml` (OpenAPI 3.1, Rückfall 3.0.3) ist die einzige Quelle. Das Gerüst erzeugt `oapi-codegen` (`std-http-server` + `strict-server`). Es wird eingecheckt und nie von Hand geändert. v2 bekäme eine eigene Spec und ein eigenes Paket, dazu ein Abschaltdatum in `info` und einen `Sunset`-Header. Öffentlich ausgeliefert werden `/v1/openapi.yaml`, `/v1/import-v1.schema.json` und `/v1/docs` (Redoc), statisch außerhalb der Spec mit offenem CORS.
- AD-9: Enum-Codes und `EventInput` sind nur in `openapi.yaml` (`components/schemas`) definiert. Das Import-Schema bindet sie per `$ref` ein. Die Kern-Konstanten spiegeln die Codes, und ein CI-Test prüft die Übereinstimmung, auch für Längengrenzen (ENT-24). `formatVersion` ist Pflicht, unbekannte Versionen werden abgelehnt.
- AD-14: Die Schreibform `EventInput` hat die Felder `title`, `type`, `location` (mitgebrachter Ort, `name` Pflicht, Rest optional), `startDate`, `startTime`, `endDate`, `endTime`, `allDay`, `source` (Objekt, ENT-10), `note`, `timetable` und `importKey`. Die Leseform `Event` ist ein eigenes Schema mit denselben Feldnamen (ohne `importKey`, `location` vollständig), plus `startPrecision`, `endPrecision`, `effectiveStart` und `effectiveEnd` (lokaler Offset Europe/Berlin), ohne Archivstatus. Keine Form nach außen enthält interne Werte. `Canonicalize` bringt Eingaben in eine kanonische Form.
- Ressourcen (aus dem Addendum): `GET /v1/events` (`from`, `to`, `type`), `GET /v1/event-types`, `GET /v1/archive/events`.

**Import**
- AD-10: Der Import ist zustandslos. (1) Beim Upload wird jeder Eintrag geparst, validiert und gegen den gesamten Bestand klassifiziert, archivierte Events eingeschlossen. Die Klassen sind `new`, `update`, `unchanged`, `duplicateSuspect`, `error` und pro Ort `newLocation` (ENT-11). (2) Innerhalb der Datei macht ein doppelter `importKey` oder ein gemeinsames Ziel-Event beide Einträge zu `error`, gleiche Einträge werden untereinander zu `duplicateSuspect`, und ein neuer Ort wird nur einmal angelegt. (3) Der Zwischenstand liegt nur im Browser-Formular. (4) Beim Speichern wird der vollständige Satz samt Entscheidungen, ursprünglicher Klasse und Ziel-ID gesendet und neu klassifiziert. Bei einer Abweichung wird der Eintrag als `stale` gemeldet. (5) `error`, `stale` und Verdachtsfälle ohne Entscheidung werden vor der Transaktion aussortiert, der Rest wird in einer Transaktion über `TxRunner` geschrieben, immer mit `allowDuplicates` (ENT-12). (6) Die Zusammenfassung nach ENT-11. Upload-Grenzen nach ENT-13.
- AD-11: IDs sind UUIDv7 per `DEFAULT uuidv7()` (PostgreSQL 18), rein intern; sie erscheinen weder in der API noch im Import-Schema. `importKey` ist eindeutig, wenn er gesetzt ist. Der Kern normalisiert alle Texte an einer Stelle auf NFC. `NormalizeKey` (trim, Leerraum zusammenfassen, lowercase) liefert den eindeutigen Ortsschlüssel `name_key` und den Titelschlüssel `title_key`. `FindDuplicateCandidates` vergleicht `title_key`, `startDate` und `locationId` über alle Events. `SaveEvent` erhält eine Policy `rejectDuplicates` oder `allowDuplicates`. Das Admin-Formular warnt zuerst und speichert erst nach Bestätigung mit `allowDuplicates`.

**Sicherheit und Admin**
- AD-12: `/v1/…` ist nur lesend (`GET`) mit offenem CORS. `/admin/…` hat kein CORS und verlangt eine Session (HttpOnly, Secure, SameSite=Strict). Gegen CSRF schützt `http.CrossOriginProtection`. Es gibt ein Konto: `ADMIN_USER` und `ADMIN_PASSWORD_HASH` (bcrypt) aus Umgebungsvariablen. Session und Anmeldeschutz nach ENT-6, ENT-7 und ENT-22.
- Die Admin-Oberfläche nutzt `html/template` und htmx 2.0.11, den Kartenpicker mit Leaflet 1.9.4 und OSM-Kacheln. htmx und Leaflet liegen als Dateien unter `adapter/admin/static`, nicht per CDN.
- Die Admin-Oberfläche ist deutsch, alles Technische englisch.

**Datenbank und Migrationen**
- PostgreSQL 18 ist ausdrücklich gepinnt. Tabellen: `events`, `locations`, `timetable_entries` (snake_case, Plural). Zugriff über pgx v5.11.0 und sqlc 1.31.1. `uuidv7()` steht nur in `DEFAULT`, nicht in sqlc-Queries.
- AD-17: goose v3.28.0, SQL eingebettet per `embed`. Migrationen laufen beim Start vor dem HTTP-Server, nur vorwärts nach expand/contract, und angewendete Migrationen werden nie geändert. Vor jedem Deploy mit einer neuen Migration zieht der Admin einen `pg_dump` über die Railway-CLI.

**Querschnitt im Code**
- Der Kern liefert typisierte Fehler (`ErrValidation`, `ErrNotFound`, `ErrConflict`, `ErrDuplicateSuspect`). Nur die Adapter übersetzen sie in HTTP-Status oder Problem Details.
- Logging mit `log/slog` als JSON auf stdout, ohne Passwörter und Session-Daten.
- Konfiguration nur über Umgebungsvariablen: `DATABASE_URL`, `PORT`, `ADMIN_USER`, `ADMIN_PASSWORD_HASH` (wegen `$` gequotet) und `SESSION_SECRET`.

**Betrieb und Deployment**
- Railway: ein Projekt mit der Umgebung `production` und den Services App und PostgreSQL 18. Die Datenbank ist nur über das private Netz erreichbar.
- Ein Multi-Stage-Dockerfile. In `railway.json` stehen `healthcheckPath: /healthz` und das Dockerfile. „Wait for CI“ ist aktiviert.
- `GET /healthz` prüft, ob die Datenbank erreichbar ist.
- Lokal läuft PostgreSQL 18 per Docker Compose (Volume unter `/var/lib/postgresql`), mit denselben Migrationen und Umgebungsvariablen.
- CI (GitHub Actions) prüft Tests, ob der generierte Code aktuell ist (oapi-codegen, sqlc), und den Enum-Abgleich.

**Tests**
- Kernregeln (Zeitmodell, `ToInstant`, Vorbei-Regel, Duplikat, Import-Klassifizierung) werden mit Unit-Tests ohne Datenbank und mit fester `Clock` getestet.
- Der Postgres-Adapter wird gegen echtes PostgreSQL 18 (Docker) getestet.

### Ergänzende Festlegungen (Review 2026-10-01)

Diese Festlegungen präzisieren den Spine und sind in den Stories umgesetzt. Seit dem Spine-Update vom 2026-10-02 stehen sie im Spine (Pfeil = AD), maßgeblich ist dort der Wortlaut. ENT-15 bis ENT-23 kamen aus dem Review dieses Updates hinzu, ENT-24 mit dem Sprint Change Proposal vom 2026-10-05.

- ENT-1 (→ AD-4) Zeitraum prüfen: Ein Ende wird abgelehnt, wenn `effectiveEnd <= effectiveStart`. Verglichen werden immer die berechneten Werte, nie die Rohfelder. Ein eintägiges Event mit `endDate = startDate` (nur Datum) ist gültig.
- ENT-2 (→ AD-3) Ganztägig: `allDay` gilt für Beginn und Ende gemeinsam. Gemischte Angaben (ein Teil ganztägig, der andere mit Uhrzeit) sind nicht darstellbar und werden abgelehnt. Die Spec dokumentiert das.
- ENT-3 (→ AD-15) Programmpunkte über Mitternacht: Liegt `endTime` vor `startTime`, endet der Punkt am Folgetag.
- ENT-4 (→ AD-16) Filter-Standardwerte: In `GET /v1/events` gilt ohne `from` und `to` der heutige Tag (FR-8). Fehlt nur `from`, beginnt der Zeitraum mit dem heutigen Tag. Fehlt nur `to`, ist er offen. In `GET /v1/archive/events` ist ein fehlendes `from` offen und ein fehlendes `to` gleich `now`. Ein Zeitpunkt in `from`/`to` braucht einen Offset. Ein Zeitpunkt in `to` wird auf die Minute abgeschnitten und um eine Minute erhöht (`hi`), damit `to` inklusive ist.
- ENT-5 (→ AD-16) Neuberechnung beim Start: Schlägt sie für ein Event fehl (Zeitangaben abgelehnt oder Ablaufplan außerhalb des neuen Zeitraums), behält es seine gespeicherten Werte. Der Fehler wird mit der Event-Kennung geloggt, das Programm startet trotzdem, und der Admin markiert das Event mit „prüfen“. Die Menge der betroffenen IDs hält der Kern nur im Speicher (Mutex). Nach erfolgreichem Commit von `SaveEvent`, `CommitImport` oder `DeleteEvent` wird die ID entfernt. Für Orte mit kollidierendem neuen `name_key` gilt dasselbe; die Markierung „prüfen“ erscheint in der Ortsliste und verschwindet nach `SaveLocation` oder `DeleteLocation`. Events mit „prüfen“ erscheinen in keiner öffentlichen Liste (Story 2.7).
- ENT-6 (→ AD-12) Admin-Session: Signiertes Cookie mit `SESSION_SECRET`, ohne Zustand auf dem Server. Sie läuft nach 8 Stunden ohne Aktivität oder spätestens 7 Tage nach der Anmeldung ab. Abmelden löscht das Cookie im Browser. Ein kopiertes Cookie bleibt bis zum Ablauf gültig; das ist bewusst hingenommen. Ein neues `SESSION_SECRET` macht alle Sessions ungültig.
- ENT-7 (→ AD-12) Anmeldeschutz: Nach 5 Fehlversuchen von einer Client-IP ist die Anmeldung von dieser IP für 15 Minuten gesperrt. Die Zähler liegen im Speicher; ein Neustart setzt sie zurück. Client-IP ist der Eintrag in `X-Forwarded-For`, den Railways Edge setzt (Prüfung nach ENT-21), ohne Header `RemoteAddr` ohne Port.
- ENT-8 (→ AD-7) Sortierung nach Name: Der Kern sortiert Orte ohne Unterschied von Groß- und Kleinschreibung, Umlaute wie ihren Grundbuchstaben (ä→a, ö→o, ü→u, ß→ss), bei Gleichstand nach `id`. Die Datenbank-Sortierung wird nicht genutzt.
- ENT-9 (→ AD-1, AD-11) Texteingaben: Der Kern normalisiert alle Texte an einer Stelle beim Bau jedes Inputs auf Unicode NFC (`golang.org/x/text/unicode/norm` v0.42.0, einzige Ausnahme von AD-1), für Admin und Import gleich. `NormalizeKey` trimmt, fasst jeden Leerraum (auch geschützte Leerzeichen) zu einem Leerzeichen zusammen und wandelt in Kleinbuchstaben. Ein Pflichtfeld, das nur aus Leerraum besteht, fehlt.
- ENT-10 (→ AD-14) Quelle: `source` ist in Lese- und Schreibform ein Objekt `{ "description": string, "url": string | null }`. `description` ist Pflicht, `url` muss eine http(s)-URL sein.
- ENT-11 (→ AD-10) Import-Klassen: Zusätzlich zu AD-10 gibt es die Klasse `unchanged` (Import-Schlüssel vorhanden, keine Abweichung, nichts wird geschrieben). Für `update` zeigt die Vorschau die geänderten Felder. Vorrang: `error` vor `update`/`unchanged` vor `duplicateSuspect`. Die Zusammenfassung zählt neu, aktualisiert, unverändert, übersprungen, ohne Entscheidung, fehlerhaft und veraltet; die Summe entspricht der Zahl der Einträge.
- ENT-12 (→ AD-10, AD-11) Übernahme: Alle Schreibvorgänge in `CommitImport` nutzen `allowDuplicates`, weil die Klassifizierung schon entschieden ist. Ein Eintrag ohne `importKey` lässt den Schlüssel des Ziel-Events beim Überschreiben unverändert. Hat das Ziel-Event einen anderen Schlüssel als der Eintrag, ist der Eintrag `error`.
- ENT-13 (→ AD-10) Import-Upload: Höchstens 2 MB je Datei. Eine Datei ohne Einträge wird mit einer Meldung abgelehnt.
- ENT-14 (→ AD-8, Stack) API-Dokumentation: `/v1/docs` rendert die Spec mit Redoc 2.5.4 (`redoc.standalone.js`), ausgeliefert aus `adapter/publicapi/v1/static`. Das ist die einzige HTML-Seite unter `/v1/…`.
- ENT-15 (→ AD-6) Transaktionen: Jeder Schreib-Anwendungsfall hat einen transaktionsgebundenen Kern, der die Repositories einer laufenden Transaktion erhält, und eine Hülle, die ihn über `TxRunner` ausführt. `CommitImport` ruft nur die Kerne auf, alles in einer Transaktion. `TxRunner` serialisiert alle Schreib-Transaktionen über eine transaktionsgebundene Advisory-Sperre (AD-6).
- ENT-16 (→ AD-6) Import-Schlüssel: `SaveEvent` aus dem Admin ändert `importKey` nie. Nur `CommitImport` setzt ihn.
- ENT-17 (→ AD-6, AD-13, AD-16) Weitere Schreib-Anwendungsfälle: `RecomputeDerived` (beim Start, alle abgeleiteten Werte: `effective*`, `title_key`, `name_key`) und `MarkArchived` (Bereinigungsjob). Startreihenfolge: Migrationen, `RecomputeDerived`, `MarkArchived`, HTTP-Server.
- ENT-18 (→ AD-14) Datenformen: Der mitgebrachte Ort heißt `location` wie in der Leseform; `locationId` und `importLocation` gibt es nicht (Sprint Change Proposal 2026-10-04). `Event` und `EventInput` sind eigene Schemas mit denselben Feldnamen. `effective*` tragen den lokalen Offset von Europe/Berlin (`+01:00`/`+02:00`). `Canonicalize` (getrimmt, `""` als `null`, Ablaufplan sortiert) wird vor dem Speichern angewendet und beim Vergleich für `unchanged` genutzt.
- ENT-19 (→ AD-10) Import-Konflikte: Zwei Einträge mit demselben Ziel-Event sind beide `error`. Ein Eintrag, dessen Ziel-Event einen anderen `importKey` hat, ist schon beim Upload `error`.
- ENT-20 (→ AD-15) Ablaufplan-Grenze: Programmpunkte liegen in `[effectiveStart, effectiveEnd]`, ein Punkt darf genau mit dem Event enden.
- ENT-21 (→ AD-12, Seed) Betrieb: Die App läuft mit genau einer Replika. Nach dem ersten Deploy wird per `curl` mit gefälschtem `X-Forwarded-For` geprüft, ob Railway den Client-Wert verwirft (dann linker Eintrag) oder anhängt (dann rechter Eintrag). Das Ergebnis steht in der README.
- ENT-22 (→ AD-12) Session-Technik: HMAC-SHA256, das Cookie trägt Anmelde- und Aktivitätszeitpunkt und wird bei Aktivität neu ausgestellt. `SESSION_SECRET` hat mindestens 32 Byte, das prüft der Start. bcrypt mit Kosten ≥ 12.
- ENT-23 (→ AD-8) Statische Pfade: `/v1/openapi.yaml`, `/v1/import-v1.schema.json` und `/v1/docs` sind statische Auslieferungen außerhalb der Spec, mit offenem CORS.
- ENT-24 (→ AD-9, AD-11) Grenzen: Der Kern begrenzt Texte nach der NFC-Normalisierung in Zeichen (Unicode-Codepoints), für Admin und Import gleich: `title`, `location.name` und `street` höchstens 200; `city` höchstens 100; `source.description` und die Beschreibung eines Programmpunkts höchstens 500; `note` (Event, Ort) und `source.url` höchstens 2000; `importKey` höchstens 200; höchstens 100 Programmpunkte je Event. Die Grenzen stehen als `maxLength`/`maxItems` in `EventInput` und den eingebundenen Schemas von `openapi.yaml`; die Kern-Konstanten spiegeln sie, und der Test aus AD-9 prüft die Übereinstimmung.

### Success Metrics

- SM-1: Der Zirndorfer Weihnachtsmarkt ist im Bestand und über eine Zeitraumabfrage für die Adventszeit vollständig abrufbar.
- SM-2: Die Testsammlung im Format v1 (`testdata/zirndorf_events.v1.json`, 39 Events) wird vollständig importiert, ein zweiter Import erzeugt 0 zusätzliche Events.
- SM-3: 100 % der ausgelieferten Events tragen Zeitgenauigkeit für Beginn und Ende, Ortsgenauigkeit und Quelle.
- SM-4: Ein OZ-Mitglied bestätigt nach Durchsicht der OpenAPI-Dokumentation, dass es Versionierung, Fehlerformat, Zeitformat und Filterkonventionen ohne Rückfrage übernehmen könnte.
- SM-C1 (Gegenmetrik Scheinpräzision): Genauigkeitswerte werden nie geraten, um die Karte schöner aussehen zu lassen. Im Zweifel gilt der vorsichtigere Wert.

**Testsammlung:** `testdata/zirndorf_events.json` (Stand 2026-09-18, Mai 2026 bis Dezember 2027, 41 Events) ist mit Story 3.4 in das Import-Format v1 überführt: `testdata/zirndorf_events.v1.json` mit 39 Events (Herbstmarkt aus drei Einträgen zusammengelegt). Dabei kommen Typ, Zeitgenauigkeit statt `00:00`, Ortsgenauigkeit, eine aus dem Feld `name` herausgelöste Quelle und Import-Schlüssel dazu. Orte werden mitgebracht (es gibt keine Referenz per Kennung), damit die Datei in jeder Umgebung importierbar ist. Der Weihnachtsmarkt ist nicht in der Datei; er ist in Produktion über die Admin-Oberfläche angelegt (Sprint Change Proposal 2026-10-06).

### Out of Scope v1

Bewusst zurückgestellt, ohne Stories in v1: automatisches Backup, Lizenz (blockiert nur den öffentlichen Start), gleichzeitige Bearbeitung, eigene Domain, Staging, Rate Limiting und Caching der öffentlichen API, Monitoring über Logs hinaus, PostGIS, serverseitige Sessions.

### UX Design Requirements

Es gibt kein UX-Dokument. Anforderungen an die Admin-Oberfläche ergeben sich aus FR-14 bis FR-18 und AD-12. Layout und CSS werden auf Story-Ebene festgelegt.

### FR Coverage Map

FR-1: Epic 1 - Event-Daten und Pflichtfelder
FR-2: Epic 1 - Zeitgenauigkeit für Beginn und Ende
FR-3: Epic 1 - Quelle
FR-4: Epic 1 - Ablaufplan
FR-5: Epic 1 - Feste Liste der Event-Typen
FR-6: Epic 1 - Ortsdaten und Ortsgenauigkeit
FR-7: Epic 1 - Ortsreferenz und Löschschutz
FR-8: Epic 2 - Standardabfrage „heute“
FR-9: Epic 2 - Filter nach Zeitraum und Event-Typ
FR-10: Epic 2 - Kartengerechte Antwort mit vollständigem Ort
FR-11: Epic 2 - Nur Listen, Liste der Event-Typen
FR-12: Epic 2 - Archiv-Zugriff
FR-13: Epic 2 - Tägliche Bereinigung
FR-14: Epic 1 - Admin-Anmeldung
FR-15: Epic 1 - Events und Orte pflegen, Kartenpicker
FR-16: Epic 3 - Versioniertes Import-Format
FR-17: Epic 3 - Import-Vorschau
FR-18: Epic 3 - Entscheidung bei Duplikatverdacht, Zusammenfassung
NFR-1 bis NFR-3, KON-1 bis KON-8: Epic 2
NFR-4: Epic 1 (Stories 1.4, 1.7) und Epic 3 (Story 3.2)
NFR-5: alle Epics, ohne eigene Story (Best Effort)
NFR-6: Epic 1 (Story 1.6)
SM-1: Epic 2 (Story 2.6, Fixture) und Epic 3 (Story 3.5, Produktion)
SM-2: Epic 3 (Stories 3.4, 3.5)
SM-3: Epic 2 (Story 2.3)
SM-4: Epic 2 (Story 2.6, manuell)

## Epic List

### Epic 1: Andreas pflegt Orte und Events im Admin
**FRs covered:** FR-1, FR-2, FR-3, FR-4, FR-5, FR-6, FR-7, FR-14, FR-15

### Epic 2: Die Karten-App liest Events über die öffentliche API
**FRs covered:** FR-8, FR-9, FR-10, FR-11, FR-12, FR-13 (plus NFR-1 bis NFR-3, KON-1 bis KON-8)

### Epic 3: Andreas importiert Recherchen per JSON-Datei
**FRs covered:** FR-16, FR-17, FR-18

## Epic 1: Andreas pflegt Orte und Events im Admin

Andreas meldet sich im Admin an, legt Orte mit Kartenpicker an und erfasst Events mit ehrlichen Angaben (Zeitgenauigkeit, Quelle, Ablaufplan, Event-Typ). Bei Duplikatverdacht wird er gewarnt. Das System läuft auf Railway, der Bestand ist echt.

### Story 1.1: Lokales Grundgerüst mit CI

Als Andreas (Entwickler und Admin),
möchte ich ein lokal lauffähiges Grundgerüst mit Datenbank und automatischen Prüfungen,
damit jede weitere Story auf einer geprüften Struktur aufbaut.

**Deckt ab:** AD-1, AD-17, Starter-Template

**Acceptance Criteria:**

**Angenommen** ein frisch geklontes Repository
**Wenn** `go build ./...` läuft
**Dann** baut das Projekt mit Go 1.27.1 fehlerfrei
**Und** die Verzeichnisse entsprechen dem Spine: `api/v1/`, `cmd/eventstore/`, `internal/core/`, `internal/adapter/{publicapi/v1,admin,postgres,cleanup}/`

**Angenommen** `compose.yaml`
**Wenn** `docker compose up` läuft
**Dann** startet lokal PostgreSQL 18 mit dem Volume unter `/var/lib/postgresql`

**Angenommen** gesetzte Umgebungsvariablen `DATABASE_URL` und `PORT`
**Wenn** `cmd/eventstore` startet
**Dann** laufen die eingebetteten goose-Migrationen, bevor der HTTP-Server Anfragen annimmt

**Angenommen** eine Pflichtvariable fehlt
**Wenn** `cmd/eventstore` startet
**Dann** bricht das Programm mit einer klaren Log-Meldung ab

**Angenommen** das laufende Programm
**Wenn** `GET /healthz` aufgerufen wird
**Dann** antwortet es mit 200, wenn die Datenbank erreichbar ist, sonst mit 503

**Angenommen** ein Push auf `main`
**Wenn** die GitHub Actions laufen
**Dann** prüfen sie `go vet`, Unit-Tests und Tests gegen einen PostgreSQL-18-Service
**Und** ein Architekturtest schlägt fehl, sobald `internal/core` etwas außerhalb der Standardbibliothek importiert oder ein Adapter einen anderen Adapter importiert (AD-1)

**Außerdem gilt:**
- Logs erscheinen als JSON (`log/slog`) auf stdout.
- `time/tzdata` ist eingebettet.
- Die README beschreibt den lokalen Start.

### Story 1.2: Auslieferung auf Railway

Als Andreas (Entwickler und Admin),
möchte ich, dass jeder grüne Stand auf `main` automatisch auf Railway läuft,
damit jede weitere Story direkt in einem laufenden System landet.

**Deckt ab:** AD-17, Betrieb und Deployment, ENT-21

**Acceptance Criteria:**

**Angenommen** grüne CI
**Wenn** Railway deployt (Multi-Stage-Dockerfile, `railway.json` mit `healthcheckPath: /healthz`, „Wait for CI“ aktiv)
**Dann** ist `/healthz` unter der Railway-Domain erreichbar
**Und** der PostgreSQL-Service läuft ausdrücklich in Version 18 und ist nur über das private Netz erreichbar

**Angenommen** rote CI
**Wenn** auf `main` gepusht wird
**Dann** deployt Railway nicht

**Angenommen** der erste Deploy läuft
**Wenn** ich `/healthz` per `curl` mit einem gefälschten `X-Forwarded-For` aufrufe und den ankommenden Header logge
**Dann** steht in der README, ob Railway den Client-Wert verwirft (Client-IP = linker Eintrag) oder anhängt (Client-IP = rechter Eintrag) (ENT-21)
**Und** die App läuft mit genau einer Replika

**Außerdem gilt:**
- Die README beschreibt das Deployment und die Regel aus AD-17: vor jedem Deploy mit neuer Migration einen `pg_dump` über die Railway-CLI ziehen; Migrationen nur vorwärts nach expand/contract; angewendete Migrationen nie ändern.

### Story 1.3: Admin-Anmeldung

Als Admin,
möchte ich mich mit meinem einzigen Konto im Admin anmelden und abmelden,
damit nur ich Daten ändern kann.

**Deckt ab:** FR-14, AD-12, ENT-6, ENT-7, ENT-21, ENT-22

**Acceptance Criteria:**

**Angenommen** `ADMIN_USER`, `ADMIN_PASSWORD_HASH` (bcrypt) und `SESSION_SECRET` sind gesetzt
**Wenn** ich auf `/admin/login` die richtigen Zugangsdaten eingebe
**Dann** bekomme ich ein signiertes Session-Cookie (HttpOnly, Secure, SameSite=Strict) und lande auf der Admin-Startseite

**Angenommen** falsche Zugangsdaten
**Wenn** ich mich anmelde
**Dann** sehe ich eine deutsche Fehlermeldung und bekomme keine Session
**Und** das Passwort erscheint nicht im Log

**Angenommen** 5 Fehlversuche von derselben Client-IP
**Wenn** von dieser IP ein weiterer Anmeldeversuch kommt, auch mit richtigen Zugangsdaten
**Dann** wird er für 15 Minuten mit einer deutschen Meldung abgelehnt, ohne das Passwort zu prüfen

**Angenommen** ich bin nicht angemeldet
**Wenn** ich eine Seite unter `/admin/…` aufrufe, die weder die Anmeldeseite (GET und POST) noch eine Datei unter `/admin/static/` ist
**Dann** werde ich zur Anmeldung umgeleitet
**Und** jeder schreibende Zugriff (POST) außer der Anmeldung schlägt fehl

**Angenommen** meine Session ist abgelaufen
**Wenn** htmx eine Anfrage schickt (Header `HX-Request`)
**Dann** antwortet der Server mit `HX-Redirect: /admin/login` statt mit einer Umleitung, damit die Anmeldeseite nicht in ein Formularfragment eingesetzt wird

**Angenommen** ein Formular wird von einer fremden Herkunft abgeschickt
**Wenn** der Request `/admin/…` erreicht
**Dann** lehnt `http.CrossOriginProtection` ihn mit 403 ab

**Angenommen** ich bin angemeldet
**Wenn** ich 8 Stunden lang keine Admin-Seite aufrufe oder 7 Tage seit der Anmeldung vergangen sind
**Dann** ist die Session abgelaufen, und ich muss mich neu anmelden
**Und** jede Anfrage innerhalb der Laufzeit verlängert die 8 Stunden, nicht aber die 7 Tage

**Angenommen** ich bin angemeldet
**Wenn** ich mich abmelde
**Dann** löscht der Server das Cookie im Browser, und ich lande auf der Anmeldeseite

**Angenommen** `SESSION_SECRET` wird geändert
**Wenn** das Programm neu startet
**Dann** sind alle bisherigen Sessions ungültig

**Außerdem gilt:**
- `/admin/…` sendet keine CORS-Header.
- Es gibt keine Registrierung.
- Fehlt eine der drei Umgebungsvariablen, bricht das Programm beim Start ab.
- Die Admin-Seiten sind deutsch, nutzen ein schlichtes Grundlayout und laden htmx 2.0.11 aus `adapter/admin/static`, nicht über ein CDN.
- Ein kopiertes Cookie bleibt bis zum Ablauf gültig (ENT-6); die README nennt das.

### Story 1.4: Orte anlegen, bearbeiten und auflisten

Als Admin,
möchte ich Orte mit Name, Adresse, Koordinaten, Ortsgenauigkeit und Notiz pflegen,
damit Events später exakt denselben Kartenpunkt referenzieren können.

**Deckt ab:** FR-6, NFR-4, AD-11, ENT-8, ENT-9

**Datenmodell:** Tabelle `locations` mit `id` (UUIDv7 per `DEFAULT uuidv7()`), `name`, eindeutigem `name_key`, `address`, `latitude`, `longitude`, `precision` und `note`.

**Acceptance Criteria:**

**Angenommen** die Ortsverwaltung im Admin
**Wenn** ich einen Ort mit allen Pflichtangaben speichere
**Dann** wird er über den Kern-Anwendungsfall `SaveLocation` angelegt und erscheint in der Ortsliste, sortiert nach ENT-8

**Angenommen** Name, Adresse, Koordinaten oder Ortsgenauigkeit fehlen oder bestehen nur aus Leerraum
**Wenn** ich speichere
**Dann** lehnt der Kern mit `ErrValidation` ab, und das Formular zeigt je Feld eine deutsche Meldung, ohne die Eingaben zu verlieren
**Und** ungültige Koordinaten (Breite außerhalb von −90 bis 90, Länge außerhalb von −180 bis 180, keine Zahl) werden ebenso abgelehnt

**Angenommen** ich gebe Koordinaten mit Dezimalkomma ein (z. B. `49,4424`)
**Wenn** ich speichere
**Dann** liest das Formular sie als Dezimalzahl
**Und** ein leeres Koordinatenfeld gilt als fehlend, nie als 0

**Angenommen** die Ortsgenauigkeit
**Wenn** ich sie auswähle
**Dann** stehen genau `building`, `street`, `area` und `district` mit deutschen Beschriftungen zur Wahl (Gebäude, Platz/Straße, Bereich, nur Ortsteil)

**Angenommen** es gibt den Ort „Paul-Metz-Halle“
**Wenn** ich einen Ort „ paul-metz-halle “ anlege oder einen anderen Ort so umbenenne
**Dann** lehnt der Kern mit `ErrConflict` und einem Hinweis auf den vorhandenen Ort ab, weil `NormalizeKey` denselben Schlüssel liefert
**Und** auch eine Verletzung des eindeutigen `name_key` in der Datenbank erscheint als diese deutsche Meldung, nie als Fehlerseite

**Angenommen** ein vorhandener Ort
**Wenn** ich Name oder Adresse ändere
**Dann** bleibt seine Kennung unverändert

**Außerdem gilt:**
- Die Codes der Ortsgenauigkeit sind Konstanten im Kern.
- Der Ort hat keine Felder für Personen. Unter Name und Notiz weist ein Hinweis darauf hin, dass keine Privatpersonen, Kontaktpersonen oder Telefonnummern eingetragen werden (NFR-4).
- `NormalizeKey` (ENT-9), die Sortierung (ENT-8) und die Validierung sind mit Unit-Tests ohne Datenbank abgedeckt, darunter Umlaute, doppelte und geschützte Leerzeichen. Der Postgres-Adapter ist mit Tests gegen PostgreSQL 18 abgedeckt.
- In dieser Story werden die Koordinaten als Zahlen eingegeben.

### Story 1.5: Koordinaten auf der Karte setzen

Als Admin,
möchte ich die Koordinaten eines Orts auf einer Karte setzen und verschieben,
damit ich sie nicht von Hand nachschlagen und eintippen muss.

**Deckt ab:** FR-15

**Acceptance Criteria:**

**Angenommen** das Formular für einen neuen Ort
**Wenn** es sich öffnet
**Dann** zeigt eine Karte Zirndorf (Leaflet 1.9.4 aus `adapter/admin/static`, OpenStreetMap-Kacheln)
**Und** der OSM-Hinweis auf die Urheber ist sichtbar

**Angenommen** die Karte
**Wenn** ich in die Karte klicke
**Dann** setzt das einen Marker, und die Felder für Breite und Länge übernehmen seine Koordinaten

**Angenommen** ein vorhandener Ort
**Wenn** ich ihn bearbeite
**Dann** steht der Marker auf den gespeicherten Koordinaten
**Und** schiebe ich den Marker, ändern sich die Felder mit

**Angenommen** ich tippe gültige Koordinaten von Hand in die Felder
**Wenn** ich ein Feld verlasse
**Dann** springt der Marker an diese Position

**Angenommen** ich tippe ungültige Koordinaten (keine Zahl oder außerhalb des Wertebereichs)
**Wenn** ich ein Feld verlasse
**Dann** bleibt der Marker stehen, und das Feld ist als fehlerhaft markiert

**Außerdem gilt:**
- Ohne JavaScript oder ohne geladene Kacheln lässt sich der Ort weiterhin über die Zahlenfelder speichern.

### Story 1.6: Ehrliches Zeitmodell im Kern

Als Admin,
möchte ich, dass das System Zeiten genau so speichert, wie sie bekannt sind, und daraus einheitlich den effektiven Zeitraum berechnet,
damit nie eine Genauigkeit entsteht, die es nicht gibt, und „vorbei“ überall dasselbe bedeutet.

**Deckt ab:** FR-2, NFR-6, AD-3, AD-4, AD-16, ENT-1, ENT-2

**Acceptance Criteria:**

**Angenommen** der Kern-Typ für Zeitangaben mit `startDate` (Pflicht), `startTime`, `endDate`, `endTime` (optional) und `allDay`
**Wenn** er validiert wird
**Dann** wird abgelehnt: kein `startDate`; `allDay` zusammen mit einer Uhrzeit; `endTime` ohne `endDate`; `effectiveEnd <= effectiveStart` (ENT-1)
**Und** eine leere Uhrzeit heißt „unbekannt“, nie 00:00

**Angenommen** `startDate = endDate`, beide ohne Uhrzeit, oder ein exakter Beginn mit einem Ende nur als Datum am selben Tag
**Wenn** validiert wird
**Dann** ist das gültig

**Angenommen** gültige Zeitangaben
**Wenn** der Kern die Genauigkeit ableitet
**Dann** gilt für den Beginn: `allDay` → `allDay`, mit Uhrzeit → `exact`, sonst `dateOnly`
**Und** für das Ende gilt: `allDay` → `allDay`, mit Uhrzeit → `exact`, nur Datum → `dateOnly`, kein Ende → keine Genauigkeit (leer)
**Und** `allDay` gilt für Beginn und Ende gemeinsam; eine gemischte Angabe ist nicht darstellbar (ENT-2)

**Angenommen** gültige Zeitangaben
**Wenn** der Kern `effectiveStart` und `effectiveEnd` berechnet (AD-4)
**Dann** gilt beim Beginn ohne Uhrzeit 00:00 des Beginn-Tages, beim Ende ohne Uhrzeit 00:00 des Tages nach dem End-Tag und ohne Ende 00:00 des Tages nach dem Beginn-Tag
**Und** jede Umrechnung läuft ausschließlich über `ToInstant` mit der Zeitzone Europe/Berlin

**Angenommen** eine Uhrzeit in der Lücke der Frühjahrsumstellung (z. B. 2027-03-28 02:30)
**Wenn** `ToInstant` sie umrechnet
**Dann** wird sie mit `ErrValidation` und einem Hinweis auf die Zeitumstellung abgelehnt

**Angenommen** eine doppelte Uhrzeit im Herbst (z. B. 2026-10-25 02:30)
**Wenn** `ToInstant` sie umrechnet
**Dann** bekommt sie den früheren Offset (+02:00)
**Und** liegt dadurch das Ende vor dem Beginn (z. B. 02:30 bis 02:15 am 2026-10-25), nennt die Meldung die doppelte Stunde der Zeitumstellung als Grund

**Angenommen** der Port `Clock` mit fester Zeit
**Wenn** der Kern prüft, ob ein Event vorbei ist
**Dann** gilt `effectiveEnd <= now`

**Außerdem gilt:**
- Die Genauigkeit wird nicht gespeichert (AD-3).
- Unit-Tests ohne Datenbank belegen mindestens:
  - Ende heute 14:00 ist um 13:59 aktiv und um 14:00 vorbei.
  - Kirchweih Freitag bis Montag (nur Datum) ist am ganzen Montag aktiv und ab Dienstag 00:00 vorbei.
  - Beginn exakt mit Ende nur als Datum ist erst nach Ablauf des End-Tages vorbei.
  - Ein eintägiges Event mit `endDate = startDate` ist gültig.

### Story 1.7: Events anlegen, bearbeiten und auflisten

Als Admin,
möchte ich Events mit Titel, Typ, Ort, Zeitangaben, Quelle und Notiz pflegen,
damit der Bestand echte Zirndorfer Termine mit ehrlichen Angaben enthält.

**Deckt ab:** FR-1, FR-3, FR-5, FR-7, FR-15, NFR-4, AD-2, AD-6, AD-14, AD-16, ENT-5, ENT-9, ENT-10, ENT-15, ENT-16, ENT-17, ENT-18

**Datenmodell:** Tabelle `events` mit `id` (UUIDv7), `title`, `type`, `location_id` (Fremdschlüssel mit `ON DELETE RESTRICT`), `start_date`, `start_time`, `end_date`, `end_time`, `all_day`, `source_description`, `source_url`, `note`, `effective_start` und `effective_end`.

**Acceptance Criteria:**

**Angenommen** das Event-Formular im Admin
**Wenn** ich ein Event mit allen Pflichtangaben speichere
**Dann** schreibt es der Kern-Anwendungsfall `SaveEvent` mit dem Eingabetyp `core.EventInput` (AD-6, AD-14)
**Und** `effective_start` und `effective_end` sind vom Kern berechnet und mitgespeichert

**Angenommen** Titel, Typ, Ort, Beginn-Datum oder Quellenbeschreibung fehlen oder bestehen nur aus Leerraum, oder der Typ ist unbekannt
**Wenn** ich speichere
**Dann** lehnt der Kern mit `ErrValidation` ab, und das Formular zeigt deutsche Meldungen je Feld, ohne die Eingaben zu verlieren
**Und** ein angegebener Quellen-Link muss eine gültige http(s)-URL sein

**Angenommen** das Feld Event-Typ
**Wenn** ich es auswähle
**Dann** stehen genau die sieben Codes `festival`, `market`, `culture`, `politics`, `club`, `sports` und `other` mit deutschen Beschriftungen zur Wahl

**Angenommen** ich lasse im Formular eine Uhrzeit leer
**Wenn** das Formular angezeigt wird
**Dann** weist ein Hinweis darauf hin, dass leer „unbekannt“ bedeutet

**Angenommen** die Event-Liste im Admin
**Wenn** ich sie öffne
**Dann** sehe ich alle Events, aktive und archivierte, jeweils mit dem über den Kern und `Clock` berechneten Status „aktiv“ oder „archiviert“

**Angenommen** ein archiviertes Event
**Wenn** ich sein Datum in die Zukunft korrigiere
**Dann** zeigt die Liste es danach als aktiv

**Angenommen** das Programm startet
**Wenn** die Migrationen gelaufen sind
**Dann** berechnet der Kern `effective*` für alle Events neu, bevor der HTTP-Server startet (AD-16)

**Angenommen** die Neuberechnung schlägt für ein Event fehl (z. B. nach einer Regel- oder tzdata-Änderung)
**Wenn** das Programm startet
**Dann** behält das Event seine gespeicherten Werte, der Fehler wird mit der Event-Kennung geloggt, und das Programm startet trotzdem (ENT-5)
**Und** die Event-Liste im Admin markiert das Event mit „prüfen“, bis es erfolgreich gespeichert wird

**Außerdem gilt:**
- Die Codes der Event-Typen sind Konstanten im Kern.
- Der Ort wird aus den vorhandenen Orten ausgewählt; das Event speichert nur die `location_id`, keine Kopie von Adresse oder Koordinaten.
- Die Quelle ist im Kern ein Objekt aus Beschreibung und optionalem Link (ENT-10).
- Das Event hat keine Felder für Personen. Unter Titel, Notiz und Quelle weist ein Hinweis darauf hin, dass keine Privatpersonen, Kontaktpersonen oder Telefonnummern eingetragen werden (NFR-4).
- Das SQL enthält keine Fachlogik und kein `now()` (AD-2).
- Die Migration dieser Story entfernt die Spalte `locations.address` und die Defaults von `street`, `postal_code` und `city` (contract nach Story 1.12, AD-17). Vorher sind alle Orte in Produktion vollständig gepflegt oder gelöscht.

### Story 1.8: Neuen Ort direkt beim Anlegen eines Events

Als Admin,
möchte ich beim Erfassen eines Events einen fehlenden Ort sofort anlegen,
damit ich das Event-Formular nicht verlassen und neu ausfüllen muss.

**Deckt ab:** FR-15

**Acceptance Criteria:**

**Angenommen** ich fülle das Event-Formular aus und der Ort fehlt in der Auswahl
**Wenn** ich „Neuer Ort“ wähle
**Dann** öffnet sich im Formular (htmx) die Ortseingabe mit Kartenpicker aus Story 1.4 und 1.5

**Angenommen** ich habe in der Ortseingabe einen gültigen neuen Ort eingegeben
**Wenn** ich ihn speichere
**Dann** ist er im Event-Formular ausgewählt
**Und** meine bisherigen Event-Eingaben sind erhalten

**Angenommen** der neue Ort ist ungültig oder sein Name existiert bereits
**Wenn** ich ihn speichere
**Dann** sehe ich dieselben Meldungen wie in Story 1.4, und das Event-Formular bleibt unverändert

**Außerdem gilt:**
- Der Ort wird über denselben Anwendungsfall `SaveLocation` angelegt wie in der Ortsverwaltung.

### Story 1.9: Ablaufplan pflegen

Als Admin,
möchte ich einem Event einen Ablaufplan mit Programmpunkten geben,
damit zum Beispiel ein Festprogramm Schritt für Schritt sichtbar ist.

**Deckt ab:** FR-4, AD-15, ENT-3, ENT-15, ENT-20

**Datenmodell:** Tabelle `timetable_entries` mit `id` (UUIDv7), `event_id` (Fremdschlüssel mit Löschweitergabe), `description`, `date`, `start_time` und `end_time`.

**Acceptance Criteria:**

**Angenommen** das Event-Formular
**Wenn** ich Programmpunkte hinzufüge oder entferne
**Dann** geht das ohne Neuladen der Seite (htmx)
**Und** jeder Punkt hat Beschreibung, Datum, optional eine Beginn- und optional eine End-Uhrzeit

**Angenommen** ein Programmpunkt mit `endTime` vor `startTime` (z. B. 22:00 bis 01:00)
**Wenn** ich speichere
**Dann** gilt das Ende als Uhrzeit am Folgetag (ENT-3)

**Angenommen** ein Programmpunkt ohne Beschreibung oder Datum, außerhalb von `[effectiveStart, effectiveEnd]` (ENT-20) oder mit einer Uhrzeit in der Lücke der Zeitumstellung
**Wenn** ich speichere
**Dann** lehnt der Kern das gesamte Speichern mit einer deutschen Meldung am betroffenen Punkt ab
**Und** „im Zeitraum“ heißt: Der über `ToInstant` berechnete Beginn und das Ende des Punkts liegen in `[effectiveStart, effectiveEnd)` des Events; ein Punkt ohne Uhrzeit liegt im Zeitraum, wenn sein Datum einer der Tage des Events ist

**Angenommen** ein Event mit Ablaufplan
**Wenn** ich Beginn oder Ende des Events so ändere, dass Programmpunkte außerhalb liegen
**Dann** lehnt der Kern das Speichern ab, und das Formular nennt die betroffenen Punkte

**Angenommen** ein gültiger Ablaufplan
**Wenn** ich speichere
**Dann** ersetzt `SaveEvent` die bisherige Liste vollständig, zusammen mit dem Event in einer Transaktion über den neuen Port `TxRunner` (AD-15)
**Und** `effective*` des Events ändert sich durch den Ablaufplan nicht

**Angenommen** gespeicherte Programmpunkte
**Wenn** das Event gelesen wird
**Dann** liefert der Kern sie chronologisch sortiert: nach Datum, Punkte ohne Uhrzeit zuerst, dann nach Beginn-Uhrzeit, bei Gleichstand nach End-Uhrzeit, Beschreibung und `id`

### Story 1.10: Warnung bei Duplikatverdacht

Als Admin,
möchte ich gewarnt werden, wenn ich ein Event anlege oder so bearbeite, dass es ein anderes doppelt, und trotzdem bewusst speichern können,
damit keine stillen Duplikate entstehen, aber eine zweite Vorstellung am selben Tag möglich bleibt.

**Deckt ab:** AD-11

**Datenmodell:** Spalte `title_key` in `events`.

**Acceptance Criteria:**

**Angenommen** ein Event wird gespeichert oder das Programm startet
**Wenn** der Kern das Event verarbeitet
**Dann** berechnet er `title_key` mit `NormalizeKey(title)` und speichert ihn mit
**Und** vorhandene Events werden mit der Neuberechnung beim Start nachgezogen (Fehler wie in ENT-5)

**Angenommen** ein vorhandenes Event, auch ein archiviertes, mit gleichem `title_key`, gleichem `startDate` und gleichem Ort
**Wenn** ich ein neues Event mit diesen Angaben speichere oder ein anderes Event so bearbeite
**Dann** speichert `SaveEvent` mit der Policy `rejectDuplicates` nicht und meldet `ErrDuplicateSuspect` mit den Kandidaten
**Und** der Admin zeigt eine Warnung mit Links auf die vorhandenen Events

**Angenommen** die Warnung
**Wenn** ich „Trotzdem speichern“ bestätige
**Dann** wird mit der Policy `allowDuplicates` gespeichert
**Und** breche ich ab, bleiben meine Eingaben erhalten

**Angenommen** ein gleicher Titel an einem anderen Datum oder an einem anderen Ort
**Wenn** ich speichere
**Dann** gibt es keine Warnung
**Und** eine andere Uhrzeit am selben Datum löst die Warnung aus, weil nur das Datum verglichen wird

**Außerdem gilt:**
- Beim Bearbeiten zählt das Event selbst nicht als Kandidat.
- `FindDuplicateCandidates` ist mit Unit-Tests ohne Datenbank abgedeckt.

### Story 1.11: Events und Orte löschen

Als Admin,
möchte ich Events und nicht mehr benötigte Orte löschen,
damit der Bestand sauber bleibt, ohne dass Events ihren Ort verlieren.

**Deckt ab:** FR-7, FR-15, AD-6

**Acceptance Criteria:**

**Angenommen** ein Event, aktiv oder archiviert
**Wenn** ich es lösche und die Rückfrage bestätige
**Dann** entfernt `DeleteEvent` das Event zusammen mit seinem Ablaufplan

**Angenommen** ein Ort ohne Events
**Wenn** ich ihn lösche und bestätige
**Dann** entfernt `DeleteLocation` den Ort

**Angenommen** ein Ort, auf den noch Events verweisen, auch nur archivierte
**Wenn** ich ihn löschen will
**Dann** lehnt der Kern mit `ErrConflict` ab, und der Admin nennt die Zahl der betroffenen Events

**Angenommen** ein Event oder Ort ist bereits gelöscht (z. B. Doppelklick oder zweiter Tab)
**Wenn** ich es noch einmal lösche
**Dann** meldet der Kern `ErrNotFound`, und der Admin zeigt eine deutsche Meldung „nicht mehr vorhanden“ statt einer Fehlerseite

**Außerdem gilt:**
- Ein Test belegt, dass auch ein direktes SQL-`DELETE` auf einen referenzierten Ort am Fremdschlüssel `ON DELETE RESTRICT` scheitert.

### Story 1.12: Adresse in Straße, PLZ und Ort aufteilen

Als Admin,
möchte ich die Adresse eines Orts in Straße, Postleitzahl und Ort getrennt pflegen,
damit Abnehmer die Adressteile ohne Raten aus einem Freitext lesen können.

**Deckt ab:** FR-6, AD-2, AD-11, AD-17

**Hinweis:** Diese Story zieht Story 1.4 nach (Sprint Change Proposal vom 2026-10-02). Sie läuft vor Story 1.5.

**Datenmodell:** Neue Migration `00003`. Die Spalten `street`, `postal_code` und `city` (`text NOT NULL DEFAULT ''`) kommen hinzu, `address` wird nullable und vom Code nicht mehr gelesen oder geschrieben (expand, AD-17). Die Migration teilt vorhandene Adressen nicht per SQL auf (AD-2). In Produktion gibt es nur Testdaten, die nach dem Deploy nachgepflegt oder gelöscht werden. Erst Story 1.7 entfernt `address` und die Defaults (contract).

**Acceptance Criteria:**

**Angenommen** das Ortsformular im Admin
**Wenn** es sich öffnet
**Dann** hat es statt des Felds „Adresse“ die drei Felder „Straße und Hausnummer“, „PLZ“ und „Ort“

**Angenommen** Straße, PLZ oder Ort fehlen oder bestehen nur aus Leerraum
**Wenn** ich speichere
**Dann** lehnt der Kern mit `ErrValidation` ab, und das Formular zeigt je Feld eine deutsche Meldung, ohne die Eingaben zu verlieren

**Angenommen** eine PLZ, die nicht aus genau fünf Ziffern besteht (z. B. `9051`, `90513a`, `D-90513`)
**Wenn** ich speichere
**Dann** lehnt der Kern sie mit einer eigenen Meldung ab
**Und** Leerraum am Rand wird vorher entfernt (`" 90513 "` ist gültig)

**Angenommen** ein vorhandener Ort
**Wenn** ich Straße, PLZ oder Ort ändere
**Dann** bleibt seine Kennung unverändert

**Angenommen** die Ortsliste
**Wenn** sie angezeigt wird
**Dann** zeigt die Spalte „Adresse“ die Teile zusammengesetzt als „Straße, PLZ Ort“
**Und** ein Testort mit leeren Adressteilen aus der Zeit vor der Migration wird ohne Fehler angezeigt und lässt sich bearbeiten

**Außerdem gilt:**
- Alle drei Felder werden wie jede Texteingabe auf NFC normalisiert (AD-11).
- Die Feldnamen im Kern sind `street`, `postalCode` und `city`. Story 2.1 legt sie in der Spec als Objekt `address` im Ort fest (AD-9).
- Validierung und Normalisierung sind mit Unit-Tests ohne Datenbank abgedeckt, der Postgres-Adapter mit Tests gegen PostgreSQL 18.
- Das Beispiel in der README zeigt den Ort mit dem Objekt `address`.
- Vor dem Deploy wird nach AD-17 ein `pg_dump` gezogen.

## Epic 2: Die Karten-App liest Events über die öffentliche API

Abnehmer fragen ohne Anmeldung ab: „heute“, Zeiträume und Event-Typen, einzelne Events, die Listen der Orte und Event-Typen sowie das Archiv. Das geschieht über einen dokumentierten Vertrag, der als Vorlage für andere OZ-Backends taugt. Die tägliche Bereinigung läuft im Hintergrund. SM-1 (Weihnachtsmarkt) ist gegen eine Fixture prüfbar.

### Story 2.1: API-Vertrag v1 und Liste der Event-Typen

Als Entwickler einer Abnehmer-App,
möchte ich einen öffentlichen, versionierten und dokumentierten API-Vertrag und als ersten Endpunkt die Liste der Event-Typen,
damit ich Kartenfilter und Icons darauf aufbauen und das Muster für eigene Backends übernehmen kann.

**Deckt ab:** FR-5, FR-11, NFR-1 bis NFR-3, KON-1 bis KON-3, KON-7, KON-8, AD-8, AD-9, ENT-18

**Acceptance Criteria:**

**Angenommen** `api/v1/openapi.yaml` (OpenAPI 3.1, Rückfall 3.0.3 nur falls `oapi-codegen` scheitert)
**Wenn** die Spec gelesen wird
**Dann** beschreibt `info` die Konventionen KON-1 bis KON-8, die Versionierungsregel (NFR-2) und die Zusage von mindestens 6 Monaten Parallelbetrieb (KON-2)
**Und** `components/schemas` definiert die Enum-Codes für Event-Typ, Zeitgenauigkeit und Ortsgenauigkeit, die Listenhülle `{ "data": [ … ] }` und das Fehlerformat nach RFC 9457
**Und** jede Feldbedeutung ist auf Englisch beschrieben

**Angenommen** die Spec
**Wenn** `oapi-codegen` v2.8.0 (`std-http-server` + `strict-server`) läuft
**Dann** liegt das erzeugte Gerüst eingecheckt in `internal/adapter/publicapi/v1`
**Und** die CI schlägt fehl, wenn der eingecheckte Code nicht dem aktuell erzeugten entspricht
**Und** ein CI-Test schlägt fehl, wenn die Enum-Konstanten im Kern von den Codes der Spec abweichen (AD-9)

**Angenommen** das laufende Programm
**Wenn** ein Abnehmer `GET /v1/event-types` aufruft
**Dann** erhält er über `ListEventTypes` alle sieben Codes mit je einer deutschen Beschriftung in der Hülle `{ "data": [ … ] }`
**Und** `GET /v1/openapi.yaml` liefert die Spec öffentlich aus

**Angenommen** eine beliebige Antwort unter `/v1/…`
**Wenn** ein Browser von einer fremden Herkunft anfragt
**Dann** erlaubt CORS jede Herkunft (`Access-Control-Allow-Origin: *`), und Preflight-Anfragen werden beantwortet
**Und** andere Methoden als `GET`, `HEAD` und `OPTIONS` werden mit 405 abgelehnt, unbekannte Pfade mit 404, beides als `application/problem+json` mit englischem `title`/`detail`

### Story 2.2: entfällt

Gestrichen mit dem Sprint Change Proposal vom 2026-10-04: Die öffentliche API liefert Events nur als Listen und gibt keine Kennungen aus (FR-11). Die Leseform `Event`, die Schreibform `EventInput` und FR-10 setzt Story 2.3 um.

### Story 2.3: Events von heute und nach Zeitraum und Typ abfragen

Als Karten-App,
möchte ich ohne Filter die Events von heute bekommen und optional nach Zeitraum und Event-Typen filtern,
damit ich zeigen kann, was heute oder in einem bestimmten Zeitraum in Zirndorf los ist.

**Deckt ab:** FR-8, FR-9, FR-10, FR-11, KON-4, KON-5, KON-6, AD-7, AD-14, AD-16, SM-3, ENT-4, ENT-10, ENT-18

**Acceptance Criteria:**

**Angenommen** die Spec ist um das Schema `Event` erweitert
**Wenn** ein Abnehmer `GET /v1/events` aufruft
**Dann** hat jedes Event der Liste die Leseform nach AD-14: `title`, `type`, `location` (vollständig: `name`, `address` als Objekt aus `street`, `postalCode` und `city`, `latitude`, `longitude`, `precision`, `note`), `startDate`, `startTime`, `endDate`, `endTime`, `allDay`, `startPrecision`, `endPrecision`, `source` (Objekt aus `description` und `url`, ENT-10), `note`, `timetable` (Einträge aus `description`, `date`, `startTime`, `endTime`), `effectiveStart`, `effectiveEnd`, `archived` (seit Story 2.8 ohne `archived`)
**Und** Uhrzeiten sind `HH:MM` oder `null`, Datumswerte `YYYY-MM-DD`, `effective*` ISO 8601 mit Offset
**Und** der Ablaufplan ist chronologisch sortiert
**Und** kein Event, kein Ort und kein Programmpunkt enthält eine Kennung, einen `importKey`, `archivedAt`, `title_key` oder `name_key`

**Angenommen** zwei Events am selben Ort
**Wenn** beide in der Liste stehen
**Dann** tragen sie denselben Ortsnamen und identische Koordinaten
**Und** nach einer Änderung der Koordinaten im Admin liefern beide die neuen Koordinaten (FR-7)

**Angenommen** die Spec enthält `GET /v1/events` mit den Parametern `from`, `to` (jeweils Datum oder Zeitpunkt) und `type` (wiederholbar) erweitert
**Wenn** ein Abnehmer ohne Parameter abfragt
**Dann** liefert `ListActiveEvents` alle Events mit `effectiveEnd > now`, deren Zeitraum den heutigen Tag (Europe/Berlin) berührt
**Und** ein mehrtägiges Event ist an jedem seiner Tage enthalten
**Und** ein Event, das heute um 14:00 endet, ist ab 14:00 nicht mehr enthalten

**Angenommen** `from` und/oder `to`
**Wenn** der Kern den Filter normalisiert (AD-16, ENT-4)
**Dann** wird daraus ein halboffenes Intervall `[lo, hi)`
**Und** ein reines Datum zählt als ganzer Tag inklusive
**Und** fehlt nur `to`, ist der Zeitraum nach hinten offen; fehlt nur `from`, beginnt er mit dem heutigen Tag (ohne beide gilt das erste Szenario: nur heute)
**Und** ein Zeitpunkt in `to` wird auf die Minute abgeschnitten und um eine Minute erhöht, sodass ein Event, das genau zum Zeitpunkt `to` beginnt, enthalten ist
**Und** das einzige Filterprädikat ist `effectiveStart < hi AND effectiveEnd > lo`
**Und** auch mit Filter enthält die Antwort nur aktive Events

**Angenommen** `type=market&type=club`
**Wenn** abgefragt wird
**Dann** enthält die Antwort Events beider Typen (ODER)
**Und** ein mehrfach genannter Typ zählt einmal

**Angenommen** ein ungültiges Datum, ein Zeitpunkt ohne Offset, ein unbekannter oder leerer Typ (`type=`) oder ein normalisiertes Intervall mit `lo >= hi`
**Wenn** abgefragt wird
**Dann** antwortet die API mit 400 als Problem Details, und `detail` nennt den betroffenen Parameter

**Angenommen** nur `to` ist angegeben und liegt vor dem heutigen Tag
**Wenn** abgefragt wird
**Dann** antwortet die API mit 400, und `detail` erklärt, dass die reguläre Abfrage nur aktive Events liefert, und verweist auf `/v1/archive/events`

**Außerdem gilt:**
- `components/schemas` enthält zusätzlich die Schreibform `EventInput` nach AD-14 (mit `location` als mitgebrachtem Ort und `importKey`, ENT-18), mit denselben Feldnamen und Formaten wie `Event`, damit das Import-Schema in Epic 3 sie per `$ref` einbinden kann (AD-9). Sie enthält keine Kennungen.
- Die Spec beschreibt bei `location.name`, dass der Name einen Ort eindeutig kennzeichnet und zum Gruppieren dient, und im `info`-Abschnitt, dass die API nur Listen liefert und keine Kennungen ausgibt.
- Der Handler bildet nur Kern-Objekte auf die erzeugten Typen ab und leitet nichts selbst ab (AD-7). Ein Test belegt, dass die JSON-Antwort keine Felder außer denen der Leseform enthält.
- Die Liste ist nach `effectiveStart` aufsteigend sortiert, bei Gleichstand nach der internen `id`, in der Hülle `{ "data": [ … ] }`.
- Die Prüfung `lo >= hi` geschieht nach der Normalisierung; `from=2026-12-24T18:00+01:00&to=2026-12-24` ist gültig.
- Kern-Tests mit fester `Clock` decken die Beispiele aus FR-8 und FR-9 ab, ein Postgres-Test prüft das Prädikat gegen PostgreSQL 18.

### Story 2.4: Archiv abfragen

Als Abnehmer,
möchte ich vergangene Events abfragen, gefiltert wie die reguläre Abfrage,
damit ich zum Beispiel zurückliegende Feste anzeigen kann und kein Event durch eine Lücke fällt.

**Deckt ab:** FR-12, KON-6, ENT-4

**Acceptance Criteria:**

**Angenommen** die Spec ist um `GET /v1/archive/events` mit denselben Parametern wie `GET /v1/events` erweitert
**Wenn** ein Abnehmer ohne Parameter abfragt
**Dann** liefert `ListArchivedEvents` alle Events mit `effectiveEnd <= now`

**Angenommen** `from`, `to` und `type`
**Wenn** abgefragt wird
**Dann** gelten dasselbe Prädikat, dieselbe Behandlung von Datum und Zeitpunkt und dieselben Fehlerantworten wie in Story 2.3
**Und** abweichend davon ist ein fehlendes `from` offen und ein fehlendes `to` gleich `now` (ENT-4)
**Und** `GET /v1/archive/events?to=2026-06-30` liefert alle archivierten Events, die sich mit der Zeit bis einschließlich 2026-06-30 überschneiden

**Angenommen** ein Event, das seit einer Minute vorbei ist
**Wenn** das Archiv abgefragt wird
**Dann** ist es enthalten, auch wenn noch keine Bereinigung gelaufen ist

**Angenommen** eine feste `Clock` und ein gemischter Bestand mit vergangenen, laufenden und zukünftigen Events
**Wenn** `ListActiveEvents` ohne Untergrenze und `ListArchivedEvents` ohne Begrenzung verglichen werden (Kern-Test)
**Dann** ist jedes Event in genau einer der beiden Listen enthalten (seit Story 2.7: jedes Event ohne „prüfen“)
**Und** derselbe Vergleich über die API nutzt `GET /v1/events?from=1900-01-01` und `GET /v1/archive/events`

**Außerdem gilt:**
- Archivierte Events haben dieselbe Struktur wie aktive, mit `archived: true` (seit Story 2.8 ohne `archived`).
- Die Liste ist nach `effectiveStart` absteigend sortiert, bei Gleichstand nach `id`.

### Story 2.5: Tägliche Bereinigung und vollständige Neuberechnung beim Start

Als Admin,
möchte ich, dass vergangene Events täglich als archiviert markiert werden, ohne dass sich die API-Antworten ändern, und dass der Start alle abgeleiteten Werte nachzieht,
damit ich den Bestand auswerten kann, nichts verloren geht und Regeländerungen überall greifen.

**Deckt ab:** FR-13, AD-5, AD-13, AD-15, AD-16, ENT-5, ENT-17

**Datenmodell:** Spalte `archived_at` in `events`.

**Acceptance Criteria:**

**Angenommen** das Programm läuft
**Wenn** der Job läuft (beim Programmstart und danach einmal täglich)
**Dann** setzt der Job in `adapter/cleanup` über einen Kern-Anwendungsfall und mit der Zeit aus `Clock` das Feld `archived_at` bei allen Events mit `effectiveEnd <= now` und leerem `archived_at`
**Und** er schreibt die Anzahl markierter Events ins Log

**Angenommen** der Job läuft mehrfach hintereinander
**Wenn** sich nichts geändert hat
**Dann** ändert er nichts (idempotent)

**Angenommen** ein Bestand mit fester `Clock`
**Wenn** die Antworten aller `/v1`-Endpunkte vor und nach dem Job verglichen werden
**Dann** sind sie identisch
**Und** keine Lese-Abfrage und kein API-Feld nutzt `archived_at`

**Angenommen** ein Event mit gesetztem `archived_at`
**Wenn** `SaveEvent` es so ändert, dass es wieder aktiv ist
**Dann** wird `archived_at` geleert

**Angenommen** der Job und `SaveEvent` laufen gleichzeitig für dasselbe Event
**Wenn** `SaveEvent` das Event wieder aktiv macht
**Dann** setzt der Job `archived_at` nur über eine einzige Anweisung, die die Bedingung `effective_end <= $now AND archived_at IS NULL` selbst prüft, sodass ein wieder aktives Event kein `archived_at` behält

**Angenommen** ein Ort, dessen gespeicherter `name_key` nicht `NormalizeKey(name)` entspricht
**Wenn** das Programm startet
**Dann** berechnet `RecomputeDerived` den Schlüssel neu und speichert ihn, bevor `MarkArchived` und der HTTP-Server starten

**Angenommen** zwei Orte, deren neu berechnete `name_key` gleich wären
**Wenn** das Programm startet
**Dann** behalten beide ihren gespeicherten Schlüssel, der Fehler wird mit beiden Ort-Kennungen geloggt, und das Programm startet trotzdem
**Und** die Ortsliste im Admin markiert beide mit „prüfen“, bis einer von ihnen erfolgreich gespeichert oder gelöscht ist
**Und** der Kern erkennt solche Kollisionen vorab über alle Orte; ein trotzdem gemeldetes `ErrConflict` der Datenbank wird genauso behandelt

**Angenommen** ein Event mit Ablaufplan, dessen neu berechneter Zeitraum (z. B. nach einer Regel- oder tzdata-Änderung) einen Programmpunkt ausschließt (AD-15, ENT-20)
**Wenn** das Programm startet
**Dann** behält das Event seine gespeicherten Werte, der Fehler nennt die Event-Kennung und den betroffenen Punkt, und die Event-Liste markiert es mit „prüfen“ (ENT-5)

**Außerdem gilt:**
- Archivierte Events werden nie automatisch gelöscht.
- Ein Fehler im Job beendet das Programm nicht, sondern wird geloggt, und der nächste Lauf versucht es erneut.
- `RecomputeDerived` läuft vor dem HTTP-Server und schreibt deshalb ohne `TxRunner`; das ist die einzige Ausnahme von der Hülle nach AD-6. Seit Story 3.3 läuft sie in einer Transaktion unter der Sperre aus AD-6 (R3 der Retro Epic 2).
- Die Fälle der Neuberechnung sind mit Kern-Tests ohne Datenbank abgedeckt (fester Bestand, Fake-Repos); ein Postgres-Test belegt die Neuberechnung von `name_key`.

### Story 2.6: Lesbare API-Dokumentation und Abnahme

Als Entwicklerin eines anderen OZ-Backends,
möchte ich die API-Dokumentation im Browser lesen und die Konventionen ohne Rückfrage übernehmen können,
damit der Event Store als Vorlage taugt.

**Deckt ab:** NFR-3, SM-1 (Fixture), SM-4, ENT-14, ENT-23

**Acceptance Criteria:**

**Angenommen** das laufende Programm
**Wenn** ich `/v1/docs` öffne
**Dann** sehe ich die Spec als lesbare HTML-Dokumentation, gerendert mit Redoc in einer gepinnten Version, ausgeliefert aus `adapter/publicapi/v1/static`, nicht über ein CDN (ENT-14)
**Und** von dort ist auch `/v1/openapi.yaml` verlinkt

**Angenommen** die Spec
**Wenn** ich sie durchsehe
**Dann** haben alle Endpunkte, Parameter und Felder eine englische Beschreibung und Beispiele
**Und** Zeitgenauigkeit und Ortsgenauigkeit sind mit Bedeutung und Beispiel erklärt, ebenso die Vorbei-Regel, `effective*`, `archived` (seit Story 2.8 entfallen), die Standardwerte von `from`/`to` je Endpunkt (ENT-4) und die Regel zu `allDay` (ENT-2)
**Und** es gibt Beispiele für Fehlerantworten

**Angenommen** eine Fixture mit dem Weihnachtsmarkt 2026 und eine feste `Clock`
**Wenn** `GET /v1/events?from=2026-11-29&to=2026-12-24` aufgerufen wird
**Dann** enthält die Antwort ihn mit Zeitraum, Ort, Zeitgenauigkeit, Ortsgenauigkeit und Quelle (SM-1)

**Außerdem gilt:**
- Die Prüfung durch ein OZ-Mitglied (SM-4) ist als manueller Abnahmeschritt vermerkt, nicht als Bedingung für den Abschluss.
- Die Prüfung von SM-1 in Produktion folgt in Story 3.5.

### Story 2.7: Events mit „prüfen“ öffentlich ausblenden

Als Karten-App,
möchte ich keine Events bekommen, deren Zeitraum nach einer Regeländerung nicht neu berechnet werden konnte,
damit ich keine Termine mit veraltetem Zeitraum oder in der falschen Liste zeige.

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

### Story 2.8: `archived` aus der Leseform entfernen

Als Karten-App,
möchte ich Events ohne ein Feld bekommen, das nur wiederholt, welche Liste ich abgefragt habe,
damit ich keinen Wert zwischenspeichere, der mit der Zeit falsch wird.

**Deckt ab:** FR-11, FR-12, NFR-2, AD-5, AD-7, AD-14

**Hinweis:** Sprint Change Proposal 2026-10-06 Teil B. Einmalige Ausnahme von NFR-2, weil v1 noch keinen Abnehmer hat. Die Story startet nach dem Merge von Story 3.5.

**Acceptance Criteria:**

**Angenommen** die Spec
**Wenn** ich das Schema `Event` lese
**Dann** enthält es kein Feld `archived`, weder in `properties` noch in `required`
**Und** die Beispiele von `GET /v1/events` und `GET /v1/archive/events` enthalten es nicht
**Und** der Abschnitt „Time model“ und die Beschreibung von `/v1/archive/events` erklären, dass die Liste angibt, ob ein Event vorbei ist, und dass Abnehmer, die Events zwischenspeichern, `effectiveEnd` gegen die aktuelle Zeit prüfen

**Angenommen** ein aktives und ein vergangenes Event
**Wenn** ein Abnehmer `GET /v1/events` und `GET /v1/archive/events` aufruft
**Dann** enthält kein Event der beiden Antworten ein Feld `archived`
**Und** der Leseform-Test aus Story 2.3 prüft die Schlüsselmenge jeder Ebene ohne `archived`

**Angenommen** eine feste `Clock` und ein gemischter Bestand
**Wenn** beide Listen verglichen werden (Kern-Test und API-Vergleich aus Story 2.4)
**Dann** ist jedes Event ohne „prüfen“ weiter in genau einer der beiden Listen

**Außerdem gilt:**
- `api.gen.go` wird mit `go generate ./...` neu erzeugt, nicht von Hand geändert (AD-8).
- Der Kern bleibt unverändert: `ListedEvent.Archived` wählt weiter das Archiv aus, die Admin-Liste zeigt weiter „archiviert“. `archived_at` und `MarkArchived` bleiben (AD-5, AD-13).
- Die Abnahmetests in `cmd/eventstore` folgen der Spec. Das README zeigt `archived` schon seit dem Planungs-PR nicht mehr.
- Nahtstellen: Leseform-Test und Abbildung in `publicapi/v1` (2.3), Beschreibung des Archivs und API-Vergleich (2.4), „Time model“ in der Spec (2.6, 2.7), Abnahme der Testsammlung in `cmd/eventstore` (3.5).

## Epic 3: Andreas importiert Recherchen per JSON-Datei

Andreas lädt eine Import-Datei hoch, sieht die Vorschau, entscheidet Duplikatverdachtsfälle und übernimmt alles in einem Schritt mit Zusammenfassung. SM-2 ist erfüllt, und SM-1 ist in Produktion bestätigt.

### Story 3.1: Import-Format v1 und Prüfung der Datei

Als Admin,
möchte ich ein dokumentiertes, versioniertes Import-Format und einen Upload, der meine Datei prüft und Fehler je Eintrag meldet,
damit ich Recherche-Dateien zuverlässig im richtigen Format erstellen kann.

**Deckt ab:** FR-16, AD-9, ENT-13, ENT-18, ENT-24

**Acceptance Criteria:**

**Angenommen** `api/v1/import-v1.schema.json`
**Wenn** das Schema gelesen wird
**Dann** verlangt es `formatVersion` und eine Liste von Events
**Und** es bindet `EventInput` und die Enum-Codes per `$ref` aus `openapi.yaml` ein, statt sie zu kopieren (AD-9)
**Und** jedes Event bringt seinen Ort als `location` mit (ENT-18); eine Kennung (`locationId`) gibt es nicht
**Und** `location` hat immer `name`; `address`, `latitude`, `longitude` und `precision` sind im Schema optional, weil ein Ort mit vorhandenem Namen sie nicht braucht; `note` ist optional
**Und** das Schema enthält keine Kennungen
**Und** `address` ist dasselbe Objekt wie in der Leseform (`street`, `postalCode`, `city`), per `$ref` aus `openapi.yaml` eingebunden (AD-9)
**Und** jedes Event kann einen `importKey` haben
**Und** `GET /v1/import-v1.schema.json` liefert das Schema öffentlich aus

**Angenommen** die Kern-Konstanten für Feldnamen (`EventField*`, `FilterField*`)
**Wenn** die CI läuft
**Dann** schlägt ein Test fehl, wenn ein Feldname, den der Kern in Fehlern oder Filtern nennt, nicht als Feld von `EventInput`/`Event` oder als Parameter in `openapi.yaml` vorkommt (AD-9)
**Und** Feldnamen, die nur das Admin-Formular kennt (z. B. `locationId`), sind im Test ausdrücklich als Ausnahme gelistet

**Angenommen** ich bin angemeldet
**Wenn** ich im Admin eine Import-Datei hochlade
**Dann** parst und validiert der Kern sie mit denselben Regeln wie `SaveEvent` (FR-1 bis FR-6) über denselben Typ `core.EventInput`

**Angenommen** eine Datei mit fehlender oder unbekannter `formatVersion`, kein gültiges JSON, größer als 2 MB oder ohne Einträge
**Wenn** ich sie hochlade
**Dann** wird die ganze Datei mit einer klaren deutschen Meldung abgelehnt

**Angenommen** eine Datei mit gültiger Version, in der einzelne Einträge fehlerhaft sind
**Wenn** ich sie hochlade
**Dann** werden genau diese Einträge mit Position, Titel und Grund gemeldet, die übrigen gelten als gültig
**Und** fehlerhaft sind unter anderem: ungültiges Datum, unbekannter Typ, ein Event ohne `location` oder ohne Ortsnamen, ein mitgebrachter neuer Ort ohne Adresse, Koordinaten oder Ortsgenauigkeit, ein neuer Ort mit unvollständiger Adresse (Straße, PLZ oder Ort fehlt) oder ungültiger PLZ

**Angenommen** ein Eintrag mit einem Text über der Grenze aus ENT-24 oder mehr als 100 Programmpunkten
**Wenn** ich die Datei hochlade
**Dann** ist genau dieser Eintrag fehlerhaft, und der Grund nennt Feld und Grenze

**Angenommen** das Event- oder Ortsformular im Admin
**Wenn** ich einen Text über der Grenze speichere
**Dann** lehnt der Kern mit `ErrValidation` ab, und das Formular zeigt die deutsche Meldung am Feld, ohne die Eingaben zu verlieren

**Außerdem gilt:**
- In dieser Story wird noch nichts in den Bestand geschrieben.
- Parser und Validierung sind mit Unit-Tests ohne Datenbank abgedeckt.
- Die Formulargrenze des Event-Formulars (`maxEventFormBytes`, heute 64 KiB) wird so angehoben, dass ein Event mit allen Feldern an der Grenze aus ENT-24 hineinpasst (256 KiB); maßgeblich sind die Grenzen des Kerns.
- Das Hinzufügen von `maxLength`/`maxItems` zu `EventInput` ist in v1 zulässig, weil die Schreibform erst mit Epic 3 benutzt wird (NFR-2).

### Story 3.2: Import-Vorschau mit Klassifizierung

Als Admin,
möchte ich vor dem Übernehmen für jedes Event sehen, ob es neu ist, eine Aktualisierung, unverändert, ein Duplikatverdacht oder fehlerhaft, und welche Orte neu angelegt würden,
damit nichts stillschweigend verdoppelt oder verworfen wird.

**Deckt ab:** FR-16, FR-17, NFR-4, AD-10, ENT-11, ENT-18, ENT-19

**Datenmodell:** Spalte `import_key` in `events` (optional, eindeutig, wenn gesetzt).

**Acceptance Criteria:**

**Angenommen** eine gültige Datei ist hochgeladen
**Wenn** klassifiziert wird
**Dann** ordnet der Kern jeden Eintrag gegen den gesamten Bestand, archivierte Events eingeschlossen, genau einer Klasse zu: `new`, `update`, `unchanged`, `duplicateSuspect` oder `error`
**Und** es gilt der Vorrang `error` vor `update`/`unchanged` vor `duplicateSuspect` (ENT-11)

**Angenommen** der `importKey` eines Eintrags kommt im Bestand vor
**Wenn** klassifiziert wird
**Dann** ist der Eintrag `update` mit dem vorhandenen Event als Ziel, oder `unchanged`, wenn sich kein Feld unterscheidet
**Und** bei `update` zeigt die Vorschau die geänderten Felder mit altem und neuem Wert
**Und** passt der Eintrag zusätzlich nach Titel, Datum und Ort zu einem anderen Event, zeigt die Vorschau das als Hinweis, ohne die Klasse zu ändern

**Angenommen** ein Eintrag ohne passenden `importKey`, dessen Titel (über `NormalizeKey`), Beginn-Datum und Ort mit einem vorhandenen Event übereinstimmen
**Wenn** klassifiziert wird
**Dann** ist er `duplicateSuspect` mit den Kandidaten aus `FindDuplicateCandidates`
**Und** gleicher Titel an einem anderen Datum ist `new`

**Angenommen** Konflikte innerhalb der Datei
**Wenn** klassifiziert wird
**Dann** macht ein doppelter `importKey` beide Einträge zu `error`
**Und** untereinander gleiche Einträge werden zu `duplicateSuspect`; als gleicher Ort zählt derselbe `NormalizeKey` des Ortsnamens
**Und** ein mehrfach mitgebrachter neuer Ort (gleicher `NormalizeKey`) wird nur einmal als `newLocation` geführt

**Angenommen** ein mitgebrachter Ort, dessen Name im Bestand existiert
**Wenn** klassifiziert wird
**Dann** wird er dem vorhandenen Ort zugeordnet, und der vorhandene Ort bleibt unverändert
**Und** weichen Adresse oder Koordinaten ab, zeigt die Vorschau einen Hinweis

**Angenommen** die Vorschau im Admin
**Wenn** sie angezeigt wird
**Dann** sehe ich die Anzahl je Klasse, jeden Eintrag mit Klasse, Grund oder Ziel-Event (verlinkt) und die neu anzulegenden Orte gesondert
**Und** ein Hinweis erinnert daran, Titel, Notizen und Quellen auf Personendaten zu prüfen (NFR-4)
**Und** der Server speichert keinen Zwischenstand (AD-10)

**Außerdem gilt:**
- Die Klassifizierung ist mit Unit-Tests ohne Datenbank abgedeckt, einschließlich aller Konflikte innerhalb der Datei und des Vorrangs der Klassen.

### Story 3.3: Duplikate entscheiden und Import übernehmen

Als Admin,
möchte ich jeden Duplikatverdacht entscheiden und den Import dann in einem Schritt übernehmen, mit einer Zusammenfassung danach,
damit meine Recherche vollständig und ohne stille Duplikate im Bestand landet.

**Deckt ab:** FR-18, AD-6, AD-10, ENT-11, ENT-12, ENT-15

**Acceptance Criteria:**

**Angenommen** die Vorschau
**Wenn** ein Eintrag `duplicateSuspect` gegenüber einem vorhandenen Event ist
**Dann** wähle ich „überspringen“, „als neues Event anlegen“ oder „vorhandenes überschreiben“

**Angenommen** ein Eintrag ist nur gegenüber einem anderen Eintrag derselben Datei `duplicateSuspect`
**Wenn** ich entscheide
**Dann** stehen nur „überspringen“ und „als neues Event anlegen“ zur Wahl, weil es kein Ziel-Event gibt

**Angenommen** ich speichere den Import
**Wenn** das Formular abgeschickt wird
**Dann** enthält es den ursprünglichen Dateiinhalt als verstecktes Feld und je Eintrag (nach Position) die Entscheidung, die beim Upload ermittelte Klasse und gegebenenfalls die Ziel-ID
**Und** `CommitImport` klassifiziert jeden Eintrag und jeden neuen Ort erneut
**Und** weicht die Klasse oder die Ziel-ID ab, oder existiert ein als neu geführter Ort inzwischen, wird der Eintrag nicht übernommen und als `stale` gemeldet

**Angenommen** zwei Einträge wollen dasselbe Ziel-Event überschreiben oder aktualisieren
**Wenn** übernommen wird
**Dann** werden beide vor der Transaktion als `error` aussortiert

**Angenommen** Einträge mit `error`, `stale`, `unchanged` oder einem Duplikatverdacht ohne Entscheidung
**Wenn** übernommen wird
**Dann** werden sie vor der Transaktion aussortiert und blockieren die übrigen nicht

**Angenommen** die übrigen Einträge
**Wenn** übernommen wird
**Dann** schreibt der Kern sie über `TxRunner` in einer Transaktion: neue Orte einmal anlegen; `new` und „als neues anlegen“ neu anlegen; `update` und „überschreiben“ ersetzen das Ziel-Event vollständig samt Ablaufplan
**Und** alle Schreibvorgänge nutzen `allowDuplicates` (ENT-12)
**Und** beim Überschreiben bekommt das Ziel-Event den `importKey` des Eintrags; hat der Eintrag keinen, bleibt der Schlüssel des Ziel-Events; hat das Ziel-Event einen anderen Schlüssel, ist der Eintrag `error`
**Und** ein Event, das durch die Übernahme wieder aktiv wird, verliert sein `archived_at`

**Angenommen** übernehmbare Einträge
**Wenn** die Transaktion an einem unerwarteten Datenbankfehler scheitert
**Dann** wird nichts übernommen, und der Admin zeigt eine klare Meldung

**Angenommen** `CommitImport` legt neue Orte an
**Wenn** übernommen wird
**Dann** nutzt es den transaktionsgebundenen Kern von `SaveLocation` in derselben Transaktion wie die Events (AD-6)
**Und** Ortsverwaltung und „Neuer Ort“ im Event-Formular nutzen dieselbe Hülle über `TxRunner`, mit unverändertem Verhalten aus Story 1.4 und 1.8

**Angenommen** zwei Schreib-Transaktionen laufen gleichzeitig, z. B. ein Doppelklick auf „Import übernehmen“ oder ein Import neben einem Speichern im Event-Formular
**Wenn** beide dasselbe Event oder denselben neuen Ort betreffen
**Dann** läuft die zweite erst nach dem Commit der ersten, weil `TxRunner` die Sperre aus AD-6 nimmt
**Und** sie klassifiziert neu: Die Einträge des zweiten Imports sind `stale` oder `unchanged`, und es entsteht kein Duplikat

**Angenommen** eine Deploy-Überlappung, in der die alte Instanz ein Event speichert, während die neue startet
**Wenn** `RecomputeDerived` läuft
**Dann** läuft die Neuberechnung von Events und Orten in einer Transaktion unter derselben Sperre wie `TxRunner` (AD-6, AD-16)
**Und** sie liest erst nach Erhalt der Sperre, sodass sie keine gleichzeitig gespeicherten Werte überschreibt
**Und** ein Postgres-Test belegt, dass eine parallele Schreib-Transaktion auf die Neuberechnung wartet

**Angenommen** der Import ist abgeschlossen
**Wenn** die Ergebnisseite erscheint
**Dann** zeigt sie die Anzahl neuer, aktualisierter, unveränderter, übersprungener, nicht entschiedener, fehlerhafter und veralteter Einträge sowie die Zahl neu angelegter Orte
**Und** die Summe der Einträge entspricht der Zahl der Einträge in der Datei (ENT-11)

**Außerdem gilt:**
- Nahtstellen: `SaveLocation` (Story 1.4, 1.8), alle Hüllen über `TxRunner` (`SaveEvent`, `DeleteEvent`, `DeleteLocation`, `MarkArchived`) und `RecomputeDerived` (Story 2.5, 2.5b); die Ausnahme „ohne `TxRunner`“ aus Story 2.5 entfällt. Der Kommentar zur nicht atomaren Namensprüfung in `location_service.go` entfällt.
- Ein Postgres-Test belegt mit zwei parallelen Transaktionen, dass die zweite auf die erste wartet und deren Ergebnis sieht.

### Story 3.4: Testsammlung ins Format v1 überführen

Als Admin,
möchte ich die Testsammlung im Format v1 vorliegen haben, mit nachvollziehbar gewählten Werten,
damit ich sie importieren kann, ohne dass Genauigkeiten geraten werden.

**Deckt ab:** SM-2, SM-C1

**Acceptance Criteria:**

**Angenommen** `zirndorf_events.json` (Stand 2026-09-18)
**Wenn** die Story beginnt
**Dann** wird die Datei unverändert unter `testdata/zirndorf_events.json` ins Repo aufgenommen

**Angenommen** die Quelldatei
**Wenn** sie in das Format v1 überführt wird
**Dann** liegt das Ergebnis als `testdata/zirndorf_events.v1.json` im Repo und ist gegen `import-v1.schema.json` gültig
**Und** jedes Event hat einen stabilen `importKey`, einen Event-Typ und eine Quelle; die Quelle ist aus dem Feld `name` herausgelöst, und der Titel enthält danach keine Quellenangabe mehr
**Und** `00:00` als Platzhalter wird zu einer leeren Uhrzeit
**Und** jedes Event bringt seinen Ort als `location` mit; gleiche Orte sind zusammengeführt (z. B. Paul-Metz-Halle nur einmal mit Adresse und Koordinaten) und haben eine Ortsgenauigkeit
**Und** jede Adresse ist in `street`, `postalCode` und `city` aufgeteilt; der Zusatz „, Germany“ entfällt; Erläuterungen in Klammern (z. B. „Ortsteil-Zentrum, kein exakter Festplatz bekannt“) wandern in die Notiz des Orts

**Angenommen** eine Angabe, die sich aus der Quelle nicht sicher ableiten lässt (Genauigkeit, Typ)
**Wenn** überführt wird
**Dann** wird der vorsichtigere Wert gewählt (SM-C1)
**Und** die Story listet diese Fälle zur Prüfung durch Andreas auf, darunter alle Orte, deren Straße nicht in der Quelle steht (Ortsteile Lind, Wintersdorf, Weinzierlein, Weiherhof: Straße der Ortsmitte; Festmeile: Bereichsangabe) und Ortsteil-Angaben im Feld „Ort“ (z. B. „Zirndorf-Lind“ oder „Zirndorf“)

**Außerdem gilt:**
- Die Story ist abgeschlossen, wenn Andreas die gelisteten Fälle geprüft und freigegeben hat.
- Geklärt (2026-10-06): Die v1-Datei enthält 39 Events; die 42 aus PRD und SM-2 waren ein Zählfehler. Der Weihnachtsmarkt ist nicht in der Datei, sondern im Admin angelegt. Die folgenden Kriterien sprechen von „allen Events der Datei“.

### Story 3.5: Testsammlung importieren und Abnahme

Als Admin,
möchte ich die Testsammlung vollständig und wiederholbar importieren,
damit der Bestand gefüllt ist und SM-1 und SM-2 belegt sind.

**Deckt ab:** SM-1 (Produktion), SM-2

**Acceptance Criteria:**

**Angenommen** ein leerer Bestand und eine feste `Clock` (2026-10-01 12:00 Europe/Berlin)
**Wenn** `testdata/zirndorf_events.v1.json` importiert wird
**Dann** werden alle Events der Datei angelegt
**Und** Events, die zu diesem Zeitpunkt vorbei sind, erscheinen sofort im Archiv-Zugriff

**Angenommen** derselbe Bestand
**Wenn** dieselbe Datei ein zweites Mal importiert wird
**Dann** entstehen 0 neue Events, und alle Einträge sind `unchanged`

**Angenommen** grüne Integrationstests, die importierte Testsammlung und der im Admin angelegte Weihnachtsmarkt in Produktion
**Wenn** `GET /v1/events` mit einem Zeitraum über die Adventszeit abgefragt wird
**Dann** liefert die Antwort die Termine des Weihnachtsmarkts mit Zeitraum, Ort, Zeitgenauigkeit, Ortsgenauigkeit und Quelle (SM-1)

**Außerdem gilt:**
- Beide Importläufe sind als Integrationstest gegen PostgreSQL 18 in der CI abgedeckt; die feste `Clock` hält den Test unabhängig vom Datum.
- Die SM-1-Prüfung in Produktion ist ein manueller Abschlussschritt. Die Testsammlung ist dort seit 2026-10-06 importiert; ein weiterer Import in Produktion ist nicht nötig.
