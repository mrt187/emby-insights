# Changelog

All notable changes to Emby Insights are documented in this file.

The project follows [Semantic Versioning](https://semver.org/):

- **MAJOR** releases contain incompatible changes.
- **MINOR** releases add backwards-compatible functionality.
- **PATCH** releases contain backwards-compatible fixes.

## [0.3.2] - 2026-07-29

### Added

- Automatic Unraid release updater: tagged Gitea releases are built into
  versioned, commit-specific, and `latest` container images, then deployed to
  the Emby Insights stack.

## [0.3.1] - 2026-07-29

### Changed

- Replaced the separate weekly metric cards with one bold, personal profile
  card featuring the Emby avatar, watch time, and key media insights.
- Refined dashboard typography and charcoal panel styling to improve hierarchy
  and match the selected visual reference.

## [0.3.0] - 2026-07-29

### Changed

- Redesigned the personal dashboard with a cinematic, content-first visual
  direction, a personal media hero, bento-style statistics, and elevated poster
  rows.
- Added a persistent design-system reference for the dashboard's visual tokens,
  responsive behavior, and component rules.

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

[0.3.2]: https://registry.example.com/Homelab/emby-insights/releases/tag/v0.3.2
[0.3.1]: https://registry.example.com/Homelab/emby-insights/releases/tag/v0.3.1
[0.3.0]: https://registry.example.com/Homelab/emby-insights/releases/tag/v0.3.0
[0.2.0]: https://registry.example.com/Homelab/emby-insights/releases/tag/v0.2.0
[0.1.0]: https://registry.example.com/Homelab/emby-insights/releases/tag/v0.1.0
