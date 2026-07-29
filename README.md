# Emby Insights

Ein persönliches, mobile-first Medien-Dashboard für Emby-Nutzer.

Emby Insights ergänzt den Player, statt ihn zu ersetzen: Es bündelt persönliche Statistiken, eigene Medienanfragen, kommende Veröffentlichungen und Benachrichtigungen in einer ruhigen, hochwertigen Oberfläche.

Die Produktgrundlage und das MVP stehen in [docs/PRODUCT-BLUEPRINT.md](docs/PRODUCT-BLUEPRINT.md). Die technische Richtung steht in [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Status

Current release: [v0.5.0](CHANGELOG.md#050---2026-07-29).

The personal dashboard foundation is implemented: Emby login, protected
sessions, personal watch-time statistics, and the all-in-one Unraid container.
The Upcoming and Requests Home cards now read real ComingSoon and Seerr data;
"Neu für dich" is still a placeholder.

## Releases

Releases use [Semantic Versioning](https://semver.org/). Each Gitea release is
identified by a Git tag in the form `vMAJOR.MINOR.PATCH`. Container images use
the same version tag, the commit SHA, and `latest`; the Unraid deployment
tracks `latest` automatically.
