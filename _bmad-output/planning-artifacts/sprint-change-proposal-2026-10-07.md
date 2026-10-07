---
title: "Sprint Change Proposal: Lastgrenzen gegen Überlastung und Missbrauch"
status: approved
created: 2026-10-07
author: Andreas (mit Architect-Agent Winston, bmad-correct-course)
scope: moderate
---

# Sprint Change Proposal: Lastgrenzen gegen Überlastung und Missbrauch

## 1. Problem

**Auslöser:** Security-Bewertung vom 2026-10-07 (Winston, Architect-Agent): Wie lässt sich der Dienst durch DDoS oder auf anderem Weg lahmlegen oder zum Absturz bringen? Andreas hat am 2026-10-07 entschieden, die Punkte B, C und D als kleine Stories umzusetzen.

**Was Railway schon abdeckt:** Seit Februar 2026 läuft der gesamte öffentliche Traffic über Fastly. Das filtert volumetrische Angriffe auf Layer 3/4 ohne Konfiguration. Railway begrenzt außerdem pro Domain auf etwa 11.000 RPS und 10.000 Verbindungen. Auf Layer 7 schützt Railway nach eigener Aussage nicht immer ausreichend. Rate Limiting und Caching bietet Railway nicht. Der „Under Attack Mode“ der WAF zeigt jedem neuen Besucher eine Browser-Prüfung und sperrt damit Apps und Skripte aus, die `/v1` lesen. Für die öffentliche API taugt er deshalb nicht. Quellen: [Changelog 2026-02-20](https://railway.com/changelog/2026-02-20-domains), [Specs & Limits](https://docs.railway.com/networking/public-networking/specs-and-limits), [WAF](https://docs.railway.com/networking/waf).

**Probleme im Code** (je eines pro Story):

- **C, unbegrenzter Stau:** `WriteTimeout` schließt zwar die Verbindung, beendet aber nicht den Kontext des Handlers (`cmd/eventstore/main.go:36-41`). Der pgx-Pool läuft mit Standardwerten (`internal/adapter/postgres/postgres.go:24-35`). Antwortet die Datenbank langsam, warten beliebig viele Requests auf eine Verbindung, und der Speicher wächst bis zum OOM. Dafür braucht es keinen Angreifer, ein kurzer Hänger der Datenbank reicht.
- **D, Login als CPU-Hebel:** Jeder Anmeldeversuch kostet einen bcrypt-Vergleich mit Kosten ≥ 12 (etwa 250 ms CPU). Wie viele davon gleichzeitig laufen, ist nicht begrenzt. Die Sperre zählt pro Adresse (`internal/adapter/admin/lockout.go`). Wer viele Adressen hat, z. B. ein IPv6-/64-Netz mit beliebig vielen davon, bekommt pro Adresse 5 Versuche und bindet damit die CPU des ganzen Dienstes, auch die der öffentlichen API. Die Sperr-Map wächst dabei mit, und `pruneExpired` durchläuft sie bei jedem Versuch unter einem globalen Mutex.
- **B, Bandbreite und fehlende Cache-Header:** `/v1/docs/redoc.standalone.js` ist 1,1 MB groß und wird bei jedem Request unkomprimiert und ohne Cache-Header ausgeliefert. Railway berechnet ausgehenden Traffic. Kein Client und kein späterer CDN darf `/v1` zwischenspeichern.

**Bewusst nicht Teil dieses Vorschlags:** Das Archiv wächst ohne Grenze. `/v1/archive/events` ohne `from` lädt bei jedem Aufruf alle vergangenen Events. Bei ein paar hundert Events ist das unkritisch. Es kommt als Eintrag in `deferred-work.md` (V9).

**Kategorie:** Neue Anforderung aus einer Security-Bewertung. Sie korrigiert die Annahme im Spine, Lastgrenzen seien „bei Hobby-Last nicht nötig“.

## 2. Auswirkungen

| # | Punkt | Status | Ergebnis |
| --- | --- | --- | --- |
| 1.1–1.3 | Auslöser und Belege | [x] | Security-Bewertung 2026-10-07, Code-Stellen oben, Railway-Doku |
| 2.1 | Aktuelles Epic | [N/A] | Epics 1–3 sind fertig; kein laufendes Epic |
| 2.2 | Epic-Änderung | [!] | Neues Epic 4 mit drei Stories (Andreas, 2026-10-07) |
| 2.3–2.5 | Weitere Epics, Reihenfolge | [x] | Epics 1–3 bleiben `done`. Die Stories in Epic 4 sind unabhängig; empfohlen ist 4.1 vor 4.2 vor 4.3 |
| 3.1 | PRD | [!] | NFR-5 bekommt eine Untergrenze: Best Effort heißt nicht schutzlos |
| 3.2 | Architektur | [!] | Neuer AD-18 „Lastgrenzen“, AD-12 „Anmeldeschutz“ ergänzt, Deferred-Eintrag „Rate Limiting / Caching“ neu gefasst, Capability-Map |
| 3.3 | UX | [N/A] | Kein UX-Dokument. Die Login-Seite bekommt eine weitere deutsche Meldung |
| 3.4 | Sonstige Artefakte | [x] | `epics.md` (ENT-7, neue ENT-25, Out of Scope, Coverage Map, Epic 4), `sprint-status.yaml`, `deferred-work.md`. `openapi.yaml` und README ändern die Stories |
| 4.1 | Direkte Anpassung | [x] Viable | Aufwand gering, Risiko gering |
| 4.2 | Rollback | [x] Not viable | Nichts zurückzunehmen |
| 4.3 | MVP-Review | [x] Not viable | MVP ist erreicht und bleibt unberührt |

**Technische Auswirkung:**

- **4.1:** Middleware in `cmd/eventstore`, die jedem Request eine Deadline gibt. Eine Pool-Obergrenze in `postgres.Connect`. Eine gemeinsame Antwort `503` in `openapi.yaml`, `api.gen.go` neu erzeugen.
- **4.2:** Nur `adapter/admin`: Semaphore vor der Sperre, Sperrschlüssel aus der Adresse. README-Hinweis zu vorgeschalteten Proxys.
- **4.3:** Nur `adapter/publicapi/v1`: `Cache-Control` auf allen Antworten, gzip für die statischen Dateien. Ein Satz zum Caching in `info` der Spec.
- **Keine Migration, kein Kern-Code.** AD-1 und AD-2 bleiben unberührt.

## 3. Empfohlener Weg

Direkte Anpassung von PRD, Spine und `epics.md` mit einem neuen Epic 4 aus drei kleinen Stories. Aufwand gering, Risiko gering: Jede Story berührt einen Adapter oder die Verdrahtung, keine den Kern.

**Erwogen und verworfen:**

- **Rate Limiting pro IP in der App:** Das ist die wirksamste Maßnahme gegen eine Request-Flut, aber mit Zustand im Speicher, Schlüsselwahl und Schwellen auch die aufwendigste. Mit 4.1 kann eine Flut den Dienst verlangsamen, aber nicht mehr zum Absturz bringen. Das genügt für NFR-5. Bleibt im Spine unter Deferred.
- **`statement_timeout` in der Datenbank:** Würde auch Migrationen und die Neuberechnung beim Start begrenzen, die über denselben Pool laufen. Die Deadline aus 4.1 bricht Abfragen eines Requests ohnehin ab.
- **Cloudflare vor Railway:** Wirksam, braucht aber eine eigene Domain (Spine-Deferred „Domain und TLS“). Die Login-Sperre müsste dann `CF-Connecting-IP` lesen. Den Netzwerkschutz liefert Railway schon. Wiedervorlage mit der eigenen Domain.
- **Obergrenze für die Sperr-Map:** Unnötig, wenn die Semaphore vor der Sperre greift. Dann legt nur ein Versuch, der bcrypt wirklich erreicht, einen Eintrag an. Bei 2 parallelen Vergleichen à etwa 250 ms sind das höchstens etwa 8 neue Einträge pro Sekunde, also einige tausend in 15 Minuten.

## 4. Änderungen

### PRD (`prds/prd-oz-zirndorf-event-store-2026-10-01/prd.md`)

#### V1 – NFR-5

ALT:
> - **NFR-5 Betrieb auf Hobby-Niveau:** Für Antwortzeit und Verfügbarkeit gibt es keine Zielwerte. Es gilt Best Effort.

NEU:
> - **NFR-5 Betrieb auf Hobby-Niveau:** Für Antwortzeit und Verfügbarkeit gibt es keine Zielwerte. Es gilt Best Effort. Best Effort heißt nicht schutzlos: Ein einzelner Client darf den Dienst nicht mit wenig Aufwand zum Absturz bringen, weder über viele oder teure Anfragen noch über die Anmeldung. Eine langsame Datenbank darf ihn ebenfalls nicht zum Absturz bringen. Schutz gegen volumetrische Angriffe liefert das Hosting.

Begründung: Gibt den Stories 4.1 bis 4.3 eine Anforderung, ohne Zielwerte einzuführen.

### Architektur (`ARCHITECTURE-SPINE.md`)

#### V2 – Frontmatter

`updated: '2026-10-06'` → `updated: '2026-10-07'`

#### V3 – Neuer AD-18 (nach AD-17)

NEU:
> ### AD-18 — Lastgrenzen [ADOPTED]
>
> - **Binds:** NFR-5, FR-14, NFR-1
> - **Prevents:** Eine Request-Flut oder eine langsame Datenbank staut Requests ohne Grenze bis zum Speicherabbruch; Anmeldeversuche von vielen Adressen binden die CPU des ganzen Dienstes; große statische Dateien treiben die Egress-Kosten.
> - **Rule:** Jeder Request hat eine Deadline (`requestTimeout`), die unter `WriteTimeout` liegt. Der Kontext jedes Requests endet spätestens dann, und mit ihm jede Datenbankabfrage und jedes Warten auf eine Pool-Verbindung. Die öffentliche API antwortet dann mit `503` als Problem Details (KON-3). Der Datenbank-Pool hat eine feste Obergrenze an Verbindungen (`MaxConns`), unabhängig von der CPU-Zahl des Hosts. Gleichzeitig laufen höchstens zwei bcrypt-Vergleiche. Ist kein Platz frei, wird ein Anmeldeversuch sofort abgelehnt, ohne als Fehlversuch zu zählen. Antworten unter `/v1/…` tragen `Cache-Control`. Die statischen Dateien liefert der Dienst gzip-komprimiert aus, wenn der Client es annimmt. Volumetrische Angriffe (Layer 3/4) wehrt Railways Edge ab, nicht die App. Railways „Under Attack Mode“ ist für `/v1` ungeeignet, weil er Clients ohne Browser aussperrt. Rate Limiting pro Client gibt es nicht (Deferred).
>   - **Werte:** `requestTimeout` 20 s (`WriteTimeout` 30 s), `MaxConns` 10, zwei gleichzeitige bcrypt-Vergleiche. Event-Listen und Event-Typen `public, max-age=60`. Spec, Import-Schema und Docs-Seite `public, max-age=300`. Redoc-Skript `public, max-age=86400`. Fehlerantworten `no-store`. Die Werte sind benannte Konstanten im jeweiligen Adapter bzw. in `cmd/eventstore`.

Begründung: Lastgrenzen sind ab jetzt eine Regel, die jeder neue Endpunkt einhalten muss, keine Story-Entscheidung.

#### V4 – AD-12, Anmeldeschutz

ALT:
> - **Anmeldeschutz:** Nach 5 Fehlversuchen von einer Client-IP ist die Anmeldung von dieser IP für 15 Minuten gesperrt. Die Zähler liegen im Speicher, ein Neustart setzt sie zurück. Als Client-IP gilt … `X-Real-IP` wird nicht genutzt. Der eigentliche Schutz ist bcrypt mit Kosten ≥ 12 und einem langen Zufallspasswort.

NEU:
> - **Anmeldeschutz:** Nach 5 Fehlversuchen von einer Client-Adresse ist die Anmeldung von dieser Adresse für 15 Minuten gesperrt. Gezählt wird pro IPv4-Adresse bzw. pro IPv6-/64-Netz, weil ein einzelner Anschluss ein ganzes /64 nutzen kann. Die Zähler liegen im Speicher, ein Neustart setzt sie zurück. Als Client-IP gilt … `X-Real-IP` wird nicht genutzt. Steht ein weiterer Proxy (z. B. Cloudflare) vor Railway, ist der linke Eintrag nicht mehr der Client; dann muss die Ermittlung der Client-IP angepasst werden. Der eigentliche Schutz ist bcrypt mit Kosten ≥ 12 und einem langen Zufallspasswort. Wie viele Vergleiche gleichzeitig laufen, begrenzt AD-18.

(Der mit „…“ ausgelassene Text bleibt unverändert.)

#### V5 – Capability → Architecture Map, Zeile NFR-5

ALT:
> | NFR-5, NFR-6 Betrieb, Zeitzone | `cmd/eventstore`, Railway | AD-13, AD-16, AD-17, Structural Seed |

NEU:
> | NFR-5, NFR-6 Betrieb, Lastgrenzen, Zeitzone | `cmd/eventstore`, `adapter/publicapi/v1`, `adapter/admin`, Railway | AD-13, AD-16, AD-17, AD-18, Structural Seed |

#### V6 – Deferred, „Rate Limiting / Caching“

ALT:
> - **Rate Limiting / Caching:** Bei Hobby-Last nicht nötig (NFR-5). Nachrüstbar als Middleware in `adapter/publicapi`, ohne den Kern zu berühren.

NEU:
> - **Rate Limiting pro Client:** Mit den Grenzen aus AD-18 kann eine Request-Flut den Dienst verlangsamen, aber nicht zum Absturz bringen. Das genügt für NFR-5. Nachrüstbar als Middleware in `adapter/publicapi`, ohne den Kern zu berühren. Wiedervorlage, wenn eine Flut den Dienst für echte Abnehmer unbrauchbar macht, oder mit der eigenen Domain (dann auch Cloudflare als Proxy prüfen, siehe AD-12).
> - **Grenze für das Archiv:** `/v1/archive/events` ohne `from` liefert das ganze Archiv. Bei einigen hundert Events ist das unkritisch. Wiedervorlage, wenn das Archiv einige tausend Events hat (Standardzeitraum, Höchstspanne oder Paginierung, siehe `deferred-work.md`).

### Epics (`epics.md`)

#### V7a – Requirements Inventory

NFR-5 ALT:
> NFR-5: Betrieb auf Hobby-Niveau: keine Zielwerte für Antwortzeit und Verfügbarkeit, Best Effort.

NFR-5 NEU:
> NFR-5: Betrieb auf Hobby-Niveau: keine Zielwerte für Antwortzeit und Verfügbarkeit, Best Effort. Ein einzelner Client und eine langsame Datenbank dürfen den Dienst aber nicht mit wenig Aufwand zum Absturz bringen; gegen volumetrische Angriffe schützt das Hosting.

Einleitung „Ergänzende Festlegungen“, letzter Satz ALT:
> … ENT-15 bis ENT-23 kamen aus dem Review dieses Updates hinzu, ENT-24 mit dem Sprint Change Proposal vom 2026-10-05.

NEU:
> … ENT-15 bis ENT-23 kamen aus dem Review dieses Updates hinzu, ENT-24 mit dem Sprint Change Proposal vom 2026-10-05, ENT-25 mit dem vom 2026-10-07.

Ebenso im Overview: „ENT-24 seit dem 2026-10-05“ → „ENT-24 seit dem 2026-10-05, ENT-25 seit dem 2026-10-07“.

ENT-7 ALT:
> - ENT-7 (→ AD-12) Anmeldeschutz: Nach 5 Fehlversuchen von einer Client-IP ist die Anmeldung von dieser IP für 15 Minuten gesperrt. Die Zähler liegen im Speicher; …

ENT-7 NEU:
> - ENT-7 (→ AD-12) Anmeldeschutz: Nach 5 Fehlversuchen von einer Client-Adresse (IPv4-Adresse oder IPv6-/64-Netz) ist die Anmeldung von dort für 15 Minuten gesperrt. Die Zähler liegen im Speicher; …

Neu nach ENT-24:
> - ENT-25 (→ AD-18) Lastgrenzen: Jeder Request hat eine Deadline von 20 s; danach endet sein Kontext samt Datenbankabfrage, `/v1` antwortet mit `503` (Problem Details). Der Pool hat höchstens 10 Verbindungen. Höchstens zwei bcrypt-Vergleiche laufen gleichzeitig; ein weiterer Anmeldeversuch wird sofort abgelehnt und zählt nicht als Fehlversuch. `/v1` sendet `Cache-Control` (Listen und Event-Typen 60 s, Spec, Schema und Docs-Seite 300 s, Redoc-Skript 1 Tag, Fehler `no-store`) und liefert die statischen Dateien gzip-komprimiert aus.

Out of Scope v1 ALT:
> … eigene Domain, Staging, Rate Limiting und Caching der öffentlichen API, Monitoring über Logs hinaus, …

Out of Scope v1 NEU:
> … eigene Domain, Staging, Rate Limiting pro Client und serverseitiges Caching der öffentlichen API, Grenze für das Archiv, Monitoring über Logs hinaus, …

FR Coverage Map ALT:
> NFR-5: alle Epics, ohne eigene Story (Best Effort)

NEU:
> NFR-5: alle Epics (Best Effort), Lastgrenzen in Epic 4 (Stories 4.1 bis 4.3)

#### V7b – Epic List, neu nach Epic 3

> ### Epic 4: Der Dienst hält Überlastung und Missbrauch stand
> **FRs covered:** keine neuen; NFR-5 (Lastgrenzen), FR-14 (Anmeldeschutz), AD-18

#### V7c – Neues Epic 4 am Ende von `epics.md`

> ## Epic 4: Der Dienst hält Überlastung und Missbrauch stand
>
> Eine langsame Datenbank, eine Request-Flut oder Anmeldeversuche von vielen Adressen bringen den Dienst nicht mehr zum Absturz, und die öffentliche API verursacht keine unnötigen Egress-Kosten. Gegen volumetrische Angriffe schützt Railways Edge (AD-18).
>
> **Hinweis:** Sprint Change Proposal 2026-10-07 (Security-Bewertung). Die Stories sind voneinander unabhängig; empfohlen ist die Reihenfolge 4.1, 4.2, 4.3.
>
> ### Story 4.1: Deadline für jeden Request und begrenzter Datenbank-Pool
>
> Als Betreiber,
> möchte ich, dass jeder Request nach einer festen Zeit endet und der Dienst nur eine begrenzte Zahl an Datenbankverbindungen nutzt,
> damit eine langsame Datenbank oder eine Request-Flut den Dienst verlangsamt, aber nicht durch Speichermangel abstürzen lässt.
>
> **Deckt ab:** NFR-5, AD-18, ENT-25, KON-3
>
> **Acceptance Criteria:**
>
> **Angenommen** eine Abfrage der öffentlichen API, deren Datenbankabfrage länger als die Deadline braucht
> **Wenn** die Deadline abläuft
> **Dann** endet der Kontext des Requests, die Abfrage wird abgebrochen
> **Und** der Client bekommt `503` als Problem Details mit englischem `detail` (KON-3), vor Ablauf von `WriteTimeout`
> **Und** das Log enthält eine Warnung, keinen Error
>
> **Angenommen** alle Pool-Verbindungen sind belegt
> **Wenn** weitere Requests eine Verbindung brauchen
> **Dann** warten sie höchstens bis zu ihrer Deadline und enden dann wie oben, statt sich ohne Grenze zu stauen
>
> **Angenommen** ein Admin-Request überschreitet die Deadline
> **Wenn** sie abläuft
> **Dann** zeigt der Admin die übliche deutsche Fehlerseite, und es ist nichts halb geschrieben (die Transaktion wird zurückgerollt)
>
> **Angenommen** der Pool wird erzeugt
> **Wenn** der Dienst startet
> **Dann** hat er höchstens 10 Verbindungen, unabhängig von der CPU-Zahl und von Parametern in `DATABASE_URL`
>
> **Angenommen** die Spec
> **Wenn** ich die Operationen unter `/v1` lese
> **Dann** ist `503` als gemeinsame Antwort mit Beispiel beschrieben
>
> **Außerdem gilt:**
> - `requestTimeout` (20 s) ist eine benannte Konstante in `cmd/eventstore` und liegt unter `writeTimeout` (30 s); ein Test prüft das Verhältnis.
> - Die Middleware umschließt den ganzen Router; `/healthz` behält seinen eigenen, kürzeren Ping-Timeout.
> - Kein `statement_timeout`: Migrationen und die Neuberechnung beim Start laufen über denselben Pool und dürfen nicht begrenzt werden.
> - `api.gen.go` wird mit `go generate ./...` neu erzeugt, nicht von Hand geändert.
> - Nahtstellen: Fehlerbehandlung der öffentlichen API (2.3, B1: `context.Canceled` wird nicht als Error geloggt), Server-Timeouts (A1), Fehlerseite und Transaktionen im Admin (1.7, ENT-15), Startreihenfolge mit Migration und Neuberechnung (2.5).
>
> ### Story 4.2: Anmeldung gegen CPU-Last von vielen Adressen schützen
>
> Als Betreiber,
> möchte ich, dass Anmeldeversuche nur begrenzt CPU binden und eine IPv6-Adresse nicht beliebig viele Versuche erzeugt,
> damit Login-Bots weder die öffentliche API ausbremsen noch die Sperre umgehen.
>
> **Deckt ab:** FR-14, NFR-5, AD-12, AD-18, ENT-7, ENT-25
>
> **Acceptance Criteria:**
>
> **Angenommen** zwei Anmeldeversuche prüfen gerade ihr Passwort
> **Wenn** ein dritter Versuch eintrifft
> **Dann** wird er sofort mit `503` und einer deutschen Meldung („Gerade laufen zu viele Anmeldeversuche. Bitte versuch es gleich noch einmal.“) abgelehnt, ohne bcrypt
> **Und** er zählt nicht als Fehlversuch und legt keinen Eintrag in der Sperre an
>
> **Angenommen** 5 Fehlversuche von Adressen aus demselben IPv6-/64-Netz
> **Wenn** ein weiterer Versuch von einer anderen Adresse aus diesem Netz kommt
> **Dann** ist er für 15 Minuten gesperrt wie bei einer einzelnen Adresse (Story 1.3)
> **Und** eine Adresse aus einem anderen /64 ist davon nicht betroffen
>
> **Angenommen** IPv4-Adressen
> **Wenn** gezählt wird
> **Dann** zählt jede Adresse für sich, wie bisher
>
> **Angenommen** der Header oder `RemoteAddr` enthält keine gültige IP
> **Wenn** gezählt wird
> **Dann** zählt der Wert unverändert als Schlüssel, wie bisher
>
> **Außerdem gilt:**
> - Die Semaphore greift vor der Sperre, damit nur Versuche, die bcrypt erreichen, einen Eintrag anlegen; das hält die Sperr-Map klein, eine eigene Obergrenze ist nicht nötig.
> - Erfolgreiche Anmeldung, Sperre nach 5 Fehlversuchen und konstante Laufzeit bleiben wie in Story 1.3.
> - Die README nennt, dass die Client-IP aus dem linken Eintrag von `X-Forwarded-For` nur hinter Railways Edge stimmt und ein zusätzlicher Proxy (z. B. Cloudflare) eine Anpassung braucht.
> - Nahtstellen: `submitLogin`, `loginLockout` und `clientIP` (1.3, ENT-7, ENT-21).
>
> ### Story 4.3: Cache-Header und komprimierte statische Dateien in der öffentlichen API
>
> Als Abnehmer der öffentlichen API,
> möchte ich wissen, wie lange ich eine Antwort zwischenspeichern darf, und die Dokumentation komprimiert laden,
> damit wiederholte Abrufe und die Docs-Seite weniger Traffic verursachen.
>
> **Deckt ab:** NFR-3, NFR-5, AD-18, ENT-14, ENT-23, ENT-25
>
> **Acceptance Criteria:**
>
> **Angenommen** eine erfolgreiche Antwort von `/v1/events`, `/v1/archive/events` oder `/v1/event-types`
> **Wenn** ich ihre Header lese
> **Dann** steht dort `Cache-Control: public, max-age=60`
>
> **Angenommen** `/v1/openapi.yaml`, `/v1/import-v1.schema.json` oder `/v1/docs`
> **Wenn** ich sie abrufe
> **Dann** steht dort `Cache-Control: public, max-age=300`
> **Und** für `/v1/docs/redoc.standalone.js` `Cache-Control: public, max-age=86400`
>
> **Angenommen** eine Fehlerantwort unter `/v1` (400, 404, 405, 500, 503)
> **Wenn** ich ihre Header lese
> **Dann** steht dort `Cache-Control: no-store`
>
> **Angenommen** ein Client sendet `Accept-Encoding: gzip`
> **Wenn** er das Redoc-Skript, die Spec oder das Import-Schema abruft
> **Dann** bekommt er sie mit `Content-Encoding: gzip` und `Vary: Accept-Encoding`, und das Redoc-Skript ist deutlich kleiner als 1,1 MB
> **Und** ohne `Accept-Encoding: gzip` bekommt er dieselbe Datei unkomprimiert
>
> **Angenommen** die Spec
> **Wenn** ich `info` lese
> **Dann** steht dort, dass Listen bis zu 60 Sekunden zwischengespeichert werden dürfen und ein Event deshalb bis zu einer Minute nach seinem Ende noch in `/v1/events` stehen kann; maßgeblich bleibt `effectiveEnd`
>
> **Außerdem gilt:**
> - Komprimiert wird einmal beim Start aus den eingebetteten Dateien, nicht pro Request; die Event-Listen werden nicht komprimiert.
> - Preflight-Antworten (`OPTIONS`) behalten `Access-Control-Max-Age` und bekommen kein `Cache-Control`.
> - CORS-Header bleiben auf jeder Antwort (NFR-1).
> - `api.gen.go` wird mit `go generate ./...` neu erzeugt, falls sich die Spec ändert.
> - Nahtstellen: `serveEmbedded`, `serveStaticFile`, `readOnlyCORS` und die Problem-Antworten (2.1, 2.3, 2.6, ENT-23).

### Sprint-Status (`implementation-artifacts/sprint-status.yaml`)

#### V8

Neu nach `epic-3-retrospective: done`:

```yaml
  epic-4: backlog
  4-1-deadline-für-jeden-request-und-begrenzter-datenbank-pool: backlog
  4-2-anmeldung-gegen-cpu-last-von-vielen-adressen-schützen: backlog
  4-3-cache-header-und-komprimierte-statische-dateien: backlog
  epic-4-retrospective: optional
```

### Deferred Work (`implementation-artifacts/deferred-work.md`)

#### V9 – Neuer Eintrag am Ende

```yaml
- source_spec: `_bmad-output/planning-artifacts/sprint-change-proposal-2026-10-07.md`
  summary: Grenze für `/v1/archive/events` (Standardzeitraum, Höchstspanne oder Paginierung), `type`-Filter in SQL und Orte nur für die Treffer laden.
  evidence: Ohne `from` lädt jeder Aufruf alle vergangenen Events samt Ablaufplänen und allen Orten, sortiert im Speicher und baut das JSON am Stück (`internal/core/event_query.go:156-187`). Bei einigen hundert Events unkritisch; eine Änderung des Standardzeitraums ist eine Vertragsänderung (NFR-2).
  status: open
  target: Wiedervorlage bei einigen tausend archivierten Events (Spine, Deferred „Grenze für das Archiv“)
```

## 5. Übergabe

**Umfang:** Moderat. Neues Epic im Backlog, Spine bekommt einen neuen Architekturentscheid. Die Umsetzung macht der Developer-Agent direkt, eine Neuplanung ist nicht nötig.

| Wer | Aufgabe |
| --- | --- |
| Architect-Agent (dieser Lauf) | V1 bis V9 in die Planungsartefakte übernehmen, Commit auf einem eigenen Branch, PR |
| Andreas | PR prüfen und mergen |
| Developer-Agent (`bmad-build`) | Stories 4.1, 4.2, 4.3 test-first umsetzen, je ein PR |

**Erfolgskriterien:**

- PRD (NFR-5), Spine (AD-18, AD-12, Deferred) und `epics.md` (ENT-7, ENT-25, Epic 4) sind konsistent. `sprint-status.yaml` führt Epic 4.
- Nach 4.1: Ein Test mit einer künstlich langsamen Abfrage und kurzer Deadline ergibt `503` vor `WriteTimeout`. Der Pool meldet `MaxConns` 10.
- Nach 4.2: Ein Test mit drei gleichzeitigen Versuchen und einem blockierenden Passwortvergleich lehnt den dritten ohne bcrypt ab. IPv6-Adressen aus einem /64 teilen sich einen Zähler.
- Nach 4.3: Alle `/v1`-Antworten tragen den passenden `Cache-Control`-Header. Das Redoc-Skript kommt mit `gzip` deutlich kleiner als 1,1 MB.
- Die CI ist grün: Abdeckung 100 % in den Pflichtpaketen, Lint, Generator-Diff, Postgres-Tests.
