---
title: 'Story 1.6: Ehrliches Zeitmodell im Kern'
type: 'feature'
created: '2026-10-03'
status: 'done'
baseline_commit: '97ab2aaed14b8791e00db421871eb799e670cf87'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-1-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Es gibt noch kein Zeitmodell: Ohne eine Stelle, die Zeitangaben prüft, die Genauigkeit ableitet und den effektiven Zeitraum berechnet, würden Admin (1.7), Import (Epic 3) und API (Epic 2) „vorbei“ und Tagesgrenzen unterschiedlich auslegen (FR-2, NFR-6, AD-3, AD-4, AD-16, ENT-1, ENT-2). Die Akzeptanzkriterien der Story 1.6 in `epics.md` gelten vollständig.

**Approach:** Reiner Kern-Code ohne Datenbank und Adapter: Typen für lokales Datum und Uhrzeit, der Wert `EventTimes` mit Prüfung, abgeleiteter Genauigkeit und effektivem Zeitraum, die einzige Umrechnung `ToInstant` (Europe/Berlin) und der Port `Clock` für die Vorbei-Regel.

## Boundaries & Constraints

**Always:** Leere Uhrzeit ist `nil` („unbekannt“), nie 00:00; ein Datum ohne Wert (Nullwert) fehlt. `allDay` gilt für Beginn und Ende gemeinsam. Zeitraum halboffen `[start, end)`; Tagesgrenzen per Kalenderrechnung (Folgetag 00:00 lokal), nie `+24h`. Jede Umrechnung lokal → Zeitpunkt nur über `ToInstant`; Frühjahrslücke → `ErrValidation`, doppelte Herbststunde → früherer Offset (+02:00). Prüfung meldet alle Probleme gesammelt als `*ValidationError` mit Feldnamen `startDate`, `startTime`, `endDate`, `endTime`, `allDay` (Konstanten wie `LocationField*`). Genauigkeits-Codes `exact`, `dateOnly`, `allDay` als Konstanten (AD-9). Test-first, 100 % Abdeckung für `internal/core`, Tests ohne DB mit fester `Clock`.

**Never:** Keine Speicherung der Genauigkeit (AD-3). Kein `time.Now()` im Kern. Keine Änderungen an Datenbank, Migrationen, Adaptern, `cmd/eventstore`. Kein `EventInput`, kein Parsen von Text (`YYYY-MM-DD`, `HH:MM`), kein `SaveEvent` (Story 1.7). Keine neue Modulabhängigkeit.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Nur Beginn-Datum | 2026-10-16 | `dateOnly`/leer; 16. 00:00 bis 17. 00:00 | N/A |
| Eintägig | Beginn = Ende = 2026-10-16, ohne Uhrzeit | gültig, 16. 00:00 bis 17. 00:00 | N/A |
| Exakt bis Datum | 2026-10-16 19:00 bis 2026-10-16 | `exact`/`dateOnly`; Ende 17. 00:00 | N/A |
| Ganztägig | `allDay`, 16. bis 19. | `allDay`/`allDay`; 16. 00:00 bis 20. 00:00 | N/A |
| Kein Beginn-Datum | Nullwert | — | `startDate` `missing` |
| `allDay` + Uhrzeit | `allDay` mit Beginn- oder End-Uhrzeit | — | betroffenes Uhrzeitfeld `conflictsWithAllDay` |
| End-Uhrzeit ohne End-Datum | `endTime` gesetzt, `endDate` leer | — | `endDate` `missing` |
| Ende nicht nach Beginn | 16. 20:00 bis 16. 20:00 | — | `endTime` (bzw. `endDate` ohne Uhrzeit) `notAfterStart` |
| Frühjahrslücke | 2027-03-28 02:30 | `ToInstant` → `ErrValidation` | Feld `nonexistentTime` |
| Herbst doppelt | 2026-10-25 02:30 | +02:00 (00:30Z) | N/A |
| Herbst Ende davor | 25.10. 02:30 bis 02:15 | — | `endTime` `notAfterStartRepeatedHour` |
| Kalender-Unsinn | 2026-02-30 oder 24:00 | — | Feld `invalidFormat` |
| Vorbei-Regel | Ende heute 14:00; Uhr 13:59 / 14:00 | aktiv / vorbei | N/A |

**Entscheidung (2026-10-03):** Volle Spec trotz Überlänge (~2.000 Tokens) behalten.

</frozen-after-approval>

## Code Map

- `internal/core/errors.go` -- `ErrValidation`, `FieldProblem`-Konstanten, `ValidationError`: neue Probleme `conflictsWithAllDay`, `notAfterStart`, `notAfterStartRepeatedHour`, `nonexistentTime` hier ergänzen; `missing`, `invalidFormat` wiederverwenden.
- `internal/core/location.go` -- Muster: Feldnamen-Konstanten mit AD-9-Kommentar, `report`-Closure sammelt alle `FieldError`, Typ-Konstanten mit Liste (`LocationPrecisions`). Namenskollision beachten: dort heißen die Codes `Precision*`, hier daher `TimePrecision*`.
- `internal/core/` -- neu: `localtime.go` (`LocalDate`, `LocalTime`, `ToInstant`, Zone Europe/Berlin), `eventtimes.go` (`EventTimes`, Genauigkeit, `EffectivePeriod`), `clock.go` (Port `Clock`, `Period.IsOver`), jeweils mit `_test.go`.
- `internal/core/location_test.go` -- Teststil: Tabellen, `errors.As` auf `*ValidationError`, einzelne Felder ändern.
- `internal/archtest/arch_test.go` -- erlaubt dem Kern nur Standardbibliothek + `norm`; `time` und `time/tzdata` sind Standardbibliothek.
- `cmd/eventstore/main.go:18` -- bettet `time/tzdata` schon ein; nicht ändern. `admin.Config.Now` bleibt wie es ist (Port-Verdrahtung kommt mit 1.7).

## Tasks & Acceptance

**Execution:**
- [x] `internal/core/errors.go`, `errors_test.go` -- neue `FieldProblem`-Konstanten -- Adapter übersetzen sie in 1.7 in deutsche Meldungen.
- [x] `internal/core/localtime_test.go`, `localtime.go` -- `LocalDate{Year, Month, Day}` (Nullwert = fehlt, `IsZero`, `NextDay` per Kalender), `LocalTime{Hour, Minute}`, `ToInstant(LocalDate, *LocalTime) (time.Time, error)` (nil = 00:00 für Tagesgrenzen); Lücke → Fehler, der `ErrNonexistentLocalTime` und `ErrValidation` matcht; ungültige Bestandteile → `ErrValidation`; doppelte Stunde → früherer Offset; Ergebnis in Zone Europe/Berlin. Zone einmal laden; Ladefehler testbar halten (z. B. Loader mit Zonennamen, Panik per `recover` getestet). Testdatei importiert `_ "time/tzdata"`.
- [x] `internal/core/eventtimes_test.go`, `eventtimes.go` -- `EventTimes{StartDate LocalDate; StartTime *LocalTime; EndDate LocalDate; EndTime *LocalTime; AllDay bool}`; `StartPrecision()`, `EndPrecision()` (`TimePrecision`, Ende ohne Datum → `""`); `EffectivePeriod() (Period, error)` prüft alle Regeln der Matrix und liefert `Period{Start, End time.Time}`.
- [x] `internal/core/clock_test.go`, `clock.go` -- `type Clock interface{ Now() time.Time }`, `Period.IsOver(Clock) bool` = `End <= now`; Pflicht-Szenarien aus `epics.md` (14:00-Ende, Kirchweih Fr–Mo, exakt + Datum, eintägig) mit fester Uhr.

**Acceptance Criteria:**
- Given die Kirchweih von Freitag 2026-10-16 bis Montag 2026-10-19 nur als Datum, when eine feste Uhr Montag 23:59 bzw. Dienstag 00:00 zeigt, then ist sie aktiv bzw. vorbei.
- Given ein Beginn 2026-10-16 19:00 mit Ende 2026-10-16 nur als Datum, when die Uhr 2026-10-16 23:59 zeigt, then ist das Event nicht vorbei.
- Given `bash scripts/check-coverage.sh` und der Architekturtest, when sie laufen, then 100 % Abdeckung und keine verbotenen Importe.

## Implementation Notes

- Neue Dateien `localtime.go`, `eventtimes.go`, `clock.go` mit Tests; `errors.go` um vier `FieldProblem`-Codes erweitert. Fehler von `ToInstant`: `ErrInvalidLocalDate`, `ErrInvalidLocalTime`, `ErrNonexistentLocalTime`, alle matchen `ErrValidation`.
- `ToInstant` bestimmt den früheren Offset selbst: Offsets 24 h vor und nach dem Wandzeit-Wert nachschlagen, größeren Offset zuerst probieren. `LocalDate` gilt für die Jahre 1–9999.
- Zone per `mustLoadZone` einmal geladen; Panik bei fehlender Zone per `recover` getestet. Der Kern bettet `time/tzdata` nicht ein (nur `cmd/eventstore` und die Kern-Tests).
- Lokal verifiziert (2026-10-03): `CI= go test ./...`, Coverage-Gate 450/450, `go vet`, golangci-lint v2.14.0 ohne Befund.
- Abnahme (2026-10-03): PR #15 mit grüner CI (inkl. Postgres-Tests) gemerged.

## Spec Change Log

- 2026-10-03, Review-Befund „Hinweis auf doppelte Stunde auch ohne Zusammenhang“ (Blind #1, Edge #6, Verification Gap): Design Note zu `notAfterStartRepeatedHour` an das eingefrorene AC („liegt dadurch das Ende vor dem Beginn“) angepasst. Vermiedener Zustand: Admin bekommt bei 02:30→01:00 die Zeitumstellung als Grund genannt, obwohl das Ende bei jedem Offset vor dem Beginn liegt. Statt Neu-Ableitung als Patch umgesetzt, weil die Korrektur eine Bedingung in `notAfterStartProblem` ist. KEEP: alles Übrige der Umsetzung.

## Review Triage Log

| # | Quelle | Befund | Verdikt | Evidenz | Route |
|---|--------|--------|---------|---------|-------|
| 1 | blind + edge + gap | `notAfterStartRepeatedHour` auch, wenn das Ende bei jedem Offset vor dem Beginn liegt | medium | 25.10. 02:30→01:00: Ende 23:00Z, Beginn 00:30Z bzw. 01:30Z; AC verlangt den Hinweis nur „dadurch“ | patch |
| 2 | blind + edge | 9999-12-31 ohne Uhrzeit → `invalidFormat`, weil `NextDay` Jahr 10000 liefert | low | Real, aber niemand trägt Events im Jahr 9999 ein; Fix braucht Sonderfall | reject |
| 3 | edge | `ToInstant`-Fehler ohne bekannten Sentinel wird verschluckt | false | `ToInstant` liefert nur die drei Sentinels (bzw. deren `errors.Join`) | reject |
| 4 | edge | `IsOver` auf leerer `Period` meldet „vorbei“ | low | Kein Aufrufer nutzt eine Periode aus dem Fehlerpfad; `EffectivePeriod` liefert dort einen Fehler | reject |
| 5 | edge | `IsOver(nil)` panikt | false | Lautes Scheitern bei Verdrahtungsfehler ist korrektes Verhalten | reject |
| 6 | edge | Genauigkeit für ungültige Zeiten ableitbar | low | AC leitet Genauigkeit nur für gültige Angaben ab; Aufrufer prüfen vorher per `EffectivePeriod` | reject |
| 7 | blind | `ToInstant(date, nil)` = 00:00 lädt zum Missbrauch für „unbekannt“ ein | low | Spec legt nil = Tagesbeginn für Tagesgrenzen bewusst fest und dokumentiert es; API-Umbau wäre mehr als eine Korrektur | reject |
| 8 | blind | Strukturfehler verdecken Format-/Lückenfehler (zwei Runden im Formular) | low | Design Note legt die Stufen bewusst fest; Kombination selten | reject |
| 9 | blind | `clock` bezeichnet Port und Uhrzeit | low | Clean-Code-Regel „Namen sagen, was es ist“; reines Umbenennen | patch |
| 10 | blind | `EventFieldAllDay` nur im Test genutzt | low | Spiegelt den OpenAPI-Feldnamen (AD-9) für Story 2.1 | reject |
| 11 | blind | Code-Tests wiederholen nur Literale, AD-9-Abgleich fehlt | low | Enum-Abgleich gegen `openapi.yaml` ist für Story 2.1 in der CI vorgesehen | reject |
| 12 | blind | Zwei Tabellentests ohne `t.Run`, `t.Fatalf` in der Schleife | low | Abweichung vom Teststil der übrigen Tabellen; direkte Korrektur | patch |
| 13 | blind | Kein Test für gültige Zeiträume in der doppelten Stunde | low | Dauer 01:30–03:00 am 25.10. ungeprüft; Tests ergänzen | patch |
| 14 | blind | Probe-Annahme für historische Jahre (Ortszeit vor 1893) ungetestet | maybe-false | Prüfen bräuchte Tests für Jahre vor 1893; Events dort kommen nicht vor, wäre höchstens low | reject |
| 15 | blind | `Period` bietet nur `IsOver`, kein „aktiv“/„begonnen“ | false | Filter für heute/Archiv (AD-16) kommen mit Epic 2; aktiv = `!IsOver` | reject |

## Design Notes

`EndDate` ist wie `StartDate` ein Wert mit Nullwert statt Zeiger: Ein Datum 0000-00-00 gibt es nie, eine Uhrzeit 00:00 schon, deshalb nur Uhrzeiten als Zeiger. Doppelte Stunde erkennen: Ein lokaler Wert ist doppelt, wenn derselbe Wandzeit-Wert auch eine Stunde später (späterer Offset) existiert; `ToInstant` wählt deterministisch den früheren Zeitpunkt, nicht das unspezifizierte Verhalten von `time.Date`. `notAfterStartRepeatedHour` statt `notAfterStart` nur, wenn das Ende in der doppelten Stunde liegt und mit dem späteren Offset nach dem Beginn läge, die Wahl des früheren Offsets also die Ursache ist. Gibt es Strukturprobleme (`missing`, `conflictsWithAllDay`), wird nicht umgerechnet; Lücken- und Formatfehler werden je Feld gemeldet, bevor der Zeitraum verglichen wird.

## Verification

**Commands:**
- `CI= go test ./...` -- grün
- `bash scripts/check-coverage.sh` -- 100 %
- `go vet ./...` und `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./...` -- ohne Befund
