---
title: 'Story 3.4: Testsammlung ins Format v1 überführen'
type: 'chore'
created: '2026-10-06'
status: 'done'
route: 'oneshot'
baseline_commit: '920a1fc4231070bcbc231a660f0f7f687d38c331'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-3-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Andreas hat die Testsammlung selbst ins Format v1 überführt (`C:\Users\andre\Downloads\zirndorf_import.json`, 39 Events) und in Produktion importiert. Im Repo liegt aber nur die Quelldatei im Root; Story 3.5 braucht die v1-Datei unter `testdata/` für ihren Integrationstest.

**Approach:** Quelldatei unverändert per `git mv` nach `testdata/zirndorf_events.json`, Andreas' Datei unverändert als `testdata/zirndorf_events.v1.json`. Ein Kern-Test (test-first) belegt: Gegen einen leeren Bestand ist jeder der 39 Einträge in der Vorschau `new`, kein Eintrag fehlerhaft, jeder `importKey` gesetzt und eindeutig. Die gültige Zahl ist 39.

Entscheidungen von Andreas (2026-10-06): Seine Datei ersetzt die Freigabeliste; ihre Werte gelten als freigegeben. Der Weihnachtsmarkt ist nicht in der Datei, sondern im Admin angelegt. Keine Änderung an Kern, Schema oder `openapi.yaml`.

</frozen-after-approval>

## Implementation Notes

- Gültige Zahl: **39 Events** statt 41 (Quelldatei) bzw. 42 (PRD/SM-2). Andreas hat Stadtführung, Museums-Herbstmarkt und Innenstadt-Herbstmarkt zu „Zirndorfer Herbstmarkt“ mit drei Programmpunkten zusammengelegt (41 − 2 = 39); die 42 aus Brief und PRD war ein Zählfehler. Der Weihnachtsmarkt (SM-1) ist im Admin angelegt, nicht in der Datei. 19 Orte.
- `testdata/zirndorf_events.v1.json` ist byte-gleich mit Andreas' Datei (SHA-256 `d62f9a86ccb0d90c16cd148ba9366958a9ef3c79fcc4fb118e27e9bdf59ce382`); `testdata/zirndorf_events.json` per `git mv`, unverändert.
- Test `TestPreviewImportAcceptsTheTestCollection` in `internal/core/import_test.go`: erst rot (Datei fehlt), dann grün.
- Schema-Prüfung einmalig mit Python `jsonschema` (Draft 2020-12, `$ref` auf `openapi.yaml` aufgelöst, Formatprüfung): 0 Fehler. Kein Validator als Modulabhängigkeit.
- `.dockerignore`: `zirndorf_events.json` → `/testdata`, damit die Testdaten weiter nicht im Build-Kontext landen.

## Verification

- `go test ./internal/core/...` (mit `CI=`) -- grün
- Schema-Prüfung: Python-Skript mit `Draft202012Validator(import-v1.schema.json)`, `referencing`-Registry mit `openapi.yaml` unter dem Namen `openapi.yaml`, `FormatChecker`; ausgeführt mit `uv run --no-cache --with jsonschema --with pyyaml --with referencing --with rfc3987 python validate.py` -- 0 Fehler, 39 Events
- `bash scripts/check-coverage.sh` -- 2108 von 2108 Anweisungen
- golangci-lint v2.14.0 -- 0 Befunde

## Review Triage Log

| # | Quelle | Befund | Urteil | Begründung | Route |
|---|---|---|---|---|---|
| 1 | blind | Schema-Gültigkeit ohne nachvollziehbaren Befehl | low | Der CI-Schutz ist der Kern-Test; der einmalige Lauf war nicht dokumentiert. | patch: Verification ergänzt |
| 2 | blind | Spec ohne Code Map, Tasks, Boundaries | false | Route `oneshot`; die Abschnitte entfallen dort laut Vorlage. | reject |
| 3 | blind | Freigabeliste fehlt | false | Eingefrorene Entscheidung von Andreas: seine Datei ersetzt die Liste. | reject |
| 4 | blind | Planungsartefakte nennen weiter 42 Events, Weihnachtsmarkt aus der Datei, Datei im Repo-Root; AC 3.5 so nicht erfüllbar | medium | `epics.md` Z. 194/199, PRD SM-2, Story 3.5 („Wenn Andreas die Datei … importiert, dann liefert … den Weihnachtsmarkt“). Änderung nur per BMad-Skill. | defer |
| 5 | blind | v1-Datei nicht gestaged | false | Wird mit der Story committet. | reject |
| 6 | blind | Byte-Gleichheit nicht prüfbar | low | Prüfsumme fehlte. | patch: SHA-256 ergänzt |
| 7 | blind | Ortsnamen als Adresse, „(Lind)“ im Namen | low | Werte von Andreas freigegeben und in Produktion importiert. | reject |
| 8 | blind | „Str.“ neben „Straße“ | low | Ortsabgleich läuft über `name_key`, nicht über die Straße; nur kosmetisch. | reject |
| 9 | blind | Festmeile und Marktplatz auf derselben Koordinate | low | Zwei bewusst getrennte Orte mit verschiedener Genauigkeit; freigegebene Daten. | reject |
| 10 | blind | Herbstmarkt mit `street` statt `area` | low | Freigegebene Daten; Ermessensfrage, keine Regelverletzung. | reject |
| 11 | blind | Pain & Gain beginnt mit 14:30 statt 15:00 | low | Einlass als Beginn ist vertretbar; Notiz nennt 15:00. Freigegebene Daten. | reject |
| 12 | blind | Quellen ohne URL | false | `url: null` ist erlaubt; keine erfundenen URLs. | reject |
| 13 | blind | Test meldet nicht, welcher Eintrag scheitert | low | Einfache Korrektur. | patch: Meldung je Eintrag mit Klasse und Problemen |
| 14 | blind | Konstante 39 ohne Begründung | low | Einfache Korrektur. | patch: Kommentar |
| 15 | blind | `/testdata` deckt verschachtelte `testdata` nicht ab | false | Vorher schloss `.dockerignore` sie ebenso wenig aus; Verhalten unverändert. | reject |
