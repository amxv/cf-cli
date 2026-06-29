---
title: Workers and R2
description: Inspect deployed Workers, read recent persisted logs, enable log collection, and create R2 resources for operational workflows.
order: 5
category: Cloudflare Operations
summary: Practical commands for Workers visibility, R2 bucket setup, credentials, and Logpush helpers.
---

## List Workers

Find deployed Workers by name or filter:

```bash
cf --profile personal workers list
cf --profile personal workers list api
```

Use this before enabling logs or querying a specific Worker so you operate against the right script name.

## Read recent logs

For persisted log access:

```bash
cf --profile personal workers logs my-worker --since 10m --limit 50
cf --profile personal workers logs recent my-worker --since 30m
```

You can choose the log view when supported:

```bash
cf workers logs my-worker --view events
cf workers logs my-worker --view invocations
```

## Enable logs

Enable Workers log collection:

```bash
cf --profile personal workers logs enable my-worker
cf --profile personal workers logs enable my-worker --persist --invocations --sample 1
```

Use explicit flags so future readers can tell whether you meant temporary inspection or persistent collection.

## R2 helpers

Create buckets and mint credentials:

```bash
cf --profile personal r2 bucket create my-workers-log-bucket
cf --profile personal r2 creds mint my-workers-log-bucket
```

Bootstrap a Logpush-style destination for Worker logs:

```bash
cf --profile personal r2 logpush bootstrap my-worker my-workers-log-bucket
```

If you omit the bucket argument, the CLI uses its default naming behavior for the Worker/logging workflow.
