# Emby Insights

Ein persönliches, mobile-first Medien-Dashboard für Emby-Nutzer. Emby Insights
ergänzt den Player, statt ihn zu ersetzen: persönliche Statistiken, eigene
Medienanfragen, kommende Veröffentlichungen und Benachrichtigungen an einem Ort.

Aktuelles Release: [v0.9.5](CHANGELOG.md#095---2026-08-02)

## Funktionsumfang

- Emby-Login mit geschützten Sessions
- Persönliche Wiedergabestatistiken: Genres, Wochentage, Wiedergabestunden,
  längste Sessions, aktivste Tage
- Detailansicht je Titel mit Beschreibung, Besetzung, Bewertung und eigenem
  Status
- Home-Karten: Meine Woche, Demnächst, Anfragen, Neu für dich
- Anfragen-Seite mit Discover-Listen aus Seerr und TMDB
- All-in-one-Docker-Container mit Backend, PostgreSQL und Redis

Die Daten stammen aus Emby, Radarr, Sonarr, TMDB und Seerr.

## Installation

Anleitung in [docker/all-in-one/README.md](docker/all-in-one/README.md).

Für die Statistiken wird zusätzlich der
[Emby Insights Connector](https://gitea.homexxlab.de/Homelab/emby-insights-plugin)
im Emby-Server benötigt.

## Dokumentation

- [Produktgrundlage und MVP](docs/PRODUCT-BLUEPRINT.md)
- [Architektur](docs/ARCHITECTURE.md)
- [Changelog](CHANGELOG.md)

## Releases

[Semantic Versioning](https://semver.org/) mit Git-Tags der Form
`vMAJOR.MINOR.PATCH`. Container-Images tragen dieselbe Version, zusätzlich die
Commit-SHA und `latest`.

## Lizenz

[MIT](LICENSE).

## Entwicklungshinweis

Dieses Projekt wurde mit KI-gestützten Programmierwerkzeugen entwickelt.
