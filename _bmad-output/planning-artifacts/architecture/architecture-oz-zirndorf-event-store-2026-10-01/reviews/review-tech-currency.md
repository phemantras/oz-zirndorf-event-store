# Review: Aktualität der Technik — Architecture Spine OZ Zirndorf Event Store

- **Geprüft:** `ARCHITECTURE-SPINE.md` (Stand 2026-10-01) und `.memlog.md`
- **Prüfdatum:** 2026-10-01
- **Linse:** Jede festgelegte Entscheidung muss im Web, im bestehenden Projekt oder im aktuellen Starter nachgeprüft sein und darf nicht nur aus Trainingswissen stammen. Geprüft wurden Versionen, ob die Technik existiert und passt, Standardwerte der Plattform und Konflikte zwischen den gepinnten Versionen.

## Urteil

**Bestanden mit Auflagen.** Die gepinnten Kernversionen (Go, PostgreSQL 18, pgx, sqlc, goose, oapi-codegen, htmx) sind im Memlog belegt, und die Stichproben bestätigen sie. Die Versionen vertragen sich untereinander. Es gibt keinen harten Blocker. Offen sind:

- eine echte Fähigkeitslücke: Der sqlc-Parser kennt nur PostgreSQL 17,
- zwei Annahmen, die als Plattformwissen dastehen, aber unvollständig sind: der Railway-Health-Check und die Werte für Leaflet und htmx,
- zwei Lücken im Stack: bcrypt/x/crypto und die Session/CSRF-Umsetzung sind nicht benannt.

## Was im Memlog schon belegt ist und hier nur stichprobenartig geprüft wurde

| Eintrag | Memlog-Beleg | Stichprobe 2026-10-01 |
| --- | --- | --- |
| Go 1.27.1 | go.dev Release-Historie | nicht erneut geprüft. pgx v5.11.0 nennt ausdrücklich Go-1.27-Features, das passt. |
| PostgreSQL 18, Railway-Template 18.6, `:latest` = 16 | belegt | nicht erneut geprüft |
| `uuidv7()` nativ in PG 18 | belegt | bestätigt (postgresql.org Doku 18, 8.12) |
| Railway-DBs ohne TCP-Proxy seit 2026-07-31 | belegt | bestätigt (bex.co, 2026-09-11) |
| oapi-codegen v2.8.0, Go ≥ 1.25 | belegt | bestätigt, aber mit Einschränkung (F-3) |
| sqlc 1.31.1 | belegt | bestätigt als neueste Version. Fähigkeitslücke siehe F-1. |
| pgx v5.11.0 | belegt | bestätigt als neueste Version, Mindest-Go 1.25 |
| goose v3.28.0, Go ≥ 1.26 | belegt | bestätigt als neueste Version, Mindest-Go 1.26. Mit Go 1.27.1 verträglich. |
| htmx 2.x / 4.0 | belegt | bestätigt: npm `latest` = 2.0.11, `next` = 4.0.0 |

## Findings

### F-1 — sqlc 1.31.1 parst mit einem PostgreSQL-17-Parser, nicht mit PG 18 · **Mittel**

- **Befund:** sqlc v1.31.1 hängt an `github.com/pganalyze/pg_query_go/v6 v6.2.2`. Das ist libpg_query auf Basis von PostgreSQL 17. Der Spine setzt PG 18 voraus und zählt sqlc trotzdem ohne Einschränkung zum Stack. Die Kombination „sqlc + PG 18“ wurde im Memlog nicht geprüft.
- **Auswirkung:**
  - `DEFAULT uuidv7()` in einer goose-Migration ist ein gewöhnlicher Funktionsaufruf. Der PG-17-Parser kann ihn lesen, also kein Problem.
  - Kritisch wird es, wenn Queries `uuidv7()` direkt aufrufen. sqlc kann dann den Rückgabetyp nicht ableiten, weil der eingebaute pg_catalog die Funktion vermutlich nicht kennt (nicht im Web bestätigt).
  - Kritisch wird es auch bei syntaktischen PG-18-Neuerungen, z. B. virtuellen generierten Spalten oder `OLD`/`NEW` in `RETURNING`. Diese bricht `sqlc generate` ab.
- **Empfehlung:** Konvention ergänzen. Migrationen und Queries nutzen nur Syntax, die schon in PG 17 gilt. `uuidv7()` steht nur im `DEFAULT` der Tabellen. Queries lassen die ID von der DB vergeben und lesen sie über `RETURNING id`. Ein früher Spike sollte `sqlc generate` gegen die erste Migration laufen lassen.
- **Quellen:** [sqlc v1.31.1 go.mod](https://raw.githubusercontent.com/sqlc-dev/sqlc/v1.31.1/go.mod), [sqlc Releases](https://github.com/sqlc-dev/sqlc/releases), [PostgreSQL 18 UUID-Typ](https://www.postgresql.org/docs/current/datatype-uuid.html)

### F-2 — Railway-Health-Check: Verhalten nur teilweise abgebildet · **Mittel**

- **Befund:** Der Spine sagt: „`GET /healthz` prüft, ob die Datenbank erreichbar ist. Railway nutzt ihn für Deployments.“ Das stimmt, ist aber unvollständig und war nicht belegt. Laut Railway-Doku gilt:
  1. Railway fragt den Pfad **nur während eines Deployments** ab. Nach dem Go-Live überwacht Railway ihn nicht mehr. `/healthz` ist also kein Laufzeit-Monitoring. Das passt zu „Railway-Logs reichen“, sollte aber ausdrücklich dastehen.
  2. Der Health-Check muss aktiv konfiguriert werden, über `healthcheckPath` in `railway.json`/`railway.toml` oder in der UI. Im Structural Seed fehlt eine `railway.json`, damit ist die Konfiguration nicht als Code festgehalten.
  3. Standard-Timeout 300 s (`RAILWAY_HEALTHCHECK_TIMEOUT_SEC`). Die Anfragen kommen vom Host `healthcheck.railway.app` auf `PORT`.
  4. Die Migrationen laufen laut Spine **vor** dem HTTP-Server. Eine lange Migration zählt deshalb gegen das Timeout.
  5. Wenn `/healthz` an der DB hängt, schlägt jedes Deployment fehl, solange die DB nicht erreichbar ist. Die alte Version läuft dann weiter. Für Hobby-Betrieb ist das akzeptabel, sollte aber eine bewusste Entscheidung sein.
- **Empfehlung:** `railway.json` mit `healthcheckPath: /healthz` in den Structural Seed aufnehmen. Ausdrücklich festhalten, dass der Check nur beim Deployment greift. Server-Middleware darf Requests nicht über den Host-Header filtern.
- **Quellen:** [Railway Healthchecks](https://docs.railway.com/deployments/healthchecks), [Railway Config as Code](https://docs.railway.com/config-as-code/reference)

### F-3 — oapi-codegen v2.8.0: OpenAPI 3.1 ist nur „initial support“ · **Mittel**

- **Befund:** AD-8 legt OpenAPI 3.1 als Vertrag fest. Laut Release-Notes bringt v2.8.0 nur **„initial OpenAPI 3.1 support“**: Webhooks/Callbacks, `oneOf`-Enums und `type: [T, "null"]` werden unterstützt, andere 3.1-Konstrukte womöglich nicht vollständig. Der Memlog hält nur „OpenAPI 3.1, braucht Go ≥ 1.25“ fest, nicht diese Einschränkung.
- **Bestätigt:** std-http-Server und Strict-Server gehen mit v2.8.0. Pfade mit Schrägstrich am Ende werden jetzt mit `{$}` verankert. Das ist eine Verhaltensänderung, die nur greift, wenn die Spec solche Pfade enthält.
- **Empfehlung:** Die Spec auf den gut unterstützten 3.1-Kern beschränken: einfache Objekte, `enum`, Nullbarkeit über `type: [T, "null"]`, keine `$dynamicRef` und kein `if`/`then`. Früh einen Spike machen: `oapi-codegen` mit `std-http-server` + `strict-server` gegen einen Spec-Entwurf laufen lassen. Fällt der Spike negativ aus, ist OpenAPI 3.0.3 als Rückfall möglich, ohne dass sich AD-8 im Kern ändert.
- **Quellen:** [oapi-codegen v2.8.0 Release](https://github.com/oapi-codegen/oapi-codegen/releases/tag/v2.8.0), [Diskussion #2478](https://github.com/oapi-codegen/oapi-codegen/discussions/2478)

### F-4 — Leaflet: „aktuelle Version beim Bau“ ist ungeprüft und mehrdeutig · **Niedrig–Mittel**

- **Befund:** Im Stack steht für Leaflet keine Version, und der Memlog enthält keinen Beleg. Laut GitHub ist **v1.9.4 die neueste stabile Version**. Die 1.x-Linie ist im Wartungsmodus. **Leaflet 2.0 liegt nur als Alpha vor** (`2.0.0-alpha.1`). Die Daten der GitHub-Seite und der Suchergebnisse widersprechen sich im Jahr. In beiden Quellen ist 2.0 aber nicht stabil. 2.0 bricht die API: nur ESM, ES6-Klassen, Pointer Events, kein globales `L` mehr.
- **Risiko:** „Aktuelle Version“ kann je nach Lesart 2.0-alpha bedeuten. Eine Story würde dann gegen eine instabile API bauen, und die meisten Beispiele im Netz passen nicht dazu.
- **Empfehlung:** Leaflet **1.9.x** pinnen (z. B. 1.9.4), lokal einbinden (`static/`) statt über CDN, und den Wechsel auf 2.0 erst nach dem stabilen Release prüfen. Zusätzlich die Nutzungsrichtlinie der OSM-Kacheln beachten: Attribution, gültiger Referer/User-Agent, keine Massenabrufe. Für einen Admin-Kartenpicker mit sehr wenig Last ist das unkritisch, sollte aber in der Story stehen.
- **Quellen:** [Leaflet Releases](https://github.com/leaflet/leaflet/releases), [Leaflet (Wikipedia)](https://en.wikipedia.org/wiki/Leaflet_(software))

### F-5 — htmx „2.x (aktuelle 2er-Version beim Projektstart)“ nicht konkret gepinnt · **Niedrig**

- **Befund:** Der Stand ist belegt: npm `latest` = **2.0.11**, `next` = **4.0.0**. Laut htmx-Ankündigung soll 4.0 Anfang 2027 `latest` werden. Wer htmx ohne Versionsangabe über CDN oder npm holt, bekommt dann still 4.x mit Breaking Changes.
- **Empfehlung:** Im Stack **htmx 2.0.11** eintragen und die Datei unter `static/` einchecken (vendoring), keine unversionierte CDN-URL.
- **Quellen:** [npm htmx.org](https://registry.npmjs.org/htmx.org), [htmx 4.0.0 Ankündigung](https://four.htmx.org/announcements/2026-08-28-htmx-4.0.0-is-released)

### F-6 — bcrypt/`golang.org/x/crypto` fehlt im Stack · **Niedrig**

- **Befund:** AD-12 verlangt einen bcrypt-Hash, aber bcrypt steht nicht in der Go-Standardbibliothek. Es kommt aus `golang.org/x/crypto/bcrypt`. Die aktuelle Version ist **v0.57.0** vom 2026-09-08. Das Paket existiert und passt, fehlt aber in der Stack-Tabelle und ist eine zusätzliche Abhängigkeit des Admin-Adapters. Nach AD-1 ist das zulässig, nur im Kern nicht.
- **Hinweis:** Ein bcrypt-Hash enthält `$`, z. B. `$2a$12$…`. In `.env`-Dateien, Docker-Compose-Interpolation und Shell-Exports wird `$` umgedeutet. Railway-Variablen nutzen `${{…}}`, das ist laut Doku unkritisch, wurde aber nicht im Einzelfall geprüft. Beim lokalen Setup braucht der Hash einfache Anführungszeichen oder `$$`.
- **Empfehlung:** `golang.org/x/crypto` (bcrypt) in den Stack aufnehmen und den Hinweis zur Quotierung in die Story zur Konfiguration übernehmen.
- **Quellen:** [pkg.go.dev golang.org/x/crypto/bcrypt](https://pkg.go.dev/golang.org/x/crypto/bcrypt), [pkg.go.dev golang.org/x/crypto](https://pkg.go.dev/golang.org/x/crypto)

### F-7 — Session- und CSRF-Umsetzung ohne benannte Technik; die Standardbibliothek bietet seit Go 1.25 eine Alternative · **Niedrig**

- **Befund:** AD-12 legt eine Session (HttpOnly, Secure, SameSite=Strict) und ein CSRF-Token fest. Wie das umgesetzt wird, steht nirgends: eigener signierter Cookie über `SESSION_SECRET`, `gorilla/sessions` oder `gorilla/csrf`. Seit **Go 1.25** gibt es `net/http.CrossOriginProtection`. Sie wehrt CSRF über `Sec-Fetch-Site`/`Origin` ab, ohne Token. In Go 1.25.1 wurde eine Bypass-Lücke geschlossen, Go 1.27.1 ist also nicht betroffen. Zusammen mit `SameSite=Strict` würde das die Token-Pflicht und eine Zusatzbibliothek ersparen.
- **Empfehlung:** Entscheiden und festhalten. Entweder AD-12 bleibt beim Token, dann die Bibliothek bzw. Eigenimplementierung benennen. Oder AD-12 wechselt auf `http.CrossOriginProtection` + `SameSite=Strict`, das bleibt nur Standardbibliothek und passt zum Stack-Prinzip.
- **Quellen:** [Go 1.25 Release Notes](https://go.dev/doc/go1.25), [Calhoun: CSRF Protection via Headers in Go 1.25](https://www.calhoun.io/csrf-protection-via-headers-in-go-125/)

### F-8 — Lokales PostgreSQL 18 in Docker: Volume-Pfad hat sich geändert · **Niedrig**

- **Befund:** Laut Seed läuft lokal PostgreSQL 18 per `compose.yaml`. Das offizielle `postgres:18`-Image hat `PGDATA` auf `/var/lib/postgresql/18/docker` geändert und das `VOLUME` auf `/var/lib/postgresql`. Das übliche Mount `…:/var/lib/postgresql/data` aus älteren Vorlagen und Trainingswissen führt zu Startfehlern oder dazu, dass Daten nicht persistiert werden.
- **Empfehlung:** In `compose.yaml` das Image `postgres:18` (oder konkret `18.x`) pinnen und das Volume auf `/var/lib/postgresql` mounten.
- **Quellen:** [Docker Hub postgres](https://hub.docker.com/_/postgres), [docker-library/postgres #1400](https://github.com/docker-library/postgres/issues/1400), [Fixing the PostgreSQL 18 PGDATA Error in Docker Compose](https://aronschueler.de/blog/2025/10/30/fixing-postgres-18-docker-compose-startup/)

### F-9 — Railway-Build und privates Netz: bestätigt, kleine Ergänzungen · **Info**

- **Dockerfile-Deploy:** Bestätigt. Railway erkennt ein `Dockerfile` im Wurzelverzeichnis automatisch; die Schreibweise mit großem D ist Pflicht. Multi-Stage-Builds sind gewöhnliche Docker-Builds. Build-Variablen sind nur per `ARG` in der jeweiligen Stage sichtbar. Für einen statischen Go-Build ist keine nötig.
- **Privates Netz / `DATABASE_URL`:** Bestätigt. `DATABASE_URL` des Postgres-Service zeigt auf `postgres.railway.internal:5432` und ist nur im selben Projekt und derselben Umgebung erreichbar. Im App-Service wird es als Referenz `${{Postgres.DATABASE_URL}}` gesetzt. Im Help-Forum gibt es vereinzelte Berichte über Verbindungsprobleme beim Start über das private Netz. Ein Retry/Backoff beim DB-Connect vor den Migrationen ist deshalb sinnvoll. Die Zeile „Datenbank nur über privates Netz erreichbar“ heißt auch: Lokal kommt man ohne TCP-Proxy nicht an die Prod-DB. Für Datenkorrekturen oder ein manuelles Backup per `pg_dump` (siehe Deferred „Datensicherung“) braucht es dann `railway connect` oder einen temporär eingeschalteten Proxy.
- **Quellen:** [Railway Dockerfiles](https://docs.railway.com/builds/dockerfiles), [bex.co: Railway's Databases Went Private by Default](https://bex.co/blog/2026/09/11/railway-private-databases-connection-string-migration), [Railway Help Station: private networking](https://station.railway.com/questions/private-networking-service-cannot-reach-3d1be833)

## Prüfung der Versionsverträglichkeit

| Komponente | Mindest-Go | Ergebnis mit Go 1.27.1 |
| --- | --- | --- |
| pgx v5.11.0 | 1.25 (unterstützt Go-1.27-Features) | ok |
| goose v3.28.0 | 1.26 | ok |
| oapi-codegen v2.8.0 | 1.25 | ok |
| sqlc 1.31.1 (CLI, `go 1.26.0`) | 1.26 | ok. Der erzeugte Code nutzt pgx v5, das ist verträglich. |
| golang.org/x/crypto v0.57.0 | aktuelle Go-Versionen | ok (Mindestversion nicht einzeln geprüft) |

Kein Konflikt zwischen den gepinnten Versionen. Die einzige echte Spannung ist der PG-17-Parser in sqlc gegenüber PG 18 als Ziel (F-1).

## Zusammenfassung nach Schweregrad

| ID | Schweregrad | Kurz |
| --- | --- | --- |
| F-1 | Mittel | sqlc-Parser = PG 17, PG-18-Syntax und `uuidv7()` in Queries meiden |
| F-2 | Mittel | Railway-Health-Check nur beim Deployment, `railway.json` fehlt, Timeout und Migrationen |
| F-3 | Mittel | oapi-codegen: OpenAPI 3.1 nur „initial support“, Spike nötig |
| F-4 | Niedrig–Mittel | Leaflet ungepinnt, stabil = 1.9.4, 2.0 nur Alpha |
| F-5 | Niedrig | htmx auf 2.0.11 pinnen und einchecken, `latest` wechselt 2027 auf 4.x |
| F-6 | Niedrig | `golang.org/x/crypto` (bcrypt) fehlt im Stack, `$` im Hash quotieren |
| F-7 | Niedrig | Session/CSRF-Technik offen, Go-1.25-`CrossOriginProtection` als Option |
| F-8 | Niedrig | `postgres:18`-Docker-Volume-Pfad geändert |
| F-9 | Info | Railway-Dockerfile und privates Netz bestätigt, Retry beim DB-Connect |
