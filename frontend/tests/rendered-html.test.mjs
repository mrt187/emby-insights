import assert from "node:assert/strict";
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
// because those load straight from the artwork CDNs. These assertions exist so
// that regression cannot happen twice.
test("document carries a nonce CSP that admits the artwork CDNs", async () => {
  const response = await render();
  const policy = response.headers.get("content-security-policy") ?? "";
  assert.notEqual(policy, "", "no Content-Security-Policy on the document");

  const directive = (name) =>
    policy
      .split(";")
      .map((part) => part.trim())
      .find((part) => part === name || part.startsWith(`${name} `)) ?? "";

  const imgSrc = directive("img-src");
  for (const source of ["'self'", "data:", "blob:", "https://image.tmdb.org", "https://artworks.thetvdb.com"]) {
    assert.ok(imgSrc.includes(source), `img-src is missing ${source}: ${imgSrc}`);
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
