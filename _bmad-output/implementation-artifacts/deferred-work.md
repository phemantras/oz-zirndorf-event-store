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
