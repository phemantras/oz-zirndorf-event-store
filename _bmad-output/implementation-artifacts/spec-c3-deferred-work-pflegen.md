---
title: 'C3: deferred-work.md pflegen (Retro Epic 3)'
type: 'chore'
created: '2026-10-07'
status: 'done'
route: 'oneshot'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-3-retro-2026-10-06.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** In `deferred-work.md` zielt der offene Eintrag „Browser-Tests für das Admin-JavaScript“ (aus Spec 1.8) auf Retro Epic 3, C3, also auf diese Maßnahme selbst; der Eintrag aus Spec 2.8 („Beispiele in `openapi.yaml` werden nicht gegen ihre Schemas geprüft“) hat weder `status` noch `target` (Retro Epic 3, Befund S5).

**Approach:** Beide Einträge bekommen ein gültiges Ziel. Entscheidungen von Andreas (2026-10-07):

- „Browser-Tests“: bleibt `open`, `target: Backlog (nächste Retro ordnet zu)`; die bisherige Begründung, warum Story 3.6 ihn nicht aufnahm, bleibt erhalten.
- Eintrag aus Spec 2.8: `status: open`, `target: Backlog (nächste Retro ordnet zu)`.
- Aktions-Item C3 in `sprint-status.yaml` geht im selben PR auf `done`, weil C3 reine Pflege ohne Code ist (kein eigener Chore-PR wie bei C1/C2).

</frozen-after-approval>

## Implementation Notes

- `deferred-work.md`: „Browser-Tests“ auf `target: Backlog …` umgestellt, Begründung wie bei früheren Umzuordnungen an `target` angehängt (Deferred-Regel aus `_bmad/custom/bmad-build.toml`); Eintrag aus Spec 2.8 um `status: open` und `target: Backlog …` ergänzt.
- `sprint-status.yaml`: C3 auf `done`, `last_updated` gesetzt. Branch von `origin/main`; der offene PR #73 (C2 auf `done`) ändert ebenfalls `last_updated`, wer später merged, löst diese eine Zeile auf.
- Kein Code geändert, daher keine Tests, kein Lint.

## Review Triage Log

- C2 in `sprint-status.yaml` noch `open` -- false: erledigt der offene PR #73 (`chore/c2-done`).
- Spec noch `in-progress`, ohne Verification -- false: Status wird beim Abschluss gesetzt; ohne Code gibt es keine Prüfbefehle.
- „Backlog“ wiederholt das Muster aus S5 -- false: Entscheidung von Andreas vom 2026-10-07 (Intent); das Ziel verweist nicht mehr auf eine erledigte Maßnahme.
- `target` enthält Begründung bzw. Herkunft -- false: entspricht der Deferred-Regel („append to it why“) und den bestehenden Einträgen (z. B. „Story 2.6 hat den Eintrag nicht aufgenommen“).
- `evidence` nicht um Epic-3-JS ergänzt -- low, verworfen: Epic 3 hat kein neues Skript eingeführt (Story 3.6 nur `hx-confirm`), die Aussage bleibt richtig.
- Übrige offene Ziele nicht geprüft -- false: A2, A3, B1 sind laut `sprint-status.yaml` offen, die Ziele also gültig; außerhalb des C3-Umfangs.
- Retro-Dokument hält Ergebnis nicht fest -- false: Retros werden nachträglich nicht geändert; das Ergebnis steht in `sprint-status.yaml`, `deferred-work.md` und dieser Spec.
- C3-Aktionstext irreführend -- false: „neu zuordnen“ ist mit dem Ziel Backlog erfüllt.

