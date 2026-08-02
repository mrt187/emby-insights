import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

// The Go API sets its own Content-Security-Policy on /api/* responses. The
// HTML document is served by the frontend runtime, so it needs its own — and
// it cannot use a plain `script-src 'self'`, because the RSC payload and the
// hydration bootstrap ship as inline <script> blocks. A per-request nonce is
// the way to keep those working without opening the door to injected inline
// script: the runtime stamps the nonce it finds in this header onto every
// script tag it emits, and nothing else on the page can guess it.
//
// styles keep 'unsafe-inline': Tailwind and React both emit inline style
// attributes, which no nonce can cover. That is a far smaller surface than
// inline script and is the same trade-off the API-side policy already makes.
// Artwork hosts the dashboard is allowed to load images from. Extend this if a
// Radarr/Sonarr instance hands back posters from somewhere else — the browser
// console names the blocked host in the CSP violation it reports.
const imageHosts = ["https://image.tmdb.org", "https://artworks.thetvdb.com"];

function contentSecurityPolicy(nonce: string): string {
  return [
    "default-src 'self'",
    // No 'strict-dynamic': it makes browsers ignore 'self', and React emits a
    // <link rel="modulepreload"> of its own that carries no nonce. Keeping
    // 'self' covers that tag; the nonce is what admits the inline blocks.
    `script-src 'self' 'nonce-${nonce}'`,
    "style-src 'self' 'unsafe-inline'",
    // Posters for anything that is not in Emby come straight from the artwork
    // CDNs: Seerr and the coming-soon lists hand out image.tmdb.org URLs, and
    // Radarr/Sonarr hand out their own remoteUrl, which is thetvdb for series.
    // Emby's own posters go through /api/images and are covered by 'self'.
    `img-src 'self' data: blob: ${imageHosts.join(" ")}`,
    "font-src 'self' data:",
    "connect-src 'self'",
    "object-src 'none'",
    "frame-ancestors 'none'",
    "base-uri 'self'",
    "form-action 'self'",
  ].join("; ");
}

export function middleware(request: NextRequest) {
  const nonce = Buffer.from(crypto.randomUUID()).toString("base64");
  const policy = contentSecurityPolicy(nonce);

  // The header has to go back out on the request as well: that is where the
  // renderer reads the nonce from before it emits the first script tag.
  const headers = new Headers(request.headers);
  headers.set("content-security-policy", policy);
  headers.set("x-nonce", nonce);

  const response = NextResponse.next({ request: { headers } });
  response.headers.set("content-security-policy", policy);
  response.headers.set("x-content-type-options", "nosniff");
  response.headers.set("x-frame-options", "DENY");
  response.headers.set("referrer-policy", "same-origin");
  response.headers.set("strict-transport-security", "max-age=31536000; includeSubDomains");
  return response;
}

export const config = {
  matcher: [
    // Everything except /api/* (proxied to the Go API, which sets its own
    // policy — two CSP headers would be intersected and break the page) and
    // the build's own static assets, which need no policy of their own.
    {
      source: "/((?!api/|assets/|_next/|favicon.ico).*)",
      missing: [
        { type: "header", key: "next-router-prefetch" },
        { type: "header", key: "purpose", value: "prefetch" },
      ],
    },
  ],
};
