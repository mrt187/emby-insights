# Changelog

All notable changes to Emby Insights are documented in this file.

The project follows [Semantic Versioning](https://semver.org/):

- **MAJOR** releases contain incompatible changes.
- **MINOR** releases add backwards-compatible functionality.
- **PATCH** releases contain backwards-compatible fixes.

## [0.8.48] - 2026-07-30

### Added

- "Offene Staffeln" renamed to "Noch nicht fertig" (eyebrow "Teilweise
  gesehen" unchanged).
- Series in that row can now be dismissed with an "Ausblenden" button
  (styled like the Anfragen button), without needing to finish watching
  them. Dismissals persist per user (new `hidden_in_progress` column,
  independent of ratings/watchlist) and are also filtered server-side, so
  a dismissed series stays hidden after a real refetch.
- Mobile bottom nav: Profil removed again (reachable via the avatar tap in
  the header, as before).

## [0.8.47] - 2026-07-30

### Changed

- Verwaltung's Seerr/Radarr/Sonarr/TMDB cards are now collapsible, closed by
  default and independent of the enabled toggle — the address and API key
  fields only show after tapping the card, so flipping the toggle no longer
  forces the card open.
- The mobile bottom navigation now shows every nav entry (Profil included)
  side by side instead of a fixed 4-column grid that hid it.

## [0.8.46] - 2026-07-30

### Fixed

- Statistik disappeared for accounts that never used the in-app star
  rating/watchlist feature: its visibility was wrongly tied to the
  `media_tracking` table (personal ratings/watchlist), not to actual
  playback data. There's no reliable way to detect the Playback Reporting
  connector without a live Emby call, so Statistik is always shown again.
- Verwaltung's library selection now matches the Dienste tiles visually:
  "Neu für dich" and "Gesehene Filme und Serien" are two tiles that open an
  overlay with a toggle switch per library, like the season picker on
  series requests, instead of two inline checkbox lists.
- Tapping the sidebar logo now jumps to Heute and force-refreshes the page
  (cache-busted URL) — works around iPad home-screen installs not having a
  browser reload control to recover from stale content.
- "Im Kino" moved to the last poster row on Heute.

## [0.8.45] - 2026-07-30

### Added

- Setup wizard replacing manual `.env` configuration: the first Emby account
  to log in successfully becomes the Emby Insights admin automatically and
  atomically (no more `ADMIN_EMBY_USER_ID`), and gets a new **Verwaltung**
  admin page to configure Emby library selection and the optional Seerr,
  Radarr, Sonarr and TMDB integrations from the browser — changes apply
  immediately, no container restart required.
- `EMBY_DEVICE_ID` is now generated once and persisted automatically instead
  of requiring a manually supplied unique ID.
- API keys for optional integrations are stored encrypted at rest (AES-256-GCM,
  keyed by a new `APP_ENCRYPTION_KEY` env var) and are never returned to the
  browser, only as a masked preview.
- Home, Statistik and Anfragen now hide sections tied to services that
  haven't been configured yet (e.g. no Seerr means no Anfragen page or
  request-related Home cards; no Radarr/Sonarr means no Demnächst/Im Kino).

### Changed

- `.env.example` no longer lists `EMBY_DEVICE_ID`, `RADARR_*`, `SONARR_*`,
  `TMDB_API_KEY`, `COMINGSOON_*`, `EMBY_NEW_FOR_YOU_LIBRARY_IDS`,
  `EMBY_WATCHED_LIBRARY_IDS` or `ADMIN_EMBY_USER_ID` — all of that now lives
  in Postgres and is managed through Verwaltung. Existing installs are
  seeded once from any of these variables still present at first boot after
  the upgrade, so nothing already configured is lost.

## [0.8.44] - 2026-07-30

### Added

- WOW added back as a provider chip under Anfragen, using the confirmed
  TMDB provider id 30 (verified via Seerr's own discover UI, replacing
  the earlier Hulu chip that was dropped for not being available in
  Germany).

## [0.8.43] - 2026-07-30

### Fixed

- The rank card's title ("Platz N") could render above the card's own
  background on narrower widths — its content was a horizontal
  avatar+text flex row that needed to fit a wrapping three-line subtitle
  next to a fixed-size avatar, unlike every other card's simple vertical
  stack. It now uses the exact same vertical flow as the metric cards
  (avatar in place of the icon, then title/label/detail stacked below),
  so it grows like the others instead of overflowing.

### Changed

- Provider chips under Anfragen are bigger and now a single, consistent
  dark/monochrome style instead of per-brand colored gradients.
- Removed the Hulu chip — it doesn't operate in Germany at all, so no
  provider id could ever return results for it. Replacing it with WOW
  (the German Sky-owned service) is pending — its exact TMDB provider id
  couldn't be confirmed without live API access; needs a quick check
  against the Seerr/TMDB discover UI to get the right id.

## [0.8.42] - 2026-07-30

### Fixed

- The own-profile avatar (header, rank card, profile page, and "my"
  messages in chat) kept showing the previous session's picture after a
  different user logged in on the same device. The `<img>` element's
  `src` was the literal string `/api/me/avatar` in every case, which
  never changed across a login switch, so the browser never even issued
  a new request — no HTTP cache involved. The URL now includes the
  user's id (`/api/me/avatar?u=<id>`), which forces a real reload.
- HBO Max's provider chip returned no results — it used TMDB's legacy
  "HBO Max" provider id (384), which predates HBO Max's actual German
  launch (13 Jan 2026) and has no DE listing. Switched to the current
  "Max" provider id (1899), the one TMDB lists for the German rollout.
- The admin's chat view looked visually different from the regular
  user's chat (no bordered/gradient card around the messages) because it
  was built on a different container. It now reuses the same `.chat-thread`
  card styling as the user-facing chat.

## [0.8.41] - 2026-07-30

### Fixed

- The chat window on mobile/tablet no longer blows up the message input and
  send button to a huge size on short threads — the thread card used CSS
  Grid, whose default `align-content` stretches auto-sized rows to fill
  leftover vertical space; switched it to a flex column so only the message
  list grows and the composer keeps its natural height at the bottom.
- Logging in as a different user on the same device no longer shows the
  previous user's profile picture — the avatar endpoints were cached for
  1h with no per-user cache key, so the browser kept serving the old
  image; they now require revalidation on every load.
- "Im Kino" was only showing 1–2 movies because any film without an
  announced digital release date (common for new releases — that date is
  often set weeks into the cinema run) was being treated as already
  available and dropped from the row entirely. A missing digital date now
  counts as "still running".

### Added

- Two more provider chips under Anfragen: Paramount+ and HBO Max.

### Changed

- "Offene Staffeln" (partially watched series) moved from the Statistics
  page to the Home/"Heute" tab, directly under "Demnächst", where it's
  more likely to be noticed day-to-day.

## [0.8.40] - 2026-07-30

### Changed

- Discover lists (`/api/discover/*`) are now cached for 1h, personal stats
  (`/api/stats*`) and requests (`/api/requests`) for 5min, backed by Redis
  with the existing coming-soon cache pattern — a Redis outage or a cache
  miss always falls through to the live upstream call, so the dashboard
  never breaks because of the cache. Submitting a new request immediately
  invalidates that user's requests cache instead of waiting out the TTL.

## [0.8.39] - 2026-07-30

### Added

- Chat messages now show the sender's profile picture next to each bubble,
  and the admin's replies use a distinct blue-tinted tone instead of the
  same neutral background as before.
- The admin can delete an entire chat thread from within it (with a
  confirmation step) — irreversible, removes both directions of messages.
- New "Offene Staffeln" row on the Statistics page: series with some, but
  not all, episodes watched.
- New "Anbieter" row under Anfragen with Netflix/Disney+/Prime Video/Apple
  TV+/Hulu chips; tapping one opens a filtered grid of titles available
  there (Seerr discover filtered by TMDB watch-provider ID — verify against
  the live Seerr instance after deploying, this couldn't be tested here).

### Fixed

- The chat composer now always sits at the bottom of the screen instead of
  floating right under a short message list with empty space beneath it.
- The reload button was invisible on iPad-width screens (850–1050px,
  between the two nav breakpoints) — now shows there too.
- The rank card's value text and internal spacing now match the other
  statistics cards.

## [0.8.38] - 2026-07-30

### Fixed

- The chat thread list's message preview ran past the screen edge on
  mobile instead of being truncated with an ellipsis: a grid/flex item's
  automatic minimum size defaults to its content's min-content width, so
  the nowrap preview text forced every ancestor (list, row) wider instead
  of clipping. Added `min-width: 0` down the whole chain.

## [0.8.37] - 2026-07-30

### Changed

- Separated the admin inbox header ("Posteingang", "Rundmail", "Admin
  schreiben") from the thread list card — they no longer sit inside the
  same tile. The header now wraps reliably on narrow phones instead of
  overflowing past the screen edge, and the action buttons are pill-styled
  for clearer affordance.
- Renamed "Neuer Chat" to "Admin schreiben".

## [0.8.36] - 2026-07-30

### Fixed

- On the Chats page, the "Rundmail"/"Neuer Chat" buttons overflowed past
  the screen edge next to the "Posteingang" heading on mobile instead of
  wrapping onto their own line.
- The mobile bottom nav had 5 items in a 4-column grid, so "Profil" wrapped
  onto its own broken second row. Removed it from the bottom nav — it's
  already reachable via the avatar button in the header.

## [0.8.35] - 2026-07-30

### Added

- A "Rundmail" button in the Chats inbox lets the admin send one message
  to every Emby user's thread at once — e.g. a maintenance heads-up —
  instead of writing to each thread individually. It reuses the normal
  chat thread and notification bell, so recipients see it exactly like
  a personal reply.

## [0.8.34] - 2026-07-30

### Added

- The admin can now start a new chat with any Emby user, not just reply to
  incoming ones: a "Neuer Chat" picker in the Chats inbox lists every user
  (with their Emby profile picture) who doesn't already have a thread.

### Internal

- New `emby.Users`/`UserPrimaryImageByID` (admin-key-authenticated, so it
  works even for people who have never logged into this app) back
  `GET /api/admin/users` and `GET /api/admin/users/avatar`.
- `store.MessageStore.Send` always seeds/refreshes the thread's cached
  display name from whichever caller has one, rather than only trusting a
  user's own first message — needed so admin-started threads show a real
  name immediately.

## [0.8.33] - 2026-07-30

### Added

- Internal chat between any user and the admin: a new "Chats" nav item,
  polled every 20s. Regular users get a single thread with the admin;
  the admin gets an inbox listing every user's thread. New messages
  activate the notification bell and surface a preview in the "Jetzt
  relevant" card on the Home page.
- Set `ADMIN_EMBY_USER_ID` (your own Emby user ID, visible via `/api/me`)
  to enable chat and be recognized as the admin — it's disabled until
  configured. See `.env.example`.

### Internal

- Repurposed the `notifications` table (defined since the very first
  migration but never used by any code) as the chat message store,
  instead of adding a parallel table.
- `useApiResource` gained an optional polling interval, reused for the
  new unread-count and chat-thread polling instead of a separate hook.

## [0.8.32] - 2026-07-30

### Changed

- Reordered the statistics week grid so the rank card comes first and the
  completed-movies/completed-series cards sit next to each other instead
  of diagonally opposite.

## [0.8.31] - 2026-07-30

### Changed

- Moved the Status/Erscheinungsdatum/Studios facts on the media detail
  screen right under the title, ahead of the watchlist/request controls
  and the overview, so they're visible immediately instead of only after
  scrolling past a potentially long overview.

## [0.8.30] - 2026-07-30

### Fixed

- The mobile media detail screen left a plain black band above the backdrop
  image, because the backdrop was anchored inside the padded scroll column
  instead of the true top of the screen.
- The detail poster showed a visible green fallback frame around loaded
  posters: the image wasn't filling its container edge-to-edge like every
  other poster surface in the app.

### Changed

- The backdrop now always renders (with a plain ambient fallback when a
  title has no backdrop image), so the hero looks consistent across titles.
- The detail poster is larger, and the watchlist toggle / request button now
  align to the start of the layout instead of drifting to the right edge
  disconnected from the title.
- Long overviews are clamped to 6 lines with a "Mehr anzeigen" toggle instead
  of running on indefinitely.
- The hero/overview area on mobile keeps a minimum height of one screen, so
  the cast section always sits behind an initial scroll rather than
  appearing immediately for titles with a short overview.

## [0.8.29] - 2026-07-30

### Changed

- Redesigned the "Dein Platz" rank card on the statistics page: the
  placement badge now sits on the user's avatar (like a leaderboard pin)
  and the value reads "Platz N" with a subtext explaining it's ranked by
  watch time across all users, instead of a disconnected medal icon that
  didn't make its purpose clear at a glance.

## [0.8.28] - 2026-07-30

### Fixed

- The mobile refresh icon next to the notification bell was drawn
  asymmetrically and sat visually off-center; replaced with a balanced,
  rotationally symmetric refresh icon.

### Added

- The app version is now shown on the profile page as well, so it stays
  visible on mobile where the sidebar (and its version line) is hidden.

## [0.8.27] - 2026-07-30

### Changed

- Touch targets for the star rating and favorite/watchlist toggles now meet
  the 44px minimum without changing their visible size.
- Poster rows, media grids, and statistics cards/charts show a skeleton
  placeholder instead of plain loading text.

### Internal

- Deduplicated ~12 near-identical fetch effects in the dashboard and
  statistics views behind a shared `useApiResource` hook, and extracted a
  shared `useEscapeKey` hook for overlay screens.

## [0.8.26] - 2026-07-30

### Fixed

- Desktop poster lists remain a single scrollable row; the arrow continues to
  open the complete overview.
- The mobile web app now has a visible refresh button in the header, and the
  mobile detail hero leaves clear space beneath its fixed close button.
- The cinema feed keeps films currently in cinemas while limiting only future
  cinema starts to the next 30 days.

## [0.8.25] - 2026-07-30

### Added

- The Home release area uses direct Radarr/Sonarr/TMDB data with a dedicated
  30-day cinema window. Near-term cinema starts also appear in the personal
  notification card.
- Statistics now show each user's all-time watch-time placement. This is
  supplied by the bundled Emby Insights plugin v0.2.6.0, whose release DLL is
  attached to this release.

### Changed

- Desktop views use the available content width and responsive poster grids;
  mobile keeps compact, touch-friendly horizontal poster rows.
- Ratings use a gold star treatment, and dashboard greetings are casual while
  remaining time-aware.

## [0.8.24] - 2026-07-30

### Fixed

- The Emby favorite endpoint (`POST`/`DELETE /api/media/emby/favorite`) now
  rejects any `itemId` that is not a plain Emby item ID. The ID was
  interpolated into the upstream URL unescaped, and that request carries the
  Emby **admin** API key — so a crafted ID could reach arbitrary Emby admin
  endpoints, including item deletion. The ID is validated in the handler and
  additionally path-escaped in the Emby client.

## [0.8.23] - 2026-07-30

### Added

- The Home dashboard now reads the release calendar directly from Radarr and
  Sonarr, optionally enriching movie release dates through TMDB for the
  configured region. It no longer needs the separate ComingSoon Emby plugin
  or its generated libraries.
- A new "Im Kino" row shows upcoming cinema starts and films currently in
  cinemas. Cards state either the cinema start date or the expected end of
  the cinema window, while the Upcoming row uses relative availability text.
- Calendar responses are cached in Redis for 15 minutes. If TMDB is
  temporarily unavailable, Radarr's dates keep the movie calendar usable.

### Changed

- Upcoming TV episodes now carry their season and episode number directly;
  no title-format parsing from generated ComingSoon entries is needed.

## [0.8.22] - 2026-07-29

### Added

- Persönliches Tracking: eigene 1-5-Sterne-Bewertung und eine Merkliste,
  gespeichert in unserer eigenen Datenbank statt bei Emby, da Emby von Haus
  aus weder Sterne noch eine Merkliste kennt. Favoriten laufen dagegen
  bewusst Emby-nativ, damit sie mit jeder anderen Emby-App synchron bleiben.
  Im Detailscreen gibt es dafür eine neue Aktionsleiste zwischen Hero und
  Übersicht; im Profil eine neue Sektion "Meine Listen".
- Ein eingebetteter Postgres-Migrationsrunner (`backend/internal/store`)
  ersetzt das bisherige `initdb`-Skript, das auf der laufenden Instanz ohnehin
  nie erneut ausgeführt worden wäre. Ein Postgres-Advisory-Lock verhindert,
  dass sich zwei Container beim "Update Stack" gegenseitig die Migration
  kaputt machen.

### Fixed

- `entrypoint.sh` beendet den Container jetzt, sobald API oder Frontend
  abstürzen, statt nur auf das Frontend zu warten — ein Absturz der API
  (etwa durch eine fehlgeschlagene Migration) blieb bisher unsichtbar grün
  in Unraid, während jeder `/api/*`-Aufruf ins Leere lief.
- Die tote Fremdschlüsselbeziehung von `user_settings`/`notifications` auf
  die nie befüllte Tabelle `user_mappings` wurde entfernt.

## [0.8.21] - 2026-07-29

### Changed

- Replaced the first Home tile entirely: the "Dein Rhythmus" hero card is now
  a compact personal inbox titled "Jetzt relevant". Instead of counters it
  shows at most three concrete, tappable events — a request that just became
  available, a release due within 48 hours, or the number of unseen titles —
  each as a row with an icon, a status line and the concrete detail. With
  more than three events an "Alle ansehen" link opens a full-screen list;
  with none it shows a single quiet line instead of an empty state.
  The card carries no poster surface any more, so the "Demnächst" poster row
  below is once again the visual focus of the page.

### Added

- `GET /api/requests/available` reports requests whose title became available
  in the library within the last seven days. `/api/requests` deliberately
  filters exactly these out for its "open requests" row, so the new tile
  needed its own source.

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
