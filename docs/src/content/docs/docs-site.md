---
title: Docs site maintenance
description: Run, edit, validate, and deploy the Astro documentation site that now lives inside the cf-cli repository.
order: 8
category: Reference
summary: Developer notes for maintaining the embedded Astro docs app alongside the Go CLI.
---

## Local development

Install dependencies and start Astro:

```bash
bun install
bun run dev
```

Astro serves the docs locally, usually at `http://localhost:4321`.

## Files to edit

The docs site is intentionally small:

```bash
src/data/docs.ts              # site name, repo URL, nav, categories
src/pages/index.astro         # landing page
src/pages/docs/index.astro    # docs index
src/pages/docs/[...slug].astro # article route
src/content/docs/*.md         # documentation pages
src/styles/global.css         # visual system
```

For most updates, edit markdown in `src/content/docs` first.

## Validate changes

Run:

```bash
bun run check
bun run build
```

`check` catches Astro and TypeScript issues. `build` verifies the static site output.

## Deployment

The site builds to static output in `dist` and can be deployed by any static host. A typical Vercel setup uses:

```bash
bun run build
```

with output directory:

```bash
dist
```

If you attach a custom domain managed by Cloudflare, use the CLI itself to create the required DNS records.
