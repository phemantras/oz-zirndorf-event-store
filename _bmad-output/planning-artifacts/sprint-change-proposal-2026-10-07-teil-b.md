---
title: "Sprint Change Proposal: Railway-Einstellungen ohne Config as Code"
status: approved
created: 2026-10-07
author: Andreas (mit Dev-Agent, bmad-correct-course)
scope: minor
---

# Sprint Change Proposal: Railway-Einstellungen ohne Config as Code

## 1. Problem

**Auslöser:** Bei der Umsetzung von D1 (Retro Epic 4, PR #86) und dem Folge-PR #87 hat sich gezeigt:

- Railway gibt einem alten Deployment standardmäßig 0 s zwischen SIGTERM und SIGKILL ([Deployment Teardown](https://docs.railway.com/deployments/deployment-teardown)).
- Railway liest Config as Code (`railway.json`) ab dem **2026-12-01** nicht mehr ([Config as Code](https://docs.railway.com/config-as-code/reference)).

Der Ersatz ist Infrastructure as Code (`.railway/railway.ts`). Er wird per CLI angewendet und braucht einen Projekt-Token sowie Node oder ein Go-SDK in Beta. Andreas hat ihn am 2026-10-07 als zu schwergewichtig für einen einzelnen Service verworfen.

**Umgesetzt in PR #89:**

- `railway.json` und `cmd/eventstore/railway_test.go` sind entfernt.
- Folgende Einstellungen stehen jetzt im Railway-Dashboard am App-Service, als Checkliste in der README:
  - Health Check `/healthz`
  - Restart Policy „On Failure“
  - genau eine Replika
  - Builder Dockerfile
- Die Draining-Zeit kommt aus der Service-Variable `RAILWAY_DEPLOYMENT_DRAINING_SECONDS=30`.
- `checkDrainingTime` warnt beim Start auf Railway, wenn die Variable fehlt oder nicht über `shutdownTimeout` (25 s) liegt.

**Kategorie:** Technische Einschränkung der Plattform (Abkündigung). Spine und `epics.md` beschreiben noch `railway.json`.

## 2. Auswirkungen

| # | Punkt | Status | Ergebnis |
| --- | --- | --- | --- |
| 2.1–2.5 | Epics | N/A | Alle Epics sind `done`. Es gibt keine neue Story und keine neue Reihenfolge. |
| 3.1 | PRD | N/A | Das PRD nennt weder `railway.json` noch den Health Check. |
| 3.2 | Architektur | Anpassung | Spine Z. 262 (Deploy), Z. 279 (Structural Seed) |
| 3.3 | UX | N/A | – |
| 3.4 | Weitere Artefakte | erledigt | README, Code und `deferred-work.md` in #89 angepasst. arc42 nennt `railway.json` nicht. |
| – | Epics-Text | Anpassung | `epics.md` Z. 109, 153, 309 (Story 1.2) |

Unverändert bleiben:

- `epic-1-context.md`: Nach der Regel für Epic-Kontexte wird er nie neu geschrieben.
- Reviews, `.memlog.md` und frühere Proposals: Sie sind Protokolle.

## 3. Empfohlener Weg

Die Texte direkt anpassen. Aufwand und Risiko sind gering. Ein Rollback ist nicht nötig, der MVP-Umfang ist nicht betroffen.

## 4. Änderungen

### E1: Spine, Abschnitt Deploy (Z. 262)

ALT:
> **Deploy:** „Wait for CI“ ist aktiviert, Railway deployt also erst, wenn die GitHub Actions grün sind. In `railway.json` sind `healthcheckPath: /healthz` und das Dockerfile eingetragen.

NEU:
> **Deploy:** „Wait for CI“ ist aktiviert, Railway deployt also erst, wenn die GitHub Actions grün sind. Railway baut das `Dockerfile`. Die Deploy-Einstellungen stehen im Railway-Dashboard am App-Service, nicht im Repository: Health Check `/healthz`, Restart Policy „On Failure“, genau eine Replika und die Service-Variable `RAILWAY_DEPLOYMENT_DRAINING_SECONDS=30`. Die Variable muss über `shutdownTimeout` (25 s) liegen, sonst killt Railway das alte Deployment, bevor ein Request an seiner Deadline antworten kann. Die App warnt beim Start, wenn sie fehlt oder zu klein ist. Die README führt die Einstellungen als Checkliste. Config as Code (`railway.json`) liest Railway ab dem 2026-12-01 nicht mehr; Infrastructure as Code (`.railway/railway.ts`) ist für einen einzelnen Service zu schwergewichtig.

Begründung: Das ist der Ist-Stand nach #89, mit der Begründung der Entscheidung.

### E2: Spine, Structural Seed (Z. 279)

Die Zeile `railway.json` entfällt.

### E3: `epics.md`, Starter-Template (Z. 109)

ALT: „… `internal/adapter/{publicapi/v1,admin,postgres,cleanup}/`, `Dockerfile`, `railway.json`, `compose.yaml`, `.github/workflows/ci.yaml`.“

NEU: „… `internal/adapter/{publicapi/v1,admin,postgres,cleanup}/`, `Dockerfile`, `compose.yaml`, `.github/workflows/ci.yaml`.“

### E4: `epics.md`, Betrieb und Deployment (Z. 153)

ALT:
> Ein Multi-Stage-Dockerfile. In `railway.json` stehen `healthcheckPath: /healthz` und das Dockerfile. „Wait for CI“ ist aktiviert.

NEU:
> Ein Multi-Stage-Dockerfile. Health Check `/healthz`, genau eine Replika und die Draining-Zeit (`RAILWAY_DEPLOYMENT_DRAINING_SECONDS`) stehen in den Einstellungen des App-Service im Railway-Dashboard, als Checkliste in der README. „Wait for CI“ ist aktiviert.

### E5: `epics.md`, Story 1.2, Acceptance Criteria (Z. 309)

ALT: „**Wenn** Railway deployt (Multi-Stage-Dockerfile, `railway.json` mit `healthcheckPath: /healthz`, „Wait for CI“ aktiv)“

NEU: „**Wenn** Railway deployt (Multi-Stage-Dockerfile, Health Check `/healthz` in den Service-Einstellungen, „Wait for CI“ aktiv)“

Begründung: Story 1.2 ist `done`. Agenten lesen `epics.md` aber als aktuellen Stand.

## 5. Übergabe

- **Umfang:** minor.
- **Umsetzung:** Dev-Agent ändert Spine und `epics.md` wie in E1–E5 und legt einen PR an.
- **`sprint-status.yaml`:** keine Änderung.
- **Erfolgskriterium:** Kein Planungsartefakt außer Protokollen nennt `railway.json` noch als aktuellen Stand.
