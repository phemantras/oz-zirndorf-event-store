# Epic 4 Context: Der Dienst hält Überlastung und Missbrauch stand

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

Eine langsame Datenbank, eine Request-Flut oder Anmeldeversuche von vielen Adressen sollen den Dienst höchstens verlangsamen, aber nicht mehr zum Absturz bringen (Speicherstau bis OOM, CPU-Bindung durch bcrypt). Außerdem soll die öffentliche API keine unnötigen Egress-Kosten verursachen (1,1 MB Redoc-Skript unkomprimiert, keine Cache-Header). Anlass war eine Security-Bewertung: Railways Edge (Fastly) wehrt volumetrische Angriffe auf Layer 3/4 ab, bietet aber kein Rate Limiting und kein Caching. Der „Under Attack Mode“ sperrt Clients ohne Browser aus und ist für `/v1` ungeeignet. Epics 1 bis 3 sind fertig. Dieses Epic härtet nur Adapter und Verdrahtung, ohne Kern-Code und ohne Migration.

## Stories

- Story 4.1: Deadline für jeden Request und begrenzter Datenbank-Pool
- Story 4.2: Anmeldung gegen CPU-Last von vielen Adressen schützen
- Story 4.3: Cache-Header und komprimierte statische Dateien in der öffentlichen API

## Requirements & Constraints

- Betrieb auf Hobby-Niveau: keine Zielwerte für Antwortzeit und Verfügbarkeit. Best Effort heißt aber nicht schutzlos: Ein einzelner Client darf den Dienst nicht mit wenig Aufwand zum Absturz bringen, weder über viele oder teure Anfragen noch über die Anmeldung. Für eine langsame Datenbank gilt dasselbe.
- Fehler unter `/v1` folgen RFC 9457 (Problem Details), mit englischem `detail`. Die Admin-Oberfläche bleibt deutsch.
- Die öffentliche API bleibt nur lesend, mit offenem CORS auf jeder Antwort, auch auf Fehlern und statischen Dateien. Admin hat kein CORS.
- Vertragsänderungen nur abwärtskompatibel: `503` als zusätzliche gemeinsame Antwort und ein Satz zum Caching in `info` sind erlaubt.
- Nicht im Umfang: Rate Limiting pro Client, serverseitiges Caching, Grenze für das Archiv, Cloudflare vor Railway, `statement_timeout`, Obergrenze für die Sperr-Map.
- Erfolgskriterien: Eine künstlich langsame Abfrage mit kurzer Deadline ergibt `503` vor `WriteTimeout`, und der Pool meldet `MaxConns` 10. Bei drei gleichzeitigen Anmeldeversuchen mit blockierendem Passwortvergleich wird der dritte ohne bcrypt abgelehnt, und IPv6-Adressen aus einem /64 teilen sich einen Zähler. Alle `/v1`-Antworten tragen den passenden `Cache-Control`, und das Redoc-Skript kommt per gzip deutlich kleiner als 1,1 MB. Die CI ist grün (100 % Abdeckung in den Pflichtpaketen, Lint, Generator-Diff, Postgres-Tests).

## Technical Decisions

**Lastgrenzen (AD-18).** Dies ist eine dauerhafte Regel für jeden neuen Endpunkt, keine einmalige Story-Entscheidung. Alle Werte sind benannte Konstanten im jeweiligen Adapter bzw. in `cmd/eventstore`:

- **Request-Deadline:** `requestTimeout` = 20 s, liegt unter `writeTimeout` = 30 s, und ein Test prüft dieses Verhältnis. Eine Middleware in `cmd/eventstore` umschließt den ganzen Router. Mit der Deadline enden auch der Kontext, jede Datenbankabfrage und jedes Warten auf eine Pool-Verbindung. `/healthz` behält seinen eigenen, kürzeren Ping-Timeout.
- **Folgen bei Ablauf:** `/v1` antwortet mit `503` als Problem Details. Geloggt wird eine Warnung, kein Error (`context.Canceled` wird schon heute nicht als Error geloggt). Der Admin zeigt die übliche deutsche Fehlerseite, und die Transaktion wird zurückgerollt.
- **Pool:** `MaxConns` = 10 in `postgres.Connect`, fest und unabhängig von der CPU-Zahl und von Parametern in `DATABASE_URL`. Es gibt kein `statement_timeout`, weil Migrationen und `RecomputeDerived` beim Start denselben Pool nutzen und nicht begrenzt werden dürfen.
- **bcrypt:** Höchstens zwei Vergleiche laufen gleichzeitig (Semaphore in `adapter/admin`). Ist kein Platz frei, wird der Versuch sofort mit `503` und der deutschen Meldung „Gerade laufen zu viele Anmeldeversuche. Bitte versuch es gleich noch einmal.“ abgelehnt. Er zählt nicht als Fehlversuch und legt keinen Sperreintrag an. Die Semaphore greift **vor** der Sperre. Dadurch bleibt die Sperr-Map klein (höchstens etwa 8 neue Einträge pro Sekunde), und eine eigene Obergrenze ist nicht nötig.
- **Cache-Control unter `/v1`:** Event-Listen und Event-Typen `public, max-age=60`. Spec, Import-Schema und Docs-Seite `public, max-age=300`. Redoc-Skript `public, max-age=86400`. Alle Fehler (400, 404, 405, 500, 503) `no-store`. Preflight (`OPTIONS`) behält `Access-Control-Max-Age` und bekommt kein `Cache-Control`.
- **gzip:** Redoc-Skript, Spec und Import-Schema werden einmal beim Start aus den eingebetteten Dateien komprimiert, nicht pro Request. Ausgeliefert wird mit `Content-Encoding: gzip` und `Vary: Accept-Encoding`, ohne `Accept-Encoding: gzip` unkomprimiert. Event-Listen werden nicht komprimiert.

**Anmeldeschutz (AD-12, ENT-7).** Nach 5 Fehlversuchen ist die Anmeldung 15 Minuten gesperrt. Der Sperrschlüssel ist die IPv4-Adresse bzw. das IPv6-/64-Netz. Ist der Wert keine gültige IP, zählt er unverändert als Schlüssel. Die Zähler liegen im Speicher. Die Client-IP kommt aus dem linken Eintrag von `X-Forwarded-For` (Railways Edge), ohne den Header aus `RemoteAddr` ohne Port. `X-Real-IP` wird nicht genutzt. Erfolgreiche Anmeldung, Sperre und konstante Laufzeit bleiben wie bisher.

**Spec und generierter Code (AD-8).** `api/v1/openapi.yaml` ist die einzige Quelle. `503` kommt als gemeinsame Antwort mit Beispiel dazu. In `info` steht, dass Listen bis zu 60 s zwischengespeichert werden dürfen und ein Event deshalb bis zu einer Minute nach seinem Ende noch in `/v1/events` stehen kann; maßgeblich bleibt `effectiveEnd`. `api.gen.go` wird nur per `go generate ./...` neu erzeugt.

**Grenzen der Schichten.** Es gibt keinen Kern-Code und keine Migration, AD-1 und AD-2 bleiben unberührt. 4.1 betrifft `cmd/eventstore`, `adapter/postgres` und die Spec. 4.2 betrifft nur `adapter/admin` und die README. 4.3 betrifft nur `adapter/publicapi/v1` und die Spec. Es gilt Test-first.

## UX & Interaction Patterns

- Die Login-Seite bekommt eine weitere deutsche Meldung für „zu viele gleichzeitige Anmeldeversuche“ (`503`). Sie ist von der Sperrmeldung nach Fehlversuchen getrennt.
- Bei abgelaufener Deadline zeigt der Admin die bestehende deutsche Fehlerseite, keine neue.

## Cross-Story Dependencies

- Die Stories sind voneinander unabhängig. Empfohlen ist die Reihenfolge 4.1, 4.2, 4.3.
- 4.1 und 4.3 berühren beide die Spec und die Problem-Antworten von `/v1`. Die `503` aus 4.1 muss in 4.3 `Cache-Control: no-store` tragen. Wer später kommt, regeneriert `api.gen.go` auf dem Stand des anderen.
- Die Deadline aus 4.1 gilt auch für Admin-Requests und damit auch für die Anmeldung aus 4.2.
- 4.1 baut auf bestehenden Nahtstellen auf: Fehlerbehandlung der öffentlichen API, Server-Timeouts in `cmd/eventstore`, die Admin-Fehlerseite mit Transaktionen über `TxRunner` und die Startreihenfolge (Migrationen, `RecomputeDerived`, `MarkArchived`, HTTP-Server).
- 4.2 baut auf `submitLogin`, `loginLockout` und `clientIP` auf. Die README muss ergänzen, dass der linke `X-Forwarded-For`-Eintrag nur hinter Railways Edge stimmt und ein zusätzlicher Proxy (z. B. Cloudflare) eine Anpassung braucht.
- 4.3 baut auf `serveEmbedded`, `serveStaticFile` und `readOnlyCORS` auf.
