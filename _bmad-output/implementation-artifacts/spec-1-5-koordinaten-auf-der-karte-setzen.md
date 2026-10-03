---
title: 'Story 1.5: Koordinaten auf der Karte setzen'
type: 'feature'
created: '2026-10-03'
status: 'done'
baseline_commit: '68d43af02c6a998cb57c63bf312537ca9c0bfce3'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-1-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Koordinaten eines Orts muss der Admin heute von Hand nachschlagen und eintippen (FR-15). Die Akzeptanzkriterien der Story 1.5 in `epics.md` gelten vollständig.

**Approach:** Leaflet 1.9.4 wird unter `internal/adapter/admin/static/leaflet/` vendort und eingebettet. Das Ortsformular bekommt eine Karte mit OSM-Kacheln und ein eigenes Skript, das Karte, Marker und die Felder Breite/Länge in beide Richtungen koppelt. Server, Kern und Datenbank bleiben unverändert.

## Boundaries & Constraints

**Always:** Kacheln von `https://tile.openstreetmap.org/{z}/{x}/{y}.png`, Attribution „© OpenStreetMap-Mitwirkende“ mit Link auf `https://www.openstreetmap.org/copyright`, sichtbar. Ohne gültige Feldwerte: Mitte Zirndorf (49,4424 / 10,9539), Zoom 14, kein Marker. Mit gültigen Feldwerten beim Laden (Bearbeiten oder Formular nach Fehler): Marker dort, Zoom 17. Klick und Ziehen schreiben Breite/Länge mit Dezimalpunkt, auf 6 Nachkommastellen gerundet, Länge per `wrap()` in −180..180. Beim Verlassen eines Feldes (`change`): beide Felder gültig → Marker springt (fehlt er, wird er gesetzt); ein Feld ungültig → Marker bleibt, nur dieses Feld bekommt `aria-invalid="true"` (rot umrandet); gültig oder leer → Markierung weg. Gültig heißt wie im Admin: `,`→`.`, endliche Zahl, Breite −90..90, Länge −180..180 inklusive; ein leeres Feld markiert nichts und bewegt nichts. Die Kartenfläche ist im HTML `hidden` und wird erst vom Skript gezeigt; das Skript bricht still ab, wenn `L` fehlt. Leaflet-Dateien unverändert aus dem npm-Paket `leaflet@1.9.4` (`dist/`), samt `LICENSE`. Test-first, 100 % Abdeckung für `internal/adapter/admin`.

**Never:** Kein CDN. Keine Änderungen an Kern, Postgres-Adapter oder Formularfeldnamen. Keine Geokodierung (Adresse → Koordinaten). Kein Inline-Skript im Template. Kein Leaflet 2.0.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Neuer Ort | Formular leer | Karte auf Zirndorf, kein Marker, Attribution sichtbar | N/A |
| Klick | Klick in die Karte | Marker dort, Felder z. B. `49.442412` / `10.953901` | N/A |
| Bearbeiten | gespeicherter Ort | Marker auf den gespeicherten Koordinaten | N/A |
| Ziehen | Marker verschoben | Felder folgen beim Loslassen | N/A |
| Eingabe gültig | `49,45` / `10,96`, Feld verlassen | Marker springt, Karte schwenkt hin | N/A |
| Eingabe ungültig | `abc` oder `91` | Marker bleibt, Feld `aria-invalid="true"` | Server prüft beim Speichern wie bisher |
| Ohne JS / ohne Kacheln | Skript oder Kacheln laden nicht | Karte unsichtbar bzw. leer, Speichern über Zahlenfelder klappt | N/A |

**Entscheidungen (2026-10-03):** Keine automatischen JS-Tests und kein neues Werkzeug (Node, Playwright). Go-Tests prüfen Template, Einbindung und Auslieferung; das Kartenverhalten wird von Hand im Browser abgenommen. Volle Spec trotz Überlänge (~2.100 Tokens) behalten.

</frozen-after-approval>

## Code Map

- `internal/adapter/admin/templates.go` -- `//go:embed static` schließt Unterordner ein; Kommentar „htmx and later Leaflet“ anpassen.
- `internal/adapter/admin/handler.go` -- Konstanten `staticPathPrefix`, `htmxPath`; hier Pfade für Leaflet-JS/-CSS und Kartenskript ergänzen. Static-Route ist öffentlich (kein Session-Check), bleibt so.
- `internal/adapter/admin/templates/layout.html` -- fester `<head>`; neuen `{{block "head" .}}{{end}}` einführen, damit nur das Ortsformular Leaflet lädt. CSS-Regel für `[aria-invalid="true"]` und Kartenhöhe ergänzen.
- `internal/adapter/admin/templates/location_form.html` -- Felder `latitude`/`longitude` (IDs gleich den Namen); Kartenfläche `<div id="location-map" hidden>` direkt danach.
- `internal/adapter/admin/static/` -- neu: `leaflet/leaflet.js`, `leaflet/leaflet.css`, `leaflet/images/*.png`, `leaflet/LICENSE`, `location-map.js`.
- `internal/adapter/admin/handler_test.go:446` -- `TestStaticFilesAreServedWithoutSession` als Muster für Leaflet/Skript-Auslieferung.
- `internal/adapter/admin/locations_test.go:240` -- `TestNewLocationFormHasThreeAddressFieldsInsteadOfAddress`, `TestEditFormShowsStoredValues` als Muster; `assertBodyContains`/`assertBodyLacks` wiederverwenden.
- `README.md:108` -- erwähnt den Kartenpicker schon; Hinweis zu vendortem Leaflet und OSM-Nutzungsrichtlinie ergänzen.

## Tasks & Acceptance

**Execution:**
- [x] `internal/adapter/admin/static/leaflet/` -- Leaflet 1.9.4 aus dem npm-Tarball `dist/` plus `LICENSE` ablegen -- kein CDN (Spine).
- [x] `internal/adapter/admin/handler_test.go`, `handler.go` -- Test: Leaflet-JS, -CSS, ein Marker-Bild und `location-map.js` kommen ohne Session mit 200; Pfad-Konstanten.
- [x] `internal/adapter/admin/locations_test.go`, `templates/layout.html`, `templates/location_form.html` -- Tests: Neu- und Bearbeiten-Formular binden Leaflet-CSS/-JS und Kartenskript ein, enthalten die Kartenfläche mit `hidden`; Login- und Listenseite laden kein Leaflet. Dann Template und `head`-Block.
- [x] `internal/adapter/admin/static/location-map.js` -- Verhalten laut Matrix, ohne Abhängigkeit außer `L`.
- [x] `README.md` -- vendortes Leaflet, OSM-Attribution/Nutzungsrichtlinie.

**Acceptance Criteria:**
- Given eine angemeldete Session, when `/admin/locations/new` im Browser lädt, then erscheinen Karte auf Zirndorf und OSM-Attribution, und das Browser-Netzwerkprotokoll zeigt Leaflet nur von `/admin/static/`.
- Given ein Browser ohne JavaScript, when ein Ort mit getippten Koordinaten gespeichert wird, then landet er wie bisher in der Liste.

## Implementation Notes

- Leaflet-Dateien byte-gleich mit `leaflet-1.9.4.tgz` (`cmp` gegen `dist/` und `LICENSE`); `leaflet.js.map` bewusst weggelassen. `.gitattributes` setzt `static/leaflet/** -text`, weil die Upstream-Dateien CRLF enthalten und `text=auto eol=lf` sie sonst umwandeln würde.
- Pfad-Konstanten in `handler.go`; Template nutzt den neuen `head`-Block im Layout. Content-Type von `.js` hängt vom Betriebssystem ab (Windows `application/javascript`, Linux `text/javascript`), der Test prüft nur „javascript“.
- Skript prüft Zahlen wie `strconv.ParseFloat` (Dezimalschreibweise, kein `0x10`/`Infinity`).
- Matrix-Zeilen sind Browserverhalten; laut Entscheidung ohne JS-Tests, Abnahme von Hand. Go-Tests decken Einbindung, versteckte Kartenfläche, Auslieferung und Formular nach Fehler ab. Der Subagent hat das Skript zusätzlich einmalig mit einer Fake-Leaflet-Umgebung unter Node durchgespielt (nicht eingecheckt).
- Lokal verifiziert (2026-10-03): `CI= go test ./...`, Coverage-Gate 355/355, `go vet`, golangci-lint v2.14.0 ohne Befund.

## Spec Change Log

## Review Triage Log

| # | Quelle | Befund | Verdikt | Evidenz | Route |
|---|--------|--------|---------|---------|-------|
| 1 | gap | IDs `latitude`/`longitude`, die das Skript sucht, sind von keinem Test festgehalten | low | Umbenennen im Template ließe alle Go-Tests grün, Karte bliebe versteckt; Fix ist eine Assertion | patch |
| 2 | gap | Kartenverhalten im Skript ungetestet | low | Entscheidung 2026-10-03: keine JS-Tests, Abnahme von Hand; Kern prüft beim Speichern | reject |
| 3 | gap | Bearbeiten-Formular nach Fehler nicht auf Karteneinbindung geprüft | low | Gleiches `locationFormTemplate` wie die drei geprüften Fälle | reject |
| 4 | edge + blind | Klick auf Weltkopie: Marker unge-wrappt, Felder gewrappt | low | `placeMarker(event.latlng)` vor `writeFields(position.wrap())`; Fix: einmal wrappen | patch |
| 5 | edge + blind | Hex-Floats (`0x1p4`) markiert das Skript als ungültig, der Kern speichert sie; Kommentar behauptet Gleichheit mit `ParseFloat` | low | Niemand tippt Hex im Admin; nur der Kommentar ist falsch | patch |
| 6 | blind | Server-Feldmeldung bleibt nach Korrektur per Karte stehen; Server setzt kein `aria-invalid` | low | Nur nach Fehler plus Korrektur per Karte; verschwindet beim Speichern; Fix braucht zusätzliche Logik in Skript und Template | reject |
| 7 | blind | Fehlerhaftes Feld nur farblich markiert | low | Markierung per `aria-invalid` ist im eingefrorenen Intent so festgelegt; Server liefert beim Speichern die Textmeldung | reject |
| 8 | blind | Geleerte Felder lassen den Marker stehen | false | Intent: „ein leeres Feld markiert nichts und bewegt nichts“ | reject |
| 9 | blind | Template schreibt Static-Pfade literal statt Konstanten | low | Bestehendes Muster (1.3 #12, 1.4 #20), auch htmx im Layout | reject |
| 10 | blind | Kein Caching für eingebettete Static-Dateien | low | Gilt schon für htmx; ein Admin, ~150 KB je Formularaufruf | reject |
| 11 | blind | Skript als IIFE mit festen IDs nicht in per htmx geladenes Teilformular (Story 1.8) einbindbar | medium | Story 1.8 lädt „Neuer Ort“ inline per htmx; Teilantworten bekommen keinen `head`-Block und lösen kein Init aus | defer |
| 12 | blind | Kartenfläche ohne zugänglichen Namen | low | `<div id="location-map">` ohne Label; Fix ist ein Attribut | patch |
| 13 | blind | Sprint-Status `in-progress`, Spec `in-review` | false | Workflow setzt den Sprint-Status beim Abschluss | reject |
| 14 | blind | Abnahme der Story nicht belegt; JS-Ausnahme nicht in AGENTS.md | low | Entscheidung 2026-10-03; Abnahme nach Deploy laut Verification | reject |
| 15 | blind | Klick bei Zoom 14 schreibt Scheingenauigkeit (6 Nachkommastellen) | low | Rundung auf 6 Stellen ist im Intent festgelegt; Ortsgenauigkeit trägt die Ungenauigkeit | reject |

## Design Notes

Die Prüfung im Skript wiederholt die Kernregel nur als Eingabehilfe; maßgeblich bleibt `SaveLocation`. Felder und Marker koppeln über `change`, nicht `input`, damit halbe Eingaben wie `49,` den Marker nicht springen lassen. Kein `data-`Konfigurationsweg: Mitte und Zoom sind Konstanten im Skript, weil es genau eine Karte gibt; Story 1.8 bindet dasselbe Skript im Event-Formular ein.

## Verification

**Commands:**
- `CI= go test ./...` -- grün
- `bash scripts/check-coverage.sh` -- 100 %
- `go vet ./...`, golangci-lint v2.14.0 -- ohne Befund

**Manual checks (if no CLI):**
- Nach dem Deploy auf Railway (lokal kein Docker): alle Matrix-Zeilen im Browser durchgehen, einmal mit deaktiviertem JavaScript speichern.
