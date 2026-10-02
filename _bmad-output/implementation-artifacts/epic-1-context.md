# Epic 1 Context: Andreas pflegt Orte und Events im Admin

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

Epic 1 baut das Fundament des Event Store: ein geprüftes Go-Grundgerüst mit PostgreSQL 18, CI und automatischem Deploy auf Railway, dazu eine geschützte, deutschsprachige Admin-Oberfläche, in der Andreas (einziger Admin) Orte mit Kartenpicker und Events mit ehrlichen Angaben pflegt: Zeitgenauigkeit statt Platzhalter-Uhrzeiten, Quelle, Event-Typ und Ablaufplan. Bei Duplikatverdacht wird er gewarnt. Am Ende läuft das System produktiv, und der Bestand ist echt. Darauf bauen die öffentliche Lese-API (Epic 2) und der JSON-Import (Epic 3) auf, die dieselben Kern-Regeln und Anwendungsfälle nutzen.

## Stories

- Story 1.1: Lokales Grundgerüst mit CI
- Story 1.2: Auslieferung auf Railway
- Story 1.3: Admin-Anmeldung
- Story 1.4: Orte anlegen, bearbeiten und auflisten
- Story 1.12: Adresse in Straße, PLZ und Ort aufteilen (läuft vor 1.5)
- Story 1.5: Koordinaten auf der Karte setzen
- Story 1.6: Ehrliches Zeitmodell im Kern
- Story 1.7: Events anlegen, bearbeiten und auflisten
- Story 1.8: Neuen Ort direkt beim Anlegen eines Events
- Story 1.9: Ablaufplan pflegen
- Story 1.10: Warnung bei Duplikatverdacht
- Story 1.11: Events und Orte löschen

## Requirements & Constraints

- **Event:** Pflicht sind Titel, Event-Typ, Ort, Beginn (mindestens Datum) und Quelle; optional Ende, Notiz, Ablaufplan. Fehlt Pflichtes oder besteht nur aus Leerraum, wird abgelehnt. Ein Ende vor (oder gleich) dem Beginn wird abgelehnt.
- **Event-Typen:** feste Liste `festival`, `market`, `culture`, `politics`, `club`, `sports`, `other` (deutsche Beschriftungen im Admin). Unbekannter Typ wird abgelehnt.
- **Quelle:** Objekt aus Pflicht-Beschreibung und optionalem Link (nur http(s)). Nie im Titel.
- **Ort:** Name, Adresse, Koordinaten (Breite −90..90, Länge −180..180) und Ortsgenauigkeit `building`/`street`/`area`/`district` sind Pflicht, Notiz optional. Die Adresse besteht aus Straße (mit Hausnummer), PLZ (genau fünf Ziffern) und Ort, alle drei Pflicht, auch bei `district`/`area`. Spalte `address` nur bis zum Contract-Schritt in Story 1.7. Stabile Kennung, die beim Umbenennen gleich bleibt. Namen sind eindeutig nach Normalisierung. Events referenzieren den Ort, keine Kopie. Ein Ort mit Events (auch archivierten) ist nicht löschbar.
- **Ablaufplan:** beliebig viele Punkte mit Beschreibung, Datum, optionaler Beginn-/End-Uhrzeit; chronologisch sortiert ausgeliefert.
- **Keine personenbezogenen Daten** in Events und Orten; Formulare weisen darauf hin.
- **Zeitzone** immer Europe/Berlin. Leere Uhrzeit heißt „unbekannt“, nie 00:00.
- **Admin:** genau ein Konto, keine Registrierung; ohne gültige Session schlägt jeder schreibende Zugriff fehl. Formulare zeigen deutsche Meldungen je Feld und verlieren keine Eingaben.
- **Betrieb:** Hobby-Niveau, Best Effort, genau eine Replika.

## Technical Decisions

**Stack (verbindlich, nicht aus lokaler Umgebung ableiten):**

| Name | Version |
| --- | --- |
| Go | 1.27.1 |
| PostgreSQL | 18 (ausdrücklich gepinnt, Railway `:latest` = 16) |
| pgx | v5.11.0 |
| sqlc | 1.31.1 |
| goose | v3.28.0 |
| oapi-codegen | v2.8.0 |
| golang.org/x/crypto (bcrypt) | v0.57.0 |
| golang.org/x/text (`unicode/norm`) | v0.42.0 |
| htmx | 2.0.11 |
| Leaflet + OSM-Kacheln | 1.9.4 |
| Redoc | 2.5.4 |
| HTTP, Templates, Logging, CSRF | Standardbibliothek (`net/http`, `html/template`, `log/slog`, `http.CrossOriginProtection`) |

golangci-lint ist im Spine nicht versioniert; Version beim Aufsetzen der CI festlegen.

- **Struktur:** `api/v1/`, `cmd/eventstore/`, `internal/core/`, `internal/adapter/{publicapi/v1,admin,postgres,cleanup}/`, `Dockerfile`, `railway.json`, `compose.yaml`, `.github/workflows/ci.yaml`.
- **Abhängigkeitsrichtung:** `core` importiert nur Standardbibliothek plus `golang.org/x/text/unicode/norm`. Adapter importieren nur `core`, nie einander; nur `cmd/eventstore` verdrahtet alles. Ein Architekturtest in CI erzwingt das. Ports im Kern: `EventRepo`, `LocationRepo`, `TxRunner`, `Clock`.
- **Fachlogik nur im Kern:** SQL nur CRUD und einfache Vergleiche; keine Views, Trigger, Funktionen, `lower()`/`trim()`, kein `now()`. Zeit kommt aus `Clock`. Abgeleitete Werte (`effective_start/end`, `name_key`, `title_key`) berechnet der Kern und speichert sie mit.
- **Schreiben nur über Kern-Anwendungsfälle** (`SaveEvent`, `DeleteEvent`, `SaveLocation`, `DeleteLocation`, `RecomputeDerived`, später `CommitImport`, `MarkArchived`). Jeder hat einen transaktionsgebundenen Kern und eine Hülle über `TxRunner`. Eingabetyp `core.EventInput` für Admin und Import; `Canonicalize` (getrimmt, `""` als `null`, Ablaufplan sortiert) vor dem Speichern. Admin-`SaveEvent` ändert `importKey` nie. Repository-Ports bieten keine Schreibmethode am Kern vorbei.
- **Zeitmodell:** gespeichert `startDate` (Pflicht), `startTime`, `endDate`, `endTime`, `allDay`. `allDay` gilt für Beginn und Ende gemeinsam; mit Uhrzeit kombiniert abgelehnt; `endTime` ohne `endDate` abgelehnt. Genauigkeit (`exact`/`dateOnly`/`allDay`, beim Ende auch leer) wird abgeleitet, nie gespeichert.
- **Effektiver Zeitraum:** halboffen `[effectiveStart, effectiveEnd)`. Beginn ohne Uhrzeit → 00:00 des Tages; Ende ohne Uhrzeit → 00:00 des Folgetages des End-Tages; kein Ende → 00:00 des Folgetages des Beginns. Abgelehnt, wenn `effectiveEnd <= effectiveStart`. Vorbei/archiviert heißt `effectiveEnd <= now`; der Archivstatus wird nur so berechnet.
- **`ToInstant`** ist die einzige Umrechnung lokal → Zeitpunkt. Frühjahrslücke wird mit `ErrValidation` abgelehnt, doppelte Herbststunde bekommt den früheren Offset. `time/tzdata` ist eingebettet.
- **Start-Reihenfolge:** Migrationen → `RecomputeDerived` → `MarkArchived` → HTTP-Server. Scheitert die Neuberechnung für ein Event, behält es seine Werte, Fehler wird mit ID geloggt, Start läuft weiter; betroffene IDs hält der Kern im Speicher (Mutex) und der Admin markiert sie mit „prüfen“, bis ein Speichern/Löschen erfolgreich committet.
- **Identität und Schlüssel:** UUIDv7 per `DEFAULT uuidv7()` (nur im `DEFAULT`, nie in sqlc-Queries). Texte an einer Stelle auf NFC normalisiert. `NormalizeKey` (trim, jeden Leerraum inkl. geschützter zu einem Leerzeichen, lowercase) liefert eindeutigen `name_key` und `title_key`.
- **Duplikate:** `FindDuplicateCandidates` vergleicht `title_key`, `startDate` (nur Datum) und `locationId` über alle Events inkl. archivierter, beim Bearbeiten ohne das Event selbst. `SaveEvent` mit Policy `rejectDuplicates` (Admin zuerst, liefert `ErrDuplicateSuspect` mit Kandidaten) oder `allowDuplicates` (nach Bestätigung).
- **Ortssortierung im Kern:** case-insensitiv, ä→a, ö→o, ü→u, ß→ss, Gleichstand nach `id`; keine DB-Sortierung.
- **Datenbank:** snake_case, Tabellen `locations`, `events` (`location_id` mit `ON DELETE RESTRICT`, `effective_start`/`effective_end` timestamptz, `title_key`, `import_key`, `archived_at`), `timetable_entries` (`event_id` mit Löschweitergabe). Löschschutz für Orte im Kern und zusätzlich per Fremdschlüssel.
- **Ablaufplan:** Wertobjekt, nur über `SaveEvent` als ganze Liste ersetzt, in einer Transaktion mit dem Event. `endTime` vor `startTime` endet am Folgetag. Punkte müssen in `[effectiveStart, effectiveEnd]` liegen (darf genau mit dem Event enden); ändert `effective*` nicht.
- **Migrationen:** goose, SQL per `embed`, laufen beim Start vor dem HTTP-Server; nur vorwärts, expand/contract; angewendete nie ändern. Vor Deploy mit neuer Migration `pg_dump` über Railway-CLI.
- **Admin-Sicherheit:** nur unter `/admin/…`, kein CORS, `http.CrossOriginProtection`. Session als HMAC-SHA256-signiertes Cookie (HttpOnly, Secure, SameSite=Strict), ohne Serverzustand; trägt Anmelde- und Aktivitätszeitpunkt, 8 h Inaktivität bzw. max. 7 Tage. `SESSION_SECRET` ≥ 32 Byte (beim Start geprüft), bcrypt-Kosten ≥ 12. Sperre nach 5 Fehlversuchen je Client-IP für 15 Minuten (im Speicher). Client-IP aus `X-Forwarded-For` nach dem in Story 1.2 ermittelten Railway-Verhalten, sonst `RemoteAddr` ohne Port; `X-Real-IP` nicht nutzen.
- **Fehler:** Kern liefert `ErrValidation`, `ErrNotFound`, `ErrConflict`, `ErrDuplicateSuspect`; nur Adapter übersetzen sie (Admin: deutsche Meldungen, nie Fehlerseite, auch bei DB-Unique-Verletzung).
- **Konfiguration** nur über Umgebungsvariablen: `DATABASE_URL`, `PORT`, `ADMIN_USER`, `ADMIN_PASSWORD_HASH` (wegen `$` quoten), `SESSION_SECRET`. Fehlt eine, bricht der Start mit klarer Log-Meldung ab.
- **Logging:** `log/slog` als JSON auf stdout, nie Passwörter oder Session-Daten.
- **Deployment:** Multi-Stage-Dockerfile; `railway.json` mit `healthcheckPath: /healthz`; „Wait for CI“ aktiv; PostgreSQL 18 nur im privaten Netz. `GET /healthz` → 200 bei erreichbarer DB, sonst 503. Lokal PostgreSQL 18 per Docker Compose (Volume `/var/lib/postgresql`).
- **Tests:** Test-first. Kernregeln ohne DB mit fester `Clock`; Postgres-Adapter gegen echtes PostgreSQL 18. 100 % Abdeckung für `internal/core` und `internal/adapter/admin` (generierter Code ausgenommen). CI: `go vet`, Unit-Tests, Postgres-Tests, Architekturtest, generierter Code aktuell.
- **Sprache:** Code, Bezeichner, Logs, Fehlermeldungen englisch; Admin-Oberfläche und Inhalte deutsch.

## UX & Interaction Patterns

- Kein UX-Dokument; schlichtes Grundlayout, Gestaltung auf Story-Ebene. `html/template` + htmx 2.0.11, Leaflet 1.9.4, beides als Dateien unter `adapter/admin/static` (kein CDN).
- Nicht angemeldet → Umleitung zu `/admin/login` (außer Login und `/admin/static/`). Bei htmx-Anfragen (`HX-Request`) stattdessen `HX-Redirect: /admin/login`.
- Fehler je Feld auf Deutsch, Eingaben bleiben erhalten. Hinweis an leeren Uhrzeitfeldern: leer = „unbekannt“. Hinweise gegen Personendaten unter Name/Titel/Notiz/Quelle.
- Koordinaten: Dezimalkomma wird akzeptiert, leeres Feld gilt als fehlend (nie 0). Kartenpicker zentriert auf Zirndorf, OSM-Attribution sichtbar; Klick setzt Marker, Marker ziehen aktualisiert Felder, gültige Eingabe verschiebt Marker, ungültige markiert das Feld. Ohne JavaScript bleibt Speichern über Zahlenfelder möglich.
- Event-Formular: Ort aus Auswahl oder „Neuer Ort“ inline per htmx, ohne bisherige Eingaben zu verlieren. Programmpunkte ohne Neuladen hinzufügen/entfernen; Fehler am betroffenen Punkt.
- Event-Liste zeigt alle Events mit berechnetem Status „aktiv“/„archiviert“ und ggf. „prüfen“.
- Duplikatverdacht: Warnung mit Links auf Kandidaten, „Trotzdem speichern“ oder Abbrechen (Eingaben bleiben). Löschen mit Rückfrage; bereits gelöscht → „nicht mehr vorhanden“; Ort mit Events → Meldung mit Anzahl betroffener Events.

## Cross-Story Dependencies

- 1.1 ist Basis für alles; 1.2 liefert das Railway-Verhalten von `X-Forwarded-For` (README), das 1.3 für die Client-IP braucht.
- 1.4 (Orte, `NormalizeKey`, NFC) und 1.6 (Zeitmodell, `ToInstant`, `Clock`) sind Voraussetzung für 1.7. 1.5 erweitert das Ortsformular aus 1.4; 1.8 nutzt Ortsformular und Kartenpicker aus 1.4/1.5 sowie `SaveLocation`.
- 1.9 führt den Port `TxRunner` ein und erweitert `SaveEvent`; 1.10 ergänzt `title_key` und die Duplikat-Policy in `SaveEvent` und `RecomputeDerived`; 1.11 braucht Events mit Ablaufplan und den Fremdschlüssel aus 1.7.
- Zu Epic 2: Die Enum-Konstanten im Kern entstehen hier; `api/v1/openapi.yaml` als einzige Quelle für Codes und `EventInput` sowie der CI-Enum-Abgleich folgen in Story 2.1 und müssen dann mit dem Kern übereinstimmen. `MarkArchived` und der Bereinigungsjob kommen in Epic 2.
- Zu Epic 3: `core.EventInput`, `SaveEvent` (inkl. `allowDuplicates`, `importKey` unverändert) und `FindDuplicateCandidates` werden von `CommitImport` wiederverwendet.
