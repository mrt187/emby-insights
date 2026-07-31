# Emby Insights

Ein persönliches, mobile-first Medien-Dashboard für Emby-Nutzer.

Emby Insights ergänzt den Player, statt ihn zu ersetzen: Es bündelt persönliche Statistiken, eigene Medienanfragen, kommende Veröffentlichungen und Benachrichtigungen in einer ruhigen, hochwertigen Oberfläche.

Die Produktgrundlage und das MVP stehen in [docs/PRODUCT-BLUEPRINT.md](docs/PRODUCT-BLUEPRINT.md). Die technische Richtung steht in [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Status

Current release: [v0.8.70](CHANGELOG.md#0870---2026-07-31).

The personal dashboard foundation is implemented: Emby login, protected
sessions, personal watch-time statistics, and the all-in-one Unraid container.
Clicking any poster opens a detail screen (description, cast, rating,
personal status). All four Home cards (My Week, Upcoming, Requests, Neu für
dich) read real data
from Emby, Radarr, Sonarr, TMDB and Seerr, the Statistics page adds continue watching,
watched movies/series, and genre/weekday charts, and the Requests page adds
five Seerr/TMDB discover lists.

## Releases

Releases use [Semantic Versioning](https://semver.org/). Each Gitea release is
identified by a Git tag in the form `vMAJOR.MINOR.PATCH`. Container images use
the same version tag, the commit SHA, and `latest`; the Unraid deployment
tracks `latest` automatically.
