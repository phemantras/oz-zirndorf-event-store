<!-- bmad:context -->
<!-- Verified 2026-10-02 against 0eb5476. Managed by bmad-project-context; edits inside this block are replaced on refresh. Keep anything you want preserved outside the markers. -->

## oz-zirndorf-event-store

Öffentliche, nur lesende REST-API für Veranstaltungen in Zirndorf, dazu eine geschützte Admin-Oberfläche mit JSON-Import. Go, PostgreSQL 18, Hexagonal light (Kern + Adapter), Hosting auf Railway. Referenzprojekt für OpenZirndorf: Code und Tests werden von anderen als Vorlage gelesen.

## Policy

- Nie direkt auf `main` pushen. Code-Änderungen ausschließlich per Pull Request.
- Agenten dürfen committen. Jeder Agent-Commit trägt einen `Co-Authored-By:`-Trailer, der den Agenten nennt.
- Test-first überall: erst ein fehlschlagender Test, dann der Code, dann Refactoring. Gilt für Kern, Handler, Repositories und Templates. Ausgenommen ist nur generierter Code.
- 100 % Testabdeckung für `internal/core`, `internal/adapter/publicapi/v1` und `internal/adapter/admin`, generierter Code ausgenommen. Fehlende Abdeckung mit Tests schließen, nie durch Ausschließen von Code.
- Clean Code nach https://lorbic.com/go-clean-code-guidelines/: Namen sagen, was eine Funktion tut und liefert; eine Aufgabe pro Funktion; benannte Konstanten statt Magic Numbers und Strings; jeden Fehler behandeln oder weitergeben; zusammengehörige Parameter als Struct; Kommentare nur für nicht offensichtliche Logik.
- Nie von Hand ändern: `_bmad/` und `.claude/skills/` (vom BMad-Installer verwaltet). Anpassungen per `bmad-customize` nach `_bmad/custom/`.
- `_bmad-output/planning-artifacts/` nur über BMad-Skills ändern (z. B. `bmad-correct-course`), nie direkt.
- Generierten Code (oapi-codegen, sqlc) nie von Hand ändern. Spec oder Query ändern und neu generieren.
- Angewendete goose-Migrationen nie ändern. Neue Migrationen nur vorwärts und expand/contract (AD-17).

## Where things are

- Verbindliche Architektur (AD-1 bis AD-17, Stack-Versionen): `_bmad-output/planning-artifacts/architecture/architecture-oz-zirndorf-event-store-2026-10-01/ARCHITECTURE-SPINE.md`
- Stories mit Akzeptanzkriterien: `_bmad-output/planning-artifacts/epics.md`
- Stand der Stories: `_bmad-output/implementation-artifacts/sprint-status.yaml`

## Running and verifying

- TODO (nach Story 1.1 per Refresh eintragen): Befehle für Build, Unit-Tests, Postgres-Tests, Codegen, golangci-lint und Coverage-Prüfung.
- Go 1.27.1 und PostgreSQL 18 verwenden, nie Railways `:latest` (= 16). Versionen der Abhängigkeiten aus der Stack-Tabelle des Spines nehmen, nicht aus der lokalen Umgebung.

## Conventions that differ from defaults

- `internal/core` importiert nur die Standardbibliothek und `golang.org/x/text/unicode/norm`. Adapter importieren `core`, nie einander (AD-1).
- Keine Fachlogik in SQL: keine Views, Trigger, Funktionen, `lower()`/`trim()`-Vergleiche, kein `now()`. Die Zeit kommt als Parameter aus dem `Clock`-Port (AD-2).
- Schreiben nur über Kern-Anwendungsfälle (`SaveEvent`, `CommitImport` …). Repository-Ports bieten keine Schreibmethode am Kern vorbei (AD-6).
- Lokale Zeiten nur mit `ToInstant` umrechnen. Filter, „heute“ und Archiv nur über `effective_start`/`effective_end` (AD-4, AD-16).
- Eine leere Uhrzeit heißt „unbekannt“, nie `00:00`.
- Feldnamen und Enum-Codes nur in `api/v1/openapi.yaml` definieren. Das Import-Schema bindet sie per `$ref` ein, die Kern-Konstanten spiegeln sie (AD-9).
- Code, Bezeichner, Fehlermeldungen und Logs englisch. Inhalte und Admin-Oberfläche deutsch.
- Der Kern liefert typisierte Fehler (`ErrValidation`, `ErrNotFound` …). Nur Adapter übersetzen sie in HTTP-Status oder Problem Details.

<!-- /bmad:context -->
