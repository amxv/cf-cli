# Install CF CLI

## Standard Install

Install the latest GitHub release to `~/.local/bin`:

```bash
curl -fsSL https://raw.githubusercontent.com/amxv/cf-cli/main/install.sh | bash
cf --help
```

## Custom Install Directory

Install to a different directory:

```bash
curl -fsSL https://raw.githubusercontent.com/amxv/cf-cli/main/install.sh | CF_INSTALL_DIR="$HOME/bin" bash
```

You can also pass the target directory as the first argument if you download the script first.

## Inspect The Script First

```bash
curl -fsSL -o install.sh https://raw.githubusercontent.com/amxv/cf-cli/main/install.sh
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
go build -o cf .
./cf --help
```

## Installer Behavior

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
