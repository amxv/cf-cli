---
title: API tokens
description: Mint scoped Cloudflare API tokens and discover permission groups without leaving the terminal.
order: 4
category: Cloudflare Operations
summary: Token commands for DNS-specific credentials, custom token minting, and permission lookup.
---

## DNS token shortcut

For common DNS automation, mint a DNS-scoped token:

```bash
cf --profile personal tokens dns my-site-dns
```

Use a descriptive name so the token is recognizable later in Cloudflare dashboards and logs.

## Custom token minting

For broader workflows:

```bash
cf --profile personal tokens mint deploy-worker
```

The CLI is intended to make repeat token creation less error-prone by keeping the account, zone, and profile context explicit.

## Permission discovery

Look up permission groups before designing a token:

```bash
cf tokens permissions list
cf tokens permissions list dns
cf tokens permissions list workers
```

This is useful when you know the operation you want but not the exact Cloudflare permission label.

## Bootstrap token

Some token workflows require a bootstrap token with enough privileges to create narrower tokens. On macOS, the default keychain service is:

```bash
<profile> cloudflare bootstrap token
```

For a `personal` profile:

```bash
personal cloudflare bootstrap token
```

Keep bootstrap credentials more restricted and more carefully stored than the short-lived tokens you mint for specific jobs.
