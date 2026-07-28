"use client";

import { useEffect, useState } from "react";
import { LoginScreen } from "./login-screen";

type Page = "Heute" | "Statistik" | "Anfragen" | "Profil";
type Period = "Woche" | "Monat" | "Jahr";
type StatisticsPeriod = "week" | "month" | "year";
type PersonalStats = { watchSeconds: number; previousWatchSeconds: number; periodStartsAt: string; periodEndsAt: string };

const nav: { label: Page; icon: string }[] = [
  { label: "Heute", icon: "⌂" },
  { label: "Statistik", icon: "◔" },
  { label: "Anfragen", icon: "✦" },
  { label: "Profil", icon: "◎" },
];

const upcoming = [
  { date: "01. Aug.", title: "The Last of Us", art: "last" },
  { date: "04. Aug.", title: "Alien: Earth", art: "alien" },
  { date: "08. Aug.", title: "Wednesday", art: "wednesday" },
  { date: "12. Aug.", title: "The Bear", art: "bear" },
  { date: "15. Aug.", title: "Andor", art: "andor" },
  { date: "22. Aug.", title: "Foundation", art: "foundation" },
];

const requests = [
  { title: "Dune: Part Three", status: "Wird gesucht", art: "dune" },
  { title: "The Bear · Staffel 5", status: "Genehmigt", art: "bear" },
  { title: "Severance · Staffel 3", status: "In Bearbeitung", art: "severance" },
  { title: "Mickey 17", status: "Angefragt", art: "mickey" },
];

const newForYou = [
  ["Sinners", "sinners"], ["The Studio", "studio"], ["Mickey 17", "mickey"],
  ["The Gorge", "gorge"], ["The Brutalist", "brutalist"], ["Black Mirror", "mirror"],
  ["Companion", "companion"], ["Anora", "anora"], ["Flow", "flow"],
  ["The Monkey", "monkey"], ["Wolfs", "wolfs"], ["Conclave", "conclave"],
  ["Nosferatu", "nosferatu"], ["Civil War", "civil"], ["The Wild Robot", "robot"],
] as const;

export default function Home() {
  const [page, setPage] = useState<Page>("Heute");
  const [noticeOpen, setNoticeOpen] = useState(false);
  const [unread, setUnread] = useState(2);
  const [user, setUser] = useState<{ id: string; name: string } | null>(null);
  const [checkingSession, setCheckingSession] = useState(true);
  const [weekStats, setWeekStats] = useState<PersonalStats | null>(null);

  useEffect(() => {
    fetch("/api/me", { credentials: "include" })
      .then(async (response) => response.ok ? setUser(await response.json()) : null)
      .catch(() => null)
      .finally(() => setCheckingSession(false));
  }, []);

  useEffect(() => {
    if (!user) return;
    fetch("/api/stats?period=week", { credentials: "include" })
      .then(async (response) => response.ok ? setWeekStats(await response.json()) : null)
      .catch(() => null);
  }, [user]);

  if (checkingSession) return <main className="login-shell"><p className="loading-copy">Emby Insights wird geladen …</p></main>;
  if (!user) return <LoginScreen onAuthenticated={setUser} />;

  function openNotices() {
    setNoticeOpen((isOpen) => !isOpen);
    setUnread(0);
  }

  return (
    <main className="app-shell">
      <aside className="side-nav" aria-label="Hauptnavigation">
        <div className="brand"><img className="brand-logo" src="/emby-insights-logo.svg" alt="Emby Insights" /><span>insights</span></div>
        <nav>{nav.map((item) => <button className={page === item.label ? "nav-item active" : "nav-item"} key={item.label} onClick={() => setPage(item.label)}><span>{item.icon}</span>{item.label}</button>)}</nav>
        <div className="server-status"><i /> Verbunden mit Emby</div>
      </aside>

      <section className="screen">
        <header className="topbar">
          <div><p className="eyebrow">DEIN PERSÖNLICHER ÜBERBLICK</p><h1>{page === "Heute" ? `Guten Abend, ${user.name}` : page}</h1></div>
          <div className="header-actions">
            <button className="notice-button" aria-label="Benachrichtigungen" onClick={openNotices}>♢{unread > 0 && <b>{unread}</b>}</button>
            <button className="avatar" aria-label="Profil öffnen" onClick={() => setPage("Profil")}><UserAvatar name={user.name} /></button>
            {noticeOpen && <div className="notifications"><strong>Benachrichtigungen</strong><p>Deine Anfrage „Severance“ wird bearbeitet.</p><p>Am Freitag erscheint Alien: Earth.</p></div>}
          </div>
        </header>
        {page === "Heute" && <Today onStats={() => setPage("Statistik")} statistics={weekStats} />}
        {page === "Statistik" && <Stats />}
        {page === "Anfragen" && <Requests />}
        {page === "Profil" && <Profile user={user} />}
      </section>

      <nav className="bottom-nav" aria-label="Hauptnavigation">
        {nav.map((item) => <button key={item.label} className={page === item.label ? "active" : ""} onClick={() => setPage(item.label)}><span>{item.icon}</span>{item.label}</button>)}
      </nav>
    </main>
  );
}

function UserAvatar({ name }: { name: string }) {
  const initial = name.trim().charAt(0).toUpperCase() || "?";
  return <span className="user-avatar" aria-label={"Profilbild von " + name}><span className="avatar-initial">{initial}</span><img src="/api/me/avatar" alt="" onError={(event) => event.currentTarget.remove()} /></span>;
}

function Today({ onStats, statistics }: { onStats: () => void; statistics: PersonalStats | null }) {
  const comparison = statistics ? comparisonText(statistics) : "Wird geladen …";
  return <div className="content today-view"><section className="section-heading"><div><p className="eyebrow">DEIN ÜBERBLICK</p><h2>Meine Woche</h2></div><button className="text-button" onClick={onStats}>Statistik <span>→</span></button></section><section className="week-grid"><article className="metric-card"><span className="metric-icon blue">◔</span><strong>{statistics ? formatDuration(statistics.watchSeconds) : "—"}</strong><p>Sehzeit</p><small className="up">{comparison}</small></article><article className="metric-card"><span className="metric-icon peach">◉</span><strong>—</strong><p>Filme abgeschlossen</p><small>folgt</small></article><article className="metric-card"><span className="metric-icon mint">✓</span><strong>—</strong><p>Serien abgeschlossen</p><small>folgt</small></article><article className="metric-card genre-card"><span className="metric-icon lilac">✦</span><p>Lieblingsgenre</p><strong>—</strong><small>folgt</small></article></section><PosterRow title="Demnächst" eyebrow="COMING SOON · NÄCHSTE 4 WOCHEN" items={upcoming} detail={(item) => item.date} /><PosterRow title="Meine Anfragen" eyebrow="SEERR · OFFEN" items={requests} detail={(item) => item.status} /><PosterRow title="Neu für dich" eyebrow="IN DEN LETZTEN 14 TAGEN" items={newForYou.map(([title, art]) => ({ title, art }))} detail={() => "Ungesehen"} /></div>;
}

function PosterRow({ title, eyebrow, items, detail }: { title: string; eyebrow: string; items: readonly { title: string; art: string }[]; detail: (item: { title: string; art: string }) => string }) {
  return <section className="poster-section">{title && <div className="section-heading"><div><p className="eyebrow">{eyebrow}</p><h2>{title}</h2></div></div>}<div className="poster-scroller">{items.map((item) => <article className="poster-entry" key={item.title}><div className={`poster wide ${item.art}`}><span>{item.title}</span></div><strong>{item.title}</strong><small>{detail(item)}</small></article>)}</div></section>;
}

function Stats() {
  const [period, setPeriod] = useState<Period>("Woche");
  const [statistics, setStatistics] = useState<PersonalStats | null>(null);
  const apiPeriod: Record<Period, StatisticsPeriod> = { Woche: "week", Monat: "month", Jahr: "year" };
  useEffect(() => { setStatistics(null); fetch(`/api/stats?period=${apiPeriod[period]}`, { credentials: "include" }).then(async (response) => response.ok ? setStatistics(await response.json()) : null).catch(() => null); }, [period]);
  return <div className="content page-view"><section className="period-tabs">{(["Woche", "Monat", "Jahr"] as Period[]).map((item) => <button className={period === item ? "selected" : ""} onClick={() => setPeriod(item)} key={item}>{item}</button>)}</section><section className="summary-banner"><p>DEINE {period.toUpperCase()}</p><h2>{statistics ? formatDuration(statistics.watchSeconds) : "—"}</h2><span>{statistics ? comparisonText(statistics) : "Statistik wird geladen …"}</span></section><section className="week-grid"><article className="metric-card"><strong>{statistics ? formatDuration(statistics.watchSeconds) : "—"}</strong><p>Sehzeit</p></article><article className="metric-card"><strong>—</strong><p>Filme abgeschlossen</p><small>folgt</small></article><article className="metric-card"><strong>—</strong><p>Serien abgeschlossen</p><small>folgt</small></article><article className="metric-card genre-card"><p>Lieblingsgenre</p><strong>—</strong><small>folgt</small></article></section></div>;
}

function formatDuration(seconds: number) { const hours = Math.floor(seconds / 3600); const minutes = Math.floor((seconds % 3600) / 60); return hours > 0 ? `${hours}h ${minutes}m` : `${minutes}m`; }
function comparisonText(statistics: PersonalStats) { if (statistics.previousWatchSeconds === 0) return "Keine Vergleichsdaten"; const change = Math.round(((statistics.watchSeconds - statistics.previousWatchSeconds) / statistics.previousWatchSeconds) * 100); return `${change >= 0 ? "↑" : "↓"} ${Math.abs(change)} % gegenüber vorher`; }
function Requests() { return <div className="content page-view"><section className="section-heading"><div><p className="eyebrow">SEERR · OFFEN</p><h2>Meine Anfragen</h2></div></section><PosterRow title="" eyebrow="" items={requests} detail={(item) => item.status} /></div>; }
function Profile({ user }: { user: { name: string } }) { return <div className="content page-view profile"><section className="profile-head"><div className="avatar big"><UserAvatar name={user.name} /></div><div><p className="eyebrow">EMBY-PROFIL</p><h2>{user.name}</h2></div></section><button className="logout-button">Abmelden</button></div>; }
