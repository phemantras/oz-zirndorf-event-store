---
title: 'Story 1.8: Neuen Ort direkt beim Anlegen eines Events'
type: 'feature'
created: '2026-10-03'
status: 'done'
baseline_commit: '35e67629ad1130bfbd1c8452b5a57d42fc5e25e8'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-1-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Fehlt der Ort eines Events in der Auswahl, muss der Admin das Event-Formular verlassen, den Ort anlegen und das Event neu ausfüllen (FR-15). Die Akzeptanzkriterien der Story 1.8 in `epics.md` gelten vollständig.

**Approach:** „Neuer Ort“ im Event-Formular lädt per htmx die Ortseingabe mit Kartenpicker in das Formular. Speichern geht über `SaveLocation`; bei Erfolg wird nur der Ortsbereich (Auswahl) ersetzt und der neue Ort ausgewählt, alle anderen Event-Eingaben bleiben im Browser unberührt.

## Boundaries & Constraints

**Always:** Derselbe Anwendungsfall `LocationUseCases.SaveLocation` und dieselben Meldungen wie die Ortsverwaltung (`locationFieldMessages`, `msgNameConflict`, Dezimalkomma per `withDecimalPoints`). Die Ortsfelder gibt es nur einmal als gemeinsames Template, genutzt von Ortsformular und Inline-Eingabe. Inline-Felder hängen per HTML-Attribut `form` an einem eigenen, leeren `<form>` außerhalb des Event-Formulars, tragen dieselben `name`-Werte wie die Kern-Konstanten und IDs mit Präfix (keine Kollision mit `note`, `location-map`); sie werden beim Speichern des Events nie mitgesendet. Fehler 422/409 werden von htmx eingetauscht (`htmx-config` im Layout), andere Fehler nicht. Ohne JavaScript führt „Neuer Ort“ als Link auf `/admin/locations/new`. Test-first, 100 % Abdeckung für `internal/adapter/admin`.

**Never:** Keine Änderung am Kern, an Postgres oder Migrationen. Kein verschachteltes `<form>`. Kein Neuladen oder Zurücksenden des Event-Formulars, keine Event-Werte in den Ort-Anfragen. Keine Ortsbearbeitung inline, kein Löschen. Kein CDN.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Öffnen | Klick „Neuer Ort“ | Fragment mit leeren Ortsfeldern, Karte, „Ort speichern“, „Abbrechen“ | N/A |
| Gültig | Name „Paul-Metz-Halle“, vollständige Adresse, `49,44`/`10,95`, `building` | 200: Eingabe geschlossen, Hinweis „Ort … angelegt und ausgewählt.“, Ortsauswahl per OOB neu mit neuem Ort `selected` | N/A |
| Ungültig | PLZ `9051`, Breite leer | 422: Fragment mit Werten und Meldungen aus 1.4; Ortsauswahl unverändert | je Feld deutsche Meldung |
| Name existiert | Name eines vorhandenen Orts | 409: Meldung aus 1.4 mit Link auf den Ort (neuer Tab) | N/A |
| Abbrechen | Klick „Abbrechen“ | geschlossener Zustand mit „Neuer Ort“, nichts gespeichert | N/A |
| Ohne Session | htmx-Anfrage | `HX-Redirect` zur Anmeldung (bestehend) | N/A |
| Fehler sonst | `SaveLocation`/`ListLocations` scheitert | 500, geloggt, nichts eingetauscht | N/A |

**Entscheidungen ohne Rückfrage (2026-10-03):** „Abbrechen“ schließt die Eingabe; Erfolgshinweis im geschlossenen Bereich (`role="status"`); Konflikt-Link öffnet in neuem Tab, damit das Event-Formular nicht verloren geht; ein alter Fehler am Ort-Feld verschwindet mit dem Austausch der Auswahl.

</frozen-after-approval>

## Code Map

- `internal/adapter/admin/events.go` -- `renderEventForm` baut die Ortsauswahl; in eine Funktion für den Auswahlbereich auslagern, die auch die Erfolgsantwort nutzt. `eventFormPage` um Ortsbereich ergänzen.
- `internal/adapter/admin/locations.go` -- `saveLocation`, `newLocationFormPage`, `inputFromForm`, `withDecimalPoints`, `locationFieldMessages`, `msgNameConflict` wiederverwenden; Felddaten in ein Struct für das gemeinsame Template ziehen (`IDPrefix`, `FormID`, `Values`, `Errors`, `Precisions`, `PrivacyHint`).
- `internal/adapter/admin/handler.go` -- Routen ergänzen; `render` führt immer `layout` aus, für Fragmente eine Variante mit Template-Namen ergänzen (gleiches Puffer-Muster).
- `internal/adapter/admin/templates.go` -- Partial-Datei zu Orts- und Event-Formular parsen; Fragment-Templates registrieren.
- `internal/adapter/admin/templates/location_form.html`, `event_form.html`, `layout.html` -- Felder ins Partial; Event-Formular lädt Leaflet und `location-map.js` im `head`; Layout bekommt `htmx-config`.
- `internal/adapter/admin/static/location-map.js` -- feste IDs durch `data-`Attribute am Kartenbereich ersetzen, alle Kartenbereiche initialisieren, auch nach `htmx:load`, ohne Doppel-Initialisierung.
- `internal/adapter/admin/locations_test.go`, `events_test.go` -- bestehende Assertions zu IDs/Map anpassen, nur wo das Markup sich ändert; `memoryLocationRepo` und Hilfen wiederverwenden.

## Tasks & Acceptance

**Execution:**
- [x] `internal/adapter/admin/templates/location_fields.html`, `location_form.html`, `templates.go`, `locations.go` (+Tests) -- gemeinsames Partial „locationFields“; Ortsformular rendert unverändert (gleiche IDs, Map-Bereich mit `data-`Attributen).
- [x] `internal/adapter/admin/static/location-map.js` -- initialisiert jeden `[data-location-map]`-Bereich mit den genannten Feldern, beim Laden und nach `htmx:load`.
- [x] `internal/adapter/admin/new_location.go` (+Test), `templates/new_location.html`, `handler.go` -- Routen `GET /admin/events/new-location` (offen), `GET /admin/events/new-location/cancel` (geschlossen), `POST /admin/events/new-location` (speichern); Fehlerübersetzung 422/409/500, Erfolg mit OOB-Ortsauswahl; Matrix-Fälle testen.
- [x] `internal/adapter/admin/events.go`, `templates/event_form.html`, `layout.html` (+Tests) -- Ortsbereich mit `id` für OOB, geschlossener „Neuer Ort“-Bereich (`<a href="/admin/locations/new" hx-get=…>`), leeres Ort-`<form>` nach dem Event-Formular, Leaflet im `head`, `htmx-config` für 409/422.

**Acceptance Criteria:**
- Given das Event-Formular, when ich es ohne geöffnete Ortseingabe oder mit ausgefüllter, ungespeicherter Ortseingabe speichere, then enthält die Anfrage keine Ortsfelder und das Event wird wie in 1.7 verarbeitet.
- Given `bash scripts/check-coverage.sh`, golangci-lint und `CI= go test ./...`, when sie laufen, then grün und 100 %.

## Implementation Notes

## Spec Change Log

## Review Triage Log

| # | Quelle | Befund | Verdikt | Evidenz | Route |
|---|--------|--------|---------|---------|-------|
| 1 | gap + blind | JavaScript und htmx-Verhalten (Karte per `data-`Attribut, `htmx:load`, `htmx-config` 409/422, OOB-Austausch) ohne automatischen Test | medium | Kein JS-/Browser-Test-Setup im Repo; nur Markup-Strings werden geprüft | defer |
| 2 | gap + blind | `TestSavingEventIgnoresLocationFields` behauptet mehr, als er prüft | low | Kollidierendes `note` wird nicht doppelt gesendet; serverseitig nicht beweisbar, dass der Browser `form=`-Felder weglässt | patch |
| 3 | blind | Doppelklick auf „Ort speichern“ ersetzt den Erfolg durch 409 für den eigenen Ort | low | Kein `hx-sync`/`hx-disabled-elt` am Ort-Formular; zweiter POST läuft nach Commit des ersten | patch |
| 4 | blind + edge | 5xx/Netzwerkfehler ohne sichtbare Rückmeldung; `ListLocations`-Fehler nach Speichern lässt Ort unsichtbar | low | Real, aber nur bei DB-Ausfall; Abhilfe bräuchte JS-Fehlerbehandlung; Matrix legt 500 ohne Austausch fest | reject |
| 5 | blind + edge | Fokus und Screenreader-Ansage nach Austausch fehlen | low | Ein Admin, bestehendes Formularmuster ohne ARIA-Führung (1.7 Triage #13); Abhilfe mehr als direkte Korrektur | reject |
| 6 | blind | Verworfene Leaflet-Karten werden nicht `remove()`d; Zoom geht nach 422 verloren | low | Wenige Austausche je Seitenaufruf; zusätzlicher Cleanup-Hook nötig | reject |
| 7 | blind | IDs/Pfade doppelt als Literale in Go und Templates | low | Bestehendes Muster der Templates (1.7 Triage #10) | reject |
| 8 | blind + edge | Fragment-Routen ohne `HX-Request` liefern nacktes Fragment | low | Link ohne htmx folgt `href` auf `/admin/locations/new`; nur bei direkt getippter URL | reject |
| 9 | blind | Bearbeiten-Seite hat die Inline-Eingabe ebenfalls, ungetestet | low | Gleicher Code-Pfad `renderEventForm`; Intent schließt Bearbeiten nicht aus, Verhalten erwünscht | reject |
| 10 | blind + edge | Ungespeicherte Inline-Eingabe geht beim Event-Speichern verloren | low | Felder werden absichtlich nicht gesendet; Warnung bräuchte JS-Guard; „Ort speichern“ ist sichtbar | reject |
| 11 | blind | Sprint-Status, Spec-Status und Code Map uneinheitlich | false | Sprint-Status folgt dem Workflow erst am Ende; Code Map nennt `events_test.go` nur „wo das Markup sich ändert“ | reject |
| 12 | blind | `htmx-config` gilt für den ganzen Admin | low | Laut Design Notes beabsichtigt; künftige Fragmente nutzen 409/422 ebenso | reject |
| 13 | blind | `area` wird vor dem `switch` gebaut und bei Erfolg verworfen | low | Gleiches Muster wie `saveLocation`; kosmetisch | reject |
| 14 | edge | Gespeicherter Ort fehlt in der Liste (gleichzeitig gelöscht) | false | Es gibt keinen Löschpfad für Orte (Story 1.11) | reject |
| 15 | edge | 409 für nach Seitenaufruf angelegten Ort: Ort nicht in der Auswahl | low | Nur bei paralleler Anlage in anderem Tab; Abhilfe ist neuer OOB-Pfad | reject |

## Design Notes

Ohne verschachtelte Formulare gehören die Inline-Felder per `form="new-location-form"` zu einem eigenen Formular, das htmx abschickt (htmx 2 liest `form.elements`, das solche Felder enthält). Antwortform bei Erfolg:

```html
<div id="new-location">…„Ort … angelegt und ausgewählt.“ + Link „Neuer Ort“…</div>
<div id="location-choice" hx-swap-oob="true">…select mit neuem Ort selected…</div>
```

`htmx-config`: `responseHandling` = Standard plus `{"code":"409|422","swap":true,"error":false}` vor der `[45]..`-Regel. Das Ortsformular aus 1.4 bleibt ein normales Formular (kein htmx).

## Verification

**Commands:**
- `CI= go test ./...` -- grün
- `bash scripts/check-coverage.sh` -- 100 %
- `go vet ./...`, golangci-lint v2.14.0 -- ohne Befund

**Manual checks (if no CLI):**
- Im Browser: Event teilweise ausfüllen, „Neuer Ort“, Karte klicken, ungültig speichern (Meldungen, Event-Felder unverändert), gültig speichern (Ort ausgewählt, Event-Felder unverändert), Event speichern.
