"use client";

import { type ReactNode, useEffect, useRef, useState } from "react";
import { LoginScreen } from "./login-screen";

type Page = "Heute" | "Statistik" | "Anfragen" | "Profil";
type Period = "Woche" | "Monat" | "Jahr";
type StatisticsPeriod = "week" | "month" | "year";
type PersonalStats = { watchSeconds: number; previousWatchSeconds: number; completedMovies: number; completedSeries: number; favouriteGenre: string; periodStartsAt: string; periodEndsAt: string };
type UpcomingItem = { id: string; title: string; posterUrl: string; premiereDate: string };
type RequestItem = { id: string; title: string; posterUrl: string; status: string; tmdbId: string; mediaType: string };
type NewForYouItem = { id: string; title: string; posterUrl: string };
type ContinueWatchingItem = { id: string; title: string; posterUrl: string; progressPercent: number };
type WatchedItem = { id: string; title: string; posterUrl: string; genres: string[]; lastPlayedDate: string };
type DiscoverItem = { id: string; title: string; posterUrl: string; mediaType: string };
type MediaSelection = { source: "emby"; id: string } | { source: "seerr"; id: string; mediaType: string };
type MediaPerson = { name: string; role: string; imageUrl: string };
type MediaSeason = { id: string; title: string; posterUrl: string; indexNumber: number; watchedEpisodes: number; totalEpisodes: number; played: boolean };
type RequestableSeason = { seasonNumber: number; episodeCount: number };
type MediaDetail = {
  id: string; title: string; overview: string; posterUrl: string; backdropUrl: string;
  genres: string[]; communityRating: number; officialRating?: string; year: number; runtimeMinutes: number;
  cast: MediaPerson[]; crew: MediaPerson[];
  isSeries?: boolean; watchedEpisodes?: number; totalEpisodes?: number; played?: boolean;
  seasons?: MediaSeason[] | RequestableSeason[];
  mediaStatus?: number;
  status?: string; releaseDate?: string; studios?: string[];
};
function isRequestableSeason(season: MediaSeason | RequestableSeason): season is RequestableSeason { return !("id" in season); }
type IconName = "home" | "chart" | "sparkle" | "user" | "bell" | "arrow" | "close" | "clock" | "movie" | "series" | "genre";
type Tone = "blue" | "peach" | "mint" | "lilac";
type LoadState = "loading" | "ready" | "error";

const nav: { label: Page; icon: IconName }[] = [
  { label: "Heute", icon: "home" }, { label: "Statistik", icon: "chart" },
  { label: "Anfragen", icon: "sparkle" }, { label: "Profil", icon: "user" },
];
const apiPeriod: Record<Period, StatisticsPeriod> = { Woche: "week", Monat: "month", Jahr: "year" };
const APP_VERSION = "0.8.13";

const dateFormatter = new Intl.DateTimeFormat("de-DE", { day: "2-digit", month: "short" });
function formatPremiereDate(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "" : dateFormatter.format(date);
}
const fullDateFormatter = new Intl.DateTimeFormat("de-DE", { day: "2-digit", month: "long", year: "numeric" });
function formatFullDate(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "" : fullDateFormatter.format(date);
}

export default function Home() {
  const [page, setPage] = useState<Page>("Heute");
  const [noticeOpen, setNoticeOpen] = useState(false);
  const [unread, setUnread] = useState(2);
  const [user, setUser] = useState<{ id: string; name: string } | null>(null);
  const [checkingSession, setCheckingSession] = useState(true);
  const [upcomingItems, setUpcomingItems] = useState<UpcomingItem[]>([]);
  const [upcomingState, setUpcomingState] = useState<LoadState>("loading");
  const [requestItems, setRequestItems] = useState<RequestItem[]>([]);
  const [requestState, setRequestState] = useState<LoadState>("loading");
  const [newForYouItems, setNewForYouItems] = useState<NewForYouItem[]>([]);
  const [newForYouState, setNewForYouState] = useState<LoadState>("loading");
  const [selectedMedia, setSelectedMedia] = useState<MediaSelection | null>(null);
  const noticeRef = useRef<HTMLDivElement>(null);
  const noticeButtonRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    fetch("/api/me", { credentials: "include" })
      .then(async (response) => response.ok ? setUser(await response.json()) : null)
      .catch(() => null)
      .finally(() => setCheckingSession(false));
  }, []);

  useEffect(() => {
    if (!user) return;
    let active = true;
    fetch("/api/upcoming", { credentials: "include" })
      .then(async (response) => {
        if (!response.ok) throw new Error("upcoming unavailable");
        const data = await response.json();
        if (active) { setUpcomingItems(data); setUpcomingState("ready"); }
      })
      .catch(() => active && setUpcomingState("error"));
    return () => { active = false; };
  }, [user]);

  const refetchRequests = () => {
    if (!user) return;
    fetch("/api/requests", { credentials: "include" })
      .then(async (response) => {
        if (!response.ok) throw new Error("requests unavailable");
        const data = await response.json();
        setRequestItems(data);
        setRequestState("ready");
      })
      .catch(() => setRequestState("error"));
  };

  useEffect(() => {
    if (!user) return;
    let active = true;
    fetch("/api/requests", { credentials: "include" })
      .then(async (response) => {
        if (!response.ok) throw new Error("requests unavailable");
        const data = await response.json();
        if (active) { setRequestItems(data); setRequestState("ready"); }
      })
      .catch(() => active && setRequestState("error"));
    return () => { active = false; };
  }, [user]);

  useEffect(() => {
    if (!user) return;
    let active = true;
    fetch("/api/new-for-you", { credentials: "include" })
      .then(async (response) => {
        if (!response.ok) throw new Error("new-for-you unavailable");
        const data = await response.json();
        if (active) { setNewForYouItems(data); setNewForYouState("ready"); }
      })
      .catch(() => active && setNewForYouState("error"));
    return () => { active = false; };
  }, [user]);

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
  const openNotices = () => { setNoticeOpen((open) => !open); setUnread(0); };

  return <main className="app-shell">
    {selectedMedia && <MediaDetailScreen selection={selectedMedia} onClose={() => setSelectedMedia(null)} onRequestCreated={refetchRequests} />}
    <a className="skip-link" href="#dashboard-content">Zum Inhalt springen</a>
    <aside className="side-nav" aria-label="Hauptnavigation">
      <div className="brand"><img className="brand-logo" src="/emby-insights-logo.svg" alt="Emby Insights" width="31" height="31" /><span>insights</span></div>
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
          <button ref={noticeButtonRef} className="notice-button" aria-label="Benachrichtigungen" aria-expanded={noticeOpen} aria-controls="notifications" onClick={openNotices}><Icon name="bell" />{unread > 0 && <b><span className="sr-only">{unread} ungelesene Benachrichtigungen</span></b>}</button>
          <button className="avatar" aria-label="Profil öffnen" onClick={() => selectPage("Profil")}><UserAvatar name={user.name} /></button>
          {noticeOpen && <div ref={noticeRef} className="notifications" id="notifications" role="dialog" aria-label="Benachrichtigungen"><strong>Benachrichtigungen</strong><p>Deine Anfrage „Severance“ wird bearbeitet.</p><p>Am Freitag erscheint Alien: Earth.</p></div>}
        </div>
      </header>
      {page === "Heute" && <Today user={user} onStats={() => selectPage("Statistik")} upcoming={upcomingItems} upcomingState={upcomingState} requests={requestItems} requestState={requestState} newForYou={newForYouItems} newForYouState={newForYouState} onSelectMedia={setSelectedMedia} />}
      {page === "Statistik" && <Stats onSelectMedia={setSelectedMedia} />}
      {page === "Anfragen" && <Requests items={requestItems} state={requestState} onSelectMedia={setSelectedMedia} />}
      {page === "Profil" && <Profile user={user} />}
    </section>

    <nav className="bottom-nav" aria-label="Hauptnavigation (mobil)">{nav.map((item) => <button key={item.label} className={page === item.label ? "active" : ""} onClick={() => selectPage(item.label)} aria-current={page === item.label ? "page" : undefined}><Icon name={item.icon} /><span className="sr-only">{item.label}</span></button>)}</nav>
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
  };
  return <svg className="icon" viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">{paths[name]}</svg>;
}

function UserAvatar({ name }: { name: string }) {
  const initial = name.trim().charAt(0).toUpperCase() || "?";
  return <span className="user-avatar"><span className="avatar-initial">{initial}</span><img src="/api/me/avatar" alt="" width="44" height="44" onError={(event) => event.currentTarget.remove()} /></span>;
}

function Today({ user, onStats, upcoming, upcomingState, requests, requestState, newForYou, newForYouState, onSelectMedia }: {
  user: { name: string }; onStats: () => void;
  upcoming: UpcomingItem[]; upcomingState: LoadState; requests: RequestItem[]; requestState: LoadState;
  newForYou: NewForYouItem[]; newForYouState: LoadState;
  onSelectMedia: (selection: MediaSelection) => void;
}) {
  return <div className="content today-view">
    <section className="section-heading rhythm-heading"><div><p className="eyebrow">DEIN PROFIL</p><h2>Dein Rhythmus</h2></div><button className="text-button" onClick={onStats}>Alle Details <Icon name="arrow" /></button></section>
    <UserInsightCard user={user} upcoming={upcoming} upcomingState={upcomingState} requests={requests} requestState={requestState} newForYou={newForYou} newForYouState={newForYouState} />
    <PosterRow title="Demnächst" eyebrow="COMING SOON · NÄCHSTE 4 WOCHEN" items={upcoming} state={upcomingState} emptyLabel="Nichts Neues in den nächsten vier Wochen." detail={(item) => formatPremiereDate(item.premiereDate)} onSelect={(item) => onSelectMedia({ source: "emby", id: item.id })} />
    <PosterRow title="Meine Anfragen" eyebrow="SEERR · OFFEN" items={requests} state={requestState} emptyLabel="Keine offenen Anfragen." detail={(item) => item.status} onSelect={(item) => onSelectMedia({ source: "seerr", id: item.tmdbId, mediaType: item.mediaType })} />
    <PosterRow title="Neu für dich" eyebrow="IN DEN LETZTEN 14 TAGEN" items={newForYou} state={newForYouState} emptyLabel="Nichts Neues in den letzten 14 Tagen." detail={() => "Ungesehen"} onSelect={(item) => onSelectMedia({ source: "emby", id: item.id })} />
  </div>;
}

function UserInsightCard({ user, upcoming, upcomingState, requests, requestState, newForYou, newForYouState }: {
  user: { name: string };
  upcoming: UpcomingItem[]; upcomingState: LoadState; requests: RequestItem[]; requestState: LoadState;
  newForYou: NewForYouItem[]; newForYouState: LoadState;
}) {
  return <section className="user-insight-card" aria-label={`Kurzüberblick von ${user.name}`}>
    <HighlightCarousel user={user} upcoming={upcoming} upcomingState={upcomingState} requests={requests} requestState={requestState} newForYou={newForYou} newForYouState={newForYouState} />
  </section>;
}

const SLIDE_INTERVAL = 5000;

function HighlightCarousel({ user, upcoming, upcomingState, requests, requestState, newForYou, newForYouState }: {
  user: { name: string };
  upcoming: UpcomingItem[]; upcomingState: LoadState; requests: RequestItem[]; requestState: LoadState;
  newForYou: NewForYouItem[]; newForYouState: LoadState;
}) {
  const nextRelease = upcoming[0];
  const slides: { key: string; icon: IconName; tone: Tone; label: string; value: string; detail: string; text?: boolean }[] = [
    {
      key: "upcoming", icon: "clock", tone: "blue", label: "Nächste Veröffentlichung",
      value: upcomingState === "ready" ? (nextRelease?.title ?? "—") : "—",
      detail: upcomingState === "ready" ? (nextRelease ? formatPremiereDate(nextRelease.premiereDate) : "Nichts geplant") : loadingCopy(upcomingState),
      text: true,
    },
    {
      key: "requests", icon: "sparkle", tone: "peach", label: "Offene Anfragen",
      value: requestState === "ready" ? String(requests.length) : "—",
      detail: requestState === "ready" ? "Bei Seerr" : loadingCopy(requestState),
    },
    {
      key: "new", icon: "genre", tone: "mint", label: "Neu für dich",
      value: newForYouState === "ready" ? String(newForYou.length) : "—",
      detail: newForYouState === "ready" ? "Letzte 14 Tage" : loadingCopy(newForYouState),
    },
  ];

  const scroller = useRef<HTMLDivElement>(null);
  const [active, setActive] = useState(0);
  const [paused, setPaused] = useState(false);

  const goTo = (index: number) => scroller.current?.scrollTo({ left: scroller.current.clientWidth * index, behavior: "smooth" });

  useEffect(() => {
    if (paused || window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
    const timer = setInterval(() => {
      const element = scroller.current;
      if (!element) return;
      const next = (Math.round(element.scrollLeft / element.clientWidth) + 1) % slides.length;
      element.scrollTo({ left: element.clientWidth * next, behavior: "smooth" });
    }, SLIDE_INTERVAL);
    return () => clearInterval(timer);
  }, [paused, slides.length]);

  // Only genuine input pauses the rotation — listening to "scroll" would also
  // catch our own programmatic advance and stop it immediately.
  const stop = () => setPaused(true);

  return <div className="weekly-carousel">
    <div
      ref={scroller}
      className="weekly-scroller"
      role="group"
      aria-roledescription="Karussell"
      aria-label="Deine Kurzübersicht"
      tabIndex={0}
      onScroll={(event) => setActive(Math.round(event.currentTarget.scrollLeft / event.currentTarget.clientWidth))}
      onPointerDown={stop}
      onWheel={stop}
      onKeyDown={stop}
    >
      {slides.map((slide, index) => <div
        key={slide.key}
        className="weekly-slide"
        role="group"
        aria-roledescription="Folie"
        aria-label={`${index + 1} von ${slides.length}: ${slide.label}`}
      >
        <div className="user-insight-identity">
          <div className="profile-avatar"><UserAvatar name={user.name} /></div>
          <div><p className="eyebrow">DEIN MEDIENPROFIL</p><h3>{user.name}</h3><p>Dein persönlicher Überblick auf einen Blick.</p></div>
        </div>
        <div className={`weekly-stat tone-${slide.tone}${slide.text ? " weekly-slide-text" : ""}`}>
          <span className="user-stat-icon"><Icon name={slide.icon} /></span>
          <span className="weekly-label">{slide.label}</span>
          <strong>{slide.value}</strong>
          <small>{slide.detail}</small>
        </div>
      </div>)}
    </div>
    <div className="weekly-dots">
      {slides.map((slide, index) => <button
        key={slide.key}
        className={index === active ? "weekly-dot active" : "weekly-dot"}
        aria-label={`${slide.label} anzeigen`}
        aria-current={index === active ? "true" : undefined}
        onClick={() => { stop(); goTo(index); }}
      ><i /></button>)}
    </div>
  </div>;
}

function MetricCard({ icon, tone, value, label, detail, positive, genre = false, onClick }: { icon: IconName; tone: Tone; value: string | number; label: string; detail: string; positive?: boolean; genre?: boolean; onClick?: () => void }) {
  const inner = <><span className="metric-icon"><Icon name={icon} /></span><strong>{value}</strong><p>{label}</p><small className={positive ? "up" : undefined}>{detail}</small></>;
  return onClick
    ? <button type="button" className={`metric-card metric-card-button tone-${tone}${genre ? " genre-card" : ""}`} onClick={onClick}>{inner}</button>
    : <article className={`metric-card tone-${tone}${genre ? " genre-card" : ""}`}>{inner}</article>;
}

function PosterRow<T extends { id: string; title: string; posterUrl?: string }>({ title, eyebrow, gridTitle, items, detail, state, emptyLabel, progress, onSelect }: {
  title?: string; eyebrow?: string; gridTitle?: string; items: readonly T[]; detail: (item: T) => string; state?: LoadState; emptyLabel?: string; progress?: (item: T) => number; onSelect?: (item: T) => void;
}) {
  const [gridOpen, setGridOpen] = useState(false);
  const resolvedGridTitle = gridTitle ?? title ?? "Übersicht";
  return <section className="poster-section">
    {(title || eyebrow) && <div className="section-heading">
      <div>{eyebrow && <p className="eyebrow">{eyebrow}</p>}{title && <h2>{title}</h2>}</div>
      {items.length > 0 && <button type="button" className="text-button poster-view-all" onClick={() => setGridOpen(true)} aria-label={`Alle ${resolvedGridTitle} anzeigen`}><Icon name="arrow" /></button>}
    </div>}
    {state === "loading" && <p className="poster-status" role="status">Wird geladen …</p>}
    {state === "error" && <p className="poster-status">Nicht verfügbar</p>}
    {state !== "loading" && state !== "error" && items.length === 0 && <p className="poster-status">{emptyLabel ?? "Nichts vorhanden."}</p>}
    {items.length > 0 && <div className="poster-scroller">{items.map((item) => {
      const inner = <>
        <div className="poster wide" role="img" aria-label={item.title}>
          {item.posterUrl ? <img src={item.posterUrl} alt="" loading="lazy" /> : <span>{item.title}</span>}
          {progress && <div className="poster-progress"><div className="poster-progress-fill" style={{ width: `${progress(item)}%` }} /></div>}
        </div>
        <strong>{item.title}</strong><small>{detail(item)}</small>
      </>;
      return onSelect
        ? <button type="button" className="poster-entry poster-entry-button" key={item.id} onClick={() => onSelect(item)}>{inner}</button>
        : <article className="poster-entry" key={item.id}>{inner}</article>;
    })}</div>}
    {gridOpen && <MediaGridScreen title={resolvedGridTitle} items={items} detail={detail} onSelect={onSelect} onClose={() => setGridOpen(false)} />}
  </section>;
}

function topGenres(movies: readonly WatchedItem[], series: readonly WatchedItem[]) {
  const counts = new Map<string, number>();
  for (const item of [...movies, ...series]) {
    for (const genre of item.genres) counts.set(genre, (counts.get(genre) ?? 0) + 1);
  }
  return [...counts.entries()].sort((a, b) => b[1] - a[1]).slice(0, 6).map(([label, value]) => ({ label, value }));
}

const WEEKDAYS = ["Mo", "Di", "Mi", "Do", "Fr", "Sa", "So"];

function weekdayActivity(movies: readonly WatchedItem[], series: readonly WatchedItem[]) {
  const counts = new Array(7).fill(0);
  for (const item of [...movies, ...series]) {
    const date = new Date(item.lastPlayedDate);
    if (Number.isNaN(date.getTime())) continue;
    counts[(date.getDay() + 6) % 7] += 1;
  }
  return WEEKDAYS.map((label, index) => ({ label, value: counts[index] }));
}

function BarChart({ title, data }: { title: string; data: { label: string; value: number }[] }) {
  const max = Math.max(1, ...data.map((entry) => entry.value));
  const hasData = data.some((entry) => entry.value > 0);
  return <section className="chart-card">
    <h3>{title}</h3>
    {hasData
      ? <div className="bar-chart" role="img" aria-label={title}>
        {data.map((entry) => <div className="bar-row" key={entry.label}>
          <span className="bar-label">{entry.label}</span>
          <div className="bar-track"><div className="bar-fill" style={{ width: `${(entry.value / max) * 100}%` }} /></div>
          <span className="bar-value">{entry.value}</span>
        </div>)}
      </div>
      : <p className="poster-status">Noch keine Daten für diesen Zeitraum.</p>}
  </section>;
}

function Stats({ onSelectMedia }: { onSelectMedia: (selection: MediaSelection) => void }) {
  const [period, setPeriod] = useState<Period>("Woche");
  const [statistics, setStatistics] = useState<PersonalStats | null>(null);
  const [state, setState] = useState<LoadState>("loading");
  const [continueWatching, setContinueWatching] = useState<ContinueWatchingItem[]>([]);
  const [continueWatchingState, setContinueWatchingState] = useState<LoadState>("loading");
  const [watchedMovies, setWatchedMovies] = useState<WatchedItem[]>([]);
  const [watchedMoviesState, setWatchedMoviesState] = useState<LoadState>("loading");
  const [watchedSeries, setWatchedSeries] = useState<WatchedItem[]>([]);
  const [watchedSeriesState, setWatchedSeriesState] = useState<LoadState>("loading");
  const [completedMovies, setCompletedMovies] = useState<WatchedItem[]>([]);
  const [completedMoviesState, setCompletedMoviesState] = useState<LoadState>("loading");
  const [completedSeries, setCompletedSeries] = useState<WatchedItem[]>([]);
  const [completedSeriesState, setCompletedSeriesState] = useState<LoadState>("loading");
  const [completedGridView, setCompletedGridView] = useState<"movies" | "series" | null>(null);

  useEffect(() => {
    let active = true;
    fetch(`/api/stats?period=${apiPeriod[period]}`, { credentials: "include" })
      .then(async (response) => { if (!response.ok) throw new Error("statistics unavailable"); const data = await response.json(); if (active) { setStatistics(data); setState("ready"); } })
      .catch(() => active && setState("error"));
    return () => { active = false; };
  }, [period]);

  useEffect(() => {
    let active = true;
    fetch("/api/continue-watching", { credentials: "include" })
      .then(async (response) => { if (!response.ok) throw new Error("continue watching unavailable"); const data = await response.json(); if (active) { setContinueWatching(data); setContinueWatchingState("ready"); } })
      .catch(() => active && setContinueWatchingState("error"));
    return () => { active = false; };
  }, []);

  useEffect(() => {
    let active = true;
    fetch("/api/watched-movies", { credentials: "include" })
      .then(async (response) => { if (!response.ok) throw new Error("watched movies unavailable"); const data = await response.json(); if (active) { setWatchedMovies(data); setWatchedMoviesState("ready"); } })
      .catch(() => active && setWatchedMoviesState("error"));
    return () => { active = false; };
  }, []);

  useEffect(() => {
    let active = true;
    fetch("/api/watched-series", { credentials: "include" })
      .then(async (response) => { if (!response.ok) throw new Error("watched series unavailable"); const data = await response.json(); if (active) { setWatchedSeries(data); setWatchedSeriesState("ready"); } })
      .catch(() => active && setWatchedSeriesState("error"));
    return () => { active = false; };
  }, []);

  useEffect(() => {
    let active = true;
    fetch(`/api/completed-movies?period=${apiPeriod[period]}`, { credentials: "include" })
      .then(async (response) => { if (!response.ok) throw new Error("completed movies unavailable"); const data = await response.json(); if (active) { setCompletedMovies(data); setCompletedMoviesState("ready"); } })
      .catch(() => active && setCompletedMoviesState("error"));
    return () => { active = false; };
  }, [period]);

  useEffect(() => {
    let active = true;
    fetch(`/api/completed-series?period=${apiPeriod[period]}`, { credentials: "include" })
      .then(async (response) => { if (!response.ok) throw new Error("completed series unavailable"); const data = await response.json(); if (active) { setCompletedSeries(data); setCompletedSeriesState("ready"); } })
      .catch(() => active && setCompletedSeriesState("error"));
    return () => { active = false; };
  }, [period]);

  return <div className="content page-view">
    <section className="period-tabs" aria-label="Zeitraum auswählen">{(["Woche", "Monat", "Jahr"] as Period[]).map((item) => <button className={period === item ? "selected" : ""} onClick={() => { setStatistics(null); setState("loading"); setCompletedMoviesState("loading"); setCompletedSeriesState("loading"); setPeriod(item); }} key={item} aria-pressed={period === item}>{item}</button>)}</section>
    <section className="week-grid" aria-label={`Kennzahlen für ${period}`}>
      <MetricCard icon="clock" tone="blue" value={statistics ? formatDuration(statistics.watchSeconds) : "—"} label="Sehzeit" detail={statistics ? comparisonText(statistics) : loadingCopy(state)} />
      <MetricCard icon="movie" tone="peach" value={statistics ? statistics.completedMovies : "—"} label="Filme abgeschlossen" detail={statistics ? period : loadingCopy(state)} onClick={statistics && statistics.completedMovies > 0 && completedMoviesState === "ready" ? () => setCompletedGridView("movies") : undefined} />
      <MetricCard icon="series" tone="mint" value={statistics ? statistics.completedSeries : "—"} label="Serien abgeschlossen" detail={statistics ? period : loadingCopy(state)} onClick={statistics && statistics.completedSeries > 0 && completedSeriesState === "ready" ? () => setCompletedGridView("series") : undefined} />
      <MetricCard icon="genre" tone="lilac" value={statistics?.favouriteGenre || "—"} label="Lieblingsgenre" detail={statistics ? "Nach Sehzeit" : loadingCopy(state)} genre />
    </section>

    <PosterRow title="Was ich gerade schaue" eyebrow="WEITERSCHAUEN" items={continueWatching} state={continueWatchingState} emptyLabel="Nichts in Bearbeitung." detail={(item) => `${item.progressPercent} % gesehen`} progress={(item) => item.progressPercent} onSelect={(item) => onSelectMedia({ source: "emby", id: item.id })} />
    <PosterRow title="Gesehene Filme" eyebrow="ALLE" items={watchedMovies} state={watchedMoviesState} emptyLabel="Noch keine Filme abgeschlossen." detail={(item) => item.genres[0] ?? ""} onSelect={(item) => onSelectMedia({ source: "emby", id: item.id })} />
    <PosterRow title="Gesehene Serien" eyebrow="ALLE" items={watchedSeries} state={watchedSeriesState} emptyLabel="Noch keine Serien abgeschlossen." detail={(item) => item.genres[0] ?? ""} onSelect={(item) => onSelectMedia({ source: "emby", id: item.id })} />

    <section className="chart-grid">
      <BarChart title="Meistgesehene Genres" data={topGenres(watchedMovies, watchedSeries)} />
      <BarChart title="Aktivität nach Wochentag" data={weekdayActivity(watchedMovies, watchedSeries)} />
    </section>

    {completedGridView === "movies" && <MediaGridScreen title={`Filme abgeschlossen · ${period}`} items={completedMovies} detail={(item) => item.genres[0] ?? ""} onSelect={(item) => onSelectMedia({ source: "emby", id: item.id })} onClose={() => setCompletedGridView(null)} />}
    {completedGridView === "series" && <MediaGridScreen title={`Serien abgeschlossen · ${period}`} items={completedSeries} detail={(item) => item.genres[0] ?? ""} onSelect={(item) => onSelectMedia({ source: "emby", id: item.id })} onClose={() => setCompletedGridView(null)} />}
  </div>;
}

function MediaGridScreen<T extends { id: string; title: string; posterUrl?: string }>({ title, items, detail, onSelect, onClose }: {
  title: string; items: readonly T[]; detail?: (item: T) => string; onSelect?: (item: T) => void; onClose: () => void;
}) {
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => { if (event.key === "Escape") onClose(); };
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [onClose]);

  return <div className="media-detail-overlay media-grid-overlay" role="dialog" aria-modal="true" aria-label={title}>
    <div className="media-detail-scroll">
      <button type="button" className="media-detail-close" onClick={onClose} aria-label="Schließen"><Icon name="close" /></button>
      <h1 className="media-grid-title">{title}</h1>
      {items.length === 0
        ? <p className="poster-status">Nichts vorhanden.</p>
        : <div className="media-grid">{items.map((item) => <button type="button" className="media-grid-entry" key={item.id} onClick={() => onSelect?.(item)}>
          <div className="poster wide" role="img" aria-label={item.title}>
            {item.posterUrl ? <img src={item.posterUrl} alt="" loading="lazy" /> : <span>{item.title}</span>}
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

function Requests({ items, state, onSelectMedia }: { items: RequestItem[]; state: LoadState; onSelectMedia: (selection: MediaSelection) => void }) {
  const [trending, trendingState] = useDiscoverList("/api/discover/trending");
  const [popularMovies, popularMoviesState] = useDiscoverList("/api/discover/movies/popular");
  const [upcomingMovies, upcomingMoviesState] = useDiscoverList("/api/discover/movies/upcoming");
  const [popularSeries, popularSeriesState] = useDiscoverList("/api/discover/series/popular");
  const [upcomingSeries, upcomingSeriesState] = useDiscoverList("/api/discover/series/upcoming");

  return <div className="content page-view">
    <section className="section-heading"><div><p className="eyebrow">SEERR · OFFEN</p><h2>Meine Anfragen</h2></div></section>
    <PosterRow title="" eyebrow="" gridTitle="Meine Anfragen" items={items} state={state} emptyLabel="Keine offenen Anfragen." detail={(item) => item.status} onSelect={(item) => onSelectMedia({ source: "seerr", id: item.tmdbId, mediaType: item.mediaType })} />

    <PosterRow title="Im Trend" eyebrow="SEERR · TMDB" items={trending} state={trendingState} emptyLabel="Nichts im Trend." detail={(item) => item.mediaType === "tv" ? "Serie" : "Film"} onSelect={(item) => onSelectMedia({ source: "seerr", id: item.id, mediaType: item.mediaType })} />
    <PosterRow title="Beliebte Filme" eyebrow="SEERR · TMDB" items={popularMovies} state={popularMoviesState} emptyLabel="Keine Daten." detail={() => "Beliebt"} onSelect={(item) => onSelectMedia({ source: "seerr", id: item.id, mediaType: item.mediaType })} />
    <PosterRow title="Demnächst erscheinende Filme" eyebrow="SEERR · TMDB" items={upcomingMovies} state={upcomingMoviesState} emptyLabel="Keine Daten." detail={() => "Demnächst"} onSelect={(item) => onSelectMedia({ source: "seerr", id: item.id, mediaType: item.mediaType })} />
    <PosterRow title="Beliebte Serien" eyebrow="SEERR · TMDB" items={popularSeries} state={popularSeriesState} emptyLabel="Keine Daten." detail={() => "Beliebt"} onSelect={(item) => onSelectMedia({ source: "seerr", id: item.id, mediaType: item.mediaType })} />
    <PosterRow title="Demnächst erscheinende Serien" eyebrow="SEERR · TMDB" items={upcomingSeries} state={upcomingSeriesState} emptyLabel="Keine Daten." detail={() => "Demnächst"} onSelect={(item) => onSelectMedia({ source: "seerr", id: item.id, mediaType: item.mediaType })} />
  </div>;
}
function MediaDetailScreen({ selection, onClose, onRequestCreated }: { selection: MediaSelection; onClose: () => void; onRequestCreated: () => void }) {
  const [detail, setDetail] = useState<MediaDetail | null>(null);
  const [state, setState] = useState<LoadState>("loading");
  const [selectedSeasons, setSelectedSeasons] = useState<number[]>([]);
  const [requestState, setRequestState] = useState<"idle" | "submitting" | "done" | "error">("idle");
  const [requestModalOpen, setRequestModalOpen] = useState(false);
  const mediaType = selection.source === "seerr" ? selection.mediaType : undefined;

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
    <div className="media-detail-scroll">
      <button type="button" className="media-detail-close" onClick={onClose} aria-label="Schließen"><Icon name="close" /></button>
      {state === "loading" && <p className="poster-status media-detail-status" role="status">Wird geladen …</p>}
      {state === "error" && <p className="poster-status media-detail-status">Details nicht verfügbar.</p>}
      {detail && <div className="media-detail">
        {detail.backdropUrl && <div className="media-detail-backdrop" style={{ backgroundImage: `url(${detail.backdropUrl})` }} />}
        <div className="media-detail-hero">
          <div className="media-detail-poster">{detail.posterUrl ? <img src={detail.posterUrl} alt="" /> : <span>{detail.title}</span>}</div>
          <div className="media-detail-info">
            {status && <span className="media-status-badge">{status}</span>}
            <h1>{detail.title}{detail.year ? ` (${detail.year})` : ""}</h1>
            <p className="media-detail-meta">
              {detail.officialRating && <span>{detail.officialRating}</span>}
              {detail.runtimeMinutes > 0 && <span>{detail.runtimeMinutes} Minuten</span>}
              {detail.genres?.length > 0 && <span>{detail.genres.join(", ")}</span>}
            </p>
            {detail.communityRating > 0 && <p className="media-detail-rating">★ {detail.communityRating.toFixed(1)}</p>}
          </div>
        </div>
        {canRequest && <div className="request-row">
          {requestState === "done"
            ? <p className="request-confirmation">Angefragt ✓</p>
            : <button type="button" className="request-button" onClick={() => setRequestModalOpen(true)}>Anfragen</button>}
        </div>}
        {detail.overview && <section className="media-detail-overview"><h2>Übersicht</h2><p>{detail.overview}</p></section>}
        {(detail.status || detail.releaseDate || (detail.studios && detail.studios.length > 0)) && <section className="media-detail-facts">
          <dl>
            {detail.status && <div><dt>Status</dt><dd>{detail.status}</dd></div>}
            {detail.releaseDate && <div><dt>Erscheinungsdatum</dt><dd>{formatFullDate(detail.releaseDate)}</dd></div>}
            {detail.studios && detail.studios.length > 0 && <div><dt>Studios</dt><dd>{detail.studios.join(", ")}</dd></div>}
          </dl>
        </section>}
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

function Profile({ user }: { user: { name: string } }) {
  const [signingOut, setSigningOut] = useState(false);
  const logout = async () => {
    setSigningOut(true);
    try { await fetch("/api/auth/logout", { method: "POST", credentials: "include" }); } catch { /* reload clears the client session either way */ }
    window.location.reload();
  };
  return <div className="content page-view profile">
    <section className="profile-head"><div className="avatar big"><UserAvatar name={user.name} /></div><div><p className="eyebrow">EMBY-PROFIL</p><h2>{user.name}</h2></div></section>
    <button className="logout-button" onClick={logout} disabled={signingOut}>{signingOut ? "Abmeldung läuft …" : "Abmelden"}</button>
  </div>;
}
function greeting() { const hour = new Date().getHours(); return hour < 12 ? "Guten Morgen" : hour < 18 ? "Guten Tag" : "Guten Abend"; }
function loadingCopy(state: LoadState) { return state === "error" ? "Nicht verfügbar" : "Wird geladen …"; }
function formatDuration(seconds: number) { const hours = Math.floor(seconds / 3600); const minutes = Math.floor((seconds % 3600) / 60); return hours > 0 ? `${hours}\u00a0Std. ${minutes}\u00a0Min.` : `${minutes}\u00a0Min.`; }
function comparisonText(statistics: PersonalStats) { if (statistics.previousWatchSeconds === 0) return "Keine Vergleichsdaten"; const change = Math.round(((statistics.watchSeconds - statistics.previousWatchSeconds) / statistics.previousWatchSeconds) * 100); return `${change >= 0 ? "Mehr" : "Weniger"} als im vorherigen Zeitraum: ${new Intl.NumberFormat("de-DE").format(Math.abs(change))}\u00a0%`; }
