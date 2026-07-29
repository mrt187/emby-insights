# Changelog

All notable changes to Emby Insights are documented in this file.

The project follows [Semantic Versioning](https://semver.org/):

- **MAJOR** releases contain incompatible changes.
- **MINOR** releases add backwards-compatible functionality.
- **PATCH** releases contain backwards-compatible fixes.

## [0.5.1] - 2026-07-29

### Fixed

- The Seerr requests connector sent an unsupported `filter=all` query
  parameter, which some Seerr versions reject with `400 Bad Request`,
  breaking the Requests Home card entirely. The parameter has been removed;
  filtering to open, non-available requests still happens client-side.

## [0.5.0] - 2026-07-29

### Added

- The Home "Demnächst" card now shows real upcoming releases read from the
  ComingSoon libraries configured via `EMBY_COMINGSOON_LIBRARY_IDS`, instead
  of placeholder titles.
- The Home and Requests "Meine Anfragen" cards now show real open Seerr
  requests via a new Jellyseerr/Overseerr connector, configured via
  `SEERR_URL` and `SEERR_API_KEY`. Both cards show real cover art where the
  source provides it.
- Both integrations are optional: leaving their environment variables empty
  disables the corresponding card instead of failing startup.

## [0.4.0] - 2026-07-29

### Added

- The personal card's weekly figure is now a slider that rotates through watch
  time, films, series, and favourite genre every five seconds. It can be swiped
  or jumped to directly via the dots, stops rotating once you interact with it,
  and stays still when the system asks for reduced motion.

### Removed

- Dropped the large introductory hero tile from the Today screen. Media content
  is now reachable without scrolling past a full screen of decoration.

### Changed

- Reduced the personal card to avatar, name, and the rotating figure.

## [0.3.5] - 2026-07-29

### Fixed

- Corrected the dashboard's responsive breakpoints so the personal overview card
  no longer overflows horizontally between 1051px and 1177px, and removed the
  `overflow-x: hidden` rule that had been masking it.
- Stopped the hero headline and long favourite-genre names from being clipped on
  tablets and phones; the statistics tiles now reflow instead of truncating.
- Wired up the profile screen's sign-out button, which previously did nothing.

### Changed

- Consolidated three stacked stylesheet layers into one, removing 28 unused rule
  blocks left over from earlier layouts.
- Derived metric accent colours from the documented design tokens instead of DOM
  order, so reordering cards no longer changes their colours.
- Improved keyboard and screen-reader handling: the notifications popover closes
  on Escape and outside clicks, the two navigation landmarks have distinct
  labels, and touch targets meet 44px.

## [0.3.4] - 2026-07-29

### Changed

- Added the current application version to the lower-left dashboard sidebar.
- Made the Unraid updater explicitly refresh and run the registry's `latest`
  image, including a safe one-time handover from the previously standalone
  application container.

## [0.3.3] - 2026-07-29

### Fixed

- Made the Unraid User Script invoke the release updater through Bash so it
  works reliably regardless of checkout file permissions.

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

[0.4.0]: https://registry.example.com/Homelab/emby-insights/releases/tag/v0.4.0
[0.3.5]: https://registry.example.com/Homelab/emby-insights/releases/tag/v0.3.5
[0.3.3]: https://registry.example.com/Homelab/emby-insights/releases/tag/v0.3.3
[0.3.4]: https://registry.example.com/Homelab/emby-insights/releases/tag/v0.3.4
[0.3.2]: https://registry.example.com/Homelab/emby-insights/releases/tag/v0.3.2
[0.3.1]: https://registry.example.com/Homelab/emby-insights/releases/tag/v0.3.1
[0.3.0]: https://registry.example.com/Homelab/emby-insights/releases/tag/v0.3.0
[0.2.0]: https://registry.example.com/Homelab/emby-insights/releases/tag/v0.2.0
[0.1.0]: https://registry.example.com/Homelab/emby-insights/releases/tag/v0.1.0
