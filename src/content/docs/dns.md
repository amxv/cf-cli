---
title: DNS records
description: Use the DNS command group to read, create, update, and delete common Cloudflare DNS records from the terminal.
order: 3
category: Cloudflare Operations
summary: Command shapes and examples for A, AAAA, CNAME, TXT, MX, list, get, update, and delete workflows.
---

## Command shape

DNS commands always use the `cf dns <cmd>` shape:

```bash
cf --profile personal dns get A @
cf --profile personal dns list TXT
cf --profile personal dns a @ 203.0.113.10
```

The profile can also come from `CF_PROFILE`.

## Common record helpers

Use helpers for the record types you edit most often:

```bash
cf dns a @ 203.0.113.10 "Main site"
cf dns aaaa @ 2001:db8::10
cf dns cname www app.example.net
cf dns txt verify abc123
cf dns mx @ 10 mx1.mailhost.com
```

The optional final argument is a Cloudflare record comment where supported.

## Read before and after writes

A safe DNS workflow is:

```bash
cf dns get CNAME www
cf dns cname www app.example.net "Vercel app"
cf dns get CNAME www
```

For broad inspection:

```bash
cf dns list
cf dns list TXT
cf dns list CNAME www
```

## Delete carefully

Deletion supports value matching and all-record deletion for a key:

```bash
cf dns delete TXT verify --value abc123
cf dns delete TXT verify --all
```

Prefer `--value` when there may be multiple records with the same type and key.

## Useful flags

```bash
--proxied=true|false
--ttl 3600
--upsert
--priority 10
```

Use `--proxied=false` for verification records and many third-party hosting targets unless the provider explicitly supports Cloudflare proxying.
