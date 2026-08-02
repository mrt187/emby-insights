import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

async function render() {
  const workerUrl = new URL("../dist/server/index.js", import.meta.url);
  workerUrl.searchParams.set("test", `${process.pid}-${Date.now()}`);
  const { default: worker } = await import(workerUrl.href);

  return worker.fetch(
    new Request("http://localhost/", {
      headers: { accept: "text/html" },
    }),
    {
      ASSETS: {
        fetch: async () => new Response("Not found", { status: 404 }),
      },
    },
    {
      waitUntil() {},
      passThroughOnException() {},
    },
  );
}

test("server-renders the login shell", async () => {
  const response = await render();
  assert.equal(response.status, 200);
  assert.match(response.headers.get("content-type") ?? "", /^text\/html\b/i);

  const html = await response.text();
  assert.match(html, /<title>Emby Insights — Dein Medien-Dashboard<\/title>/);
  assert.match(html, /class="login-shell"/);
});

// The document CSP is the only place scripts and images are gated. It arrived
// in 0.9.0 and immediately broke every poster that does not come from Emby,
// because those loaded straight from the artwork CDNs. Those now go through
// /api/artwork, so 'self' is enough again — and naming a CDN here would mean
// the proxy was bypassed somewhere.
test("document carries a nonce CSP that keeps images same-origin", async () => {
  const response = await render();
  const policy = response.headers.get("content-security-policy") ?? "";
  assert.notEqual(policy, "", "no Content-Security-Policy on the document");

  const directive = (name) =>
    policy
      .split(";")
      .map((part) => part.trim())
      .find((part) => part === name || part.startsWith(`${name} `)) ?? "";

  const imgSrc = directive("img-src");
  for (const source of ["'self'", "data:", "blob:"]) {
    assert.ok(imgSrc.includes(source), `img-src is missing ${source}: ${imgSrc}`);
  }
  for (const host of ["image.tmdb.org", "artworks.thetvdb.com", "http:", "https:"]) {
    assert.ok(!imgSrc.includes(host), `img-src should not name ${host}, artwork is proxied: ${imgSrc}`);
  }

  const scriptSrc = directive("script-src");
  assert.match(scriptSrc, /'nonce-[^']+'/, `script-src carries no nonce: ${scriptSrc}`);
  assert.ok(!scriptSrc.includes("'unsafe-inline'"), "script-src must not allow inline script");

  const html = await response.text();
  const nonce = scriptSrc.match(/'nonce-([^']+)'/)[1];
  const scriptTags = html.match(/<script[^>]*>/g) ?? [];
  assert.ok(scriptTags.length > 0, "no script tags rendered");
  for (const tag of scriptTags) {
    assert.ok(tag.includes(`nonce="${nonce}"`), `script tag without the current nonce: ${tag}`);
  }
});

// Ein <img> mit einer URL, die 404 oder 500 liefert, zeigt im Browser das
// kaputte Bild-Icon. Emby antwortet mit 500, wenn die Bilddatei eines Titels
// defekt ist — der Platzhalter wurde bis 0.11.0 aber nur gerendert, wenn gar
// keine URL vorlag, also genau dann nicht, wenn man ihn braucht. Jede
// Poster-Stelle muss den Fehlerfall deshalb selbst abfangen.
test("every poster image falls back to the placeholder when it fails to load", async () => {
  const source = await readFile(new URL("../app/page.tsx", import.meta.url), "utf8");

  // Kommentare raus, sonst schlaegt die Suche auf Prosa an, die <img> erwaehnt.
  const code = source.replace(/^\s*\/\/.*$/gm, "");
  // Bis zum Attributende statt bis zum ersten ">": ein onError-Handler enthaelt
  // mit dem Pfeil selbst ein ">".
  const rawImgTags = code.match(/<img\s[^>]*/g) ?? [];
  const unguarded = rawImgTags.filter((tag) => !tag.includes("onError") && !tag.includes("brand-logo"));
  assert.deepEqual(
    unguarded,
    [],
    `these <img> tags render a broken-image icon when the source fails:\n${unguarded.join("\n")}`,
  );

  assert.match(source, /function PosterImage\(/, "PosterImage is the shared fallback and must exist");
});
