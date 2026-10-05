# OZ Zirndorf Event Store

Der **OZ Zirndorf Event Store** sammelt Veranstaltungen in Zirndorf, also Kirchweihen und Feste, Märkte, Vorstellungen in der Paul-Metz-Halle, Vereinstreffen und Stadtratssitzungen, und stellt sie über eine öffentliche REST-API bereit. Erster Abnehmer ist die Karten-App von [OpenZirndorf](#über-openzirndorf), die Events als eigene Ebene zeigt.

> **Status:** Umsetzung läuft. PRD und Architektur sind fertig (siehe [Dokumentation](#dokumentation)). Es gibt das Grundgerüst mit Datenbank, Migrationen, Health Check und CI sowie die Auslieferung auf Railway, die Admin-Anmeldung und die Ortsverwaltung im Admin (anlegen, bearbeiten, auflisten). Events werden im Admin gepflegt. Von der öffentlichen API gibt es den Vertrag `api/v1/openapi.yaml`, lesbar unter `/v1/docs`, sowie die Endpunkte `GET /v1/events`, `GET /v1/archive/events` und `GET /v1/event-types`.

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
| Vertrag | **Spec-first:** `api/v1/openapi.yaml` (OpenAPI 3.1) ist die einzige Quelle, der Server-Code wird daraus generiert. Die Spec ist öffentlich unter `/v1/openapi.yaml` abrufbar und unter `/v1/docs` als lesbare Dokumentation. |
| Fehler | [RFC 9457 Problem Details](https://www.rfc-editor.org/rfc/rfc9457) (`application/problem+json`), Texte auf Englisch |
| Zeiten | Datum `YYYY-MM-DD` und getrennte Uhrzeit `HH:MM` (lokal Europe/Berlin, `null` = unbekannt). Berechnete Zeitpunkte als ISO 8601 mit Offset. |
| Zeitraumfilter | `from` / `to`, beide inklusive. Ein Event ist enthalten, wenn sich sein Zeitraum mit dem Filter überschneidet. Fehlt `to`, ist der Zeitraum nach hinten offen. |
| Listen | Immer eine Hülle `{ "data": [ … ] }`, nie ein nacktes Array. So lassen sich später Metadaten ohne Bruch ergänzen. |
| Keine Kennungen | Events gibt es nur als gefilterte Listen, nie einzeln. Die API gibt keine internen IDs aus; Abnehmer müssen sich nichts merken. Ein Ort ist an seinem eindeutigen Namen erkennbar. |
| Namen | Alles Technische englisch, Feldnamen in camelCase (`startDate`, `effectiveEnd`). Inhalte bleiben deutsch. |
| CORS | Die öffentliche API ist offen für alle Herkünfte. Die Admin-Oberfläche ist davon getrennt und geschützt. |

## API im Überblick

Die API ist öffentlich, ohne Anmeldung nutzbar und **nur lesend**. Die Ressourcen:

| Ressource | Zweck |
| --- | --- |
| `GET /v1/events` | Aktive Events. Ohne Filter: alles, was heute noch stattfindet. Filter: `from`, `to`, `type`. Events am selben Ort tragen denselben Ortsnamen und dieselben Koordinaten und lassen sich so auf der Karte gruppieren. |
| `GET /v1/archive/events` | Vergangene Events (`effectiveEnd` erreicht), gleiche Filter, gleiche Form mit `archived: true`, absteigend nach `effectiveStart`. Ohne Filter: alle vergangenen Events. |
| `GET /v1/event-types` | Liste der Event-Typen |

Event-Typen: `festival`, `market`, `culture`, `politics`, `club`, `sports`, `other`.

Parameter von `GET /v1/events`, alle optional:

| Parameter | Bedeutung |
| --- | --- |
| `from` | Beginn des Zeitraums, inklusive: Datum `YYYY-MM-DD` (ab 00:00 Europe/Berlin) oder Zeitpunkt mit `Z` oder Offset, z. B. `2026-12-24T18:00+01:00`, in der URL als `2026-12-24T18:00%2B01:00`. Fehlt er, beginnt der Zeitraum heute; leer angegeben (`from=`) ergibt 400. |
| `to` | Ende des Zeitraums, inklusive: Datum (bis Tagesende) oder Zeitpunkt mit Offset (einschließlich seiner Minute). Fehlt er, ist der Zeitraum nach hinten offen; fehlen beide, gilt nur heute; leer angegeben (`to=`) ergibt 400. Nur `to` vor heute ergibt 400 mit Verweis auf `/v1/archive/events`. |
| `type` | Event-Typ, wiederholbar (`type=market&type=club`), ODER-verknüpft. |

Ein Event passt, wenn sein berechneter Zeitraum `[effectiveStart, effectiveEnd)` den Filterzeitraum überschneidet; geliefert werden nur aktive Events (`effectiveEnd` nach jetzt), aufsteigend nach `effectiveStart`. Das `+` eines Offsets muss in der URL als `%2B` kodiert sein; ein unkodiertes `+` kommt als Leerzeichen an und ergibt 400. Ungültige Parameter beantwortet die API mit 400 als `application/problem+json`, `detail` nennt den Parameter.

`GET /v1/archive/events` nimmt dieselben Parameter mit denselben Regeln und Fehlern, aber anderen Standardwerten: Fehlt `from`, ist der Zeitraum nach vorn offen; fehlt `to`, endet er jetzt. Nur `from` ab jetzt ergibt 400 mit Verweis auf `/v1/events`; liegen `from` und `to` beide in der Zukunft, ist die Liste leer. Ein Event ist ab der Minute seines `effectiveEnd` im Archiv und nicht mehr in `/v1/events`, unabhängig von der Bereinigung; jedes Event steht zu jedem Zeitpunkt in genau einer der beiden Listen. Einzige Ausnahme sind Events, deren Neuberechnung beim Start gescheitert ist (siehe „Programm starten“): Sie fehlen in beiden Listen, bis sie erfolgreich gespeichert oder gelöscht sind; gelöscht stehen sie in keiner.

Der Vertrag selbst liegt unter `GET /v1/openapi.yaml`, als lesbare HTML-Dokumentation unter `GET /v1/docs` (Redoc, ohne CDN und ohne Anfragen an fremde Hosts). Jede Antwort unter `/v1/` erlaubt jede Herkunft (`Access-Control-Allow-Origin: *`), Preflight-Anfragen (`OPTIONS`) beantwortet die API mit 204. Andere Methoden als `GET`, `HEAD` und `OPTIONS` lehnt sie mit 405 ab, unbekannte Pfade mit 404, beides als `application/problem+json`.

```sh
curl -s localhost:8080/v1/events
curl -s 'localhost:8080/v1/events?from=2026-11-29&to=2026-12-24&type=market'
curl -s 'localhost:8080/v1/events?from=2026-12-24T18:00%2B01:00&type=culture'
curl -s localhost:8080/v1/archive/events
curl -s 'localhost:8080/v1/archive/events?to=2026-06-30&type=festival'
curl -s localhost:8080/v1/event-types
# {"data":[{"code":"festival","label":"Fest/Kirchweih"},{"code":"market","label":"Markt"},…]}
curl -s localhost:8080/v1/openapi.yaml
# Lesbare Doku im Browser: http://localhost:8080/v1/docs
```

Ein Event aus `GET /v1/events` (gekürzt):

```json
{
  "data": [
    {
      "title": "Zirndorfer Weihnachtsmarkt, 1. Adventswochenende",
      "type": "market",
      "startDate": "2026-11-27",
      "startTime": "17:00",
      "endDate": "2026-11-29",
      "endTime": null,
      "allDay": false,
      "startPrecision": "exact",
      "endPrecision": "dateOnly",
      "effectiveStart": "2026-11-27T17:00:00+01:00",
      "effectiveEnd": "2026-11-30T00:00:00+01:00",
      "archived": false,
      "location": {
        "name": "Marktplatz Zirndorf",
        "address": { "street": "Marktplatz", "postalCode": "90513", "city": "Zirndorf" },
        "latitude": 49.4427,
        "longitude": 10.9545,
        "precision": "street"
      },
      "source": { "description": "Amtsblatt der Stadt Zirndorf, November 2026", "url": "https://www.zirndorf.de/amtsblatt" }
    }
  ]
}
```

*Die Beispieldaten sind erfunden. Ein Event mit Lücken wie der Weihnachtsmarkt steht als ein Event je zusammenhängendem Block in der API (hier je Adventswochenende), damit er an den Werktagen dazwischen nicht als laufend gilt. Verbindlich ist die OpenAPI-Spec `api/v1/openapi.yaml` mit den Schemas `Event` (Leseform) und `EventInput` (Schreibform für den Import). Die API gibt keine Kennungen aus; ein Ort ist an seinem eindeutigen Namen erkennbar.*

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
| API-Doku | Redoc 2.5.4 unter `/v1/docs`, eingebettet, kein CDN |
| Admin-Oberfläche | serverseitiges HTML (`html/template`) + htmx 2, Kartenpicker mit Leaflet und OpenStreetMap |
| Hosting | [Railway](https://railway.com): ein Service für die App, einer für PostgreSQL |
| CI | GitHub Actions. Railway deployt erst, wenn die CI grün ist. |

htmx und Leaflet 1.9.4 liegen als Dateien unter `internal/adapter/admin/static/` und werden ins Programm eingebettet, kein CDN. Leaflet stammt unverändert aus dem npm-Paket `leaflet@1.9.4` (`dist/`, Lizenz in `static/leaflet/LICENSE`); `.gitattributes` schützt die Dateien vor Zeilenende-Umwandlung. Die Kartenkacheln kommen direkt von `tile.openstreetmap.org`. Das ist nach der [Tile Usage Policy](https://operations.osmfoundation.org/policies/tiles/) der OSM Foundation für geringe Nutzung wie diesen einen Admin erlaubt, verlangt aber den sichtbaren Hinweis „© OpenStreetMap-Mitwirkende“ mit Link auf die [Urheberseite](https://www.openstreetmap.org/copyright). Ohne JavaScript oder ohne Kacheln bleibt die Karte unsichtbar bzw. leer; Speichern über die Zahlenfelder funktioniert weiter.

Redoc 2.5.4 für die API-Doku unter `/v1/docs` liegt ebenfalls eingebettet unter `internal/adapter/publicapi/v1/static/`: `redoc.standalone.js` unverändert aus dem npm-Paket `redoc@2.5.4` (`bundles/`, Lizenz MIT in `static/LICENSE`, Hinweise der gebündelten Bibliotheken in `static/redoc.standalone.js.LICENSE.txt`), ebenfalls per `.gitattributes` geschützt. Redoc zeigt in der Seitenleiste ein Logo von `cdn.redoc.ly`; eine Content-Security-Policy in `static/docs.html` lässt Bilder, Schriften und Anfragen nur vom eigenen Host zu, sodass der Browser es gar nicht erst lädt.

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
| `ADMIN_USER` | ja | `admin`, der Benutzername des einzigen Admin-Kontos |
| `ADMIN_PASSWORD_HASH` | ja | bcrypt-Hash des Admin-Passworts, Kosten mindestens 12 (`$2y$12$…`) |
| `SESSION_SECRET` | ja | mindestens 32 Byte, signiert das Session-Cookie |

Fehlt eine Variable, ist `PORT` ungültig, ist `ADMIN_PASSWORD_HASH` kein bcrypt-Hash oder hat Kosten unter 12 oder ist `SESSION_SECRET` kürzer als 32 Byte, bricht der Start mit einer JSON-Logzeile ab, die die Variable nennt. Der Wert selbst steht nie im Log.

Passwort-Hash erzeugen (fragt das Passwort ab, damit es nicht in der Shell-History landet; `htpasswd` stammt aus den Apache-Tools, z. B. Paket `apache2-utils`):

```sh
htpasswd -nBC 12 admin | cut -d: -f2   # liefert $2y$12$…
```

Session-Secret erzeugen:

```sh
openssl rand -base64 48
```

Der Hash enthält `$`. In der Shell deshalb in einfache Anführungszeichen setzen (`export ADMIN_PASSWORD_HASH='$2y$12$…'`), sonst ersetzt die Shell Teile davon durch leere Variablen. Im Railway-Dashboard wird der Wert ohne Anführungszeichen eingetragen.

Ein neues `SESSION_SECRET` macht alle bestehenden Sessions ungültig. Das ist auch der Weg, eine Session vorzeitig zu beenden: Die Sessions liegen nur im signierten Cookie, ohne Serverzustand. Ein kopiertes Cookie gilt deshalb bis zu seinem Ablauf weiter (8 Stunden ohne Anfrage, höchstens 7 Tage nach der Anmeldung), auch wenn sich der Admin abmeldet.

### Programm starten

```sh
export DATABASE_URL='postgres://eventstore:eventstore@localhost:5432/eventstore?sslmode=disable'
export PORT=8080
export ADMIN_USER=admin
export ADMIN_PASSWORD_HASH='$2y$12$…'   # siehe oben
export SESSION_SECRET="$(openssl rand -base64 48)"
go run ./cmd/eventstore
```

Beim Start laufen nacheinander die eingebetteten goose-Migrationen, die Neuberechnung der Namensschlüssel aller Orte, die Neuberechnung der abgeleiteten Werte aller Events und die Bereinigung; erst danach nimmt der HTTP-Server Anfragen an. Logs gehen als JSON auf stdout.

Kollidieren bei der Neuberechnung mehrere Orte mit demselben neuen Namensschlüssel, behalten sie ihre gespeicherten Schlüssel; das Log nennt alle beteiligten IDs (`locationIds`). Meldet erst die Datenbank den neuen Schlüssel als belegt, wird auch der Ort markiert, der ihn gerade hält, selbst wenn sein Name ganz anders lautet. Auflösen lässt sich eine Kollision nur durch Umbenennen eines der Orte; unverändertes Speichern entfernt nur die Markierung, die Kollision kehrt beim nächsten Start zurück. Ein Event, dessen Zeitangaben abgelehnt werden oder dessen Ablaufplan nicht mehr in den neu berechneten Zeitraum passt, behält seinen gespeicherten Zeitraum; das Log nennt die Event-ID, die Meldung den Programmpunkt. In beiden Fällen startet das Programm trotzdem, und die Orts- bzw. Event-Liste im Admin markiert die Betroffenen mit „prüfen“, bis ein erfolgreiches Speichern oder Löschen die Markierung entfernt; bei Orten genügt dafür ein Ort der Kollisionsgruppe. Markierte Events fehlen in `/v1/events` und `/v1/archive/events`, weil ihr gespeicherter Zeitraum falsch sein kann; nach erfolgreichem Speichern stehen sie wieder in genau einer der beiden Listen, gelöscht in keiner. Die Markierung liegt nur im Speicher und wird bei jedem Start neu ermittelt.

Die Bereinigung markiert vergangene Events (`effective_end` erreicht) in der Spalte `archived_at` und loggt die Anzahl (`marked`). Sie läuft beim Start und danach alle 24 Stunden, ist idempotent und löscht nichts. Ein Fehler wird nur geloggt; das Programm läuft weiter, der nächste Lauf versucht es erneut. Die Markierung ist reine Buchführung: Ob ein Event archiviert ist, entscheidet in API und Admin-Oberfläche immer `effectiveEnd`, die Antworten sind vor und nach der Bereinigung gleich. Speichern und die Neuberechnung beim Start leeren die Markierung, wenn sie das Event ändern; bleibt das Event vergangen, markiert der nächste Lauf es neu.

```sh
curl -i localhost:8080/healthz   # 200, solange die Datenbank erreichbar ist, sonst 503
```

Die Admin-Oberfläche liegt unter <http://localhost:8080/admin/>. Ohne Session leitet sie zur Anmeldung um. Das Session-Cookie ist `Secure`; Chrome und Firefox akzeptieren es auf `localhost` trotzdem über HTTP, andere Browser (z. B. Safari) unter Umständen nicht. Nach 5 Fehlversuchen von derselben IP ist die Anmeldung 15 Minuten gesperrt (im Speicher, ein Neustart hebt die Sperre auf).

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

Die CI (GitHub Actions) führt bei jedem Pull Request und jedem Push auf `main` `go vet`, golangci-lint v2.14.0, Unit-, Architektur- und Postgres-Tests, die Abdeckungsprüfung, die Prüfung, ob der generierte sqlc- und oapi-codegen-Code aktuell ist, sowie einen Docker-Build (ohne Push) mit Startprüfung aus. Der Architekturtest (`internal/archtest`) lässt die CI scheitern, wenn `internal/core` mehr als die Standardbibliothek und `golang.org/x/text/unicode/norm` importiert oder ein Adapter einen anderen Adapter importiert.

### Abnahme

- **SM-1 (Fixture):** `cmd/eventstore/acceptance_test.go` speichert zwei der vier Adventswochenenden des Weihnachtsmarkts 2026 über `SaveEvent` und prüft bei fester `Clock` (20.11.2026), dass `GET /v1/events?from=2026-11-29&to=2026-12-24` ihn mit Zeitraum, Ort mit Koordinaten, Zeit- und Ortsgenauigkeit und Quelle liefert. Der Test läuft mit den Postgres-Tests. Die Prüfung mit echten Daten in Produktion folgt in Story 3.5.
- **SM-4 (manuell):** Ein OZ-Mitglied öffnet `/v1/docs` und sieht die Doku durch: Lassen sich Versionierung, Fehlerformat, Zeitformat und Filterkonventionen ohne Rückfrage übernehmen? Das ist ein manueller Schritt, keine Bedingung für den Abschluss einer Story.

### Generierter Datenbankcode (sqlc)

Die Datenbankzugriffe in `internal/adapter/postgres/db` erzeugt [sqlc](https://sqlc.dev) 1.31.1 aus den Migrationen (`internal/adapter/postgres/migrations`) und den Abfragen (`internal/adapter/postgres/queries`), konfiguriert in `sqlc.yaml`. Den generierten Code nie von Hand ändern, sondern Abfrage oder Migration anpassen und neu erzeugen:

```sh
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate   # braucht cgo (gcc); alternativ das Release-Binary von sqlc 1.31.1
```

Den erzeugten Code mit committen. Die CI erzeugt ihn erneut und scheitert bei einem Unterschied.

### Generierter API-Code (oapi-codegen)

Der Vertrag der öffentlichen API ist `api/v1/openapi.yaml` (OpenAPI 3.1). Daraus erzeugt [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen) v2.8.0 das Servergerüst `internal/adapter/publicapi/v1/api.gen.go` (`std-http-server`, `strict-server`, Modelle), konfiguriert in `internal/adapter/publicapi/v1/oapi-codegen.yaml`. Den generierten Code nie von Hand ändern, sondern die Spec anpassen und neu erzeugen:

```sh
go generate ./...
```

Den erzeugten Code mit committen. Die CI erzeugt ihn erneut und scheitert bei einem Unterschied. Ein Test vergleicht die Enum-Codes der Spec (Event-Typ, Zeit- und Ortsgenauigkeit) mit den Konstanten im Kern und scheitert bei einer Abweichung. Die Abdeckungsprüfung zählt Dateien mit der Kopfzeile `// Code generated … DO NOT EDIT.` nicht mit.

## Deployment auf Railway

Die App läuft auf [Railway](https://railway.com) als ein Service aus diesem Repository, daneben ein PostgreSQL-18-Service, der nur im privaten Netz erreichbar ist. Gebaut wird das `Dockerfile` (Multi-Stage: statisches Go-Binary auf `gcr.io/distroless/static-debian13:nonroot`, ohne Shell, als Nicht-root). Die Deploy-Einstellungen stehen als Code in `railway.json`: Dockerfile-Builder, Health Check auf `/healthz`, genau eine Replika, Neustart bei Absturz.

### Einmalige Einrichtung (Railway-Dashboard)

1. Neues Projekt anlegen, Umgebung `production`.
2. PostgreSQL-Service hinzufügen und das Image auf Major 18 pinnen: unter *Settings → Source* `ghcr.io/railwayapp-templates/postgres-ssl:18` eintragen, nie `:latest` (das ist 16).
3. Am PostgreSQL-Service unter *Settings → Networking* keinen TCP-Proxy einrichten bzw. einen vorhandenen entfernen. Die Datenbank bleibt so ohne öffentliche Verbindung.
4. App-Service aus GitHub hinzufügen (dieses Repository, Branch `main`). Railway erkennt `railway.json` und baut das `Dockerfile`.
5. Am App-Service die Variable `DATABASE_URL=${{Postgres.DATABASE_URL}}` setzen, also die Referenz auf die private URL des Postgres-Services (Service-Name ggf. anpassen). `PORT` setzt Railway selbst.
   Außerdem `ADMIN_USER`, `ADMIN_PASSWORD_HASH` und `SESSION_SECRET` setzen (siehe [Umgebungsvariablen](#umgebungsvariablen)). Neue Pflichtvariablen müssen gesetzt sein, **bevor** der Pull Request gemergt wird, der sie einführt; sonst startet die neue Version nicht und die alte bleibt live.
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
- Railway baut das Image selbst und startet die neue Version. Beim Start laufen zuerst die Migrationen, die Neuberechnung und die Bereinigung, dann der HTTP-Server. Erst wenn `/healthz` mit 200 antwortet, wird die neue Version live; scheitert der Health Check (z. B. Datenbank nicht erreichbar), bleibt die alte Version aktiv und der Fehler steht im Railway-Log.

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

Die Login-Sperre der Admin-Anmeldung nimmt deshalb den linken Eintrag von `X-Forwarded-For` und nur ohne Header `RemoteAddr` ohne Port.

## Dokumentation

- [PRD](_bmad-output/planning-artifacts/prds/prd-oz-zirndorf-event-store-2026-10-01/prd.md): was der Event Store können muss
- [Architecture Spine](_bmad-output/planning-artifacts/architecture/architecture-oz-zirndorf-event-store-2026-10-01/ARCHITECTURE-SPINE.md): verbindliche Architekturentscheidungen
- [Product Brief](_bmad-output/planning-artifacts/briefs/brief-oz-zirndorf-event-store-2026-10-01/brief.md): Problem und Idee

## Über OpenZirndorf

OpenZirndorf (OZ) ist eine Open-Source-Community in Zirndorf. Jedes Mitglied baut sein Backend selbst und wählt die Technik frei. Ein gemeinsames Frontend bindet die Backends ein.

## Lizenz

Noch nicht festgelegt. Die Lizenz für Code und Daten wird vor dem öffentlichen Start entschieden.
