"use client";

import { FormEvent, useState } from "react";
import { type Lang, t } from "./translations";

type Features = { requests: boolean; movieDates: boolean; seriesDates: boolean; upcoming: boolean; statistics: boolean; tracearr: boolean };
type User = { id: string; name: string; isAdmin: boolean; features: Features; language?: Lang };

// `lang` arrives as a prop rather than from LanguageContext: this screen
// renders before the authenticated tree (and its provider) exists, so the root
// component seeds it from the unauthenticated GET /api/language.
export function LoginScreen({ lang, onAuthenticated }: { lang: Lang; onAuthenticated: (user: User) => void }) {
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
        setError(t(lang, response.status === 401 ? "login_error_credentials" : "login_error_unavailable"));
        return;
      }
      onAuthenticated(await response.json() as User);
    } catch {
      setError(t(lang, "login_error_connection"));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="login-shell">
      <section className="login-card" aria-labelledby="login-title">
        <div className="login-brand"><img className="brand-logo" src="/emby-insights-logo.svg" alt="Emby Insights" width="31" height="31" /><span>insights</span></div>
        <p className="eyebrow">{t(lang, "login_eyebrow")}</p>
        <h1 id="login-title">{t(lang, "login_title")}</h1>
        <p className="login-copy">{t(lang, "login_copy")}</p>
        <form onSubmit={submit}>
          <label>{t(lang, "login_username")}<input name="username" value={username} onChange={(event) => setUsername(event.target.value)} autoComplete="username" spellCheck={false} required /></label>
          <label>{t(lang, "login_password")}<input name="password" type="password" value={password} onChange={(event) => setPassword(event.target.value)} autoComplete="current-password" required /></label>
          {error && <p className="login-error" role="alert">{error}</p>}
          <button className="login-button" disabled={submitting}>{t(lang, submitting ? "login_submitting" : "login_submit")}</button>
        </form>
      </section>
    </main>
  );
}
