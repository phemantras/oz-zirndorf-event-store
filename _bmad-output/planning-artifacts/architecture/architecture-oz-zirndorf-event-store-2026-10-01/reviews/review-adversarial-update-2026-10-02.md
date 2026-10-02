---
title: 'Adversarial Review (Update) — Architecture Spine OZ Zirndorf Event Store'
reviewed: ARCHITECTURE-SPINE.md (final, updated 2026-10-02, ENT-1..ENT-14 eingearbeitet)
against: epics.md (Stories 1.1–3.5, ENT-1..ENT-14)
date: 2026-10-02
method: 'Paar-Konstruktion: zwei Stories, die jede AD wörtlich einhalten und trotzdem inkompatibel bauen'
verdict: 'BEDINGT BAUREIF — 4 hohe, 6 mittlere, 3 niedrige Löcher; die hohen vor Beginn von Epic 2/3 schließen'
---

# Adversarial Review (Update) — Architecture Spine

## Urteil

**Bedingt baureif.** Die Einarbeitung von ENT-1 bis ENT-14 ist sauber. Die Löcher aus dem ersten Review (Feldform, Duplikat-Policy, Ablaufplan-Eigentum, Stale-Erkennung, DST) sind geschlossen. Neu aufgerissen bzw. jetzt sichtbar sind vor allem **Nahtstellen zwischen `SaveEvent` und `CommitImport`** (Transaktion, `importKey`, „prüfen“-Menge), ein **Widerspruch zwischen AD-16-Standardwerten und FR-8** und die **NFC-Regel, die für den JSON-Import technisch nicht im Adapter umsetzbar ist**, solange der Kern parst. Alle Funde lassen sich mit kurzen Regelzusätzen schließen. Nichts davon braucht neue Infrastruktur.

Schweregrade wie im ersten Review:

- **Hoch** — Zwei Stories werden mit hoher Wahrscheinlichkeit inkompatibel gebaut, oder eine FR/SM wird verletzt.
- **Mittel** — Abweichendes Verhalten zwischen Stories in einem realistischen Fall.
- **Niedrig** — Klarstellung, lokal reparierbar.

## Übersicht

| # | Thema | Paar | Schwere | AD |
| --- | --- | --- | --- | --- |
| U-1 | `GET /v1/events` ohne Parameter: „heute“ oder alles ab heute? | Story 2.3 AC1 × Story 2.3 AC2 / 2.6 (AD-16) | Hoch | AD-16 |
| U-2 | `SaveEvent` öffnet eigene Transaktion, `CommitImport` braucht eine einzige | Story 1.9 × Story 3.3 | Hoch | AD-6, AD-10, AD-16 |
| U-3 | Admin-Bearbeitung löscht `importKey` | Story 1.7 × Story 3.2/3.5 | Hoch | AD-14, AD-10 |
| U-4 | NFC im Adapter, aber der Kern parst die Import-Datei | Story 3.1 × Story 1.4/1.10 | Hoch | AD-11, Conventions |
| U-5 | `unchanged` ohne kanonische Vergleichsform | Story 1.7/1.9 × Story 3.2/3.5 | Mittel | AD-10 |
| U-6 | Feld für mitgebrachten Ort kollidiert mit `Event.location` | Story 2.2 × Story 3.1 | Mittel | AD-14, AD-9 |
| U-7 | „Im Zeitraum“ des Ablaufplans: halboffenes Ende, Punkt ohne Ende | Story 1.9 × Story 1.6 | Mittel | AD-15 |
| U-8 | Schreibpfade außerhalb der AD-6-Liste (Job, Neuberechnung) und Start-Reihenfolge | Story 2.5 × Story 1.7/1.10 | Mittel | AD-6, AD-13, AD-16 |
| U-9 | `importKey`-Konflikt beim Überschreiben erst beim Commit erkennbar | Story 3.2 × Story 3.3 | Mittel | AD-10 |
| U-10 | `endTime` ohne `endDate`: Adapter ergänzt, Import lehnt ab | Story 1.7 × Story 3.1 | Mittel | AD-3, AD-6 |
| U-11 | Client-IP: `RemoteAddr` mit Port, Format von `X-Forwarded-For` | Story 1.3 lokal × Story 1.3 Railway | Niedrig | AD-12 |
| U-12 | Offset von `effective*` in der Ausgabe | Story 2.2 × Story 2.3/2.4 | Niedrig | AD-14 |
| U-13 | Neuberechnung deckt `name_key` nicht ab | Story 1.4 × Story 1.10 | Niedrig | AD-16 |

---

## Hoch

### U-1 — `GET /v1/events` ohne Parameter: „heute“ oder alles ab heute?

**Paar:**
- **Story 2.3, AC 1** (nach FR-8): Ohne Parameter liefert `ListActiveEvents` die aktiven Events, „deren Zeitraum den heutigen Tag berührt“, also nur heute.
- **Story 2.3, AC 2 / Story 2.6 / AD-16 „Standardwerte“** (nach ENT-4, KON-5): Fehlt `from`, gilt der Beginn des heutigen Tages, ein fehlendes `to` ist offen. Ohne beide Parameter ergibt das `[heute 00:00, ∞)`, also **alle** künftigen Events.

Beide Lesarten halten AD-16 wörtlich ein. Die erste setzt FR-8 um, die zweite die Standardwert-Regel. Die Karten-App bekommt je nach Entwickler 3 oder 300 Events. Das ist ein Vertragsbruch der öffentlichen API und wird in der Spec (Story 2.6) festgeschrieben.

**Regel (AD-16, Standardwerte, ersetzen):**
> `GET /v1/events`: Fehlen `from` und `to`, gilt `[Beginn heute, Beginn morgen)` (FR-8). Fehlt nur `to`, ist der Zeitraum nach hinten offen (KON-5). Fehlt nur `from`, gilt der Beginn des heutigen Tages. `GET /v1/archive/events`: Ein fehlendes `from` ist offen, ein fehlendes `to` gilt als `now`.

### U-2 — `SaveEvent` öffnet eine eigene Transaktion, `CommitImport` braucht eine einzige

**Paar:**
- **Story 1.9**: `SaveEvent` ersetzt Event und Ablaufplan „in einer Transaktion über den neuen Port `TxRunner`“. Natürliche Umsetzung: `SaveEvent` ruft selbst `tx.Run(...)`.
- **Story 3.3 / AD-10.5**: `CommitImport` schreibt alle Einträge „in **einer** Transaktion“ über `TxRunner`, „mit denselben Anwendungsfällen“ (AD-6).

Ruft `CommitImport` innerhalb seines `Run` den Anwendungsfall `SaveEvent`, öffnet dieser eine zweite, unabhängige Transaktion (pgx kennt keine verschachtelten Transaktionen ohne Savepoints). Folge: Einträge werden außerhalb der Import-Transaktion festgeschrieben, ein späterer Fehler rollt nur einen Teil zurück, und die AC „scheitert die Transaktion, wird nichts übernommen“ ist verletzt. Baut Story 3.3 dagegen einen eigenen Schreibpfad, umgeht sie `SaveEvent` und verletzt AD-6.

Daran hängt die **„prüfen“-Menge (AD-16)**: „Ein erfolgreiches `SaveEvent` entfernt die ID.“ Wann ist „erfolgreich“? Entfernt `SaveEvent` die ID vor dem Commit der äußeren Import-Transaktion und rollt diese zurück, ist die Markierung weg, obwohl nichts gespeichert wurde. Überschreibt der Import ein markiertes Event über einen eigenen Pfad, bleibt die Markierung stehen. Außerdem nennt der Spine keine Synchronisation: Die Menge wird von HTTP-Goroutinen gelesen und geschrieben.

**Regel (AD-6 ergänzen):**
> Jeder schreibende Anwendungsfall besteht aus einem transaktionsgebundenen Kern (`saveEvent(ctx, repos, …)`) und einer Hülle, die `TxRunner.Run(ctx, func(repos) error)` aufruft. `TxRunner` reicht transaktionsgebundene Repositories herein. `CommitImport` ruft innerhalb **eines** `Run` die transaktionsgebundenen Kerne von `SaveLocation` und `SaveEvent` auf, nie die Hüllen.

**Regel (AD-16, Neuberechnung ergänzen):**
> Die „prüfen“-Menge gehört dem Kern und ist gegen gleichzeitigen Zugriff geschützt (Mutex). Eine ID wird erst **nach dem erfolgreichen Commit** entfernt, und zwar für jedes Event, das `SaveEvent` oder `CommitImport` geschrieben hat. `DeleteEvent` entfernt die ID ebenfalls.

### U-3 — Admin-Bearbeitung löscht den `importKey`

**Paar:**
- **Story 1.7**: Das Admin-Formular baut ein `core.EventInput` und ruft `SaveEvent`. AD-14 sagt: `importKey` gehört „nur Import“ zu `EventInput`. AD-15 legt für den Ablaufplan Ersetzen-Semantik fest. Die naheliegende Umsetzung schreibt alle Spalten aus `EventInput`, also `import_key = NULL`, weil das Formular keinen Schlüssel kennt.
- **Story 3.2 / 3.5**: Ein zweiter Import derselben Datei liefert für jeden Eintrag `unchanged` (SM-2).

Korrigiert der Admin nach dem Import einen Tippfehler im Titel, verliert das Event seinen Schlüssel. Beim nächsten Import wird der Eintrag `duplicateSuspect` oder (nach Titeländerung) sogar `new`, und es entsteht ein stilles Duplikat. AD-10.5 regelt den Erhalt des Schlüssels **nur** für den Import, nicht für das Admin-Formular.

**Regel (AD-14 ergänzen):**
> `SaveEvent` ändert `import_key` nur, wenn der Aufrufer ihn ausdrücklich setzt. Das tut nur `CommitImport`. Ein Speichern ohne `importKey` lässt einen vorhandenen Schlüssel unverändert. Das Admin-Formular zeigt den Schlüssel nur an.

### U-4 — NFC im Adapter, aber der Kern parst die Import-Datei

**Paar:**
- **Story 3.1 / AD-10.1**: „Der Kern parst, validiert und klassifiziert.“ Der Admin-Adapter reicht also Bytes an den Kern.
- **AD-11 / Conventions**: „Die Adapter normalisieren Texteingaben auf Unicode NFC, bevor sie den Kern erreichen.“ Der Kern darf NFC gar nicht selbst, weil `golang.org/x/text/unicode/norm` nicht zur Standardbibliothek gehört (AD-1).

Der Adapter kann einzelne Felder nicht normalisieren, ohne selbst zu parsen. Normalisiert er die ganze Datei als Byte-Strom, bleiben JSON-Escapes wie `"Mühle"` unberührt und werden erst beim Parsen im Kern zu NFD. Folgen: `NormalizeKey("Mühle")` ≠ `NormalizeKey("Mühle")`, zwei Orte mit optisch gleichem Namen trotz eindeutigem `name_key` (Story 1.4), kein Duplikatverdacht (Story 1.10), falsche Umlaut-Sortierung (AD-7). Dasselbe trifft das versteckte Feld mit dem Dateiinhalt beim Commit (Story 3.3), das erneut geparst wird. Hält sich jeder Adapter selbst an „NFC im Adapter“, gibt es zwei unabhängige Implementierungen, die auseinanderlaufen können.

**Regel (AD-11, Texteingaben ersetzen):**
> Der Kern definiert den Port `TextNormalizer` (`func(string) string`). `cmd/eventstore` implementiert ihn mit `golang.org/x/text/unicode/norm.NFC`. Der Kern wendet ihn an **einer** Stelle an: beim Bau von `EventInput` und `LocationInput`, egal ob die Daten aus dem Formular oder aus der geparsten Import-Datei stammen. Adapter normalisieren nicht selbst.

Damit ist die Regel für Formular, Import und Commit identisch, und die Neuberechnung von `title_key` arbeitet auf gespeicherten, schon normalisierten Werten.

---

## Mittel

### U-5 — `unchanged` ohne kanonische Vergleichsform

**Paar:**
- **Story 1.7 / 1.9**: `SaveEvent` speichert, was es bekommt. Offen ist, ob Titel getrimmt, eine leere Notiz als `""` oder `NULL`, ein leerer Link als `""` oder `NULL` gespeichert und der Ablaufplan in Datei- oder in sortierter Reihenfolge abgelegt wird.
- **Story 3.2 / 3.5**: `unchanged` heißt „keine Abweichung“. Story 3.5 verlangt, dass beim zweiten Import **alle** Einträge `unchanged` sind.

Vergleicht der Klassifizierer den rohen Eintrag mit dem gespeicherten Event, ergibt jede kleine Abweichung ein falsches `update`: ein abschließendes Leerzeichen im Titel, `"url": ""` gegenüber `NULL`, ein Ablaufplan in anderer Reihenfolge, ein mitgebrachter Ort (Name) gegenüber der gespeicherten `locationId`. SM-2 scheitert dann, ohne dass eine AD verletzt wäre.

**Regel (AD-10 ergänzen):**
> Der Kern bringt jedes `EventInput` vor dem Speichern in eine kanonische Form (`Canonicalize`): Texte getrimmt, leere optionale Texte und leerer Link als `null`, Ablaufplan in der Lesereihenfolge (Story 1.9), Ort als aufgelöste `locationId`. `SaveEvent` speichert genau diese Form. `unchanged` heißt `Canonicalize(Eintrag) == gespeichertes Event` über die Felder von `EventInput` (ohne IDs der Ablaufplan-Einträge).

### U-6 — Feld für den mitgebrachten Ort kollidiert mit `Event.location`

**Paar:**
- **Story 2.2**: Definiert in `openapi.yaml` die Schreibform `EventInput` mit „mitgebrachtem Ort“ und die Leseform `Event` als „`EventInput` plus `location` (vollständig)“ (AD-14). Naheliegend: `Event = allOf[EventInput, {location: Location, …}]`.
- **Story 3.1**: Der mitgebrachte Ort hat nur `name` als Pflicht, `address`, Koordinaten und `precision` sind optional, es gibt keine `id`.

AD-14 nennt keinen Feldnamen für den mitgebrachten Ort. Heißt er (naheliegend) `location`, hat `Event` zwei widersprüchliche Definitionen desselben Felds: eine teilweise ohne `id` aus `EventInput` und eine vollständige mit `id`. `allOf` ist dann nicht erfüllbar, oder `oapi-codegen` erzeugt einen unbrauchbaren Typ. Außerdem bleibt offen, ob `Event` `locationId` enthält, denn „`EventInput` plus …“ schließt es ein. Story 2.2 listet es nicht auf. Der Abnehmer weiß nicht, ob `locationId` Teil des v1-Vertrags ist. Nachträgliches Entfernen ist ein Bruch nach NFR-2.

**Regel (AD-14 präzisieren):**
> `openapi.yaml` definiert getrennt `Location` (Leseform, alle Felder Pflicht, mit `id`) und `LocationInput` (nur `name` Pflicht, keine `id`). `EventInput` hat genau eines von `locationId` und `newLocation: LocationInput` (nur Import). `Event` ist keine `allOf`-Ableitung von `EventInput`, sondern listet die gemeinsamen Felder selbst, ohne `locationId`, `newLocation` und `importKey`, und mit `location: Location`.

(Den Namen `newLocation` nur als Vorschlag; entscheidend ist, dass er sich von `location` unterscheidet.)

### U-7 — „Im Zeitraum“ des Ablaufplans: halboffenes Ende, Punkt ohne Ende

**Paar:**
- **Story 1.9**: Beginn **und Ende** eines Punkts müssen in `[effectiveStart, effectiveEnd)` liegen.
- **Story 1.6 / AD-4**: Intervalle sind halboffen, ein Event mit Ende 22:00 hat `effectiveEnd = 22:00`.

Ein Programmpunkt „Feuerwerk 21:30–22:00“ eines Events bis 22:00 hat sein Ende genau bei `effectiveEnd`. Im halboffenen Intervall liegt es damit **nicht**, der Punkt wird abgelehnt. Außerdem ist das Ende eines Punkts mit `startTime`, aber ohne `endTime`, nicht definiert. Wendet ein Entwickler die AD-4-Analogie an (Ende = Folgetag 00:00), wird jeder Punkt ohne Ende an einem Abend-Event mit exaktem Ende abgelehnt. Ein anderer Entwickler setzt Ende = Beginn. Undefiniert sind auch `endTime` ohne `startTime` (dort greift die Regel „endTime vor startTime → Folgetag“ nicht) und „Tage des Events“ bei einem Ende genau um Mitternacht.

**Regel (AD-15 ergänzen):**
> Ein Punkt liegt im Zeitraum, wenn `effectiveStart <= Beginn < effectiveEnd` und, falls `endTime` gesetzt ist, `Ende <= effectiveEnd`. Ohne `endTime` wird nur der Beginn geprüft. `endTime` ohne `startTime` wird abgelehnt. Ein Punkt ohne Uhrzeit liegt im Zeitraum, wenn sich sein Tag `[d 00:00, d+1 00:00)` mit `[effectiveStart, effectiveEnd)` überschneidet.

### U-8 — Schreibpfade außerhalb der AD-6-Liste und Start-Reihenfolge

**Paar:**
- **Story 2.5**: Der Job setzt `archived_at` „über einen Kern-Anwendungsfall“ mit einer einzigen bedingten SQL-Anweisung.
- **Story 1.7 / 1.10 / AD-16**: Die Neuberechnung schreibt beim Start `effective*` und `title_key` für alle Events.
- **AD-6**: Geschrieben wird „ausschließlich“ über `SaveEvent`, `DeleteEvent`, `SaveLocation`, `DeleteLocation`, `CommitImport`. Repository-Ports bieten keine Schreibmethode, die den Kern umgeht.

Wer AD-6 wörtlich nimmt, schickt Job und Neuberechnung durch `SaveEvent`. Dann validiert der Job jedes Event erneut, scheitert an genau den markierten Events, und `SaveEvent` leert `archived_at` wieder (AD-5). Wer es nicht wörtlich nimmt, baut zwei zusätzliche Repository-Methoden, die ein Reviewer nach AD-6 ablehnen müsste. Die Reihenfolge beim Start ist auch offen: Läuft der Job vor der Neuberechnung, markiert er auf Basis alter Werte. Die Neuberechnung leert `archived_at` nicht, weil nur `SaveEvent` das tut. Das ist nur Statistik, aber genau die Unschärfe, die AD-5 vermeiden will.

**Regel (AD-6 ergänzen):**
> Zusätzlich gibt es zwei interne Anwendungsfälle, die nur abgeleitete Spalten schreiben: `RecomputeDerived` (beim Start: `effective_start`, `effective_end`, `title_key`, `archived_at` leeren, wenn wieder aktiv) und `MarkArchived` (Job: eine bedingte Anweisung mit `now` aus `Clock`). Ihre Repository-Methoden ändern keine eingegebenen Felder.

**Regel (AD-13 ergänzen):**
> Reihenfolge beim Start: Migrationen → `RecomputeDerived` → erster Lauf von `MarkArchived` → HTTP-Server.

### U-9 — `importKey`-Konflikt beim Überschreiben erst beim Commit erkennbar

**Paar:**
- **Story 3.2**: Klassifiziert einen Eintrag ohne passenden Schlüssel als `duplicateSuspect` mit Kandidaten. Der Viewer bietet „vorhandenes überschreiben“ für jeden Kandidaten an.
- **Story 3.3 / AD-10.5**: „Hat das Ziel-Event einen anderen Schlüssel als der Eintrag, ist der Eintrag `error`.“ Das wird erst beim Commit festgestellt.

Der Admin wählt beim Kandidaten mit eigenem Schlüssel „überschreiben“ und bekommt nach dem Speichern ein `error`, das er beim Upload hätte sehen können. Der Vorrang „`error` vor … `duplicateSuspect`“ ist damit beim Upload nicht vollständig anwendbar, und die Zahlen der Vorschau weichen von der Zusammenfassung ab.

**Regel (AD-10.1 ergänzen):**
> Der Klassifizierer markiert beim Upload jeden Kandidaten, dessen `importKey` gesetzt ist und vom Schlüssel des Eintrags abweicht, als „nicht überschreibbar“. Der Viewer bietet für ihn nur „überspringen“ und „als neues anlegen“ an. Die Prüfung beim Commit bleibt als Schutz gegen veraltete Stände bestehen und meldet dann `stale`, nicht `error`.

### U-10 — `endTime` ohne `endDate`: Adapter ergänzt, Import lehnt ab

**Paar:**
- **Story 1.6**: Der Kern lehnt `endTime` ohne `endDate` ab. AD-3 sagt das nicht.
- **Story 1.7**: Das Formular hat getrennte Felder. Die häufigste Eingabe ist „24.12., 18:00 bis 22:00“ ohne End-Datum. Ein Entwickler setzt aus Bequemlichkeit im Adapter `endDate := startDate`.
- **Story 3.1**: Dieselbe Angabe in der Import-Datei wird abgelehnt.

Dieselbe fachliche Eingabe ist im Admin gültig und im Import fehlerhaft. Genau das soll AD-6 verhindern. Der Spine verbietet Adaptern nicht, Felder zu ergänzen.

**Regel (AD-3 ergänzen):**
> `endTime` ohne `endDate` wird abgelehnt. Adapter ergänzen keine fachlichen Felder. Eine Vorbelegung im Formular ist erlaubt, wenn sie sichtbar im Feld steht, bevor gespeichert wird.

---

## Niedrig

### U-11 — Client-IP: `RemoteAddr` mit Port, Format von `X-Forwarded-For`

**Paar:** Story 1.3 lokal (ohne Proxy, Rückfall `RemoteAddr`) gegenüber Story 1.3 auf Railway (`X-Forwarded-For`). `RemoteAddr` ist `ip:port`. Wird es roh als Schlüssel genutzt, hat jede Verbindung einen neuen Zähler, und die Sperre greift lokal nie. Der Test zur Sperre ist dann vom Testaufbau abhängig. Der erste Eintrag von `X-Forwarded-For` kann Leerraum, einen Port oder Müll (`unknown`) enthalten.

**Regel (AD-12, Anmeldeschutz ergänzen):**
> Die Client-IP wird mit `netip.ParseAddr` nach `strings.TrimSpace` gelesen, bei `RemoteAddr` nach `net.SplitHostPort`. Ist der Eintrag aus `X-Forwarded-For` ungültig, gilt `RemoteAddr`.

Hinweis ohne Regelbedarf: Hängt Railways Proxy die echte IP **an** `X-Forwarded-For` an, kontrolliert der Client den ersten Eintrag. Die Sperre lässt sich dann durch wechselnde Header umgehen. Das ist laut AD-12 hingenommen. Robuster wäre der **letzte** Eintrag, falls Railway genau einen Proxy-Hop einfügt. Das sollte beim Bau einmal geprüft werden.

### U-12 — Offset von `effective*` in der Ausgabe

**Paar:** Story 2.2 (Einzelabruf) und Story 2.3/2.4 (Listen) bilden `time.Time` aus pgx ab. pgx liefert `timestamptz` je nach Verbindung in UTC. „ISO 8601 mit Offset“ ist mit `Z` wie mit `+01:00` erfüllt. Zwei Handler oder zwei Abnehmer-Erwartungen können auseinanderlaufen. Für die Vorlage-Funktion (SM-4) ist das unschön.

**Regel (AD-14 präzisieren):**
> `effectiveStart`/`effectiveEnd` werden in Europe/Berlin mit numerischem Offset und Sekunden ausgegeben (`2026-12-24T18:00:00+01:00`). Der Kern liefert die Zeitpunkte bereits in dieser Zone.

### U-13 — Neuberechnung deckt `name_key` nicht ab

**Paar:** Story 1.10 nimmt `title_key` in die Neuberechnung auf. Story 1.4 speichert `name_key` für Orte, wird aber von keiner Neuberechnung erfasst. Ändert sich `NormalizeKey` später (z. B. weitere Leerraumzeichen), vergleicht der Kern neue Schlüssel mit alten gespeicherten. Die Eindeutigkeitsprüfung in `SaveLocation` und die Zuordnung mitgebrachter Orte beim Import (FR-16) laufen dann auseinander.

**Regel (AD-16, Neuberechnung ergänzen):**
> Die Neuberechnung umfasst alle abgeleiteten Spalten, auch `locations.name_key`. Kollidiert ein neuer `name_key` mit einem vorhandenen, bleibt der alte Wert, der Fehler wird mit der Orts-Kennung geloggt, und der Ort kommt in die „prüfen“-Menge.

---

## Geprüft und ohne Befund

- **ENT-1 gegen DST:** Story 1.6 deckt den Herbstfall mit eigener Meldung ab. Kein Widerspruch zu AD-4/AD-16.
- **`to` auf die Minute abgeschnitten gegen reines Datum:** Beide Pfade liefern ein `hi` auf einer Minutengrenze. `effective*` liegen immer auf vollen Minuten. Kein Randfall zwischen Datum und Zeitpunkt; Story 2.3 regelt `lo >= hi` nach der Normalisierung.
- **Quelle als Objekt (AD-14) gegen AD-9:** Das Import-Schema übernimmt `source` per `$ref`. Das Objekt hat eine einzige Quelle. Offen ist nur `""` gegen `null` bei `url`, das ist in U-5 enthalten.
- **`importKey`-Eindeutigkeit gegen ENT-12:** Ein Eintrag mit Schlüssel `K`, der überschreibt, kann `K` nur tragen, wenn `K` noch nicht im Bestand ist (sonst wäre er `update`). Doppelte Schlüssel in der Datei sind `error`. Die Eindeutigkeit bleibt gewahrt. Zwei Einträge mit demselben Ziel regelt Story 3.3; diese Regel sollte bei Gelegenheit in AD-10 wandern.
