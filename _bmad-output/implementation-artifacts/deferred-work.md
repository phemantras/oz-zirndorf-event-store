# Deferred Work

- source_spec: `_bmad-output/implementation-artifacts/spec-1-1-lokales-grundgeruest-mit-ci.md`
  summary: Coverage-Gate `scripts/check-coverage.sh` muss generierten Code (Header `// Code generated ... DO NOT EDIT.`) ausnehmen, sobald oapi-codegen in Story 2.1 Code unter `internal/adapter/publicapi/v1` erzeugt.
  evidence: Das Skript zählt jede Anweisung der Pflichtpakete; die Policy nimmt generierten Code aus, heute existiert noch keiner.
- source_spec: `_bmad-output/implementation-artifacts/spec-1-1-lokales-grundgeruest-mit-ci.md`
  summary: AGENTS.md-Abschnitt „Running and verifying“ per bmad-project-context-Refresh mit den Befehlen aus der README füllen.
  evidence: Der Abschnitt enthält noch das TODO „nach Story 1.1 per Refresh eintragen“; Agent-Kontext wird nicht im Build geändert.
- source_spec: `_bmad-output/implementation-artifacts/spec-1-1-lokales-grundgeruest-mit-ci.md`
  summary: Test, dass `run` den Datenbank-Pool beim Shutdown schließt.
  evidence: Nur `defer pool.Close()` in `cmd/eventstore/main.go`; kein Test beobachtet das, eine Prüfung braucht eine Injektionsnaht oder `pg_stat_activity`.
- source_spec: `_bmad-output/implementation-artifacts/spec-1-2-auslieferung-auf-railway.md`
  summary: Befristetes Log von `X-Forwarded-For` und `RemoteAddr` in `/healthz` (`cmd/eventstore/health.go`) nach der ENT-21-Messung per Folge-PR wieder entfernen, spätestens mit Story 1.3.
  evidence: Entscheidung vom 2026-10-02 in der Spec; im Dauerbetrieb dürfen keine Client-IPs im Log stehen.
- source_spec: `_bmad-output/implementation-artifacts/spec-1-2-auslieferung-auf-railway.md`
  summary: Backup per `railway ssh --service Postgres -- pg_dump … > datei` vor dem ersten Deploy mit echter Migration (Story 1.4) einmal ausprobieren, Dump-Ende (`-- PostgreSQL database dump complete`) prüfen und einen getesteten Restore-Befehl in der README ergänzen.
  evidence: Unverifiziert (maybe-false, wäre medium): Eine TTY/CRLF-Umwandlung durch `railway ssh` könnte den Dump verfälschen; AD-17 stützt sich auf diesen Dump statt auf Down-Migrationen.
- source_spec: `_bmad-output/implementation-artifacts/spec-1-4-orte-anlegen-bearbeiten-und-auflisten.md`
  summary: AGENTS.md-TODO zu Codegen per `bmad-project-context`-Refresh auflösen: sqlc-Befehl (`go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate`, braucht cgo; unter Windows Release-Binary) und den CI-Schritt „generierter Code aktuell“ eintragen.
  evidence: Story 1.4 führt sqlc und die CI-Prüfung ein; die Befehle stehen nur in der README, AGENTS.md darf im Build nicht geändert werden.
