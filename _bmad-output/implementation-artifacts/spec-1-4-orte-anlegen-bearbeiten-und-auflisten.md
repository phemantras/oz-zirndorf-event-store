---
title: 'Story 1.4: Orte anlegen, bearbeiten und auflisten'
type: 'feature'
created: '2026-10-02'
status: 'done'
baseline_commit: 'becd6ed9e2d172f86c837237fb6ac30c53f387c6'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-1-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Es gibt noch keine Orte. Events (ab Story 1.7) brauchen einen Ort mit stabiler Kennung, eindeutigem Namen und Koordinaten (FR-6, NFR-4, AD-11, ENT-8, ENT-9). Die Akzeptanzkriterien der Story 1.4 in `epics.md` gelten vollständig.

**Approach:** Kern bekommt Ort, Ortsgenauigkeit, `NormalizeKey`, NFC-Normalisierung, Validierung, Sortierung, typisierte Fehler, Port `LocationRepo` und die Anwendungsfälle `SaveLocation`, `GetLocation`, `ListLocations`. Postgres-Adapter: Migration `locations`, sqlc-Abfragen (sqlc wird hier eingeführt), Repository. Admin: Ortsliste, Formular für Neu und Bearbeiten mit deutschen Feldmeldungen. `cmd/eventstore` verdrahtet.

## Boundaries & Constraints

**Always:** Texte im Kern an einer Stelle NFC-normalisiert und getrimmt; Pflichtfeld aus Leerraum fehlt. `name_key = NormalizeKey(name)`: trimmen, jeden Leerraum (`unicode.IsSpace`, also auch U+00A0, U+202F) zu einem Leerzeichen, Kleinbuchstaben. Sortierung im Kern: Kleinbuchstaben, ä→a, ö→o, ü→u, ß→ss, Gleichstand nach `id`. Konflikt (Kern-Vorabprüfung oder Unique-Verletzung 23505 aus der DB) → `ErrConflict` mit dem vorhandenen Ort; Admin zeigt „Es gibt bereits einen Ort mit diesem Namen: <Name>“ mit Link. Koordinaten kommen als Text in den Kern: leer = fehlt, keine endliche Zahl = ungültig, Bereich −90..90 / −180..180 inklusive. Der Admin ersetzt nur `,` durch `.`. Eingaben bleiben bei jedem Fehler im Formular. Test-first, 100 % Abdeckung für `internal/core` und `internal/adapter/admin`.

**Never:** Keine Löschfunktion (Story 1.11), kein Kartenpicker (1.5), kein `TxRunner` (1.9). Kein SQL mit `lower()`, `trim()`, `ORDER BY` für die Anzeige, `CHECK` mit Fachregeln oder `now()`. `uuidv7()` nur im `DEFAULT`. Admin bekommt nie das Repository, nur die Kern-Anwendungsfälle. Keine Felder für Personen. Generierten sqlc-Code nie von Hand ändern.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Anlegen ok | Alle Pflichtfelder, Breite `49,4424` | 303 → `/admin/locations`, Ort in der Liste | N/A |
| Pflicht fehlt | Name `"   "`, Länge leer | 422, Formular mit „Bitte einen Namen angeben.“ / „Bitte eine Länge angeben.“, Eingaben erhalten | `ErrValidation` mit Feldern |
| Ungültige Koordinate | Breite `91`, Länge `abc`, `NaN`, `Inf` | 422, Feldmeldung Bereich bzw. „keine Zahl“ | `ErrValidation` |
| Unbekannte Genauigkeit | `precision=city` | 422, Feldmeldung | `ErrValidation` |
| Namenskonflikt | Vorhanden „Paul-Metz-Halle“, neu „ paul-metz-halle “ | 409, Meldung mit Link auf den vorhandenen Ort | `ErrConflict` |
| Umbenennen in Konflikt | anderer Ort → „PAUL-METZ-HALLE“ | 409, gleiche Meldung | `ErrConflict` |
| Eigenen Namen behalten | Ort speichern ohne Namensänderung | 303, kein Konflikt mit sich selbst | N/A |
| Bearbeiten | Name/Adresse geändert | gleiche `id` | N/A |
| Unbekannte ID | GET/POST `/admin/locations/{id}` mit fremder oder kaputter UUID | 404, deutsche Seite „Ort nicht gefunden.“ | `ErrNotFound` |
| Ohne Session | `/admin/locations` | 303 → `/admin/login` (bestehende Middleware) | N/A |

**Entscheidungen (2026-10-02):** Volle Spec trotz Überlänge (~2.800 Tokens) behalten. Die Backup-Probe (`railway ssh … pg_dump`, Restore-Befehl in der README) aus `deferred-work.md` ist nicht Teil dieser Story, sondern kommt separat; die Migration `locations` darf ohne geprüften Dump live gehen (Datenbank noch leer).

</frozen-after-approval>

## Code Map

- `internal/core/doc.go` -- Kern ist leer; neue Dateien hier. Erlaubte Fremd-Imports nur `golang.org/x/text/unicode/norm` (`internal/archtest`).
- `internal/adapter/admin/handler.go` -- `routes()`: neue Routen in den `protected`-Mux; `render` mit Status wiederverwenden; Konstanten-Stil (Pfade, Meldungen, Log-Texte) fortführen; Body per `http.MaxBytesReader` begrenzen.
- `internal/adapter/admin/templates.go` + `templates/` -- `parsePage` für neue Seiten; `home.html` bekommt Link „Orte“.
- `internal/adapter/admin/handler_test.go` -- `newTestServer`, `validCookie`, `withCookie` wiederverwenden; `Config` erhält ein Fake der Kern-Anwendungsfälle.
- `internal/adapter/postgres/postgres.go` + `migrations/00001_baseline.sql` -- Baseline nie ändern; neue Migration `00002_locations.sql`. `postgres_test.go`: `testDatabaseURL(t)` wiederverwenden (Skip ohne URL, Fehler mit `CI`).
- `cmd/eventstore/main.go` -- `run`/`newAdminHandler`: Repository aus `pool` bauen, Kern-Service in `admin.Config`.
- `scripts/check-coverage.sh` -- deckt `core` und `admin` schon ab; sqlc-Code liegt im nicht erfassten `postgres`.
- `.github/workflows/ci.yaml` -- neuer Schritt „generierter Code aktuell“ (sqlc 1.31.1, `git diff --exit-code`).
- `AGENTS.md` -- TODO-Zeile zu Codegen bleibt; Eintrag per `bmad-project-context`-Refresh (deferred).

## Tasks & Acceptance

**Execution:**
- [x] `go.mod` -- `golang.org/x/text v0.42.0` direkt -- NFC im Kern.
- [x] `internal/core/errors.go` (+ Test) -- `ErrValidation`, `ErrNotFound`, `ErrConflict`; `ValidationError` mit Feldfehlern (Feld + Problemcode `missing`/`notANumber`/`outOfRange`/`unknownCode`), `LocationConflictError` mit vorhandenem Ort; beide per `errors.Is` erkennbar.
- [x] `internal/core/text.go` (+ Test) -- `NormalizeKey`, NFC+Trim-Helfer, Sortierschlüssel -- Umlaute, doppelte und geschützte Leerzeichen.
- [x] `internal/core/location.go` (+ Test) -- `Location`, `LocationInput` (Koordinaten als Text), Genauigkeits-Konstanten, Feldnamen-Konstanten, Validierung, Sortierung.
- [x] `internal/core/location_service.go` (+ Test mit Fake-Repo) -- Port `LocationRepo` (`List`, `Get`, `FindByNameKey`, `Create`, `Update`), `LocationService` mit `SaveLocation(ctx, id, input)`, `GetLocation`, `ListLocations`; Konflikt bei fremdem Ort mit gleichem `name_key`; Repo-`ErrConflict` → vorhandenen Ort nachladen.
- [x] `internal/adapter/postgres/migrations/00002_locations.sql` -- `id uuid PRIMARY KEY DEFAULT uuidv7()`, `name`, `name_key UNIQUE`, `address`, `latitude`/`longitude double precision`, `precision`, `note` (nullable), alle anderen `NOT NULL`.
- [x] `sqlc.yaml`, `internal/adapter/postgres/queries/locations.sql`, generiertes Paket `internal/adapter/postgres/db` -- pgx/v5.
- [x] `internal/adapter/postgres/locations.go` (+ Test gegen PG 18) -- implementiert `core.LocationRepo`; 23505 → `core.ErrConflict`, keine Zeile / ungültige UUID → `core.ErrNotFound`.
- [x] `internal/adapter/admin/locations.go`, `templates/locations.html`, `templates/location_form.html`, `templates/not_found.html` (+ Tests) -- Routen GET `/admin/locations`, GET `/admin/locations/new`, POST `/admin/locations`, GET/POST `/admin/locations/{id}`; deutsche Beschriftungen Gebäude, Platz/Straße, Bereich, nur Ortsteil; NFR-4-Hinweis unter Name und Notiz; alle Matrix-Zeilen als Tests.
- [x] `cmd/eventstore/main.go` (+ Test) -- Verdrahtung; Postgres-`run`-Test prüft `/admin/locations` mit Session.
- [x] `.github/workflows/ci.yaml` -- sqlc-Aktualitätsprüfung.
- [x] `README.md` -- sqlc-Befehl, Status-Zeile.

**Acceptance Criteria:**
- Given drei Orte „Zirndorfer Ölmühle“, „alte Feuerwache“, „Bibertpark“, when die Liste geladen wird, then lautet die Reihenfolge alte Feuerwache, Bibertpark, Zirndorfer Ölmühle.
- Given zwei Orte, die parallel denselben `name_key` anlegen, when die DB-Unique-Verletzung eintritt, then zeigt das Formular dieselbe Konfliktmeldung, nie eine Fehlerseite.
- Given `sqlc generate` auf sauberem Stand, when CI läuft, then gibt es keinen Diff.

## Implementation Notes

- `SaveLocation` lädt bei gesetzter `id` zuerst den Ort und arbeitet mit der gespeicherten Schreibweise der ID weiter; sonst meldete eine großgeschriebene UUID in der URL einen Namenskonflikt mit sich selbst. Unbekannte ID → `ErrNotFound` vor der Validierung.
- Repo-`ErrConflict` (Unique-Verletzung) → Kern lädt den Gewinner per `FindByNameKey` nach und liefert `LocationConflictError`; Postgres verpackt 23505 per `errors.Join(core.ErrConflict, err)`, damit die Ursache im Log bleibt.
- Admin: 303 nach Speichern, 422 Feldfehler, 409 Konflikt, 404 „Ort nicht gefunden.“, 400 bei Formular > 16 KiB, 500 + Log bei sonstigen Fehlern. Liste zeigt Koordinaten mit Dezimalpunkt.
- Leere Notiz wird als `NULL` gespeichert. `ParseFloat`-Überlauf (`1e400`) gilt als `outOfRange`.
- CI-Schritt im Lint-Job: `go run …sqlc@v1.31.1 generate`, `git diff --exit-code` plus Prüfung auf neue, nicht eingecheckte Dateien.
- Lokal verifiziert (2026-10-02): `CI= go test ./...`, Coverage-Gate 340/340, `go vet`, golangci-lint v2.14.0 (0 Befunde), sqlc-Windows-Binary ohne Diff. Nicht lokal: Postgres-Repository-Tests und erweiterter `run`-Test (kein Docker), `-race` (kein cgo) — laufen in der CI.
- Abnahme (2026-10-02, nach Merge von PR #9): CI auf `main` grün inkl. Postgres-Tests und sqlc-Prüfung. Auf der Railway-Domain im Browser von Andreas bestätigt: Ort anlegen, umbenennen, Namenskonflikt auslösen.

## Spec Change Log

## Review Triage Log

| # | Quelle | Befund | Verdikt | Evidenz | Route |
|---|--------|--------|---------|---------|-------|
| 1 | gap | Sortier-Test hängt an Map-Reihenfolge der Fakes; fehlendes `SortLocations` bliebe oft grün | medium | `fakeLocationRepo.List`/`memoryLocationRepo.List` iterieren über Maps; Coverage fängt es nicht | patch |
| 2 | gap | Koordinaten in der Ortsliste ungetestet | low | Kein Test prüft die Spalte; vertauschte Werte blieben grün; Fix ist eine Assertion | patch |
| 3 | blind | Kommentare behaupten, Konstanten spiegeln `api/v1/openapi.yaml`, die es noch nicht gibt | low | `api/v1/` enthält nur `.gitkeep`; Spec entsteht in Story 2.1 | patch |
| 4 | blind | Bearbeiten-Formular übernimmt die rohe URL-ID ins `action` | low | `showLocation` nutzt `r.PathValue`; Kern kanonisiert die ID sonst; Fix ist eine Zeile | patch |
| 5 | edge + blind | Unique-Verletzung, dann Gewinner umbenannt → Anlegen zeigt 404 | low | Braucht zwei gleichzeitige Speichervorgänge plus Umbenennung dazwischen; ein Admin | reject |
| 6 | edge | NUL-Byte oder ungültiges UTF-8 → PostgreSQL-Fehler, 500 | low | Browserformulare senden gültiges UTF-8; nur manipulierte Anfragen; Fix braucht neuen Problemcode | reject |
| 7 | edge + blind | `ParseFloat` akzeptiert Hex-Floats (`0x1p4`) | low | Gibt im Admin niemand ein; Wert bleibt endlich und im Bereich geprüft; Fix braucht Zusatzprüfung | reject |
| 8 | edge | `-0` wird als `-0` gespeichert und angezeigt | low | Kosmetisch, nur bei absichtlicher Eingabe | reject |
| 9 | edge | `Config.Locations` nil → Panic | low | Einziger Aufrufer setzt es, Wiring-Test prüft `/admin/locations` | reject |
| 10 | edge + blind | Unbekannter Genauigkeitscode zeigt leere Zelle | low | Kern speichert nur gültige Codes; nur per manueller DB-Änderung | reject |
| 11 | edge | Eingaben gehen bei 400/404/500 verloren | low | Nur bei > 16 KiB, gelöschtem Ort (Löschen gibt es noch nicht) oder DB-Ausfall | reject |
| 12 | blind | Jede 23505 gilt als Namenskonflikt, Constraint-Name ungeprüft | low | Einziger weiterer Unique ist der UUIDv7-PK; neue Constraints kommen mit ihren Stories | reject |
| 13 | blind | Keine Längen-/Steuerzeichen-Grenzen für Texte | low | Nicht in Intent/Epic gefordert; 16-KiB-Grenze für den Body | reject |
| 14 | blind | Unsichtbare Zeichen (U+200B) umgehen Eindeutigkeit | low | `NormalizeKey` ist in AD-11 genau so festgelegt; Admin tippt so etwas nicht | reject |
| 15 | blind | 400/500 englischer Klartext | low | Wie 1.3 #16: nur bei Angriff oder DB-Ausfall | reject |
| 16 | blind | DB-Race nur mit Fakes getestet | low | 23505→`ErrConflict` gegen PG getestet, Nachladen im Kern getestet; Kombination folgt daraus | reject |
| 17 | blind | Story-Status in Spec und Sprint-Status uneinheitlich | false | Workflow setzt den Sprint-Status beim Abschluss | reject |
| 18 | blind | Spec enthält lokalen Scratchpad-Pfad | low | Fix ändert diese Spec | reject |
| 19 | blind | AGENTS.md-TODO zu Codegen bleibt offen | low | Agent-Kontextdatei, Refresh per `bmad-project-context` | defer |
| 20 | blind | Templates schreiben Routen literal | low | Bestehendes Muster (1.3 #12) | reject |
| 21 | blind | `not_found.html` hat feste Linkbeschriftung „Ortsliste“ | low | Heute nur für Orte genutzt; Anpassung mit Story 1.7 | reject |
| 22 | blind | Kein Cross-Origin-Test für neue POST-Routen | low | `CrossOriginProtection` umschließt den ganzen Mux, Login-Test belegt das | reject |

## Design Notes

Kern-Anwendungsfälle hängen an `LocationService{repo LocationRepo}`; Admin sieht nur ein schmales Interface (`SaveLocation`, `GetLocation`, `ListLocations`), so bleibt das Repo unerreichbar (AD-6). Ohne `TxRunner` ist die Vorabprüfung nicht atomar, der Unique-Index schließt die Lücke. `id` leer = anlegen. Formularwerte gehen 1:1 zurück ins Template; Kern liefert nur Codes, der Admin übersetzt.

## Verification

**Commands:**
- `CI= go test ./...` -- grün
- `bash scripts/check-coverage.sh` -- 100 %
- `go vet ./...` und golangci-lint v2.14.0 -- ohne Befund
- `sqlc generate` (1.31.1) -- kein Diff. Lokal unter Windows: Release-Binary von sqlc 1.31.1 (kein cgo, kein Docker); CI: `go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1` auf Ubuntu.

**Manual checks (if no CLI):**
- Postgres-Tests laufen lokal nicht (kein Docker), nur in der CI.
- Nach Deploy auf Railway: Ort anlegen, umbenennen, Konflikt auslösen.
