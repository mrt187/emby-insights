"use client";

import { type FormEvent, type ReactNode, useEffect, useRef, useState } from "react";
import { LoginScreen } from "./login-screen";
import { calendarSelection, type MediaSelection } from "./media-selection";
import { isLang, LanguageContext, type Lang, locales, t, type TranslationKey, useLang, useT } from "./translations";
import { PushNotificationSettings } from "./push-notifications";

type Page = "today" | "stats" | "requests" | "chats" | "profile" | "admin";
const pages: Page[] = ["today", "stats", "requests", "chats", "profile", "admin"];
function isPage(value: unknown): value is Page { return pages.includes(value as Page); }

// The refresh button in the topbar reloads the document (see
// goHomeAndRefresh's comment on why a plain reload isn't enough on iOS), but
// `page` is in-memory React state — without this, reloading always landed
// back on "today" instead of wherever the user was.
const PAGE_STORAGE_KEY = "ei-current-page";
function storedPage(): Page {
  if (typeof window === "undefined") return "today";
  const value = window.sessionStorage.getItem(PAGE_STORAGE_KEY);
  return isPage(value) ? value : "today";
}
type Features = { requests: boolean; movieDates: boolean; seriesDates: boolean; upcoming: boolean; statistics: boolean };
type CurrentUser = { id: string; name: string; isAdmin: boolean; features: Features; language?: Lang };
type Period = "week" | "month" | "year";
type PersonalStats = { watchSeconds: number; previousWatchSeconds: number; completedMovies: number; completedSeries: number; favouriteGenre: string; periodStartsAt: string; periodEndsAt: string };
type WatchTimeRank = { rank: number };
type DeviceWatchTime = { deviceName: string; watchSeconds: number };
type HourWatchTime = { hour: number; watchSeconds: number };
type WeekdayWatchTime = { weekday: number; watchSeconds: number };
type LongestSession = { itemName: string; watchSeconds: number; startedAt: string };
type MostActiveDay = { date: string; watchSeconds: number };
type UserProfile = { memberSince: string; lastActiveDate: string; lastLoginDate: string };
type UpcomingItem = { id: string; tmdbId: string; source: "radarr" | "sonarr"; detailId: string; title: string; posterUrl: string; mediaType: string; availabilityDate: string; cinemaStartDate?: string; cinemaEndDate?: string; seasonNumber?: number; episodeNumber?: number; episodeTitle?: string };
type RequestItem = { id: string; title: string; posterUrl: string; status: string; tmdbId: string; mediaType: string; availableSince?: string };
type NewForYouItem = { id: string; title: string; posterUrl: string; dateCreated?: string; seriesName?: string; seasonNumber?: number; episodeNumber?: number };
type TopRatedItem = { id: string; mediaSource: string; mediaId: string; mediaType: string; title: string; posterUrl: string; averageRating: number };
type ContinueWatchingItem = { id: string; title: string; posterUrl: string; progressPercent: number };
type WatchedItem = { id: string; title: string; posterUrl: string; genres: string[]; lastPlayedDate: string; backdropUrl?: string; dateAdded?: string };
type SeriesProgress = { id: string; title: string; posterUrl: string; watchedEpisodes: number; totalEpisodes: number };
type DiscoverItem = { id: string; title: string; posterUrl: string; mediaType: string };

type MediaPerson = { name: string; role: string; imageUrl: string };

// Ein Poster kann fehlschlagen, obwohl eine URL da ist: Emby antwortet mit 500,
// wenn die Bilddatei eines Titels defekt ist, und ein <img> mit einer URL, die
// nicht liefert, zeigt im Browser das kaputte Bild-Icon. Der Platzhalter wurde
// bisher nur gerendert, wenn gar keine URL vorlag — also genau dann nicht, wenn
// man ihn braucht.
function PosterImage({ src, fallback, lazy = true }: { src?: string; fallback: ReactNode; lazy?: boolean }) {
  if (!src) return <span>{fallback}</span>;
  return <PosterImageSource key={src} src={src} fallback={fallback} lazy={lazy} />;
}

function PosterImageSource({ src, fallback, lazy }: { src: string; fallback: ReactNode; lazy: boolean }) {
  const [failed, setFailed] = useState(false);
  if (failed) return <span>{fallback}</span>;
  return <img src={src} alt="" loading={lazy ? "lazy" : undefined} onError={() => setFailed(true)} />;
}

type MediaSeason = { id: string; title: string; posterUrl: string; indexNumber: number; watchedEpisodes: number; totalEpisodes: number; played: boolean };
type RequestableSeason = { seasonNumber: number; episodeCount: number; available?: boolean; requested?: boolean };
type MediaDetail = {
  id: string; title: string; overview: string; posterUrl: string; backdropUrl: string;
  genres: string[]; communityRating: number; officialRating?: string; year: number; runtimeMinutes: number;
  cast: MediaPerson[]; crew: MediaPerson[];
  isSeries?: boolean; watchedEpisodes?: number; totalEpisodes?: number; played?: boolean; isFavorite?: boolean;
  currentSeasonNumber?: number; currentEpisodeNumber?: number;
  seasons?: MediaSeason[] | RequestableSeason[];
  mediaStatus?: number;
  status?: string; releaseDate?: string; studios?: string[];
  imdbRating?: string; rottenTomatoesRating?: string;
};
type MediaTrackingEntry = { mediaSource: string; mediaId: string; mediaType: string; title: string; posterUrl: string; rating?: number; onWatchlist: boolean; hiddenInProgress?: boolean };
function isRequestableSeason(season: MediaSeason | RequestableSeason): season is RequestableSeason { return !("id" in season); }
type IconName = "home" | "chart" | "sparkle" | "heart" | "user" | "bell" | "arrow" | "close" | "clock" | "movie" | "series" | "genre" | "medal" | "refresh" | "chat" | "settings";
type Tone = "blue" | "peach" | "mint" | "lilac";
type LoadState = "loading" | "ready" | "error";

type ChatMessage = { id: string; body: string; fromAdmin: boolean; createdAt: string };
type ChatThread = { userId: string; displayName: string; lastMessage: string; lastAt: string; unreadCount: number };
type Contact = { id: string; name: string };

const UNREAD_POLL_MS = 20_000;
const CHAT_POLL_MS = 20_000;

// Which nav entries a user sees depends on which optional services the admin
// has configured in Verwaltung: Statistik needs tracked watch data, Anfragen
// needs Seerr, and Verwaltung only ever shows for the Emby Insights admin.
// The entry carries a translation key rather than a label, so `page` stays a
// stable internal identifier no matter which language the UI speaks.
function visibleNav(user: CurrentUser): { page: Page; labelKey: TranslationKey; icon: IconName }[] {
  const items: { page: Page; labelKey: TranslationKey; icon: IconName }[] = [{ page: "today", labelKey: "nav_today", icon: "home" }];
  if (user.features.statistics) items.push({ page: "stats", labelKey: "nav_stats", icon: "chart" });
  if (user.features.requests) items.push({ page: "requests", labelKey: "nav_requests", icon: "sparkle" });
  items.push({ page: "chats", labelKey: "nav_chats", icon: "chat" }, { page: "profile", labelKey: "nav_profile", icon: "user" });
  if (user.isAdmin) items.push({ page: "admin", labelKey: "nav_admin", icon: "settings" });
  return items;
}
const pageTitleKey: Record<Page, TranslationKey> = { today: "nav_today", stats: "nav_stats", requests: "nav_requests", chats: "nav_chats", profile: "nav_profile", admin: "nav_admin" };
const periodLabelKey: Record<Period, TranslationKey> = { week: "period_week", month: "period_month", year: "period_year" };
const APP_VERSION = "0.13.4";

// One formatter per language and purpose, built once: Intl.DateTimeFormat is
// expensive enough that constructing it inside a render loop is wasteful.
function localeFormatter(options: Intl.DateTimeFormatOptions): Record<Lang, Intl.DateTimeFormat> {
  return { de: new Intl.DateTimeFormat(locales.de, options), en: new Intl.DateTimeFormat(locales.en, options) };
}
const dateFormatters = localeFormatter({ day: "2-digit", month: "short" });
function formatPremiereDate(value: string, lang: Lang) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "" : dateFormatters[lang].format(date);
}
function availabilityWording(value: string, lang: Lang) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return t(lang, "availability_soon");
  const today = new Date(); today.setHours(0, 0, 0, 0);
  const release = new Date(date); release.setHours(0, 0, 0, 0);
  const days = Math.round((release.getTime() - today.getTime()) / 86_400_000);
  if (days <= 0) return t(lang, "availability_today");
  if (days === 1) return t(lang, "availability_tomorrow");
  if (days < 7) return t(lang, "availability_in_days", { days });
  const weeks = Math.ceil(days / 7);
  return weeks === 1 ? t(lang, "availability_in_one_week") : t(lang, "availability_in_weeks", { weeks });
}
function cinemaWording(item: UpcomingItem, lang: Lang) {
  const start = item.cinemaStartDate ? new Date(item.cinemaStartDate) : null;
  if (start && start.getTime() > Date.now()) return t(lang, "cinema_start", { date: formatPremiereDate(item.cinemaStartDate!, lang) });
  return item.cinemaEndDate ? t(lang, "cinema_until", { date: formatPremiereDate(item.cinemaEndDate, lang) }) : t(lang, "cinema_now");
}
function upcomingTitle(item: UpcomingItem, lang: Lang) {
  if (item.mediaType !== "tv") return item.title;
  const episode = item.seasonNumber && item.episodeNumber ? `S${item.seasonNumber.toString().padStart(2, "0")}E${item.episodeNumber.toString().padStart(2, "0")}` : t(lang, "new_episode");
  return `${item.title} · ${episode}`;
}

function newForYouTitle(item: NewForYouItem) {
  if (!item.seriesName) return item.title;
  const episode = item.seasonNumber && item.episodeNumber ? `S${item.seasonNumber.toString().padStart(2, "0")}E${item.episodeNumber.toString().padStart(2, "0")}` : item.title;
  return `${item.seriesName} · ${episode}`;
}
const fullDateFormatters = localeFormatter({ day: "2-digit", month: "long", year: "numeric" });
function formatFullDate(value: string, lang: Lang) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "" : fullDateFormatters[lang].format(date);
}
function daysAgoWording(value: string, lang: Lang) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  const days = Math.round((Date.now() - date.getTime()) / 86_400_000);
  if (days <= 0) return t(lang, "day_today");
  if (days === 1) return t(lang, "day_yesterday");
  return t(lang, "days_ago", { days });
}
const shortDateFormatters = localeFormatter({ day: "numeric", month: "long" });
function dateRangeWording(start: string, end: string, lang: Lang) {
  const startDate = new Date(start);
  const endDate = new Date(end);
  if (Number.isNaN(startDate.getTime()) || Number.isNaN(endDate.getTime())) return "";
  // German abbreviates the range start to its bare day number ("3.–9. Mai");
  // English has no equivalent shorthand, so both ends are spelled out.
  return lang === "de"
    ? `${startDate.getDate()}.–${shortDateFormatters.de.format(endDate)}`
    : `${shortDateFormatters.en.format(startDate)} – ${shortDateFormatters.en.format(endDate)}`;
}

function useApiResource<T>(path: string | null, initialValue: T, pollMs?: number): [T, LoadState, () => void] {
  const [data, setData] = useState<T>(initialValue);
  const [result, setResult] = useState<{ path: string | null; state: LoadState }>({ path: null, state: "loading" });
  const [reloadToken, setReloadToken] = useState(0);
  const refetch = () => setReloadToken((token) => token + 1);

  useEffect(() => {
    if (!path) return;
    let active = true;
    fetch(path, { credentials: "include" })
      .then(async (response) => {
        if (!response.ok) throw new Error(`${path} unavailable`);
        const json = await response.json();
        if (active) { setData(json); setResult({ path, state: "ready" }); }
      })
      .catch(() => active && setResult({ path, state: "error" }));
    return () => { active = false; };
  }, [path, reloadToken]);

  useEffect(() => {
    if (!path || !pollMs) return;
    const interval = setInterval(refetch, pollMs);
    return () => clearInterval(interval);
  }, [path, pollMs]);

  // Derived rather than set eagerly in the effect above: a path change (e.g.
  // switching the stats period) should read as "loading" immediately, without
  // an extra synchronous setState at the top of the effect.
  const state: LoadState = !path || result.path !== path ? "loading" : result.state;
  return [data, state, refetch];
}

function useEscapeKey(onEscape: () => void) {
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => { if (event.key === "Escape") onEscape(); };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [onEscape]);
}

function useBodyScrollLock() {
  useEffect(() => {
    const previous = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => { document.body.style.overflow = previous; };
  }, []);
}

export default function Home() {
  const [page, setPage] = useState<Page>(storedPage);
  const [noticeOpen, setNoticeOpen] = useState(false);
  const [user, setUser] = useState<CurrentUser | null>(null);
  const [publicLang, setPublicLang] = useState<Lang>("en");
  const [savedLang, setSavedLang] = useState<Lang | null>(null);
  const [checkingSession, setCheckingSession] = useState(true);
  const [selectedMedia, setSelectedMedia] = useState<MediaSelection | null>(null);
  const noticeRef = useRef<HTMLDivElement>(null);
  const noticeButtonRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    fetch("/api/me", { credentials: "include" })
      .then(async (response) => response.ok ? setUser(await response.json()) : null)
      .catch(() => null)
      .finally(() => setCheckingSession(false));
  }, []);

  // Runs in parallel with /api/me because the login screen needs the language
  // before any session exists. /api/me carries it too, so a logged-in user
  // never depends on this second response landing first.
  useEffect(() => {
    fetch("/api/language", { credentials: "include" })
      .then(async (response) => {
        if (!response.ok) return;
        const data: { language?: string } = await response.json();
        if (isLang(data.language)) setPublicLang(data.language);
      })
      .catch(() => null);
  }, []);

  // Derived rather than synced through an effect, so there is never a render
  // showing the previous language after a newer source has already resolved.
  // Precedence: what the admin just saved, then the session payload, then the
  // unauthenticated fallback above.
  const lang: Lang = savedLang ?? (user && isLang(user.language) ? user.language : publicLang);

  // layout.tsx ships a static lang="de" (it stays a plain, non-async Server
  // Component); the live value is stamped on from here instead.
  useEffect(() => { document.documentElement.lang = lang; }, [lang]);

  const [upcomingItems, upcomingState] = useApiResource<UpcomingItem[]>(user?.features.upcoming ? "/api/upcoming" : null, []);
  const [cinemaItems, cinemaState] = useApiResource<UpcomingItem[]>(user?.features.movieDates ? "/api/in-cinemas" : null, []);
  const [requestItems, requestState, refetchRequestItems] = useApiResource<RequestItem[]>(user?.features.requests ? "/api/requests" : null, []);
  const [requestTotal, , refetchRequestTotal] = useApiResource<{ total: number } | null>(user?.features.requests ? "/api/requests/total" : null, null);
  const totalRequests = requestTotal?.total ?? null;
  const [newForYouItems, newForYouState] = useApiResource<NewForYouItem[]>(user ? "/api/new-for-you" : null, []);
  const [topRatedItems, topRatedState] = useApiResource<TopRatedItem[]>(user ? "/api/top-rated" : null, []);
  const [seriesInProgress, seriesInProgressState, refetchSeriesInProgress] = useApiResource<SeriesProgress[]>(user ? "/api/series-in-progress" : null, []);
  const [continueWatching, continueWatchingState] = useApiResource<ContinueWatchingItem[]>(user ? "/api/continue-watching" : null, []);
  const [availableRequests] = useApiResource<RequestItem[]>(user?.features.requests ? "/api/requests/available" : null, []);
  const [userProfile] = useApiResource<UserProfile | null>(user ? "/api/me/profile" : null, null);

  const refetchRequests = () => { refetchRequestItems(); refetchRequestTotal(); };

  const [unreadData] = useApiResource<{ count: number }>(user ? "/api/messages/unread-count" : null, { count: 0 }, UNREAD_POLL_MS);
  const unread = unreadData.count;
  const [ownThread] = useApiResource<ChatMessage[]>(user && !user.isAdmin && unread > 0 ? "/api/messages" : null, [], CHAT_POLL_MS);
  const latestAdminMessage = [...ownThread].reverse().find((message) => message.fromAdmin);
  const messagePreview = !user || unread === 0 ? null
    : user.isAdmin ? { preview: unread === 1 ? t(lang, "msg_preview_admin_one") : t(lang, "msg_preview_admin_other", { count: unread }) }
    : { preview: latestAdminMessage ? latestAdminMessage.body : t(lang, "msg_preview_user") };

  useEffect(() => {
    if (!noticeOpen) return;
    const close = (returnFocus: boolean) => { setNoticeOpen(false); if (returnFocus) noticeButtonRef.current?.focus(); };
    const onKeyDown = (event: KeyboardEvent) => { if (event.key === "Escape") close(true); };
    const onPointerDown = (event: PointerEvent) => {
      const target = event.target as Node;
      if (!noticeRef.current?.contains(target) && !noticeButtonRef.current?.contains(target)) close(false);
    };
    document.addEventListener("keydown", onKeyDown);
    document.addEventListener("pointerdown", onPointerDown);
    return () => { document.removeEventListener("keydown", onKeyDown); document.removeEventListener("pointerdown", onPointerDown); };
  }, [noticeOpen]);

  if (checkingSession) return <main className="login-shell"><p className="loading-copy" role="status">{t(lang, "app_loading")}</p></main>;
  if (!user) return <LoginScreen lang={lang} onAuthenticated={setUser} />;

  const selectPage = (next: Page) => { setPage(next); setNoticeOpen(false); window.sessionStorage.setItem(PAGE_STORAGE_KEY, next); };
  const openNotices = () => setNoticeOpen((open) => !open);
  const nav = visibleNav(user);

  return <LanguageContext.Provider value={lang}><main className="app-shell">
    {selectedMedia && <MediaDetailScreen selection={selectedMedia} seerrConfigured={user.features.requests} onClose={() => setSelectedMedia(null)} onRequestCreated={refetchRequests} onHiddenChanged={refetchSeriesInProgress} />}
    <a className="skip-link" href="#dashboard-content">{t(lang, "skip_to_content")}</a>
    <aside className="side-nav" aria-label={t(lang, "nav_main")}>
      <button type="button" className="brand" onClick={goHomeAndRefresh} aria-label={t(lang, "brand_home_aria")}><img className="brand-logo" src="/emby-insights-logo.svg" alt="Emby Insights" width="31" height="31" /><span>insights</span></button>
      <nav>{nav.map((item) => <button className={page === item.page ? "nav-item active" : "nav-item"} key={item.page} onClick={() => selectPage(item.page)} aria-current={page === item.page ? "page" : undefined}><Icon name={item.icon} />{t(lang, item.labelKey)}</button>)}</nav>
      <div className="sidebar-meta">
        <div className="server-status"><i aria-hidden="true" /> {t(lang, "connected_to_emby")}</div>
        <p className="app-version">{t(lang, "version")} <strong>v{APP_VERSION}</strong></p>
      </div>
    </aside>

    <section className="screen" id="dashboard-content" tabIndex={-1}>
      <header className="topbar">
        <div><p className="eyebrow">{t(lang, "topbar_eyebrow")}</p><h1>{page === "today" ? `${greeting(lang)}, ${user.name}` : t(lang, pageTitleKey[page])}</h1></div>
        <div className="header-actions">
          <button type="button" className="refresh-button" aria-label={t(lang, "refresh_dashboard")} onClick={() => { window.location.href = `${window.location.pathname}?refresh=${Date.now()}`; }}><Icon name="refresh" /></button>
          <button ref={noticeButtonRef} className="notice-button" aria-label={t(lang, "notifications")} aria-expanded={noticeOpen} aria-controls="notifications" onClick={openNotices}><Icon name="bell" />{unread > 0 && <b><span className="sr-only">{t(lang, "unread_notifications", { count: unread })}</span></b>}</button>
          <button className="avatar" aria-label={t(lang, "open_profile")} onClick={() => selectPage("profile")}><UserAvatar name={user.name} userId={user.id} /></button>
          {noticeOpen && <div ref={noticeRef} className="notifications" id="notifications" role="dialog" aria-label={t(lang, "notifications")}>
            <strong>{t(lang, "notifications")}</strong>
            {messagePreview ? <p>{messagePreview.preview}</p> : <p>{t(lang, "no_new_notifications")}</p>}
            {unread > 0 && <button type="button" className="text-button" onClick={() => selectPage("chats")}>{t(lang, "go_to_chats")} <Icon name="arrow" /></button>}
          </div>}
        </div>
      </header>
      {page === "today" && <Today upcoming={upcomingItems} upcomingState={upcomingState} cinema={cinemaItems} cinemaState={cinemaState} requests={requestItems} requestState={requestState} newForYou={newForYouItems} newForYouState={newForYouState} topRated={topRatedItems} topRatedState={topRatedState} seriesInProgress={seriesInProgress} seriesInProgressState={seriesInProgressState} continueWatching={continueWatching} continueWatchingState={continueWatchingState} availableRequests={availableRequests} features={user.features} message={messagePreview} onSelectMedia={setSelectedMedia} onOpenChats={() => selectPage("chats")} />}
      {page === "stats" && user.features.statistics && <Stats user={user} onSelectMedia={setSelectedMedia} />}
      {page === "requests" && user.features.requests && <Requests onSelectMedia={setSelectedMedia} />}
      {page === "chats" && <Chats user={user} />}
      {page === "profile" && <Profile user={user} userProfile={userProfile} totalRequests={user.features.requests ? totalRequests : null} onSelectMedia={setSelectedMedia} />}
      {page === "admin" && user.isAdmin && <AdminSettings onLanguageChange={setSavedLang} />}
    </section>

    <nav className="bottom-nav" aria-label={t(lang, "nav_main_mobile")}>{nav.filter((item) => item.page !== "profile").map((item) => <button key={item.page} className={page === item.page ? "active" : ""} onClick={() => selectPage(item.page)} aria-current={page === item.page ? "page" : undefined}><Icon name={item.icon} /><span className="sr-only">{t(lang, item.labelKey)}</span></button>)}</nav>
  </main></LanguageContext.Provider>;
}

function Icon({ name }: { name: IconName }) {
  const paths: Record<IconName, ReactNode> = {
    home: <path d="m3 10 9-7 9 7v10a1 1 0 0 1-1 1h-5v-6H9v6H4a1 1 0 0 1-1-1V10Z" />,
    chart: <><path d="M4 20V10m8 10V4m8 16v-7" /><path d="M2 20h20" /></>,
    sparkle: <path d="m12 2 1.8 6.2L20 10l-6.2 1.8L12 18l-1.8-6.2L4 10l6.2-1.8L12 2Zm7 13 .8 2.2L22 18l-2.2.8L19 21l-.8-2.2L16 18l2.2-.8L19 15Z" />,
    heart: <path d="M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78Z" />,
    user: <><circle cx="12" cy="8" r="3.5" /><path d="M4.5 21c.8-4 3.2-6 7.5-6s6.7 2 7.5 6" /></>,
    bell: <><path d="M18 10a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9" /><path d="M10 22h4" /></>,
    arrow: <path d="M5 12h14m-6-6 6 6-6 6" />,
    close: <path d="M18 6 6 18M6 6l12 12" />,
    clock: <><circle cx="12" cy="12" r="8.5" /><path d="M12 7v5l3.5 2" /></>,
    movie: <><rect x="3" y="5" width="18" height="15" rx="2" /><path d="M7 5v15m10-15v15M3 10h18" /></>,
    series: <><path d="M5 3h14v18H5z" /><path d="M8 7h8M8 11h8M8 15h5" /></>,
    genre: <><path d="M4 4h8l8 8-8 8-10-10V4Z" /><circle cx="9" cy="9" r="1" /></>,
    medal: <><circle cx="12" cy="14" r="6" /><path d="m8 3 2.5 5M16 3l-2.5 5M10 14l1.4 1.4L14.5 12" /></>,
    refresh: <><path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8" /><path d="M21 3v5h-5" /><path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16" /><path d="M8 16H3v5" /></>,
    chat: <path d="M4 4.5h16v11H9.5L5 20v-4.5H4v-11Z" />,
    settings: <><circle cx="12" cy="12" r="3" /><path d="M19.4 15a1.7 1.7 0 0 0 .34 1.87l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.7 1.7 0 0 0-1.87-.34 1.7 1.7 0 0 0-1.04 1.56V21a2 2 0 1 1-4 0v-.09A1.7 1.7 0 0 0 9 19.35a1.7 1.7 0 0 0-1.87.34l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06A1.7 1.7 0 0 0 4.65 15a1.7 1.7 0 0 0-1.56-1.04H3a2 2 0 1 1 0-4h.09A1.7 1.7 0 0 0 4.65 9a1.7 1.7 0 0 0-.34-1.87l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.7 1.7 0 0 0 9 4.65a1.7 1.7 0 0 0 1.04-1.56V3a2 2 0 1 1 4 0v.09A1.7 1.7 0 0 0 15 4.65a1.7 1.7 0 0 0 1.87-.34l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06A1.7 1.7 0 0 0 19.35 9c.14.62.68 1.34 1.56 1.04H21a2 2 0 1 1 0 4h-.09a1.7 1.7 0 0 0-1.51 1.96Z" /></>,
  };
  return <svg className="icon" viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">{paths[name]}</svg>;
}

function PersonAvatar({ name, src }: { name: string; src: string }) {
  const initial = name.trim().charAt(0).toUpperCase() || "?";
  return <span className="user-avatar"><span className="avatar-initial">{initial}</span><img src={src} alt="" width="44" height="44" onError={(event) => event.currentTarget.remove()} /></span>;
}

// UserAvatar's src must include the user id: an <img> only re-fetches when
// its src string actually changes, and "/api/me/avatar" alone stayed
// identical across a user switch on the same device (no full page reload
// in between), so the browser kept showing whichever bitmap was already
// painted into that element instead of requesting the new user's picture.
function UserAvatar({ name, userId }: { name: string; userId: string }) {
  return <PersonAvatar name={name} src={`/api/me/avatar?u=${encodeURIComponent(userId)}`} />;
}

function Today({ upcoming, upcomingState, cinema, cinemaState, requests, requestState, newForYou, newForYouState, topRated, topRatedState, seriesInProgress, seriesInProgressState, continueWatching, continueWatchingState, availableRequests, features, message, onSelectMedia, onOpenChats }: {
	upcoming: UpcomingItem[]; upcomingState: LoadState; requests: RequestItem[]; requestState: LoadState;
	cinema: UpcomingItem[]; cinemaState: LoadState;
  newForYou: NewForYouItem[]; newForYouState: LoadState; topRated: TopRatedItem[]; topRatedState: LoadState; availableRequests: RequestItem[];
  seriesInProgress: SeriesProgress[]; seriesInProgressState: LoadState;
  continueWatching: ContinueWatchingItem[]; continueWatchingState: LoadState;
  features: Features;
  message: { preview: string } | null;
  onSelectMedia: (selection: MediaSelection) => void; onOpenChats: () => void;
}) {
  const lang = useLang();
  const translate = useT();
  const [allEventsOpen, setAllEventsOpen] = useState(false);
  const [newForYouGridOpen, setNewForYouGridOpen] = useState(false);

  // Ohne Radarr fehlen Filmtermine, ohne Sonarr Serientermine — beide fließen
  // serverseitig in dieselbe Liste, deshalb wird hier nach Medientyp gefiltert
  // statt eine eigene Abfrage pro Dienst zu brauchen.
  const visibleUpcoming = upcoming.filter((item) => (item.mediaType === "movie" ? features.movieDates : features.seriesDates));

  const events = relevantEvents({
    availableRequests,
    upcoming: upcomingState === "ready" ? visibleUpcoming : [],
    cinema: cinemaState === "ready" ? cinema : [],
    newForYou: newForYouState === "ready" ? newForYou : [],
    message,
    seerrConfigured: features.requests,
    lang,
    onSelectMedia,
    onShowNewForYou: () => setNewForYouGridOpen(true),
    onOpenChats,
  });

  const newForYouDetail = (item: NewForYouItem) => item.dateCreated ? translate("added_when", { when: daysAgoWording(item.dateCreated, lang) }) : translate("unwatched");

  return <div className="content today-view">
    <RelevantNow events={events} onShowAll={() => setAllEventsOpen(true)} />
    <PosterRow title={translate("row_new_for_you")} eyebrow={translate("row_new_for_you_eyebrow")} items={newForYou} state={newForYouState} emptyLabel={translate("row_new_for_you_empty")} itemTitle={newForYouTitle} detail={newForYouDetail} onSelect={(item) => onSelectMedia({ source: "emby", id: item.id })} />
    <PosterRow title={translate("row_continue")} eyebrow={translate("row_continue_eyebrow")} items={continueWatching} state={continueWatchingState} emptyLabel={translate("row_continue_empty")} detail={(item) => translate("percent_watched", { percent: item.progressPercent })} progress={(item) => item.progressPercent} onSelect={(item) => onSelectMedia({ source: "emby", id: item.id })} />
    {features.upcoming && <PosterRow title={translate("row_upcoming")} eyebrow={translate("row_upcoming_eyebrow")} items={visibleUpcoming} state={upcomingState} emptyLabel={translate("row_upcoming_empty")} itemTitle={(item) => upcomingTitle(item, lang)} detail={(item) => availabilityWording(item.availabilityDate, lang)} onSelect={(item) => onSelectMedia(calendarSelection(item, features.requests))} />}
    <PosterRow title={translate("row_series_progress")} eyebrow={translate("row_series_progress_eyebrow")} items={seriesInProgress} state={seriesInProgressState} emptyLabel={translate("row_series_progress_empty")} detail={(item) => translate("episodes_of", { watched: item.watchedEpisodes, total: item.totalEpisodes })} progress={(item) => Math.round((item.watchedEpisodes / item.totalEpisodes) * 100)} onSelect={(item) => onSelectMedia({ source: "emby", id: item.id })} />
    {features.requests && <PosterRow title={translate("row_my_requests")} eyebrow={translate("row_my_requests_eyebrow")} items={requests} state={requestState} emptyLabel={translate("row_my_requests_empty")} detail={(item) => item.status} onSelect={(item) => onSelectMedia({ source: "seerr", id: item.tmdbId, mediaType: item.mediaType })} />}
    <PosterRow title={translate("row_top_rated")} eyebrow={translate("row_top_rated_eyebrow")} items={topRated} state={topRatedState} emptyLabel={translate("row_top_rated_empty")} detail={(item) => `★ ${item.averageRating.toFixed(1)}`} onSelect={(item) => onSelectMedia(item.mediaSource === "seerr" ? { source: "seerr", id: item.mediaId, mediaType: item.mediaType } : { source: "emby", id: item.mediaId })} />
    {features.movieDates && <PosterRow title={translate("row_cinema")} eyebrow={translate("row_cinema_eyebrow")} items={cinema} state={cinemaState} emptyLabel={translate("row_cinema_empty")} detail={(item) => cinemaWording(item, lang)} onSelect={(item) => onSelectMedia(calendarSelection(item, features.requests))} />}

    {allEventsOpen && <RelevantAllScreen events={events} onClose={() => setAllEventsOpen(false)} />}
    {newForYouGridOpen && <MediaGridScreen title={translate("row_new_for_you")} items={newForYou} itemTitle={newForYouTitle} detail={newForYouDetail} onSelect={(item) => onSelectMedia({ source: "emby", id: item.id })} onClose={() => setNewForYouGridOpen(false)} />}
  </div>;
}

type RelevantEvent = { key: string; tone: Tone; icon: IconName; status: string; detail: ReactNode; onOpen: () => void };

const RELEASE_WINDOW_HOURS = 48;

function releaseWording(premiereDate: string, lang: Lang) {
  const date = new Date(premiereDate);
  if (Number.isNaN(date.getTime())) return t(lang, "release_soon");
  const startOfToday = new Date();
  startOfToday.setHours(0, 0, 0, 0);
  const days = Math.round((new Date(date).setHours(0, 0, 0, 0) - startOfToday.getTime()) / 86_400_000);
  if (days <= 0) return t(lang, "release_today");
  if (days === 1) return t(lang, "release_tomorrow");
  return t(lang, "release_soon");
}

// Builds the prioritised event list: newly available requests first, then
// releases due within the next two days, then the unseen-titles summary.
function relevantEvents({ availableRequests, upcoming, cinema, newForYou, message, seerrConfigured, lang, onSelectMedia, onShowNewForYou, onOpenChats }: {
  availableRequests: RequestItem[]; upcoming: UpcomingItem[]; cinema: UpcomingItem[]; newForYou: NewForYouItem[];
  message: { preview: string } | null; seerrConfigured: boolean; lang: Lang;
  onSelectMedia: (selection: MediaSelection) => void; onShowNewForYou: () => void; onOpenChats: () => void;
}): RelevantEvent[] {
  const events: RelevantEvent[] = [];

  if (message) {
    events.push({
      key: "message", tone: "blue", icon: "chat", status: t(lang, "event_new_message"),
      detail: message.preview,
      onOpen: onOpenChats,
    });
  }

  for (const request of availableRequests) {
    events.push({
      key: `available-${request.id}`, tone: "mint", icon: "sparkle", status: t(lang, "event_now_available"),
      detail: request.availableSince
        ? t(lang, "event_request_available_since", { title: request.title, when: daysAgoWording(request.availableSince, lang) })
        : t(lang, "event_request_available", { title: request.title }),
      onOpen: () => onSelectMedia({ source: "seerr", id: request.tmdbId, mediaType: request.mediaType }),
    });
  }

  const horizon = Date.now() + RELEASE_WINDOW_HOURS * 60 * 60 * 1000;
  for (const item of upcoming) {
    const premiere = new Date(item.availabilityDate).getTime();
    if (Number.isNaN(premiere) || premiere > horizon) continue;
    events.push({
      key: `release-${item.id}`, tone: "lilac", icon: "clock", status: upcomingTitle(item, lang),
      detail: releaseWording(item.availabilityDate, lang),
      onOpen: () => onSelectMedia(calendarSelection(item, seerrConfigured)),
    });
  }

  for (const item of cinema) {
    const premiere = new Date(item.cinemaStartDate ?? item.availabilityDate).getTime();
    if (Number.isNaN(premiere) || premiere > horizon || premiere < Date.now() - 24 * 60 * 60 * 1000) continue;
    events.push({
      key: `cinema-${item.id}`, tone: "lilac", icon: "movie", status: item.title,
      detail: premiere <= Date.now() ? t(lang, "event_cinema_now") : t(lang, "event_cinema_soon"),
      onOpen: () => onSelectMedia(calendarSelection(item, seerrConfigured)),
    });
  }

  if (newForYou.length > 0) {
    events.push({
      key: "new-for-you", tone: "blue", icon: "genre", status: t(lang, "row_new_for_you"),
      detail: <><b>{newForYou.length}</b> {t(lang, "unwatched_titles")}</>,
      onOpen: onShowNewForYou,
    });
  }

  return events;
}

function RelevantNow({ events, onShowAll }: { events: RelevantEvent[]; onShowAll: () => void }) {
  const translate = useT();
  return <section className="relevant-card" aria-label={translate("relevant_now")}>
    <div className="relevant-head">
      <p className="eyebrow">{translate("relevant_now_eyebrow")}</p>
      {events.length > 3 && <button type="button" className="text-button" onClick={onShowAll}>{translate("view_all")} <Icon name="arrow" /></button>}
    </div>
    {events.length === 0
      ? <p className="relevant-empty">{translate("relevant_empty")}</p>
      : <ul className="relevant-list">{events.slice(0, 3).map((event) => <RelevantRow key={event.key} event={event} />)}</ul>}
  </section>;
}

function RelevantRow({ event, onOpen }: { event: RelevantEvent; onOpen?: () => void }) {
  return <li>
    <button type="button" className={`relevant-row tone-${event.tone}`} onClick={onOpen ?? event.onOpen}>
      <span className="relevant-icon"><Icon name={event.icon} /></span>
      <span className="relevant-text"><strong>{event.status}</strong><small>{event.detail}</small></span>
      <span className="relevant-chevron"><Icon name="arrow" /></span>
    </button>
  </li>;
}

function RelevantAllScreen({ events, onClose }: { events: RelevantEvent[]; onClose: () => void }) {
  useEscapeKey(onClose);
  useBodyScrollLock();
  const translate = useT();

  return <div className="media-detail-overlay media-grid-overlay" role="dialog" aria-modal="true" aria-label={translate("relevant_now")}>
    <div className="media-detail-scroll">
      <button type="button" className="media-detail-close" onClick={onClose} aria-label={translate("close")}><Icon name="close" /></button>
      <h1 className="media-grid-title">{translate("relevant_now")}</h1>
      <ul className="relevant-list">{events.map((event) => <RelevantRow key={event.key} event={event} onOpen={() => { onClose(); event.onOpen(); }} />)}</ul>
    </div>
  </div>;
}

function MetricCard({ icon, tone, value, label, detail, positive, loading, onClick }: { icon: IconName; tone: Tone; value: string | number; label: string; detail: string; positive?: boolean; loading?: boolean; onClick?: () => void }) {
  const translate = useT();
  const inner = loading
    ? <><span className="metric-icon"><Icon name={icon} /></span><span className="skeleton skeleton-value" aria-hidden="true" /><p>{label}</p><span className="skeleton skeleton-detail" aria-hidden="true" /><span className="sr-only" role="status">{translate("loading")}</span></>
    : <><span className="metric-icon"><Icon name={icon} /></span><strong>{value}</strong><p>{label}</p><small className={positive ? "up" : undefined}>{detail}</small></>;
  return onClick
    ? <button type="button" className={`metric-card metric-card-button tone-${tone}`} onClick={onClick}>{inner}</button>
    : <article className={`metric-card tone-${tone}`}>{inner}</article>;
}

// Replaces two separate "Filme abgeschlossen"/"Serien abgeschlossen"
// MetricCards — the period was redundant with the period tabs directly
// above this grid, and two full-height cards for one number each felt
// heavy, so both counts live in one compact card as independently
// clickable segments instead.
function CompletedCard({ movies, series, loading, onOpenMovies, onOpenSeries }: { movies?: number; series?: number; loading?: boolean; onOpenMovies?: () => void; onOpenSeries?: () => void }) {
  const translate = useT();
  return <article className="metric-card completed-card tone-peach">
    <button type="button" className="completed-card-segment" onClick={onOpenMovies} disabled={!onOpenMovies}>
      <span className="metric-icon"><Icon name="movie" /></span>
      {loading ? <span className="skeleton skeleton-value" aria-hidden="true" /> : <strong>{movies ?? "—"}</strong>}
      <small>{translate("movies")}</small>
    </button>
    <span className="completed-card-divider" aria-hidden="true" />
    <button type="button" className="completed-card-segment" onClick={onOpenSeries} disabled={!onOpenSeries}>
      <span className="metric-icon"><Icon name="series" /></span>
      {loading ? <span className="skeleton skeleton-value" aria-hidden="true" /> : <strong>{series ?? "—"}</strong>}
      <small>{translate("series")}</small>
    </button>
  </article>;
}

// Combines the two "Rekorde" metrics into the same compact two-segment
// layout as CompletedCard, so they read as a single 4th tile in the week
// grid instead of a separate, half-width row below it. Neither segment
// navigates anywhere, so the divider/labels stay but the button semantics
// (and cursor) don't. Longest session drops its item-name subtitle and
// the record day uses the short day/month date (no year) — both existed
// only as MetricCards' third detail line, which doesn't fit two metrics
// side by side in one card.
function RecordsCard({ longestSession, longestSessionState, mostActiveDay, mostActiveDayState }: {
  longestSession: LongestSession | null; longestSessionState: LoadState;
  mostActiveDay: MostActiveDay | null; mostActiveDayState: LoadState;
}) {
  const lang = useLang();
  const translate = useT();
  return <article className="metric-card completed-card tone-lilac">
    <div className="completed-card-segment static">
      <span className="metric-icon"><Icon name="clock" /></span>
      {longestSessionState === "loading"
        ? <span className="skeleton skeleton-value" aria-hidden="true" />
        : <strong>{longestSessionState === "ready" && longestSession ? formatDuration(longestSession.watchSeconds, lang) : "—"}</strong>}
      <small>{translate("longest_session")}</small>
    </div>
    <span className="completed-card-divider" aria-hidden="true" />
    <div className="completed-card-segment static">
      <span className="metric-icon"><Icon name="genre" /></span>
      {mostActiveDayState === "loading"
        ? <span className="skeleton skeleton-value" aria-hidden="true" />
        : <strong>{mostActiveDayState === "ready" && mostActiveDay ? formatPremiereDate(mostActiveDay.date, lang) : "—"}</strong>}
      <small>{translate("most_active_day")}</small>
    </div>
  </article>;
}

function WatchedCategoryTile({ label, items, onOpen }: { label: string; items: readonly WatchedItem[]; onOpen: () => void }) {
  const translate = useT();
  const featured = items.length > 0
    ? [...items].sort((a, b) => (b.dateAdded ?? "").localeCompare(a.dateAdded ?? ""))[0]
    : undefined;
  return <button
    type="button"
    className="watched-tile"
    style={featured?.backdropUrl ? { backgroundImage: `url(${featured.backdropUrl})` } : undefined}
    onClick={onOpen}
    aria-label={translate("show_label_aria", { label })}
  >
    <span className="watched-tile-scrim" />
    <span className="watched-tile-label">{label}</span>
  </button>;
}

// Same vertical flow as MetricCard (icon, then strong/p/small stacked in
// normal document flow) instead of the previous side-by-side avatar+text
// row — that horizontal layout needed the avatar and a wrapping subtitle to
// both fit a fixed row height, and on narrower widths the text overflowed
// above the card instead of just growing the box like every other card.
function RankCard({ rank, name, userId }: { rank: number | null; name: string; userId: string }) {
  const translate = useT();
  const hasRank = rank !== null && rank > 0;
  const medalClass = rank === 1 ? " gold" : rank === 3 ? " bronze" : "";
  return <article className="metric-card rank-card tone-lilac">
    <span className="rank-avatar-badge">
      <span className="rank-avatar"><UserAvatar name={name} userId={userId} /></span>
      <span className={`rank-badge${medalClass}`}>{hasRank ? rank : "—"}</span>
    </span>
    <strong>{hasRank ? translate("rank_place", { rank }) : "—"}</strong>
    <small>{translate("rank_by_watch_time")}</small>
  </article>;
}

function PosterRow<T extends { id: string; title: string; posterUrl?: string }>({ title, eyebrow, gridTitle, items, detail, itemTitle, state, emptyLabel, progress, onSelect }: {
  title?: string; eyebrow?: string; gridTitle?: string; items: readonly T[]; detail: (item: T) => string; itemTitle?: (item: T) => string; state?: LoadState; emptyLabel?: string; progress?: (item: T) => number; onSelect?: (item: T) => void;
}) {
  const translate = useT();
  const [gridOpen, setGridOpen] = useState(false);
  const resolvedGridTitle = gridTitle ?? title ?? translate("overview");
  return <section className="poster-section">
    {(title || eyebrow) && <div className="section-heading">
      <div>{eyebrow && <p className="eyebrow">{eyebrow}</p>}{title && <h2>{title}</h2>}</div>
      {items.length > 0 && <button type="button" className="text-button poster-view-all" onClick={() => setGridOpen(true)} aria-label={translate("show_all_aria", { title: resolvedGridTitle })}><Icon name="arrow" /></button>}
    </div>}
    {state === "loading" && <PosterSkeletonRow />}
    {state === "error" && <p className="poster-status">{translate("not_available")}</p>}
    {state !== "loading" && state !== "error" && items.length === 0 && <p className="poster-status">{emptyLabel ?? translate("nothing_here")}</p>}
    {items.length > 0 && <div className="poster-scroller">{items.map((item) => {
      const inner = <>
        <div className="poster wide" role="img" aria-label={itemTitle?.(item) ?? item.title}>
          <PosterImage src={item.posterUrl} fallback={itemTitle?.(item) ?? item.title} />
          {progress && <div className="poster-progress"><div className="poster-progress-fill" style={{ width: `${progress(item)}%` }} /></div>}
        </div>
        <strong>{itemTitle?.(item) ?? item.title}</strong><small className={detail(item).includes("★") ? "rating-stars" : undefined}>{detail(item)}</small>
      </>;
      return onSelect
        ? <button type="button" className="poster-entry poster-entry-button" key={item.id} onClick={() => onSelect(item)}>{inner}</button>
        : <article className="poster-entry" key={item.id}>{inner}</article>;
    })}</div>}
    {gridOpen && <MediaGridScreen title={resolvedGridTitle} items={items} detail={detail} progress={progress} onSelect={onSelect} onClose={() => setGridOpen(false)} />}
  </section>;
}

function PosterSkeletonRow({ count = 4 }: { count?: number }) {
  const translate = useT();
  return <>
    <p className="poster-status sr-only" role="status">{translate("loading")}</p>
    <div className="poster-scroller" aria-hidden="true">
      {Array.from({ length: count }, (_, index) => <div className="poster-entry" key={index}>
        <div className="poster wide skeleton" />
        <span className="skeleton skeleton-line" /><span className="skeleton skeleton-line" />
      </div>)}
    </div>
  </>;
}

function topGenres(movies: readonly WatchedItem[], series: readonly WatchedItem[]) {
  const counts = new Map<string, number>();
  for (const item of [...movies, ...series]) {
    for (const genre of item.genres) counts.set(genre, (counts.get(genre) ?? 0) + 1);
  }
  return [...counts.entries()].sort((a, b) => b[1] - a[1]).slice(0, 6).map(([label, value]) => ({ label, value }));
}

function watchedItemDetail(item: WatchedItem, lang: Lang) {
  const genre = item.genres[0] ?? "";
  if (!item.lastPlayedDate) return genre;
  const lastPlayed = t(lang, "last_played_when", { when: daysAgoWording(item.lastPlayedDate, lang) });
  return genre ? `${genre} · ${lastPlayed}` : lastPlayed;
}

const WEEKDAY_KEYS: TranslationKey[] = ["weekday_mon", "weekday_tue", "weekday_wed", "weekday_thu", "weekday_fri", "weekday_sat", "weekday_sun"];

function weekdayChartData(weekdays: readonly WeekdayWatchTime[], lang: Lang) {
  const seconds = new Array(7).fill(0);
  for (const entry of weekdays) seconds[entry.weekday] = entry.watchSeconds;
  return WEEKDAY_KEYS.map((key, index) => ({ label: t(lang, key), value: seconds[index] }));
}

const DAYPARTS: { key: TranslationKey; hours: number[] }[] = [
  { key: "daypart_night", hours: [0, 1, 2, 3, 4, 5] },
  { key: "daypart_morning", hours: [6, 7, 8, 9, 10, 11] },
  { key: "daypart_afternoon", hours: [12, 13, 14, 15, 16, 17] },
  { key: "daypart_evening", hours: [18, 19, 20, 21, 22, 23] },
];

function daypartChartData(hours: readonly HourWatchTime[], lang: Lang) {
  const secondsByHour = new Array(24).fill(0);
  for (const entry of hours) secondsByHour[entry.hour] = entry.watchSeconds;
  return DAYPARTS.map((daypart) => ({ label: t(lang, daypart.key), value: daypart.hours.reduce((sum, hour) => sum + secondsByHour[hour], 0) }));
}

function BarChart({ title, subtitle, data, formatValue, loading }: { title: string; subtitle?: string; data: { label: string; value: number }[]; formatValue?: (value: number) => string; loading?: boolean }) {
  const translate = useT();
  const max = Math.max(1, ...data.map((entry) => entry.value));
  const hasData = data.some((entry) => entry.value > 0);
  return <section className="chart-card">
    <h3>{title}</h3>
    {subtitle && <p className="chart-card-subtitle">{subtitle}</p>}
    {loading
      ? <div className="bar-chart" aria-hidden="true">
        <p className="sr-only" role="status">{translate("loading")}</p>
        {Array.from({ length: 4 }, (_, index) => <div className="bar-row" key={index}>
          <span className="skeleton skeleton-line-xs" />
          <div className="bar-track"><div className="skeleton bar-fill-skeleton" /></div>
          <span className="skeleton skeleton-line-xs" />
        </div>)}
      </div>
      : hasData
      ? <div className="bar-chart" role="img" aria-label={title}>
        {data.map((entry) => <div className="bar-row" key={entry.label}>
          <span className="bar-label">{entry.label}</span>
          <div className="bar-track"><div className="bar-fill" style={{ width: `${(entry.value / max) * 100}%` }} /></div>
          <span className="bar-value">{formatValue ? formatValue(entry.value) : entry.value}</span>
        </div>)}
      </div>
      : <p className="poster-status">{translate("chart_no_data")}</p>}
  </section>;
}

function Stats({ user, onSelectMedia }: { user: { id: string; name: string }; onSelectMedia: (selection: MediaSelection) => void }) {
  const lang = useLang();
  const translate = useT();
  const [period, setPeriod] = useState<Period>("week");
  const [completedGridView, setCompletedGridView] = useState<"movies" | "series" | null>(null);
  const [watchedGridView, setWatchedGridView] = useState<"movies" | "series" | null>(null);
  const periodLabel = translate(periodLabelKey[period]);

  const [statistics, state] = useApiResource<PersonalStats | null>(`/api/stats?period=${period}`, null);
  const [watchTimeRankData] = useApiResource<WatchTimeRank | null>("/api/stats/rank", null);
  const watchTimeRank = watchTimeRankData?.rank ?? null;
  const [watchedMovies, watchedMoviesState] = useApiResource<WatchedItem[]>("/api/watched-movies", []);
  const [watchedSeries, watchedSeriesState] = useApiResource<WatchedItem[]>("/api/watched-series", []);
  const [completedMovies, completedMoviesState] = useApiResource<WatchedItem[]>(`/api/completed-movies?period=${period}`, []);
  const [completedSeries, completedSeriesState] = useApiResource<WatchedItem[]>(`/api/completed-series?period=${period}`, []);
  const [deviceStats, deviceStatsState] = useApiResource<DeviceWatchTime[]>(`/api/stats/devices?period=${period}`, []);
  const [hourStats, hourStatsState] = useApiResource<HourWatchTime[]>(`/api/stats/hours?period=${period}`, []);
  const [weekdayStats, weekdayStatsState] = useApiResource<WeekdayWatchTime[]>(`/api/stats/weekdays?period=${period}`, []);
  const [longestSession, longestSessionState] = useApiResource<LongestSession | null>(`/api/stats/longest-session?period=${period}`, null);
  const [mostActiveDay, mostActiveDayState] = useApiResource<MostActiveDay | null>(`/api/stats/most-active-day?period=${period}`, null);

  return <div className="content page-view">
    <section className="period-tabs" aria-label={translate("select_period")}>{(["week", "month", "year"] as Period[]).map((item) => <button className={period === item ? "selected" : ""} onClick={() => setPeriod(item)} key={item} aria-pressed={period === item}>{translate(periodLabelKey[item])}</button>)}</section>
    {statistics?.periodStartsAt && statistics?.periodEndsAt && <p className="period-range">{dateRangeWording(statistics.periodStartsAt, statistics.periodEndsAt, lang)}</p>}
    <section className="week-grid" aria-label={translate("metrics_for", { period: periodLabel })}>
      <RankCard rank={watchTimeRank} name={user.name} userId={user.id} />
      <MetricCard icon="clock" tone="blue" value={statistics ? formatDuration(statistics.watchSeconds, lang) : "—"} label={translate("watch_time")} detail={statistics ? comparisonText(statistics, lang) : loadingCopy(state, lang)} loading={state === "loading"} />
      <CompletedCard
        loading={state === "loading"}
        movies={statistics?.completedMovies}
        series={statistics?.completedSeries}
        onOpenMovies={statistics && statistics.completedMovies > 0 && completedMoviesState === "ready" ? () => setCompletedGridView("movies") : undefined}
        onOpenSeries={statistics && statistics.completedSeries > 0 && completedSeriesState === "ready" ? () => setCompletedGridView("series") : undefined}
      />
      <RecordsCard longestSession={longestSession} longestSessionState={longestSessionState} mostActiveDay={mostActiveDay} mostActiveDayState={mostActiveDayState} />
    </section>

    <section className="chart-grid">
      <BarChart title={translate("chart_top_genres")} subtitle={statistics?.favouriteGenre ? translate("chart_favourite_genre", { genre: statistics.favouriteGenre }) : undefined} data={topGenres(watchedMovies, watchedSeries)} loading={watchedMoviesState === "loading" || watchedSeriesState === "loading"} />
      <BarChart title={translate("chart_by_weekday")} data={weekdayStatsState === "ready" ? weekdayChartData(weekdayStats, lang) : []} formatValue={(value) => formatDuration(value, lang)} loading={weekdayStatsState === "loading"} />
      <BarChart title={translate("chart_by_hour")} data={hourStatsState === "ready" ? daypartChartData(hourStats, lang) : []} formatValue={(value) => formatDuration(value, lang)} loading={hourStatsState === "loading"} />
      <BarChart title={translate("chart_by_device")} data={deviceStatsState === "ready" ? deviceStats.slice(0, 6).map((device) => ({ label: device.deviceName, value: device.watchSeconds })) : []} formatValue={(value) => formatDuration(value, lang)} loading={deviceStatsState === "loading"} />
    </section>

    <section aria-label={translate("watched_content")}>
      <h2 className="watched-tiles-heading">{translate("fully_watched")}</h2>
      <div className="watched-tiles">
        <WatchedCategoryTile label={translate("movies")} items={watchedMovies} onOpen={() => setWatchedGridView("movies")} />
        <WatchedCategoryTile label={translate("series")} items={watchedSeries} onOpen={() => setWatchedGridView("series")} />
      </div>
    </section>

    {completedGridView === "movies" && <MediaGridScreen title={translate("grid_completed_movies", { period: periodLabel })} items={completedMovies} detail={(item) => item.genres[0] ?? ""} onSelect={(item) => onSelectMedia({ source: "emby", id: item.id })} onClose={() => setCompletedGridView(null)} />}
    {completedGridView === "series" && <MediaGridScreen title={translate("grid_completed_series", { period: periodLabel })} items={completedSeries} detail={(item) => item.genres[0] ?? ""} onSelect={(item) => onSelectMedia({ source: "emby", id: item.id })} onClose={() => setCompletedGridView(null)} />}
    {watchedGridView === "movies" && <MediaGridScreen title={translate("grid_watched_movies")} items={watchedMovies} detail={(item) => watchedItemDetail(item, lang)} onSelect={(item) => onSelectMedia({ source: "emby", id: item.id })} onClose={() => setWatchedGridView(null)} />}
    {watchedGridView === "series" && <MediaGridScreen title={translate("grid_watched_series")} items={watchedSeries} detail={(item) => watchedItemDetail(item, lang)} onSelect={(item) => onSelectMedia({ source: "emby", id: item.id })} onClose={() => setWatchedGridView(null)} />}
  </div>;
}

function MediaGridScreen<T extends { id: string; title: string; posterUrl?: string }>({ title, items, detail, itemTitle, progress, state, emptyLabel, headerExtra, onSelect, onClose }: {
  title: string; items: readonly T[]; detail?: (item: T) => string; itemTitle?: (item: T) => string; progress?: (item: T) => number;
  state?: LoadState; emptyLabel?: string; headerExtra?: ReactNode;
  onSelect?: (item: T) => void; onClose: () => void;
}) {
  useEscapeKey(onClose);
  useBodyScrollLock();
  const translate = useT();

  return <div className="media-detail-overlay media-grid-overlay" role="dialog" aria-modal="true" aria-label={title}>
    <div className="media-detail-scroll">
      <button type="button" className="media-detail-close" onClick={onClose} aria-label={translate("close")}><Icon name="close" /></button>
      <h1 className="media-grid-title">{title}</h1>
      {headerExtra}
      {state === "loading" && <>
        <p className="poster-status sr-only" role="status">{translate("loading")}</p>
        <div className="media-grid" aria-hidden="true">
          {Array.from({ length: 8 }, (_, index) => <div className="media-grid-entry" key={index}>
            <div className="poster wide skeleton" />
            <span className="skeleton skeleton-line" /><span className="skeleton skeleton-line" />
          </div>)}
        </div>
      </>}
      {state === "error" && <p className="poster-status">{translate("not_available")}</p>}
      {state !== "loading" && state !== "error" && items.length === 0 && <p className="poster-status">{emptyLabel ?? translate("nothing_here")}</p>}
      {items.length > 0 && <div className="media-grid">{items.map((item) => <button type="button" className="media-grid-entry" key={item.id} onClick={() => onSelect?.(item)}>
        <div className="poster wide" role="img" aria-label={itemTitle?.(item) ?? item.title}>
          <PosterImage src={item.posterUrl} fallback={item.title} />
          {progress && <div className="poster-progress"><div className="poster-progress-fill" style={{ width: `${progress(item)}%` }} /></div>}
        </div>
        <strong>{itemTitle?.(item) ?? item.title}</strong>
        {detail && <small>{detail(item)}</small>}
      </button>)}</div>}
    </div>
  </div>;
}

function useDiscoverList(path: string) {
  const [items, setItems] = useState<DiscoverItem[]>([]);
  const [state, setState] = useState<LoadState>("loading");
  useEffect(() => {
    let active = true;
    fetch(path, { credentials: "include" })
      .then(async (response) => { if (!response.ok) throw new Error("discover list unavailable"); const data = await response.json(); if (active) { setItems(data); setState("ready"); } })
      .catch(() => active && setState("error"));
    return () => { active = false; };
  }, [path]);
  return [items, state] as const;
}

const STREAMING_PROVIDERS: { id: string; name: string }[] = [
  { id: "8", name: "Netflix" },
  { id: "337", name: "Disney+" },
  { id: "9", name: "Prime Video" },
  { id: "350", name: "Apple TV+" },
  { id: "531", name: "Paramount+" },
  { id: "1899", name: "HBO Max" },
  { id: "30", name: "WOW" },
];

function Requests({ onSelectMedia }: { onSelectMedia: (selection: MediaSelection) => void }) {
  const translate = useT();
  const [trending, trendingState] = useDiscoverList("/api/discover/trending");
  const [popularMovies, popularMoviesState] = useDiscoverList("/api/discover/movies/popular");
  const [upcomingMovies, upcomingMoviesState] = useDiscoverList("/api/discover/movies/upcoming");
  const [popularSeries, popularSeriesState] = useDiscoverList("/api/discover/series/popular");
  const [upcomingSeries, upcomingSeriesState] = useDiscoverList("/api/discover/series/upcoming");

  const [query, setQuery] = useState("");
  const [searchResults, setSearchResults] = useState<DiscoverItem[]>([]);
  const [searchState, setSearchState] = useState<LoadState>("loading");
  const [searchScreenQuery, setSearchScreenQuery] = useState<string | null>(null);

  const [providerId, setProviderId] = useState<string | null>(null);
  const [providerItems, providerItemsState] = useApiResource<DiscoverItem[]>(providerId ? `/api/discover/provider?id=${encodeURIComponent(providerId)}` : null, []);
  const selectedProvider = STREAMING_PROVIDERS.find((provider) => provider.id === providerId) ?? null;

  const runSearch = (searchQuery: string) => {
    const trimmed = searchQuery.trim();
    if (!trimmed) return;
    setSearchScreenQuery(trimmed);
    setSearchState("loading");
    fetch(`/api/discover/search?query=${encodeURIComponent(trimmed)}`, { credentials: "include" })
      .then(async (response) => {
        if (!response.ok) throw new Error("search unavailable");
        const data = await response.json();
        setSearchResults(data);
        setSearchState("ready");
      })
      .catch(() => setSearchState("error"));
  };

  return <div className="content page-view">
    <form className="search-form" onSubmit={(event) => { event.preventDefault(); runSearch(query); }}>
      <input
        type="search"
        className="search-input"
        placeholder={translate("search_placeholder")}
        value={query}
        onChange={(event) => setQuery(event.target.value)}
        aria-label={translate("search_aria")}
      />
      <button type="submit" className="search-button" disabled={query.trim() === ""}>{translate("search_submit")}</button>
    </form>
    {searchScreenQuery !== null && <MediaGridScreen
      title={translate("search_results_for", { query: searchScreenQuery })}
      items={searchResults}
      state={searchState}
      emptyLabel={translate("no_results")}
      detail={(item) => translate(item.mediaType === "tv" ? "media_type_series" : "media_type_movie")}
      onSelect={(item) => onSelectMedia({ source: "seerr", id: item.id, mediaType: item.mediaType })}
      onClose={() => setSearchScreenQuery(null)}
      headerExtra={<form className="search-form media-grid-search" onSubmit={(event) => { event.preventDefault(); runSearch(query); }}>
        <input type="search" className="search-input" placeholder={translate("search_placeholder")} value={query} onChange={(event) => setQuery(event.target.value)} aria-label={translate("search_aria")} />
        <button type="submit" className="search-button" disabled={query.trim() === ""}>{translate("search_submit")}</button>
      </form>}
    />}

    <PosterRow title={translate("row_trending")} eyebrow="SEERR · TMDB" items={trending} state={trendingState} emptyLabel={translate("row_trending_empty")} detail={(item) => translate(item.mediaType === "tv" ? "media_type_series" : "media_type_movie")} onSelect={(item) => onSelectMedia({ source: "seerr", id: item.id, mediaType: item.mediaType })} />
    <PosterRow title={translate("row_popular_movies")} eyebrow="SEERR · TMDB" items={popularMovies} state={popularMoviesState} emptyLabel={translate("no_data")} detail={() => translate("label_popular")} onSelect={(item) => onSelectMedia({ source: "seerr", id: item.id, mediaType: item.mediaType })} />
    <PosterRow title={translate("row_upcoming_movies")} eyebrow="SEERR · TMDB" items={upcomingMovies} state={upcomingMoviesState} emptyLabel={translate("no_data")} detail={() => translate("label_upcoming")} onSelect={(item) => onSelectMedia({ source: "seerr", id: item.id, mediaType: item.mediaType })} />
    <PosterRow title={translate("row_popular_series")} eyebrow="SEERR · TMDB" items={popularSeries} state={popularSeriesState} emptyLabel={translate("no_data")} detail={() => translate("label_popular")} onSelect={(item) => onSelectMedia({ source: "seerr", id: item.id, mediaType: item.mediaType })} />
    <PosterRow title={translate("row_upcoming_series")} eyebrow="SEERR · TMDB" items={upcomingSeries} state={upcomingSeriesState} emptyLabel={translate("no_data")} detail={() => translate("label_upcoming")} onSelect={(item) => onSelectMedia({ source: "seerr", id: item.id, mediaType: item.mediaType })} />

    <section className="poster-section">
      <div className="section-heading"><div><p className="eyebrow">{translate("providers_eyebrow")}</p><h2>{translate("providers")}</h2></div></div>
      <div className="provider-scroller">
        {STREAMING_PROVIDERS.map((provider) => <button type="button" key={provider.id} className="provider-chip" onClick={() => setProviderId(provider.id)}>{provider.name}</button>)}
      </div>
    </section>
    {selectedProvider && <MediaGridScreen
      title={selectedProvider.name}
      items={providerItems}
      state={providerItemsState}
      emptyLabel={translate("no_titles_found")}
      detail={(item) => translate(item.mediaType === "tv" ? "media_type_series" : "media_type_movie")}
      onSelect={(item) => onSelectMedia({ source: "seerr", id: item.id, mediaType: item.mediaType })}
      onClose={() => setProviderId(null)}
    />}
  </div>;
}

const OVERVIEW_CLAMP_THRESHOLD = 220;

function OverviewText({ text }: { text: string }) {
  const translate = useT();
  const [expanded, setExpanded] = useState(false);
  const isLong = text.length > OVERVIEW_CLAMP_THRESHOLD;
  return <>
    <p className={isLong && !expanded ? "media-detail-overview-text clamped" : "media-detail-overview-text"}>{text}</p>
    {isLong && <button type="button" className="text-button overview-toggle" onClick={() => setExpanded((value) => !value)}>{translate(expanded ? "show_less" : "show_more")}</button>}
  </>;
}
function MediaDetailScreen({ selection, seerrConfigured, onClose, onRequestCreated, onHiddenChanged }: { selection: MediaSelection; seerrConfigured: boolean; onClose: () => void; onRequestCreated: () => void; onHiddenChanged?: () => void }) {
  useBodyScrollLock();
  const lang = useLang();
  const translate = useT();
  const [detail, setDetail] = useState<MediaDetail | null>(null);
  const [state, setState] = useState<LoadState>("loading");
  const [selectedSeasons, setSelectedSeasons] = useState<number[]>([]);
  const [requestState, setRequestState] = useState<"idle" | "submitting" | "done" | "error">("idle");
  const [requestModalOpen, setRequestModalOpen] = useState(false);
  const [tracking, setTracking] = useState<{ rating: number; onWatchlist: boolean; hiddenInProgress: boolean }>({ rating: 0, onWatchlist: false, hiddenInProgress: false });
  const [favorite, setFavorite] = useState(false);
  const [favoriteBusy, setFavoriteBusy] = useState(false);
  const [markPlayedBusy, setMarkPlayedBusy] = useState(false);
  const mediaType = selection.source === "emby" ? undefined : selection.mediaType;

  // Seerr can take a moment to reflect a just-created request in its own
  // media/season status (the request itself is immediate, but the show's
  // mediaInfo sync lags slightly) — reopening the detail screen right after
  // requesting can briefly show the season as requestable again. This
  // remembers what this browser itself just requested so the UI stays on
  // "Angefragt ✓" regardless of what a fresh, still-lagging fetch reports.
  // movieRequestedSentinel stands in for "the movie itself", since movies
  // have no season numbers to key off of.
  const movieRequestedSentinel = -1;
  const requestedStorageKey = selection.source === "seerr" ? `insights:requested:${selection.mediaType}:${selection.id}` : null;
  const [locallyRequestedSeasons, setLocallyRequestedSeasons] = useState<number[]>(() => {
    if (!requestedStorageKey || typeof window === "undefined") return [];
    try {
      const raw = window.localStorage.getItem(requestedStorageKey);
      return raw ? (JSON.parse(raw) as number[]) : [];
    } catch {
      return [];
    }
  });

  // Guards against the initial GET resolving after the user already saved a
  // local change (e.g. clicked "Erneut gesehen" on a slow connection) — a
  // late GET response would otherwise clobber the newer optimistic value
  // back down to whatever was in the DB when the fetch started.
  const trackingWrittenRef = useRef(false);

  useEffect(() => {
    let active = true;
    trackingWrittenRef.current = false;
    fetch(`/api/tracking?source=${selection.source}&id=${encodeURIComponent(selection.id)}`, { credentials: "include" })
      .then(async (response) => {
        if (!response.ok) throw new Error("tracking unavailable");
        const data: MediaTrackingEntry = await response.json();
        if (active && !trackingWrittenRef.current) setTracking({ rating: data.rating ?? 0, onWatchlist: data.onWatchlist, hiddenInProgress: data.hiddenInProgress ?? false });
      })
      .catch(() => null);
    return () => { active = false; };
  }, [selection.source, selection.id]);

  // Always resends hiddenInProgress (Upsert overwrites the whole row), so a
  // plain rating/watchlist change never silently un-hides a dismissed series.
  const saveTracking = (next: { rating: number; onWatchlist: boolean; hiddenInProgress: boolean }) => {
    trackingWrittenRef.current = true;
    setTracking(next);
    if (!detail) return;
    fetch("/api/tracking", {
      method: "PUT",
      credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        mediaSource: selection.source,
        mediaId: selection.id,
        mediaType: selection.source === "seerr" ? selection.mediaType : (detail.isSeries ? "series" : "movie"),
        title: detail.title,
        posterUrl: detail.posterUrl,
        rating: next.rating,
        onWatchlist: next.onWatchlist,
        hiddenInProgress: next.hiddenInProgress,
      }),
    }).then((response) => { if (response.ok) onHiddenChanged?.(); }).catch(() => null);
  };

  const toggleFavorite = async () => {
    if (selection.source !== "emby" || favoriteBusy) return;
    setFavoriteBusy(true);
    const next = !favorite;
    setFavorite(next);
    try {
      const response = await fetch(`/api/media/emby/favorite?itemId=${encodeURIComponent(selection.id)}`, { method: next ? "POST" : "DELETE", credentials: "include" });
      if (!response.ok) throw new Error("favorite update failed");
    } catch {
      setFavorite(!next);
    } finally {
      setFavoriteBusy(false);
    }
  };

  const markAsWatched = async () => {
    if (selection.source !== "emby" || markPlayedBusy) return;
    setMarkPlayedBusy(true);
    try {
      const response = await fetch(`/api/media/emby/played?itemId=${encodeURIComponent(selection.id)}`, { method: "POST", credentials: "include" });
      if (!response.ok) throw new Error("mark as watched failed");
      setDetail((current) => current && (current.isSeries
        ? { ...current, watchedEpisodes: current.totalEpisodes }
        : { ...current, played: true }));
      onHiddenChanged?.();
    } catch {
      // Best effort — Emby stays the source of truth, so a failed request
      // just means the item keeps showing up in "Noch nicht fertig" until
      // the user retries, not that anything is left in an inconsistent state.
    } finally {
      setMarkPlayedBusy(false);
    }
  };

  useEffect(() => {
    let active = true;
    const url = selection.source === "emby"
      ? `/api/media/emby?id=${encodeURIComponent(selection.id)}`
      : selection.source === "comingsoon"
      // Radarr und Sonarr tragen alles, was dieser Screen braucht, schon im
      // Kalender — deshalb braucht "Demnächst" und "Im Kino" kein Seerr.
      ? `/api/media/comingsoon?source=${encodeURIComponent(selection.via)}&id=${encodeURIComponent(selection.id)}`
      : `/api/media/seerr?mediaType=${encodeURIComponent(mediaType ?? "")}&id=${encodeURIComponent(selection.id)}`;
    fetch(url, { credentials: "include" })
      .then(async (response) => {
        if (!response.ok) throw new Error("detail unavailable");
        const data: MediaDetail = await response.json();
        if (!active) return;
        setDetail(data);
        setFavorite(data.isFavorite ?? false);
        setState("ready");
      })
      .catch(() => active && setState("error"));
    return () => { active = false; };
  }, [selection.source, selection.id, selection.via, mediaType]);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      if (requestModalOpen) { setRequestModalOpen(false); return; }
      onClose();
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [onClose, requestModalOpen]);

  const status = detail ? mediaStatus(detail, lang) : null;
  const crewAndCast = detail ? [...(detail.crew ?? []), ...(detail.cast ?? [])] : [];
  const embySeasons = (detail?.seasons?.filter((season): season is MediaSeason => !isRequestableSeason(season)) ?? []).slice().sort((a, b) => a.indexNumber - b.indexNumber);
  // Seerr's own MediaStatus enum: 4 = partially available. Only for that
  // status (and only for series) do missing seasons remain requestable —
  // any other non-zero status (pending, processing, fully available, ...)
  // means there is nothing left to request. Seasons already available or
  // with an open request (pending/processing) are excluded so they can't
  // be re-requested.
  // Seerr's own truth, ignoring what this browser has locally recorded —
  // used only to detect once Seerr's status has caught up with a request
  // already made, so the "Angefragt ✓" override below can hand off to the
  // normal status display (matching every other title) instead of masking
  // it forever.
  const seerrRequestableSeasons = (detail?.seasons?.filter(isRequestableSeason) ?? []).filter((season) => !season.available && !season.requested);
  const requestableSeasons = seerrRequestableSeasons.filter((season) => !locallyRequestedSeasons.includes(season.seasonNumber));
  const seerrStatusPending = 2;
  const seerrStatusProcessing = 3;
  const seerrStatusPartiallyAvailable = 4;
  const seerrStatusAvailable = 5;
  const seerrKnowsAboutMovie = mediaType !== "tv" && !!detail?.mediaStatus;
  const locallyRequestedMovie = mediaType !== "tv" && locallyRequestedSeasons.includes(movieRequestedSentinel) && !seerrKnowsAboutMovie;
  // Ohne TMDB liefert Sonarr nur eine TVDB-Id. Die als TMDB-Id an Seerr zu
  // schicken, würde den falschen Titel anfragen — deshalb ist Anfragen nur
  // möglich, wenn eine echte TMDB-Id vorliegt.
  const requestTmdbId = selection.source === "seerr" ? selection.id : selection.source === "comingsoon" ? selection.tmdbId : undefined;
  const canRequest = selection.source !== "emby" && seerrConfigured && !!requestTmdbId && !locallyRequestedMovie && (!detail?.mediaStatus || (detail.mediaStatus === seerrStatusPartiallyAvailable && mediaType === "tv" && requestableSeasons.length > 0));
  // Once this browser has requested everything that's currently missing,
  // keep showing the confirmation across a remount/refetch — but only
  // while Seerr's own data hasn't caught up yet (seerrRequestableSeasons
  // still non-empty / mediaStatus still 0). Once it has, this steps aside
  // for seerrAvailabilityLabel below so the title looks like any other
  // already-requested one instead of staying stuck on "Angefragt ✓".
  const showRequestConfirmation = requestState === "done" || locallyRequestedMovie
    || (mediaType === "tv" && locallyRequestedSeasons.length > 0 && requestableSeasons.length === 0 && seerrRequestableSeasons.length > 0);
  const seerrAvailabilityLabel = detail?.mediaStatus === seerrStatusAvailable ? translate("status_available")
    : detail?.mediaStatus === seerrStatusPartiallyAvailable ? translate("status_partially_available")
    : detail?.mediaStatus === seerrStatusPending || detail?.mediaStatus === seerrStatusProcessing ? translate("status_already_requested")
    : null;

  const toggleSeason = (seasonNumber: number) => {
    setSelectedSeasons((current) => current.includes(seasonNumber) ? current.filter((value) => value !== seasonNumber) : [...current, seasonNumber]);
  };

  const submitRequest = async () => {
    if (selection.source === "emby" || !seerrConfigured || !requestTmdbId) return;
    setRequestState("submitting");
    try {
      const response = await fetch("/api/media/seerr/request", {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ mediaType: selection.mediaType, tmdbId: Number(requestTmdbId), seasons: selection.mediaType === "tv" ? selectedSeasons : undefined }),
      });
      if (!response.ok) throw new Error("request failed");
      setRequestState("done");
      setRequestModalOpen(false);
      onRequestCreated();
      if (requestedStorageKey) {
        const requestedNow = selection.mediaType === "tv" ? selectedSeasons : [movieRequestedSentinel];
        const merged = Array.from(new Set([...locallyRequestedSeasons, ...requestedNow]));
        setLocallyRequestedSeasons(merged);
        try {
          window.localStorage.setItem(requestedStorageKey, JSON.stringify(merged));
        } catch {
          // Best effort — a persistence failure just means the "Angefragt ✓"
          // confirmation might not survive a remount, not that the request
          // (which already succeeded above) is lost.
        }
      }
    } catch {
      setRequestState("error");
    }
  };

  return <div className="media-detail-overlay" role="dialog" aria-modal="true" aria-label={detail?.title ?? translate("details")}>
    {detail && <div className="media-detail-backdrop" style={detail.backdropUrl ? { backgroundImage: `url(${detail.backdropUrl})` } : undefined} />}
    <div className="media-detail-scroll">
      <button type="button" className="media-detail-close" onClick={onClose} aria-label={translate("close")}><Icon name="close" /></button>
      {state === "loading" && <p className="poster-status media-detail-status" role="status">{translate("loading")}</p>}
      {state === "error" && <p className="poster-status media-detail-status">{translate("details_unavailable")}</p>}
      {detail && <div className="media-detail">
        <div className="media-detail-above-fold">
        <div className="media-detail-hero">
          <div className="media-detail-poster"><PosterImage src={detail.posterUrl} fallback={detail.title} lazy={false} /></div>
          <div className="media-detail-info">
            {status && <span className="media-status-badge">{status}</span>}
            {detail.currentSeasonNumber !== undefined && detail.currentEpisodeNumber !== undefined && <span className="media-status-badge media-status-badge-progress">{translate("season_episode_badge", { season: detail.currentSeasonNumber, episode: detail.currentEpisodeNumber })}</span>}
            <h1>{detail.title}{detail.year ? ` (${detail.year})` : ""}</h1>
            <p className="media-detail-meta">
              {detail.officialRating && <span>{detail.officialRating}</span>}
              {detail.runtimeMinutes > 0 && <span>{translate("minutes_long", { minutes: detail.runtimeMinutes })}</span>}
              {detail.genres?.length > 0 && <span>{detail.genres.join(", ")}</span>}
            </p>
            {(detail.communityRating > 0 || detail.imdbRating || detail.rottenTomatoesRating) && <div className="media-detail-ratings">
              {detail.communityRating > 0 && <span className="media-detail-rating">★ {detail.communityRating.toFixed(1)}</span>}
              {detail.imdbRating && <span className="media-detail-rating">IMDb {detail.imdbRating}</span>}
              {detail.rottenTomatoesRating && <span className="media-detail-rating">RT {detail.rottenTomatoesRating}</span>}
            </div>}
          </div>
        </div>
        {(detail.status || detail.releaseDate || (detail.studios && detail.studios.length > 0)) && <section className="media-detail-facts">
          <dl>
            {detail.status && <div><dt>{translate("fact_status")}</dt><dd>{detail.status}</dd></div>}
            {detail.releaseDate && <div><dt>{translate("fact_release_date")}</dt><dd>{formatFullDate(detail.releaseDate, lang)}</dd></div>}
            {detail.studios && detail.studios.length > 0 && <div><dt>{translate("fact_studios")}</dt><dd>{detail.studios.join(", ")}</dd></div>}
          </dl>
        </section>}
        <div className="tracking-bar">
          {selection.source === "emby" && <>
            <div className="star-rating" role="radiogroup" aria-label={translate("your_rating")}>
              {[1, 2, 3, 4, 5].map((value) => <button
                key={value}
                type="button"
                className={value <= tracking.rating ? "star-button filled" : "star-button"}
                aria-pressed={value <= tracking.rating}
                aria-label={translate("stars_of_five", { value })}
                onClick={() => saveTracking({ ...tracking, rating: value === tracking.rating ? 0 : value })}
              >★</button>)}
            </div>
            <button
              type="button"
              className={favorite ? "icon-toggle-button active" : "icon-toggle-button"}
              onClick={toggleFavorite}
              disabled={favoriteBusy}
              aria-pressed={favorite}
              aria-label={translate(favorite ? "favorite_remove" : "favorite_add")}
              title={translate(favorite ? "favorite_remove" : "favorite_add")}
            ><Icon name="heart" /></button>
          </>}
          <label className="watchlist-toggle">
            <span>{translate("watchlist")}</span>
            <span className="toggle-switch">
              <input type="checkbox" checked={tracking.onWatchlist} onChange={() => saveTracking({ ...tracking, onWatchlist: !tracking.onWatchlist })} />
              <span className="toggle-track"><span className="toggle-thumb" /></span>
            </span>
          </label>
        </div>
        {selection.source === "emby" && detail.isSeries && !isFullyWatched(detail) && <div className="request-row">
          <button type="button" className="request-button secondary" onClick={markAsWatched} disabled={markPlayedBusy}>
            {translate("mark_as_watched")}
          </button>
        </div>}
        {selection.source !== "emby" && (canRequest || seerrAvailabilityLabel || showRequestConfirmation || !seerrConfigured) && <div className="request-row">
          {seerrAvailabilityLabel && !showRequestConfirmation && <span className="media-availability-badge">{seerrAvailabilityLabel}</span>}
          {/* Ohne Seerr bleibt der Knopf sichtbar, sagt aber warum er nichts
              tut — sonst wirkt es, als fehle die Funktion. */}
          {!seerrConfigured
            ? <><button type="button" className="request-button" disabled aria-describedby="request-unconfigured">{translate("request")}</button>
                <p id="request-unconfigured" className="request-unconfigured">{translate("request_unconfigured")}</p></>
            : (canRequest || showRequestConfirmation) && (showRequestConfirmation
              ? <p className="request-confirmation">{translate("requested_confirmation")}</p>
              : <button type="button" className="request-button" onClick={() => setRequestModalOpen(true)}>{translate("request")}</button>)}
        </div>}
        {detail.overview && <section className="media-detail-overview"><h2>{translate("overview")}</h2><OverviewText text={detail.overview} /></section>}
        </div>
        {embySeasons.length > 0 && <section className="media-detail-seasons">
          <h2>{translate("seasons")}</h2>
          <div className="poster-scroller">{embySeasons.map((season) => {
            const progress = season.totalEpisodes > 0 ? Math.round((season.watchedEpisodes / season.totalEpisodes) * 100) : 0;
            return <article className="poster-entry" key={season.id}>
              <div className="poster wide" role="img" aria-label={season.title}>
                <PosterImage src={season.posterUrl} fallback={season.title} />
                <div className="poster-progress"><div className="poster-progress-fill" style={{ width: `${progress}%` }} /></div>
              </div>
              <strong>{season.title}</strong>
              <small>{season.played ? translate("watched") : translate("episodes_of", { watched: season.watchedEpisodes, total: season.totalEpisodes })}</small>
            </article>;
          })}</div>
        </section>}
        {crewAndCast.length > 0 && <section className="media-detail-cast">
          <h2>{translate("cast")}</h2>
          <div className="cast-grid">
            {crewAndCast.slice(0, 12).map((person, index) => <div className="cast-entry" key={`${person.name}-${index}`}>
              <div className="cast-avatar"><PosterImage src={person.imageUrl} fallback={person.name.charAt(0)} /></div>
              <strong>{person.name}</strong><small>{person.role}</small>
            </div>)}
          </div>
        </section>}
      </div>}
    </div>
    {detail && requestModalOpen && <div className="request-modal-backdrop" role="presentation" onClick={() => requestState !== "submitting" && setRequestModalOpen(false)}>
      <div className="request-modal" role="dialog" aria-modal="true" aria-label={translate("request_title_aria", { title: detail.title })} onClick={(event) => event.stopPropagation()}>
        <div className="request-modal-head">
          <div className="request-modal-poster"><PosterImage src={detail.posterUrl} fallback="" lazy={false} /></div>
          <div><p className="eyebrow">{translate("request_eyebrow")}</p><h3>{detail.title}</h3></div>
        </div>
        {requestableSeasons.length > 0 && <div className="season-list">
          {requestableSeasons.map((season) => <label className="season-toggle-row" key={season.seasonNumber}>
            <span>{translate("season_number", { number: season.seasonNumber })} <small>{translate("season_episode_count", { count: season.episodeCount })}</small></span>
            <span className="toggle-switch">
              <input type="checkbox" checked={selectedSeasons.includes(season.seasonNumber)} onChange={() => toggleSeason(season.seasonNumber)} />
              <span className="toggle-track"><span className="toggle-thumb" /></span>
            </span>
          </label>)}
        </div>}
        {requestState === "error" && <p className="request-error">{translate("request_failed")}</p>}
        <div className="request-modal-actions">
          <button type="button" className="request-button secondary" disabled={requestState === "submitting"} onClick={() => setRequestModalOpen(false)}>{translate("cancel")}</button>
          <button
            type="button"
            className="request-button"
            disabled={requestState === "submitting" || (requestableSeasons.length > 0 && selectedSeasons.length === 0)}
            onClick={submitRequest}
          >
            {translate(requestState === "submitting" ? "requesting" : "request_now")}
          </button>
        </div>
      </div>
    </div>}
  </div>;
}

function mediaStatus(detail: MediaDetail, lang: Lang) {
  if (detail.isSeries) {
    if (!detail.totalEpisodes) return null;
    if (isFullyWatched(detail)) return t(lang, "watched");
    return t(lang, "episodes_of", { watched: detail.watchedEpisodes ?? 0, total: detail.totalEpisodes });
  }
  if (detail.played !== undefined) return t(lang, detail.played ? "watched" : "status_available");
  return null;
}

// Split out of mediaStatus so "is this series finished?" stays a data question
// — comparing the rendered label against "Angesehen" would silently stop
// matching the moment the UI language changes.
function isFullyWatched(detail: MediaDetail) {
  return !!detail.totalEpisodes && (detail.watchedEpisodes ?? 0) >= detail.totalEpisodes;
}

function useTrackingList(path: string) {
  const [items, setItems] = useState<MediaTrackingEntry[]>([]);
  const [state, setState] = useState<LoadState>("loading");
  useEffect(() => {
    let active = true;
    fetch(path, { credentials: "include" })
      .then(async (response) => { if (!response.ok) throw new Error("tracking list unavailable"); const data = await response.json(); if (active) { setItems(data); setState("ready"); } })
      .catch(() => active && setState("error"));
    return () => { active = false; };
  }, [path]);
  return [items, state] as const;
}

function Profile({ user, userProfile, totalRequests, onSelectMedia }: { user: CurrentUser; userProfile: UserProfile | null; totalRequests: number | null; onSelectMedia: (selection: MediaSelection) => void }) {
  const lang = useLang();
  const translate = useT();
  const [signingOut, setSigningOut] = useState(false);
  const [watchlist, watchlistState] = useTrackingList("/api/tracking/watchlist");
  const [ratings, ratingsState] = useTrackingList("/api/tracking/ratings");
  const toSelection = (entry: MediaTrackingEntry): MediaSelection =>
    entry.mediaSource === "seerr" ? { source: "seerr", id: entry.mediaId, mediaType: entry.mediaType } : { source: "emby", id: entry.mediaId };
  const withId = (entries: readonly MediaTrackingEntry[]) => entries.map((entry) => ({ ...entry, id: entry.mediaId }));
  const logout = async () => {
    setSigningOut(true);
    try { await fetch("/api/auth/logout", { method: "POST", credentials: "include" }); } catch { /* reload clears the client session either way */ }
    window.location.reload();
  };
  return <div className="content page-view profile">
    <section className="profile-head"><div className="avatar big"><UserAvatar name={user.name} userId={user.id} /></div><div><p className="eyebrow">{translate("profile_eyebrow")}</p><h2>{user.name}</h2></div></section>
    <section className="media-detail-facts profile-facts">
      <dl>
        <div><dt>{translate("member_since")}</dt><dd>{userProfile ? formatFullDate(userProfile.memberSince, lang) : "—"}</dd></div>
        <div><dt>{translate("last_active")}</dt><dd>{userProfile ? formatFullDate(userProfile.lastActiveDate, lang) : "—"}</dd></div>
        <div><dt>{translate("last_login")}</dt><dd>{userProfile ? formatFullDate(userProfile.lastLoginDate, lang) : "—"}</dd></div>
        {user.features.requests && <div><dt>{translate("total_requests")}</dt><dd>{totalRequests !== null ? totalRequests : "—"}</dd></div>}
        <div><dt>{translate("version")}</dt><dd>v{APP_VERSION}</dd></div>
        <PushNotificationSettings />
      </dl>
    </section>
    <button className="logout-button" onClick={logout} disabled={signingOut}>{translate(signingOut ? "signing_out" : "sign_out")}</button>
    <PosterRow title={translate("my_watchlist")} eyebrow={translate("my_lists_eyebrow")} items={withId(watchlist)} state={watchlistState} emptyLabel={translate("watchlist_empty")} detail={() => translate("watchlist")} onSelect={(item) => onSelectMedia(toSelection(item))} />
    <PosterRow title={translate("my_ratings")} items={withId(ratings)} state={ratingsState} emptyLabel={translate("ratings_empty")} detail={(item) => "★".repeat(item.rating ?? 0)} onSelect={(item) => onSelectMedia(toSelection(item))} />
  </div>;
}

type EmbyLibrary = { id: string; name: string };
type ServiceView = { enabled: boolean; baseUrl?: string; apiKeySet: boolean; apiKeyPreview?: string };
type AdminSettingsView = {
  newForYouLibraryIds: string[]; watchedLibraryIds: string[];
  seerr: ServiceView; radarr: ServiceView; sonarr: ServiceView; tmdb: ServiceView; omdb: ServiceView;
  comingSoonRegion: string; comingSoonDaysAhead: number;
  language: Lang;
};
type DailyActivity = { date: string; requestCount: number; activeUsers: number };
type ServiceDraft = { enabled: boolean; baseUrl: string; apiKey: string };
const SETUP_INTRO_SEEN_KEY = "emby-insights-setup-intro-seen";

const activityDayFormatters = localeFormatter({ weekday: "short" });

// ActivityChart answers "is Emby Insights actually being used?" — grouped
// columns, one shared count axis (never two y-scales for two count series
// of the same unit), oldest day first. A <details> table covers the
// non-visual/accessible path since a hand-rolled SVG-free bar chart has no
// other way to expose exact values to a screen reader.
function ActivityChart({ data, state }: { data: DailyActivity[]; state: LoadState }) {
  const lang = useLang();
  const translate = useT();
  const max = Math.max(1, ...data.flatMap((day) => [day.activeUsers, day.requestCount]));
  const hasData = data.some((day) => day.activeUsers > 0 || day.requestCount > 0);
  return <section className="admin-section activity-chart-card" aria-label={translate("activity")}>
    <div className="section-heading"><div><p className="eyebrow">{translate("activity_eyebrow")}</p><h2>{translate("activity_heading")}</h2></div></div>
    {state === "loading" && <div className="activity-chart" aria-hidden="true">
      <p className="sr-only" role="status">{translate("loading")}</p>
      <div className="activity-chart-columns">{Array.from({ length: 7 }, (_, index) => <div className="activity-chart-column" key={index}>
        <div className="activity-chart-bars"><span className="skeleton activity-chart-bar-skeleton" /><span className="skeleton activity-chart-bar-skeleton" /></div>
        <span className="skeleton skeleton-line-xs" />
      </div>)}</div>
    </div>}
    {state === "error" && <p className="poster-status">{translate("activity_unavailable")}</p>}
    {state === "ready" && !hasData && <p className="poster-status">{translate("activity_empty")}</p>}
    {state === "ready" && hasData && <>
      <div className="activity-chart-legend">
        <span className="activity-legend-item"><span className="activity-legend-swatch activity-swatch-users" /> {translate("legend_active_users")}</span>
        <span className="activity-legend-item"><span className="activity-legend-swatch activity-swatch-requests" /> {translate("legend_seerr_requests")}</span>
      </div>
      <div className="activity-chart-columns" role="img" aria-label={translate("activity_chart_aria")}>
        {data.map((day) => {
          const weekday = activityDayFormatters[lang].format(new Date(`${day.date}T00:00:00`));
          return <div className="activity-chart-column" key={day.date}>
            <div className="activity-chart-bars">
              <div className="activity-chart-bar activity-bar-users" style={{ height: `${(day.activeUsers / max) * 100}%` }} title={translate("activity_bar_users", { weekday, count: day.activeUsers })} />
              <div className="activity-chart-bar activity-bar-requests" style={{ height: `${(day.requestCount / max) * 100}%` }} title={translate("activity_bar_requests", { weekday, count: day.requestCount })} />
            </div>
            <span className="activity-chart-day-label">{weekday}</span>
          </div>;
        })}
      </div>
      <details className="activity-chart-table-details">
        <summary>{translate("show_as_table")}</summary>
        <table className="activity-chart-table">
          <thead><tr><th scope="col">{translate("table_day")}</th><th scope="col">{translate("legend_active_users")}</th><th scope="col">{translate("legend_seerr_requests")}</th></tr></thead>
          <tbody>{data.map((day) => <tr key={day.date}><th scope="row">{formatFullDate(day.date, lang)}</th><td>{day.activeUsers}</td><td>{day.requestCount}</td></tr>)}</tbody>
        </table>
      </details>
    </>}
  </section>;
}

// AdminSettings is the Verwaltung page: it replaces manually editing .env
// with a GUI for library selection and the four optional integrations.
// Nothing here is enforced client-side only — every /api/admin/* call is
// re-checked server-side (see requireAdmin in the backend).
function AdminSettings({ onLanguageChange }: { onLanguageChange: (lang: Lang) => void }) {
  const translate = useT();
  const [activity, activityState] = useApiResource<DailyActivity[]>("/api/admin/activity", []);
  const [settings, settingsState, refetchSettings] = useApiResource<AdminSettingsView | null>("/api/admin/settings", null);
  const [libraries, librariesState] = useApiResource<EmbyLibrary[]>("/api/admin/libraries", []);

  if (settingsState === "loading") return <div className="content page-view admin-page"><p className="poster-status" role="status">{translate("loading")}</p></div>;
  if (settingsState === "error" || !settings) return <div className="content page-view admin-page"><p className="poster-status">{translate("settings_unavailable")}</p></div>;

  // A successful save refetches settings. Remounting the editable form then
  // deliberately resets its draft from that fresh server response.
  return <AdminSettingsForm
    key={JSON.stringify(settings)}
    activity={activity}
    activityState={activityState}
    settings={settings}
    refetchSettings={refetchSettings}
    onLanguageChange={onLanguageChange}
    libraries={libraries}
    librariesState={librariesState}
  />;
}

function AdminSettingsForm({ activity, activityState, settings, refetchSettings, onLanguageChange, libraries, librariesState }: {
  activity: DailyActivity[];
  activityState: LoadState;
  settings: AdminSettingsView;
  refetchSettings: () => void;
  onLanguageChange: (lang: Lang) => void;
  libraries: EmbyLibrary[];
  librariesState: LoadState;
}) {
  const translate = useT();
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState(false);
  const [savedAt, setSavedAt] = useState<number | null>(null);
  const [showIntro, setShowIntro] = useState(() => typeof window !== "undefined" && !window.localStorage.getItem(SETUP_INTRO_SEEN_KEY));

  const [newForYouIds, setNewForYouIds] = useState<string[]>(() => settings.newForYouLibraryIds);
  const [watchedIds, setWatchedIds] = useState<string[]>(() => settings.watchedLibraryIds);
  const [libraryPicker, setLibraryPicker] = useState<"newForYou" | "watched" | null>(null);
  const [seerr, setSeerr] = useState<ServiceDraft>(() => ({ enabled: settings.seerr.enabled, baseUrl: settings.seerr.baseUrl ?? "", apiKey: "" }));
  const [radarr, setRadarr] = useState<ServiceDraft>(() => ({ enabled: settings.radarr.enabled, baseUrl: settings.radarr.baseUrl ?? "", apiKey: "" }));
  const [sonarr, setSonarr] = useState<ServiceDraft>(() => ({ enabled: settings.sonarr.enabled, baseUrl: settings.sonarr.baseUrl ?? "", apiKey: "" }));
  const [tmdb, setTmdb] = useState<ServiceDraft>(() => ({ enabled: settings.tmdb.enabled, baseUrl: "", apiKey: "" }));
  const [omdb, setOmdb] = useState<ServiceDraft>(() => ({ enabled: settings.omdb.enabled, baseUrl: "", apiKey: "" }));
  const [comingSoonRegion, setComingSoonRegion] = useState(() => settings.comingSoonRegion || "DE");
  const [comingSoonDaysAhead, setComingSoonDaysAhead] = useState(() => settings.comingSoonDaysAhead || 28);
  const [language, setLanguage] = useState<Lang>(() => isLang(settings.language) ? settings.language : "en");

  const dismissIntro = () => {
    window.localStorage.setItem(SETUP_INTRO_SEEN_KEY, "1");
    setShowIntro(false);
  };

  const toggleLibrary = (list: string[], setList: (next: string[]) => void, id: string) => {
    setList(list.includes(id) ? list.filter((entry) => entry !== id) : [...list, id]);
  };

  const save = async () => {
    setSaving(true);
    setSaveError(false);
    try {
      const response = await fetch("/api/admin/settings", {
        method: "PUT",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          newForYouLibraryIds: newForYouIds,
          watchedLibraryIds: watchedIds,
          seerr, radarr, sonarr,
          tmdb: { enabled: tmdb.enabled, apiKey: tmdb.apiKey },
          omdb: { enabled: omdb.enabled, apiKey: omdb.apiKey },
          comingSoonRegion,
          comingSoonDaysAhead,
          language,
        }),
      });
      if (!response.ok) throw new Error("saving settings failed");
      setSavedAt(Date.now());
      // Lifts the saved language to the root component so the whole tree
      // relabels immediately, without waiting for a reload or a fresh /api/me.
      onLanguageChange(language);
      refetchSettings();
    } catch {
      setSaveError(true);
    } finally {
      setSaving(false);
    }
  };

  return <div className="content page-view admin-page">
    {showIntro && <section className="admin-intro-banner">
      <p className="eyebrow">{translate("welcome_eyebrow")}</p>
      <h2>{translate("admin_intro_title")}</h2>
      <p>{translate("admin_intro_body")}</p>
      <button type="button" className="request-button" onClick={dismissIntro}>{translate("got_it")}</button>
    </section>}

    <ActivityChart data={activity} state={activityState} />

    <section className="admin-section" aria-label={translate("libraries")}>
      <div className="section-heading"><div><p className="eyebrow">{translate("libraries_eyebrow")}</p><h2>{translate("libraries_heading")}</h2></div></div>
      {librariesState === "loading" && <p className="poster-status" role="status">{translate("libraries_loading")}</p>}
      {librariesState === "error" && <p className="poster-status">{translate("libraries_unavailable")}</p>}
      {librariesState === "ready" && libraries.length === 0 && <p className="poster-status">{translate("libraries_none")}</p>}
      {libraries.length > 0 && <div className="admin-service-grid">
        <LibraryTile
          title={translate("row_new_for_you")} description={translate("library_new_for_you_description")}
          selectedCount={newForYouIds.length} onOpen={() => setLibraryPicker("newForYou")}
        />
        <LibraryTile
          title={translate("library_watched_title")} description={translate("library_watched_description")}
          selectedCount={watchedIds.length} onOpen={() => setLibraryPicker("watched")}
        />
      </div>}
    </section>

    {libraryPicker && <LibraryPickerModal
      title={translate(libraryPicker === "newForYou" ? "row_new_for_you" : "library_watched_title")}
      libraries={libraries}
      selectedIds={libraryPicker === "newForYou" ? newForYouIds : watchedIds}
      onToggle={(id) => libraryPicker === "newForYou" ? toggleLibrary(newForYouIds, setNewForYouIds, id) : toggleLibrary(watchedIds, setWatchedIds, id)}
      onClose={() => setLibraryPicker(null)}
    />}

    <section className="admin-section" aria-label={translate("optional_services")}>
      <div className="section-heading"><div><p className="eyebrow">{translate("optional_services_eyebrow")}</p><h2>{translate("connections")}</h2></div></div>
      <div className="admin-service-grid">
        <ServiceCard
          title="Seerr" description={translate("service_seerr_description")}
          shows={translate("service_seerr_shows")}
          draft={seerr} onChange={setSeerr} existing={settings.seerr} showsBaseUrl
        />
        <ServiceCard
          title="Radarr" description={translate("service_radarr_description")}
          shows={translate("service_radarr_shows")}
          draft={radarr} onChange={setRadarr} existing={settings.radarr} showsBaseUrl
        />
        <ServiceCard
          title="Sonarr" description={translate("service_sonarr_description")}
          shows={translate("service_sonarr_shows")}
          draft={sonarr} onChange={setSonarr} existing={settings.sonarr} showsBaseUrl
        />
        <ServiceCard
          title="TMDB" description={translate("service_tmdb_description")}
          shows={translate("service_tmdb_shows")}
          draft={tmdb} onChange={setTmdb} existing={settings.tmdb} showsBaseUrl={false}
        />
        <ServiceCard
          title="OMDB" description={translate("service_omdb_description")}
          shows={translate("service_omdb_shows")}
          draft={omdb} onChange={setOmdb} existing={settings.omdb} showsBaseUrl={false}
        />
      </div>
    </section>

    <section className="admin-section" aria-label={translate("row_upcoming")}>
      <div className="section-heading"><div><p className="eyebrow">{translate("comingsoon_eyebrow")}</p><h2>{translate("comingsoon_heading")}</h2></div></div>
      <div className="admin-service-grid">
        <label className="admin-field">
          <span>{translate("field_region")}</span>
          <input type="text" className="search-input" maxLength={2} value={comingSoonRegion} onChange={(event) => setComingSoonRegion(event.target.value.toUpperCase())} placeholder="DE" />
        </label>
        <label className="admin-field">
          <span>{translate("field_days_ahead")}</span>
          <input type="number" className="search-input" min={1} max={90} value={comingSoonDaysAhead} onChange={(event) => setComingSoonDaysAhead(Number(event.target.value) || 1)} />
        </label>
      </div>
    </section>

    {/* Deliberately its own section, away from the Demnächst region field:
        the two answer different questions (which language the app speaks vs.
        which country's release dates it shows) and pairing them would suggest
        they move together. */}
    <section className="admin-section" aria-label={translate("interface_section")}>
      <div className="section-heading"><div><p className="eyebrow">{translate("interface_eyebrow")}</p><h2>{translate("interface_heading")}</h2></div></div>
      <div className="admin-service-grid">
        <label className="admin-field">
          <span>{translate("field_language")}</span>
          <select className="search-input" value={language} onChange={(event) => setLanguage(isLang(event.target.value) ? event.target.value : "en")}>
            <option value="de">{translate("language_german")}</option>
            <option value="en">{translate("language_english")}</option>
          </select>
        </label>
      </div>
      <p className="admin-hint">{translate("language_hint")}</p>
    </section>

    <div className="admin-save-bar">
      {saveError && <p className="request-error">{translate("save_failed")}</p>}
      {savedAt !== null && !saveError && <p className="request-confirmation">{translate("saved")}</p>}
      <button type="button" className="request-button" disabled={saving} onClick={save}>{translate(saving ? "saving" : "save_settings")}</button>
    </div>
  </div>;
}

// Collapsed by default and independent of the enabled toggle: the operator
// can peek at / edit the address and key without flipping the service on,
// and enabling it doesn't force the card open every time it re-renders.
function ServiceCard({ title, description, shows, draft, onChange, existing, showsBaseUrl }: {
  title: string; description: string; shows: string;
  draft: ServiceDraft; onChange: (next: ServiceDraft) => void; existing: ServiceView; showsBaseUrl: boolean;
}) {
  const translate = useT();
  const [expanded, setExpanded] = useState(false);
  return <article className={expanded ? "admin-service-card expanded" : "admin-service-card"}>
    <button type="button" className="admin-service-head" onClick={() => setExpanded((value) => !value)} aria-expanded={expanded}>
      <div><strong>{title}</strong><p className="admin-hint">{description}</p></div>
      <span className="admin-service-head-controls">
        <label className="toggle-switch" onClick={(event) => event.stopPropagation()}>
          <input type="checkbox" checked={draft.enabled} onChange={(event) => onChange({ ...draft, enabled: event.target.checked })} aria-label={translate("service_enable_aria", { title })} />
          <span className="toggle-track"><span className="toggle-thumb" /></span>
        </label>
        <span className="admin-service-chevron"><Icon name="arrow" /></span>
      </span>
    </button>
    {expanded && <div className="admin-service-body">
      {showsBaseUrl && <label className="admin-field">
        <span>{translate("field_server_address")}</span>
        <input type="text" className="search-input" placeholder="https://…" value={draft.baseUrl} onChange={(event) => onChange({ ...draft, baseUrl: event.target.value })} />
      </label>}
      <label className="admin-field">
        <span>{translate("field_api_key")}{existing.apiKeySet ? translate("field_api_key_current", { preview: existing.apiKeyPreview ?? "" }) : ""}</span>
        <input type="password" className="search-input" placeholder={translate(existing.apiKeySet ? "api_key_replace_placeholder" : "api_key_placeholder")} value={draft.apiKey} onChange={(event) => onChange({ ...draft, apiKey: event.target.value })} autoComplete="off" />
      </label>
    </div>}
    <p className="admin-service-shows">{shows}</p>
  </article>;
}

// LibraryTile mirrors ServiceCard's look (same admin-service-card grid) so
// the Bibliotheken and Optionale Dienste sections read as one consistent set
// of tiles, even though a library group opens a picker instead of expanding
// inline fields.
function LibraryTile({ title, description, selectedCount, onOpen }: { title: string; description: string; selectedCount: number; onOpen: () => void }) {
  const translate = useT();
  return <button type="button" className="admin-service-card admin-library-tile" onClick={onOpen}>
    <div className="admin-service-head">
      <div><strong>{title}</strong><p className="admin-hint">{description}</p></div>
      <Icon name="arrow" />
    </div>
    <p className="admin-library-tile-count">{selectedCount === 0 ? translate("library_none_selected") : selectedCount === 1 ? translate("library_selected_one") : translate("library_selected_other", { count: selectedCount })}</p>
  </button>;
}

function LibraryPickerModal({ title, libraries, selectedIds, onToggle, onClose }: {
  title: string; libraries: EmbyLibrary[]; selectedIds: string[]; onToggle: (id: string) => void; onClose: () => void;
}) {
  useEscapeKey(onClose);
  const translate = useT();
  return <div className="request-modal-backdrop" role="presentation" onClick={onClose}>
    <div className="request-modal" role="dialog" aria-modal="true" aria-label={title} onClick={(event) => event.stopPropagation()}>
      <div><p className="eyebrow">{translate("libraries_eyebrow")}</p><h3>{title}</h3></div>
      {libraries.length === 0
        ? <p className="poster-status">{translate("libraries_none")}</p>
        : <div className="season-list">{libraries.map((library) => <label className="season-toggle-row" key={library.id}>
          <span>{library.name}</span>
          <span className="toggle-switch">
            <input type="checkbox" checked={selectedIds.includes(library.id)} onChange={() => onToggle(library.id)} />
            <span className="toggle-track"><span className="toggle-thumb" /></span>
          </span>
        </label>)}</div>}
      <div className="request-modal-actions">
        <button type="button" className="request-button" onClick={onClose}>{translate("done")}</button>
      </div>
    </div>
  </div>;
}

const chatTimeFormatters = localeFormatter({ hour: "2-digit", minute: "2-digit" });
function formatChatTime(value: string, lang: Lang) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "" : chatTimeFormatters[lang].format(date);
}

function ChatMessageList({ messages, mineWhenFromAdmin, mineName, mineAvatarSrc, theirsName, theirsAvatarSrc }: {
  messages: ChatMessage[]; mineWhenFromAdmin: boolean;
  mineName: string; mineAvatarSrc: string; theirsName: string; theirsAvatarSrc: string;
}) {
  const lang = useLang();
  const listRef = useRef<HTMLDivElement>(null);
  useEffect(() => { listRef.current?.scrollTo({ top: listRef.current.scrollHeight }); }, [messages]);
  return <div className="chat-messages" ref={listRef}>
    {messages.map((message) => {
      const isMine = message.fromAdmin === mineWhenFromAdmin;
      return <div key={message.id} className={isMine ? "chat-bubble mine" : "chat-bubble theirs"}>
        <span className="chat-bubble-avatar"><PersonAvatar name={isMine ? mineName : theirsName} src={isMine ? mineAvatarSrc : theirsAvatarSrc} /></span>
        <div className="chat-bubble-body">
          <p>{message.body}</p>
          <small>{formatChatTime(message.createdAt, lang)}</small>
        </div>
      </div>;
    })}
  </div>;
}

function ChatComposer({ placeholder, onSend }: { placeholder: string; onSend: (body: string) => Promise<boolean> }) {
  const translate = useT();
  const [body, setBody] = useState("");
  const [sending, setSending] = useState(false);
  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const trimmed = body.trim();
    if (!trimmed || sending) return;
    setSending(true);
    try { if (await onSend(trimmed)) setBody(""); } finally { setSending(false); }
  };
  return <form className="chat-composer" onSubmit={submit}>
    <input type="text" className="search-input" placeholder={placeholder} value={body} onChange={(event) => setBody(event.target.value)} aria-label={placeholder} maxLength={4000} />
    <button type="submit" className="search-button" disabled={body.trim() === "" || sending}>{translate("send")}</button>
  </form>;
}

function Chats({ user }: { user: { id: string; name: string; isAdmin: boolean } }) {
  return user.isAdmin ? <AdminChats adminName={user.name} adminUserId={user.id} /> : <UserChat userName={user.name} userId={user.id} />;
}

function UserChat({ userName, userId }: { userName: string; userId: string }) {
  const translate = useT();
  const [messages, state, refetch] = useApiResource<ChatMessage[]>("/api/messages", [], CHAT_POLL_MS);

  useEffect(() => { fetch("/api/messages/read", { method: "POST", credentials: "include" }).catch(() => null); }, []);

  const send = async (body: string) => {
    const response = await fetch("/api/messages", {
      method: "POST", credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ body }),
    });
    if (response.ok) refetch();
    return response.ok;
  };

  return <div className="content page-view chat-view">
    <section className="chat-thread" aria-label={translate("chat_with_operator")}>
      {state === "loading" && <p className="poster-status" role="status">{translate("loading")}</p>}
      {state === "error" && <p className="poster-status">{translate("not_available")}</p>}
      {state === "ready" && messages.length === 0 && <p className="chat-empty">{translate("chat_empty_user")}</p>}
      <ChatMessageList messages={messages} mineWhenFromAdmin={false} mineName={userName} mineAvatarSrc={`/api/me/avatar?u=${encodeURIComponent(userId)}`} theirsName={translate("admin")} theirsAvatarSrc="/api/messages/admin-avatar" />
      <ChatComposer placeholder={translate("compose_placeholder")} onSend={send} />
    </section>
  </div>;
}

function AdminChats({ adminName, adminUserId }: { adminName: string; adminUserId: string }) {
  const translate = useT();
  const [threads, threadsState, refetchThreads] = useApiResource<ChatThread[]>("/api/admin/messages/threads", [], CHAT_POLL_MS);
  const [contacts] = useApiResource<Contact[]>("/api/admin/users", []);
  const [selectedUserId, setSelectedUserId] = useState<string | null>(null);
  const [pickerOpen, setPickerOpen] = useState(false);
  const [broadcastOpen, setBroadcastOpen] = useState(false);
  const [newThreadContact, setNewThreadContact] = useState<Contact | null>(null);

  const existingThread = threads.find((thread) => thread.userId === selectedUserId) ?? null;
  const selectedThread: ChatThread | null = existingThread ?? (newThreadContact
    ? { userId: newThreadContact.id, displayName: newThreadContact.name, lastMessage: "", lastAt: new Date().toISOString(), unreadCount: 0 }
    : null);
  const availableContacts = contacts.filter((contact) => !threads.some((thread) => thread.userId === contact.id));

  const closeThread = () => { setSelectedUserId(null); setNewThreadContact(null); refetchThreads(); };
  const startNewThread = (contact: Contact) => { setNewThreadContact(contact); setPickerOpen(false); };

  return <div className="content page-view chat-view">
    <div className="chat-inbox-header">
      <div><p className="eyebrow">{translate("messages_eyebrow")}</p><h2>{translate("inbox")}</h2></div>
      <div className="chat-inbox-actions">
        <button type="button" className="chat-action-button" onClick={() => setBroadcastOpen(true)}><Icon name="bell" /> {translate("broadcast")}</button>
        <button type="button" className="chat-action-button" onClick={() => setPickerOpen(true)}><Icon name="arrow" /> {translate("broadcast_new_chat")}</button>
      </div>
    </div>
    <section className="chat-inbox" aria-label={translate("inbox_aria")}>
      {threadsState === "loading" && <p className="poster-status" role="status">{translate("loading")}</p>}
      {threadsState === "error" && <p className="poster-status">{translate("not_available")}</p>}
      {threadsState === "ready" && threads.length === 0 && <p className="chat-empty">{translate("inbox_empty")}</p>}
      <ul className="chat-thread-list">
        {threads.map((thread) => <li key={thread.userId}>
          <button type="button" className="chat-thread-row" onClick={() => setSelectedUserId(thread.userId)}>
            <span className="chat-avatar"><PersonAvatar name={thread.displayName || "?"} src={`/api/admin/users/avatar?userId=${encodeURIComponent(thread.userId)}`} /></span>
            <span className="chat-thread-name">{thread.displayName || translate("unknown_user")}</span>
            <span className="chat-thread-preview">{thread.lastMessage}</span>
            {thread.unreadCount > 0 && <span className="chat-thread-badge">{thread.unreadCount}</span>}
          </button>
        </li>)}
      </ul>
    </section>
    {pickerOpen && <ContactPickerScreen contacts={availableContacts} onSelect={startNewThread} onClose={() => setPickerOpen(false)} />}
    {broadcastOpen && <BroadcastScreen onClose={() => setBroadcastOpen(false)} onSent={() => { setBroadcastOpen(false); refetchThreads(); }} />}
    {selectedThread && <AdminChatThreadScreen thread={selectedThread} adminName={adminName} adminUserId={adminUserId} onClose={closeThread} />}
  </div>;
}

function BroadcastScreen({ onClose, onSent }: { onClose: () => void; onSent: () => void }) {
  const translate = useT();
  const [body, setBody] = useState("");
  const [sending, setSending] = useState(false);
  const [error, setError] = useState(false);
  const [sentCount, setSentCount] = useState<number | null>(null);

  const send = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const trimmed = body.trim();
    if (!trimmed || sending) return;
    setSending(true);
    setError(false);
    try {
      const response = await fetch("/api/admin/messages/broadcast", {
        method: "POST", credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ body: trimmed }),
      });
      if (!response.ok) throw new Error("broadcast failed");
      const data: { count: number } = await response.json();
      setSentCount(data.count);
    } catch {
      setError(true);
    } finally {
      setSending(false);
    }
  };

  return <div className="request-modal-backdrop" role="presentation" onClick={() => !sending && (sentCount !== null ? onSent() : onClose())}>
    <div className="request-modal" role="dialog" aria-modal="true" aria-label={translate("broadcast_aria")} onClick={(event) => event.stopPropagation()}>
      <div><p className="eyebrow">{translate("broadcast_eyebrow")}</p><h3>{translate("broadcast_title")}</h3></div>
      {sentCount !== null
        ? <>
          <p className="request-confirmation">{sentCount === 1 ? translate("broadcast_sent_one") : translate("broadcast_sent_other", { count: sentCount })}</p>
          <div className="request-modal-actions"><button type="button" className="request-button" onClick={onSent}>{translate("done")}</button></div>
        </>
        : <form onSubmit={send}>
          <textarea className="broadcast-textarea" placeholder={translate("broadcast_placeholder")} value={body} onChange={(event) => setBody(event.target.value)} maxLength={4000} rows={5} aria-label={translate("broadcast_textarea_aria")} />
          {error && <p className="request-error">{translate("broadcast_failed")}</p>}
          <div className="request-modal-actions">
            <button type="button" className="request-button secondary" disabled={sending} onClick={onClose}>{translate("cancel")}</button>
            <button type="submit" className="request-button" disabled={body.trim() === "" || sending}>{translate(sending ? "sending" : "send_to_all")}</button>
          </div>
        </form>}
    </div>
  </div>;
}

function ContactPickerScreen({ contacts, onSelect, onClose }: { contacts: Contact[]; onSelect: (contact: Contact) => void; onClose: () => void }) {
  useEscapeKey(onClose);
  useBodyScrollLock();
  const translate = useT();
  return <div className="media-detail-overlay media-grid-overlay" role="dialog" aria-modal="true" aria-label={translate("broadcast_new_chat")}>
    <div className="media-detail-scroll">
      <button type="button" className="media-detail-close" onClick={onClose} aria-label={translate("close")}><Icon name="close" /></button>
      <h1 className="media-grid-title">{translate("broadcast_new_chat")}</h1>
      {contacts.length === 0
        ? <p className="chat-empty">{translate("all_users_have_thread")}</p>
        : <ul className="chat-thread-list">{contacts.map((contact) => <li key={contact.id}>
          <button type="button" className="chat-thread-row" onClick={() => onSelect(contact)}>
            <span className="chat-avatar"><PersonAvatar name={contact.name || "?"} src={`/api/admin/users/avatar?userId=${encodeURIComponent(contact.id)}`} /></span>
            <span className="chat-thread-name">{contact.name || translate("unknown_user")}</span>
          </button>
        </li>)}</ul>}
    </div>
  </div>;
}

function AdminChatThreadScreen({ thread, adminName, adminUserId, onClose }: { thread: ChatThread; adminName: string; adminUserId: string; onClose: () => void }) {
  useEscapeKey(onClose);
  useBodyScrollLock();
  const translate = useT();
  const [messages, state, refetch] = useApiResource<ChatMessage[]>(`/api/admin/messages/thread?userId=${encodeURIComponent(thread.userId)}`, [], CHAT_POLL_MS);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [deleting, setDeleting] = useState(false);

  useEffect(() => {
    fetch(`/api/admin/messages/thread/read?userId=${encodeURIComponent(thread.userId)}`, { method: "POST", credentials: "include" }).catch(() => null);
  }, [thread.userId]);

  const send = async (body: string) => {
    const response = await fetch(`/api/admin/messages/thread?userId=${encodeURIComponent(thread.userId)}`, {
      method: "POST", credentials: "include",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ body, displayName: thread.displayName }),
    });
    if (response.ok) refetch();
    return response.ok;
  };

  const deleteThread = async () => {
    setDeleting(true);
    try {
      const response = await fetch(`/api/admin/messages/thread?userId=${encodeURIComponent(thread.userId)}`, { method: "DELETE", credentials: "include" });
      if (response.ok) onClose();
    } finally {
      setDeleting(false);
    }
  };

  return <div className="media-detail-overlay media-grid-overlay" role="dialog" aria-modal="true" aria-label={translate("chat_with", { name: thread.displayName })}>
    <div className="media-detail-scroll chat-overlay-scroll">
      <button type="button" className="media-detail-close" onClick={onClose} aria-label={translate("close")}><Icon name="close" /></button>
      <div className="chat-thread-header">
        <h1 className="media-grid-title">{thread.displayName || translate("unknown_user")}</h1>
        <button type="button" className="chat-delete-button" onClick={() => setConfirmDelete(true)} aria-label={translate("delete_chat")}><Icon name="close" /></button>
      </div>
      <section className="chat-thread" aria-label={translate("chat_with", { name: thread.displayName || translate("unknown_user") })}>
        {state === "loading" && <p className="poster-status" role="status">{translate("loading")}</p>}
        {state === "error" && <p className="poster-status">{translate("not_available")}</p>}
        <ChatMessageList messages={messages} mineWhenFromAdmin={true} mineName={adminName} mineAvatarSrc={`/api/me/avatar?u=${encodeURIComponent(adminUserId)}`} theirsName={thread.displayName || translate("unknown_user")} theirsAvatarSrc={`/api/admin/users/avatar?userId=${encodeURIComponent(thread.userId)}`} />
        <ChatComposer placeholder={translate("reply_placeholder")} onSend={send} />
      </section>
    </div>
    {confirmDelete && <div className="request-modal-backdrop" role="presentation" onClick={() => !deleting && setConfirmDelete(false)}>
      <div className="request-modal" role="dialog" aria-modal="true" aria-label={translate("delete_chat")} onClick={(event) => event.stopPropagation()}>
        <div><p className="eyebrow">{translate("delete_eyebrow")}</p><h3>{translate("delete_chat_confirm", { name: thread.displayName || translate("this_user") })}</h3></div>
        <p className="request-error">{translate("delete_chat_warning")}</p>
        <div className="request-modal-actions">
          <button type="button" className="request-button secondary" disabled={deleting} onClick={() => setConfirmDelete(false)}>{translate("cancel")}</button>
          <button type="button" className="request-button" disabled={deleting} onClick={deleteThread}>{translate(deleting ? "deleting" : "delete_permanently")}</button>
        </div>
      </div>
    </div>}
  </div>;
}

function greeting(lang: Lang) {
  const hour = new Date().getHours();
  return t(lang, hour < 12 ? "greeting_morning" : hour < 18 ? "greeting_afternoon" : "greeting_evening");
}

// A plain reload() can still be served from Safari's cache when the app is
// added to the iPad home screen (no address bar / pull-to-refresh there to
// force a fresh fetch). The cache-busting query string makes this an
// unmatched URL, so the browser has to hit the network. Clearing the stored
// page first means landing back on "/" also resets to "today" — the refresh
// button (which reloads the same way) leaves it in place instead, since it
// means "refresh this view", not "go home".
function goHomeAndRefresh() {
  window.sessionStorage.removeItem(PAGE_STORAGE_KEY);
  window.location.href = `${window.location.pathname}?refresh=${Date.now()}`;
}
function loadingCopy(state: LoadState, lang: Lang) { return t(lang, state === "error" ? "not_available" : "loading"); }
function formatDuration(seconds: number, lang: Lang) { const hours = Math.floor(seconds / 3600); const minutes = Math.floor((seconds % 3600) / 60); return hours > 0 ? t(lang, "duration_hours_minutes", { hours, minutes }) : t(lang, "duration_minutes", { minutes }); }
const numberFormatters: Record<Lang, Intl.NumberFormat> = { de: new Intl.NumberFormat(locales.de), en: new Intl.NumberFormat(locales.en) };
function comparisonText(statistics: PersonalStats, lang: Lang) {
  if (statistics.previousWatchSeconds === 0) return t(lang, "no_comparison_data");
  const change = Math.round(((statistics.watchSeconds - statistics.previousWatchSeconds) / statistics.previousWatchSeconds) * 100);
  return t(lang, "comparison_change", { change: `${change >= 0 ? "+" : "\u2212"}${numberFormatters[lang].format(Math.abs(change))}` });
}
