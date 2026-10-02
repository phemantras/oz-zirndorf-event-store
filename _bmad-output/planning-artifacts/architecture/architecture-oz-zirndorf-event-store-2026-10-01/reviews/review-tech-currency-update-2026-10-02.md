# Review: Tech-Currency (Update 2026-10-02)

- **Gegenstand:** `ARCHITECTURE-SPINE.md` (updated 2026-10-02), Abgleich mit `.memlog.md`
- **Linse:** Ist jede festgelegte Technik-Entscheidung per Web-Recherche bzw. Realitätscheck belegt (Versionen aktuell, Technik existiert und passt)?
- **Prüfdatum:** 2026-10-02
- **Spine wurde nicht verändert.**

## Urteil

**Bestanden mit Auflagen.** Alle gepinnten Versionen sind am 2026-10-02 weiterhin die jeweils neueste stabile Version. Die neuen Tech-Aussagen (Redoc 2.5.4, Railway-Header) sind belegt. Es gibt eine echte Lücke: Die NFC-Normalisierung braucht `golang.org/x/text/unicode/norm` (nicht Standardbibliothek). Diese Abhängigkeit fehlt in der Stack-Tabelle. Außerdem passt die Platzierung „Normalisierung im Adapter“ nicht sauber zu AD-10 („Der Kern parst“ die Import-Datei).

## Findings

### MITTEL-1 — NFC-Normalisierung: Abhängigkeit fehlt im Stack, Import-Pfad umgeht die Adapter-Normalisierung

**Befund.** Die Go-Standardbibliothek hat **keine** Unicode-Normalisierung. NFC gibt es nur über `golang.org/x/text/unicode/norm`. Die Kopie unter `internal/x/text/unicode/norm` bzw. `vendor/golang.org/x/text/unicode/norm` in der Standardbibliothek ist intern und lässt sich nicht importieren. Aktuell ist `golang.org/x/text` **v0.42.0** (2026-09-08).

- **AD-1 an sich wird nicht verletzt:** AD-11 und die Convention „Texteingaben“ legen die Normalisierung in die Adapter. AD-1 beschränkt nur `internal/core` auf die Standardbibliothek. Adapter dürfen `x/text` importieren.
- **Lücke 1 (Stack):** `golang.org/x/text` steht nicht in der Stack-Tabelle und hat keine gepinnte Version. Andere `x/`-Module (z. B. `x/crypto`) sind dagegen gepinnt.
- **Lücke 2 (Import-Pfad, Konsistenz):** In AD-10 Schritt 1 steht: „Der Kern parst, validiert und klassifiziert“. Wenn der Kern das JSON der Import-Datei selbst parst, gelangen Texte aus der Datei **ohne** NFC in den Kern. Der Adapter könnte zwar die rohen Bytes vorher normalisieren (`norm.NFC.Bytes`). Das erfasst aber keine JSON-Escapes wie `"ä"` (a + kombinierendes Trema). Diese werden erst beim Unmarshal zu Zeichen. Folge: `NormalizeKey` und die Umlaut-Sortierung (AD-7, ä→a) sehen ein zerlegtes „ä“. Das führt zu verpassten Duplikaten oder zu doppelten Orten mit optisch gleichem Namen, also genau zu den Fehlern, die AD-11 verhindern soll.

**Vorschlag (eine der beiden Varianten festlegen):**
- **(a) empfohlen:** Eine benannte Ausnahme in AD-1 aufnehmen: „`internal/core` importiert nur die Standardbibliothek **und `golang.org/x/text/unicode/norm`**.“ Dann normalisiert der Kern selbst (z. B. in `EventInput`-Validierung bzw. `NormalizeKey`), und jeder Eingabeweg ist abgedeckt. `x/text` wird vom Go-Team gepflegt, hat keine weiteren Abhängigkeiten und dient im Stdlib-Build intern selbst als Vendoring-Quelle. Das Risiko ist minimal.
- **(b)** Normalisierung bleibt im Adapter. Dann muss AD-10 präzisieren: Der Admin-Adapter dekodiert die Import-Datei in `core.EventInput`-Werte, normalisiert sie per NFC, und erst der Kern validiert und klassifiziert sie. Andernfalls braucht der Kern einen Port `TextNormalizer`.
- In beiden Fällen kommt eine Zeile `golang.org/x/text (unicode/norm) | v0.42.0` in die Stack-Tabelle.

**Quellen:**
- https://pkg.go.dev/golang.org/x/text/unicode/norm
- https://go.dev/blog/normalization
- https://pkg.go.dev/internal/x/text/unicode/norm (intern, nicht importierbar)
- https://proxy.golang.org/golang.org/x/text/@latest → v0.42.0, 2026-09-08

### NIEDRIG-1 — Railway X-Forwarded-For: belegt, aber Fälschbarkeit ist größer als formuliert

**Befund.** Bestätigt: Ein Railway-Mitarbeiter (Sam-a, 2026-03-09) empfiehlt in Central Station `X-Forwarded-For` mit dem ersten (linken) Eintrag. Laut derselben Quelle zeigt `X-Real-IP` seit ca. Februar 2026 bei Fastly-Routing die Fastly-Edge-IP. `Fastly-Client-Ip` gibt es nur, wenn das CDN im Pfad ist. Die Spine-Aussage stimmt also.

Die Begründung des Mitarbeiters lautet: „unser Edge-Proxy hängt an“. Genau deshalb steht ein vom Client mitgeschickter `X-Forwarded-For`-Wert **links**, sofern Railway eingehende Werte nicht verwirft. Die Quelle sagt dazu nichts Eindeutiges. In der Praxis kann jeder Angreifer pro Versuch eine neue Fantasie-IP schicken. Die 5/15-min-Sperre greift dann gar nicht, nicht nur „bremst etwas“. Umgekehrt lässt sich die Admin-IP gezielt sperren, wenn man sie kennt. Die Spine nimmt das bewusst hin (Schutz über bcrypt), das ist für Hobby-Stakes vertretbar. Die Quelle ist 7 Monate alt, und Railway hat angekündigt, `X-Real-IP` zu reparieren.

**Vorschlag:**
- Den Satz in AD-12 präzisieren: „Die Sperre wirkt nur gegen naive Angreifer; mit gefälschtem `X-Forwarded-For` ist sie umgehbar.“ Zusätzlich das Passwort ausdrücklich lang und zufällig wählen und bcrypt-Kosten ≥ 12.
- Nach dem ersten Deploy einen Realitätscheck machen: `curl -H "X-Forwarded-For: 203.0.113.9" https://<app>/admin/login` mit einem Debug-Log der Header. So zeigt sich, ob Railway den Client-Wert verwirft oder durchreicht. Das Ergebnis gehört ins Memlog.
- Optional als Härtung ohne Mehraufwand: zusätzlich ein globaler, hoher Zähler (z. B. 50 Fehlversuche pro 15 min, danach nur Verzögerung statt Sperre).

**Quellen:**
- https://station.railway.com/questions/which-header-should-i-rely-on-for-real-c-d78a6f96
- https://www.fastly.com/documentation/reference/http/http-headers/X-Forwarded-For/

### NIEDRIG-2 — Redoc 2.5.4: bestätigt, Hinweis zu Web Worker/CSP und Redoc 3

**Befund (bestätigt).**
- `redoc@2.5.4` ist npm `latest` (veröffentlicht 2026-09-10). `next` steht auf `3.0.0-rc.1`.
- `bundles/redoc.standalone.js` existiert im Paket (unpkg 200, ca. 1,1 MB).
- Das Bundle behandelt `openapi: 3.1.x` (und 3.2) ausdrücklich (`e.openapi.startsWith("3.1")`) und enthält JSON-Schema-2020-12-Unterstützung. OpenAPI 3.1 wird laut README unterstützt.
- Das Bundle lädt **keine** externen Ressourcen (keine Google Fonts, kein CDN). Self-Hosting ohne CDN funktioniert also. `<redoc spec-url="/v1/openapi.yaml">` kann YAML direkt laden.

**Hinweise:**
- Die Suche läuft in einem Web Worker aus einer `blob:`-URL, und styled-components erzeugt Inline-Styles. Bekommt `/v1/docs` später eine Content-Security-Policy, braucht diese `worker-src blob:` und `style-src 'unsafe-inline'`. Sonst fallen Suche oder Styling aus. Die Spine definiert derzeit keine CSP, akut ist das also nicht.
- Redoc 3 steht als RC bereit. Die Pinning-Strategie (Datei im Repo) schützt vor ungewollten Sprüngen. Für 3.x ist eine Wiedervorlage sinnvoll.

**Vorschlag:** Den CSP-Hinweis optional in die Story zu `/v1/docs` aufnehmen. Am Spine ist keine Änderung nötig.

**Quellen:**
- https://registry.npmjs.org/redoc (dist-tags: latest 2.5.4, next 3.0.0-rc.1)
- https://unpkg.com/redoc@2.5.4/bundles/redoc.standalone.js
- https://github.com/Redocly/redoc
- https://redocly.com/docs/redoc

### NIEDRIG-3 — Signiertes zustandsloses Session-Cookie: mit stdlib machbar, Details sind unterspezifiziert

**Befund.** Bestätigt: Das geht vollständig mit der Standardbibliothek: `crypto/hmac` + `crypto/sha256` zum Signieren, `hmac.Equal` für den konstantzeitigen Vergleich, `encoding/base64` (RawURLEncoding), `net/http.Cookie`. Ein externes Paket (z. B. `gorilla/securecookie`) ist nicht nötig. Die Inhalte sind nicht geheim (nur Benutzer und Zeitstempel), daher reicht Signieren ohne Verschlüsselung.

**Implikationen, die die Spine nicht nennt:**
- „8 h ohne Aktivität“ ist bei einem zustandslosen Cookie nur möglich, wenn das Cookie den Zeitpunkt der letzten Aktivität **und** den Anmeldezeitpunkt trägt. Es muss bei Anfragen neu ausgestellt werden (Sliding Expiry), sonst lässt sich die Inaktivitätsgrenze nicht prüfen.
- `SESSION_SECRET` braucht eine Mindestlänge (z. B. ≥ 32 Byte zufällig), die beim Start geprüft wird.
- Kontext-/Versionspräfix in der Signatur (z. B. `v1|user|iat|lastSeen`), damit sich das Format später ändern lässt.

**Vorschlag:** In AD-12 einen Halbsatz ergänzen: „HMAC-SHA256 (stdlib), Cookie trägt Anmelde- und Aktivitätszeitpunkt und wird bei Aktivität neu ausgestellt; `SESSION_SECRET` ≥ 32 Byte, beim Start geprüft.“

**Quellen:**
- https://pkg.go.dev/crypto/hmac
- https://pkg.go.dev/net/http#Cookie

### INFO — Stichprobe Stack-Versionen (am 2026-10-02 erneut abgefragt)

Alle Pins sind weiterhin neueste stabile Version. Seit der Prüfung am 2026-10-01 hat sich nichts geändert.

| Name | Spine | Aktuell (2026-10-02) | Quelle | Status |
| --- | --- | --- | --- | --- |
| Go | 1.27.1 | go1.27.1 | https://go.dev/dl/?mode=json | aktuell |
| pgx | v5.11.0 | v5.11.0 (2026-09-07) | https://proxy.golang.org/github.com/jackc/pgx/v5/@latest | aktuell |
| sqlc | 1.31.1 | v1.31.1 (2026-04-22) | https://proxy.golang.org/github.com/sqlc-dev/sqlc/@latest | aktuell (über 5 Monate alt, keine neuere Version) |
| goose | v3.28.0 | v3.28.0 (2026-09-02) | https://proxy.golang.org/github.com/pressly/goose/v3/@latest | aktuell |
| oapi-codegen | v2.8.0 | v2.8.0 (2026-07-17) | https://proxy.golang.org/github.com/oapi-codegen/oapi-codegen/v2/@latest | aktuell |
| golang.org/x/crypto | v0.57.0 | v0.57.0 (2026-09-08) | https://proxy.golang.org/golang.org/x/crypto/@latest | aktuell |
| htmx | 2.0.11 | latest 2.0.11 (next 4.0.0) | https://registry.npmjs.org/htmx.org | aktuell (bewusst 2.x) |
| Leaflet | 1.9.4 | latest 1.9.4 (alpha 2.0.0-alpha.1) | https://registry.npmjs.org/leaflet | aktuell |
| Redoc | 2.5.4 | latest 2.5.4 (next 3.0.0-rc.1) | https://registry.npmjs.org/redoc | aktuell |
| golang.org/x/text | **fehlt** | v0.42.0 (2026-09-08) | https://proxy.golang.org/golang.org/x/text/@latest | siehe MITTEL-1 |
| PostgreSQL | 18 | (Memlog 2026-10-01: Railway-Template 18.6) | — | nicht erneut geprüft, keine Hinweise auf Änderung |

Weitere Stichproben ohne Befund: `http.CrossOriginProtection` (stdlib seit Go 1.25) und `unicode.IsSpace` decken das geschützte Leerzeichen U+00A0 sowie weitere White_Space-Zeichen (U+202F, U+3000) ab. Damit ist `NormalizeKey` mit stdlib umsetzbar. Nicht erfasst werden Zero-Width-Zeichen (U+200B), die kein White_Space sind. Für Hobby-Stakes ist das hinnehmbar.

## Zusammenfassung der Fixes

1. **MITTEL-1:** `golang.org/x/text v0.42.0` in den Stack aufnehmen. Entweder eine AD-1-Ausnahme für `x/text/unicode/norm` mit Normalisierung im Kern (empfohlen), oder AD-10 präzisieren (der Adapter dekodiert und normalisiert vor dem Kern).
2. **NIEDRIG-1:** In AD-12 deutlicher sagen, dass die IP-Sperre per gefälschtem XFF umgehbar ist. Das Railway-Verhalten nach dem ersten Deploy per curl prüfen.
3. **NIEDRIG-2:** CSP-Hinweis für Redoc (`worker-src blob:`, `style-src 'unsafe-inline'`) in die Story aufnehmen. Wiedervorlage bei Redoc 3.
4. **NIEDRIG-3:** In AD-12 Mechanik und Secret-Länge der Session präzisieren (HMAC-SHA256, Sliding Expiry).
