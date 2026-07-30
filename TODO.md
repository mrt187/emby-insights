# TODO

## UI wahlweise auf Englisch

Sprachumschalter (z. B. in Profil), damit die Oberfläche wahlweise auf
Englisch statt Deutsch angezeigt wird.

- **Aufwand**: kein Architektur-Umbau, aber mechanisch — ca. 150–190 harte
  deutsche String-Literale in `frontend/app/page.tsx` (1254 Zeilen) und
  `frontend/app/login-screen.tsx`. Keine i18n-Bibliothek vorhanden.
  Geschätzt ein Nachmittag bis Tag.
- **Vorarbeiten**: `Period`-Type (`"Woche"|"Monat"|"Jahr"`) von
  Anzeige-Label auf sprachneutrale interne Werte (`"week"|"month"|"year"`)
  umstellen; drei hartcodierte `Intl.DateTimeFormat("de-DE", ...)` /
  `Intl.NumberFormat("de-DE")`-Stellen (page.tsx:58, 85, 1007, 1254) müssen
  sprachabhängig werden; Tageszeit-Buckets ("Nacht"/"Morgen"/"Mittag"/
  "Abend", page.tsx:484-487) und Wochentag-Labels (`weekdayChartData`,
  page.tsx:477) sind ebenfalls hart verdrahtet.
- **Empfohlener Ansatz**: schlanke eigene Lösung statt externer
  i18n-Bibliothek (`frontend/app/i18n.ts` mit `t()`-Helper +
  `Language`-Context, `localStorage`-Persistenz) — reicht für zwei
  Sprachen ohne Pluralregeln. Inhalte aus Emby/TMDB/Seerr und
  Chat-Nachrichten bleiben unübersetzt (Originaldaten).
