# Cloudflare DNS Updater (CLI Tool)

A CLI tool written in Go using Cobra to update or insert **A records** (or other types) for a domain in Cloudflare via the Cloudflare API.

---

## 🚀 Features

- Update existing DNS records
- Insert (upsert) DNS records if not found
- Add optional comment to records
- Supports TTL and Cloudflare proxy settings
- Reads Cloudflare API token from `--apiToken` flag or `CF_API_TOKEN` environment variable

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

---

## ⚙️ Flags

| Flag         | Default | Description                                  |
|--------------|---------|----------------------------------------------|
| `--proxied`  | true    | Whether the record should be proxied         |
| `--ttl`      | 3600    | Time To Live for the DNS record (in seconds) |
| `--upsert`   | false   | Create the record if it doesn't exist        |

---

## 🔐 Authentication

Set your Cloudflare API token as an environment variable:

```bash
export CF_API_TOKEN=your_token_here
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
