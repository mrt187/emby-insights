# Emby Insights — Frontend

Next.js frontend for [Emby Insights](../README.md), built on
[vinext](https://github.com/cloudflare/vinext). Talks to the Go backend
(`../backend`) over `/api/*`; see [../docs/ARCHITECTURE.md](../docs/ARCHITECTURE.md)
for how the pieces fit together.

## Prerequisites

- Node.js `>=22.13.0`

## Quick Start

```bash
npm install
npm run dev
```

The dev server proxies API calls to a locally running backend — see the root
[README](../README.md) and [docker/all-in-one](../docker/all-in-one) for the
full stack.

## Useful Commands

- `npm run dev`: start local development
- `npm run build`: production build
- `npm test`: build and verify the rendered loading skeleton
- `npm run lint`: ESLint

## Notes

- `vite.config.ts` configures the Cloudflare Workers runtime (`@cloudflare/vite-plugin`)
  that vinext builds on; `.openai/hosting.json` toggles its optional D1/R2
  bindings, both left `null` since this project doesn't use them.
- `worker/index.ts` is the Workers entrypoint; `build/sites-vite-plugin.ts` wires
  up static site serving.
