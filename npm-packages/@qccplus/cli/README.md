# @qccplus/cli

CLI installer and launcher for qcc_plus.

[![npm version](https://img.shields.io/npm/v/@qccplus/cli.svg)](https://www.npmjs.com/package/@qccplus/cli)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## What It Does

`@qccplus/cli` installs the correct qcc_plus binary for your platform and exposes the `qccplus` command.

It supports:

- background service management
- foreground proxy mode
- local config file management
- upgrade and version helpers

## Install

```bash
npm install -g @qccplus/cli
```

## Quick Start

```bash
qccplus config init
qccplus config set upstream.api_key sk-ant-your-key
qccplus start
```

Open: `http://localhost:8000/admin`

Default admin login:

- username: `admin`
- password: `admin123`

Note:

- qcc_plus now defaults to SQLite, not in-memory mode.
- Without `mysql.dsn`, the server uses a local SQLite database.
- Regular default accounts are not auto-created; create an account and node after first login.

## Commands

| Command | Description |
|---------|-------------|
| `qccplus start` | Start proxy server in background |
| `qccplus stop` | Stop background server |
| `qccplus restart` | Restart background server |
| `qccplus status` | Show process status |
| `qccplus logs` | Show logs |
| `qccplus logs -f` | Follow logs |
| `qccplus proxy` | Run proxy in foreground |
| `qccplus config init` | Create config file |
| `qccplus config get <key>` | Read config value |
| `qccplus config set <key> <value>` | Update config value |
| `qccplus config list` | Print config |
| `qccplus config edit` | Open config in editor |
| `qccplus config path` | Show config paths |
| `qccplus config reset -y` | Reset config |
| `qccplus service install` | Install system service |
| `qccplus service uninstall` | Remove system service |
| `qccplus service status` | Show service status |
| `qccplus service enable` | Enable boot start |
| `qccplus service disable` | Disable boot start |
| `qccplus upgrade` | Upgrade package |
| `qccplus version` | Show version info |

## Config File

The config file lives at `~/.qccplus/config.yaml`.

Default shape:

```yaml
listen_addr: ':8000'

upstream:
  base_url: 'https://api.anthropic.com'
  api_key: ''
  name: 'default'

proxy:
  retry_max: 3
  fail_threshold: 3
  health_interval_sec: 30

mysql:
  dsn: ''

admin:
  api_key: 'admin'
  default_account: 'default'
  default_proxy_key: 'default-proxy-key'

tunnel:
  enabled: false
  subdomain: ''
  zone: ''
  api_token: ''
```

## Storage Modes

### Default: SQLite

If `mysql.dsn` is empty, the Go server uses SQLite automatically.

### Optional: MySQL

Set:

```bash
qccplus config set mysql.dsn "user:pass@tcp(localhost:3306)/qcc_plus?parseTime=true"
```

## Platform Packages

| Platform | Architecture | Package |
|----------|--------------|---------|
| macOS | arm64 | `@qccplus/darwin-arm64` |
| macOS | x64 | `@qccplus/darwin-x64` |
| Linux | arm64 | `@qccplus/linux-arm64` |
| Linux | x64 | `@qccplus/linux-x64` |
| Windows | x64 | `@qccplus/win32-x64` |

## Environment Variables

| Variable | Description |
|----------|-------------|
| `LISTEN_ADDR` | Server listen address |
| `UPSTREAM_BASE_URL` | Upstream API base URL |
| `UPSTREAM_API_KEY` | Upstream API key |
| `PROXY_SQLITE_PATH` | SQLite file path |
| `PROXY_MYSQL_DSN` | MySQL DSN |
| `ADMIN_API_KEY` | Admin proxy key |

## Repository

https://github.com/yxhpy/qcc_plus
