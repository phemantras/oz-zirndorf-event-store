---
title: "Sprint Change Proposal: Teil B von Story 2.5 einordnen"
status: approved
created: 2026-10-05
author: Andreas (mit Developer-Agent, bmad-correct-course)
scope: minor
---

# Sprint Change Proposal: Teil B von Story 2.5 einordnen

## 1. Problem

**Problem:** Story 2.5 wurde beim Build per Scope-Prüfung geteilt. Teil A (tägliche Bereinigung mit `archived_at` und `MarkArchived`) ist mit PR #40 ausgeliefert, Spec `spec-2-5-taegliche-bereinigung.md` hat `status: done`. Teil B (vollständige Neuberechnung beim Start) steht nur als Eintrag in `deferred-work.md`. Dieser Eintrag hat weder `status` noch `target`, und als `source_spec` steht `none`. Zu klären war, ob Teil B eine eigene Story wird oder in Story 2.5 bleibt.

**Wann es aufgefallen ist:** Nach dem Merge von Teil A (2d7cb47), vor dem Start von Teil B.

**Kategorie:** Organisation der Umsetzung, keine fachliche Änderung.

**Belege:**

- `deferred-work.md`, letzter Eintrag: `source_spec: none`, ohne `status` und `target`.
- `deferred-work.md`, Eintrag aus Spec 1.7 zu `name_key`: `target: Story 2.5 (Sprint Change Proposal 2026-10-05)`. Er beschreibt einen Teil desselben Umfangs und nennt Teil B nicht.
- `sprint-status.yaml`: `2-5-tägliche-bereinigung: in-progress`.
- Implementation Notes von Spec 2.5: „Story 2.5 bleibt im Sprint-Status `in-progress`, bis Teil B (`deferred-work.md`) umgesetzt ist.“

## 2. Auswirkungen

### Epics

- **Epic 2:** Story 2.5 bleibt unverändert, ebenso Titel und ACs. Die ACs zu `name_key`, Ortskollisionen, „prüfen“ für Orte und Ablaufplan-Grenze sind offen. Sie werden in einer zweiten Spec zu Story 2.5 umgesetzt.
- **Reihenfolge:** Teil B ändert keine `/v1`-Antwort. Gegenüber Story 2.6 ist die Reihenfolge deshalb frei. Teil B muss vor Epic 3 fertig sein, weil Story 3.2 Orte über `name_key` zuordnet.
- **Epic 3:** keine Änderung.

### Artefakte

| Artefakt | Auswirkung |
| --- | --- |
| PRD | keine |
| Architektur-Spine | keine. AD-6 und AD-16 beschreiben Teil B bereits vollständig. |
| UX | Es gibt kein UX-Dokument. |
| `epics.md` | keine |
| `epic-2-context.md` | keine. Die Zeile „Neuberechnung beim Start (2.5)“ beschreibt Teil B bereits. |
| `deferred-work.md` | Teil-B-Eintrag vervollständigen, `name_key`-Eintrag auf Teil B verweisen |
| `sprint-status.yaml` | keine, 2.5 bleibt `in-progress` |

### Technik

Dieses Proposal betrifft keinen Code. Für die spätere Spec von Teil B gelten diese Nahtstellen aus dem Bestand:

- `RecomputeDerived` liegt am `EventService`, `name_key` und `SaveLocation` liegen am `LocationService`. Die Menge der Orte, die „prüfen“ brauchen, und ihr Leeren bei `SaveLocation`/`DeleteLocation` brauchen eine Verbindung zwischen beiden. Die Lösung wählt die Spec.
- `UpdateEventDerived` leert `archived_at` bereits (Teil A, Review #1). Das gilt auch, wenn die Neuberechnung ein Event wieder aktiv macht.
- `SaveLocation` hat noch keinen transaktionsgebundenen Kern (G1, Story 3.3). Teil B braucht ihn nicht.

## 3. Empfohlenes Vorgehen

**Direkte Anpassung:** Nur die Nachverfolgung wird angepasst. Es gibt keinen Rollback, und das MVP-Ziel bleibt unverändert.

- **Aufwand:** gering. Betroffen sind zwei Einträge in `deferred-work.md`.
- **Risiko:** gering.
- **Zeitplan:** keine Verzögerung.
- **Abwägung:** Verworfen wurde eine eigene Story für Teil B. Sie hätte `epics.md`, den Epic-Kontext und den Sprint-Status geändert, ohne fachlich etwas zu gewinnen (Entscheidung Andreas, 2026-10-05).

## 4. Die Änderungen im Einzelnen

### 4.1 `deferred-work.md`, Eintrag „Story 2.5, Teil B“

```
OLD:
- source_spec: none
  summary: Story 2.5, Teil B – vollständige Neuberechnung beim Start: …
  evidence: Beim Build von Story 2.5 per Scope-Prüfung abgetrennt (2026-10-05): …

NEW:
- source_spec: `_bmad-output/implementation-artifacts/spec-2-5-taegliche-bereinigung.md`
  summary: Story 2.5, Teil B – vollständige Neuberechnung beim Start: … (unverändert)
  evidence: Beim Build von Story 2.5 per Scope-Prüfung abgetrennt (2026-10-05): … (unverändert)
  status: open
  target: Story 2.5, Teil B (eigene Spec; vor Epic 3, Reihenfolge zu 2.6 frei)
```

**Begründung:** Teil B soll nachverfolgbar sein wie die anderen Einträge (A6). Die Herkunft ist die Spec von Teil A.

### 4.2 `deferred-work.md`, Eintrag zu `name_key` aus Spec 1.7

```
OLD:
  target: Story 2.5 (Sprint Change Proposal 2026-10-05)

NEW:
  target: Story 2.5, Teil B (Sprint Change Proposal 2026-10-05)
```

**Begründung:** Beide Einträge werden mit derselben Spec geschlossen.

## 5. Übergabe

- **Umfang:** Minor. Der Developer-Agent setzt das direkt um.
- **Umsetzung:** Die beiden Änderungen in `deferred-work.md` gehen per PR. Danach startet Teil B per `bmad-build` mit einer eigenen Spec, z. B. `spec-2-5b-neuberechnung-beim-start.md`.
- **Erfolgskriterien:**
  - Beide Einträge in `deferred-work.md` haben `status` und `target`.
  - Mit dem Abschluss von Teil B werden beide Einträge auf `done` gesetzt und Story 2.5 im Sprint-Status ebenfalls.
