# Deferred Work

- source_spec: `_bmad-output/implementation-artifacts/spec-1-1-lokales-grundgeruest-mit-ci.md`
  summary: Coverage-Gate `scripts/check-coverage.sh` muss generierten Code (Header `// Code generated ... DO NOT EDIT.`) ausnehmen, sobald oapi-codegen in Story 2.1 Code unter `internal/adapter/publicapi/v1` erzeugt.
  evidence: Das Skript zählt jede Anweisung der Pflichtpakete; die Policy nimmt generierten Code aus, heute existiert noch keiner.
  status: done
  target: Story 2.1 (`scripts/check-coverage.sh` nimmt Dateien mit `// Code generated … DO NOT EDIT.` aus)
- source_spec: `_bmad-output/implementation-artifacts/spec-1-1-lokales-grundgeruest-mit-ci.md`
  summary: AGENTS.md-Abschnitt „Running and verifying“ per bmad-project-context-Refresh mit den Befehlen aus der README füllen.
  evidence: Der Abschnitt enthält noch das TODO „nach Story 1.1 per Refresh eintragen“; Agent-Kontext wird nicht im Build geändert.
  status: done
  target: bmad-project-context-Refresh vom 2026-10-02 (268241f)
- source_spec: `_bmad-output/implementation-artifacts/spec-1-1-lokales-grundgeruest-mit-ci.md`
  summary: Test, dass `run` den Datenbank-Pool beim Shutdown schließt.
  evidence: Nur `defer pool.Close()` in `cmd/eventstore/main.go`; kein Test beobachtet das, eine Prüfung braucht eine Injektionsnaht oder `pg_stat_activity`.
  status: open
  target: Retro Epic 2, B1 (Fix-PR „Epic-2-Kleinigkeiten“); Story 2.6 hat den Eintrag nicht aufgenommen
- source_spec: `_bmad-output/implementation-artifacts/spec-1-2-auslieferung-auf-railway.md`
  summary: Befristetes Log von `X-Forwarded-For` und `RemoteAddr` in `/healthz` (`cmd/eventstore/health.go`) nach der ENT-21-Messung per Folge-PR wieder entfernen, spätestens mit Story 1.3.
  evidence: Entscheidung vom 2026-10-02 in der Spec; im Dauerbetrieb dürfen keine Client-IPs im Log stehen.
  status: done
  target: PR mit 1839757 (Log entfernt, `TestHealthLogsNothingWhenDatabaseAnswers`)
- source_spec: `_bmad-output/implementation-artifacts/spec-1-2-auslieferung-auf-railway.md`
  summary: Backup per `railway ssh --service Postgres -- pg_dump … > datei` vor dem ersten Deploy mit echter Migration (Story 1.4) einmal ausprobieren, Dump-Ende (`-- PostgreSQL database dump complete`) prüfen und einen getesteten Restore-Befehl in der README ergänzen.
  evidence: Unverifiziert (maybe-false, wäre medium): Eine TTY/CRLF-Umwandlung durch `railway ssh` könnte den Dump verfälschen; AD-17 stützt sich auf diesen Dump statt auf Down-Migrationen.
  status: open
  target: Retro Epic 1, A3
- source_spec: `_bmad-output/implementation-artifacts/spec-1-4-orte-anlegen-bearbeiten-und-auflisten.md`
  summary: AGENTS.md-TODO zu Codegen per `bmad-project-context`-Refresh auflösen: sqlc-Befehl (`go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate`, braucht cgo; unter Windows Release-Binary) und den CI-Schritt „generierter Code aktuell“ eintragen.
  evidence: Story 1.4 führt sqlc und die CI-Prüfung ein; die Befehle stehen nur in der README, AGENTS.md darf im Build nicht geändert werden.
  status: done
  target: Retro Epic 1, A4 (bmad-project-context-Refresh vom 2026-10-05)
- source_spec: `_bmad-output/implementation-artifacts/spec-1-12-adresse-in-strasse-plz-und-ort-aufteilen.md`
  summary: AGENTS.md-TODO zu Codegen-Befehlen (sqlc 1.31.1) und CI-Prüfung „generierter Code aktuell“ per `bmad-project-context`-Refresh eintragen.
  evidence: Story 1.12 hat sqlc-Code neu erzeugt; die CI prüft das bereits, AGENTS.md nennt den Befehl aber noch nicht.
  status: done
  target: Retro Epic 1, A4 (bmad-project-context-Refresh vom 2026-10-05)
- source_spec: `_bmad-output/implementation-artifacts/spec-1-5-koordinaten-auf-der-karte-setzen.md`
  summary: Kartenskript `location-map.js` für das per htmx geladene Ortsformular in Story 1.8 wiederverwendbar machen (Init-Funktion, Aufruf nach `htmx:afterSwap`, Leaflet im Event-Formular laden).
  evidence: Das Skript ist eine IIFE, die einmal beim Laden mit festen IDs initialisiert; htmx-Teilantworten haben keinen `head`-Block, Leaflet würde dort weder geladen noch initialisiert.
  status: done
  target: Story 1.8 (`initLocationMapsWithin`, Init per `htmx:load`)
- source_spec: `_bmad-output/implementation-artifacts/spec-1-7-events-anlegen-bearbeiten-und-auflisten.md`
  summary: Contract-Schritt der Adress-Migration (AD-17) als eigener PR: neue Migration entfernt `locations.address` und die Defaults von `street`, `postal_code`, `city`; vorher `pg_dump`.
  evidence: Beim Spec-Umfang von Story 1.7 ausgegliedert (Entscheidung 2026-10-03); Produktionsorte sind laut Andreas vollständig gepflegt.
  status: open
  target: Retro Epic 1, A2
- source_spec: `_bmad-output/implementation-artifacts/spec-1-7-events-anlegen-bearbeiten-und-auflisten.md`
  summary: `RecomputeDerived` auch `name_key` der Orte neu berechnen lassen (AD-16, ENT-17), inklusive Umgang mit dabei entstehenden Namenskonflikten.
  evidence: Story 1.7 deckt laut AC nur `effective*` ab, `title_key` folgt mit 1.10; `name_key` ist keiner Story zugeordnet.
  status: done
  target: Story 2.5, Teil B (Sprint Change Proposal 2026-10-05)
- source_spec: `_bmad-output/implementation-artifacts/spec-1-8-neuer-ort-beim-anlegen-eines-events.md`
  summary: Browser-Tests für das Admin-JavaScript (`location-map.js`, htmx-Austausch inkl. `htmx-config` für 409/422 und OOB-Ortsauswahl) einführen.
  evidence: Das Repo hat kein JS-/Browser-Test-Setup; ein Tippfehler in `dataset.latitudeField`, ein fehlender `htmx:load`-Hook oder eine falsch sortierte `responseHandling`-Regel bliebe bei grüner CI unentdeckt (Review Story 1.8). Bis dahin deckt nur die manuelle Browserprüfung das ab.
  status: open
  target: Retro Epic 3, C3 (Andreas ordnet neu zu oder schließt). Story 3.6 hat den Eintrag nicht aufgenommen, weil sie nur einen Knopf mit `hx-confirm` nach dem Muster des Löschens ergänzt und kein neues Skript einführt; ein JS-/Browser-Test-Setup ist ein eigener Umfang.
- source_spec: `_bmad-output/implementation-artifacts/spec-1-10-warnung-bei-duplikatverdacht.md`
  summary: Contract-Schritt für `events.title_key` (AD-17): neue Migration entfernt den Default `''`, sobald ein Deploy mit Story 1.10 alle Schlüssel per `RecomputeDerived` nachgezogen hat; gut mit dem Contract-Schritt der Adress-Migration bündelbar.
  evidence: Migration `00006` legt `title_key` mit Default `''` an, damit älterer Code nach einem Rollback weiter schreiben kann; der Default ist danach überflüssig.
  status: open
  target: Retro Epic 1, A2
- source_spec: `_bmad-output/implementation-artifacts/spec-2-1-api-vertrag-v1-und-liste-der-event-typen.md`
  summary: AGENTS.md-TODO zu Codegen per `bmad-project-context`-Refresh auflösen: oapi-codegen-Befehl (`go generate ./...`, ruft `oapi-codegen@v2.8.0` mit `oapi-codegen.yaml` auf) und den erweiterten CI-Schritt „Generated code is up to date (sqlc, oapi-codegen)“ eintragen.
  evidence: Story 2.1 führt oapi-codegen und die CI-Prüfung ein; die Befehle stehen nur in der README, AGENTS.md darf im Build nicht geändert werden.
  status: done
  target: Retro Epic 1, A4 (bmad-project-context-Refresh vom 2026-10-05)
- source_spec: `_bmad-output/implementation-artifacts/spec-2-5-taegliche-bereinigung.md`
  summary: Story 2.5, Teil B – vollständige Neuberechnung beim Start: `RecomputeDerived` zieht `name_key` nach, erkennt Kollisionen vorab, merkt fehlgeschlagene Events/Orte im Speicher und markiert sie in Event- und Ortsliste im Admin mit „prüfen“; Postgres-Test für `name_key`.
  evidence: Beim Build von Story 2.5 per Scope-Prüfung abgetrennt (2026-10-05): eigenständig auslieferbar neben Teil A (tägliche Bereinigung mit `archived_at`/`MarkArchived`); einzige Nahtstelle ist die Startreihenfolge in `cmd/eventstore`.
  status: done
  target: Story 2.5, Teil B (eigene Spec; vor Epic 3, Reihenfolge zu 2.6 frei)
- source_spec: `_bmad-output/implementation-artifacts/spec-3-4-testsammlung-ins-format-v1-ueberfuehren.md`
  summary: Planungsartefakte per `bmad-correct-course` an Story 3.4 angleichen (39 Events statt 42, Weihnachtsmarkt im Admin statt in der Datei, Datei unter `testdata/`), vor allem das SM-1-Kriterium von Story 3.5.
  evidence: `epics.md` Z. 194 und 199, PRD SM-2 und Story 3.5 („Wenn Andreas die Datei in Produktion importiert, dann liefert … den Weihnachtsmarkt“) widersprechen der Entscheidung vom 2026-10-06.
  status: done
  target: Sprint Change Proposal 2026-10-06 (vor Story 3.5)
- source_spec: `_bmad-output/implementation-artifacts/spec-2-8-archived-aus-der-leseform-entfernen.md`
  summary: Die Beispiele in `api/v1/openapi.yaml` werden von keinem Test gegen ihre Schemas geprüft.
  evidence: `contract_test.go` prüft nur Feldnamen, Grenzen und `$ref`s; ein Beispiel mit einem gestrichenen Feld wie `archived` bliebe unbemerkt (Seam-Review Story 2.8).
