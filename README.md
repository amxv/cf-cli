# Cloudflare DNS Updater (CLI Tool)

A CLI tool written in Go using Cobra to update or insert **A records** (or other types) for a domain in Cloudflare via the Cloudflare API.

---

## 🚀 Features

- Update existing DNS records
- Insert (upsert) DNS records if not found
- Mint new zone-scoped DNS API tokens through the Cloudflare API
- List deployed Workers for the current account
- View recent persisted Worker logs from Cloudflare observability
- Enable persisted Worker logs and invocation logs from the CLI
- Create account-level Workers Logpush jobs that export to R2
- Add optional comment to records
- Supports TTL and Cloudflare proxy settings
- Supports record types beyond `A`
- Reads Cloudflare API token, bootstrap token, zone ID, account ID, default domain, and optional R2 log sink credentials from environment variables or the macOS keychain

---

## 🔧 Installation

1. Clone the repo:
   ```bash
   git clone https://github.com/sagar290/cf.git
   cd cloudflare-dns-updater
   ```

2. Build the binary:
   ```bash
   go build -o cf
   ```

## 📦 Download Binaries

Prebuilt binaries for all major platforms are available on the [Releases](https://github.com/sagar290/cf/releases) page.

| Platform      | File Name                   |
|---------------|-----------------------------|
| Linux (amd64) | `cf-linux-amd64`            |
| Linux (arm64) | `cf-linux-arm64`            |
| macOS         | `cf-darwin-amd64`           |
| Windows       | `cf-windows-amd64.exe`      |
| Windows (ARM) | `cf-windows-arm64.exe`      |

---

## 📥 Quick Download Example (Linux)

```bash
curl -L -o cf https://github.com/sagar290/cf/releases/latest/download/cf-linux-amd64
chmod +x cf
./cf --help
```

---

## ✅ Usage

```bash
cf update:dns [domain] [type] [key] [value] [comment (optional)]
```

```bash
cf mint:dns-token [name]
```

```bash
cf set [type] [key] [value] [comment]
cf a [key] [ipv4] [comment]
cf aaaa [key] [ipv6] [comment]
cf cname [key] [target] [comment]
cf txt [key] [text] [comment]
cf mx [key] [priority] [mail-server] [comment]
cf list [type] [key]
cf get [type] [key]
cf delete [type] [key] [--value value] [--all]
cf workers:list [filter]
cf worker:logs [worker] [--since 1h] [--limit 20] [--view events|invocations] [--search text]
cf worker logs [worker] [--since 10m] [--limit 50] [--view events|invocations] [--search text]
cf worker logs recent [worker] [--since 10m] [--limit 50]
cf worker logs enable [worker] [--persist] [--invocations] [--sample 1]
cf worker logs sink setup-r2 [worker]
cf doctor
```

### Arguments:

| Argument | Description                                  | Required |
|----------|----------------------------------------------|----------|
| domain   | Your domain name (e.g., example.com)         | ✅       |
| type     | DNS record type (e.g., A, CNAME)             | ✅       |
| key      | DNS key to update (e.g., `@`, `www`)         | ✅       |
| value    | New value (e.g., IP address or CNAME target) | ✅       |
| comment  | Optional comment for the DNS record          | ❌       |

---

### 🔁 Example

```bash
cf update:dns example.com A @ 123.123.123.123 "Main site IP"
```

This updates or inserts the A record for `example.com` with a comment.

Agent-friendly shortcuts:

```bash
cf a @ 123.123.123.123
cf aaaa @ 2001:db8::10
cf set CNAME www app.example.net
cf txt verify abc123
cf mx @ 10 mx1.mailhost.com
cf list TXT
cf get CNAME www
cf delete TXT verify --value abc123
cf workers:list
cf worker:logs api-worker --since 2h --limit 25
cf worker logs api-worker --since 10m --limit 50
cf worker logs enable api-worker
cf doctor
```

---

## ⚙️ Flags

| Flag         | Default | Description                                  |
|--------------|---------|----------------------------------------------|
| `--profile` | `ama` | Profile prefix for keychain lookups |
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

Set your Cloudflare API token as an environment variable:

```bash
export CF_API_TOKEN=your_token_here
```

Or store the active token in the macOS keychain and let the CLI load it automatically:

```bash
security add-generic-password -U -a 'cloudflare-dns-cli' -s 'ama cloudflare api token' -w 'your_token_here' ~/Library/Keychains/login.keychain-db
```

For token minting, the CLI also looks for:

```bash
security add-generic-password -U -a 'cloudflare-dns-cli' -s 'ama cloudflare bootstrap token' -w 'your_bootstrap_token_here' ~/Library/Keychains/login.keychain-db
security add-generic-password -U -a 'cloudflare-dns-cli' -s 'ama cloudflare zone id' -w 'your_zone_id_here' ~/Library/Keychains/login.keychain-db
security add-generic-password -U -a 'cloudflare-dns-cli' -s 'ama cloudflare account id' -w 'your_account_id_here' ~/Library/Keychains/login.keychain-db
security add-generic-password -U -a 'cloudflare-dns-cli' -s 'ama cloudflare domain' -w 'your_domain_here' ~/Library/Keychains/login.keychain-db
```

Equivalent environment variables are:

```bash
export CF_BOOTSTRAP_TOKEN=your_bootstrap_token_here
export CF_ZONE_ID=your_zone_id_here
export CF_ACCOUNT_ID=your_account_id_here
export CF_DOMAIN=your_domain_here
```

## Fastest Path

For agents, the shortest workflow is:

```bash
cf doctor
cf mint:dns-token
cf a @ 123.123.123.123
cf set CNAME www target.example.net
cf list
cf delete TXT old-verification --all
```

## Workers Logs

This CLI reads the same persisted Workers observability dataset that powers Cloudflare's dashboard-style recent logs view. It is not a real-time tail. For the common agent workflow of "show me the last 5-10 minutes of logs", use persisted observability first instead of forcing everything through R2.

List deployed Workers:

```bash
cf workers:list
cf workers:list upload
```

View recent persisted logs for one Worker:

```bash
cf worker logs my-worker
cf worker logs my-worker --since 10m --limit 50
cf worker logs recent my-worker --view invocations
cf worker logs recent my-worker --search timeout
```

Enable persisted logs and invocation logs for a Worker:

```bash
cf worker logs enable my-worker
cf worker logs enable my-worker --logpush
```

If your active token is DNS-only, mint and activate a Workers-readable account token:

```bash
cf mint:token "ama workers logs" --preset workers-logs-read --store --activate
```

If you also want to enable logs or create Logpush jobs, mint and activate the admin preset instead:

```bash
cf mint:token "ama workers logs admin" --preset workers-logs-admin --store --activate
```

The `workers-logs-read` preset includes:

- `Workers Scripts Read`
- `Workers Observability Read`
- `Workers Observability Telemetry Write`

The `workers-logs-admin` preset adds:

- `Workers Scripts Write`
- `Workers Observability Write`
- `Logs Write`

## Workers Logpush To R2

Use this when you want exported trace-event logs in R2 in addition to the built-in recent-log query flow.

Create or verify an account-level `workers_trace_events` Logpush job to R2 and enable the target Worker's `logpush` flag:

```bash
cf worker logs sink setup-r2 my-worker
```

You can also provide the R2 sink details explicitly:

```bash
cf worker logs sink setup-r2 my-worker \
  --bucket my-log-bucket \
  --path workers-trace-events \
  --r2-access-key-id "$CF_R2_ACCESS_KEY_ID" \
  --r2-secret-access-key "$CF_R2_SECRET_ACCESS_KEY"
```

The CLI resolves the R2 sink details from these environment variables or matching macOS keychain entries:

```bash
export CF_R2_LOG_BUCKET=my-log-bucket
export CF_R2_ACCESS_KEY_ID=...
export CF_R2_SECRET_ACCESS_KEY=...
export CF_WORKERS_LOGPUSH_PATH=workers-trace-events
```

Expected keychain services for the current profile:

```bash
security add-generic-password -U -a 'cloudflare-dns-cli' -s 'ama cloudflare r2 log bucket' -w 'my-log-bucket' ~/Library/Keychains/login.keychain-db
security add-generic-password -U -a 'cloudflare-dns-cli' -s 'ama cloudflare r2 access key id' -w 'your_r2_access_key_id' ~/Library/Keychains/login.keychain-db
security add-generic-password -U -a 'cloudflare-dns-cli' -s 'ama cloudflare r2 secret access key' -w 'your_r2_secret_access_key' ~/Library/Keychains/login.keychain-db
security add-generic-password -U -a 'cloudflare-dns-cli' -s 'ama cloudflare workers logpush path' -w 'workers-trace-events' ~/Library/Keychains/login.keychain-db
```

Notes:

- `cf worker logs ...` and `cf worker:logs ...` both use the persisted Workers observability query API, which is the best path for "show me the last 5-10 minutes".
- `cf worker logs enable ...` uses the Worker script settings API and requires broader Worker permissions than the read-only recent-log flow.
- `cf worker logs sink setup-r2 ...` uses the account-level Logpush API and requires `Logs Write` plus valid R2 credentials.
- These Worker commands are account-level, so they use `CF_ACCOUNT_ID` or the configured account ID keychain entry in addition to the API token.

## 🔁 Mint A New DNS Token

Mint a fresh zone-scoped token and store it as the active API token for the current profile:

```bash
./cf mint:dns-token
```

Mint one with a custom display name and expiry:

```bash
./cf mint:dns-token "ama bot token" --expires-on 2026-12-31T23:59:59Z
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
