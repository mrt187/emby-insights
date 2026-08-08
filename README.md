# Emby Insights

![Emby Insights](frontend/public/og.webp)

**English** | [Deutsch](#deutsch)

A personal, mobile-first media dashboard for Emby users. Emby Insights
complements the player instead of replacing it: personal statistics, your own
media requests, upcoming releases, and notifications all in one place.

Current release: [v0.14.1](CHANGELOG.md#0141---2026-08-08)

## Features

- Emby login with protected sessions
- Personal playback statistics: genres, weekdays, watch hours, longest
  sessions, most active days
- Detail view per title with description, cast, rating, and personal status
- Home cards: My Week, Coming Soon, Requests, New For You
- Requests page with discover lists from Seerr and TMDB
- All-in-one Docker container with backend, PostgreSQL, and Redis
- English and German UI, selectable by the admin

Data comes from Emby, Radarr, Sonarr, TMDB, and Seerr.

## Installation

See [docker/all-in-one/README.md](docker/all-in-one/README.md).

The [Emby Insights Plugin](https://github.com/mrt187/emby-insights-plugin/releases/latest)
is also required on the Emby server for statistics.

## Documentation

- [Product foundation and MVP](docs/PRODUCT-BLUEPRINT.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Changelog](CHANGELOG.md)

## Releases

[Semantic Versioning](https://semver.org/) with git tags of the form
`vMAJOR.MINOR.PATCH`. Container images carry the same version, plus the
commit SHA and `latest`.

## License

[MIT](LICENSE).

## Development note

This project was built with AI-assisted programming tools.

---

## Deutsch

[English](#emby-insights) | **Deutsch**

Ein persönliches, mobile-first Medien-Dashboard für Emby-Nutzer. Emby Insights
ergänzt den Player, statt ihn zu ersetzen: persönliche Statistiken, eigene
Medienanfragen, kommende Veröffentlichungen und Benachrichtigungen an einem Ort.

Aktuelles Release: [v0.14.1](CHANGELOG.md#0141---2026-08-08)

### Funktionsumfang

- Emby-Login mit geschützten Sessions
- Persönliche Wiedergabestatistiken: Genres, Wochentage, Wiedergabestunden,
  längste Sessions, aktivste Tage
- Detailansicht je Titel mit Beschreibung, Besetzung, Bewertung und eigenem
  Status
- Home-Karten: Meine Woche, Demnächst, Anfragen, Neu für dich
- Anfragen-Seite mit Discover-Listen aus Seerr und TMDB
- All-in-one-Docker-Container mit Backend, PostgreSQL und Redis
- Deutsche und englische Oberfläche, umschaltbar durch den Admin

Die Daten stammen aus Emby, Radarr, Sonarr, TMDB und Seerr.

### Installation

Anleitung in [docker/all-in-one/README.md](docker/all-in-one/README.md).

Für die Statistiken wird zusätzlich der
[Emby Insights Plugin](https://github.com/mrt187/emby-insights-plugin/releases/latest)
im Emby-Server benötigt.

### Dokumentation

- [Produktgrundlage und MVP](docs/PRODUCT-BLUEPRINT.md)
- [Architektur](docs/ARCHITECTURE.md)
- [Changelog](CHANGELOG.md)

### Releases

[Semantic Versioning](https://semver.org/) mit Git-Tags der Form
`vMAJOR.MINOR.PATCH`. Container-Images tragen dieselbe Version, zusätzlich die
Commit-SHA und `latest`.

### Lizenz

[MIT](LICENSE).

### Entwicklungshinweis

Dieses Projekt wurde mit KI-gestützten Programmierwerkzeugen entwickelt.
