---
title: 'Story 1.2: Auslieferung auf Railway'
type: 'feature'
created: '2026-10-02'
baseline_commit: '634a0b53988037919851c134c7ebf341a8022a5f'
status: 'done'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-1-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Das Grundgerüst aus Story 1.1 läuft nur lokal. Jede weitere Story soll direkt in einem laufenden System landen, und Story 1.3 braucht das gemessene Railway-Verhalten von `X-Forwarded-For` (ENT-21) für die Client-IP.

**Approach:** Multi-Stage-Dockerfile und `railway.json` (Dockerfile-Builder, `healthcheckPath: /healthz`, eine Replika) ins Repo, CI baut das Image mit. Andreas richtet das Railway-Projekt per Dashboard ein (App + PostgreSQL 18 nur im privaten Netz, „Wait for CI“). Nach dem ersten Deploy wird `X-Forwarded-For` per `curl` mit gefälschtem Header gemessen und das Ergebnis samt Deployment-Anleitung und AD-17-Regel in die README geschrieben.

## Boundaries & Constraints

**Always:** Builder-Image Go 1.27.1, statisches Binary (`CGO_ENABLED=0`), Laufzeit-Image ohne Shell als Nicht-root. Konfiguration nur über `DATABASE_URL` und `PORT`; `DATABASE_URL` ist die Railway-Referenz auf die private Postgres-URL. PostgreSQL-Image auf Major 18 gepinnt, nie `:latest`. Test-first für jede Go-Änderung. README deutsch, Code und Logs englisch. Nie Passwörter oder `DATABASE_URL` loggen.

**Never:** Keine Admin-Variablen und keine Client-IP-Logik (Story 1.3). Keine öffentliche TCP-Proxy-Freigabe der Datenbank. Kein Push von Images aus der CI, kein Railway-Token in GitHub. Keine Änderung an Migrationen.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Deploy grün | Push auf `main`, CI grün | Railway baut Dockerfile, `/healthz` 200 → neue Version live | N/A |
| Deploy rot | Push auf `main`, CI rot | Railway überspringt den Deploy | Alte Version bleibt live |
| Health scheitert | Neue Version, DB nicht erreichbar | Healthcheck schlägt fehl, alte Version bleibt live | Fehler im Railway-Log |
| XFF-Messung | `curl -H 'X-Forwarded-For: 203.0.113.7' https://<domain>/healthz` | Ankommender Header steht im Log | Keine Session-/Passwortdaten im Log |

**Entscheidungen (2026-10-02):** Das Log von `X-Forwarded-For` und `RemoteAddr` in `/healthz` ist befristet: Nach der Messung entfernt es ein Folge-PR wieder (Eintrag in `deferred-work.md`, spätestens mit Story 1.3), damit im Dauerbetrieb keine Client-IPs im Log stehen. Volle Spec trotz Überlänge (~2.100 Tokens) behalten.

</frozen-after-approval>

## Code Map

- `cmd/eventstore/health.go` -- `newHealthHandler(db pinger, logger)`: hier kommt das befristete XFF-Log hinein (Info-Zeile, nur bei `/healthz`); `newRouter` unverändert.
- `cmd/eventstore/health_test.go` -- bestehende Handler-Tests mit Fake-`pinger`; Log-Assertion über `slog.NewJSONHandler` auf `bytes.Buffer` ergänzen.
- `cmd/eventstore/config.go` -- `ListenAddress()` liefert `":PORT"` (Dual-Stack); Railway setzt `PORT`. Nichts ändern.
- `cmd/eventstore/main.go` -- `_ "time/tzdata"` eingebettet, Distroless ohne Zonendaten funktioniert also. Nichts ändern.
- `.github/workflows/ci.yaml` -- Jobs `lint`, `unit`, `postgres` laufen auf Push nach `main` (Voraussetzung für „Wait for CI“); neuen Job ergänzen.
- `README.md` -- Abschnitt „Lokal starten“ existiert; Deployment-Abschnitt dahinter einfügen, Status-Zeile oben aktualisieren.
- Keine Railway-CLI, kein Docker lokal: Image-Build nur in der CI verifizierbar, Railway-Einrichtung macht Andreas im Dashboard. `gh` liegt unter `C:/Program Files/GitHub CLI/gh.exe` (in PowerShell im PATH, in Git Bash nicht).

## Tasks & Acceptance

**Execution:**
- [x] `cmd/eventstore/health_test.go`, `health.go` -- erst Test, dann Log von `X-Forwarded-For` (leer, wenn fehlt) und `RemoteAddr` als Info-Zeile mit benannter Konstante für Nachricht/Feldnamen -- ENT-21-Messung.
- [x] `Dockerfile` -- Stage 1 `golang:1.27.1` (Tag beim Umsetzen prüfen), `go mod download` vor `COPY . .`, `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w"`; Stage 2 `gcr.io/distroless/static-debian12:nonroot` (aktuellste Debian-Version prüfen), `USER nonroot`, `ENTRYPOINT` auf das Binary -- Multi-Stage.
- [x] `.dockerignore` -- `.git`, `_bmad*`, `.claude`, `*.md`, `zirndorf_events.json` -- kleiner Build-Kontext.
- [x] `railway.json` -- `$schema`, `build.builder: DOCKERFILE`, `build.dockerfilePath: Dockerfile`, `deploy.healthcheckPath: /healthz`, `deploy.numReplicas: 1` (Schlüssel gegen aktuelles Railway-Schema prüfen), Restart-Policy `ON_FAILURE` -- Deploy-Konfiguration als Code.
- [x] `.github/workflows/ci.yaml` -- Job `docker`: `docker build .` ohne Push -- rotes Dockerfile blockiert per „Wait for CI“ den Deploy.
- [x] `README.md` -- Abschnitt „Deployment auf Railway“: einmalige Einrichtung (Projekt `production`, Postgres-Service mit gepinntem 18er-Image, TCP-Proxy aus, App-Service aus GitHub mit `DATABASE_URL=${{Postgres.DATABASE_URL}}`, „Wait for CI“ an, Domain erzeugen), Ablauf eines Deploys, AD-17-Regeln (`railway run pg_dump …` vor Deploys mit neuer Migration, nur vorwärts, expand/contract, angewendete Migrationen nie ändern), Platzhalter-Abschnitt „Client-IP hinter Railway“ für das Messergebnis -- AC README.
- [x] `_bmad-output/implementation-artifacts/deferred-work.md` -- Eintrag: XFF-Log in `/healthz` nach der Messung entfernen -- befristetes Log.
- [x] Nach Merge und erstem Deploy (Andreas + Agent): XFF-Messung, Ergebnis (links/rechts) in README per eigenem PR -- ENT-21.

**Acceptance Criteria:**
- Given grüne CI auf `main`, when Railway deployt, then antwortet `https://<railway-domain>/healthz` mit 200 und der Postgres-Service meldet `SHOW server_version` = 18.x ohne öffentliche Verbindung.
- Given rote CI, when auf `main` gepusht wird, then erscheint in Railway kein neuer aktiver Deploy.
- Given der erste Deploy läuft, when per `curl` ein gefälschter `X-Forwarded-For` geschickt wird, then steht in der README, ob Railway den Client-Wert verwirft (Client-IP = linker Eintrag) oder anhängt (rechter Eintrag), und der App-Service hat genau eine Replika.

## Implementation Notes

- Images geprüft (2026-10-02): `golang:1.27.1` existiert (Debian trixie), neueste Distroless ist `static-debian13` (`static-debian14` gibt es nicht). Railway-Postgres gepinnt über `ghcr.io/railwayapp-templates/postgres-ssl:18`.
- `railway.json` gegen `https://railway.com/railway.schema.json` geprüft: `build.builder`, `build.dockerfilePath`, `deploy.numReplicas`, `deploy.healthcheckPath`, `deploy.restartPolicyType` sind gültig.
- `.dockerignore` schließt zusätzlich `.github` aus (für den Build irrelevant).
- README: `railway run pg_dump …` erreicht ohne TCP-Proxy die private DB nicht; dokumentiert ist daher `railway ssh --service Postgres -- pg_dump …`. Ebenso `SHOW server_version` per `railway ssh … psql` statt `railway connect` (braucht den TCP-Proxy).
- Log-Zeile `health check request` mit `x_forwarded_for` (leer bei fehlendem Header) und `remote_addr`, bei jedem `/healthz`-Aufruf, also auch bei Railways Health Checks.
- Matrix-Abgleich: „XFF-Messung“ durch `TestHealthLogsForwardedForAndRemoteAddress` und `TestHealthLogsEmptyForwardedForWhenHeaderIsMissing` abgedeckt, „Health scheitert“ (503 bei Ping-Fehler) durch `TestHealthReturnsUnavailableAndLogsWhenPingFails`; beide lokal grün. „Deploy grün“ und „Deploy rot“ sind Railway-Verhalten und nur manuell nach dem Merge prüfbar (Checkliste in der README); ebenso ob die Image-Builds gelingen (erstmals im CI-Job `docker`). Offener Task: XFF-Messung nach dem ersten Deploy, Story bleibt bis dahin nicht `done`.
- Messung ENT-21 (2026-10-02, nach Deploy von PR #5): Ohne Header kam `<Client-IP>, <Edge-IP>` an, mit gefälschtem `203.0.113.7` ebenfalls `<Client-IP>, <Edge-IP>`. Railway verwirft den Client-Wert, Client-IP = linker Eintrag; `RemoteAddr` ist `100.64.x.x` (intern). Beide Anfragen trafen dieselbe Replika. Ergebnis in der README, XFF-Log im selben Folge-PR entfernt (Test `TestHealthLogsNothingWhenDatabaseAnswers` zuerst rot).

## Spec Change Log

## Review Triage Log

| # | Quelle | Befund | Verdikt | Evidenz | Route |
|---|--------|--------|---------|---------|-------|
| 1 | edge | Mehrzeiliger `X-Forwarded-For` wird nur mit erster Zeile geloggt | medium | `Header.Get` liefert nur den ersten Wert; die ENT-21-Messung könnte links/rechts falsch ablesen | patch |
| 2 | edge | Riesiger Header landet ungekürzt im Log | low | Nur bei gezieltem Missbrauch, Log ist befristet; Kürzung bringt neue Logik | reject |
| 3 | edge | Kein Test für mehrzeiligen Header | medium | Gehört zu #1, Test wird mit dem Fix ergänzt | patch |
| 4 | edge + blind | `railway.json` ohne `healthcheckTimeout`, `restartPolicyMaxRetries`, `watchPatterns` | low | Railway-Standard 300 s reicht für die Baseline-Migration; Deploys bei README-Änderungen sind harmlos | reject |
| 5 | edge + blind | Backup per `railway ssh … pg_dump >` evtl. durch TTY/CRLF verfälscht, kein Restore-Weg | maybe-false (wäre medium) | Abweichung von `railway run` ist in den Implementation Notes begründet; ob die Ausgabe sauber ist, zeigt erst ein echter Dump samt Restore-Test | defer |
| 6 | blind | `.dockerignore` schließt `.env*` und Coverage-Dateien nicht aus | low | Lokaler `docker build` würde eine `.env` in eine Build-Schicht kopieren; direkte Ergänzung | patch |
| 7 | blind | Basis-Images nicht per Digest gepinnt | low | Tag-Pinning wie in Story 1.1 (#11) entschieden | reject |
| 8 | blind + gap | CI baut das Image, startet es aber nie | medium | Falscher `ENTRYPOINT` oder dynamisches Binary fiele erst beim Railway-Deploy auf; README verspricht mehr | patch |
| 9 | blind | Entfernung des XFF-Logs nicht nachverfolgt | false | Eintrag in `deferred-work.md` existiert (außerhalb des geprüften Diffs) | reject |
| 10 | blind | Log-Test prüft Status 200 nicht mit; 503-Pfad mit zwei Logzeilen | low | Status ist durch die bestehenden Health-Tests abgedeckt, der 503-Test nutzt `strings.Contains` | reject |
| 11 | blind | Deployte Version nicht identifizierbar | low | Nicht gefordert; Railway zeigt Commit je Deploy im Dashboard | reject |
| 12 | blind | README erklärt Überlappung alter/neuer Instanz und Teil-Migrationen nicht | false | README begründet expand/contract mit der laufenden alten Version; goose führt jede Migration in einer Transaktion aus | reject |
| 13 | gap | `healthcheckPath` in `railway.json` nicht gegen `healthPath` geprüft | medium | Umbenennen der Konstante ließe alle Tests grün und jeden Deploy scheitern | patch |
| 14 | gap | README-Satz „kaputtes Dockerfile macht CI rot“ überzogen | low | Mit #8 wird er zutreffend | patch |

## Design Notes

Railway setzt `PORT` selbst; die App bindet an `":PORT"` (IPv4 und IPv6), passend zum privaten Netz. Distroless `static:nonroot` reicht, weil das Binary statisch ist und Zonendaten per `time/tzdata` eingebettet sind. Die Railway-Einrichtung bleibt Dashboard-Arbeit und wird nur in der README dokumentiert: kein API-Token im Repo, kein Infrastruktur-Code für einen Hobby-Betrieb.

## Verification

**Commands:**
- `go test ./cmd/eventstore/...` (mit `CI=`) -- grün, neuer Log-Test eingeschlossen
- `go vet ./...` und golangci-lint v2.14.0 -- ohne Befund
- CI-Job `docker` im PR -- Image baut

**Manual checks (if no CLI):**
- Railway-Dashboard: ein aktiver Deploy, Replikas = 1, Postgres-Image-Tag 18, kein TCP-Proxy auf Postgres; `curl -i https://<domain>/healthz` → 200.
