---
title: "Sprint Change Proposal: Ort-Adresse in Straße, PLZ und Ort aufteilen"
status: approved
created: 2026-10-02
author: Andreas (mit Developer-Agent, bmad-correct-course)
scope: minor
---

# Sprint Change Proposal: Ort-Adresse in Straße, PLZ und Ort aufteilen

## 1. Problem

**Problem:** Ein Ort hat heute eine Adresse als einzelnes Freitextfeld (`address`). Die Karten-App und andere Abnehmer brauchen Straße, Postleitzahl und Ort als getrennte Felder. Aus einem Freitext lassen sie sich nicht zuverlässig herauslösen.

**Wann es aufgefallen ist:** nach Abschluss von Story 1.4 (Orte anlegen, bearbeiten und auflisten, gemergt als PR #9). Bisher war keine Story fehlgeschlagen. Auslöser ist eine neue Anforderung von Abnehmern.

**Belege:** Die Testsammlung `zirndorf_events.json` zeigt, wie uneinheitlich die Adressen sind:

- `"Volkhardtstraße 33, 90513 Zirndorf, Germany"`
- `"Lindenstraße 51, 90513 Zirndorf-Lind, Germany"`
- `"Lind, 90513 Zirndorf, Germany (Ortsteil-Zentrum, kein exakter Festplatz bekannt)"`
- `"Marktplatz bis Schulsportplatz, 90513 Zirndorf, Germany (gesamte Festmeile, kein Einzelpunkt)"`

## 2. Auswirkungen

### Epics

- **Epic 1:** Es kommt eine Story dazu (1.12), die vor Story 1.5 läuft, weil 1.5 und 1.8 dasselbe Ortsformular erweitern. Story 1.7 übernimmt den Contract-Schritt der Migration. Das Ziel von Epic 1 bleibt gleich.
- **Epic 2:** In Story 2.2 bekommt der Ort in der Leseform ein Objekt `address` aus `street`, `postalCode` und `city`.
- **Epic 3:** Story 3.1 bekommt dasselbe Adress-Objekt im Import-Schema und weitere Fehlerfälle. In Story 3.4 werden die Adressen der Testsammlung aufgeteilt, und es gibt zusätzliche Fälle zur Prüfung.
- Kein Epic wird überflüssig, keins kommt dazu, die Reihenfolge bleibt.

### Artefakte

| Artefakt | Auswirkung |
| --- | --- |
| PRD | Glossar: neuer Begriff „Adresse“. FR-6 legt Aufbau, Pflichtteile und PLZ-Format fest. FR-7, FR-10 und FR-16 bleiben unverändert. |
| Architektur-Spine | Keine Änderung. AD-2 (Adressen nicht per SQL aufteilen) und AD-17 (expand/contract) geben den Ablauf der Migration vor. |
| UX | Es gibt kein UX-Dokument. |
| `epics.md` | FR-6 in der Anforderungsliste, neue Story 1.12, Stories 1.7, 2.2, 3.1 und 3.4 |
| `epic-1-context.md` | Story-Liste und Eintrag „Ort“ |
| `sprint-status.yaml` | neuer Eintrag 1.12 hinter 1.4 |

### Technik (wird mit Story 1.12 umgesetzt)

- Neue goose-Migration `00003`. `00002` ist angewendet und bleibt unverändert.
- Kern: `Location`, `LocationInput`, Validierung und Feldkonstanten `street`, `postalCode`, `city`.
- sqlc-Queries und neu erzeugter Code.
- Admin: Formular, Liste und deutsche Fehlermeldungen.
- README: Beispiel mit dem Objekt `address`.
- Betrieb: vor dem Deploy ein `pg_dump` (AD-17). In Produktion gibt es nur Testorte. Sie werden nach dem Deploy nachgepflegt oder gelöscht.

## 3. Empfohlenes Vorgehen

**Gewählt: Option 1, direkte Anpassung.** Eine neue Story und Textänderungen an Stories, die noch im Backlog sind.

| Option | Bewertung |
| --- | --- |
| 1 Direkte Anpassung | machbar. Aufwand niedrig bis mittel, Risiko niedrig. |
| 2 Story 1.4 zurückrollen | nicht machbar. Das brächte nichts und widerspräche AD-17. |
| 3 MVP überprüfen | nicht nötig. Das MVP ist nicht betroffen. |

**Begründung:** In Produktion gibt es nur Testdaten, und die öffentliche API und das Import-Format gibt es noch nicht. Darum bricht die Änderung keinen Vertrag. Weil sie vor Story 1.5 kommt, muss niemand doppelt am Ortsformular arbeiten.

**Bekannte Folge:** Alle drei Adressteile sind Pflicht, auch für Orte mit Genauigkeit `district` oder `area`. Als Straße gilt dort die Straße der Ortsmitte oder eine Bereichsangabe. Die Ortsgenauigkeit zeigt, dass die Angabe ungenau ist (SM-C1). Story 3.4 legt diese Fälle Andreas zur Prüfung vor.

## 4. Die Änderungen im Einzelnen

### 4.1 PRD (`prds/prd-oz-zirndorf-event-store-2026-10-01/prd.md`)

**§3 Glossar:** Unter dem Eintrag „Ort“ kommt ein neuer Eintrag dazu:

> **Adresse** — Teil eines Orts, aufgeteilt in Straße (mit Hausnummer, falls vorhanden), Postleitzahl und Ort (Gemeinde oder Ortsteil, z. B. „Zirndorf“).

**FR-6, erste Konsequenz**

ALT:
> Name, Adresse, Koordinaten und Ortsgenauigkeit sind Pflichtangaben. Ein Ort ohne Adresse oder ohne Koordinaten wird abgelehnt. Auch ein nur ortsteilgenauer Ort hat eine Adresse (z. B. den Ortsteil-Mittelpunkt).

NEU:
> Name, Adresse, Koordinaten und Ortsgenauigkeit sind Pflichtangaben. Die Adresse besteht aus Straße, Postleitzahl und Ort. Alle drei sind Pflicht, die Postleitzahl hat genau fünf Ziffern. Fehlt einer der Teile oder die Koordinaten, wird der Ort abgelehnt. Auch ein nur ortsteilgenauer Ort und ein Bereich haben eine vollständige Adresse. Als Straße gilt dann die Straße der Ortsmitte oder eine Bereichsangabe (z. B. „Marktplatz bis Schulsportplatz“). Die Ortsgenauigkeit zeigt, dass die Angabe ungenau ist.

**Frontmatter:** `updated: 2026-10-02`.

**Begründung:** Abnehmer brauchen die Adresse in festen Feldern. Weil der Begriff „Adresse“ jetzt im Glossar steht, bleiben FR-7, FR-10 und FR-16 ohne Änderung richtig.

### 4.2 Epics: FR-6 in der Anforderungsliste

ALT:
> FR-6: Ein Ort besteht aus Name, Adresse, Koordinaten, Ortsgenauigkeit und optionaler Notiz. Name, Adresse, Koordinaten und Ortsgenauigkeit sind Pflicht. Die Ortsgenauigkeit hat …

NEU:
> FR-6: Ein Ort besteht aus Name, Adresse, Koordinaten, Ortsgenauigkeit und optionaler Notiz. Name, Adresse, Koordinaten und Ortsgenauigkeit sind Pflicht. Die Adresse besteht aus Straße (mit Hausnummer, falls vorhanden), Postleitzahl (fünf Ziffern) und Ort. Alle drei sind Pflicht, auch bei einem nur ortsteilgenauen Ort und bei einem Bereich. Die Ortsgenauigkeit hat …

**Begründung:** Die Anforderungsliste gibt das PRD wieder und muss damit übereinstimmen.

### 4.3 Epics: neue Story 1.12 (in Epic 1 hinter Story 1.11)

> ### Story 1.12: Adresse in Straße, PLZ und Ort aufteilen
>
> Als Admin,
> möchte ich die Adresse eines Orts in Straße, Postleitzahl und Ort getrennt pflegen,
> damit Abnehmer die Adressteile ohne Raten aus einem Freitext lesen können.
>
> **Deckt ab:** FR-6, AD-2, AD-11, AD-17
>
> **Hinweis:** Diese Story zieht Story 1.4 nach (Sprint Change Proposal vom 2026-10-02). Sie läuft vor Story 1.5.
>
> **Datenmodell:** Neue Migration `00003`. Die Spalten `street`, `postal_code` und `city` (`text NOT NULL DEFAULT ''`) kommen hinzu, `address` wird nullable und vom Code nicht mehr gelesen oder geschrieben (expand, AD-17). Die Migration teilt vorhandene Adressen nicht per SQL auf (AD-2). In Produktion gibt es nur Testdaten, die nach dem Deploy nachgepflegt oder gelöscht werden. Erst Story 1.7 entfernt `address` und die Defaults (contract).
>
> **Acceptance Criteria:** (Formular mit drei Feldern; Pflichtprüfung je Feld; PLZ genau fünf Ziffern nach Trimmen; Kennung bleibt gleich; Liste zeigt „Straße, PLZ Ort“, auch Testorte mit leeren Teilen lassen sich anzeigen und bearbeiten. Der vollständige Wortlaut steht in `epics.md`.)
>
> **Außerdem gilt:** NFC-Normalisierung (AD-11); die Feldnamen im Kern sind `street`, `postalCode` und `city`, Story 2.1 legt sie in der Spec als Objekt `address` fest (AD-9); Unit- und Postgres-Tests; README-Beispiel; `pg_dump` vor dem Deploy.

**Begründung:** Story 1.4 ist abgeschlossen, deshalb kommt die Änderung als eigene, kleine Story. Läuft nach einem Rollback wieder der alte Code, kann er weiterhin in `address` schreiben, weil die neuen Spalten Defaults haben. Was gültig ist, prüft weiter nur der Kern.

**API-Form:** In der öffentlichen API wird die Adresse ein Objekt: `"address": { "street", "postalCode", "city" }`. So bleibt der Begriff aus FR-10 als Feld erhalten, und `street` stößt nicht mit dem Code `street` der Ortsgenauigkeit zusammen.

### 4.4 Epics: Story 1.7, Contract-Schritt

Zusätzlicher Punkt unter „Außerdem gilt“:

> - Die Migration dieser Story entfernt die Spalte `locations.address` und die Defaults von `street`, `postal_code` und `city` (contract nach Story 1.12, AD-17). Vorher sind alle Orte in Produktion vollständig gepflegt oder gelöscht.

**Begründung:** AD-17 erlaubt das Entfernen der Spalte erst in einem späteren Deploy. Story 1.7 bringt ohnehin eine Migration mit.

### 4.5 Epics: Story 2.2, Leseform des Orts

ALT:
> `location` (vollständig: `id`, `name`, `address`, `latitude`, `longitude`, `precision`, `note`)

NEU:
> `location` (vollständig: `id`, `name`, `address` als Objekt aus `street`, `postalCode` und `city`, `latitude`, `longitude`, `precision`, `note`)

**Begründung:** Legt die API-Form aus 4.3 fest. Zur `id` siehe den offenen Punkt in Abschnitt 5.

### 4.6 Epics: Story 3.1, Import-Schema und Fehler

Neues Kriterium hinter dem Kriterium zu `importLocation`:

> **Und** `address` ist dasselbe Objekt wie in der Leseform (`street`, `postalCode`, `city`), per `$ref` aus `openapi.yaml` eingebunden (AD-9)

Die Aufzählung der fehlerhaften Einträge bekommt am Ende:

> …, ein neuer Ort mit unvollständiger Adresse (Straße, PLZ oder Ort fehlt) oder ungültiger PLZ

**Begründung:** Die Adresse sieht im Import genauso aus wie in der API (AD-9, AD-14). Im Import gelten dieselben Regeln wie im Admin.

### 4.7 Epics: Story 3.4, Testsammlung überführen

Neues Kriterium hinter dem Kriterium zu den Orten:

> **Und** jede Adresse ist in `street`, `postalCode` und `city` aufgeteilt; der Zusatz „, Germany“ entfällt; Erläuterungen in Klammern (z. B. „Ortsteil-Zentrum, kein exakter Festplatz bekannt“) wandern in die Notiz des Orts

Die Liste der Fälle zur Prüfung wird erweitert:

> … darunter alle Orte, deren Straße nicht in der Quelle steht (Ortsteile Lind, Wintersdorf, Weinzierlein, Weiherhof: Straße der Ortsmitte; Festmeile: Bereichsangabe) und Ortsteil-Angaben im Feld „Ort“ (z. B. „Zirndorf-Lind“ oder „Zirndorf“)

**Begründung:** Die Straße, die in der Quelle fehlt, wird sichtbar gewählt statt heimlich ergänzt (SM-C1).

### 4.8 `epic-1-context.md`

- In der Story-Liste kommt hinter Story 1.4 dazu: `Story 1.12: Adresse in Straße, PLZ und Ort aufteilen (läuft vor 1.5)`.
- Der Eintrag „Ort“ bekommt dazu: Die Adresse besteht aus Straße (mit Hausnummer), PLZ (genau fünf Ziffern) und Ort, alle drei Pflicht, auch bei `district`/`area`. Die Spalte `address` gibt es nur bis zum Contract-Schritt in Story 1.7.

### 4.9 `sprint-status.yaml`

Direkt hinter `1-4-orte-anlegen-bearbeiten-und-auflisten: done` kommt `1-12-adresse-in-straße-plz-und-ort-aufteilen: backlog`, außerdem wird `last_updated` aktualisiert. Die Nummer bleibt stabil, und durch die Position ist die Story als nächste dran.

## 5. Übergabe an die Umsetzung

**Umfang: Minor.** Der Developer-Agent setzt die Änderung direkt um.

| Rolle | Aufgabe |
| --- | --- |
| Developer-Agent (Correct Course) | Die Änderungen 4.1 bis 4.9 in die Planungsartefakte übernehmen, als Pull Request, nicht direkt auf `main`. |
| Developer-Agent (`bmad-build`) | Story 1.12 test-first umsetzen, mit 100 % Abdeckung für `internal/core` und `internal/adapter/admin`. |
| Andreas | Vor dem Deploy `pg_dump` ziehen, danach die Testorte nachpflegen oder löschen. Die Prüffälle aus Story 3.4 freigeben. |

**Erfolgskriterien**

- Ein Ort lässt sich im Admin nur mit Straße, fünfstelliger PLZ und Ort speichern. Bei Fehlern zeigt das Formular je Feld eine deutsche Meldung.
- Migration `00003` läuft lokal und auf Railway durch, und der alte Code bleibt mit dem neuen Schema lauffähig.
- `go test ./...`, das Abdeckungs-Gate und golangci-lint sind grün.
- PRD, Epics, Epic-Kontext und Sprint-Status sind konsistent.

**Offener Punkt, nicht Teil dieses Proposals:** Die öffentliche API soll **keine internen UUIDs** ausgeben, weder für Events noch für Orte (UUIDv7 enthält den Zeitpunkt, zu dem der Datensatz angelegt wurde). Betroffen sind AD-11, FR-10, FR-11, UJ-2 und die Stories 2.1 bis 2.4. Das klärt ein eigener Correct Course, der **vor Story 2.1** abgeschlossen sein muss.
