# Changelog

All notable changes to Emby Insights are documented in this file.

The project follows [Semantic Versioning](https://semver.org/):

- **MAJOR** releases contain incompatible changes.
- **MINOR** releases add backwards-compatible functionality.
- **PATCH** releases contain backwards-compatible fixes.

## [0.8.20] - 2026-07-29

### Changed

- Replaced the rotating Home carousel entirely: a fixed hero tile for the
  next upcoming release, plus two always-visible tiles for open requests
  and new-for-you — no more rotation, dots or auto-advance. Three earlier
  attempts at refining the carousel's visuals never addressed the actual
  problem: a cinematic poster hero and two abstract counters don't belong
  in the same rotating tile.
- Removed the redundant "Bereits angefragt" list from the Anfragen tab —
  the same open requests already appear on the Home page.
- "Aktivität nach Wochentag" now reads real Playback Reporting sessions
  instead of approximating from each fully-watched title's last-played
  date (which missed partially-watched series entirely).

### Added

- Statistics gained "Aktivität nach Uhrzeit" (by time of day, bucketed into
  Nacht/Morgen/Nachmittag/Abend) and two record tiles: längste Session and
  aktivster Tag im gewählten Zeitraum. Requires the updated connector
  plugin (v0.2.5.0).

## [0.8.19] - 2026-07-29

### Changed

- Reviewed every screen against the `responsive-design` skill (container
  queries, fluid typography, mobile-first breakpoints). Most of the app
  already followed these practices from earlier work (`100dvh`, fluid
  `clamp()` type scales, 44px touch targets). Replaced the one real gap: the
  Statistics metric-card and chart grids used manually-computed viewport
  breakpoints to approximate the sidebar-adjusted content width; `.content`
  is now an actual CSS container (`container: content / inline-size`), so
  those grids query their real available width directly instead of
  hand-calculated viewport math that would drift if the sidebar width ever
  changes.

## [0.8.18] - 2026-07-29

### Added

- Statistics now shows a "Nach Gerät" chart (which device — TV, phone,
  browser, ...) using watch time already collected by Playback Reporting
  but never surfaced before. Requires the updated Emby Insights connector
  plugin (v0.2.4.0).
- The Profil page now shows account facts (Mitglied seit, Zuletzt aktiv,
  Letzter Login, Anfragen insgesamt) — moved here from the Home carousel.

### Changed

- Search on the Anfragen tab now opens results in their own full-screen
  poster overview instead of an inline row, with its own search field to
  refine without leaving the screen.
- The Home carousel dropped its personal-info tile entirely (now on the
  Profil page); it's back to two tiles: next release and open
  requests/new-for-you.

### Fixed

- Requesting a title found via search silently failed and the layout broke
  on mobile after clearing the search field — both are resolved by moving
  search results into the same full-screen overlay pattern already used
  elsewhere in the app (proven to handle both correctly).

## [0.8.17] - 2026-07-29

### Changed

- "Meine Anfragen" on the Anfragen tab is now labeled "Bereits angefragt" to
  distinguish it from search results.
- Home carousel: the profile tile now shows the avatar and text side by
  side instead of stacked; the upcoming-release tile fills the whole tile
  with the title's poster as a background image with the text overlaid; the
  open-requests/new-for-you tile lost its internal divider line in favor of
  a mint glow border around the whole tile.

### Fixed

- Search results on the Anfragen tab now hide again as soon as the search
  field is cleared, instead of staying visible until a new search runs.

## [0.8.16] - 2026-07-29

### Changed

- Reworked the Home carousel into three purpose-built tiles instead of a
  generic repeated layout: the profile picture together with "Mitglied
  seit"/last-active/total-requests; the next upcoming release shown with
  its actual poster; and open requests + new-for-you side by side in one
  tile.
- Moved the "Suchen" field on the Anfragen tab above the "Meine Anfragen"
  headline instead of between it and the poster row.

## [0.8.15] - 2026-07-29

### Changed

- The Home carousel no longer repeats your name on every slide — only the
  profile picture stays. A new combined slide shows "Mitglied seit" together
  with your last activity and total Seerr request count. All slide text is
  now centered so the tiles look fuller.

### Added

- A "Suchen" button on the Anfragen tab searches Seerr/TMDB directly — no
  more being limited to the 5 curated discover lists. Results open the
  existing detail screen, so requesting (including season selection for
  series) works exactly like everywhere else.

## [0.8.14] - 2026-07-29

### Fixed

- "Serien abgeschlossen" showed a count but an empty poster overview: Emby
  never rolls a series' `LastPlayedDate` up from its episodes (only `Played`
  and `UnplayedItemCount` do), so the period filter silently excluded every
  series. Resolved via the most recently played episode instead. The same
  fix applies to "Gesehene Serien" and the weekday activity chart, which
  had the same latent gap.

### Added

- The "Weiterschauen" poster overview now shows a progress bar per title,
  matching the Home/Statistics row.
- Clicking a poster while an episode is in progress (e.g. from
  "Weiterschauen") now shows the series' full detail screen plus which
  season and episode you're currently on, instead of a bare, out-of-context
  episode detail.

## [0.8.13] - 2026-07-29

### Added

- Every poster row now has a "view all" arrow by default, opening a
  scrollable poster grid — previously only "Gesehene Filme"/"Gesehene
  Serien" had this.
- The "Filme abgeschlossen" and "Serien abgeschlossen" tiles on the
  Statistics page are now clickable and open a poster overview of exactly
  the titles completed in the selected period (new `/api/completed-movies`
  and `/api/completed-series` endpoints, using the same period boundaries
  as `/api/stats`).

### Fixed

- The "Zum Inhalt springen" skip link no longer appears pinned in the
  corner while scrolling on mobile/touch devices.

## [0.8.12] - 2026-07-29

### Added

- The detail screen for not-yet-owned titles (Requests, discover lists) now
  also shows Status, Erscheinungsdatum and Studios, sourced from Seerr's own
  TMDB proxy.

### Fixed

- "Meine Anfragen" (both on Home and the Anfragen page) now refreshes right
  after a request is created, instead of only updating on the next reload.

## [0.8.11] - 2026-07-29

### Fixed

- The detail screen's close button reserved space in the normal document
  flow (sticky, with a margin), which pushed the backdrop image down and
  left a solid black gap above it. The button is now `position: fixed`
  (floating, no longer in flow) and uses the app's mint accent color
  instead of a dark circle.

### Changed

- Season toggles in the request popup now start off by default (none
  requested until explicitly switched on), not all-on.

## [0.8.10] - 2026-07-29

### Changed

- Redesigned the request flow to match the app's visual language: "Anfragen"
  now sits top-right above the description, and clicking it opens a
  centered, scrollable popup instead of an inline reveal. Movies show
  "Jetzt anfragen" / "Abbrechen"; series additionally list their seasons
  as a scrollable toggle-switch list styled to match the rest of the app.

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
