# OZ Zirndorf Event Store

Der **OZ Zirndorf Event Store** sammelt Veranstaltungen in Zirndorf, also Kirchweihen und Feste, Märkte, Vorstellungen in der Paul-Metz-Halle, Vereinstreffen und Stadtratssitzungen, und stellt sie über eine öffentliche REST-API bereit. Erster Abnehmer ist die Karten-App von [OpenZirndorf](#über-openzirndorf), die Events als eigene Ebene zeigt.

> **Status:** Umsetzung läuft. PRD und Architektur sind fertig (siehe [Dokumentation](#dokumentation)). Es gibt das Grundgerüst mit Datenbank, Migrationen, Health Check und CI sowie die Auslieferung auf Railway, aber noch keine Fachfunktionen.

## Worum es geht: ehrliche Angaben

Viele Zirndorfer Termine sind unscharf. Mal fehlt die Uhrzeit, mal ist das Ende offen, bei einem Fest ist manchmal nur der Ortsteil bekannt. Der Event Store versteckt das nicht hinter Platzhaltern wie `00:00`. Er speichert ausdrücklich, **wie genau** Zeit und Ort sind und **woher** ein Event stammt. So setzt eine Karte keine Pins, die genauer wirken, als sie sind.

- **Zeit:** Datum, optionale Uhrzeit und „ganztägig“. Eine leere Uhrzeit heißt „unbekannt“, nie Mitternacht.
- **Ort:** Jeder Ort hat eine Ortsgenauigkeit (`building`, `street`, `area`, `district`).
- **Quelle:** Jedes Event nennt seine Quelle.

## Für OZ-Mitglieder: das Muster zum Übernehmen

Für OZ-Backends gibt es noch keine gemeinsamen Schnittstellenregeln. Dieses Projekt lebt ein Muster vor. Das ist **ein Angebot, keine Vorgabe**. Nimm dir, was für dein Backend passt.

| Konvention | Umsetzung |
| --- | --- |
| Versionierung | Hauptversion im Pfad (`/v1/…`). Nach einer neuen Version läuft die alte mindestens 6 Monate weiter, das Abschaltdatum steht in der Doku und im `Sunset`-Header. |
| Vertrag | **Spec-first:** `api/v1/openapi.yaml` (OpenAPI 3.1) ist die einzige Quelle, der Server-Code wird daraus generiert. Die Spec ist öffentlich unter `/v1/openapi.yaml` abrufbar. |
| Fehler | [RFC 9457 Problem Details](https://www.rfc-editor.org/rfc/rfc9457) (`application/problem+json`), Texte auf Englisch |
| Zeiten | Datum `YYYY-MM-DD` und getrennte Uhrzeit `HH:MM` (lokal Europe/Berlin, `null` = unbekannt). Berechnete Zeitpunkte als ISO 8601 mit Offset. |
| Zeitraumfilter | `from` / `to`, beide inklusive. Ein Event ist enthalten, wenn sich sein Zeitraum mit dem Filter überschneidet. Fehlt `to`, ist der Zeitraum nach hinten offen. |
| Listen | Immer eine Hülle `{ "data": [ … ] }`, nie ein nacktes Array. So lassen sich später Metadaten ohne Bruch ergänzen. |
| Namen | Alles Technische englisch, Feldnamen in camelCase (`startDate`, `locationId`). Inhalte bleiben deutsch. |
| CORS | Die öffentliche API ist offen für alle Herkünfte. Die Admin-Oberfläche ist davon getrennt und geschützt. |

## API im Überblick

Die API ist öffentlich, ohne Anmeldung nutzbar und **nur lesend**. Die geplanten Ressourcen:

| Ressource | Zweck |
| --- | --- |
| `GET /v1/events` | Aktive Events. Ohne Filter: alles, was heute noch stattfindet. Filter: `from`, `to`, `type` |
| `GET /v1/events/{id}` | Einzelnes Event, auch archivierte |
| `GET /v1/archive/events` | Vergangene Events, gleiche Filter |
| `GET /v1/locations` | Alle Orte. Events am selben Ort haben dieselbe Ort-ID und eignen sich so zum Gruppieren auf der Karte. |
| `GET /v1/event-types` | Liste der Event-Typen |

Event-Typen: `festival`, `market`, `culture`, `politics`, `club`, `sports`, `other`.

Ein Event, so wie es geplant ist (gekürzt):

```json
{
  "data": [
    {
      "id": "0192f0c4-…",
      "title": "Zirndorfer Weihnachtsmarkt",
      "type": "market",
      "startDate": "2026-11-27",
      "startTime": null,
      "endDate": "2026-12-21",
      "endTime": null,
      "allDay": false,
      "startPrecision": "dateOnly",
      "endPrecision": "dateOnly",
      "effectiveStart": "2026-11-27T00:00:00+01:00",
      "effectiveEnd": "2026-12-22T00:00:00+01:00",
      "archived": false,
      "location": {
        "id": "0192f0b1-…",
        "name": "Marktplatz",
        "address": "Marktplatz, 90513 Zirndorf",
        "latitude": 49.4425,
        "longitude": 10.9547,
        "precision": "street"
      },
      "source": { "description": "Stadt Zirndorf, Veranstaltungskalender", "url": "https://…" }
    }
  ]
}
```

*Die Beispieldaten sind erfunden. Verbindlich ist die OpenAPI-Spec, sobald sie existiert.*

## Architektur

**Hexagonal light (Ports & Adapters).** Die Fachregeln (wann ein Event vorbei ist, was ein Duplikat ist, wie genau eine Zeitangabe ist) liegen in einem Kern ohne Abhängigkeiten. HTTP, Datenbank und der tägliche Bereinigungsjob sind Adapter darum herum. In SQL steht keine Fachlogik.

```mermaid
flowchart LR
  publicapi["Public API /v1<br/>(nur lesend, CORS offen)"] --> core
  admin["Admin /admin<br/>(Login, Pflege, JSON-Import)"] --> core
  cleanup["Bereinigungsjob<br/>(täglich)"] --> core
  postgres["PostgreSQL-Adapter"] --> core
  core["Kern<br/>Regeln + Anwendungsfälle"]
```

Ein paar Entscheidungen, die auch für andere Backends interessant sein könnten:

- **Archiv ohne Umzug:** Alle Events liegen in einer Tabelle. Ob ein Event vergangen ist, ergibt sich allein aus seinem berechneten Ende. Der tägliche Job markiert nur, er verschiebt nichts. Fällt er aus, merkt die API davon nichts.
- **Zeitrechnung an einer Stelle:** Eine einzige Funktion im Kern rechnet lokale Zeiten in Zeitpunkte um, einschließlich der Zeitumstellung. Filter, „heute“ und Archiv nutzen nur die vorberechneten Zeitpunkte.
- **Zustandsloser Import:** JSON hochladen, Vorschau durcharbeiten (neu, Aktualisierung, Duplikatverdacht, Fehler), speichern. Beim Speichern wird erneut gegen den aktuellen Bestand geprüft, dann alles in einer Transaktion geschrieben.

Alle 17 Entscheidungen mit Regeln stehen im [Architecture Spine](_bmad-output/planning-artifacts/architecture/architecture-oz-zirndorf-event-store-2026-10-01/ARCHITECTURE-SPINE.md).

## Technik

| Bereich | Wahl |
| --- | --- |
| Sprache | Go 1.27 |
| Datenbank | PostgreSQL 18 |
| Datenbankzugriff | sqlc + pgx v5, Migrationen mit goose |
| API-Code | oapi-codegen (aus OpenAPI generiert) |
| Admin-Oberfläche | serverseitiges HTML (`html/template`) + htmx 2, Kartenpicker mit Leaflet und OpenStreetMap |
| Hosting | [Railway](https://railway.com): ein Service für die App, einer für PostgreSQL |
| CI | GitHub Actions. Railway deployt erst, wenn die CI grün ist. |

## Projektstruktur

```text
api/v1/              OpenAPI-Spec und Import-Schema
cmd/eventstore/      Programmstart und Verdrahtung
internal/core/       Kern: Regeln, Anwendungsfälle, Ports
internal/adapter/    publicapi/v1, admin, postgres, cleanup
```

## Lokal starten

### Voraussetzungen

- Go 1.27.1. Mit `GOTOOLCHAIN=auto` (Standard) lädt eine ältere Go-1.27-Installation die passende Version selbst nach.
- Docker mit Docker Compose für das lokale PostgreSQL 18.

### Datenbank starten

```sh
docker compose up -d
```

`compose.yaml` startet PostgreSQL 18 auf Port 5432 mit Benutzer, Passwort und Datenbank `eventstore`. Die Daten liegen im Volume `pgdata` unter `/var/lib/postgresql`. Die Zugangsdaten sind nur für die lokale Entwicklung gedacht.

### Umgebungsvariablen

| Variable | Pflicht | Beispiel |
| --- | --- | --- |
| `DATABASE_URL` | ja | `postgres://eventstore:eventstore@localhost:5432/eventstore?sslmode=disable` |
| `PORT` | ja | `8080` (1 bis 65535) |

Fehlt eine Variable oder ist `PORT` ungültig, bricht der Start mit einer JSON-Logzeile ab, die die Variable nennt.

### Programm starten

```sh
export DATABASE_URL='postgres://eventstore:eventstore@localhost:5432/eventstore?sslmode=disable'
export PORT=8080
go run ./cmd/eventstore
```

Beim Start laufen zuerst die eingebetteten goose-Migrationen, erst danach nimmt der HTTP-Server Anfragen an. Logs gehen als JSON auf stdout.

```sh
curl -i localhost:8080/healthz   # 200, solange die Datenbank erreichbar ist, sonst 503
```

### Tests und Prüfungen

```sh
go build ./...
go vet ./...
go test ./...                    # Unit- und Architekturtest; Postgres-Tests werden ohne Datenbank übersprungen
bash scripts/check-coverage.sh   # 100 % Abdeckung für core, publicapi/v1 und admin
```

Die Postgres-Tests laufen gegen eine eigene Testdatenbank:

```sh
docker compose exec postgres createdb -U eventstore eventstore_test
export EVENTSTORE_TEST_DATABASE_URL='postgres://eventstore:eventstore@localhost:5432/eventstore_test?sslmode=disable'
go test -p 1 ./internal/adapter/postgres/... ./cmd/eventstore/...   # -p 1: beide Pakete migrieren dieselbe Datenbank
```

Die CI (GitHub Actions) führt bei jedem Pull Request und jedem Push auf `main` `go vet`, golangci-lint v2.14.0, Unit-, Architektur- und Postgres-Tests, die Abdeckungsprüfung sowie einen Docker-Build (ohne Push) mit Startprüfung aus. Der Architekturtest (`internal/archtest`) lässt die CI scheitern, wenn `internal/core` mehr als die Standardbibliothek und `golang.org/x/text/unicode/norm` importiert oder ein Adapter einen anderen Adapter importiert.

## Deployment auf Railway

Die App läuft auf [Railway](https://railway.com) als ein Service aus diesem Repository, daneben ein PostgreSQL-18-Service, der nur im privaten Netz erreichbar ist. Gebaut wird das `Dockerfile` (Multi-Stage: statisches Go-Binary auf `gcr.io/distroless/static-debian13:nonroot`, ohne Shell, als Nicht-root). Die Deploy-Einstellungen stehen als Code in `railway.json`: Dockerfile-Builder, Health Check auf `/healthz`, genau eine Replika, Neustart bei Absturz.

### Einmalige Einrichtung (Railway-Dashboard)

1. Neues Projekt anlegen, Umgebung `production`.
2. PostgreSQL-Service hinzufügen und das Image auf Major 18 pinnen: unter *Settings → Source* `ghcr.io/railwayapp-templates/postgres-ssl:18` eintragen, nie `:latest` (das ist 16).
3. Am PostgreSQL-Service unter *Settings → Networking* keinen TCP-Proxy einrichten bzw. einen vorhandenen entfernen. Die Datenbank bleibt so ohne öffentliche Verbindung.
4. App-Service aus GitHub hinzufügen (dieses Repository, Branch `main`). Railway erkennt `railway.json` und baut das `Dockerfile`.
5. Am App-Service die Variable `DATABASE_URL=${{Postgres.DATABASE_URL}}` setzen, also die Referenz auf die private URL des Postgres-Services (Service-Name ggf. anpassen). `PORT` setzt Railway selbst.
6. Unter *Settings → Deploy* „Wait for CI“ einschalten.
7. Unter *Settings → Networking* eine Railway-Domain erzeugen.

Prüfen nach dem ersten Deploy:

```sh
curl -i https://<railway-domain>/healthz   # 200
railway ssh --service Postgres -- psql -U postgres -c 'SHOW server_version;'   # 18.x
```

Im Dashboard: genau ein aktiver Deploy, Replikas = 1, Postgres-Image-Tag 18, kein TCP-Proxy auf Postgres.

### Ablauf eines Deploys

- Jeder Push auf `main` (also jeder gemergte Pull Request) löst einen Deploy aus. Railway wartet, bis die CI grün ist; ist sie rot, wird der Deploy übersprungen und die alte Version bleibt live.
- Die CI baut das Docker-Image mit (ohne Push) und startet es einmal ohne Umgebungsvariablen: Es muss mit Exit-Code 1 und der Meldung über fehlende Variablen enden. Scheitert Build oder Startprüfung (z. B. kaputtes `ENTRYPOINT` oder nicht statisches Binary), wird die CI rot und blockiert so den Deploy. Die CI braucht kein Railway-Token.
- Railway baut das Image selbst und startet die neue Version. Beim Start laufen zuerst die Migrationen, dann der HTTP-Server. Erst wenn `/healthz` mit 200 antwortet, wird die neue Version live; scheitert der Health Check (z. B. Datenbank nicht erreichbar), bleibt die alte Version aktiv und der Fehler steht im Railway-Log.

### Migrationen und Datensicherung (AD-17)

- Vor jedem Deploy mit einer neuen Migration ein Backup mit der Railway-CLI ziehen. Weil die Datenbank keinen TCP-Proxy hat, läuft `pg_dump` (Version 18) im Postgres-Container, nicht lokal:

  ```sh
  railway link                                  # einmalig: Projekt und Umgebung production wählen
  railway ssh --service Postgres -- pg_dump -U postgres -d railway > backup-$(date +%F).sql
  ```

  `railway run pg_dump …` führt `pg_dump` dagegen lokal aus und erreicht die private Datenbank-URL nicht. Den Dump vor dem Merge kurz auf Inhalt prüfen.
- Migrationen nur vorwärts, es gibt kein Zurückrollen per Down-Migration.
- Schemaänderungen nach expand/contract: erst erweitern (neue Spalte, neue Tabelle), Code umstellen und deployen, erst in einem späteren Deploy das Alte entfernen. So passt die laufende alte Version immer zum Schema.
- Bereits angewendete Migrationen nie ändern, sondern eine neue anlegen.

### Client-IP hinter Railway

Gemessen am 2026-10-02 nach dem ersten Deploy: Railways Edge **verwirft** einen vom Client mitgeschickten `X-Forwarded-For` und setzt den Header selbst. Die Client-IP ist der **linke Eintrag**.

```sh
curl -i -H 'X-Forwarded-For: 203.0.113.7' https://<railway-domain>/healthz
```

| Anfrage | Ankommender `X-Forwarded-For` |
| --- | --- |
| ohne Header | `<Client-IP>, <Edge-IP>` |
| mit gefälschtem `203.0.113.7` | `<Client-IP>, <Edge-IP>` (der gefälschte Wert fehlt) |

- Der rechte Eintrag ist ein Railway-Edge-Knoten und wechselt von Anfrage zu Anfrage.
- `RemoteAddr` ist eine interne Railway-Adresse (`100.64.0.0/10`) und taugt nicht als Client-IP.

Story 1.3 (Admin-Anmeldung) nimmt deshalb für die Login-Sperre den linken Eintrag von `X-Forwarded-For` und nur ohne Header `RemoteAddr` ohne Port.

## Dokumentation

- [PRD](_bmad-output/planning-artifacts/prds/prd-oz-zirndorf-event-store-2026-10-01/prd.md): was der Event Store können muss
- [Architecture Spine](_bmad-output/planning-artifacts/architecture/architecture-oz-zirndorf-event-store-2026-10-01/ARCHITECTURE-SPINE.md): verbindliche Architekturentscheidungen
- [Product Brief](_bmad-output/planning-artifacts/briefs/brief-oz-zirndorf-event-store-2026-10-01/brief.md): Problem und Idee

## Über OpenZirndorf

OpenZirndorf (OZ) ist eine Open-Source-Community in Zirndorf. Jedes Mitglied baut sein Backend selbst und wählt die Technik frei. Ein gemeinsames Frontend bindet die Backends ein.

## Lizenz

Noch nicht festgelegt. Die Lizenz für Code und Daten wird vor dem öffentlichen Start entschieden.
