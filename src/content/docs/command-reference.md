---
title: Command reference
description: A compact map of the main cf-cli command groups, arguments, and common examples.
order: 7
category: Reference
summary: A scannable command map for day-to-day Cloudflare operations.
---

## Health and profiles

```bash
cf doctor
cf profiles add personal
cf profiles list
```

## DNS

```bash
cf dns update [domain] [type] [key] [value] [comment]
cf dns set [type] [key] [value] [comment]
cf dns a [key] [ipv4] [comment]
cf dns aaaa [key] [ipv6] [comment]
cf dns cname [key] [target] [comment]
cf dns txt [key] [text] [comment]
cf dns mx [key] [priority] [mail-server] [comment]
cf dns list [type] [key]
cf dns get [type] [key]
cf dns delete [type] [key] [--value value] [--all]
```

## Tokens

```bash
cf tokens dns [name]
cf tokens mint [name]
cf tokens permissions list [filter]
```

## Workers

```bash
cf workers list [filter]
cf workers logs [worker] [--since 10m] [--limit 50] [--view events|invocations] [--search text]
cf workers logs recent [worker] [--since 10m] [--limit 50]
cf workers logs enable [worker] [--persist] [--invocations] [--sample 1]
cf workers logs sink setup-r2 [worker]
```

## R2

```bash
cf r2 bucket create [name]
cf r2 creds mint [bucket]
cf r2 logpush bootstrap [worker] [bucket]
```

## Wrangler

```bash
cf wrangler list
cf wrangler current
cf wrangler add
cf wrangler switch <name-or-id>
cf wrangler login
```
