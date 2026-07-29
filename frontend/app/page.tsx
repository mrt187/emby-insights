"use client";

import { type ReactNode, useEffect, useRef, useState } from "react";
import { LoginScreen } from "./login-screen";

type Page = "Heute" | "Statistik" | "Anfragen" | "Profil";
type Period = "Woche" | "Monat" | "Jahr";
type StatisticsPeriod = "week" | "month" | "year";
type PersonalStats = { watchSeconds: number; previousWatchSeconds: number; completedMovies: number; completedSeries: number; favouriteGenre: string; periodStartsAt: string; periodEndsAt: string };
type UpcomingItem = { id: string; title: string; posterUrl: string; premiereDate: string };
type RequestItem = { id: string; title: string; posterUrl: string; status: string };
type NewForYouItem = { id: string; title: string; posterUrl: string };
type ContinueWatchingItem = { id: string; title: string; posterUrl: string; progressPercent: number };
type WatchedItem = { id: string; title: string; posterUrl: string; genres: string[]; lastPlayedDate: string };
type DiscoverItem = { id: string; title: string; posterUrl: string; mediaType: string };
type IconName = "home" | "chart" | "sparkle" | "user" | "bell" | "arrow" | "clock" | "movie" | "series" | "genre";
type Tone = "blue" | "peach" | "mint" | "lilac";
type LoadState = "loading" | "ready" | "error";

const nav: { label: Page; icon: IconName }[] = [
  { label: "Heute", icon: "home" }, { label: "Statistik", icon: "chart" },
  { label: "Anfragen", icon: "sparkle" }, { label: "Profil", icon: "user" },
];
const apiPeriod: Record<Period, StatisticsPeriod> = { Woche: "week", Monat: "month", Jahr: "year" };
const APP_VERSION = "0.8.1";

const dateFormatter = new Intl.DateTimeFormat("de-DE", { day: "2-digit", month: "short" });
function formatPremiereDate(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "" : dateFormatter.format(date);
}

export default function Home() {
  const [page, setPage] = useState<Page>("Heute");
  const [noticeOpen, setNoticeOpen] = useState(false);
  const [unread, setUnread] = useState(2);
  const [user, setUser] = useState<{ id: string; name: string } | null>(null);
  const [checkingSession, setCheckingSession] = useState(true);
  const [weekStats, setWeekStats] = useState<PersonalStats | null>(null);
  const [weekState, setWeekState] = useState<LoadState>("loading");
  const [upcomingItems, setUpcomingItems] = useState<UpcomingItem[]>([]);
  const [upcomingState, setUpcomingState] = useState<LoadState>("loading");
  const [requestItems, setRequestItems] = useState<RequestItem[]>([]);
  const [requestState, setRequestState] = useState<LoadState>("loading");
  const [newForYouItems, setNewForYouItems] = useState<NewForYouItem[]>([]);
  const [newForYouState, setNewForYouState] = useState<LoadState>("loading");
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
    fetch("/api/stats?period=week", { credentials: "include" })
      .then(async (response) => {
        if (!response.ok) throw new Error("statistics unavailable");
        const data = await response.json();
        if (active) { setWeekStats(data); setWeekState("ready"); }
      })
      .catch(() => active && setWeekState("error"));
    return () => { active = false; };
  }, [user]);

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
      {page === "Heute" && <Today user={user} onStats={() => selectPage("Statistik")} statistics={weekStats} state={weekState} upcoming={upcomingItems} upcomingState={upcomingState} requests={requestItems} requestState={requestState} newForYou={newForYouItems} newForYouState={newForYouState} />}
      {page === "Statistik" && <Stats />}
      {page === "Anfragen" && <Requests items={requestItems} state={requestState} />}
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

function Today({ user, onStats, statistics, state, upcoming, upcomingState, requests, requestState, newForYou, newForYouState }: {
  user: { name: string }; onStats: () => void; statistics: PersonalStats | null; state: LoadState;
  upcoming: UpcomingItem[]; upcomingState: LoadState; requests: RequestItem[]; requestState: LoadState;
  newForYou: NewForYouItem[]; newForYouState: LoadState;
}) {
  const detail = state === "error" ? "Noch keine Statistik verfügbar" : statistics ? comparisonText(statistics) : "Wird geladen …";
  return <div className="content today-view">
    <section className="section-heading rhythm-heading"><div><p className="eyebrow">DEIN PROFIL</p><h2>Dein Rhythmus</h2></div><button className="text-button" onClick={onStats}>Alle Details <Icon name="arrow" /></button></section>
    <UserInsightCard user={user} statistics={statistics} state={state} detail={detail} />
    <PosterRow title="Demnächst" eyebrow="COMING SOON · NÄCHSTE 4 WOCHEN" items={upcoming} state={upcomingState} emptyLabel="Nichts Neues in den nächsten vier Wochen." detail={(item) => formatPremiereDate(item.premiereDate)} />
    <PosterRow title="Meine Anfragen" eyebrow="SEERR · OFFEN" items={requests} state={requestState} emptyLabel="Keine offenen Anfragen." detail={(item) => item.status} />
    <PosterRow title="Neu für dich" eyebrow="IN DEN LETZTEN 14 TAGEN" items={newForYou} state={newForYouState} emptyLabel="Nichts Neues in den letzten 14 Tagen." detail={() => "Ungesehen"} />
  </div>;
}

function UserInsightCard({ user, statistics, state, detail }: { user: { name: string }; statistics: PersonalStats | null; state: LoadState; detail: string }) {
  return <section className="user-insight-card" aria-label={`Wochenübersicht von ${user.name}`}>
    <WeeklyCarousel user={user} statistics={statistics} state={state} detail={detail} />
    {state === "loading" && <span className="sr-only" role="status">Deine Wochenstatistik wird geladen …</span>}
  </section>;
}

const SLIDE_INTERVAL = 5000;

function WeeklyCarousel({ user, statistics, state, detail }: { user: { name: string }; statistics: PersonalStats | null; state: LoadState; detail: string }) {
  const slides: { key: string; icon: IconName; tone: Tone; label: string; value: string; detail: string; text?: boolean }[] = [
    { key: "week", icon: "clock", tone: "blue", label: "Diese Woche", value: statistics ? formatDuration(statistics.watchSeconds) : "—", detail },
    { key: "movies", icon: "movie", tone: "peach", label: "Filme", value: statistics ? String(statistics.completedMovies) : "—", detail: statistics ? "Abgeschlossen" : loadingCopy(state) },
    { key: "series", icon: "series", tone: "mint", label: "Serien", value: statistics ? String(statistics.completedSeries) : "—", detail: statistics ? "Abgeschlossen" : loadingCopy(state) },
    { key: "genre", icon: "genre", tone: "lilac", label: "Lieblingsgenre", value: statistics?.favouriteGenre || "—", detail: statistics ? "Nach Sehzeit" : loadingCopy(state), text: true },
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
      aria-label="Deine Wochenwerte"
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
          <div><p className="eyebrow">DEIN MEDIENPROFIL</p><h3>{user.name}</h3><p>Deine ganz persönliche Woche in Emby.</p></div>
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

function MetricCard({ icon, tone, value, label, detail, positive, genre = false }: { icon: IconName; tone: Tone; value: string | number; label: string; detail: string; positive?: boolean; genre?: boolean }) {
  return <article className={`metric-card tone-${tone}${genre ? " genre-card" : ""}`}><span className="metric-icon"><Icon name={icon} /></span><strong>{value}</strong><p>{label}</p><small className={positive ? "up" : undefined}>{detail}</small></article>;
}

function PosterRow<T extends { id: string; title: string; posterUrl?: string }>({ title, eyebrow, items, detail, state, emptyLabel, progress }: {
  title?: string; eyebrow?: string; items: readonly T[]; detail: (item: T) => string; state?: LoadState; emptyLabel?: string; progress?: (item: T) => number;
}) {
  return <section className="poster-section">
    {(title || eyebrow) && <div className="section-heading"><div>{eyebrow && <p className="eyebrow">{eyebrow}</p>}{title && <h2>{title}</h2>}</div></div>}
    {state === "loading" && <p className="poster-status" role="status">Wird geladen …</p>}
    {state === "error" && <p className="poster-status">Nicht verfügbar</p>}
    {state !== "loading" && state !== "error" && items.length === 0 && <p className="poster-status">{emptyLabel ?? "Nichts vorhanden."}</p>}
    {items.length > 0 && <div className="poster-scroller">{items.map((item) => <article className="poster-entry" key={item.id}>
      <div className="poster wide" role="img" aria-label={item.title}>
        {item.posterUrl ? <img src={item.posterUrl} alt="" loading="lazy" /> : <span>{item.title}</span>}
        {progress && <div className="poster-progress"><div className="poster-progress-fill" style={{ width: `${progress(item)}%` }} /></div>}
      </div>
      <strong>{item.title}</strong><small>{detail(item)}</small>
    </article>)}</div>}
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

function Stats() {
  const [period, setPeriod] = useState<Period>("Woche");
  const [statistics, setStatistics] = useState<PersonalStats | null>(null);
  const [state, setState] = useState<LoadState>("loading");
  const [continueWatching, setContinueWatching] = useState<ContinueWatchingItem[]>([]);
  const [continueWatchingState, setContinueWatchingState] = useState<LoadState>("loading");
  const [watchedMovies, setWatchedMovies] = useState<WatchedItem[]>([]);
  const [watchedMoviesState, setWatchedMoviesState] = useState<LoadState>("loading");
  const [watchedSeries, setWatchedSeries] = useState<WatchedItem[]>([]);
  const [watchedSeriesState, setWatchedSeriesState] = useState<LoadState>("loading");

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
    fetch(`/api/watched-movies?period=${apiPeriod[period]}`, { credentials: "include" })
      .then(async (response) => { if (!response.ok) throw new Error("watched movies unavailable"); const data = await response.json(); if (active) { setWatchedMovies(data); setWatchedMoviesState("ready"); } })
      .catch(() => active && setWatchedMoviesState("error"));
    return () => { active = false; };
  }, [period]);

  useEffect(() => {
    let active = true;
    fetch(`/api/watched-series?period=${apiPeriod[period]}`, { credentials: "include" })
      .then(async (response) => { if (!response.ok) throw new Error("watched series unavailable"); const data = await response.json(); if (active) { setWatchedSeries(data); setWatchedSeriesState("ready"); } })
      .catch(() => active && setWatchedSeriesState("error"));
    return () => { active = false; };
  }, [period]);

  return <div className="content page-view">
    <section className="period-tabs" aria-label="Zeitraum auswählen">{(["Woche", "Monat", "Jahr"] as Period[]).map((item) => <button className={period === item ? "selected" : ""} onClick={() => { setStatistics(null); setState("loading"); setWatchedMoviesState("loading"); setWatchedSeriesState("loading"); setPeriod(item); }} key={item} aria-pressed={period === item}>{item}</button>)}</section>
    <section className="summary-banner" aria-live="polite"><p>DEINE {period.toUpperCase()}</p><h2>{statistics ? formatDuration(statistics.watchSeconds) : "—"}</h2><span>{state === "error" ? "Statistik ist gerade nicht verfügbar." : statistics ? comparisonText(statistics) : "Statistik wird geladen …"}</span></section>
    <section className="week-grid" aria-label={`Kennzahlen für ${period}`}>
      <MetricCard icon="movie" tone="peach" value={statistics ? statistics.completedMovies : "—"} label="Filme abgeschlossen" detail={statistics ? period : loadingCopy(state)} />
      <MetricCard icon="series" tone="mint" value={statistics ? statistics.completedSeries : "—"} label="Serien abgeschlossen" detail={statistics ? period : loadingCopy(state)} />
      <MetricCard icon="genre" tone="lilac" value={statistics?.favouriteGenre || "—"} label="Lieblingsgenre" detail={statistics ? "Nach Sehzeit" : loadingCopy(state)} genre />
    </section>

    <PosterRow title="Was ich gerade schaue" eyebrow="WEITERSCHAUEN" items={continueWatching} state={continueWatchingState} emptyLabel="Nichts in Bearbeitung." detail={(item) => `${item.progressPercent} % gesehen`} progress={(item) => item.progressPercent} />
    <PosterRow title="Gesehene Filme" eyebrow={period.toUpperCase()} items={watchedMovies} state={watchedMoviesState} emptyLabel="Noch keine Filme abgeschlossen." detail={(item) => item.genres[0] ?? ""} />
    <PosterRow title="Gesehene Serien" eyebrow={period.toUpperCase()} items={watchedSeries} state={watchedSeriesState} emptyLabel="Noch keine Serien abgeschlossen." detail={(item) => item.genres[0] ?? ""} />

    <section className="chart-grid">
      <BarChart title="Meistgesehene Genres" data={topGenres(watchedMovies, watchedSeries)} />
      <BarChart title="Aktivität nach Wochentag" data={weekdayActivity(watchedMovies, watchedSeries)} />
    </section>
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

function Requests({ items, state }: { items: RequestItem[]; state: LoadState }) {
  const [trending, trendingState] = useDiscoverList("/api/discover/trending");
  const [popularMovies, popularMoviesState] = useDiscoverList("/api/discover/movies/popular");
  const [upcomingMovies, upcomingMoviesState] = useDiscoverList("/api/discover/movies/upcoming");
  const [popularSeries, popularSeriesState] = useDiscoverList("/api/discover/series/popular");
  const [upcomingSeries, upcomingSeriesState] = useDiscoverList("/api/discover/series/upcoming");

  return <div className="content page-view">
    <section className="section-heading"><div><p className="eyebrow">SEERR · OFFEN</p><h2>Meine Anfragen</h2></div></section>
    <PosterRow title="" eyebrow="" items={items} state={state} emptyLabel="Keine offenen Anfragen." detail={(item) => item.status} />

    <PosterRow title="Im Trend" eyebrow="SEERR · TMDB" items={trending} state={trendingState} emptyLabel="Nichts im Trend." detail={(item) => item.mediaType === "tv" ? "Serie" : "Film"} />
    <PosterRow title="Beliebte Filme" eyebrow="SEERR · TMDB" items={popularMovies} state={popularMoviesState} emptyLabel="Keine Daten." detail={() => "Beliebt"} />
    <PosterRow title="Demnächst erscheinende Filme" eyebrow="SEERR · TMDB" items={upcomingMovies} state={upcomingMoviesState} emptyLabel="Keine Daten." detail={() => "Demnächst"} />
    <PosterRow title="Beliebte Serien" eyebrow="SEERR · TMDB" items={popularSeries} state={popularSeriesState} emptyLabel="Keine Daten." detail={() => "Beliebt"} />
    <PosterRow title="Demnächst erscheinende Serien" eyebrow="SEERR · TMDB" items={upcomingSeries} state={upcomingSeriesState} emptyLabel="Keine Daten." detail={() => "Demnächst"} />
  </div>;
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
