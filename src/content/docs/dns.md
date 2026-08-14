---
title: DNS records
description: Use the DNS command group to read, create, update, and delete common Cloudflare DNS records from the terminal.
order: 3
category: Cloudflare Operations
summary: Command shapes and examples for safe create, upsert, JSON, dry-run, and A, AAAA, CNAME, TXT, MX, and SRV workflows.
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
cf dns mx @ 20 mx2.mailhost.com
cf dns srv _sip._tcp 10 5 5060 sip.example.com
```

The optional final argument is a Cloudflare record comment where supported.
MX helpers match existing records by both priority and mail server. This lets
multiple MX records share the same key without one helper call overwriting
another. Repeating an identical helper call updates that exact MX record.

TXT helpers match exact content. SRV helpers use Cloudflare's structured
`priority`, `weight`, `port`, and `target` fields and match all four fields.
TXT, MX, and SRV records are always sent as DNS-only records.

## Create without overwriting

Use `create` when the intent is always to insert a new record. It sends a POST
without searching for an existing type/name pair:

```bash
cf dns create TXT @ "verification=value"
cf dns create MX @ mx2.mailhost.com --priority 20
cf dns create SRV _sip._tcp sip.example.com --priority 10 --weight 5 --port 5060
```

Use the shorter `txt`, `mx`, and `srv` helpers for safe, repeatable upserts.

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
cf dns get SRV _sip._tcp --json
```

`--json` preserves structured SRV data and is the recommended output for
agents and scripts.

## Preview writes

Add `--dry-run` to a write helper to print the exact payload without calling
Cloudflare:

```bash
cf dns mx @ 10 mx1.mailhost.com --dry-run
cf dns srv _sip._tcp 10 5 5060 sip.example.com --dry-run
cf dns create TXT @ "verification=value" --dry-run
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
