---
reviewed: ARCHITECTURE-SPINE.md (status draft, 2026-10-01)
inputs: ARCHITECTURE-SPINE.md, .memlog.md, prd.md, addendum.md
method: Good-Spine-Checkliste (Divergenzpunkte, durchsetzbare Rules, Deferred ohne Divergenzrisiko, aktuelle Technik, Spec-Abdeckung, Dimensionen inkl. Betriebsumfeld, Bloat, Mermaid)
date: 2026-10-01
---

# Review: Architecture Spine — OZ Zirndorf Event Store

## Urteil

Ein solider, gut begründeter Spine mit klarem Paradigma und einem starken Zeitmodell. Er lässt aber vier echte Divergenzpunkte offen: die Zeitdarstellung in der API, die einzige Quelle für Enum-Codes und Feldnamen, den Transaktions-Port für den Import und ein Betriebsrisiko (keine Sicherung bei automatischen, nur vorwärts laufenden Migrationen). Nach diesen Korrekturen ist er tragfähig.

| Schweregrad | Anzahl |
| --- | --- |
| critical | 0 |
| high | 4 |
| medium | 8 |
| low | 9 |

## Was gut ist

- Paradigma (Hexagonal light) und AD-1/AD-2 verhindern verstreute Fachregeln wirksam. Das passt zu einem Projekt mit nur einem Entwickler.
- AD-3/AD-4/AD-5 lösen Konflikt D sauber und erfüllen FR-8, FR-12 und FR-13 („genau einer der beiden Zugriffe“) ohne Abhängigkeit vom Job-Zeitpunkt.
- AD-10 (zustandsloser Import, erneute Klassifizierung, eine Transaktion) ist eine echte und gut gewählte Abwägung.
- Die genannte Technik ist aktuell und belegt: Go 1.27.1 (2026-09-01), oapi-codegen v2.8.0 mit OpenAPI 3.1 (2026-07-17, braucht Go ≥ 1.25), goose v3.28, sqlc 1.31.1, pgx v5.11, PostgreSQL 18 mit `uuidv7()`. Der Hinweis, dass Railway `:latest` auf PG 16 zeigt, ist wertvoll.
- Das Betriebsumfeld ist grundsätzlich adressiert: Railway-Projekt, Umgebungen, privates Netz, Health Check, CI, Dockerfile, lokale Umgebung.
- Alle drei Mermaid-Diagramme sind syntaktisch gültig (Flowchart mit `subgraph id [Titel]`, Zylinder `[( )]`, erDiagram mit typisierten Attributen).

## Findings

### HIGH-1 — Zeitdarstellung in der API ist widersprüchlich

**Wo:** Consistency Conventions, Zeilen „Feldnamen JSON“ und „Zeiten (API)“; AD-7, AD-9.
**Problem:** Die Feldnamen-Zeile nennt `startDate`, `startTime` und `effectiveEnd` als JSON-Felder. Die Zeiten-Zeile verlangt dagegen *einen* ISO-8601-Wert: einen Zeitpunkt mit Offset, wenn die Uhrzeit bekannt ist, sonst nur `YYYY-MM-DD`. Damit ist offen, ob die API getrennte Felder (`startDate` + `startTime` + `startPrecision`) oder ein polymorphes Feld `start` liefert. Offen ist auch, ob `effectiveStart`/`effectiveEnd` und ein Archiv-Kennzeichen (FR-11) Teil des Vertrags sind. Weil AD-9 identische Feldnamen für Import und API verlangt, können das API-Epic und das Import-Epic hier unabhängig voneinander inkompatibel entscheiden. Ein polymorphes `string`-Feld (date oder date-time) erzeugt in oapi-codegen außerdem untypisierten Code.
**Fix:** Ein AD „API-Zeitdarstellung“ ergänzen. Es legt die exakte Feldform für Beginn, Ende und Genauigkeit fest, z. B. `start: "2026-12-01"` oder `"2026-12-01T18:00:00+01:00"` plus `startPrecision`, analog für `end`. Es klärt, ob `effectiveStart`/`effectiveEnd` ausgeliefert werden (Empfehlung: nein, das ist ein interner Abfrageschlüssel) und wie „archiviert“ erkennbar ist (z. B. `archived: bool`, abgeleitet nach AD-4). Die Feldnamen-Zeile entsprechend korrigieren und für das Import-Format dieselbe oder eine bewusst abweichende Form festhalten.

### HIGH-2 — Enum-Codes und Feldnamen haben drei Quellen ohne Abgleich

**Wo:** AD-8, AD-9, AD-1, Conventions „Enum-Codes“.
**Problem:** Enum-Codes stehen laut Spine „in `api/openapi.yaml`“. Der Kern muss sie aber selbst kennen (FR-5: unbekannter Typ wird abgelehnt) und darf nach AD-1 keine generierten Adapter-Typen importieren. Dazu kommt `import-v1.schema.json` als dritte Quelle. Die Rule „Feldnamen und Enum-Codes sind identisch“ hat keinen Prüfmechanismus. Der Kern nutzt zur Laufzeit keinen JSON-Schema-Validator (nur stdlib), also ist das Import-Schema reine Doku und kann vom Parser abweichen. AD-9 verhindert damit genau die Divergenz nicht, die es verhindern soll.
**Fix:** Eine Quelle bestimmen und den Abgleich mechanisch machen. Variante A: Das Import-Schema referenziert die Komponenten aus `openapi.yaml` per `$ref` (OpenAPI 3.1 ist kompatibel zu JSON Schema 2020-12). Dazu kommt ein Test, der die Enum-Konstanten des Kerns gegen die Spec prüft. Variante B: Die Kern-Konstanten sind die Quelle, und ein Test vergleicht sie mit beiden Dateien. Zusätzlich ein Test, der die Import-Testsammlung (42 Events) sowohl gegen das Schema als auch durch den Kern-Parser schickt. Den CI-Check aus AD-8 auf diese Tests ausweiten.

### HIGH-3 — Kein Port für Transaktionen, obwohl der Kern „in einer Transaktion“ schreibt

**Wo:** AD-1, AD-6, AD-10.
**Problem:** AD-10 verlangt, dass `CommitImport` alle Einträge in **einer** DB-Transaktion schreibt, inklusive neu anzulegender Orte, auf die Events per ID verweisen. Der Kern kennt aber nur stdlib und Repository-Ports, und wie eine Transaktion über mehrere Ports gespannt wird, ist nicht festgelegt. Das Postgres-Epic und das Import-Epic werden das unabhängig lösen, z. B. mit `pgx.Tx` im Kontext, mit einer Bulk-Methode `SaveImport` im Repository (dann liegt Logik im Adapter, was AD-6 verletzt) oder mit einem Unit-of-Work-Interface. Dasselbe gilt für FR-7: Löschschutz im Kern und gleichzeitiges Löschen sind nur in einer Transaktion korrekt.
**Fix:** In AD-6 oder einem neuen AD das Muster festlegen, z. B. einen Kern-Port `type TxRunner interface { InTx(ctx, func(Repos) error) error }`. Der Postgres-Adapter implementiert ihn mit `pgx.BeginFunc` und liefert transaktionsgebundene Repositories. Jeder schreibende Anwendungsfall läuft in `InTx`. Ergänzend: Der FK `events.location_id` ist `ON DELETE RESTRICT` als Sicherheitsnetz (das ist ein Constraint, keine Fachlogik im Sinne von AD-2).

### HIGH-4 — Keine Sicherung, aber automatische, nur vorwärts laufende Migrationen bei jedem Push

**Wo:** Deferred „Datensicherung“, Conventions „Migrationen“, Structural Seed (Deploy-Fluss).
**Problem:** Jeder Push auf `main` deployt (siehe MEDIUM-5, auch ohne grünes CI). Migrationen laufen beim Start automatisch und nur vorwärts. Eine fehlerhafte oder destruktive Migration (`DROP`/`ALTER … TYPE`) zerstört von Hand gepflegte Daten unwiederbringlich. Die Import-Dateien sichern Admin-Änderungen, Orte mit gesetzten Koordinaten und Duplikatentscheidungen nicht. Die Begründung für das Zurückstellen stimmt nur halb: Railway-Hobby hat zwar keine nativen Backups, aber der übliche Workaround ist ein Cron-Service in Railway, der per `pg_dump` in einen Railway-Bucket oder S3 sichert. Wegen des seit 2026-07-31 rein privaten DB-Netzes ist ein `pg_dump` aus GitHub Actions **nicht** möglich, die Sicherung muss also innerhalb von Railway laufen. Das ist eine Architekturentscheidung (ein dritter Service) und keine reine Zurückstellung.
**Fix:** Die Sicherung aus Deferred holen und als Seed festlegen: ein dritter Railway-Service „backup“ (Cron, täglich `pg_dump` mit PG-18-Client in einen Bucket, Aufbewahrung 14 Tage, Prüfung der Lesbarkeit). Alternativ, minimal: ein `pg_dump` als Pre-Deploy-Command vor jedem Deploy. Dazu die Regel „Migrationen sind additiv. Destruktive Änderungen nur in zwei Schritten (expand/contract) und nach einer Sicherung.“

### MEDIUM-1 — Tagesgrenze und Quelle von „now“ nicht festgelegt

**Wo:** AD-4, AD-13; Addendum „Vorbei-Regel“.
**Problem:** Das Addendum sagt „23:59:59 des End-Tages“, der Spine sagt „Ende des End-Tages“. Ob `effectiveEnd` als `23:59:59` oder als exklusiver Wert (nächster Tag `00:00` Europe/Berlin) gespeichert wird, ist offen. Mit `effectiveEnd > now` entsteht bei 23:59:59 eine Lücke von einer Sekunde, und der Wert ist nicht DST-robust zu „vorbei ab Mitternacht“. Offen ist auch, ob „now“ aus Go kommt (Parameter) oder aus SQL `now()`. Cleanup-Epic und API-Epic können das unterschiedlich machen, und Tests wie „seit einer Minute vorbei“ (FR-12) brauchen eine injizierbare Uhr.
**Fix:** In AD-4 festhalten, dass `effectiveEnd` exklusiv ist (Beginn des Folgetags in Europe/Berlin, DST-korrekt berechnet) und aktiv `effectiveEnd > now` gilt. Zusätzlich einen Kern-Port `Clock`. Alle Abfragen bekommen `now` als Parameter, und SQL nutzt nie `now()`/`CURRENT_DATE`. Den Satz im Addendum als überholt markieren.

### MEDIUM-2 — Gültige Kombinationen im Zeitmodell undefiniert

**Wo:** AD-3.
**Problem:** Offen ist, ob `allDay=true` zusammen mit `startTime` erlaubt ist, ob `endTime` ohne `endDate` erlaubt ist (typisch „19–22 Uhr“ am selben Tag, so auch in `zirndorf_events.json`) und wie die Endgenauigkeit lautet, wenn `allDay` für beide gilt. Das PRD verlangt getrennte Genauigkeit für Beginn und Ende, ein einzelnes Boolean kann z. B. „Beginn exakt, Ende ganztägig“ nicht ausdrücken. Ob das gewollt ist, steht nicht da. Admin-Formular und Import-Format werden diese Fälle unterschiedlich modellieren, obwohl die Validierung im Kern liegt, weil die Formate vorher festgelegt werden.
**Fix:** In AD-3 eine kleine Tabelle der zulässigen Kombinationen ergänzen, mit der abgeleiteten Genauigkeit je Fall. Empfehlung: `endTime` ohne `endDate` bedeutet gleicher Tag. `allDay` schließt beide Uhrzeiten aus. Außerdem ausdrücklich festhalten, dass die Ableitung aus dem einen `allDay` die Anforderung von FR-2 erfüllt.

### MEDIUM-3 — Zeitmodell des Ablaufplans (FR-4) fehlt

**Wo:** AD-3, Structural Seed (`TIMETABLE_ENTRIES` ohne Zeitspalten).
**Problem:** Programmpunkte haben Beginn und optionales Ende. Unklar ist, ob nur Uhrzeit, Datum + Uhrzeit (bei mehrtägigen Festen nötig) oder auch Genauigkeiten erlaubt sind. Auch hier gilt KON-4 („nie eine erfundene Uhrzeit“). Import und Admin werden das sonst unterschiedlich lösen.
**Fix:** AD-3 auf Programmpunkte erweitern: `date` (Pflicht), `startTime` (optional), `endTime` (optional) plus Sortierregel für FR-4 (Programmpunkte ohne Uhrzeit zuerst oder zuletzt). Sortierung ist Kern-Sache (AD-7).

### MEDIUM-4 — Zeitzonendaten im Container und in der DB-Session

**Wo:** Stack, Structural Seed (Multi-Stage-Dockerfile), AD-3/AD-4.
**Problem:** Endet das Multi-Stage-Image auf `scratch` oder `distroless/static`, fehlt `/usr/share/zoneinfo`. Dann schlägt `time.LoadLocation("Europe/Berlin")` fehl, und die gesamte Vorbei-Regel bricht. Die PostgreSQL-Session-Zeitzone (Railway: UTC) betrifft außerdem jede Umwandlung `date`→`timestamptz` in SQL.
**Fix:** Als Convention aufnehmen: `import _ "time/tzdata"` in `cmd/eventstore`, Zeitzone als Kern-Konstante, Start bricht ab, wenn sie nicht lädt. Keine Zeitzonen-Umwandlung in SQL, denn `timestamptz` kommt fertig aus dem Kern (folgt aus AD-2, sollte aber ausdrücklich dastehen).

### MEDIUM-5 — Auto-Deploy ist nicht an grünes CI gekoppelt

**Wo:** Structural Seed (Diagramm: `repo → build` parallel zu `ci`), memlog.
**Problem:** Railway baut bei jedem Push auf `main`, unabhängig vom Ergebnis der GitHub Actions. Ein roter Codegen-Check (AD-8) oder rote Tests verhindern also kein Deployment. Der CI-Check aus AD-8 wird damit für die Produktion wirkungslos.
**Fix:** In Railway „Wait for CI“ (Check Suites) aktivieren oder den Deploy aus der CI per `railway up` nach erfolgreichen Jobs anstoßen. Das Diagramm auf `ci → build` ändern.

### MEDIUM-6 — Parallele Instanzen bei Deploy und Migration

**Wo:** Conventions „Migrationen“, AD-13, Structural Seed.
**Problem:** Railway lässt beim Zero-Downtime-Deploy alte und neue Instanz kurz parallel laufen. Bei mehr als einer Replika laufen Migrationen und Job mehrfach gleichzeitig. Der Job ist idempotent, Migrationen ohne Lock sind es nicht. Außerdem muss jede Migration mit der noch laufenden alten Version verträglich sein.
**Fix:** Festlegen: Replikas = 1, und goose mit Postgres-Advisory-Lock (`lock.NewPostgresSessionLocker`) ausführen. Migrationen sind rückwärtskompatibel zur Vorversion (passt zur Expand/Contract-Regel aus HIGH-4).

### MEDIUM-7 — Semantik von „Aktualisierung“ und „Überschreiben“ im Import offen

**Wo:** AD-10, AD-11; FR-16 bis FR-18.
**Problem:** Offen ist, ob ein Update per `importKey` alle Felder ersetzt (auch einen Ablaufplan oder eine Notiz, die im Admin geändert wurden) oder ob es zusammenführt. Offen ist auch, ob „vorhandenes überschreiben“ bei Duplikatverdacht den `importKey` am bestehenden Event setzt. Davon hängt SM-2 ab: Ein zweiter Import muss 0 neue Events erzeugen. Weitere offene Fälle: Ein mitgebrachter Ort mit vorhandenem Namen, aber anderen Koordinaten (wird ignoriert oder aktualisiert?), und doppelte `importKey`s *innerhalb* einer Datei (lassen sonst die ganze Transaktion am Unique-Index scheitern).
**Fix:** AD-10/AD-11 ergänzen: Update ersetzt alle Felder, die das Import-Format abdeckt, und den Ablaufplan vollständig. „Überschreiben“ übernimmt den `importKey` der Datei. Ein mitgebrachter Ort mit vorhandenem Namen ändert den Ort nicht und zeigt in der Vorschau einen Hinweis, wenn er abweicht. Doppelte Schlüssel in der Datei klassifiziert der Kern als `error`.

### MEDIUM-8 — Auslegung der Filter `from`/`to` nicht dem Kern zugewiesen

**Wo:** AD-7, KON-5.
**Problem:** KON-5 erlaubt Datum *oder* Zeitpunkt, beide inklusive. `to=2026-12-24` muss also bis zum Ende des Tages reichen (exklusiv Folgetag 00:00). AD-7 weist dem Kern nur „heute“ und Sortierung zu. Die Umrechnung der Parameter könnte damit im Adapter landen, und die Archiv-Abfrage (FR-12) könnte sie anders umsetzen.
**Fix:** AD-7 erweitern: Der Adapter reicht `from`/`to` als geparsten Wert mit Genauigkeit (Datum oder Zeitpunkt) weiter. Der Kern rechnet ihn nach derselben Tagesgrenzen-Regel wie AD-4 in `timestamptz` um und prüft die Überschneidung `effectiveStart <= toEnd AND effectiveEnd > from`. Gleiche Funktion für aktive Abfrage und Archiv.

### LOW-1 — `archivedAt` nach Reaktivierung (FR-15)

**Problem:** Wird ein archiviertes Event wieder aktiv, bleibt `archivedAt` gesetzt. Die API ist davon nicht betroffen (AD-5), aber die Statistik wird falsch.
**Fix:** In AD-5 aufnehmen: `SaveEvent` setzt `archivedAt = NULL`, wenn das neue `effectiveEnd > now` ist.

### LOW-2 — AD-11: offene Annahme und Spannung zu AD-2

**Problem:** `[ASSUMPTION: Vergleich ohne Groß-/Kleinschreibung]` ist nicht aufgelöst. Ein eindeutiger Index auf `lower(btrim(name))` wäre ein SQL-Ausdruck mit Fachbedeutung, den AD-2 wörtlich verbietet. Ohne ihn liegt die Eindeutigkeit nur im Kern.
**Fix:** Die Annahme bestätigen oder in eine offene Frage überführen. AD-2 präzisieren: „Integritäts-Constraints (FK, UNIQUE, auch auf normalisierten Spalten) sind erlaubt.“ Am besten speichert der Kern eine Spalte `name_key` mit dem normalisierten Namen, und darauf liegt ein einfacher UNIQUE-Index.

### LOW-3 — AD-12 enthält Details, die nur ein Epic betreffen

**Problem:** Cookie-Flags, bcrypt und die Env-Variablen betreffen nur das Admin-Epic und sind keine Divergenzpunkte. Seit Go 1.25 gibt es `http.CrossOriginProtection` als einfachere Alternative zu CSRF-Tokens.
**Fix:** In AD-12 nur die Trennung `/v1` (GET, offenes CORS) vs. `/admin` (Session, kein CORS) sowie die vollständige Liste der Routen-Namensräume behalten (`/v1`, `/admin`, `/healthz`, `/v1/openapi.yaml`). Den Rest als Seed oder Convention führen und die CSRF-Umsetzung dem Epic überlassen (Hinweis auf `CrossOriginProtection`).

### LOW-4 — AD-13 und AD-5 überschneiden sich

**Problem:** AD-13 betrifft nur ein Epic (Cleanup). Seine Invariante („ändert keine API-Antwort“) steht schon in AD-5. Der Job existiert nur, um FR-13 wörtlich zu erfüllen.
**Fix:** AD-13 in AD-5 aufgehen lassen und den Zeitplan („beim Start + täglich, idempotent“) als Seed führen.

### LOW-5 — Stack-Einträge ungepinnt oder fehlend

**Problem:** htmx „2.x aktuell“ und Leaflet „aktuell“ sind nicht reproduzierbar. Ob sie vendored oder per CDN geladen werden, ist offen. `golang.org/x/crypto/bcrypt` fehlt im Stack. Für die OSM-Kacheln gilt die Tile Usage Policy (Attribution, User-Agent).
**Fix:** Konkrete Versionen pinnen (htmx 2.0.x, Leaflet 1.9.x), Dateien in `internal/adapter/admin/static` vendoren und per `embed` ausliefern. x/crypto in den Stack aufnehmen und die OSM-Attribution als Convention ergänzen.

### LOW-6 — Keine Sektion „Offene Fragen“; Lizenz und KON-2 schweigen

**Problem:** Die `[ASSUMPTION]`-Markierungen (Enum-Codes, AD-11) und die PRD-Frage 1 (Lizenz, blockiert den öffentlichen Start) tauchen nicht als offene Fragen auf. KON-2 (v1 bleibt neben v2 noch 6 Monate erreichbar) hat keine Struktur-Aussage, etwa zur Paketbenennung `publicapi` gegenüber `publicapi/v1`.
**Fix:** Eine Sektion „Open Questions“ ergänzen. Für KON-2 einen Deferred-Eintrag mit Struktur-Hinweis aufnehmen: Paket und Spec je Hauptversion (`api/v1/openapi.yaml`, `adapter/publicapi/v1`). Die Kern-Abfragen bleiben dabei versionsunabhängig.

### LOW-7 — Session-Speicherung nicht festgelegt

**Problem:** `SESSION_SECRET` deutet auf signierte Cookies hin, ausgesprochen ist das nicht. Eine In-Memory-Session meldet den Admin bei jedem Deploy ab.
**Fix:** Als Seed festhalten: zustandslose, signierte Session-Cookies (HMAC mit `SESSION_SECRET`) mit Ablaufzeit.

### LOW-8 — Öffentliche Dokumentation nur als YAML

**Problem:** NFR-3 und SM-4 verlangen eine öffentlich abrufbare, vorlagetaugliche Doku. `/v1/openapi.yaml` ist maschinenlesbar, aber für die Durchsicht durch ein OZ-Mitglied unbequem.
**Fix:** Als Seed eine statische Doku-Seite unter `/v1/docs` ergänzen (z. B. Scalar oder Redoc, eingebettet und vendored), die dieselbe Spec rendert.

### LOW-9 — Capability-Map: NFR-4 schwach zugeordnet

**Problem:** „NFR-4 → AD-6“ stimmt so nicht, denn AD-6 regelt Schreibwege, keine personenbezogenen Daten.
**Fix:** NFR-4 als „Datenmodell ohne Personenfelder. Konvention: keine Freitextfelder für Kontakte, Hinweis in der OpenAPI-Beschreibung“ führen oder ausdrücklich als nicht architekturrelevant markieren.

## Dimensionen-Check

| Dimension | Status |
| --- | --- |
| Paradigma / Modulgrenzen | entschieden (AD-1) |
| Datenmodell & Identität | entschieden, Lücken: MEDIUM-2, MEDIUM-3, MEDIUM-7 |
| API-Vertrag | entschieden, Widerspruch: HIGH-1, HIGH-2 |
| Transaktionen / Konsistenz | **fehlt** (HIGH-3) |
| Zeit / Zeitzone | gut, Lücken: MEDIUM-1, MEDIUM-4, MEDIUM-8 |
| Sicherheit / Auth | entschieden (AD-12) |
| Deployment & Umgebungen | entschieden, Lücken: MEDIUM-5, MEDIUM-6 |
| Infra/Provider-Strategie | entschieden (Railway, PG 18 gepinnt, privates Netz) |
| Betrieb: Sicherung/Wiederherstellung | zurückgestellt, aber mit Risiko (HIGH-4) |
| Betrieb: Monitoring/Logs | entschieden bzw. bewusst zurückgestellt |
| Offene Fragen | **fehlt** als Sektion (LOW-6) |

## Mermaid

Alle drei Diagramme sind gültig. Inhaltlich zeigt das Deployment-Diagramm Build und CI als unabhängige Pfade, was zu MEDIUM-5 passt.
