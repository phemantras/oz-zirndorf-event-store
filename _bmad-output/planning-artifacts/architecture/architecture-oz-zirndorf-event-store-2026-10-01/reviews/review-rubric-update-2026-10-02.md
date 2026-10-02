# Rubrik-Review — Spine-Update ENT-1 bis ENT-14 (2026-10-02)

**Gegenstand:** `ARCHITECTURE-SPINE.md` (updated 2026-10-02), Abgleich mit `epics.md` (ENT-1..ENT-14, Stories), `prd.md` und `.memlog.md`.

**Urteil:** Mit Nacharbeit freigeben. Alle 14 ENT-Festlegungen sind sinngetreu und an der richtigen AD gelandet, die AD-IDs sind stabil. Ein echter Widerspruch zum PRD (Standardabfrage ohne Filter) und eine Lücke in der abschließenden Schreibliste von AD-6 müssen vor dem Bau korrigiert werden. Der Rest sind Präzisierungen.

## Rubrik-Ergebnis

| Kriterium | Ergebnis |
| --- | --- |
| (1) ENT-1..14 sinngetreu übernommen | Erfüllt. ENT-4 ist wörtlich übernommen, erbt aber einen Konflikt mit FR-8 (siehe H-1). ENT-5 und ENT-7 sind über den ENT-Text hinaus präzisiert (In-Memory-Markierung, linker `X-Forwarded-For`-Eintrag), das ist im Memlog begründet. |
| (2) Rules durchsetzbar und verhindern ihre Divergenz | Überwiegend. Lücken: AD-6-Liste nicht abschließend (H-2), `/v1/docs` & Co. vs. „Spec ist einzige Quelle“ (M-2), `title_key` nicht verankert (M-4). |
| (3) Keine Widersprüche zu ADs, PRD, Stories | Ein Widerspruch zu FR-8/Story 2.3 (H-1). Kleinere Abweichungen zu Story 1.6/3.3 (M-3, M-5). |
| (4) Deferred lässt keine Divergenz zu | Fast. Die Annahme „eine Instanz“ für In-Memory-Zustand fehlt (M-6). „Weitere Anwendungsfälle legen die Stories fest“ kollidiert mit der abschließenden Liste in AD-6 (H-2). |
| (5) Knapp, Entscheidungen statt Begründung | Weitgehend. Einige Begründungssätze sind hinzugekommen (L-1). |

## Findings

### Critical

Keine.

### High

**H-1 — Standardwerte in AD-16 widersprechen FR-8 bei einer Abfrage ganz ohne Filter**
- *Stelle:* AD-16, „Standardwerte“ (aus ENT-4).
- *Befund:* Die Regel lautet: Fehlt `from`, gilt der Beginn des heutigen Tages, und ein fehlendes `to` ist offen. Ohne Parameter ergibt das `[heute 00:00, ∞)`, also **alle** künftigen aktiven Events. FR-8 („alle aktiven Events, deren Zeitraum den heutigen Tag berührt“), UJ-2 und Story 2.3 AC 1 verlangen dagegen nur den heutigen Tag. Ein Entwickler, der AD-16 umsetzt, liefert für `GET /v1/events` die komplette Zukunft, ein anderer, der Story 2.3 umsetzt, nur heute. Der Fehler steckt schon in ENT-4, ist aber durch die Übernahme jetzt bindend.
- *Fix:* AD-16 „Standardwerte“ dreiteilen: „`GET /v1/events`: Fehlen `from` **und** `to`, gilt der heutige Tag `[heute, morgen)` (FR-8). Fehlt nur `from`, gilt der Beginn des heutigen Tages. Fehlt nur `to`, ist der Zeitraum offen (KON-5).“ ENT-4 in `epics.md` entsprechend korrigieren und in Story 2.3 einen Akzeptanztest „nur `from` angegeben → offen nach hinten“ ergänzen.

**H-2 — AD-6 nennt die Schreib-Anwendungsfälle abschließend, aber Neuberechnung und Bereinigung schreiben auch**
- *Stelle:* AD-6 („ausschließlich über … `SaveEvent`, `DeleteEvent`, `SaveLocation`, `DeleteLocation`, `CommitImport`“), AD-13, AD-16 „Neuberechnung“, Deferred „Interne Kern-Struktur“.
- *Befund:* Die Neuberechnung beim Start schreibt `effective*` (und laut Story 1.10 `title_key`) für alle Events. Der Bereinigungsjob setzt `archivedAt`, laut Story 2.5 „über einen Kern-Anwendungsfall“. Keiner der beiden steht in der Liste. Ein Entwickler baut dafür eine Repository-Methode, die am Kern vorbei schreibt (verboten nach AD-6), ein anderer einen neuen Anwendungsfall (nach Deferred erlaubt, nach AD-6 verboten). Durch ENT-5 (Fehler-Menge, Entfernen bei `SaveEvent`) ist die Neuberechnung jetzt fachlich relevanter.
- *Fix:* In AD-6 die Liste um `RecomputeDerived` (Neuberechnung beim Start, AD-16) und `MarkArchived` (Bereinigung, AD-13) ergänzen und ausdrücklich als abschließend markieren. Deferred „Interne Kern-Struktur“ auf „weitere **Lese**-Anwendungsfälle“ einschränken oder „neue Schreib-Anwendungsfälle erfordern ein Spine-Update“ ergänzen.

### Medium

**M-1 — Unicode-NFC braucht `golang.org/x/text`, das fehlt im Stack**
- *Stelle:* AD-11 „Texteingaben“, Convention „Texteingaben“, Stack.
- *Befund:* Die Standardbibliothek hat keine NFC-Normalisierung. Die Adapter brauchen `golang.org/x/text/unicode/norm`. Ohne Pin wählt jeder Adapter seine Version, oder einer verzichtet ganz. Die Zuordnung zu den Adaptern ist mit AD-1 konsistent (der Kern darf das Paket nicht importieren), sollte aber ausdrücklich so stehen.
- *Fix:* Stack-Zeile `golang.org/x/text (unicode/norm) | <aktuelle Version>` ergänzen. In AD-11 einen Halbsatz ergänzen: „über eine gemeinsame Hilfsfunktion je Adapter, nicht im Kern (AD-1)“. Den Satz mit dem Architekturtest aus Story 1.1 abgleichen.

**M-2 — `/v1/docs`, `/v1/openapi.yaml` und `/v1/import-v1.schema.json` stehen neben der Regel „Spec ist einzige Quelle für Pfade“**
- *Stelle:* AD-8.
- *Befund:* AD-8 sagt, `openapi.yaml` sei die einzige Quelle für die Pfade von v1. Mit ENT-14 gibt es drei Pfade unter `/v1/…`, die keine JSON-Ressourcen sind. Offen bleibt: Stehen sie in der Spec, laufen sie über das generierte Gerüst oder über eigene Handler, und gelten für sie CORS und Problem Details? Zwei Umsetzungen können hier verschieden ausfallen, etwa `/v1/docs` mit 404 als HTML statt als Problem Details.
- *Fix:* In AD-8 ergänzen: „`/v1/docs`, `/v1/openapi.yaml` und `/v1/import-v1.schema.json` sind statische Auslieferungen, stehen nicht in der Spec und werden im Adapter `publicapi/v1` neben dem generierten Gerüst registriert. Sie haben dasselbe offene CORS wie die API.“

**M-3 — `endTime` ohne `endDate` ist in AD-3/AD-4 nicht geregelt**
- *Stelle:* AD-3, AD-4 (Story 1.6 lehnt das ab).
- *Befund:* AD-4 behandelt „Ende ohne Uhrzeit“ und „kein Ende“. Ein Ende, das nur eine Uhrzeit hat, kommt nicht vor. Ein Entwickler interpretiert es als `endDate = startDate`, Story 1.6 lehnt es ab. Weil Import und Admin denselben Kern nutzen, ist das Risiko geringer, aber Spec und Import-Schema können es unterschiedlich dokumentieren.
- *Fix:* In AD-3 ergänzen: „`endTime` ohne `endDate` wird abgelehnt.“ Optional auch die Ableitung der Ende-Genauigkeit präzisieren: „ohne Ende ist `endPrecision` `null`, auch bei `allDay`“ (Story 1.6 ist hier mehrdeutig).

**M-4 — Duplikatprüfung ohne verankerte Spalte `title_key`**
- *Stelle:* AD-11 „Duplikatprüfung“, AD-2, Structural Seed.
- *Befund:* AD-2 verbietet `lower()`/`trim()` in SQL. Die Duplikatprüfung braucht also entweder eine gespeicherte Spalte `title_key` (Story 1.10) oder lädt alle Events in den Kern. Der Spine legt das nicht fest, und `title_key` fehlt im ER-Diagramm. Zwei Umsetzungen (Import-Klassifizierung und Admin-Warnung) können verschiedene Wege gehen.
- *Fix:* In AD-11 ergänzen: „Der Kern speichert `NormalizeKey(title)` als Spalte `title_key`. Kandidaten werden per Gleichheit auf `title_key`, `start_date` und `location_id` gesucht.“ `title_key` und `name_key` im ER-Diagramm aufführen.

**M-5 — Zwei Einträge mit demselben Ziel-Event fehlen in AD-10**
- *Stelle:* AD-10 Schritt 2 und 5 (Story 3.3 macht beide zu `error`).
- *Befund:* AD-10 regelt den doppelten `importKey` innerhalb der Datei, aber nicht den Fall, dass zwei Entscheidungen („überschreiben“ oder `update`) dasselbe Ziel treffen. Ohne Regel gewinnt in der Transaktion der zuletzt geschriebene Eintrag, und die Zusammenfassung zählt zwei Aktualisierungen. Außerdem fehlt „ein als neu geführter Ort existiert inzwischen → `stale`“ (Story 3.3).
- *Fix:* In AD-10 Schritt 4 oder 5 ergänzen: „Zielen mehrere Einträge auf dasselbe Event, werden alle als `error` aussortiert. Existiert ein als `newLocation` geführter Ort inzwischen, sind die betroffenen Einträge `stale`.“

**M-6 — In-Memory-Zustand setzt eine Instanz voraus, das steht nirgends**
- *Stelle:* AD-12 „Anmeldeschutz“, AD-16 „Neuberechnung“ (neue Markierung „prüfen“), AD-13.
- *Befund:* Fehlversuchszähler und die Menge der „prüfen“-Events liegen nur im Speicher. Bei zwei Railway-Replikas hat jede ihre eigenen Zähler und ihre eigene Menge, und der Bereinigungsjob läuft doppelt (das ist idempotent und unkritisch). Die Markierung „prüfen“ würde je nach Replika erscheinen oder fehlen.
- *Fix:* Unter Structural Seed / Railway ergänzen: „Der App-Service läuft mit genau einer Replika.“ Unter Deferred: „Mehrere Replikas: erfordert geteilten Zustand für Anmeldeschutz und ‚prüfen‘-Markierung.“

**M-7 — Linker `X-Forwarded-For`-Eintrag macht die IP-Sperre umgehbar, nicht nur fälschbar**
- *Stelle:* AD-12 „Anmeldeschutz“.
- *Befund:* Wenn Railways Edge einen vom Client gesendeten `X-Forwarded-For` nicht ersetzt, sondern ergänzt, wählt der Angreifer den linken Eintrag selbst. Mit einer neuen Fantasie-IP pro Versuch läuft er nie in die Sperre. Mit der IP des Admins kann er umgekehrt den Admin 15 Minuten aussperren. „Die Sperre bremst nur“ trifft dann nicht zu. Der Memlog stützt sich auf eine Railway-Empfehlung, ohne zu klären, ob die Edge den Header überschreibt.
- *Fix:* Vor dem Bau von Story 1.3 einmal prüfen, ob Railway einen vom Client gesendeten `X-Forwarded-For` überschreibt. Wenn nicht: den **rechten** Eintrag nehmen (den die Edge angehängt hat) oder zusätzlich einen globalen Zähler mit Verzögerung pro Fehlversuch einführen, ohne Sperre. Das Ergebnis als eine Zeile in AD-12 festhalten.

### Low

**L-1 — Begründungssätze im Spine**
- *Stellen:* AD-12 („das ist bewusst hingenommen“, „Die Sperre bremst nur, geschützt wird über bcrypt“), AD-16 („So greifen Änderungen an den Regeln sofort“), AD-10 Schritt 5 („weil die Klassifizierung schon entschieden ist“).
- *Fix:* Diese Halbsätze streichen. Die Begründung steht schon im Memlog. Die Entscheidung bleibt („Ein kopiertes Cookie bleibt bis zum Ablauf gültig.“).

**L-2 — Kern-Abfrage für die „prüfen“-Menge ohne Namen; `DeleteEvent` nicht erwähnt**
- *Stelle:* AD-16 „Neuberechnung“.
- *Fix:* Die Abfrage benennen (z. B. `ListRecomputeFailures`) und ergänzen: „`DeleteEvent` entfernt die ID ebenfalls.“

**L-3 — AD-12 „nur lesend (`GET`)“ vs. Story 2.1 (`GET`, `HEAD`, `OPTIONS`)**
- *Fix:* In AD-12 „nur lesend (`GET`/`HEAD`, dazu `OPTIONS` für CORS-Preflight)“ schreiben.

**L-4 — Standardwert `to = now` im Archiv: gilt die Minuten-Aufrundung?**
- *Stelle:* AD-16 „Standardwerte“.
- *Fix:* Klarstellen: „Der Standardwert `now` wird nicht gerundet: `hi = now`.“

**L-5 — Sortierung bei Gleichstand im Archiv: `id` auf- oder absteigend?**
- *Stelle:* AD-7.
- *Fix:* „bei Gleichstand nach `id` aufsteigend“ (gilt für beide Listen).

**L-6 — ENT-12-Fehler steht im Text nach „vor der Transaktion aussortiert“**
- *Stelle:* AD-10 Schritt 5.
- *Befund:* „Hat das Ziel-Event einen anderen Schlüssel als der Eintrag, ist der Eintrag `error`“ steht nach dem Satz über die Transaktion. Daraus lässt sich ein Abbruch innerhalb der Transaktion lesen.
- *Fix:* Den Satz vor „Alle übrigen schreibt der Kern …“ stellen. Ergänzen: „Hat das Ziel-Event keinen Schlüssel, erhält es den des Eintrags.“

**L-7 — Größenangabe 2 MB und Ort der Prüfung**
- *Fix:* „höchstens 2 MiB, geprüft im Admin-Adapter vor dem Parsen (`http.MaxBytesReader`)“.

**L-8 — `epics.md` ist nach dem Update veraltet**
- *Befund:* Zeile 15 („gelten, bis der Spine nachgezogen ist“) und das Requirements Inventory (AD-10 ohne `unchanged`, AD-11 „NormalizeKey (trim, lowercase)“) entsprechen nicht mehr dem Spine.
- *Fix:* Den Abschnitt ENT-1..14 als „in den Spine übernommen am 2026-10-02“ kennzeichnen und die Zusammenfassungen von AD-10, AD-11 und AD-16 im Inventory angleichen. H-1 auch in ENT-4 korrigieren.

## Prüfung ENT → Spine im Einzelnen

| ENT | Ziel | Befund |
| --- | --- | --- |
| ENT-1 | AD-4 | treu |
| ENT-2 | AD-3 | treu |
| ENT-3 | AD-15 | treu |
| ENT-4 | AD-16 | treu, aber im Konflikt mit FR-8 (H-1), L-4 |
| ENT-5 | AD-16 | treu und präzisiert (nur im Speicher), L-2, M-6 |
| ENT-6 | AD-12 | treu |
| ENT-7 | AD-12 | treu und präzisiert (Client-IP), M-7 |
| ENT-8 | AD-7 | treu |
| ENT-9 | AD-11 + Convention | treu, M-1 |
| ENT-10 | AD-14 | treu |
| ENT-11 | AD-10 | treu, M-5 |
| ENT-12 | AD-10, AD-11 | treu, L-6 |
| ENT-13 | AD-10 | treu, L-7 |
| ENT-14 | AD-8, Stack, Convention | treu, M-2 |
