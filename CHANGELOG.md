# Changelog

All notable changes to Emby Insights are documented in this file.

The project follows [Semantic Versioning](https://semver.org/):

- **MAJOR** releases contain incompatible changes.
- **MINOR** releases add backwards-compatible functionality.
- **PATCH** releases contain backwards-compatible fixes.

## [0.2.0] - 2026-07-29

### Changed

- Improved dashboard accessibility and mobile interaction: keyboard navigation,
  visible focus states, a skip link, semantic notifications, and safe-area
  support.
- Improved login form semantics, dark theme metadata, image layout stability,
  and localized statistics formatting.

## [0.1.0] - 2026-07-29

### Added

- First versioned release of the personal Emby Insights dashboard.
- Emby authentication with Redis-backed sessions.
- Personal watch-time statistics from the read-only Emby Insights connector.
- PostgreSQL and Redis readiness checks in the all-in-one Unraid container.

### Changed

- Container images are published with the matching release version, the commit
  SHA, and `latest`.

[0.2.0]: https://registry.example.com/Homelab/emby-insights/releases/tag/v0.2.0
[0.1.0]: https://registry.example.com/Homelab/emby-insights/releases/tag/v0.1.0
