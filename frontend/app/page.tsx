"use client";

import { type FormEvent, type ReactNode, useEffect, useRef, useState } from "react";
import { LoginScreen } from "./login-screen";

type Page = "Heute" | "Statistik" | "Anfragen" | "Chats" | "Profil" | "Verwaltung";
type Features = { requests: boolean; movieDates: boolean; seriesDates: boolean; upcoming: boolean; statistics: boolean };
type CurrentUser = { id: string; name: string; isAdmin: boolean; features: Features };
type Period = "Woche" | "Monat" | "Jahr";
type StatisticsPeriod = "week" | "month" | "year";
type PersonalStats = { watchSeconds: number; previousWatchSeconds: number; completedMovies: number; completedSeries: number; favouriteGenre: string; periodStartsAt: string; periodEndsAt: string };
type WatchTimeRank = { rank: number };
type DeviceWatchTime = { deviceName: string; watchSeconds: number };
type HourWatchTime = { hour: number; watchSeconds: number };
type WeekdayWatchTime = { weekday: number; watchSeconds: number };
type LongestSession = { itemName: string; watchSeconds: number; startedAt: string };
type MostActiveDay = { date: string; watchSeconds: number };
type UserProfile = { memberSince: string; lastActiveDate: string; lastLoginDate: string };
type UpcomingItem = { id: string; tmdbId: string; title: string; posterUrl: string; mediaType: string; availabilityDate: string; cinemaStartDate?: string; cinemaEndDate?: string; seasonNumber?: number; episodeNumber?: number; episodeTitle?: string };
type RequestItem = { id: string; title: string; posterUrl: string; status: string; tmdbId: string; mediaType: string };
type NewForYouItem = { id: string; title: string; posterUrl: string };
type ContinueWatchingItem = { id: string; title: string; posterUrl: string; progressPercent: number };
type WatchedItem = { id: string; title: string; posterUrl: string; genres: string[]; lastPlayedDate: string };
type SeriesProgress = { id: string; title: string; posterUrl: string; watchedEpisodes: number; totalEpisodes: number };
type DiscoverItem = { id: string; title: string; posterUrl: string; mediaType: string };
type MediaSelection = { source: "emby"; id: string } | { source: "seerr"; id: string; mediaType: string };
type MediaPerson = { name: string; role: string; imageUrl: string };
type MediaSeason = { id: string; title: string; posterUrl: string; indexNumber: number; watchedEpisodes: number; totalEpisodes: number; played: boolean };
type RequestableSeason = { seasonNumber: number; episodeCount: number };
type MediaDetail = {
  id: string; title: string; overview: string; posterUrl: string; backdropUrl: string;
  genres: string[]; communityRating: number; officialRating?: string; year: number; runtimeMinutes: number;
  cast: MediaPerson[]; crew: MediaPerson[];
  isSeries?: boolean; watchedEpisodes?: number; totalEpisodes?: number; played?: boolean; isFavorite?: boolean;
  currentSeasonNumber?: number; currentEpisodeNumber?: number;
  seasons?: MediaSeason[] | RequestableSeason[];
  mediaStatus?: number;
  status?: string; releaseDate?: string; studios?: string[];
};
type MediaTrackingEntry = { mediaSource: string; mediaId: string; mediaType: string; title: string; posterUrl: string; rating?: number; onWatchlist: boolean; rewatchCount: number; hiddenInProgress?: boolean };
function isRequestableSeason(season: MediaSeason | RequestableSeason): season is RequestableSeason { return !("id" in season); }
type IconName = "home" | "chart" | "sparkle" | "user" | "bell" | "arrow" | "close" | "clock" | "movie" | "series" | "genre" | "medal" | "refresh" | "chat" | "settings";
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
function visibleNav(user: CurrentUser): { label: Page; icon: IconName }[] {
  const items: { label: Page; icon: IconName }[] = [{ label: "Heute", icon: "home" }];
  if (user.features.statistics) items.push({ label: "Statistik", icon: "chart" });
  if (user.features.requests) items.push({ label: "Anfragen", icon: "sparkle" });
  items.push({ label: "Chats", icon: "chat" }, { label: "Profil", icon: "user" });
  if (user.isAdmin) items.push({ label: "Verwaltung", icon: "settings" });
  return items;
}
const apiPeriod: Record<Period, StatisticsPeriod> = { Woche: "week", Monat: "month", Jahr: "year" };
const APP_VERSION = "0.8.51";

const dateFormatter = new Intl.DateTimeFormat("de-DE", { day: "2-digit", month: "short" });
function formatPremiereDate(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "" : dateFormatter.format(date);
}
function availabilityWording(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "Demnächst verfügbar";
  const today = new Date(); today.setHours(0, 0, 0, 0);
  const release = new Date(date); release.setHours(0, 0, 0, 0);
  const days = Math.round((release.getTime() - today.getTime()) / 86_400_000);
  if (days <= 0) return "Heute verfügbar";
  if (days === 1) return "Verfügbar morgen";
  if (days < 7) return `Verfügbar in ${days} Tagen`;
  const weeks = Math.ceil(days / 7);
  return `Verfügbar in ${weeks} ${weeks === 1 ? "Woche" : "Wochen"}`;
}
function cinemaWording(item: UpcomingItem) {
  const start = item.cinemaStartDate ? new Date(item.cinemaStartDate) : null;
  if (start && start.getTime() > Date.now()) return `Kinostart ${formatPremiereDate(item.cinemaStartDate!)}`;
  return item.cinemaEndDate ? `Im Kino bis ${formatPremiereDate(item.cinemaEndDate)}` : "Im Kino";
}
function upcomingTitle(item: UpcomingItem) {
  if (item.mediaType !== "tv") return item.title;
  const episode = item.seasonNumber && item.episodeNumber ? `S${item.seasonNumber.toString().padStart(2, "0")}E${item.episodeNumber.toString().padStart(2, "0")}` : "Neue Folge";
  return `${item.title} · ${episode}`;
}
const fullDateFormatter = new Intl.DateTimeFormat("de-DE", { day: "2-digit", month: "long", year: "numeric" });
function formatFullDate(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "" : fullDateFormatter.format(date);
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

export default function Home() {
  const [page, setPage] = useState<Page>("Heute");
  const [noticeOpen, setNoticeOpen] = useState(false);
  const [user, setUser] = useState<CurrentUser | null>(null);
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

  const [upcomingItems, upcomingState] = useApiResource<UpcomingItem[]>(user?.features.upcoming ? "/api/upcoming" : null, []);
  const [cinemaItems, cinemaState] = useApiResource<UpcomingItem[]>(user?.features.movieDates ? "/api/in-cinemas" : null, []);
  const [requestItems, requestState, refetchRequestItems] = useApiResource<RequestItem[]>(user?.features.requests ? "/api/requests" : null, []);
  const [requestTotal, , refetchRequestTotal] = useApiResource<{ total: number } | null>(user?.features.requests ? "/api/requests/total" : null, null);
  const totalRequests = requestTotal?.total ?? null;
  const [newForYouItems, newForYouState] = useApiResource<NewForYouItem[]>(user ? "/api/new-for-you" : null, []);
  const [seriesInProgress, seriesInProgressState, refetchSeriesInProgress] = useApiResource<SeriesProgress[]>(user ? "/api/series-in-progress" : null, []);
  const [availableRequests] = useApiResource<RequestItem[]>(user?.features.requests ? "/api/requests/available" : null, []);
  const [userProfile] = useApiResource<UserProfile | null>(user ? "/api/me/profile" : null, null);

  const refetchRequests = () => { refetchRequestItems(); refetchRequestTotal(); };

  const [unreadData] = useApiResource<{ count: number }>(user ? "/api/messages/unread-count" : null, { count: 0 }, UNREAD_POLL_MS);
  const unread = unreadData.count;
  const [ownThread] = useApiResource<ChatMessage[]>(user && !user.isAdmin && unread > 0 ? "/api/messages" : null, [], CHAT_POLL_MS);
  const latestAdminMessage = [...ownThread].reverse().find((message) => message.fromAdmin);
  const messagePreview = !user || unread === 0 ? null
    : user.isAdmin ? { preview: `${unread} neue ${unread === 1 ? "Nachricht" : "Nachrichten"} von Nutzern` }
    : { preview: latestAdminMessage ? latestAdminMessage.body : "Neue Nachricht erhalten" };

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

  if (checkingSession) return <main className="login-shell"><p className="loading-copy" role="status">Emby Insights wird geladen …</p></main>;
  if (!user) return <LoginScreen onAuthenticated={setUser} />;

  const selectPage = (next: Page) => { setPage(next); setNoticeOpen(false); };
  const openNotices = () => setNoticeOpen((open) => !open);
  const nav = visibleNav(user);

  return <main className="app-shell">
    {selectedMedia && <MediaDetailScreen selection={selectedMedia} onClose={() => setSelectedMedia(null)} onRequestCreated={refetchRequests} onHiddenChanged={refetchSeriesInProgress} />}
    <a className="skip-link" href="#dashboard-content">Zum Inhalt springen</a>
    <aside className="side-nav" aria-label="Hauptnavigation">
      <button type="button" className="brand" onClick={goHomeAndRefresh} aria-label="Zu Heute und Dashboard aktualisieren"><img className="brand-logo" src="/emby-insights-logo.svg" alt="Emby Insights" width="31" height="31" /><span>insights</span></button>
      <nav>{nav.map((item) => <button className={page === item.label ? "nav-item active" : "nav-item"} key={item.label} onClick={() => selectPage(item.label)} aria-current={page === item.label ? "page" : undefined}><Icon name={item.icon} />{item.label}</button>)}</nav>
      <div className="sidebar-meta">
        <div className="server-status"><i aria-hidden="true" /> Verbunden mit Emby</div>
        <p className="app-version">Version <strong>v{APP_VERSION}</strong></p>
      </div>
    </aside>

    <section className="screen" id="dashboard-content" tabIndex={-1}>
      <header className="topbar">
        <div><p className="eyebrow">DEIN PERSÖNLICHER ÜBERBLICK</p><h1>{page === "Heute" ? `${greeting()}, ${user.name}` : page}</h1></div>
        <div className="header-actions">
          <button type="button" className="refresh-button" aria-label="Dashboard aktualisieren" onClick={() => window.location.reload()}><Icon name="refresh" /></button>
          <button ref={noticeButtonRef} className="notice-button" aria-label="Benachrichtigungen" aria-expanded={noticeOpen} aria-controls="notifications" onClick={openNotices}><Icon name="bell" />{unread > 0 && <b><span className="sr-only">{unread} ungelesene Benachrichtigungen</span></b>}</button>
          <button className="avatar" aria-label="Profil öffnen" onClick={() => selectPage("Profil")}><UserAvatar name={user.name} userId={user.id} /></button>
          {noticeOpen && <div ref={noticeRef} className="notifications" id="notifications" role="dialog" aria-label="Benachrichtigungen">
            <strong>Benachrichtigungen</strong>
            {messagePreview ? <p>{messagePreview.preview}</p> : <p>Keine neuen Benachrichtigungen.</p>}
            {unread > 0 && <button type="button" className="text-button" onClick={() => selectPage("Chats")}>Zu den Chats <Icon name="arrow" /></button>}
          </div>}
        </div>
      </header>
      {page === "Heute" && <Today upcoming={upcomingItems} upcomingState={upcomingState} cinema={cinemaItems} cinemaState={cinemaState} requests={requestItems} requestState={requestState} newForYou={newForYouItems} newForYouState={newForYouState} seriesInProgress={seriesInProgress} seriesInProgressState={seriesInProgressState} availableRequests={availableRequests} features={user.features} message={messagePreview} onSelectMedia={setSelectedMedia} onOpenChats={() => selectPage("Chats")} />}
      {page === "Statistik" && user.features.statistics && <Stats user={user} onSelectMedia={setSelectedMedia} />}
      {page === "Anfragen" && user.features.requests && <Requests onSelectMedia={setSelectedMedia} />}
      {page === "Chats" && <Chats user={user} />}
      {page === "Profil" && <Profile user={user} userProfile={userProfile} totalRequests={user.features.requests ? totalRequests : null} onSelectMedia={setSelectedMedia} />}
      {page === "Verwaltung" && user.isAdmin && <AdminSettings />}
    </section>

    <nav className="bottom-nav" aria-label="Hauptnavigation (mobil)">{nav.filter((item) => item.label !== "Profil").map((item) => <button key={item.label} className={page === item.label ? "active" : ""} onClick={() => selectPage(item.label)} aria-current={page === item.label ? "page" : undefined}><Icon name={item.icon} /><span className="sr-only">{item.label}</span></button>)}</nav>
  </main>;
}

function Icon({ name }: { name: IconName }) {
  const paths: Record<IconName, ReactNode> = {
    home: <path d="m3 10 9-7 9 7v10a1 1 0 0 1-1 1h-5v-6H9v6H4a1 1 0 0 1-1-1V10Z" />,
    chart: <><path d="M4 20V10m8 10V4m8 16v-7" /><path d="M2 20h20" /></>,
    sparkle: <path d="m12 2 1.8 6.2L20 10l-6.2 1.8L12 18l-1.8-6.2L4 10l6.2-1.8L12 2Zm7 13 .8 2.2L22 18l-2.2.8L19 21l-.8-2.2L16 18l2.2-.8L19 15Z" />,
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

function Today({ upcoming, upcomingState, cinema, cinemaState, requests, requestState, newForYou, newForYouState, seriesInProgress, seriesInProgressState, availableRequests, features, message, onSelectMedia, onOpenChats }: {
	upcoming: UpcomingItem[]; upcomingState: LoadState; requests: RequestItem[]; requestState: LoadState;
	cinema: UpcomingItem[]; cinemaState: LoadState;
  newForYou: NewForYouItem[]; newForYouState: LoadState; availableRequests: RequestItem[];
  seriesInProgress: SeriesProgress[]; seriesInProgressState: LoadState;
  features: Features;
  message: { preview: string } | null;
  onSelectMedia: (selection: MediaSelection) => void; onOpenChats: () => void;
}) {
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
    onSelectMedia,
    onShowNewForYou: () => setNewForYouGridOpen(true),
    onOpenChats,
  });

  return <div className="content today-view">
    <RelevantNow events={events} onShowAll={() => setAllEventsOpen(true)} />
    {features.upcoming && <PosterRow title="Demnächst" eyebrow="IN DEN NÄCHSTEN 4 WOCHEN AUF EMBY" items={visibleUpcoming} state={upcomingState} emptyLabel="Nichts Neues in den nächsten vier Wochen." itemTitle={upcomingTitle} detail={(item) => availabilityWording(item.availabilityDate)} onSelect={(item) => onSelectMedia({ source: "seerr", id: item.tmdbId, mediaType: item.mediaType })} />}
    <PosterRow title="Noch nicht fertig" eyebrow="TEILWEISE GESEHEN" items={seriesInProgress} state={seriesInProgressState} emptyLabel="Keine Serien mit offenen Folgen." detail={(item) => `${item.watchedEpisodes} von ${item.totalEpisodes} Folgen`} progress={(item) => Math.round((item.watchedEpisodes / item.totalEpisodes) * 100)} onSelect={(item) => onSelectMedia({ source: "emby", id: item.id })} />
    {features.requests && <PosterRow title="Meine Anfragen" eyebrow="SEERR · OFFEN" items={requests} state={requestState} emptyLabel="Keine offenen Anfragen." detail={(item) => item.status} onSelect={(item) => onSelectMedia({ source: "seerr", id: item.tmdbId, mediaType: item.mediaType })} />}
    <PosterRow title="Neu für dich" eyebrow="IN DEN LETZTEN 14 TAGEN" items={newForYou} state={newForYouState} emptyLabel="Nichts Neues in den letzten 14 Tagen." detail={() => "Ungesehen"} onSelect={(item) => onSelectMedia({ source: "emby", id: item.id })} />
    {features.movieDates && <PosterRow title="Im Kino" eyebrow="KOMMENDE UND AKTUELLE KINOSTARTS" items={cinema} state={cinemaState} emptyLabel="Zurzeit keine Kinostarts." detail={cinemaWording} onSelect={(item) => onSelectMedia({ source: "seerr", id: item.tmdbId, mediaType: item.mediaType })} />}

    {allEventsOpen && <RelevantAllScreen events={events} onClose={() => setAllEventsOpen(false)} />}
    {newForYouGridOpen && <MediaGridScreen title="Neu für dich" items={newForYou} detail={() => "Ungesehen"} onSelect={(item) => onSelectMedia({ source: "emby", id: item.id })} onClose={() => setNewForYouGridOpen(false)} />}
  </div>;
}

type RelevantEvent = { key: string; tone: Tone; icon: IconName; status: string; detail: ReactNode; onOpen: () => void };

const RELEASE_WINDOW_HOURS = 48;

function releaseWording(premiereDate: string) {
  const date = new Date(premiereDate);
  if (Number.isNaN(date.getTime())) return "Erscheint bald";
  const startOfToday = new Date();
  startOfToday.setHours(0, 0, 0, 0);
  const days = Math.round((new Date(date).setHours(0, 0, 0, 0) - startOfToday.getTime()) / 86_400_000);
  if (days <= 0) return "Heute erscheint";
  if (days === 1) return "Morgen erscheint";
  return "Erscheint bald";
}

// Builds the prioritised event list: newly available requests first, then
// releases due within the next two days, then the unseen-titles summary.
function relevantEvents({ availableRequests, upcoming, cinema, newForYou, message, onSelectMedia, onShowNewForYou, onOpenChats }: {
  availableRequests: RequestItem[]; upcoming: UpcomingItem[]; cinema: UpcomingItem[]; newForYou: NewForYouItem[];
  message: { preview: string } | null;
  onSelectMedia: (selection: MediaSelection) => void; onShowNewForYou: () => void; onOpenChats: () => void;
}): RelevantEvent[] {
  const events: RelevantEvent[] = [];

  if (message) {
    events.push({
      key: "message", tone: "blue", icon: "chat", status: "Neue Nachricht",
      detail: message.preview,
      onOpen: onOpenChats,
    });
  }

  for (const request of availableRequests) {
    events.push({
      key: `available-${request.id}`, tone: "mint", icon: "sparkle", status: "Jetzt verfügbar",
      detail: <>Deine Anfrage „{request.title}“ ist in Emby</>,
      onOpen: () => onSelectMedia({ source: "seerr", id: request.tmdbId, mediaType: request.mediaType }),
    });
  }

  const horizon = Date.now() + RELEASE_WINDOW_HOURS * 60 * 60 * 1000;
  for (const item of upcoming) {
    const premiere = new Date(item.availabilityDate).getTime();
    if (Number.isNaN(premiere) || premiere > horizon) continue;
    events.push({
      key: `release-${item.id}`, tone: "peach", icon: "clock", status: releaseWording(item.availabilityDate),
      detail: upcomingTitle(item),
      onOpen: () => onSelectMedia({ source: "seerr", id: item.tmdbId, mediaType: item.mediaType }),
    });
  }

  for (const item of cinema) {
    const premiere = new Date(item.cinemaStartDate ?? item.availabilityDate).getTime();
    if (Number.isNaN(premiere) || premiere > horizon || premiere < Date.now() - 24 * 60 * 60 * 1000) continue;
    events.push({
      key: `cinema-${item.id}`, tone: "peach", icon: "movie", status: premiere <= Date.now() ? "Jetzt im Kino" : "Bald im Kino",
      detail: item.title,
      onOpen: () => onSelectMedia({ source: "seerr", id: item.tmdbId, mediaType: item.mediaType }),
    });
  }

  if (newForYou.length > 0) {
    events.push({
      key: "new-for-you", tone: "blue", icon: "genre", status: "Neu für dich",
      detail: <><b>{newForYou.length}</b> ungesehene Titel</>,
      onOpen: onShowNewForYou,
    });
  }

  return events;
}

function RelevantNow({ events, onShowAll }: { events: RelevantEvent[]; onShowAll: () => void }) {
  return <section className="relevant-card" aria-label="Jetzt relevant">
    <div className="relevant-head">
      <p className="eyebrow">JETZT RELEVANT</p>
      {events.length > 3 && <button type="button" className="text-button" onClick={onShowAll}>Alle ansehen <Icon name="arrow" /></button>}
    </div>
    {events.length === 0
      ? <p className="relevant-empty">Heute nichts Neues für dich.</p>
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

  return <div className="media-detail-overlay media-grid-overlay" role="dialog" aria-modal="true" aria-label="Jetzt relevant">
    <div className="media-detail-scroll">
      <button type="button" className="media-detail-close" onClick={onClose} aria-label="Schließen"><Icon name="close" /></button>
      <h1 className="media-grid-title">Jetzt relevant</h1>
      <ul className="relevant-list">{events.map((event) => <RelevantRow key={event.key} event={event} onOpen={() => { onClose(); event.onOpen(); }} />)}</ul>
    </div>
  </div>;
}

function MetricCard({ icon, tone, value, label, detail, positive, genre = false, loading, onClick }: { icon: IconName; tone: Tone; value: string | number; label: string; detail: string; positive?: boolean; genre?: boolean; loading?: boolean; onClick?: () => void }) {
  const inner = loading
    ? <><span className="metric-icon"><Icon name={icon} /></span><span className="skeleton skeleton-value" aria-hidden="true" /><p>{label}</p><span className="skeleton skeleton-detail" aria-hidden="true" /><span className="sr-only" role="status">Wird geladen …</span></>
    : <><span className="metric-icon"><Icon name={icon} /></span><strong>{value}</strong><p>{label}</p><small className={positive ? "up" : undefined}>{detail}</small></>;
  return onClick
    ? <button type="button" className={`metric-card metric-card-button tone-${tone}${genre ? " genre-card" : ""}`} onClick={onClick}>{inner}</button>
    : <article className={`metric-card tone-${tone}${genre ? " genre-card" : ""}`}>{inner}</article>;
}

// Same vertical flow as MetricCard (icon, then strong/p/small stacked in
// normal document flow) instead of the previous side-by-side avatar+text
// row — that horizontal layout needed the avatar and a wrapping subtitle to
// both fit a fixed row height, and on narrower widths the text overflowed
// above the card instead of just growing the box like every other card.
function RankCard({ rank, name, userId }: { rank: number | null; name: string; userId: string }) {
  const hasRank = rank !== null && rank > 0;
  const medalClass = rank === 1 ? " gold" : rank === 3 ? " bronze" : "";
  return <article className="metric-card rank-card tone-lilac">
    <span className="rank-avatar-badge">
      <span className="rank-avatar"><UserAvatar name={name} userId={userId} /></span>
      <span className={`rank-badge${medalClass}`}>{hasRank ? rank : "—"}</span>
    </span>
    <strong>{hasRank ? `Platz ${rank}` : "—"}</strong>
    <p>Dein Platz</p>
    <small>Nach Sehzeit unter allen Nutzern</small>
  </article>;
}

function PosterRow<T extends { id: string; title: string; posterUrl?: string }>({ title, eyebrow, gridTitle, items, detail, itemTitle, state, emptyLabel, progress, onSelect }: {
  title?: string; eyebrow?: string; gridTitle?: string; items: readonly T[]; detail: (item: T) => string; itemTitle?: (item: T) => string; state?: LoadState; emptyLabel?: string; progress?: (item: T) => number; onSelect?: (item: T) => void;
}) {
  const [gridOpen, setGridOpen] = useState(false);
  const resolvedGridTitle = gridTitle ?? title ?? "Übersicht";
  return <section className="poster-section">
    {(title || eyebrow) && <div className="section-heading">
      <div>{eyebrow && <p className="eyebrow">{eyebrow}</p>}{title && <h2>{title}</h2>}</div>
      {items.length > 0 && <button type="button" className="text-button poster-view-all" onClick={() => setGridOpen(true)} aria-label={`Alle ${resolvedGridTitle} anzeigen`}><Icon name="arrow" /></button>}
    </div>}
    {state === "loading" && <PosterSkeletonRow />}
    {state === "error" && <p className="poster-status">Nicht verfügbar</p>}
    {state !== "loading" && state !== "error" && items.length === 0 && <p className="poster-status">{emptyLabel ?? "Nichts vorhanden."}</p>}
    {items.length > 0 && <div className="poster-scroller">{items.map((item) => {
      const inner = <>
        <div className="poster wide" role="img" aria-label={itemTitle?.(item) ?? item.title}>
          {item.posterUrl ? <img src={item.posterUrl} alt="" loading="lazy" /> : <span>{itemTitle?.(item) ?? item.title}</span>}
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
  return <>
    <p className="poster-status sr-only" role="status">Wird geladen …</p>
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

const WEEKDAYS = ["Mo", "Di", "Mi", "Do", "Fr", "Sa", "So"];

function weekdayChartData(weekdays: readonly WeekdayWatchTime[]) {
  const seconds = new Array(7).fill(0);
  for (const entry of weekdays) seconds[entry.weekday] = entry.watchSeconds;
  return WEEKDAYS.map((label, index) => ({ label, value: seconds[index] }));
}

const DAYPARTS = [
  { label: "Nacht", hours: [0, 1, 2, 3, 4, 5] },
  { label: "Morgen", hours: [6, 7, 8, 9, 10, 11] },
  { label: "Nachmittag", hours: [12, 13, 14, 15, 16, 17] },
  { label: "Abend", hours: [18, 19, 20, 21, 22, 23] },
];

function daypartChartData(hours: readonly HourWatchTime[]) {
  const secondsByHour = new Array(24).fill(0);
  for (const entry of hours) secondsByHour[entry.hour] = entry.watchSeconds;
  return DAYPARTS.map((daypart) => ({ label: daypart.label, value: daypart.hours.reduce((sum, hour) => sum + secondsByHour[hour], 0) }));
}

function BarChart({ title, data, formatValue, loading }: { title: string; data: { label: string; value: number }[]; formatValue?: (value: number) => string; loading?: boolean }) {
  const max = Math.max(1, ...data.map((entry) => entry.value));
  const hasData = data.some((entry) => entry.value > 0);
  return <section className="chart-card">
    <h3>{title}</h3>
    {loading
      ? <div className="bar-chart" aria-hidden="true">
        <p className="sr-only" role="status">Wird geladen …</p>
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
      : <p className="poster-status">Noch keine Daten für diesen Zeitraum.</p>}
  </section>;
}

function Stats({ user, onSelectMedia }: { user: { id: string; name: string }; onSelectMedia: (selection: MediaSelection) => void }) {
  const [period, setPeriod] = useState<Period>("Woche");
  const [completedGridView, setCompletedGridView] = useState<"movies" | "series" | null>(null);

  const [statistics, state] = useApiResource<PersonalStats | null>(`/api/stats?period=${apiPeriod[period]}`, null);
  const [watchTimeRankData] = useApiResource<WatchTimeRank | null>("/api/stats/rank", null);
  const watchTimeRank = watchTimeRankData?.rank ?? null;
  const [continueWatching, continueWatchingState] = useApiResource<ContinueWatchingItem[]>("/api/continue-watching", []);
  const [watchedMovies, watchedMoviesState] = useApiResource<WatchedItem[]>("/api/watched-movies", []);
  const [watchedSeries, watchedSeriesState] = useApiResource<WatchedItem[]>("/api/watched-series", []);
  const [completedMovies, completedMoviesState] = useApiResource<WatchedItem[]>(`/api/completed-movies?period=${apiPeriod[period]}`, []);
  const [completedSeries, completedSeriesState] = useApiResource<WatchedItem[]>(`/api/completed-series?period=${apiPeriod[period]}`, []);
  const [deviceStats, deviceStatsState] = useApiResource<DeviceWatchTime[]>(`/api/stats/devices?period=${apiPeriod[period]}`, []);
  const [hourStats, hourStatsState] = useApiResource<HourWatchTime[]>(`/api/stats/hours?period=${apiPeriod[period]}`, []);
  const [weekdayStats, weekdayStatsState] = useApiResource<WeekdayWatchTime[]>(`/api/stats/weekdays?period=${apiPeriod[period]}`, []);
  const [longestSession, longestSessionState] = useApiResource<LongestSession | null>(`/api/stats/longest-session?period=${apiPeriod[period]}`, null);
  const [mostActiveDay, mostActiveDayState] = useApiResource<MostActiveDay | null>(`/api/stats/most-active-day?period=${apiPeriod[period]}`, null);

  return <div className="content page-view">
    <section className="period-tabs" aria-label="Zeitraum auswählen">{(["Woche", "Monat", "Jahr"] as Period[]).map((item) => <button className={period === item ? "selected" : ""} onClick={() => setPeriod(item)} key={item} aria-pressed={period === item}>{item}</button>)}</section>
    <section className="week-grid" aria-label={`Kennzahlen für ${period}`}>
      <RankCard rank={watchTimeRank} name={user.name} userId={user.id} />
      <MetricCard icon="clock" tone="blue" value={statistics ? formatDuration(statistics.watchSeconds) : "—"} label="Sehzeit" detail={statistics ? comparisonText(statistics) : loadingCopy(state)} loading={state === "loading"} />
      <MetricCard icon="movie" tone="peach" value={statistics ? statistics.completedMovies : "—"} label="Filme abgeschlossen" detail={statistics ? period : loadingCopy(state)} loading={state === "loading"} onClick={statistics && statistics.completedMovies > 0 && completedMoviesState === "ready" ? () => setCompletedGridView("movies") : undefined} />
      <MetricCard icon="series" tone="mint" value={statistics ? statistics.completedSeries : "—"} label="Serien abgeschlossen" detail={statistics ? period : loadingCopy(state)} loading={state === "loading"} onClick={statistics && statistics.completedSeries > 0 && completedSeriesState === "ready" ? () => setCompletedGridView("series") : undefined} />
    </section>

    <PosterRow title="Was ich gerade schaue" eyebrow="WEITERSCHAUEN" items={continueWatching} state={continueWatchingState} emptyLabel="Nichts in Bearbeitung." detail={(item) => `${item.progressPercent} % gesehen`} progress={(item) => item.progressPercent} onSelect={(item) => onSelectMedia({ source: "emby", id: item.id })} />
    <PosterRow title="Gesehene Filme" eyebrow="ALLE" items={watchedMovies} state={watchedMoviesState} emptyLabel="Noch keine Filme abgeschlossen." detail={(item) => item.genres[0] ?? ""} onSelect={(item) => onSelectMedia({ source: "emby", id: item.id })} />
    <PosterRow title="Gesehene Serien" eyebrow="ALLE" items={watchedSeries} state={watchedSeriesState} emptyLabel="Noch keine Serien abgeschlossen." detail={(item) => item.genres[0] ?? ""} onSelect={(item) => onSelectMedia({ source: "emby", id: item.id })} />

    <section className="chart-grid">
      <BarChart title="Meistgesehene Genres" data={topGenres(watchedMovies, watchedSeries)} loading={watchedMoviesState === "loading" || watchedSeriesState === "loading"} />
      <BarChart title="Aktivität nach Wochentag" data={weekdayStatsState === "ready" ? weekdayChartData(weekdayStats) : []} formatValue={formatDuration} loading={weekdayStatsState === "loading"} />
      <BarChart title="Aktivität nach Uhrzeit" data={hourStatsState === "ready" ? daypartChartData(hourStats) : []} formatValue={formatDuration} loading={hourStatsState === "loading"} />
      <BarChart title="Nach Gerät" data={deviceStatsState === "ready" ? deviceStats.slice(0, 6).map((device) => ({ label: device.deviceName, value: device.watchSeconds })) : []} formatValue={formatDuration} loading={deviceStatsState === "loading"} />
    </section>

    <section className="records-grid" aria-label="Rekorde">
      <MetricCard icon="clock" tone="lilac" value={longestSessionState === "ready" && longestSession ? formatDuration(longestSession.watchSeconds) : "—"} label="Längste Session" detail={longestSessionState === "ready" ? (longestSession?.itemName ?? "Keine Daten für diesen Zeitraum") : loadingCopy(longestSessionState)} loading={longestSessionState === "loading"} />
      <MetricCard icon="genre" tone="peach" value={mostActiveDayState === "ready" && mostActiveDay ? formatFullDate(mostActiveDay.date) : "—"} label="Aktivster Tag" detail={mostActiveDayState === "ready" ? (mostActiveDay ? formatDuration(mostActiveDay.watchSeconds) : "Keine Daten für diesen Zeitraum") : loadingCopy(mostActiveDayState)} loading={mostActiveDayState === "loading"} />
    </section>

    {completedGridView === "movies" && <MediaGridScreen title={`Filme abgeschlossen · ${period}`} items={completedMovies} detail={(item) => item.genres[0] ?? ""} onSelect={(item) => onSelectMedia({ source: "emby", id: item.id })} onClose={() => setCompletedGridView(null)} />}
    {completedGridView === "series" && <MediaGridScreen title={`Serien abgeschlossen · ${period}`} items={completedSeries} detail={(item) => item.genres[0] ?? ""} onSelect={(item) => onSelectMedia({ source: "emby", id: item.id })} onClose={() => setCompletedGridView(null)} />}
  </div>;
}

function MediaGridScreen<T extends { id: string; title: string; posterUrl?: string }>({ title, items, detail, progress, state, emptyLabel, headerExtra, onSelect, onClose }: {
  title: string; items: readonly T[]; detail?: (item: T) => string; progress?: (item: T) => number;
  state?: LoadState; emptyLabel?: string; headerExtra?: ReactNode;
  onSelect?: (item: T) => void; onClose: () => void;
}) {
  useEscapeKey(onClose);

  return <div className="media-detail-overlay media-grid-overlay" role="dialog" aria-modal="true" aria-label={title}>
    <div className="media-detail-scroll">
      <button type="button" className="media-detail-close" onClick={onClose} aria-label="Schließen"><Icon name="close" /></button>
      <h1 className="media-grid-title">{title}</h1>
      {headerExtra}
      {state === "loading" && <>
        <p className="poster-status sr-only" role="status">Wird geladen …</p>
        <div className="media-grid" aria-hidden="true">
          {Array.from({ length: 8 }, (_, index) => <div className="media-grid-entry" key={index}>
            <div className="poster wide skeleton" />
            <span className="skeleton skeleton-line" /><span className="skeleton skeleton-line" />
          </div>)}
        </div>
      </>}
      {state === "error" && <p className="poster-status">Nicht verfügbar</p>}
      {state !== "loading" && state !== "error" && items.length === 0 && <p className="poster-status">{emptyLabel ?? "Nichts vorhanden."}</p>}
      {items.length > 0 && <div className="media-grid">{items.map((item) => <button type="button" className="media-grid-entry" key={item.id} onClick={() => onSelect?.(item)}>
        <div className="poster wide" role="img" aria-label={item.title}>
          {item.posterUrl ? <img src={item.posterUrl} alt="" loading="lazy" /> : <span>{item.title}</span>}
          {progress && <div className="poster-progress"><div className="poster-progress-fill" style={{ width: `${progress(item)}%` }} /></div>}
        </div>
        <strong>{item.title}</strong>
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
        placeholder="Filme oder Serien suchen …"
        value={query}
        onChange={(event) => setQuery(event.target.value)}
        aria-label="Bei Seerr suchen"
      />
      <button type="submit" className="search-button" disabled={query.trim() === ""}>Suchen</button>
    </form>
    {searchScreenQuery !== null && <MediaGridScreen
      title={`Suchergebnisse für „${searchScreenQuery}“`}
      items={searchResults}
      state={searchState}
      emptyLabel="Keine Treffer."
      detail={(item) => item.mediaType === "tv" ? "Serie" : "Film"}
      onSelect={(item) => onSelectMedia({ source: "seerr", id: item.id, mediaType: item.mediaType })}
      onClose={() => setSearchScreenQuery(null)}
      headerExtra={<form className="search-form media-grid-search" onSubmit={(event) => { event.preventDefault(); runSearch(query); }}>
        <input type="search" className="search-input" placeholder="Filme oder Serien suchen …" value={query} onChange={(event) => setQuery(event.target.value)} aria-label="Bei Seerr suchen" />
        <button type="submit" className="search-button" disabled={query.trim() === ""}>Suchen</button>
      </form>}
    />}

    <PosterRow title="Im Trend" eyebrow="SEERR · TMDB" items={trending} state={trendingState} emptyLabel="Nichts im Trend." detail={(item) => item.mediaType === "tv" ? "Serie" : "Film"} onSelect={(item) => onSelectMedia({ source: "seerr", id: item.id, mediaType: item.mediaType })} />
    <PosterRow title="Beliebte Filme" eyebrow="SEERR · TMDB" items={popularMovies} state={popularMoviesState} emptyLabel="Keine Daten." detail={() => "Beliebt"} onSelect={(item) => onSelectMedia({ source: "seerr", id: item.id, mediaType: item.mediaType })} />
    <PosterRow title="Demnächst erscheinende Filme" eyebrow="SEERR · TMDB" items={upcomingMovies} state={upcomingMoviesState} emptyLabel="Keine Daten." detail={() => "Demnächst"} onSelect={(item) => onSelectMedia({ source: "seerr", id: item.id, mediaType: item.mediaType })} />
    <PosterRow title="Beliebte Serien" eyebrow="SEERR · TMDB" items={popularSeries} state={popularSeriesState} emptyLabel="Keine Daten." detail={() => "Beliebt"} onSelect={(item) => onSelectMedia({ source: "seerr", id: item.id, mediaType: item.mediaType })} />
    <PosterRow title="Demnächst erscheinende Serien" eyebrow="SEERR · TMDB" items={upcomingSeries} state={upcomingSeriesState} emptyLabel="Keine Daten." detail={() => "Demnächst"} onSelect={(item) => onSelectMedia({ source: "seerr", id: item.id, mediaType: item.mediaType })} />

    <section className="poster-section">
      <div className="section-heading"><div><p className="eyebrow">WO STREAMEN?</p><h2>Anbieter</h2></div></div>
      <div className="provider-scroller">
        {STREAMING_PROVIDERS.map((provider) => <button type="button" key={provider.id} className="provider-chip" onClick={() => setProviderId(provider.id)}>{provider.name}</button>)}
      </div>
    </section>
    {selectedProvider && <MediaGridScreen
      title={selectedProvider.name}
      items={providerItems}
      state={providerItemsState}
      emptyLabel="Keine Titel gefunden."
      detail={(item) => item.mediaType === "tv" ? "Serie" : "Film"}
      onSelect={(item) => onSelectMedia({ source: "seerr", id: item.id, mediaType: item.mediaType })}
      onClose={() => setProviderId(null)}
    />}
  </div>;
}

const OVERVIEW_CLAMP_THRESHOLD = 220;

function OverviewText({ text }: { text: string }) {
  const [expanded, setExpanded] = useState(false);
  const isLong = text.length > OVERVIEW_CLAMP_THRESHOLD;
  return <>
    <p className={isLong && !expanded ? "media-detail-overview-text clamped" : "media-detail-overview-text"}>{text}</p>
    {isLong && <button type="button" className="text-button overview-toggle" onClick={() => setExpanded((value) => !value)}>{expanded ? "Weniger anzeigen" : "Mehr anzeigen"}</button>}
  </>;
}
function MediaDetailScreen({ selection, onClose, onRequestCreated, onHiddenChanged }: { selection: MediaSelection; onClose: () => void; onRequestCreated: () => void; onHiddenChanged?: () => void }) {
  const [detail, setDetail] = useState<MediaDetail | null>(null);
  const [state, setState] = useState<LoadState>("loading");
  const [selectedSeasons, setSelectedSeasons] = useState<number[]>([]);
  const [requestState, setRequestState] = useState<"idle" | "submitting" | "done" | "error">("idle");
  const [requestModalOpen, setRequestModalOpen] = useState(false);
  const [tracking, setTracking] = useState<{ rating: number; onWatchlist: boolean; hiddenInProgress: boolean }>({ rating: 0, onWatchlist: false, hiddenInProgress: false });
  const [favorite, setFavorite] = useState(false);
  const [favoriteBusy, setFavoriteBusy] = useState(false);
  const mediaType = selection.source === "seerr" ? selection.mediaType : undefined;

  useEffect(() => {
    let active = true;
    fetch(`/api/tracking?source=${selection.source}&id=${encodeURIComponent(selection.id)}`, { credentials: "include" })
      .then(async (response) => {
        if (!response.ok) throw new Error("tracking unavailable");
        const data: MediaTrackingEntry = await response.json();
        if (active) setTracking({ rating: data.rating ?? 0, onWatchlist: data.onWatchlist, hiddenInProgress: data.hiddenInProgress ?? false });
      })
      .catch(() => null);
    return () => { active = false; };
  }, [selection.source, selection.id]);

  // Always resends hiddenInProgress (Upsert overwrites the whole row), so a
  // plain rating/watchlist change never silently un-hides a dismissed series.
  const saveTracking = (next: { rating: number; onWatchlist: boolean; hiddenInProgress: boolean }) => {
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

  useEffect(() => {
    let active = true;
    const url = selection.source === "emby"
      ? `/api/media/emby?id=${encodeURIComponent(selection.id)}`
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
  }, [selection.source, selection.id, mediaType]);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      if (requestModalOpen) { setRequestModalOpen(false); return; }
      onClose();
    };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [onClose, requestModalOpen]);

  const status = detail ? mediaStatus(detail) : null;
  const crewAndCast = detail ? [...(detail.crew ?? []), ...(detail.cast ?? [])] : [];
  const embySeasons = detail?.seasons?.filter((season): season is MediaSeason => !isRequestableSeason(season)) ?? [];
  const requestableSeasons = detail?.seasons?.filter(isRequestableSeason) ?? [];
  const canRequest = selection.source === "seerr" && !detail?.mediaStatus;

  const toggleSeason = (seasonNumber: number) => {
    setSelectedSeasons((current) => current.includes(seasonNumber) ? current.filter((value) => value !== seasonNumber) : [...current, seasonNumber]);
  };

  const submitRequest = async () => {
    if (selection.source !== "seerr") return;
    setRequestState("submitting");
    try {
      const response = await fetch("/api/media/seerr/request", {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ mediaType: selection.mediaType, tmdbId: Number(selection.id), seasons: selection.mediaType === "tv" ? selectedSeasons : undefined }),
      });
      if (!response.ok) throw new Error("request failed");
      setRequestState("done");
      setRequestModalOpen(false);
      onRequestCreated();
    } catch {
      setRequestState("error");
    }
  };

  return <div className="media-detail-overlay" role="dialog" aria-modal="true" aria-label={detail?.title ?? "Details"}>
    {detail && <div className="media-detail-backdrop" style={detail.backdropUrl ? { backgroundImage: `url(${detail.backdropUrl})` } : undefined} />}
    <div className="media-detail-scroll">
      <button type="button" className="media-detail-close" onClick={onClose} aria-label="Schließen"><Icon name="close" /></button>
      {state === "loading" && <p className="poster-status media-detail-status" role="status">Wird geladen …</p>}
      {state === "error" && <p className="poster-status media-detail-status">Details nicht verfügbar.</p>}
      {detail && <div className="media-detail">
        <div className="media-detail-above-fold">
        <div className="media-detail-hero">
          <div className="media-detail-poster">{detail.posterUrl ? <img src={detail.posterUrl} alt="" /> : <span>{detail.title}</span>}</div>
          <div className="media-detail-info">
            {status && <span className="media-status-badge">{status}</span>}
            {detail.currentSeasonNumber !== undefined && detail.currentEpisodeNumber !== undefined && <span className="media-status-badge media-status-badge-progress">Staffel {detail.currentSeasonNumber} · Folge {detail.currentEpisodeNumber}</span>}
            <h1>{detail.title}{detail.year ? ` (${detail.year})` : ""}</h1>
            <p className="media-detail-meta">
              {detail.officialRating && <span>{detail.officialRating}</span>}
              {detail.runtimeMinutes > 0 && <span>{detail.runtimeMinutes} Minuten</span>}
              {detail.genres?.length > 0 && <span>{detail.genres.join(", ")}</span>}
            </p>
            {detail.communityRating > 0 && <p className="media-detail-rating">★ {detail.communityRating.toFixed(1)}</p>}
          </div>
        </div>
        {(detail.status || detail.releaseDate || (detail.studios && detail.studios.length > 0)) && <section className="media-detail-facts">
          <dl>
            {detail.status && <div><dt>Status</dt><dd>{detail.status}</dd></div>}
            {detail.releaseDate && <div><dt>Erscheinungsdatum</dt><dd>{formatFullDate(detail.releaseDate)}</dd></div>}
            {detail.studios && detail.studios.length > 0 && <div><dt>Studios</dt><dd>{detail.studios.join(", ")}</dd></div>}
          </dl>
        </section>}
        <div className="tracking-bar">
          {selection.source === "emby" && <>
            <div className="star-rating" role="radiogroup" aria-label="Deine Bewertung">
              {[1, 2, 3, 4, 5].map((value) => <button
                key={value}
                type="button"
                className={value <= tracking.rating ? "star-button filled" : "star-button"}
                aria-pressed={value <= tracking.rating}
                aria-label={`${value} von 5 Sternen`}
                onClick={() => saveTracking({ ...tracking, rating: value === tracking.rating ? 0 : value })}
              >★</button>)}
            </div>
            <button
              type="button"
              className={favorite ? "icon-toggle-button active" : "icon-toggle-button"}
              onClick={toggleFavorite}
              disabled={favoriteBusy}
              aria-pressed={favorite}
              aria-label={favorite ? "Aus Favoriten entfernen" : "Zu Favoriten hinzufügen"}
            ><Icon name="sparkle" /></button>
          </>}
          <label className="watchlist-toggle">
            <span>Merkliste</span>
            <span className="toggle-switch">
              <input type="checkbox" checked={tracking.onWatchlist} onChange={() => saveTracking({ ...tracking, onWatchlist: !tracking.onWatchlist })} />
              <span className="toggle-track"><span className="toggle-thumb" /></span>
            </span>
          </label>
        </div>
        {selection.source === "emby" && detail.isSeries && <div className="request-row">
          <button type="button" className="request-button secondary" onClick={() => saveTracking({ ...tracking, hiddenInProgress: !tracking.hiddenInProgress })}>
            {tracking.hiddenInProgress ? "Wieder in „Noch nicht fertig“ zeigen" : "Aus „Noch nicht fertig“ ausblenden"}
          </button>
        </div>}
        {canRequest && <div className="request-row">
          {requestState === "done"
            ? <p className="request-confirmation">Angefragt ✓</p>
            : <button type="button" className="request-button" onClick={() => setRequestModalOpen(true)}>Anfragen</button>}
        </div>}
        {detail.overview && <section className="media-detail-overview"><h2>Übersicht</h2><OverviewText text={detail.overview} /></section>}
        </div>
        {embySeasons.length > 0 && <section className="media-detail-seasons">
          <h2>Staffeln</h2>
          <div className="poster-scroller">{embySeasons.map((season) => {
            const progress = season.totalEpisodes > 0 ? Math.round((season.watchedEpisodes / season.totalEpisodes) * 100) : 0;
            return <article className="poster-entry" key={season.id}>
              <div className="poster wide" role="img" aria-label={season.title}>
                {season.posterUrl ? <img src={season.posterUrl} alt="" loading="lazy" /> : <span>{season.title}</span>}
                <div className="poster-progress"><div className="poster-progress-fill" style={{ width: `${progress}%` }} /></div>
              </div>
              <strong>{season.title}</strong>
              <small>{season.played ? "Angesehen" : `${season.watchedEpisodes} von ${season.totalEpisodes} Folgen`}</small>
            </article>;
          })}</div>
        </section>}
        {crewAndCast.length > 0 && <section className="media-detail-cast">
          <h2>Besetzung</h2>
          <div className="cast-grid">
            {crewAndCast.slice(0, 12).map((person, index) => <div className="cast-entry" key={`${person.name}-${index}`}>
              <div className="cast-avatar">{person.imageUrl ? <img src={person.imageUrl} alt="" loading="lazy" /> : <span>{person.name.charAt(0)}</span>}</div>
              <strong>{person.name}</strong><small>{person.role}</small>
            </div>)}
          </div>
        </section>}
      </div>}
    </div>
    {detail && requestModalOpen && <div className="request-modal-backdrop" role="presentation" onClick={() => requestState !== "submitting" && setRequestModalOpen(false)}>
      <div className="request-modal" role="dialog" aria-modal="true" aria-label={`${detail.title} anfragen`} onClick={(event) => event.stopPropagation()}>
        <div className="request-modal-head">
          <div className="request-modal-poster">{detail.posterUrl && <img src={detail.posterUrl} alt="" />}</div>
          <div><p className="eyebrow">ANFRAGE</p><h3>{detail.title}</h3></div>
        </div>
        {requestableSeasons.length > 0 && <div className="season-list">
          {requestableSeasons.map((season) => <label className="season-toggle-row" key={season.seasonNumber}>
            <span>Staffel {season.seasonNumber} <small>({season.episodeCount} Folgen)</small></span>
            <span className="toggle-switch">
              <input type="checkbox" checked={selectedSeasons.includes(season.seasonNumber)} onChange={() => toggleSeason(season.seasonNumber)} />
              <span className="toggle-track"><span className="toggle-thumb" /></span>
            </span>
          </label>)}
        </div>}
        {requestState === "error" && <p className="request-error">Anfrage fehlgeschlagen. Bitte erneut versuchen.</p>}
        <div className="request-modal-actions">
          <button type="button" className="request-button secondary" disabled={requestState === "submitting"} onClick={() => setRequestModalOpen(false)}>Abbrechen</button>
          <button
            type="button"
            className="request-button"
            disabled={requestState === "submitting" || (requestableSeasons.length > 0 && selectedSeasons.length === 0)}
            onClick={submitRequest}
          >
            {requestState === "submitting" ? "Wird angefragt …" : "Jetzt anfragen"}
          </button>
        </div>
      </div>
    </div>}
  </div>;
}

function mediaStatus(detail: MediaDetail) {
  if (detail.isSeries) {
    if (!detail.totalEpisodes) return null;
    if ((detail.watchedEpisodes ?? 0) >= detail.totalEpisodes) return "Angesehen";
    return `${detail.watchedEpisodes ?? 0} von ${detail.totalEpisodes} Folgen`;
  }
  if (detail.played !== undefined) return detail.played ? "Angesehen" : "Verfügbar";
  return null;
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
    <section className="profile-head"><div className="avatar big"><UserAvatar name={user.name} userId={user.id} /></div><div><p className="eyebrow">EMBY-PROFIL</p><h2>{user.name}</h2></div></section>
    <section className="media-detail-facts profile-facts">
      <dl>
        <div><dt>Mitglied seit</dt><dd>{userProfile ? formatFullDate(userProfile.memberSince) : "—"}</dd></div>
        <div><dt>Zuletzt aktiv</dt><dd>{userProfile ? formatFullDate(userProfile.lastActiveDate) : "—"}</dd></div>
        <div><dt>Letzter Login</dt><dd>{userProfile ? formatFullDate(userProfile.lastLoginDate) : "—"}</dd></div>
        {user.features.requests && <div><dt>Anfragen insgesamt</dt><dd>{totalRequests !== null ? totalRequests : "—"}</dd></div>}
        <div><dt>Version</dt><dd>v{APP_VERSION}</dd></div>
      </dl>
    </section>
    <PosterRow title="Meine Merkliste" eyebrow="MEINE LISTEN" items={withId(watchlist)} state={watchlistState} emptyLabel="Noch nichts auf der Merkliste." detail={() => "Merkliste"} onSelect={(item) => onSelectMedia(toSelection(item))} />
    <PosterRow title="Meine Bewertungen" items={withId(ratings)} state={ratingsState} emptyLabel="Noch keine Bewertungen." detail={(item) => "★".repeat(item.rating ?? 0)} onSelect={(item) => onSelectMedia(toSelection(item))} />
    <button className="logout-button" onClick={logout} disabled={signingOut}>{signingOut ? "Abmeldung läuft …" : "Abmelden"}</button>
  </div>;
}

type EmbyLibrary = { id: string; name: string };
type ServiceView = { enabled: boolean; baseUrl?: string; apiKeySet: boolean; apiKeyPreview?: string };
type AdminSettingsView = {
  newForYouLibraryIds: string[]; watchedLibraryIds: string[];
  seerr: ServiceView; radarr: ServiceView; sonarr: ServiceView; tmdb: ServiceView;
  comingSoonRegion: string; comingSoonDaysAhead: number;
};
type ServiceDraft = { enabled: boolean; baseUrl: string; apiKey: string };
const SETUP_INTRO_SEEN_KEY = "emby-insights-setup-intro-seen";

// AdminSettings is the Verwaltung page: it replaces manually editing .env
// with a GUI for library selection and the four optional integrations.
// Nothing here is enforced client-side only — every /api/admin/* call is
// re-checked server-side (see requireAdmin in the backend).
function AdminSettings() {
  const [settings, settingsState, refetchSettings] = useApiResource<AdminSettingsView | null>("/api/admin/settings", null);
  const [libraries, librariesState] = useApiResource<EmbyLibrary[]>("/api/admin/libraries", []);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState(false);
  const [savedAt, setSavedAt] = useState<number | null>(null);
  const [showIntro, setShowIntro] = useState(() => typeof window !== "undefined" && !window.localStorage.getItem(SETUP_INTRO_SEEN_KEY));

  const [newForYouIds, setNewForYouIds] = useState<string[]>([]);
  const [watchedIds, setWatchedIds] = useState<string[]>([]);
  const [libraryPicker, setLibraryPicker] = useState<"newForYou" | "watched" | null>(null);
  const [seerr, setSeerr] = useState<ServiceDraft>({ enabled: false, baseUrl: "", apiKey: "" });
  const [radarr, setRadarr] = useState<ServiceDraft>({ enabled: false, baseUrl: "", apiKey: "" });
  const [sonarr, setSonarr] = useState<ServiceDraft>({ enabled: false, baseUrl: "", apiKey: "" });
  const [tmdb, setTmdb] = useState<ServiceDraft>({ enabled: false, baseUrl: "", apiKey: "" });

  useEffect(() => {
    if (!settings) return;
    setNewForYouIds(settings.newForYouLibraryIds);
    setWatchedIds(settings.watchedLibraryIds);
    setSeerr({ enabled: settings.seerr.enabled, baseUrl: settings.seerr.baseUrl ?? "", apiKey: "" });
    setRadarr({ enabled: settings.radarr.enabled, baseUrl: settings.radarr.baseUrl ?? "", apiKey: "" });
    setSonarr({ enabled: settings.sonarr.enabled, baseUrl: settings.sonarr.baseUrl ?? "", apiKey: "" });
    setTmdb({ enabled: settings.tmdb.enabled, baseUrl: "", apiKey: "" });
  }, [settings]);

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
          comingSoonRegion: settings?.comingSoonRegion ?? "DE",
          comingSoonDaysAhead: settings?.comingSoonDaysAhead ?? 28,
        }),
      });
      if (!response.ok) throw new Error("saving settings failed");
      setSavedAt(Date.now());
      refetchSettings();
    } catch {
      setSaveError(true);
    } finally {
      setSaving(false);
    }
  };

  if (settingsState === "loading") return <div className="content page-view admin-page"><p className="poster-status" role="status">Wird geladen …</p></div>;
  if (settingsState === "error" || !settings) return <div className="content page-view admin-page"><p className="poster-status">Einstellungen nicht verfügbar.</p></div>;

  return <div className="content page-view admin-page">
    {showIntro && <section className="admin-intro-banner">
      <p className="eyebrow">WILLKOMMEN</p>
      <h2>Du bist jetzt Admin von Emby Insights</h2>
      <p>Richte hier ein, welche Bibliotheken genutzt werden und welche optionalen Dienste (Seerr, Radarr, Sonarr, TMDB) angebunden sind. Alles ist optional — ohne Einrichtung bleiben die zugehörigen Bereiche einfach ausgeblendet.</p>
      <button type="button" className="request-button" onClick={dismissIntro}>Verstanden</button>
    </section>}

    <section className="admin-section" aria-label="Bibliotheken">
      <div className="section-heading"><div><p className="eyebrow">BIBLIOTHEKEN</p><h2>Welche Bibliotheken sollen genutzt werden?</h2></div></div>
      {librariesState === "loading" && <p className="poster-status" role="status">Bibliotheken werden geladen …</p>}
      {librariesState === "error" && <p className="poster-status">Emby-Bibliotheken nicht verfügbar.</p>}
      {librariesState === "ready" && libraries.length === 0 && <p className="poster-status">Keine Bibliotheken gefunden.</p>}
      {libraries.length > 0 && <div className="admin-service-grid">
        <LibraryTile
          title="Neu für dich" description="Bibliotheken, aus denen kürzlich hinzugefügte, ungesehene Titel vorgeschlagen werden."
          selectedCount={newForYouIds.length} onOpen={() => setLibraryPicker("newForYou")}
        />
        <LibraryTile
          title="Gesehene Filme und Serien" description="Bibliotheken, aus denen die Statistik- und Verlaufslisten gespeist werden."
          selectedCount={watchedIds.length} onOpen={() => setLibraryPicker("watched")}
        />
      </div>}
    </section>

    {libraryPicker && <LibraryPickerModal
      title={libraryPicker === "newForYou" ? "Neu für dich" : "Gesehene Filme und Serien"}
      libraries={libraries}
      selectedIds={libraryPicker === "newForYou" ? newForYouIds : watchedIds}
      onToggle={(id) => libraryPicker === "newForYou" ? toggleLibrary(newForYouIds, setNewForYouIds, id) : toggleLibrary(watchedIds, setWatchedIds, id)}
      onClose={() => setLibraryPicker(null)}
    />}

    <section className="admin-section" aria-label="Optionale Dienste">
      <div className="section-heading"><div><p className="eyebrow">OPTIONALE DIENSTE</p><h2>Verbindungen</h2></div></div>
      <div className="admin-service-grid">
        <ServiceCard
          title="Seerr" description="Für Medienanfragen, Suche, Trends und Streaming-Anbieter."
          shows="Schaltet die Seite „Anfragen“ frei, inklusive Suche, Trends, Streaming-Anbieter und die Anfrage-Zahl im Profil."
          draft={seerr} onChange={setSeerr} existing={settings.seerr} showsBaseUrl
        />
        <ServiceCard
          title="Radarr" description="Für Filmtermine unter „Demnächst“ und „Im Kino“."
          shows="Schaltet Filmtermine unter „Demnächst“ und die Reihe „Im Kino“ frei."
          draft={radarr} onChange={setRadarr} existing={settings.radarr} showsBaseUrl
        />
        <ServiceCard
          title="Sonarr" description="Für Serien- und Folgentermine unter „Demnächst“."
          shows="Schaltet Serien- und Folgentermine unter „Demnächst“ frei."
          draft={sonarr} onChange={setSonarr} existing={settings.sonarr} showsBaseUrl
        />
        <ServiceCard
          title="TMDB" description="Für genauere regionale Filmtermine."
          shows="Ohne TMDB bleiben Termine aus Radarr/Sonarr nutzbar, nur etwas ungenauer."
          draft={tmdb} onChange={setTmdb} existing={settings.tmdb} showsBaseUrl={false}
        />
      </div>
    </section>

    <div className="admin-save-bar">
      {saveError && <p className="request-error">Speichern fehlgeschlagen. Bitte erneut versuchen.</p>}
      {savedAt !== null && !saveError && <p className="request-confirmation">Gespeichert ✓</p>}
      <button type="button" className="request-button" disabled={saving} onClick={save}>{saving ? "Wird gespeichert …" : "Einstellungen speichern"}</button>
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
  const [expanded, setExpanded] = useState(false);
  return <article className={expanded ? "admin-service-card expanded" : "admin-service-card"}>
    <button type="button" className="admin-service-head" onClick={() => setExpanded((value) => !value)} aria-expanded={expanded}>
      <div><strong>{title}</strong><p className="admin-hint">{description}</p></div>
      <span className="admin-service-head-controls">
        <label className="toggle-switch" onClick={(event) => event.stopPropagation()}>
          <input type="checkbox" checked={draft.enabled} onChange={(event) => onChange({ ...draft, enabled: event.target.checked })} aria-label={`${title} aktivieren`} />
          <span className="toggle-track"><span className="toggle-thumb" /></span>
        </label>
        <span className="admin-service-chevron"><Icon name="arrow" /></span>
      </span>
    </button>
    {expanded && <div className="admin-service-body">
      {showsBaseUrl && <label className="admin-field">
        <span>Server-Adresse</span>
        <input type="text" className="search-input" placeholder="https://…" value={draft.baseUrl} onChange={(event) => onChange({ ...draft, baseUrl: event.target.value })} />
      </label>}
      <label className="admin-field">
        <span>API-Schlüssel{existing.apiKeySet ? ` (aktuell ${existing.apiKeyPreview})` : ""}</span>
        <input type="password" className="search-input" placeholder={existing.apiKeySet ? "Neuen Schlüssel eingeben, um ihn zu ersetzen" : "API-Schlüssel eingeben"} value={draft.apiKey} onChange={(event) => onChange({ ...draft, apiKey: event.target.value })} autoComplete="off" />
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
  return <button type="button" className="admin-service-card admin-library-tile" onClick={onOpen}>
    <div className="admin-service-head">
      <div><strong>{title}</strong><p className="admin-hint">{description}</p></div>
      <Icon name="arrow" />
    </div>
    <p className="admin-library-tile-count">{selectedCount === 0 ? "Keine Bibliothek ausgewählt" : `${selectedCount} ${selectedCount === 1 ? "Bibliothek" : "Bibliotheken"} ausgewählt`}</p>
  </button>;
}

function LibraryPickerModal({ title, libraries, selectedIds, onToggle, onClose }: {
  title: string; libraries: EmbyLibrary[]; selectedIds: string[]; onToggle: (id: string) => void; onClose: () => void;
}) {
  useEscapeKey(onClose);
  return <div className="request-modal-backdrop" role="presentation" onClick={onClose}>
    <div className="request-modal" role="dialog" aria-modal="true" aria-label={title} onClick={(event) => event.stopPropagation()}>
      <div><p className="eyebrow">BIBLIOTHEKEN</p><h3>{title}</h3></div>
      {libraries.length === 0
        ? <p className="poster-status">Keine Bibliotheken gefunden.</p>
        : <div className="season-list">{libraries.map((library) => <label className="season-toggle-row" key={library.id}>
          <span>{library.name}</span>
          <span className="toggle-switch">
            <input type="checkbox" checked={selectedIds.includes(library.id)} onChange={() => onToggle(library.id)} />
            <span className="toggle-track"><span className="toggle-thumb" /></span>
          </span>
        </label>)}</div>}
      <div className="request-modal-actions">
        <button type="button" className="request-button" onClick={onClose}>Fertig</button>
      </div>
    </div>
  </div>;
}

const chatTimeFormatter = new Intl.DateTimeFormat("de-DE", { hour: "2-digit", minute: "2-digit" });
function formatChatTime(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "" : chatTimeFormatter.format(date);
}

function ChatMessageList({ messages, mineWhenFromAdmin, mineName, mineAvatarSrc, theirsName, theirsAvatarSrc }: {
  messages: ChatMessage[]; mineWhenFromAdmin: boolean;
  mineName: string; mineAvatarSrc: string; theirsName: string; theirsAvatarSrc: string;
}) {
  const listRef = useRef<HTMLDivElement>(null);
  useEffect(() => { listRef.current?.scrollTo({ top: listRef.current.scrollHeight }); }, [messages]);
  return <div className="chat-messages" ref={listRef}>
    {messages.map((message) => {
      const isMine = message.fromAdmin === mineWhenFromAdmin;
      return <div key={message.id} className={isMine ? "chat-bubble mine" : "chat-bubble theirs"}>
        <span className="chat-bubble-avatar"><PersonAvatar name={isMine ? mineName : theirsName} src={isMine ? mineAvatarSrc : theirsAvatarSrc} /></span>
        <div className="chat-bubble-body">
          <p>{message.body}</p>
          <small>{formatChatTime(message.createdAt)}</small>
        </div>
      </div>;
    })}
  </div>;
}

function ChatComposer({ placeholder, onSend }: { placeholder: string; onSend: (body: string) => Promise<boolean> }) {
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
    <button type="submit" className="search-button" disabled={body.trim() === "" || sending}>Senden</button>
  </form>;
}

function Chats({ user }: { user: { id: string; name: string; isAdmin: boolean } }) {
  return user.isAdmin ? <AdminChats adminName={user.name} adminUserId={user.id} /> : <UserChat userName={user.name} userId={user.id} />;
}

function UserChat({ userName, userId }: { userName: string; userId: string }) {
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
    <section className="chat-thread" aria-label="Chat mit dem Betreiber">
      {state === "loading" && <p className="poster-status" role="status">Wird geladen …</p>}
      {state === "error" && <p className="poster-status">Nicht verfügbar</p>}
      {state === "ready" && messages.length === 0 && <p className="chat-empty">Schreib mir gern, wenn du eine Frage oder einen Wunsch hast.</p>}
      <ChatMessageList messages={messages} mineWhenFromAdmin={false} mineName={userName} mineAvatarSrc={`/api/me/avatar?u=${encodeURIComponent(userId)}`} theirsName="Admin" theirsAvatarSrc="/api/messages/admin-avatar" />
      <ChatComposer placeholder="Nachricht schreiben …" onSend={send} />
    </section>
  </div>;
}

function AdminChats({ adminName, adminUserId }: { adminName: string; adminUserId: string }) {
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
      <div><p className="eyebrow">NACHRICHTEN</p><h2>Posteingang</h2></div>
      <div className="chat-inbox-actions">
        <button type="button" className="chat-action-button" onClick={() => setBroadcastOpen(true)}><Icon name="bell" /> Rundmail</button>
        <button type="button" className="chat-action-button" onClick={() => setPickerOpen(true)}><Icon name="arrow" /> Admin schreiben</button>
      </div>
    </div>
    <section className="chat-inbox" aria-label="Nachrichten-Posteingang">
      {threadsState === "loading" && <p className="poster-status" role="status">Wird geladen …</p>}
      {threadsState === "error" && <p className="poster-status">Nicht verfügbar</p>}
      {threadsState === "ready" && threads.length === 0 && <p className="chat-empty">Noch keine Nachrichten von Nutzern.</p>}
      <ul className="chat-thread-list">
        {threads.map((thread) => <li key={thread.userId}>
          <button type="button" className="chat-thread-row" onClick={() => setSelectedUserId(thread.userId)}>
            <span className="chat-avatar"><PersonAvatar name={thread.displayName || "?"} src={`/api/admin/users/avatar?userId=${encodeURIComponent(thread.userId)}`} /></span>
            <span className="chat-thread-name">{thread.displayName || "Unbekannt"}</span>
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
    <div className="request-modal" role="dialog" aria-modal="true" aria-label="Rundmail senden" onClick={(event) => event.stopPropagation()}>
      <div><p className="eyebrow">RUNDMAIL</p><h3>An alle Nutzer senden</h3></div>
      {sentCount !== null
        ? <>
          <p className="request-confirmation">An {sentCount} {sentCount === 1 ? "Nutzer" : "Nutzer"} gesendet ✓</p>
          <div className="request-modal-actions"><button type="button" className="request-button" onClick={onSent}>Fertig</button></div>
        </>
        : <form onSubmit={send}>
          <textarea className="broadcast-textarea" placeholder="Nachricht an alle Nutzer, z. B. eine Wartungsankündigung …" value={body} onChange={(event) => setBody(event.target.value)} maxLength={4000} rows={5} aria-label="Rundmail-Text" />
          {error && <p className="request-error">Senden fehlgeschlagen. Bitte erneut versuchen.</p>}
          <div className="request-modal-actions">
            <button type="button" className="request-button secondary" disabled={sending} onClick={onClose}>Abbrechen</button>
            <button type="submit" className="request-button" disabled={body.trim() === "" || sending}>{sending ? "Wird gesendet …" : "An alle senden"}</button>
          </div>
        </form>}
    </div>
  </div>;
}

function ContactPickerScreen({ contacts, onSelect, onClose }: { contacts: Contact[]; onSelect: (contact: Contact) => void; onClose: () => void }) {
  useEscapeKey(onClose);
  return <div className="media-detail-overlay media-grid-overlay" role="dialog" aria-modal="true" aria-label="Neuen Chat starten">
    <div className="media-detail-scroll">
      <button type="button" className="media-detail-close" onClick={onClose} aria-label="Schließen"><Icon name="close" /></button>
      <h1 className="media-grid-title">Neuen Chat starten</h1>
      {contacts.length === 0
        ? <p className="chat-empty">Alle Nutzer haben bereits einen Thread.</p>
        : <ul className="chat-thread-list">{contacts.map((contact) => <li key={contact.id}>
          <button type="button" className="chat-thread-row" onClick={() => onSelect(contact)}>
            <span className="chat-avatar"><PersonAvatar name={contact.name || "?"} src={`/api/admin/users/avatar?userId=${encodeURIComponent(contact.id)}`} /></span>
            <span className="chat-thread-name">{contact.name || "Unbekannt"}</span>
          </button>
        </li>)}</ul>}
    </div>
  </div>;
}

function AdminChatThreadScreen({ thread, adminName, adminUserId, onClose }: { thread: ChatThread; adminName: string; adminUserId: string; onClose: () => void }) {
  useEscapeKey(onClose);
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

  return <div className="media-detail-overlay media-grid-overlay" role="dialog" aria-modal="true" aria-label={`Chat mit ${thread.displayName}`}>
    <div className="media-detail-scroll chat-overlay-scroll">
      <button type="button" className="media-detail-close" onClick={onClose} aria-label="Schließen"><Icon name="close" /></button>
      <div className="chat-thread-header">
        <h1 className="media-grid-title">{thread.displayName || "Unbekannt"}</h1>
        <button type="button" className="chat-delete-button" onClick={() => setConfirmDelete(true)} aria-label="Chat löschen"><Icon name="close" /></button>
      </div>
      <section className="chat-thread" aria-label={`Chat mit ${thread.displayName || "Unbekannt"}`}>
        {state === "loading" && <p className="poster-status" role="status">Wird geladen …</p>}
        {state === "error" && <p className="poster-status">Nicht verfügbar</p>}
        <ChatMessageList messages={messages} mineWhenFromAdmin={true} mineName={adminName} mineAvatarSrc={`/api/me/avatar?u=${encodeURIComponent(adminUserId)}`} theirsName={thread.displayName || "Unbekannt"} theirsAvatarSrc={`/api/admin/users/avatar?userId=${encodeURIComponent(thread.userId)}`} />
        <ChatComposer placeholder="Antwort schreiben …" onSend={send} />
      </section>
    </div>
    {confirmDelete && <div className="request-modal-backdrop" role="presentation" onClick={() => !deleting && setConfirmDelete(false)}>
      <div className="request-modal" role="dialog" aria-modal="true" aria-label="Chat löschen" onClick={(event) => event.stopPropagation()}>
        <div><p className="eyebrow">LÖSCHEN</p><h3>Chat mit {thread.displayName || "diesem Nutzer"} löschen?</h3></div>
        <p className="request-error">Das entfernt den kompletten Verlauf unwiderruflich.</p>
        <div className="request-modal-actions">
          <button type="button" className="request-button secondary" disabled={deleting} onClick={() => setConfirmDelete(false)}>Abbrechen</button>
          <button type="button" className="request-button" disabled={deleting} onClick={deleteThread}>{deleting ? "Wird gelöscht …" : "Endgültig löschen"}</button>
        </div>
      </div>
    </div>}
  </div>;
}

function greeting() {
  const hour = new Date().getHours();
  return hour < 12 ? "Moin" : hour < 18 ? "Mahlzeit" : "Nabend";
}

// A plain reload() can still be served from Safari's cache when the app is
// added to the iPad home screen (no address bar / pull-to-refresh there to
// force a fresh fetch). The cache-busting query string makes this an
// unmatched URL, so the browser has to hit the network — and landing back on
// "/" also resets the in-memory page state to "Heute" (its initial value).
function goHomeAndRefresh() {
  window.location.href = `${window.location.pathname}?refresh=${Date.now()}`;
}
function loadingCopy(state: LoadState) { return state === "error" ? "Nicht verfügbar" : "Wird geladen …"; }
function formatDuration(seconds: number) { const hours = Math.floor(seconds / 3600); const minutes = Math.floor((seconds % 3600) / 60); return hours > 0 ? `${hours}\u00a0Std. ${minutes}\u00a0Min.` : `${minutes}\u00a0Min.`; }
function comparisonText(statistics: PersonalStats) { if (statistics.previousWatchSeconds === 0) return "Keine Vergleichsdaten"; const change = Math.round(((statistics.watchSeconds - statistics.previousWatchSeconds) / statistics.previousWatchSeconds) * 100); return `${change >= 0 ? "Mehr" : "Weniger"} als im vorherigen Zeitraum: ${new Intl.NumberFormat("de-DE").format(Math.abs(change))}\u00a0%`; }
