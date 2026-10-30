---
title: Wrangler account switching
description: Snapshot and switch local Wrangler auth state separately from Cloudflare API profiles.
order: 6
category: Local Workflows
summary: How cf-cli manages Wrangler's local OAuth/config snapshots for multi-account development.
---

## Separate auth systems

Cloudflare API profiles and Wrangler auth snapshots solve different problems:

- API profiles power `cf dns`, `cf tokens`, `cf workers`, and `cf r2` commands.
- Wrangler snapshots manage local Wrangler OAuth/config state.

That separation lets you script API work while still switching the Wrangler account used by local Worker tooling.

## Common commands

```bash
cf wrangler list
cf wrangler current
cf wrangler add --wrangler-cmd "npx wrangler" --label personal-wrangler
cf wrangler switch personal
cf wrangler login
```

`cf wrangler add` captures the current Wrangler config as a named account snapshot.

## Storage paths

On macOS, Wrangler auth is read from:

```bash
~/Library/Preferences/.wrangler/config/default.toml
```

`cf-cli` stores snapshots under:

```bash
~/.cf-cli/wrangler-auth/accounts/
~/.cf-cli/wrangler-auth/accounts.json
```

Older snapshots under `~/.gg/codex/wrangler-auth/` are copied into the new state directory automatically when needed.

## Switching behavior

Before switching, the CLI re-saves the current Wrangler config if its file hash changed. This helps avoid losing changes made outside `cf-cli`.
