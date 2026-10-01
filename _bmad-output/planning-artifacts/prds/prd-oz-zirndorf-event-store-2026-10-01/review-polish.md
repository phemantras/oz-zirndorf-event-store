---
title: "Review-Polish: PRD und Addendum OZ Zirndorf Event Store"
created: 2026-10-01
---

# Review-Polish (bmad-review, Linsen: structure, prose)

Leser: Menschen (Andreas, spätere Architektur- und Story-Arbeit). Strukturmodell: Strategic/Pyramid. Inhalte, IDs, Zahlen und Bedeutung sind unverändert.

## prd.md (21 Änderungen)

**Struktur**
- §0: Redundanten Satz zu Annahmen (doppelt mit §12) gestrichen; §12 bleibt die einzige Stelle.
- §4.3: „Out of Scope“ als „Out of Scope (gesamte Lese-API)“ gekennzeichnet, damit es nicht wie ein Teil von FR-11 wirkt.
- §4.5: Zwei Sätze zur Admin-Oberfläche zu einem zusammengefasst.

**Prosa**
- Glossar-Konsistenz: „Dubletten“ → „Duplikate“ (JTBD Admin, UJ-1); „2 Mal Verdacht auf Duplikat“ → „zweimal Duplikatverdacht“.
- Englische Marker vereinheitlicht: „Edge case“ → „Sonderfall“, „Notes“ → „Hinweis“.
- FR-6: „Pflicht“ → „Pflichtangaben“ (wie FR-1); Ortsgenauigkeits-Wert „Platz oder Straße“ → „Platz/Straße“ (wie Glossar).
- FR-2: „über das leere Ende“ → „durch ein leeres Ende“.
- FR-12: „vor einer Minute vorbei“ → „seit einer Minute vorbei“; Satz zu „genau einem der beiden Zugriffe“ grammatisch geglättet.
- FR-16: Genitiv „einschließlich des optionalen Import-Schlüssels“.
- FR-17: Mehrdeutigen Relativsatz aufgelöst („Ein Event, dessen Import-Schlüssel schon im Bestand vorkommt“).
- §1: Doppeltes „und … und durch“ aufgelöst.
- NFR-2: „zum Beispiel“ → „z. B.“; NFR-3: „Doku“ → „Dokumentation“.
- KON-5: „Namensregel wie KON-7“ → „englisch benannt gemäß KON-7“; KON-6: „(das Jüngste zuerst)“ → „(neuestes zuerst)“.
- SM-C1: Kongruenz („Mehr Events … ist“ → „Ein höherer Anteil von Events … ist“); SM-C2: „weniger, dafür gepflegte Events“.

## addendum.md (8 Änderungen)

- Technologie: „frei wählbar (OZ-Prinzip)“.
- Vorbei-Regel: vollständiger Satz („Ein Event ist vorbei, wenn sein Ende überschritten ist.“).
- Ressourcen: Namensherkunft auf PRD KON-7 verwiesen statt umschrieben.
- Kartenpicker: „Die konkrete Bibliothek wählt die Architektur.“
- API-Konventionen: zeitliches „inzwischen“ entfernt.
- Testsammlung: „archiviert bzw. ausgeblendet“ → „gehören diese also sofort zum Archiv“ (Glossar-Begriff).
- „rund 20 Mal“ → „rund 20-mal“.
- Verworfene Alternative: „ohne IDs“ → „ohne Import-Schlüssel“ (Glossar-Begriff).

## Übersprungen (menschliche Entscheidung nötig)

- **UJ-1 widerspricht FR-17:** Der zweite Duplikatverdacht ist „eine Stadtratssitzung an einem anderen Tag“. Laut FR-17 ist gleicher Titel an einem anderen Datum aber gerade *kein* Duplikatverdacht. Das Beispiel sollte angepasst werden (z. B. anderes Event am selben Tag und Ort mit gleichem Titel).
- **Wiederholung der Vorbei-/Bereinigungsregel** (Glossar „Archiv“, FR-8, FR-11, §4.4, FR-13) und **FR-18 vs. SM-2** (Testsammlung 42 Events): bewusst nicht gekürzt, da es testbare Konsequenzen sind.
- **Fehlende UJ-Rückverweise** bei FR-4, FR-5, FR-7: nicht ergänzt, da das die Traceability inhaltlich ändert.
