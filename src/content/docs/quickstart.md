---
title: Quickstart
description: Install cf-cli, run your first health check, and make a safe DNS read before changing Cloudflare state.
order: 1
category: Start
summary: The fastest path from install to a working profile and first Cloudflare command.
---

## Install the CLI

Install the latest GitHub release:

```bash
curl -fsSL https://raw.githubusercontent.com/amxv/cf-cli/main/install.sh | bash
cf --help
```

The installer places the `cf` binary in `~/.local/bin` by default. Make sure that directory is on your `PATH` before opening a new shell.

## Add or select a profile

Most operational commands need an explicit profile. Use a profile name that matches the account or project you are operating against:

```bash
cf profiles add personal
cf profiles list
```

Then pass it on every command:

```bash
cf --profile personal doctor
```

You can also set it for a shell session:

```bash
export CF_PROFILE=personal
cf doctor
```

## Run a read-only command first

Before mutating DNS, verify that credentials and account context resolve correctly:

```bash
cf --profile personal doctor
cf --profile personal dns get A @
cf --profile personal dns list TXT
```

`doctor` is the safest first command because it checks how the CLI resolves environment variables, keychain entries, and local profile state.

## Make a DNS change

After the profile is healthy, use the short DNS helpers:

```bash
cf --profile personal dns a @ 203.0.113.10 "Main site"
cf --profile personal dns cname www example.vercel.app
cf --profile personal dns txt verify abc123
```

Use `cf dns get` or `cf dns list` after a change so the terminal history includes both the command and the observed result.
