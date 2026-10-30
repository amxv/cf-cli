---
title: Authentication and profiles
description: Understand how cf-cli resolves Cloudflare credentials from flags, environment variables, macOS keychain services, and local profile metadata.
order: 2
category: Start
summary: A practical guide to the profile-first auth model used across cf-cli commands.
---

## Profile-first behavior

`cf-cli` is designed for multi-account Cloudflare operations. Operational commands require either:

```bash
cf --profile personal doctor
```

or:

```bash
export CF_PROFILE=personal
cf doctor
```

This makes account selection visible and reduces accidental edits against the wrong Cloudflare account.

## Where local state lives

Local profile metadata is stored under:

```bash
~/.cf-cli/
```

The profile registry lives at:

```bash
~/.cf-cli/cloudflare-profiles.json
```

Older state under `~/.gg/codex/` is migrated automatically when the CLI needs it.

## Environment variables

Environment variables work everywhere and are the simplest cross-platform auth path:

```bash
export CF_API_TOKEN=...
export CF_ACCOUNT_ID=...
export CF_ZONE_ID=...
export CF_DOMAIN=example.com
```

Use env vars for CI, servers, or non-macOS machines.

## macOS keychain services

On macOS, the CLI can read secrets from the login keychain. The default service names are profile-scoped:

```bash
<profile> cloudflare api token
<profile> cloudflare bootstrap token
<profile> cloudflare account id
<profile> cloudflare zone id
<profile> cloudflare domain
```

For a `personal` profile, the API token service name is:

```bash
personal cloudflare api token
```

This keeps credentials out of shell history while still making profiles easy to switch.
