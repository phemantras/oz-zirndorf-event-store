# Epic 2 Context: Die Karten-App liest Events über die öffentliche API

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

Epic 2 öffnet den in Epic 1 gepflegten Bestand für Abnehmer, zuerst die Karten-App: eine öffentliche, nur lesende, versionierte REST-API unter `/v1/…` ohne Anmeldung. Sie liefert Events ausschließlich als gefilterte Listen („heute“, Zeiträume und Event-Typen, Archiv) und die Liste der Event-Typen, jeweils mit ehrlicher Zeit- und Ortsgenauigkeit und Quelle. Der Vertrag ist spec-first in OpenAPI beschrieben und als Redoc-Seite lesbar, damit andere OZ-Backends Versionierung, Fehlerformat, Zeitformat und Filterkonventionen als Vorlage übernehmen können. Eine tägliche Bereinigung markiert vergangene Events im Hintergrund, ohne dass sich API-Antworten ändern. **Keine Kennungen nach außen** (Sprint Change Proposal 2026-10-04): kein Einzelabruf, keine Ortsliste, keine internen IDs in API und Import-Schema; ein Ort ist an seinem eindeutigen Namen erkennbar.

## Stories

- Story 2.1: API-Vertrag v1 und Liste der Event-Typen
- Story 2.2: entfällt (Sprint Change Proposal 2026-10-04)
- Story 2.3: Events von heute und nach Zeitraum und Typ abfragen
- Story 2.4: Archiv abfragen
- Story 2.5: Tägliche Bereinigung
- Story 2.6: Lesbare API-Dokumentation und Abnahme

## Requirements & Constraints

- **Ressourcen:** `GET /v1/events` (`from`, `to`, `type` wiederholbar), `GET /v1/event-types`, `GET /v1/archive/events`. Kein Schreibzugriff, kein Einzelabruf, keine Ortsliste, keine Volltext- oder Umkreissuche, keine Paginierung.
- **„Heute“ ohne Filter:** alle aktiven Events, deren Zeitraum den heutigen Tag (Europe/Berlin) berührt; mehrtägige an jedem Tag; ein Event, das um 14:00 endet, ist ab 14:00 im Archiv, unabhängig von der Bereinigung.
- **Filter:** Überschneidung mit dem Filterzeitraum; mehrere Typen ODER-verknüpft, doppelter Typ zählt einmal. Reguläre Abfrage liefert auch mit Filter nur aktive Events. Ungültiges Datum, Zeitpunkt ohne Offset, unbekannter oder leerer Typ (`type=`) oder normalisiertes `lo >= hi` → 400, `detail` nennt den Parameter. Nur `to` vor heute in `/v1/events` → 400 mit Verweis auf `/v1/archive/events`.
- **Archiv:** dieselbe Struktur wie aktive Events mit `archived: true`; ein seit einer Minute vergangenes Event ist schon enthalten. Jedes Event liegt zu jedem Zeitpunkt in genau einer der beiden Abfragen. Archivierte Events werden nie automatisch gelöscht.
- **Antwortinhalt:** jedes Event vollständig (alle Event-Angaben plus vollständiger Ort). Zwei Events am selben Ort liefern denselben, eindeutigen Ortsnamen und identische Koordinaten; Koordinatenänderungen im Admin wirken auf alle. Zeitgenauigkeit, Ortsgenauigkeit und Quelle immer enthalten (SM-3). Keine internen Werte: keine Kennungen (Event, Ort, Programmpunkt), kein `importKey`, `archivedAt`, `title_key`, `name_key`.
- **Konventionen (Vertragsteil):** Hauptversion im Pfad; alte Hauptversion mindestens 6 Monate parallel, Abschaltdatum in der Doku; Fehler nach RFC 9457 mit englischem `title`/`detail`; Datum `YYYY-MM-DD`, Uhrzeit `HH:MM` lokal oder `null`, berechnete Zeitpunkte ISO 8601 mit Offset; `from`/`to` Datum oder Zeitpunkt, beide inklusive, ohne `to` offen; aktive Listen aufsteigend, Archiv absteigend; alles Technische englisch, camelCase; Listen in `{ "data": [ … ] }`.
- **Versionierung:** Inkompatible Änderungen (Typenliste, Feldnamen) nur in neuer Hauptversion; neue optionale Felder dürfen in v1 dazukommen.
- **CORS:** `/v1/…` von jeder Herkunft (`Access-Control-Allow-Origin: *`, Preflight beantwortet); andere Methoden als `GET`/`HEAD`/`OPTIONS` → 405, unbekannte Pfade → 404, beides als `application/problem+json`.
- **Doku:** vollständig in OpenAPI, öffentlich; alle Endpunkte, Parameter und Felder englisch beschrieben mit Beispielen, inkl. Fehlerbeispielen. Ausdrücklich erklärt: Zeit- und Ortsgenauigkeit, Vorbei-Regel, `effective*`, `archived`, Standardwerte von `from`/`to` je Endpunkt, `allDay`-Regel.
- **Abnahme:** Fixture mit Weihnachtsmarkt 2026 und fester `Clock`: `GET /v1/events?from=2026-11-29&to=2026-12-24` enthält ihn mit Zeitraum, Ort, Genauigkeiten und Quelle (SM-1). Prüfung durch ein OZ-Mitglied (SM-4) ist manueller Schritt, keine Abschlussbedingung.
- **Betrieb:** Hobby-Niveau, Best Effort; kein Rate Limiting oder Caching in v1.

## Technical Decisions

- **Spec-first:** `api/v1/openapi.yaml` (OpenAPI 3.1, Rückfall 3.0.3 nur falls `oapi-codegen` scheitert) ist einzige Quelle für Pfade, Parameter, Feldnamen, Enum-Codes und Fehlerformat. Gerüst per `oapi-codegen` v2.8.0 (`std-http-server` + `strict-server`) nach `internal/adapter/publicapi/v1`, eingecheckt, nie von Hand geändert. CI prüft „generierter Code aktuell“.
- **Enum-Abgleich:** Codes stehen nur in `components/schemas`; Kern-Konstanten spiegeln sie, ein CI-Test schlägt bei Abweichung fehl. Event-Typen `festival`, `market`, `culture`, `politics`, `club`, `sports`, `other` (je deutsche Beschriftung in `/v1/event-types`); Zeitgenauigkeit `exact`, `dateOnly`, `allDay`; Ortsgenauigkeit `building`, `street`, `area`, `district`.
- **Schemas:** Leseform `Event` mit `title`, `type`, `location` (`name`, `address` als Objekt `{street, postalCode, city}`, `latitude`, `longitude`, `precision`, `note`), `startDate`, `startTime`, `endDate`, `endTime`, `allDay`, `startPrecision`, `endPrecision`, `source` `{description, url|null}`, `note`, `timetable` (chronologisch, Einträge aus `description`, `date`, `startTime`, `endTime`), `effectiveStart`, `effectiveEnd` (Offset `+01:00`/`+02:00`), `archived`. Schreibform `EventInput` als eigenes Schema mit denselben Feldnamen (`location` als mitgebrachter Ort, `name` Pflicht, Rest optional) plus `importKey`, ohne Kennungen, kommt in Story 2.3 in die Spec, damit das Import-Schema (Epic 3) sie per `$ref` einbindet.
- **Lesen nur über Kern-Abfragen:** `ListActiveEvents`, `ListArchivedEvents`, `ListEventTypes` (`GetEvent` und `ListLocations` nur für den Admin). Sie liefern Kern-Objekte mit abgeleiteter Genauigkeit und `archived`; der Handler bildet nur auf generierte Typen ab und leitet nichts selbst ab. Sortierung: aktiv nach `effectiveStart` auf-, Archiv absteigend, Gleichstand nach der internen `id`, nie per DB.
- **Vorbei-Regel:** aktiv heißt `effectiveEnd > now`, archiviert `effectiveEnd <= now`. Alle Lese-Abfragen filtern nur über `effective_start`/`effective_end`. `archived_at` wertet keine Abfrage und kein API-Feld aus.
- **Filter-Normalisierung im Kern:** `from`/`to` → halboffenes `[lo, hi)`; reines Datum = ganzer Tag inklusive; Zeitpunkt braucht Offset; Zeitpunkt in `to` wird auf die Minute abgeschnitten und um eine Minute erhöht. Einziges Prädikat: `effectiveStart < hi AND effectiveEnd > lo`. Prüfung `lo >= hi` nach Normalisierung (`from=2026-12-24T18:00+01:00&to=2026-12-24` ist gültig). Umrechnung nur über `ToInstant`, Zeit nur aus `Clock`, nie `now()` in SQL.
- **Standardwerte:** `/v1/events`: ohne beide → heute; nur `from` fehlt → ab Beginn heute; nur `to` fehlt → offen. `/v1/archive/events`: fehlendes `from` offen, fehlendes `to` = `now`.
- **Bereinigung:** Job in `internal/adapter/cleanup`, beim Start und dann täglich, ruft Kern-Anwendungsfall `MarkArchived` (transaktionsgebundener Kern + Hülle über `TxRunner`). Setzt `archived_at` per **einer** Anweisung mit `effective_end <= $now AND archived_at IS NULL` (sicher gegen gleichzeitiges `SaveEvent`), idempotent, loggt die Anzahl. Fehler beenden das Programm nicht. `SaveEvent` leert `archived_at`, wenn das Event wieder aktiv wird. Startreihenfolge: Migrationen → `RecomputeDerived` → `MarkArchived` → HTTP-Server. Neue Spalte `events.archived_at` per neuer goose-Migration (nur vorwärts, expand/contract, `pg_dump` vor Deploy).
- **Statische Pfade außerhalb der Spec, offenes CORS:** `/v1/openapi.yaml`, `/v1/import-v1.schema.json`, `/v1/docs`. `/v1/docs` rendert mit Redoc 2.5.4 (`redoc.standalone.js`) aus `adapter/publicapi/v1/static`, kein CDN, einzige HTML-Seite unter `/v1/…`, verlinkt die Spec.
- **v2-Vorsorge:** neue Hauptversion bekäme eigene Spec `api/v2/…` und eigenes Paket; Abschaltdatum in `info` der alten Spec und `Sunset`-Header.
- **Fehler:** Kern liefert `ErrValidation`, `ErrNotFound` …; nur der Adapter übersetzt in Status und Problem Details.
- **Abhängigkeiten:** `publicapi/v1` und `cleanup` importieren nur `core`, nie andere Adapter; Verdrahtung nur in `cmd/eventstore`.
- **Tests:** Test-first. Kern-Tests mit fester `Clock` für FR-8/FR-9-Beispiele und den Genau-eine-Liste-Vergleich; Postgres-Test für das Filterprädikat gegen PostgreSQL 18 (`-p 1`). 100 % Abdeckung für `internal/core` und `internal/adapter/publicapi/v1`; das Coverage-Gate `scripts/check-coverage.sh` muss ab Story 2.1 generierten Code (Header `// Code generated ... DO NOT EDIT.`) ausnehmen.

## Cross-Story Dependencies

- **Aus Epic 1:** Kern-Enum-Konstanten, Zeitmodell (`ToInstant`, `Clock`), `effective_start`/`effective_end`, Orte mit Adressteilen `street`/`postalCode`/`city`, Ablaufplan, `TxRunner`, `SaveEvent` und `RecomputeDerived` existieren und werden wiederverwendet.
- **Kennungen:** Erledigt mit Sprint Change Proposal 2026-10-04: keine Kennungen nach außen, nur Listen.
- 2.1 legt Spec, Codegen, CI-Prüfungen, CORS/405/404-Verhalten und `/v1/openapi.yaml` an; alle weiteren Stories erweitern dieselbe Spec.
- 2.3 liefert `Event` und `EventInput`; 2.4 nutzt die Leseform. 2.4 nutzt Filter-Normalisierung und Fehlerantworten aus 2.3 mit eigenen Standardwerten.
- 2.5 braucht die Endpunkte aus 2.1, 2.3 und 2.4 für den Vorher-nachher-Vergleich und erweitert `SaveEvent` um das Leeren von `archived_at`.
- 2.6 setzt die vollständige Spec voraus; die SM-1-Prüfung in Produktion folgt in Story 3.5.
- **Zu Epic 3:** `api/v1/import-v1.schema.json` bindet Enums, `EventInput` und das Adress-Objekt per `$ref` aus `openapi.yaml` ein (Story 3.1); die statische Auslieferung unter `/v1/import-v1.schema.json` gehört zu den statischen Pfaden dieses Epics.
