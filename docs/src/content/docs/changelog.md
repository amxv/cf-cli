---
title: Changelog
description: "Release notes for cf-cli."
order: 99
category: Reference
summary: Version-by-version changes for the Cloudflare CLI.
---

This changelog tracks code and product changes in cf-cli. It intentionally skips docs-site-only updates.

## 0.1.2 — 2026-06-24

- Changed the installer to fetch the latest release binary by default.
- Documented the release-based installation path.

## 0.1.1 — 2026-06-24

- Fixed release workflow permissions so published builds can complete correctly.

## 0.1.0 — 2026-06-24

- Imported the original Cloudflare CLI foundation.
- Added agent-friendly Cloudflare DNS automation.
- Added DNS list, get, delete, and shortcut commands.
- Added generic Cloudflare API token minting.
- Added Workers log control and recent-log workflows.
- Added R2 token helpers and logpush bootstrap helpers.
- Improved Workers log setup and R2 handling.
- Required explicit Cloudflare profile selection.
- Added a local Cloudflare profile registry.
- Added Wrangler auth switching commands.
- Reorganized the CLI into top-level product areas.
- Added a built-in CLI skill guide.
- Made `wrangler add` fail fast and show progress.
- Prepared the repository for OSS usage.
- Renamed the module and product to cf-cli.
