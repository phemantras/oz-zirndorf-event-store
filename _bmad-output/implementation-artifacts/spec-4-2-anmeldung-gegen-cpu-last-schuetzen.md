---
title: '4.2: Anmeldung gegen CPU-Last von vielen Adressen schützen'
type: 'feature'
created: '2026-10-07'
status: 'done'
route: 'dispatch'
baseline_commit: 'be3ad97925f1a79feb20e5b2aacbae16eba8e451'
review_loop_iteration: 1
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-4-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Jeder Anmeldeversuch, der nicht gesperrt ist, startet sofort einen bcrypt-Vergleich (Kosten ≥ 12). Viele Adressen gleichzeitig binden so alle CPUs und bremsen auch die öffentliche API. Außerdem zählt die Sperre jede IPv6-Adresse einzeln, obwohl ein Anschluss meist ein ganzes /64 hat; ein Bot umgeht die Sperre also durch Adresswechsel (NFR-5, AD-12, AD-18, ENT-7).

**Approach:** Eine Semaphore in `adapter/admin` lässt höchstens zwei Passwortvergleiche gleichzeitig zu. Ist kein Platz frei, antwortet `submitLogin` sofort mit `503` und der deutschen Meldung, ohne bcrypt, ohne Fehlversuch und ohne Sperreintrag; die Semaphore greift vor der Sperre. Der Sperrschlüssel ist die IPv4-Adresse bzw. das IPv6-/64-Netz der Client-IP, ein ungültiger Wert bleibt unverändert. Die README nennt die Grenze des `X-Forwarded-For`-Vertrauens.

## Boundaries & Constraints

**Always:** Test-first, 100 % Abdeckung in `admin`. Werte als benannte Konstanten (`maxConcurrentPasswordChecks = 2`, `ipv6LockoutPrefixBits = 64`). Meldung wörtlich: „Gerade laufen zu viele Anmeldeversuche. Bitte versuch es gleich noch einmal.“, getrennt von `msgLockedOut`. Erfolgreiche Anmeldung, Sperre nach 5 Fehlversuchen für 15 Minuten, konstante Laufzeit (`credentialsMatch` läuft immer) und `429` bei Sperre bleiben wie in Story 1.3. Logs ohne IP, Name, Passwort. Entscheidungen: IPv4-in-IPv6 (`::ffff:a.b.c.d`) zählt als IPv4-Adresse; eine IPv6-Zone wird verworfen; der Platz wird erst nach dem Passwortvergleich freigegeben.

**Never:** Kein Warten auf einen freien Platz, kein Rate Limiting pro Client, keine Obergrenze für die Sperr-Map, kein Kern-Code, keine Migration, keine Änderung an `/v1`, kein `X-Real-IP`, kein `Retry-After`.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Dritter gleichzeitiger Versuch | zwei Versuche blockieren in `comparePassword`, dritter von anderer IP | `503`, Login-Seite mit neuer Meldung, kein bcrypt-Aufruf, kein Sperreintrag für dessen Schlüssel | Log `WARN` |
| Platz wieder frei | die zwei blockierten Versuche enden | nächster Versuch läuft durch bcrypt | – |
| IPv6 /64 | 5 Fehlversuche von `2001:db8:1:2::1`…`::5` | Versuch von `2001:db8:1:2::ffff` → `429`; `2001:db8:1:3::1` → Anmeldung möglich | – |
| IPv4 | 5 Fehlversuche von `203.0.113.7` | `203.0.113.8` nicht gesperrt | – |
| Ungültige IP | Header `unknown` | Schlüssel `unknown` | – |

</frozen-after-approval>

## Code Map

- `internal/adapter/admin/handler.go` -- `submitLogin` (Z. 242–264): zuerst `http.MaxBytesReader` setzen und Benutzername und Passwort lesen (`PostFormValue`, Fehler wie bisher ignoriert), damit ein langsam gesendeter Body keinen Platz hält; erst danach, noch vor `beginAttempt`, nicht blockierend einen Platz der Semaphore belegen (`select` mit `default`), sonst `h.logger.Warn(logMsgLoginBusy)` und `h.render(..., http.StatusServiceUnavailable, loginPage{Error: msgTooManyLogins})`; danach `lockoutKey(clientIP(r))` statt `clientIP(r)` an `beginAttempt`/`reset`; Platz per `defer` freigeben. `handler` bekommt Feld `passwordCheckSlots chan struct{}`, `newHandler` legt ihn mit Kapazität `maxConcurrentPasswordChecks` an. Neue Konstanten in den Blöcken Z. 56–59 und Z. 63–68.
- `internal/adapter/admin/clientip.go` -- neue Funktion `lockoutKey(ip string) string` mit `net/netip`: ungültig → unverändert; IPv4/4in6 → `Unmap().String()`; IPv6 → `netip.PrefixFrom(addr.WithZone(""), ipv6LockoutPrefixBits).Masked().String()`. `clientIP` bleibt unverändert.
- `internal/adapter/admin/clientip_test.go` -- Tabellentest für `lockoutKey` nach dem Muster von `TestClientIPPrefersLeftmostForwardedForEntry`.
- `internal/adapter/admin/lockout.go` -- Doc-Kommentare „per client IP“ auf „per lockout key“ anpassen; Logik unverändert.
- `internal/adapter/admin/handler_test.go` -- `newTestServer`, `loginRequest` (setzt `X-Forwarded-For`), `ts.handler.comparePassword` als Fake, `ts.handler.lockout.failures` zum Prüfen der Sperr-Map. `TestLoginRejectsSixthAttemptWhileEarlierAttemptsAreInFlight` (Z. 247) blockiert heute 5 Versuche gleichzeitig in bcrypt; mit 2 Plätzen bekämen 3 davon `503`. Test-Helfer: ein Helfer startet N Logins und blockiert genau N Vergleiche (kein getrenntes `count`/`blocking`, das auseinanderlaufen kann). `loginRequestFrom` nutzt als Edge-Eintrag eine eigene Konstante (`testEdgeIP`), nicht `otherTestIP`; `loginRequest` ebenso.
- `README.md` -- Z. 241 („von derselben IP“) auf IPv4-Adresse bzw. IPv6-/64-Netz und die `503`-Grenze ändern; Abschnitt „Client-IP hinter Railway“ (Z. 331–347) um den Satz ergänzen, dass der linke Eintrag nur hinter Railways Edge stimmt und ein zusätzlicher Proxy (z. B. Cloudflare) eine Anpassung von `clientIP` braucht.

## Nahtstellen

- `submitLogin` -- 1.3 -- Erfolg setzt Session und leitet um, falsche Daten `200` mit Meldung, Sperre `429` ohne bcrypt, Log ohne Geheimnisse -- bestehende Login-Tests in `handler_test.go` bleiben grün.
- `TestLoginRejectsSixthAttemptWhileEarlierAttemptsAreInFlight` -- 1.3 (ENT-21) -- belegt, dass ein laufender Versuch schon als Fehlversuch zählt; muss bleiben, aber mit höchstens einem Versuch im Vergleich: `maxFailedLogins - maxConcurrentPasswordChecks` Fehlversuche nacheinander, `maxConcurrentPasswordChecks` blockiert in bcrypt, der Zähler steht dabei schon auf `maxFailedLogins`, nach der Freigabe → `429` -- `TestLoginCountsAttemptsAsFailuresWhileInFlight`.
- `loginLockout` -- 1.3 -- zählt jetzt Schlüssel statt IPs; Ablauf, Reset und Aufräumen unverändert -- `lockout_test.go` unverändert grün.
- `clientIP` -- 1.2/1.3 -- linker `X-Forwarded-For`-Eintrag, sonst `RemoteAddr` ohne Port -- `clientip_test.go` unverändert grün.
- Request-Deadline aus 4.1 -- gilt auch für die Anmeldung; keine Änderung nötig.

## Tasks & Acceptance

**Execution:**
- [x] `internal/adapter/admin/clientip.go` + Test -- `lockoutKey` mit Tabellentest (IPv4, IPv6, zwei Adressen eines /64 gleich, anderes /64 verschieden, 4in6, Zone, ungültig, leer) -- AD-12
- [x] `internal/adapter/admin/handler.go` + `handler_test.go` -- Formular lesen, dann Semaphore vor der Sperre, `503` mit Meldung und `WARN`, Freigabe nach dem Vergleich, Sperre per `lockoutKey`; Tests aus der Matrix; In-Flight-Test anpassen -- AD-18
- [x] `internal/adapter/admin/handler_test.go` -- weitere Tests: (a) Formularfelder werden vor dem Belegen des Platzes gelesen (`maxConcurrentPasswordChecks` Logins, deren Body beim Lesen blockiert; ein weiterer Login erreicht trotzdem bcrypt statt `503`); (b) gesperrter Schlüssel bekommt bei belegten Plätzen `503`, Zähler unverändert; (c) nach `maxConcurrentPasswordChecks` gesperrten (`429`) Versuchen und nach ebenso vielen erfolgreichen Anmeldungen erreicht ein weiterer Versuch bcrypt; (d) erfolgreiche Anmeldung aus einem IPv6-/64 setzt den Zähler des Netzes zurück; (e) In-Flight-Test hält `maxConcurrentPasswordChecks` Versuche derselben IP nach `maxFailedLogins - maxConcurrentPasswordChecks` Fehlversuchen; schon währenddessen steht der Zähler auf `maxFailedLogins`, nach der Freigabe → `429`; (f) Log-Test der `503`-Warnung prüft auch Namen und IPs der blockierten Versuche und `100.64.0.2` -- Review
- [x] `internal/adapter/admin/lockout.go` -- Kommentare auf Sperrschlüssel -- Klarheit
- [x] `README.md` -- Sperrschlüssel, `503` und Proxy-Hinweis -- Story-AC

**Acceptance Criteria:**
- Given zwei blockierte Passwortvergleiche, when ein dritter Versuch mit derselben IP kommt und danach beide enden, then ist der Fehlversuchszähler dieser IP nur um die zwei laufenden Versuche gestiegen.
- Given ein abgelehnter `503`-Versuch, when ich das Log lese, then steht dort eine Warnung ohne IP, Name oder Passwort.

## Verification

**Commands:**
- `CI= go test ./...` -- grün
- `bash scripts/check-coverage.sh` -- 100 %
- `go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./...` -- keine Befunde
- `go test -race ./internal/adapter/admin/...` -- grün

## Implementation Notes

## Spec Change Log

- **Rücksprung 1 (Review, bad_spec):** Befund: Der Code Map ließ den Platz „zuerst“ belegen, also vor dem Lesen des Bodys; zwei langsam sendende Verbindungen hielten so beide Plätze bis `readTimeout` (15 s) und jeder Login bekam `503`, ohne bcrypt. Geändert: Code Map `handler.go` (Formular vor dem Platz lesen), Test-Helfer, Nahtstelle In-Flight-Test, neue Task mit Tests (a)–(f). Vermiedener Zustand: billiger Ausschluss des Admins mit zwei offenen Verbindungen. KEEP: `lockoutKey` samt Tabellentest, Konstanten- und Meldungsnamen (`msgTooManyLogins`, `logMsgLoginBusy`, `maxConcurrentPasswordChecks`, `ipv6LockoutPrefixBits`), `passwordCheckSlots` mit `select`/`default` und `defer`-Freigabe, Kommentare in `lockout.go`, README-Texte, alle bestehenden neuen Tests. Der Code wurde nicht zurückgesetzt (Zurücksetzen vom Nutzer-Rechtesystem abgelehnt), sondern wird an die geänderte Spec angepasst.
- **Korrektur zu Rücksprung 1:** Test (e) forderte `429`, während beide Plätze belegt sind; das widerspricht „Semaphore vor der Sperre“ (dann `503`). Neu: Zähler während der gehaltenen Versuche prüfen, `429` erst nach der Freigabe.

## Review Triage Log

| # | Quelle | Befund | Verdikt | Begründung | Route |
|---|--------|--------|---------|------------|-------|
| 1 | edge, seam, blind | Platz vor dem Lesen des Bodys belegt; zwei langsame Bodies blockieren jeden Login bis 15 s | medium | `readTimeout` 15 s (`cmd/eventstore/main.go:38`), Body wird erst nach `select` gelesen | bad_spec |
| 2 | edge | ungültiger Wert gleich einem /64-Schlüssel teilt den Zähler | false | Railways Edge setzt `X-Forwarded-For` selbst, ohne Header `RemoteAddr`; beliebige Strings erreichen `lockoutKey` nicht | – |
| 3 | edge, blind | `releaseAndWait` wartet `blocking`-mal, `startLogins` hat eigenes `count` | low | Künftiger Test hängt oder liest die Map zu früh; ein Helfer mit einem N ist direkt | bad_spec (mitgenommen) |
| 4 | seam, blind | Gesperrter Schlüssel bei belegten Plätzen → `503` ungetestet | low | Reihenfolge laut Intent, aber kein Test | bad_spec (mitgenommen) |
| 5 | verification-gap, blind | Freigabe des Platzes auf `429`- und Erfolgspfad ungetestet | medium | Ein Leck auf diesen Pfaden bliebe grün und sperrte nach zwei Treffern alle Logins | bad_spec (mitgenommen) |
| 6 | verification-gap | `reset` mit /64-Schlüssel für IPv6 ungetestet | medium | `reset(clientIP(r))` bliebe grün; IPv6-Admin behielte Fehlversuche | bad_spec (mitgenommen) |
| 7 | blind | Platz auch während Sperrprüfung und Rendern gehalten | false | Nach Fix 1 nur Mikrosekunden ohne I/O; Freigabe nach dem Vergleich ist Entscheidung im Frozen-Block | – |
| 8 | blind | Zwei Angreifer mit wechselnden /64 halten den Admin dauerhaft auf `503` | low | Folge der Intent-Entscheidung (kein Rate Limiting, kein Warten); Doku wäre Zusatz | – |
| 9 | blind | In-Flight-Test hält nur einen Versuch | low | Zwei gleichzeitige Versuche derselben IP sind möglich und belegen ENT-21 besser; direkte Änderung | bad_spec (mitgenommen) |
| 10 | blind | Log-Test der `503`-Warnung prüft „mallory“, `testIP`, `100.64.0.2` nicht | low | Ergänzen der Liste ist direkt | bad_spec (mitgenommen) |
| 11 | blind | Map-Zugriffe in Tests ohne `lockoutKey` | false | Testkonstanten sind IPv4; Schlüssel = IP, belegt im Tabellentest | – |
| 12 | blind | `loginRequestFrom(otherTestIP)` erzeugt `198.51.100.4, 198.51.100.4` | low | Verwischt Client/Edge im Test; eigene Konstante ist direkt | bad_spec (mitgenommen) |
| 13 | blind | Klammer-IPv6 `[2001:db8::1]` umgeht /64 | false | Railways Edge liefert Adressen ohne Klammern (README, Messung 1.2) | – |
| 14 | blind | Sprint-Status und Spec-Status weichen ab | false | Sync am Ende des Workflows (Persistent Fact) | – |
| 15 | blind | README nennt Wert 2 statt Konstante, kein Hinweis auf Angriff | low | Kosmetisch, Betreiberdoku nennt Verhalten korrekt | – |
| 16 | edge | `holdPasswordChecks` mit `count` > Plätze hängt | low | Nur künftiger Testfehlgebrauch; Guard wäre Zusatzzweig | – |
| 17 | edge | `release()` doppelt → Panik | low | Lauter Fehler bei Fehlgebrauch; `sync.Once` wäre Zusatz | – |
| 18 | edge, blind, verification-gap | Regression lässt Tests hängen statt scheitern (`entered`, `done`, `reading` ohne Timeout) | low | Scheitert weiterhin, nur langsam; Timeouts wären neue Zweige in allen Helfern | – |
| 19 | blind | `503`-Seite verliert den Benutzernamen | low | Name ist schon gelesen; `Username: user` ist direkt | patch |
| 20 | blind | Platz bleibt beim Schreiben der Antwort belegt | false | Login-Seite und Redirect passen in Socket-Puffer, Schreiben blockiert nicht auf langsamen Leser | – |
| 21 | blind | Kein Handler-Test für `::ffff:`-Adresse | low | Tabellentest deckt `lockoutKey`; Railway liefert IPv4 direkt | – |
| 22 | blind | `lockoutKey` gehört nach `lockout.go` | low | Kein benannter Schaden; Verschieben ohne Verhaltensänderung | – |
| 23 | blind | README ohne Hinweis auf dauerhafte `503` durch zwei Angreifer | low | carried #8 | – |
| 24 | blind | Log-Flut durch `503`-Warnungen | low | Wie bestehende `429`-Warnung; Sampling wäre neue Logik | – |
| 25 | blind | arc42 QS-7 nennt nur „die Adresse“ | low | Doku weicht vom Sperrschlüssel ab; Umformulieren ist direkt | patch |
| 26 | blind | Status-Abweichung, Zeilennummern im Code Map | false | Sync am Ende des Workflows; Spec-Änderung als Fix ist ausgeschlossen | – |
| 27 | blind | In-Flight-Test prüft Vorläufe nicht | low | `countInFlight` belegt ENT-21; Zusatz-Asserts ohne Schaden | – |
| 28 | blind, seam | Gesperrter Client lädt Body (≤ 4 KiB) vor `429` | low | Bewusste Folge von Befund 1, begrenzt durch `maxLoginFormBytes` und `readTimeout` | – |
