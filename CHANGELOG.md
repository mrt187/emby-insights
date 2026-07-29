# Changelog

All notable changes to Emby Insights are documented in this file.

The project follows [Semantic Versioning](https://semver.org/):

- **MAJOR** releases contain incompatible changes.
- **MINOR** releases add backwards-compatible functionality.
- **PATCH** releases contain backwards-compatible fixes.

## [0.8.9] - 2026-07-29

### Changed

- The season checklist for series requests is now hidden until "Anfragen" is
  clicked, revealed with a short expand animation (`prefers-reduced-motion`
  respected) instead of always showing under the button. Checkbox touch
  targets now meet the 44px minimum.

## [0.8.8] - 2026-07-29

### Added

- Media requests can now be created directly from the app — no more switching
  to Seerr. The detail screen shows an "Anfragen" button for not-yet-owned
  titles; series show a season checklist so you can request specific seasons
  instead of the whole show. Requests always use Seerr's own default
  Radarr/Sonarr profile; there is no profile picker.

## [0.8.7] - 2026-07-29

### Added

- The media detail screen now shows a series' seasons (poster, progress bar,
  "Angesehen" or "X von Y Folgen"), read via Emby's `/Shows/{id}/Seasons`.
  Seasons with no episodes in the library yet (e.g. an empty "Specials")
  are skipped.

## [0.8.6] - 2026-07-29

### Fixed

- The media detail screen failed to open for series without crew entries in
  Emby (common — many shows only list actors): `crew` serialized as `null`
  instead of `[]`, and the frontend's array spread crashed on it. Cast,
  crew and genres are now always non-nil arrays.

### Added

- "Gesehene Filme" and "Gesehene Serien" gained a small arrow next to the
  heading that opens a full poster-grid overview of every item in that
  list; clicking a poster there opens the same media detail screen.

## [0.8.5] - 2026-07-29

### Added

- Clicking any poster now opens a detail screen: description, cast/crew
  with photos, rating, genres, and a personal status badge ("Angesehen",
  "Verfügbar", or "X von Y Folgen" for series). Sourced from Emby for
  everything already in the library (Demnächst, Neu für dich, Weiterschauen,
  Gesehene Filme/Serien), and from Seerr/TMDB for posters that may not be in
  Emby yet (Anfragen, the five discover lists) — the latter without a
  personal status.
- New endpoints: `GET /api/media/emby?id=`, `GET /api/media/seerr?mediaType=&id=`.

## [0.8.4] - 2026-07-29

### Changed

- The Statistics page's Filme/Serien metric cards are period-scoped again
  (from `/api/stats`), so switching Woche/Monat/Jahr now updates all four
  cards, not just Sehzeit/Lieblingsgenre. "Gesehene Filme"/"Gesehene Serien"
  (the lists below) still always show everything, independent of the
  period selector — only the two concerns were conflated before.
- "Gesehene Filme"/"Gesehene Serien" are now scoped to the Emby libraries
  configured via the new `EMBY_WATCHED_LIBRARY_IDS` (comma-separated),
  matching the Movies/TV shows/4K Filme scoping already used for "Neu für
  dich".

## [0.8.3] - 2026-07-29

### Changed

- The Home "Dein Rhythmus" carousel no longer repeats Sehzeit/Filme/Serien/
  Lieblingsgenre, since those now live on the Statistics page. It shows
  three fresh highlights instead: the next upcoming release, the number of
  open Seerr requests, and the number of new-for-you items.

## [0.8.2] - 2026-07-29

### Changed

- "Gesehene Filme" and "Gesehene Serien" (and their `GET /api/watched-movies`
  / `GET /api/watched-series` endpoints) no longer depend on the Woche/
  Monat/Jahr selector — they always show everything Emby has marked fully
  played.
- The Statistics page's top rows are now one row of four equal metric
  cards (Sehzeit, Filme abgeschlossen, Serien abgeschlossen, Lieblingsgenre)
  instead of a separate summary banner plus three cards. Filme/Serien now
  show all-time totals; Sehzeit/Lieblingsgenre stay tied to the period
  selector.

## [0.8.1] - 2026-07-29

### Changed

- Toned down the Statistics page's top two rows (weekly summary banner and
  the metric cards): smaller padding, type, and card height, so they no
  longer dominate the page above the newer sections.
- The mobile bottom navigation now shows icons only; the labels remain for
  screen readers but are no longer shown visually.

## [0.8.0] - 2026-07-29

### Fixed

- "Gesehene Filme" and "Gesehene Serien" were always empty: this Emby
  server's bulk `/Items` list endpoint omits `UserData.LastPlayedDate`
  (only the single-item endpoint returns it). The watched-items reader now
  looks up each item's last-played date individually, stopping as soon as it
  finds one played before the selected period (the list is already sorted
  by play date, descending).

### Added

- The Requests page now shows five Seerr/TMDB discover lists: "Im Trend",
  "Beliebte Filme", "Demnächst erscheinende Filme", "Beliebte Serien" and
  "Demnächst erscheinende Serien", via five new endpoints under
  `/api/discover/*`.

## [0.7.0] - 2026-07-29

### Added

- The Statistics page now shows "Was ich gerade schaue" (Emby's own resume/
  continue-watching list with progress bars), "Gesehene Filme" and "Gesehene
  Serien" (fully watched titles for the selected period), plus two charts:
  most-watched genres and activity by weekday, both derived from the watched
  lists.
- New endpoints: `GET /api/continue-watching`, `GET /api/watched-movies`,
  `GET /api/watched-series` (the latter two take the same `period` query
  parameter as `/api/stats`).

### Changed

- The Home "Dein Rhythmus" card is now a single carousel: every slide repeats
  the profile identity alongside its stat, instead of a static identity panel
  next to a separately rotating stat panel.
- The Statistics page's metric row dropped the redundant watch-time tile
  (already shown in the summary banner above) and now shows three
  equal-width cards.

## [0.6.1] - 2026-07-29

### Changed

- "Neu für dich" is now scoped to the Emby libraries configured via the new
  `EMBY_NEW_FOR_YOU_LIBRARY_IDS` (comma-separated), instead of reading unseen
  items across every library. Leaving it empty disables the card.

## [0.6.0] - 2026-07-29

### Added

- The Home "Neu für dich" card now shows real unseen items added to Emby in
  the last 14 days, via a new `GET /api/new-for-you` endpoint, instead of a
  fixed placeholder list.

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
