# Install CF CLI

## Standard Install

Install the latest GitHub release to `~/.local/bin`:

```bash
curl -fsSL https://cf.ashray.xyz/install.sh | bash
cf --help
```

## Custom Install Directory

Install to a different directory:

```bash
curl -fsSL https://cf.ashray.xyz/install.sh | CF_INSTALL_DIR="$HOME/bin" bash
```

You can also pass the target directory as the first argument if you download the script first.

## Inspect The Script First

```bash
curl -fsSL -o install.sh https://cf.ashray.xyz/install.sh
chmod +x install.sh
./install.sh
cf --help
```

## Build From Source

Clone the repository:

```bash
git clone https://github.com/amxv/cf-cli.git
cd cf-cli
```

Build locally:

```bash
go build -o cf ./cmd/cf
./cf --help
```

## Installer Behavior

- The canonical installer is served from `https://cf.ashray.xyz/install.sh` by the documentation site. The repository-root `install.sh` remains available for existing GitHub-based install commands and must stay identical to `docs/public/install.sh`.
- `install.sh` installs the latest published GitHub release by default
- default install directory: `~/.local/bin`
- override install directory with `CF_INSTALL_DIR`
- override binary name with `CF_BINARY_NAME`
- supported installer targets: `linux` and `darwin`, `amd64` and `arm64`

## Releases

Published release assets are here:

```text
https://github.com/amxv/cf-cli/releases
```
