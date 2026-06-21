# Cloudflare DNS Updater (CLI Tool)

A CLI tool written in Go using Cobra to update or insert **A records** (or other types) for a domain in Cloudflare via the Cloudflare API.

---

## 🚀 Features

- Update existing DNS records
- Insert (upsert) DNS records if not found
- Mint new zone-scoped DNS API tokens through the Cloudflare API
- Add optional comment to records
- Supports TTL and Cloudflare proxy settings
- Supports record types beyond `A`
- Reads Cloudflare API token, bootstrap token, zone ID, account ID, and default domain from environment variables or the macOS keychain

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
cf set CNAME www app.example.net
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
```

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
