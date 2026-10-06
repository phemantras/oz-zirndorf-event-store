# Epic 2 Context: Die Karten-App liest Events über die öffentliche API

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

Abnehmer-Apps, zuerst die Karten-App, lesen ohne Anmeldung die Events von heute, nach Zeitraum und Event-Typ gefiltert, das Archiv und die Liste der Event-Typen. Das geschieht über einen versionierten, vollständig dokumentierten OpenAPI-Vertrag, der anderen OpenZirndorf-Backends als Vorlage dient. Eine tägliche Bereinigung markiert vergangene Events im Hintergrund, ohne dass sich API-Antworten ändern. Der Start berechnet alle abgeleiteten Werte neu. Die API liefert nur Listen und gibt keine Kennungen aus. Die Story 2.8 ist noch offen: Sie entfernt das Feld `archived` aus der Leseform, als einmalige Ausnahme von der Versionierungsregel, weil v1 noch keinen Abnehmer hat.

## Stories

- Story 2.1: API-Vertrag v1 und Liste der Event-Typen
- Story 2.2: entfällt (gestrichen, keine Einzelabrufe)
- Story 2.3: Events von heute und nach Zeitraum und Typ abfragen
- Story 2.4: Archiv abfragen
- Story 2.5: Tägliche Bereinigung und vollständige Neuberechnung beim Start
- Story 2.6: Lesbare API-Dokumentation und Abnahme
- Story 2.7: Events mit „prüfen“ öffentlich ausblenden
- Story 2.8: `archived` aus der Leseform entfernen

## Requirements & Constraints

- **Endpunkte:** `GET /v1/events` und `GET /v1/archive/events` (Parameter `from`, `to`, `type` wiederholbar, ODER-verknüpft, Duplikate zählen einmal) sowie `GET /v1/event-types` (sieben Codes mit deutscher Beschriftung). Es gibt keinen Einzelabruf, keine Ortsliste, keine Paginierung, keine Suche und keinen Schreibzugriff.
- **Vorbei-Regel:** Ein Event ist vorbei, sobald `effectiveEnd <= now`. Jedes Event steht zu jedem Zeitpunkt in genau einer der beiden Listen. Ausnahme: Events mit „prüfen“ stehen in keiner. Ein Event, das seit einer Minute vorbei ist, steht schon im Archiv, auch vor der Bereinigung.
- **Standardwerte für `from`/`to`:** Bei `/v1/events` gilt ohne beide Parameter nur heute. Fehlt nur `from`, beginnt der Zeitraum heute. Fehlt nur `to`, ist er offen. Beim Archiv ist ein fehlendes `from` offen, ein fehlendes `to` gilt als `now`.
- **Filter:** Ein Datum oder ein Zeitpunkt mit Offset, beide inklusive. Es zählt die Überschneidung. Ungültige Werte, ein Zeitpunkt ohne Offset, ein unbekannter oder leerer Typ oder ein Intervall mit `lo >= hi` nach der Normalisierung führen zu 400 als Problem Details. `detail` nennt den Parameter. Liegt nur `to` vor heute, verweist `detail` auf `/v1/archive/events`.
- **Sortierung:** Aktive Events nach `effectiveStart` aufsteigend, das Archiv absteigend, bei Gleichstand nach der internen `id`. Der Ablaufplan ist chronologisch sortiert.
- **Leseform `Event`:** `title`, `type`, `location` vollständig (`name`, `address{street,postalCode,city}`, `latitude`, `longitude`, `precision`, `note`), `startDate`, `startTime`, `endDate`, `endTime`, `allDay`, `startPrecision`, `endPrecision`, `source{description,url}`, `note`, `timetable[{description,date,startTime,endTime}]`, `effectiveStart`, `effectiveEnd`. Ab Story 2.8 ohne `archived`. Die Form enthält keine internen Werte (IDs, `importKey`, `archivedAt`, `title_key`, `name_key`). Ein Test belegt die Schlüsselmenge auf jeder Ebene.
- **Formate:** Datum `YYYY-MM-DD`, Uhrzeit `HH:MM` oder `null` (unbekannt, nie `00:00`), `effective*` nach ISO 8601 mit dem lokalen Offset von Europe/Berlin. Listen stehen in der Hülle `{ "data": [ … ] }`. Fehler folgen RFC 9457 als `application/problem+json`, auf Englisch.
- **HTTP:** Für `/v1/…` gilt offenes CORS (`*`, mit Preflight). Nur `GET`, `HEAD` und `OPTIONS` sind erlaubt, sonst kommt 405; unbekannte Pfade liefern 404, beides als Problem Details.
- **Versionierung:** Eine Hauptversion steht im Pfad. Inkompatible Änderungen gibt es nur in einer neuen Version, die alte bleibt mindestens 6 Monate parallel erreichbar. Neue optionale Felder sind erlaubt.
- **Bereinigung:** Läuft beim Start und danach täglich. Sie ist idempotent, loggt die Anzahl markierter Events und beendet bei einem Fehler das Programm nicht. Archivierte Events werden nie gelöscht.
- **Doku:** Alle Endpunkte, Parameter und Felder haben eine englische Beschreibung mit Beispielen. Zeit- und Ortsgenauigkeit, die Vorbei-Regel, `effective*`, die Standardwerte von `from`/`to` und die `allDay`-Regel sind erklärt, dazu gibt es Beispiele für Fehler.
- **Erfolgskriterien:** Jedes ausgelieferte Event trägt Zeitgenauigkeit, Ortsgenauigkeit und Quelle. Der Weihnachtsmarkt 2026 ist per Fixture über `from=2026-11-29&to=2026-12-24` abrufbar. Die Prüfung durch ein OZ-Mitglied ist ein manueller Schritt.

## Technical Decisions

- **Spec-first:** `api/v1/openapi.yaml` (OpenAPI 3.1) ist die einzige Quelle für Feldnamen, Enum-Codes und Längengrenzen. Das Gerüst erzeugt oapi-codegen v2.8.0 (`std-http-server` + `strict-server`) mit `go generate ./...` nach `internal/adapter/publicapi/v1`. Es wird eingecheckt und nie von Hand geändert. Die CI prüft den Diff und den Abgleich zwischen den Kern-Konstanten und der Spec.
- **Statische Auslieferung:** `/v1/openapi.yaml`, `/v1/import-v1.schema.json` und `/v1/docs` liegen außerhalb der Spec, mit offenem CORS. `/v1/docs` rendert Redoc 2.5.4 aus `adapter/publicapi/v1/static`, nicht per CDN.
- **Lesen über den Kern:** `ListActiveEvents`, `ListArchivedEvents` und `ListEventTypes`. Der Handler bildet nur Kern-Objekte auf die erzeugten Typen ab und leitet nichts selbst ab oder filtert. Typisierte Kernfehler übersetzt nur der Adapter.
- **Zeitmodell:** Der Kern normalisiert Filter zu `[lo, hi)`: Ein Datum zählt als ganzer Tag, ein Zeitpunkt in `to` wird auf die Minute abgeschnitten und um eine Minute erhöht. Das einzige Prädikat lautet `effectiveStart < hi AND effectiveEnd > lo`, und SQL filtert nur über `effective_start`/`effective_end`. `now` kommt aus `Clock`, nie aus SQL. Zeiten werden nur mit `ToInstant` umgerechnet, `time/tzdata` ist eingebettet.
- **Archivstatus:** Er ergibt sich nur aus der Vorbei-Regel. `archived_at` ist eine reine Statistik-Markierung, die keine Abfrage und kein API-Feld nutzt. `MarkArchived` setzt sie in einer einzigen Anweisung mit der Bedingung `effective_end <= $now AND archived_at IS NULL`. Jedes Speichern und jede ändernde Neuberechnung leert sie. Im Kern wählt `ListedEvent.Archived` weiter die Liste aus.
- **Startreihenfolge:** Migrationen, `RecomputeDerived` (alle `effective*`, `title_key`, `name_key`), `MarkArchived`, HTTP-Server. Scheitert die Neuberechnung bei einem Event (Zeitangaben ungültig, Ablaufplan außerhalb des Zeitraums) oder kollidiert ein `name_key`, bleiben die gespeicherten Werte. Der Fehler wird mit den Kennungen geloggt und das Programm startet trotzdem. Die betroffenen IDs liegen im Kern nur im Speicher (Mutex) als „prüfen“ und werden nach erfolgreichem `SaveEvent`, `CommitImport` oder `DeleteEvent` bzw. `SaveLocation`/`DeleteLocation` entfernt. Die öffentlichen Listen schließen diese Events im Kern aus; dafür gibt es keine Spalte und keine SQL-Änderung.
- **Bereinigungsjob:** Liegt in `adapter/cleanup` und schreibt nur über den Kern-Anwendungsfall `MarkArchived`.
- **Tests:** Kernregeln werden mit fester `Clock` und Fake-Repos getestet, ohne Datenbank (FR-8/9-Beispiele, Partition aktiv/Archiv, Neuberechnung). Das Prädikat und die Neuberechnung von `name_key` prüfen Postgres-Tests gegen PostgreSQL 18. Die API-Vergleiche liegen in `cmd/eventstore` (`/v1/events?from=1900-01-01` gegen `/v1/archive/events`). Die Abdeckung in `core` und `publicapi/v1` liegt bei 100 %.

## Cross-Story Dependencies

- Story 2.1 legt Spec, Fehlerformat, CORS und Codegen an; alle späteren Stories erweitern dieselbe Spec.
- Story 2.3 definiert `Event` und die Schreibform `EventInput`. Das Import-Schema aus Epic 3 bindet `EventInput` per `$ref` ein, deshalb muss sich eine Änderung an Feldnamen oder Grenzen dort mit auswirken.
- Story 2.4 nutzt Prädikat, Normalisierung und Fehlerantworten aus 2.3 und unterscheidet sich nur bei den Standardwerten und der Sortierung.
- Story 2.7 berührt `listMatchingEvents` (2.3, 2.4), den Partitionstest, den API-Vergleich und die Menge „prüfen“ aus 2.5.
- Story 2.8 startet nach Story 3.5 und berührt den Leseform-Test und die Abbildung in `publicapi/v1` (2.3), die Archivbeschreibung und den API-Vergleich (2.4), „Time model“ in der Spec (2.6, 2.7) und die Abnahme der Testsammlung in `cmd/eventstore` (3.5). Der Kern, `archived_at`, `MarkArchived` und die Admin-Anzeige „archiviert“ bleiben unverändert.
- Abhängigkeit zu Epic 1: Das Zeitmodell (`ToInstant`, `effective*`), die Ortsreferenz und die Event-Typen stammen aus Epic 1. Koordinatenänderungen im Admin wirken sofort auf alle Events des Orts.
