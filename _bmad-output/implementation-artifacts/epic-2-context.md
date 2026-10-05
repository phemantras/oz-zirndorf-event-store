# Epic 2 Context: Die Karten-App liest Events über die öffentliche API

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

Epic 2 öffnet den in Epic 1 gepflegten Bestand für Abnehmer, zuerst die Karten-App: eine öffentliche, nur lesende, versionierte REST-API unter `/v1/…` ohne Anmeldung. Sie liefert Events ausschließlich als gefilterte Listen („heute“, Zeiträume und Event-Typen, Archiv) und die Liste der Event-Typen, jeweils mit ehrlicher Zeit- und Ortsgenauigkeit und Quelle. Der Vertrag ist spec-first in OpenAPI beschrieben und als Redoc-Seite lesbar, damit andere OZ-Backends Versionierung, Fehlerformat, Zeitformat und Filterkonventionen als Vorlage übernehmen können. Eine tägliche Bereinigung markiert vergangene Events im Hintergrund, ohne dass sich API-Antworten ändern; der Programmstart zieht alle abgeleiteten Werte nach, damit Regeländerungen überall greifen. Events, deren Neuberechnung gescheitert ist („prüfen“), erscheinen öffentlich nicht, bis sie korrigiert sind: Fehlen ist besser als ein falscher Zeitraum. Keine Kennungen nach außen: kein Einzelabruf, keine Ortsliste, keine internen IDs in API und Import-Schema; ein Ort ist an seinem eindeutigen Namen erkennbar.

## Stories

- Story 2.1: API-Vertrag v1 und Liste der Event-Typen
- Story 2.2: entfällt (gestrichen, keine Kennungen nach außen)
- Story 2.3: Events von heute und nach Zeitraum und Typ abfragen
- Story 2.4: Archiv abfragen
- Story 2.5: Tägliche Bereinigung und vollständige Neuberechnung beim Start
- Story 2.6: Lesbare API-Dokumentation und Abnahme
- Story 2.7: Events mit „prüfen“ öffentlich ausblenden

## Requirements & Constraints

- **Ressourcen:** `GET /v1/events` (`from`, `to`, `type` wiederholbar), `GET /v1/event-types`, `GET /v1/archive/events`. Kein Schreibzugriff, kein Einzelabruf, keine Ortsliste, keine Volltext- oder Umkreissuche, keine Paginierung, kein Rate Limiting oder Caching.
- **Aktiv/Archiv:** aktiv heißt `effectiveEnd > now`, archiviert `effectiveEnd <= now`. Jedes Event liegt zu jedem Zeitpunkt in genau einer der beiden Abfragen, unabhängig davon, ob die Bereinigung gelaufen ist. **Einzige Ausnahme:** Events mit „prüfen“ fehlen in beiden, bis sie erfolgreich gespeichert sind (dann wieder in genau einer) oder gelöscht werden (dann in keiner). Archivierte Events haben dieselbe Struktur mit `archived: true` und werden nie automatisch gelöscht.
- **„Heute“ ohne Filter:** alle aktiven Events, deren Zeitraum den heutigen Tag (Europe/Berlin) berührt; mehrtägige an jedem Tag.
- **Filter:** Überschneidung mit dem Filterzeitraum; mehrere Typen ODER-verknüpft. Ungültiges Datum, Zeitpunkt ohne Offset, unbekannter oder leerer Typ oder normalisiertes `lo >= hi` → 400, `detail` nennt den Parameter. Nur `to` vor heute in `/v1/events` → 400 mit Verweis auf `/v1/archive/events`.
- **Antwortinhalt:** jedes Event vollständig mit vollständigem Ort; gleicher Ort → gleicher Name und gleiche Koordinaten. Zeitgenauigkeit, Ortsgenauigkeit und Quelle immer enthalten. Keine internen Werte (Kennungen, `importKey`, `archivedAt`, `title_key`, `name_key`).
- **Konventionen:** Hauptversion im Pfad, alte Hauptversion mindestens 6 Monate parallel; inkompatible Änderungen nur in neuer Hauptversion, neue optionale Felder in v1 erlaubt. Das Schema `Event` bekommt für „prüfen“ kein neues Feld. Fehler nach RFC 9457 mit englischem `title`/`detail`. Datum `YYYY-MM-DD`, Uhrzeit `HH:MM` oder `null`, berechnete Zeitpunkte ISO 8601 mit Offset. Technisches englisch, camelCase, Listen in `{ "data": [ … ] }`; aktive Listen aufsteigend, Archiv absteigend.
- **CORS:** `/v1/…` von jeder Herkunft, Preflight beantwortet; andere Methoden als `GET`/`HEAD`/`OPTIONS` → 405, unbekannte Pfade → 404, beides als `application/problem+json`.
- **Bereinigung:** beim Start und danach täglich; idempotent; loggt die Anzahl; API-Antworten vor und nach dem Job identisch. Fehler beenden das Programm nicht, der nächste Lauf versucht es erneut.
- **Doku:** alle Endpunkte, Parameter und Felder englisch beschrieben, mit Beispielen inkl. Fehlerbeispielen. Ausdrücklich erklärt: Zeit- und Ortsgenauigkeit, Vorbei-Regel, `effective*`, `archived`, Standardwerte von `from`/`to` je Endpunkt, `allDay`-Regel. Abschnitt „Time model“ und Beschreibung von `/v1/archive/events` nennen die Ausnahme für nicht neu berechenbare Events, auch bei der Aussage „every event is in exactly one of …“.
- **Abnahme:** Fixture mit Weihnachtsmarkt 2026 und fester `Clock`: `GET /v1/events?from=2026-11-29&to=2026-12-24` enthält ihn mit Zeitraum, Ort, Genauigkeiten und Quelle. Prüfung durch ein OZ-Mitglied ist ein manueller Schritt, keine Abschlussbedingung; die Prüfung in Produktion folgt in Epic 3.

## Technical Decisions

- **Spec-first:** `api/v1/openapi.yaml` ist einzige Quelle für Pfade, Parameter, Feldnamen, Enum-Codes und Fehlerformat. Gerüst per `oapi-codegen` v2.8.0 (Laufzeit `oapi-codegen/runtime` v1.7.0) nach `internal/adapter/publicapi/v1`, eingecheckt, nie von Hand geändert; CI prüft Aktualität und den Enum-Abgleich mit den Kern-Konstanten.
- **Lesen nur über Kern-Abfragen** (`ListActiveEvents`, `ListArchivedEvents`, `ListEventTypes`). Der Handler bildet nur auf generierte Typen ab, leitet nichts selbst ab und filtert nichts. Sortierung im Kern, Gleichstand nach interner `id`.
- **Filter-Normalisierung im Kern:** `from`/`to` → halboffenes `[lo, hi)`; reines Datum = ganzer Tag; Zeitpunkt in `to` auf die Minute abgeschnitten plus eine Minute. Einziges Prädikat: `effectiveStart < hi AND effectiveEnd > lo`. Standardwerte: `/v1/events` ohne beide → heute, ohne `from` → ab heute, ohne `to` → offen; Archiv ohne `from` → offen, ohne `to` → `now`.
- **Zeit:** Umrechnung nur über `ToInstant`, Zeit nur aus `Clock`, nie `now()` in SQL. Alle Lese-Abfragen filtern nur über `effective_start`/`effective_end`; `archived_at` wertet keine Abfrage und kein API-Feld aus.
- **Bereinigung:** Job in `internal/adapter/cleanup` ruft Kern-Anwendungsfall `MarkArchived` (Kern + Hülle über `TxRunner`). Setzt `archived_at` per **einer** Anweisung mit `effective_end <= $now AND archived_at IS NULL`. `archived_at` bedeutet „zuletzt als archiviert markiert“, nicht „archiviert seit“: jedes Speichern und jede Neuberechnung, die abgeleitete Werte ändert, leert es; der nächste `MarkArchived`-Lauf setzt es neu.
- **Neuberechnung beim Start:** Startreihenfolge Migrationen → `RecomputeDerived` → `MarkArchived` → HTTP-Server. `RecomputeDerived` zieht auch `name_key` der Orte nach (`NormalizeKey(name)`). Fehlgeschlagen ist ein Event, wenn die Regeln seine Zeitangaben ablehnen oder sein Ablaufplan nicht mehr in den neuen Zeitraum passt; zwei Orte mit gleichem neuen `name_key` gelten als Kollision (vorab im Kern erkannt, ein `ErrConflict` der DB wird genauso behandelt). Betroffene behalten ihre gespeicherten Werte, der Fehler wird mit Kennung(en) geloggt, das Programm startet trotzdem. Der Kern hält die IDs nur im Speicher; Admin-Listen markieren sie mit „prüfen“, bis erfolgreiches Speichern oder Löschen sie entfernt. Weil die Menge vor dem HTTP-Server aufgebaut wird, wird nie ein markiertes Event ausgeliefert. `RecomputeDerived` schreibt ohne `TxRunner` (einzige Ausnahme); ab Story 3.3 läuft sie in einer Transaktion unter der Advisory-Sperre.
- **Ausblenden „prüfen“ (2.7):** Die Auswahl trifft der Kern in `ListActiveEvents`/`ListArchivedEvents` über dieselbe Menge, die die Admin-Liste liest. Keine neue Spalte, keine SQL-Änderung, kein neuer Port, kein Filter im Handler.
- **Statische Pfade außerhalb der Spec, offenes CORS:** `/v1/openapi.yaml`, `/v1/import-v1.schema.json`, `/v1/docs`. `/v1/docs` rendert mit Redoc 2.5.4 aus `adapter/publicapi/v1/static`, kein CDN, einzige HTML-Seite unter `/v1/…`, verlinkt die Spec.
- **v2-Vorsorge:** eigene Spec `api/v2/…` und eigenes Paket; Abschaltdatum in `info` der alten Spec und als `Sunset`-Header.
- **Abhängigkeiten:** `publicapi/v1` und `cleanup` importieren nur `core`; Verdrahtung nur in `cmd/eventstore`. Nur Adapter übersetzen Kern-Fehler in HTTP-Status.
- **Tests:** test-first, feste `Clock`, Kern-Tests mit Fake-Repos; 100 % Abdeckung für `internal/core`, `internal/adapter/publicapi/v1` und `internal/adapter/admin` (generierter Code ausgenommen).

## Cross-Story Dependencies

- **Aus Epic 1:** Zeitmodell (`ToInstant`, `Clock`), `effective_start`/`effective_end`, Orte mit `name_key` und Adressteilen, Ablaufplan, `TxRunner`, `SaveEvent`, `SaveLocation`, `DeleteLocation` und `RecomputeDerived` werden wiederverwendet.
- 2.1 bis 2.6 sind umgesetzt; 2.7 ist offen und muss vor Epic 3 fertig sein.
- 2.7 setzt auf die Menge „prüfen“ und ihr Leeren nach dem Commit aus 2.5 (Teil B) auf und berührt die gemeinsame Event-Abfrage aus 2.3/2.4, den Partitionstest im Kern und den API-Vergleich in `cmd/eventstore` (2.4). Das AC aus 2.5 „Antworten vor und nach dem Job identisch“ gilt unverändert; die Spec-Texte aus 2.6 werden um die Ausnahme ergänzt.
- **Zu Epic 3:** `api/v1/import-v1.schema.json` bindet Enums, `EventInput` und das Adress-Objekt per `$ref` aus `openapi.yaml` ein; Längengrenzen und der CI-Abgleich der Kern-Feldnamen (`EventField*`, `FilterField*`) kommen mit Story 3.1. Story 3.3 führt die Advisory-Sperre ein und legt `RecomputeDerived` darunter.
