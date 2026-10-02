---
title: 'Story 1.3: Admin-Anmeldung'
type: 'feature'
created: '2026-10-02'
status: 'done'
baseline_commit: '8995c3b6f9834bce82a5206c2b75986d27a9743d'
route: 'dispatch'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-1-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Unter `/admin` gibt es noch nichts. Bevor Orte und Events gepflegt werden (ab Story 1.4), muss der Admin geschützt sein: genau ein Konto, Session, Login-Sperre, CSRF-Schutz (FR-14, AD-12, ENT-6/7/21/22). Die Akzeptanzkriterien der Story 1.3 in `epics.md` gelten vollständig.

**Approach:** Alles im Adapter `internal/adapter/admin` (Kern bleibt unverändert): zustandsloses HMAC-SHA256-Session-Cookie, In-Memory-Sperre je Client-IP, Login-/Logout-Handler, Auth-Middleware, `http.CrossOriginProtection`, deutsche Templates mit schlichtem Layout und htmx 2.0.11 aus `admin/static`. `cmd/eventstore` liest die drei neuen Variablen, prüft sie beim Start und hängt den Admin unter `/admin/` ein.

## Boundaries & Constraints

**Always:** Cookie HttpOnly, Secure, SameSite=Strict, Path `/admin`, Inhalt Anmelde- und Aktivitätszeitpunkt plus MAC; jede authentifizierte Anfrage stellt es mit neuem Aktivitätszeitpunkt neu aus. Gültig nur bei korrektem MAC, `now < Aktivität + 8 h` und `now < Anmeldung + 7 Tage`. Client-IP = linker Eintrag von `X-Forwarded-For` (README, Story 1.2), ohne Header `RemoteAddr` ohne Port; nie `X-Real-IP`. bcrypt immer ausführen, auch bei falschem Benutzernamen; Benutzername in konstanter Zeit vergleichen. Zeit über injizierte `now func() time.Time`. Start bricht ab bei fehlender Variable, `SESSION_SECRET` < 32 Byte, ungültigem Hash oder bcrypt-Kosten < 12. Test-first, 100 % Abdeckung für `internal/adapter/admin`. Benannte Konstanten für Dauern, Grenzen, Pfade, Cookie-Name.

**Never:** Keine Passwörter, Hashes, Secrets, Cookie-Werte und keine Client-IPs im Log. Keine CORS-Header unter `/admin`. Keine Registrierung, kein Serverzustand für Sessions, keine Datenbank-Tabelle. Kein CDN. Keine Änderung an `internal/core`, Migrationen oder `/healthz`.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Login ok | POST `/admin/login` mit richtigen Daten | 303 → `/admin/`, Session-Cookie gesetzt, Fehlerzähler der IP gelöscht | N/A |
| Login falsch | Falscher Name oder falsches Passwort | 200, Formular mit „Benutzername oder Passwort ist falsch.“, Name bleibt im Feld, kein Cookie | Zähler +1 |
| Gesperrt | 5 Fehlversuche, 6. Versuch (auch richtig) innerhalb 15 min | 429, deutsche Sperrmeldung, bcrypt nicht aufgerufen | Sperre endet 15 min nach dem 5. Fehlversuch |
| Ohne Session GET/POST | `/admin/…` außer Login und `/admin/static/` | 303 → `/admin/login`, Handler läuft nicht | N/A |
| Ohne Session htmx | Header `HX-Request: true` | 401 mit `HX-Redirect: /admin/login` | N/A |
| Fremde Herkunft | POST mit `Sec-Fetch-Site: cross-site` | 403 | N/A |
| Abgelaufen / manipuliert | Aktivität > 8 h, Anmeldung > 7 d, falscher MAC, kaputtes Format, altes Secret | wie „Ohne Session“ | N/A |
| Logout | POST `/admin/logout` mit Session | Cookie gelöscht (`Max-Age` < 0), 303 → `/admin/login` | N/A |
| Login-Seite angemeldet | GET `/admin/login` mit gültiger Session | 303 → `/admin/` | N/A |

**Entscheidungen (2026-10-02, bei Freigabe bestätigt; volle Spec trotz Überlänge ~2.600 Tokens behalten):** Fehlversuche einer IP verfallen 15 min nach dem letzten Fehlversuch; erfolgreicher Login setzt den Zähler zurück; abgelaufene Einträge werden bei jedem Zugriff aufgeräumt. Startseite `/admin/` zeigt Überschrift, Hinweis „Orte und Events folgen“ und Abmelde-Button. Fehlgeschlagene Logins werden ohne IP und ohne Namen geloggt.

</frozen-after-approval>

## Code Map

- `cmd/eventstore/config.go` -- `loadConfig`/`missingVariablesError` um `ADMIN_USER`, `ADMIN_PASSWORD_HASH`, `SESSION_SECRET` erweitern; Hash-/Secret-Prüfung als eigene Fehler, nie mit Wert in der Meldung.
- `cmd/eventstore/health.go` -- `newRouter(db, logger)` bekommt den Admin-Handler und mountet ihn unter `/admin/`; `newHealthHandler` unverändert.
- `cmd/eventstore/main.go` -- `run` baut `admin.NewHandler(...)` aus der Config, `newServer` reicht ihn weiter.
- `cmd/eventstore/*_test.go` -- `envFrom`-Maps in bestehenden `run`-Tests um gültige Admin-Variablen ergänzen, sonst testen sie nur noch den Config-Fehler.
- `internal/adapter/admin/doc.go` -- Paket existiert leer; Adapter importieren nur `core` und Stdlib/x-Pakete (Architekturtest `internal/archtest`).
- `scripts/check-coverage.sh` -- 100 %-Gate deckt `internal/adapter/admin/...` schon ab.
- `.github/workflows/ci.yaml` -- Docker-Startprüfung greppt „missing required environment variables“; Meldung beibehalten.

## Tasks & Acceptance

**Execution:**
- [x] `go.mod` -- `golang.org/x/crypto v0.57.0` hinzufügen -- bcrypt.
- [x] `internal/adapter/admin/session.go` (+ Test) -- Cookie ausstellen, prüfen, verlängern, löschen -- Session ohne Serverzustand.
- [x] `internal/adapter/admin/lockout.go` (+ Test) -- Sperre je IP mit Mutex, Grenze 5, Dauer 15 min -- ENT-7.
- [x] `internal/adapter/admin/clientip.go` (+ Test) -- linker XFF-Eintrag getrimmt, sonst `RemoteAddr` ohne Port -- ENT-21.
- [x] `internal/adapter/admin/handler.go` (+ Test) -- `NewHandler(Config)` mit Login (GET/POST), Logout (POST), Startseite (GET `/admin/`), Static, Auth-Middleware, CrossOriginProtection -- alle Matrix-Zeilen als Tests.
- [x] `internal/adapter/admin/templates/{layout,login,home}.html` -- deutsch, schlichtes Layout, `<script src="/admin/static/htmx.min.js">`, per `embed`.
- [x] `internal/adapter/admin/static/htmx.min.js` -- htmx 2.0.11 unverändert von npm, SHA-256 in Implementation Notes -- kein CDN.
- [x] `cmd/eventstore/config.go`, `main.go`, `health.go` (+ Tests) -- Variablen, Startprüfungen, Verdrahtung.
- [x] `README.md` -- Variablen-Tabelle, Hash erzeugen (bcrypt-Kosten 12, quoten wegen `$`), Secret erzeugen (`openssl rand -base64 48`), Railway-Variablen vor dem Merge setzen, ENT-6-Hinweis (kopiertes Cookie gilt bis Ablauf), Status-Zeile.

**Acceptance Criteria:**
- Given alle fünf Variablen gültig, when das Programm startet, then ist `/admin/login` erreichbar und `/healthz` unverändert.
- Given `SESSION_SECRET` mit 31 Byte oder ein Hash mit Kosten 10, when das Programm startet, then bricht es mit klarer Meldung ohne den Wert ab.
- Given ein Cookie, ausgestellt mit Secret A, when das Programm mit Secret B läuft, then gilt es als ungültig.
- Given eine Anfrage unter `/admin/`, when sie beantwortet wird, then fehlt jeder `Access-Control-*`-Header.
- Given Login-Fehlversuche mit Passwort `geheim123`, when das Log gelesen wird, then kommt `geheim123` nicht vor.

## Implementation Notes

- htmx 2.0.11: `internal/adapter/admin/static/htmx.min.js` unverändert aus `https://registry.npmjs.org/htmx.org/-/htmx.org-2.0.11.tgz` (`package/dist/htmx.min.js`, 52182 Byte), SHA-256 `d6fdc75f204e6bdefa99b69bf1e6d4ac69b8a364f77929f45c13476b4000f717`.
- Cookie-Name `eventstore_admin_session`. Zeitpunkte werden auf Sekunden abgeschnitten, weil das Cookie Unix-Sekunden trägt; `Max-Age` wird aufgerundet und ist so nie 0 (net/http würde das Attribut sonst weglassen).
- Benutzername: Vergleich der SHA-256-Digests mit `subtle.ConstantTimeCompare`, damit auch die Länge nicht über die Laufzeit erkennbar ist. bcrypt läuft in jedem nicht gesperrten Versuch.
- Sperrprüfung und Zählen sind atomar (`beginAttempt`): Jeder zugelassene Versuch zählt schon vor bcrypt als Fehlversuch, ein Erfolg setzt den Zähler zurück. So kommen parallele Versuche nicht an der Grenze vorbei. Versuche während einer Sperre zählen nicht; die Sperre endet 15 min nach dem 5. Versuch.
- Der eingegebene Benutzername wird getrimmt (wie `ADMIN_USER` beim Start). `ADMIN_PASSWORD_HASH` muss genau 60 Zeichen lang sein.
- Logout läuft durch die Auth-Middleware, die das Cookie bereits erneuert hat; der Handler entfernt diesen `Set-Cookie`-Header und setzt nur das Löschcookie.
- Login-Formular-Body ist auf 4 KiB begrenzt (`http.MaxBytesReader`).
- Gesperrte Versuche werden als `admin login rejected during lockout` geloggt, ohne IP und Namen.
- README empfiehlt `htpasswd -nBC 12` (Präfix `$2y$`); `golang.org/x/crypto/bcrypt` akzeptiert das, ein Config-Test sichert es ab.
- Lokal nicht ausgeführt: `-race` (kein cgo unter Windows) und die Postgres-Tests inkl. der neuen `/admin/login`-Prüfung in `TestRunMigratesThenServesHealthUntilCancelled` (kein Docker). Beides läuft in der CI.

## Spec Change Log

## Review Triage Log

| # | Quelle | Befund | Verdikt | Evidenz | Route |
|---|--------|--------|---------|---------|-------|
| 1 | gap | Handler-Tests prüfen nicht, dass die Sperre am weitergeleiteten Client-IP hängt | medium | `loginRequest` setzt immer dieselbe XFF und dieselbe `RemoteAddr`; `ip := r.RemoteAddr` bliebe grün | patch |
| 2 | gap | Verdrahtung `newAdminHandler` nur per GET der Login-Seite geprüft | medium | Vertauschte Felder oder fehlendes `Now` (Panic in `pruneExpired`) blieben grün | patch |
| 3 | edge + blind | Parallele Logins einer IP umgehen die Sperre | medium | `isLocked` und `recordFailure` nehmen den Mutex getrennt, dazwischen ~250 ms bcrypt; viele parallele Versuche passieren | patch |
| 4 | edge + blind | Gefälschter XFF umgeht die Sperre außerhalb von Railway, kein `ParseIP` | low | Railways Edge verwirft Client-XFF (Messung 1.2), App nur über Edge erreichbar; so von AD-12 gewollt | reject |
| 5 | edge + blind | Fehler-Map unbegrenzt, IPv6 nicht per /64 gruppiert | low | Einträge verfallen nach 15 min, Schlüssel nur echte Client-IPs hinter Railway; AD-12 sperrt je IP | reject |
| 6 | edge | `GET /admin/static/` listet das Verzeichnis | low | Listet nur öffentliche Dateien; Abhilfe bräuchte eigenen Wrapper | reject |
| 7 | edge | Abgeschnittener oder verlängerter Hash startet, aber kein Login klappt | low | `bcrypt.Cost` prüft nur den Präfix; bcrypt-Hashes haben immer 60 Zeichen, Prüfung ist eine Zeile | patch |
| 8 | edge | Benutzername mit Leerzeichen am Rand scheitert und zählt als Fehlversuch | low | `ADMIN_USER` wird getrimmt, das Formularfeld nicht; mobile Tastaturen hängen Leerzeichen an | patch |
| 9 | edge + blind | Body > 4 KiB zählt als Fehlversuch ohne 413 | low | Nur bei manipulierten Anfragen; Abhilfe braucht neuen Zweig | reject |
| 10 | blind | Keine globale Grenze, kein Semaphor um bcrypt | low | Nicht Teil von AD-12; Hobby-Betrieb, Schutz ist bcrypt + langes Passwort | reject |
| 11 | blind | Kein `X-Frame-Options`/CSP/`Cache-Control: no-store` | low | SameSite=Strict schickt das Cookie nicht in fremde Frames; Admin zeigt keine geheimen Daten | reject |
| 12 | blind | Templates schreiben Pfade und Feldnamen literal | low | Routen ändern sich selten, Tests prüfen Formular-Ziele; Durchreichen bräuchte FuncMap | reject |
| 13 | blind | „15 Minuten“ in der Meldung doppelt zur Konstante, kein `Retry-After` | low | Konstante ändert sich kaum; Formatierung wäre zusätzlicher Code | reject |
| 14 | blind | Passwortwechsel beendet Sessions nicht | low | AD-12: nur neues `SESSION_SECRET` beendet Sessions, README nennt diesen Weg | reject |
| 15 | blind | `NewHandler` prüft seine `Config` nicht | low | Einziger Aufrufer prüft beim Start; Schutzcode ohne erreichbaren Fall | reject |
| 16 | blind | 404/403/500 unter `/admin` englisch | low | Nur bei Tippfehlern im Pfad oder Angriffen; eigene Fehlerseiten sind neue Oberfläche | reject |
| 17 | blind | Logout löscht jedes `Set-Cookie` | low | Es gibt kein weiteres Cookie | reject |
| 18 | blind | `SESSION_SECRET` misst Zeichen statt Schlüsselstärke | false | `len` auf Strings zählt Bytes; geprüft wird genau die zugesagte Mindestlänge von 32 Byte | reject |
| 19 | blind | README: Safari und Secure-Cookie auf localhost; Beispiel `andreas` vs. `admin` | low | Aussage gilt sicher nur für Chrome/Firefox; Beispiele widersprechen sich; reine Textkorrektur | patch |

## Design Notes

Cookie-Wert `<anmeldung_unix>.<aktivität_unix>.<base64url(HMAC-SHA256(secret, "anmeldung.aktivität"))>`, Vergleich mit `hmac.Equal`. `Max-Age` = Sekunden bis `min(Aktivität + 8 h, Anmeldung + 7 d)`. htmx 2 wertet `HX-Redirect` auch bei 401 aus, daher kein 200 nötig. Auth-Middleware sitzt vor dem Admin-Mux, CrossOriginProtection außen um alles (auch Login). Die Zeit kommt per `now`-Funktion, weil der Kern-Port `Clock` erst mit Story 1.6 entsteht; Session-Ablauf ist Adapter-Logik, nicht Fachlogik.

## Verification

**Commands:**
- `CI= go test ./...` -- grün
- `bash scripts/check-coverage.sh` -- 100 %
- `go vet ./...` und golangci-lint v2.14.0 -- ohne Befund

**Manual checks (if no CLI):**
- Lokal `go run ./cmd/eventstore`, im Browser anmelden, abmelden, 6× falsch anmelden → Sperrmeldung.
- Railway: drei Variablen gesetzt, bevor der PR gemergt wird; nach Deploy Login auf der Railway-Domain.
