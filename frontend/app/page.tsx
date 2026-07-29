"use client";

import { type ReactNode, useEffect, useRef, useState } from "react";
import { LoginScreen } from "./login-screen";

type Page = "Heute" | "Statistik" | "Anfragen" | "Profil";
type Period = "Woche" | "Monat" | "Jahr";
type StatisticsPeriod = "week" | "month" | "year";
type PersonalStats = { watchSeconds: number; previousWatchSeconds: number; completedMovies: number; completedSeries: number; favouriteGenre: string; periodStartsAt: string; periodEndsAt: string };
type IconName = "home" | "chart" | "sparkle" | "user" | "bell" | "arrow" | "clock" | "movie" | "series" | "genre";
type Tone = "blue" | "peach" | "mint" | "lilac";
type LoadState = "loading" | "ready" | "error";

const nav: { label: Page; icon: IconName }[] = [
  { label: "Heute", icon: "home" }, { label: "Statistik", icon: "chart" },
  { label: "Anfragen", icon: "sparkle" }, { label: "Profil", icon: "user" },
];
const apiPeriod: Record<Period, StatisticsPeriod> = { Woche: "week", Monat: "month", Jahr: "year" };
const APP_VERSION = "0.3.5";

const upcoming = [
  { date: "01. Aug.", title: "The Last of Us", art: "last" }, { date: "04. Aug.", title: "Alien: Earth", art: "alien" },
  { date: "08. Aug.", title: "Wednesday", art: "wednesday" }, { date: "12. Aug.", title: "The Bear", art: "bear" },
  { date: "15. Aug.", title: "Andor", art: "andor" }, { date: "22. Aug.", title: "Foundation", art: "foundation" },
];

const requests = [
  { title: "Dune: Part Three", status: "Wird gesucht", art: "dune" }, { title: "The Bear · Staffel 5", status: "Genehmigt", art: "bear" },
  { title: "Severance · Staffel 3", status: "In Bearbeitung", art: "severance" }, { title: "Mickey 17", status: "Angefragt", art: "mickey" },
];

const newForYou = [
  ["Sinners", "sinners"], ["The Studio", "studio"], ["Mickey 17", "mickey"], ["The Gorge", "gorge"], ["The Brutalist", "brutalist"],
  ["Black Mirror", "mirror"], ["Companion", "companion"], ["Anora", "anora"], ["Flow", "flow"], ["The Monkey", "monkey"],
  ["Wolfs", "wolfs"], ["Conclave", "conclave"], ["Nosferatu", "nosferatu"], ["Civil War", "civil"], ["The Wild Robot", "robot"],
] as const;

export default function Home() {
  const [page, setPage] = useState<Page>("Heute");
  const [noticeOpen, setNoticeOpen] = useState(false);
  const [unread, setUnread] = useState(2);
  const [user, setUser] = useState<{ id: string; name: string } | null>(null);
  const [checkingSession, setCheckingSession] = useState(true);
  const [weekStats, setWeekStats] = useState<PersonalStats | null>(null);
  const [weekState, setWeekState] = useState<LoadState>("loading");
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
      {page === "Heute" && <Today user={user} onStats={() => selectPage("Statistik")} statistics={weekStats} state={weekState} />}
      {page === "Statistik" && <Stats />}
      {page === "Anfragen" && <Requests />}
      {page === "Profil" && <Profile user={user} />}
    </section>

    <nav className="bottom-nav" aria-label="Hauptnavigation (mobil)">{nav.map((item) => <button key={item.label} className={page === item.label ? "active" : ""} onClick={() => selectPage(item.label)} aria-current={page === item.label ? "page" : undefined}><Icon name={item.icon} /><span>{item.label}</span></button>)}</nav>
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

function Today({ user, onStats, statistics, state }: { user: { name: string }; onStats: () => void; statistics: PersonalStats | null; state: LoadState }) {
  const detail = state === "error" ? "Noch keine Statistik verfügbar" : statistics ? comparisonText(statistics) : "Wird geladen …";
  return <div className="content today-view">
    <section className="today-hero" aria-labelledby="today-hero-title">
      <div className="hero-copy-block">
        <p className="eyebrow">DEIN MEDIENMOMENT</p>
        <h2 id="today-hero-title">Deine Mediathek.<br /><em>Dein Rhythmus.</em></h2>
        <p>Alles, was diese Woche für dich zählt – auf einen Blick, ohne Ablenkung.</p>
        <button className="hero-action" onClick={onStats}>Statistik entdecken <Icon name="arrow" /></button>
      </div>
      <div className="hero-weekly-total">
        <span>Diese Woche</span>
        <strong>{statistics ? formatDuration(statistics.watchSeconds) : "—"}</strong>
        <small>{state === "error" ? "Später erneut versuchen" : statistics ? "Zeit für deine Favoriten" : "Statistik wird geladen …"}</small>
      </div>
      <div className="hero-orbit" aria-hidden="true"><i /><i /><b /></div>
    </section>
    <section className="section-heading rhythm-heading"><div><p className="eyebrow">DEIN PROFIL</p><h2>Dein Rhythmus</h2></div><button className="text-button" onClick={onStats}>Alle Details <Icon name="arrow" /></button></section>
    <UserInsightCard user={user} statistics={statistics} state={state} detail={detail} />
    <PosterRow title="Demnächst" eyebrow="COMING SOON · NÄCHSTE 4 WOCHEN" items={upcoming} detail={(item) => item.date} />
    <PosterRow title="Meine Anfragen" eyebrow="SEERR · OFFEN" items={requests} detail={(item) => item.status} />
    <PosterRow title="Neu für dich" eyebrow="IN DEN LETZTEN 14 TAGEN" items={newForYou.map(([title, art]) => ({ title, art }))} detail={() => "Ungesehen"} />
  </div>;
}

function UserInsightCard({ user, statistics, state, detail }: { user: { name: string }; statistics: PersonalStats | null; state: LoadState; detail: string }) {
  return <section className="user-insight-card" aria-label={`Wochenübersicht von ${user.name}`}>
    <div className="user-insight-identity">
      <div className="profile-avatar"><UserAvatar name={user.name} /></div>
      <div><p className="eyebrow">DEIN MEDIENPROFIL</p><h3>{user.name}</h3><p>Deine ganz persönliche Woche in Emby.</p></div>
    </div>
    <div className="user-insight-feature">
      <span>Diese Woche</span>
      <strong>{statistics ? formatDuration(statistics.watchSeconds) : "—"}</strong>
      <small>{detail}</small>
    </div>
    <div className="user-insight-stats">
      <div className="tone-peach"><span className="user-stat-icon"><Icon name="movie" /></span><strong>{statistics ? statistics.completedMovies : "—"}</strong><small>Filme</small></div>
      <div className="tone-mint"><span className="user-stat-icon"><Icon name="series" /></span><strong>{statistics ? statistics.completedSeries : "—"}</strong><small>Serien</small></div>
      <div className="tone-lilac stat-text"><span className="user-stat-icon"><Icon name="genre" /></span><strong>{statistics?.favouriteGenre || "—"}</strong><small>Lieblingsgenre</small></div>
    </div>
    {state === "loading" && <span className="sr-only" role="status">Deine Wochenstatistik wird geladen …</span>}
  </section>;
}

function MetricCard({ icon, tone, value, label, detail, positive, genre = false }: { icon: IconName; tone: Tone; value: string | number; label: string; detail: string; positive?: boolean; genre?: boolean }) {
  return <article className={`metric-card tone-${tone}${genre ? " genre-card" : ""}`}><span className="metric-icon"><Icon name={icon} /></span><strong>{value}</strong><p>{label}</p><small className={positive ? "up" : undefined}>{detail}</small></article>;
}

function PosterRow({ title, eyebrow, items, detail }: { title?: string; eyebrow?: string; items: readonly { title: string; art: string }[]; detail: (item: { title: string; art: string }) => string }) {
  return <section className="poster-section">{(title || eyebrow) && <div className="section-heading"><div>{eyebrow && <p className="eyebrow">{eyebrow}</p>}{title && <h2>{title}</h2>}</div></div>}<div className="poster-scroller">{items.map((item) => <article className="poster-entry" key={item.title}><div className={`poster wide ${item.art}`} role="img" aria-label={item.title}><span>{item.title}</span></div><strong>{item.title}</strong><small>{detail(item)}</small></article>)}</div></section>;
}

function Stats() {
  const [period, setPeriod] = useState<Period>("Woche");
  const [statistics, setStatistics] = useState<PersonalStats | null>(null);
  const [state, setState] = useState<LoadState>("loading");
  useEffect(() => {
    let active = true;
    fetch(`/api/stats?period=${apiPeriod[period]}`, { credentials: "include" })
      .then(async (response) => { if (!response.ok) throw new Error("statistics unavailable"); const data = await response.json(); if (active) { setStatistics(data); setState("ready"); } })
      .catch(() => active && setState("error"));
    return () => { active = false; };
  }, [period]);
  return <div className="content page-view">
    <section className="period-tabs" aria-label="Zeitraum auswählen">{(["Woche", "Monat", "Jahr"] as Period[]).map((item) => <button className={period === item ? "selected" : ""} onClick={() => { setStatistics(null); setState("loading"); setPeriod(item); }} key={item} aria-pressed={period === item}>{item}</button>)}</section>
    <section className="summary-banner" aria-live="polite"><p>DEINE {period.toUpperCase()}</p><h2>{statistics ? formatDuration(statistics.watchSeconds) : "—"}</h2><span>{state === "error" ? "Statistik ist gerade nicht verfügbar." : statistics ? comparisonText(statistics) : "Statistik wird geladen …"}</span></section>
    <section className="week-grid" aria-label={`Kennzahlen für ${period}`}>
      <MetricCard icon="clock" tone="blue" value={statistics ? formatDuration(statistics.watchSeconds) : "—"} label="Sehzeit" detail={statistics ? period : loadingCopy(state)} />
      <MetricCard icon="movie" tone="peach" value={statistics ? statistics.completedMovies : "—"} label="Filme abgeschlossen" detail={statistics ? period : loadingCopy(state)} />
      <MetricCard icon="series" tone="mint" value={statistics ? statistics.completedSeries : "—"} label="Serien abgeschlossen" detail={statistics ? period : loadingCopy(state)} />
      <MetricCard icon="genre" tone="lilac" value={statistics?.favouriteGenre || "—"} label="Lieblingsgenre" detail={statistics ? "Nach Sehzeit" : loadingCopy(state)} genre />
    </section>
  </div>;
}

function Requests() { return <div className="content page-view"><section className="section-heading"><div><p className="eyebrow">SEERR · OFFEN</p><h2>Meine Anfragen</h2></div></section><PosterRow title="" eyebrow="" items={requests} detail={(item) => item.status} /></div>; }
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
