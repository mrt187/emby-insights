"use client";

import { FormEvent, useState } from "react";

type Features = { requests: boolean; movieDates: boolean; seriesDates: boolean; upcoming: boolean; statistics: boolean };
type User = { id: string; name: string; isAdmin: boolean; features: Features };

export function LoginScreen({ onAuthenticated }: { onAuthenticated: (user: User) => void }) {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      const response = await fetch("/api/auth/emby/login", {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username, password }),
      });
      if (!response.ok) {
        setError(response.status === 401 ? "Benutzername oder Passwort sind nicht korrekt." : "Die Anmeldung ist gerade nicht verfügbar.");
        return;
      }
      onAuthenticated(await response.json() as User);
    } catch {
      setError("Die Verbindung zu Emby Insights konnte nicht hergestellt werden.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="login-shell">
      <section className="login-card" aria-labelledby="login-title">
        <div className="login-brand"><img className="brand-logo" src="/emby-insights-logo.svg" alt="Emby Insights" width="31" height="31" /><span>insights</span></div>
        <p className="eyebrow">DEIN PERSÖNLICHES MEDIEN-DASHBOARD</p>
        <h1 id="login-title">Willkommen zurück.</h1>
        <p className="login-copy">Melde dich mit deinem normalen Emby-Konto an.</p>
        <form onSubmit={submit}>
          <label>Benutzername<input name="username" value={username} onChange={(event) => setUsername(event.target.value)} autoComplete="username" spellCheck={false} required /></label>
          <label>Passwort<input name="password" type="password" value={password} onChange={(event) => setPassword(event.target.value)} autoComplete="current-password" required /></label>
          {error && <p className="login-error" role="alert">{error}</p>}
          <button className="login-button" disabled={submitting}>{submitting ? "Anmeldung läuft …" : "Mit Emby anmelden"}</button>
        </form>
      </section>
    </main>
  );
}
