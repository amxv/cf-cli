# CF CLI

`cf` is a practical Cloudflare operations CLI. It is built for fast DNS edits, token minting, recent Workers log access, R2 helpers, and local multi-account workflows.

## What It Covers

- `cf dns ...` for DNS records
- `cf tokens ...` for scoped token minting and permission lookup
- `cf workers ...` for deployed Worker listing and persisted log access
- `cf r2 ...` for R2 buckets, credentials, and Logpush helpers
- `cf profiles ...` for local API profile discovery
- `cf wrangler ...` for local Wrangler auth snapshot and switching
- `cf doctor` for env/keychain resolution checks

## Key Behaviors

- DNS commands always use the `cf dns <cmd>` shape
- operational commands require `--profile <name>` or `CF_PROFILE=<name>`
- environment variables work everywhere
- macOS users can also use the login keychain for secret storage
- local CLI state lives under `~/.cf-cli/`
- older local state under `~/.gg/codex/` is migrated automatically when needed

## Quick Start

Clone the canonical repository:

```bash
git clone https://github.com/amxv/cf-cli.git
cd cf-cli
```

Install to `~/.local/bin` by default:

```bash
./install.sh
cf --help
```

Or build locally without installing:

```bash
go build -o cf .
./cf --help
```

`install.sh` accepts an optional target directory as its first argument, or you can set `CF_INSTALL_DIR`. The installed binary name defaults to `cf`, and can be overridden with `CF_BINARY_NAME`.

This repository does not assume published release binaries are available. Build from source unless this repo's Releases page says otherwise.

## Fast Start

```bash
cf --profile personal doctor
cf --profile personal dns get A @
cf --profile personal dns a @ 203.0.113.10
cf --profile personal dns txt verify abc123
cf --profile personal workers logs my-worker --since 10m --limit 50
```

## Core Syntax

The CLI is organized by top-level product area. DNS commands always use the `cf dns <cmd>` shape.

## ✅ Usage

```bash
cf dns update [domain] [type] [key] [value] [comment (optional)]
cf dns set [type] [key] [value] [comment]
cf dns a [key] [ipv4] [comment]
cf dns aaaa [key] [ipv6] [comment]
cf dns cname [key] [target] [comment]
cf dns txt [key] [text] [comment]
cf dns mx [key] [priority] [mail-server] [comment]
cf dns list [type] [key]
cf dns get [type] [key]
cf dns delete [type] [key] [--value value] [--all]

cf tokens dns [name]
cf tokens mint [name]
cf tokens permissions list [filter]

cf workers list [filter]
cf workers logs [worker] [--since 10m] [--limit 50] [--view events|invocations] [--search text]
cf workers logs recent [worker] [--since 10m] [--limit 50]
cf workers logs enable [worker] [--persist] [--invocations] [--sample 1]
cf workers logs sink setup-r2 [worker]

cf r2 bucket create [name]
cf r2 creds mint [bucket]
cf r2 logpush bootstrap [worker] [bucket(optional)]

cf profiles list
cf wrangler list
cf doctor
```

### DNS Arguments

| Argument | Description                                  | Required |
|----------|----------------------------------------------|----------|
| domain   | Your domain name (e.g., example.com)         | ✅       |
| type     | DNS record type (e.g., A, CNAME)             | ✅       |
| key      | DNS key to update (e.g., `@`, `www`)         | ✅       |
| value    | New value (e.g., IP address or CNAME target) | ✅       |
| comment  | Optional comment for the DNS record          | ❌       |

---

### DNS Example

```bash
cf dns update example.com A @ 123.123.123.123 "Main site IP"
```

This updates or inserts the A record for `example.com` with a comment.

Common DNS tasks:

```bash
cf dns a @ 123.123.123.123
cf dns aaaa @ 2001:db8::10
cf dns set CNAME www app.example.net
cf dns txt verify abc123
cf dns mx @ 10 mx1.mailhost.com
cf dns list TXT
cf dns get CNAME www
cf dns delete TXT verify --value abc123
```

## Common Non-DNS Tasks

```bash
cf --profile personal tokens dns
cf --profile personal workers list
cf --profile personal workers logs api-worker --since 10m --limit 50
cf --profile personal workers logs enable api-worker
cf --profile personal r2 bucket create my-workers-log-bucket
cf profiles list
cf wrangler list
cf --profile personal doctor
```

## ⚙️ Flags

| Flag         | Default | Description                                  |
|--------------|---------|----------------------------------------------|
| `--profile` | none | Required profile prefix for keychain lookups unless `CF_PROFILE` is set |
| `--api-token` | `""`   | Cloudflare API token                         |
| `--api-token-keychain-service` | `"<profile> cloudflare api token"` | macOS keychain service name for the API token |
| `--bootstrap-token-keychain-service` | `"<profile> cloudflare bootstrap token"` | macOS keychain service name for the bootstrap token |
| `--zone-id-keychain-service` | `"<profile> cloudflare zone id"` | macOS keychain service name for the zone ID |
| `--account-id-keychain-service` | `"<profile> cloudflare account id"` | macOS keychain service name for the account ID |
| `--domain-keychain-service` | `"<profile> cloudflare domain"` | macOS keychain service name for the default domain |
| `--proxied`  | true    | Whether the record should be proxied         |
| `--ttl`      | 3600    | Time To Live for the DNS record (in seconds) |
| `--upsert`   | false   | Create the record if it doesn't exist        |
| `--priority` | `0`     | Priority for MX records                      |

---

## 🔐 Authentication

The CLI supports environment variables everywhere. On macOS it can also read and write secrets through the login keychain.

## Profiles

Profiles are explicit. Every operational command requires either `--profile <name>` or `CF_PROFILE=<name>`.

You can see locally known profiles with:

```bash
cf profiles list
```

You can also register a profile name locally without touching the keychain:

```bash
cf profiles add personal
```

The CLI stores this local profile registry at:

```bash
~/.cf-cli/cloudflare-profiles.json
```

If you used an older build that stored local state under `~/.gg/codex/`, the CLI automatically copies that registry into `~/.cf-cli/` the first time it needs it.

This registry is only for discovery and convenience. Actual credentials and IDs still come from environment variables or macOS keychain services such as:

```bash
<profile> cloudflare api token
<profile> cloudflare bootstrap token
<profile> cloudflare account id
<profile> cloudflare zone id
<profile> cloudflare domain
```

## Wrangler Auth Switching

This is separate from the Cloudflare API profiles above.

Use the `wrangler` command group to snapshot and switch the local Wrangler OAuth/config state stored in Wrangler's `default.toml`.

Available commands:

```bash
cf wrangler list
cf wrangler current
cf wrangler add
cf wrangler switch <name-or-id>
cf wrangler login
```

Examples:

```bash
cf wrangler add --wrangler-cmd "npx wrangler" --label personal-wrangler
cf wrangler list
cf wrangler current
cf wrangler switch personal
```

How it works:

- Wrangler auth is currently read from `~/Library/Preferences/.wrangler/config/default.toml` on macOS.
- This CLI stores snapshots in `~/.cf-cli/wrangler-auth/accounts/`.
- The local Wrangler account database is stored at `~/.cf-cli/wrangler-auth/accounts.json`.
- Before switching, the CLI automatically re-saves the current Wrangler config if the file hash changed.
- If you used an older build that stored Wrangler snapshots under `~/.gg/codex/wrangler-auth/`, the CLI automatically copies them into `~/.cf-cli/wrangler-auth/`.

If Wrangler is not in `PATH`, set:

```bash
export CF_WRANGLER_CMD="npx wrangler"
```

or pass:

```bash
cf wrangler add --wrangler-cmd "npx wrangler"
```

Set your Cloudflare API token as an environment variable:

```bash
export CF_API_TOKEN=your_token_here
```

Or store the active token in the macOS keychain and let the CLI load it automatically:

```bash
security add-generic-password -U -a 'cloudflare-dns-cli' -s 'personal cloudflare api token' -w 'your_token_here' ~/Library/Keychains/login.keychain-db
```

For token minting, the CLI also looks for:

```bash
security add-generic-password -U -a 'cloudflare-dns-cli' -s 'personal cloudflare bootstrap token' -w 'your_bootstrap_token_here' ~/Library/Keychains/login.keychain-db
security add-generic-password -U -a 'cloudflare-dns-cli' -s 'personal cloudflare zone id' -w 'your_zone_id_here' ~/Library/Keychains/login.keychain-db
security add-generic-password -U -a 'cloudflare-dns-cli' -s 'personal cloudflare account id' -w 'your_account_id_here' ~/Library/Keychains/login.keychain-db
security add-generic-password -U -a 'cloudflare-dns-cli' -s 'personal cloudflare domain' -w 'your_domain_here' ~/Library/Keychains/login.keychain-db
```

Equivalent environment variables are:

```bash
export CF_BOOTSTRAP_TOKEN=your_bootstrap_token_here
export CF_ZONE_ID=your_zone_id_here
export CF_ACCOUNT_ID=your_account_id_here
export CF_DOMAIN=your_domain_here
```

## Fastest Path

For agents, the shortest DNS workflow is:

```bash
cf --profile <name> doctor
cf --profile <name> tokens dns
cf --profile <name> dns a @ 123.123.123.123
cf --profile <name> dns set CNAME www target.example.net
cf --profile <name> dns list
cf --profile <name> dns delete TXT old-verification --all
```

## Workers Logs

This CLI reads the same persisted Workers observability dataset that powers Cloudflare's dashboard-style recent logs view. It is not a real-time tail. For the common agent workflow of "show me the last 5-10 minutes of logs", use persisted observability first instead of forcing everything through R2.

List deployed Workers:

```bash
cf workers list
cf workers list upload
```

View recent persisted logs for one Worker:

```bash
cf workers logs my-worker
cf workers logs my-worker --since 10m --limit 50
cf workers logs recent my-worker --view invocations
cf workers logs recent my-worker --search timeout
```

Enable persisted logs and invocation logs for a Worker:

```bash
cf workers logs enable my-worker
cf workers logs enable my-worker --logpush
```

If your active token is DNS-only, mint and activate a Workers-readable account token:

```bash
cf tokens mint "personal workers logs" --preset workers-logs-read --store --activate
```

If you also want to enable logs or create Logpush jobs, mint and activate the admin preset instead:

```bash
cf tokens mint "personal workers logs admin" --preset workers-logs-admin --store --activate
```

The `workers-logs-read` preset includes:

- `Workers Scripts Read`
- `Workers Observability Read`
- `Workers Observability Telemetry Write`

The `workers-logs-admin` preset adds:

- `Workers Scripts Write`
- `Workers Observability Write`
- `Logs Write`

If your active token is too narrow, `cf workers logs enable ...` will automatically mint a temporary `workers-logs-admin` helper token from the configured bootstrap token and retry once.

## Workers Logpush To R2

Use this when you want exported trace-event logs in R2 in addition to the built-in recent-log query flow.

Create or verify an account-level `workers_trace_events` Logpush job to R2 and enable the target Worker's `logpush` flag:

```bash
cf workers logs sink setup-r2 my-worker
```

You can also provide the R2 sink details explicitly:

```bash
cf workers logs sink setup-r2 my-worker \
  --bucket my-workers-log-bucket \
  --path workers-trace-events \
  --r2-access-key-id "$CF_R2_ACCESS_KEY_ID" \
  --r2-secret-access-key "$CF_R2_SECRET_ACCESS_KEY"
```

The CLI resolves the R2 sink details from these environment variables or matching macOS keychain entries:

```bash
export CF_R2_LOG_BUCKET=my-workers-log-bucket
export CF_R2_ACCESS_KEY_ID=...
export CF_R2_SECRET_ACCESS_KEY=...
export CF_WORKERS_LOGPUSH_PATH=workers-trace-events
```

Expected keychain services for the current profile:

```bash
security add-generic-password -U -a 'cloudflare-dns-cli' -s 'personal cloudflare r2 log bucket' -w 'my-workers-log-bucket' ~/Library/Keychains/login.keychain-db
security add-generic-password -U -a 'cloudflare-dns-cli' -s 'personal cloudflare r2 access key id' -w 'your_r2_access_key_id' ~/Library/Keychains/login.keychain-db
security add-generic-password -U -a 'cloudflare-dns-cli' -s 'personal cloudflare r2 secret access key' -w 'your_r2_secret_access_key' ~/Library/Keychains/login.keychain-db
security add-generic-password -U -a 'cloudflare-dns-cli' -s 'personal cloudflare workers logpush path' -w 'workers-trace-events' ~/Library/Keychains/login.keychain-db
```

Notes:

- `cf workers logs ...` uses the persisted Workers observability query API, which is the best path for "show me the last 5-10 minutes".
- `cf workers logs enable ...` uses the Worker script settings API and requires broader Worker permissions than the read-only recent-log flow.
- `cf workers logs sink setup-r2 ...` uses the account-level Logpush API and requires `Logs Write` plus valid R2 credentials.
- If your active token is too narrow, `cf workers logs sink setup-r2 ...` will automatically mint a temporary `workers-r2-logpush-admin` helper token from the configured bootstrap token and retry once.
- Cloudflare also enforces account-level Logpush access separately from Workers permissions. If the command still fails with `10000: Authentication error`, the account member backing the token likely needs `Administrator`, `Super Administrator`, or `Log Share` edit access in the Cloudflare dashboard.
- These Worker commands are account-level, so they use `CF_ACCOUNT_ID` or the configured account ID keychain entry in addition to the API token.

## R2 Helpers

Create or reuse an R2 bucket:

```bash
cf r2 bucket create my-workers-trace-events
```

If the bucket already exists and you own it, the command treats that as a reusable success path.

Mint bucket-scoped R2 S3 credentials:

```bash
cf r2 creds mint my-workers-trace-events
```

This returns:

- `access_key_id`: the Cloudflare token ID
- `secret_access_key`: the SHA-256 hash of the minted token value
- `endpoint`: the R2 S3 endpoint for the account and jurisdiction

Bootstrap the full Worker Logpush R2 path in one command:

```bash
cf r2 logpush bootstrap my-worker my-workers-trace-events
```

Useful presets:

```bash
cf tokens mint "personal r2 admin" --preset r2-admin --store
cf tokens mint "personal workers+r2 admin" --preset workers-r2-logpush-admin --store --activate
```

The `r2-admin` preset includes:

- `Workers R2 Storage Read`
- `Workers R2 Storage Write`

The `workers-r2-logpush-admin` preset combines:

- Worker observability/logging management permissions
- `Logs Write`
- `Workers R2 Storage Read`
- `Workers R2 Storage Write`

## 🔁 Mint A New DNS Token

Mint a fresh zone-scoped token and store it as the active API token for the current profile:

```bash
./cf tokens dns
```

Mint one with a custom display name and expiry:

```bash
./cf tokens dns "personal bot token" --expires-on 2026-12-31T23:59:59Z
```

---

## 📦 Sample Output

```
✅ Inserted www to 123.123.123.123
Response: {...}
```
or
```
✅ Updated www to 123.123.123.123
Response: {...}
```

---

## 📝 Notes

- If `--upsert` is not enabled, a missing record will cause an error.
- Use `@` to target the root domain.
- Comments are supported and visible in Cloudflare dashboard.

---



## 📄 License

This project is licensed under the [MIT License](LICENSE).
