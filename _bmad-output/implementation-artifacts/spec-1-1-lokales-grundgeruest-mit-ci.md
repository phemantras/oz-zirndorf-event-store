---
title: 'Story 1.1: Lokales Grundgerüst mit CI'
type: 'feature'
created: '2026-10-02'
status: 'done'
baseline_commit: 'fb2a2b3cab4ebb23ff950501223f88b46cb98a10'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-1-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Das Repository enthält noch keinen Code. Jede weitere Story braucht ein baubares Go-Gerüst mit lokalem PostgreSQL 18, Migrationen beim Start, Health Check und einer CI, die die Abhängigkeitsrichtung (AD-1) erzwingt.

**Approach:** Go-Modul mit der Spine-Verzeichnisstruktur, `cmd/eventstore` liest Konfiguration, loggt JSON, führt eingebettete goose-Migrationen aus und startet erst danach den HTTP-Server mit `/healthz`. `compose.yaml` für PostgreSQL 18, GitHub Actions mit vet, Unit-, Postgres- und Architekturtest.

## Boundaries & Constraints

**Always:** Go 1.27.1, Versionen exakt aus der Stack-Tabelle (pgx v5.11.0, goose v3.28.0). Test-first. Logs als JSON über `log/slog` auf stdout, nie Secrets im Log. `time/tzdata` in `cmd/eventstore` eingebettet. Pflichtvariablen in dieser Story: `DATABASE_URL`, `PORT`. Benannte Konstanten statt Magic Strings. Englische Bezeichner und Logs, README deutsch.

**Entscheidungen (2026-10-02):** Go 1.27.1 wird lokal per `winget` systemweit installiert; Docker installiert Andreas selbst, bis dahin laufen Postgres-Tests nur in der CI. golangci-lint läuft ab dieser Story in der CI, gepinnt auf v2.14.0, mit schlanker `.golangci.yml` (Standard-Linter). Volle Spec trotz leichter Überlänge behalten.

**Never:** Keine Fachtabellen (kommen ab Story 1.4). Kein Dockerfile, kein `railway.json` (Story 1.2). Kein OpenAPI, kein sqlc-Code (Story 2.1 / spätere Stories). Keine Admin-Variablen (`ADMIN_*`, `SESSION_SECRET`, Story 1.3). Kein `now()` oder Regel-SQL.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Start ok | `DATABASE_URL`, `PORT` gesetzt, DB erreichbar | Migrationen laufen, danach lauscht Server auf `:PORT` | N/A |
| Variable fehlt | `DATABASE_URL` oder `PORT` leer/ungesetzt | Kein Server, Exit-Code ≠ 0 | JSON-Log nennt fehlende Variable(n) |
| PORT ungültig | `PORT=abc` oder außerhalb 1–65535 | Kein Server, Exit-Code ≠ 0 | JSON-Log nennt ungültigen Port |
| Migration scheitert | DB nicht erreichbar beim Start | Kein Server, Exit-Code ≠ 0 | Fehler geloggt, nicht stillschweigend |
| Health ok | `GET /healthz`, Ping ok | 200 | N/A |
| Health DB weg | `GET /healthz`, Ping schlägt fehl | 503 | Fehler geloggt |
| Shutdown | SIGINT/SIGTERM | Server beendet laufende Anfragen, Pool wird geschlossen | Timeout als benannte Konstante |

</frozen-after-approval>

## Code Map

- Repository leer bis auf `README.md`, `AGENTS.md`, `CLAUDE.md`, `zirndorf_events.json` (Testsammlung, nicht anfassen), `_bmad*` (nie von Hand ändern).
- Remote: `github.com/phemantras/oz-zirndorf-event-store` → Modulpfad.
- Aktuelle Versionen geprüft: setup-go v7.0.0, checkout v7.0.1, golangci-lint-action v9.3.0.

## Tasks & Acceptance

**Execution:**
- [x] `go.mod`, `go.sum` -- Modul `github.com/phemantras/oz-zirndorf-event-store`, `go 1.27.1`, pgx v5.11.0, goose v3.28.0 -- Basis.
- [x] `api/v1/.gitkeep`, `internal/core/doc.go`, `internal/adapter/{publicapi/v1,admin,cleanup}/doc.go` -- Paketkommentare, Verzeichnisse laut Spine -- AC Struktur.
- [x] `internal/adapter/postgres/` -- `Connect` (pgxpool), `Migrate` (goose über `stdlib.OpenDBFromPool`, `embed` von `migrations/*.sql`), Baseline-Migration `00001_baseline.sql` ohne Fachtabellen; Tests gegen echtes PG 18, übersprungen ohne `EVENTSTORE_TEST_DATABASE_URL` -- AC Migrationen.
- [x] `cmd/eventstore/` -- `config.go` (`loadConfig(getenv)`, sammelt alle fehlenden Variablen), `health.go` (Handler über `pinger`-Interface), `main.go` (`run`: Logger → Config → Connect → Migrate → Server, Graceful Shutdown, `_ "time/tzdata"`); Tests für Config und Health -- AC Start, Pflichtvariable, Health.
- [x] `internal/archtest/arch_test.go` -- parst Imports aller Pakete per `go/parser`; `core` nur Stdlib + `golang.org/x/text/unicode/norm`; Adapter importieren keinen fremden Adapter; Negativtest mit Fixture beweist, dass Verstöße erkannt werden -- AC AD-1.
- [x] `compose.yaml` -- `postgres:18`, Volume auf `/var/lib/postgresql`, Port 5432, lokale Dev-Zugangsdaten -- AC Compose.
- [x] `.github/workflows/ci.yaml` -- auf Push nach `main` und Pull Requests: `go vet`, Unit-Tests, Postgres-Tests mit `postgres:18`-Service, Coverage-Gate 100 % für `internal/core`, `internal/adapter/publicapi/v1`, `internal/adapter/admin`, golangci-lint v2.14.0 mit `.golangci.yml` -- AC CI, Policy.
- [x] `README.md` -- Abschnitt „Lokal starten“: Voraussetzungen, `docker compose up -d`, Umgebungsvariablen, `go run ./cmd/eventstore`, Tests -- AC README.

**Acceptance Criteria:**
- Given ein frisch geklontes Repository, when `go build ./...` läuft, then baut es mit Go 1.27.1 fehlerfrei und die Spine-Verzeichnisse existieren.
- Given `compose.yaml`, when `docker compose up` läuft, then startet PostgreSQL 18 mit Volume unter `/var/lib/postgresql`.
- Given ein Push auf `main` oder ein PR, when die Actions laufen, then laufen vet, Unit-, Postgres- und Architekturtest, und ein verbotener Import in `internal/core` oder zwischen Adaptern lässt die CI scheitern.

## Implementation Notes

- Umgebung: Go 1.27.0 per winget unter `C:/Program Files/Go/bin` installiert, aber in laufenden Shells noch nicht im PATH (Bash: `export PATH="/c/Program Files/Go/bin:$PATH"`). `GOTOOLCHAIN=auto` lädt go1.27.1 automatisch, sobald `go.mod` `go 1.27.1` verlangt. Docker ist lokal nicht installiert: Postgres-Tests lokal überspringen, sie laufen in der CI.

- Umgesetzt laut Spec, alle Tasks erledigt. Zusätzlich: `scripts/check-coverage.sh` (Coverage-Gate), `.gitattributes` (`*.sh` mit LF), `golang.org/x/text` auf v0.42.0 angehoben (Stack-Tabelle).
- Postgres-Tests in CI und README mit `-p 1`, weil `internal/adapter/postgres` und `cmd/eventstore` dieselbe Testdatenbank migrieren und parallele goose-Läufe kollidieren können.
- Matrix-Abgleich: „Variable fehlt“, „PORT ungültig“, „Migration scheitert“, „Health ok“, „Health DB weg“ und „Shutdown“ (Server-Teil; das Schließen des Pools ist nicht testabgedeckt, siehe Triage #20) sind durch lokal gelaufene Tests abgedeckt. „Start ok“ (`TestRunMigratesThenServesHealthUntilCancelled`) und die Migrationstests brauchen PostgreSQL 18; mangels Docker lokal übersprungen, sie laufen im CI-Job `postgres` und müssen dort grün sein, bevor die Story `done` wird.

## Spec Change Log

## Review Triage Log

| # | Quelle | Befund | Verdikt | Evidenz | Route |
|---|--------|--------|---------|---------|-------|
| 1 | blind | Coverage-Gate nimmt generierten Code nicht aus | low | Heute existiert kein generierter Code; trifft erst mit oapi-codegen (Story 2.1) zu, die das Gate dann erweitern muss | defer |
| 2 | blind | AGENTS.md „Running and verifying“ noch TODO | medium | Agenten fehlen verifizierte Befehle; Fix ändert Agent-Kontext (laut AGENTS.md per bmad-project-context-Refresh) | defer |
| 3 | blind + edge | Passwort-Leck bei unparsebarer `DATABASE_URL` ungetestet | medium | pgconn `redactPW` ist laut eigenem Kommentar nur best effort; kein Test sichert die Zusage aus `loadConfig` | patch |
| 4 | blind | `.golangci.yml` nur Standard-Linter | false | Schlanke Standard-Konfiguration ist im gesperrten Block entschieden | reject |
| 5 | blind | `.gitattributes` normalisiert nur `*.sh` | medium | `core.autocrlf=true` hier; frischer Checkout erzeugt CRLF, lokales gofmt/golangci-lint meldet dann jede Datei | patch |
| 6 | blind | Server ohne Read/Write/IdleTimeout | low | Railway-Proxy davor, Hobby-Last; im Alltag nicht anzutreffen, Fix fügt Konfiguration hinzu | reject |
| 7 | blind | Health loggt jeden Fehlschlag als Error, kein `Cache-Control` | low | Railway prüft `/healthz` nur beim Deploy; kein Alltagsschaden | reject |
| 8 | blind | Zweites Signal bricht Shutdown nicht ab | low | Shutdown ist auf 10 s begrenzt; Fix braucht Umbau von `run` | reject |
| 9 | blind | Baseline-Test sieht ersten Apply nicht | low | CI-DB ist pro Job frisch, `version >= 1` belegt angewendete Baseline; Umbau mit Schema-Reset zu teuer | reject |
| 10 | blind | Kommentar der Baseline-Migration ist Fehlschluss | low | Begründung „unveränderlich, also leer“ folgt nicht; direkte Korrektur, Datei noch nirgends angewendet | patch |
| 11 | blind | CI-Redundanz, Tag- statt SHA-Pinning, kein `go mod tidy`/govulncheck | low | Tags als Versionen in der Spec entschieden; Redundanz kostet nur Sekunden | reject |
| 12 | edge | Paralleles `Migrate` ohne Session-Locker | low | Genau eine Replika; Tests per `-p 1` serialisiert | reject |
| 13 | edge | `PartialError` liefert Zähler 0 | false | `run` ignoriert den Zähler im Fehlerfall und gibt nur den Fehler zurück | reject |
| 14 | edge | Shutdown-Timeout ohne `server.Close` | low | Health-Ping auf 2 s begrenzt, Prozess endet danach per `os.Exit` | reject |
| 15 | edge | `freePort`-Race im Integrationstest | low | Theoretisch, auf CI-Runnern praktisch nicht anzutreffen | reject |
| 16 | edge | cgo-Import `"C"` gilt als Standardbibliothek | low | `isStandardLibrary("C")` liefert true; direkte Korrektur | patch |
| 17 | edge | Transitive Adapter-Abhängigkeit unerkannt | false | AD-1 regelt direkte Imports; ein Nicht-Adapter-Paket mit Adapter-Import gibt es per Struktur nicht | reject |
| 18 | gap | „Migration vor Server“ nicht verifiziert | medium | Vorgezogenes Listen bliebe grün; `TestRunFailsWhenMigrationFails` prüft keinen Port | patch |
| 19 | gap | Postgres-Tests überspringen in CI still | medium | `t.Skipf` ohne Variable, Job bleibt grün ohne Postgres-Abdeckung | patch |
| 20 | gap | Schließen des Pools beim Shutdown unverifiziert | low | Nur `defer pool.Close()`, kein Test; braucht Injektionsnaht | defer |

## Design Notes

Baseline-Migration: goose verlangt mindestens eine Datei; die erste angewendete Migration ist danach unveränderlich (AD-17), deshalb enthält sie nur einen Kommentar und `SELECT 1`, keine Struktur. `/healthz` liegt in `cmd/eventstore`, weil es weder zur Public API v1 (OpenAPI-generiert) noch zum Admin gehört. Der Health-Ping hat ein eigenes Timeout als Konstante.

## Verification

**Commands:**
- `go build ./...` -- ohne Fehler
- `go vet ./...` -- ohne Befund
- `go test ./...` -- grün (Postgres-Tests ohne DB übersprungen)
- `EVENTSTORE_TEST_DATABASE_URL=… go test ./internal/adapter/postgres/...` -- grün gegen PG 18
- `docker compose up -d && go run ./cmd/eventstore` + `curl -i localhost:$PORT/healthz` -- 200; nach `docker compose stop` -- 503
