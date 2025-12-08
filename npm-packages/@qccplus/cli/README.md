# @qccplus/cli

Enterprise-grade Claude Code CLI proxy server - Command Line Interface

[![npm version](https://img.shields.io/npm/v/@qccplus/cli.svg)](https://www.npmjs.com/package/@qccplus/cli)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## Overview

QCC Plus is a powerful proxy server for Claude Code CLI, providing:

- **Zero Dependencies**: No database required! Runs in memory mode by default
- **Multi-Tenant Architecture**: Account isolation with independent node pools
- **Smart Node Routing**: Intelligent load balancing with automatic failover
- **Health Monitoring**: Real-time health checks with auto-recovery
- **Web Management**: React-based admin interface at `http://localhost:8000/admin`

## Installation

```bash
# Using npm
npm install -g @qccplus/cli

# Using yarn
yarn global add @qccplus/cli

# Using pnpm
pnpm add -g @qccplus/cli
```

## Quick Start (3 Steps!)

```bash
# 1. Initialize configuration
qccplus config init

# 2. Set your Anthropic API key
qccplus config set upstream.api_key sk-ant-xxx

# 3. Start the proxy server
qccplus start

# That's it! No database setup required.
# Access admin UI at http://localhost:8000/admin
```

### Configure Claude Code to use your proxy

Add to your Claude Code settings (`~/.claude/settings.json`):

```json
{
  "apiBaseUrl": "http://localhost:8000"
}
```

## Commands

### Core Commands

| Command | Description |
|---------|-------------|
| `qccplus start` | Start proxy server in background |
| `qccplus stop` | Stop the proxy server |
| `qccplus restart` | Restart the proxy server |
| `qccplus status` | Show server status |
| `qccplus logs` | View server logs |
| `qccplus proxy` | Run proxy in foreground |

### Configuration

| Command | Description |
|---------|-------------|
| `qccplus config init` | Initialize config file |
| `qccplus config set <key> <value>` | Set a config value |
| `qccplus config get <key>` | Get a config value |
| `qccplus config list` | List all config values |
| `qccplus config edit` | Open config in editor |
| `qccplus config path` | Show config file paths |
| `qccplus config reset` | Reset to defaults |

### Service Management

| Command | Description |
|---------|-------------|
| `qccplus service install` | Install as system service |
| `qccplus service uninstall` | Remove system service |
| `qccplus service status` | Show service status |
| `qccplus service enable` | Enable auto-start |
| `qccplus service disable` | Disable auto-start |

### Other Commands

| Command | Description |
|---------|-------------|
| `qccplus version` | Show detailed version info |
| `qccplus upgrade` | Upgrade to latest version |
| `qccplus --help` | Show help |

## Configuration

Configuration is stored in `~/.qccplus/config.yaml`:

```yaml
listen_addr: ':8000'

upstream:
  base_url: 'https://api.anthropic.com'
  api_key: 'sk-ant-xxx'  # Your Anthropic API key
  name: 'default'

proxy:
  retry_max: 3
  fail_threshold: 3
  health_interval_sec: 30

# MySQL is OPTIONAL! Leave empty for memory mode (default)
# Only needed for: production multi-instance, data persistence across restarts
mysql:
  dsn: ''  # Empty = memory mode (no database required!)

admin:
  api_key: 'admin'
  default_account: 'default'
  default_proxy_key: 'default-proxy-key'

# Cloudflare Tunnel (optional, for exposing to internet)
tunnel:
  enabled: false
  subdomain: ''
  zone: ''
  api_token: ''
```

## Storage Modes

### Memory Mode (Default) ✅

- **No setup required** - just start and use
- Perfect for single-user/development use
- All data stored in memory
- Data resets on restart (but that's usually fine for personal use)

### MySQL Mode (Optional)

Only needed if you want:
- Data persistence across restarts
- Multi-instance deployment with shared state
- Production multi-tenant environment

```bash
# Enable MySQL persistence
qccplus config set mysql.dsn "user:pass@tcp(localhost:3306)/qccplus"
```

## Platform Support

The CLI automatically installs the correct binary for your platform:

| Platform | Architecture | Package |
|----------|--------------|---------|
| macOS | Apple Silicon (M1/M2/M3) | `@qccplus/darwin-arm64` |
| macOS | Intel | `@qccplus/darwin-x64` |
| Linux | ARM64 | `@qccplus/linux-arm64` |
| Linux | x64 | `@qccplus/linux-x64` |
| Windows | x64 | `@qccplus/win32-x64` |

## Usage Examples

### Start with Custom Port

```bash
qccplus start --port 9000
```

### Run with Upstream Override

```bash
qccplus start --upstream https://my-proxy.example.com --api-key sk-xxx
```

### Install as System Service (Linux)

```bash
# System-wide (requires root)
sudo qccplus service install

# User service (no root required)
qccplus service install --user
```

### Install as System Service (macOS)

```bash
# System-wide (requires root)
sudo qccplus service install

# User agent
qccplus service install --user
```

### Check for Updates

```bash
qccplus upgrade --check
```

### View Logs in Real-time

```bash
qccplus logs -f
```

## Environment Variables

The CLI respects the following environment variables:

| Variable | Description |
|----------|-------------|
| `LISTEN_ADDR` | Server listen address |
| `UPSTREAM_BASE_URL` | Upstream API base URL |
| `UPSTREAM_API_KEY` | Upstream API key |
| `PROXY_MYSQL_DSN` | MySQL connection string (optional) |
| `ADMIN_API_KEY` | Admin API key |

## Troubleshooting

### Binary Not Found

If you see "Binary not found" error:

```bash
# Reinstall the package
npm install -g @qccplus/cli

# Or install platform package manually
npm install -g @qccplus/darwin-arm64  # for M1/M2/M3 Mac
```

### Permission Denied

For service installation on Linux:

```bash
# Use sudo for system service
sudo qccplus service install

# Or install as user service
qccplus service install --user
```

### Check Version and Path

```bash
qccplus version
```

## Documentation

- [GitHub Repository](https://github.com/yxhpy/qcc_plus)
- [Docker Hub](https://hub.docker.com/r/yxhpy520/qcc_plus)
- [Full Documentation](https://github.com/yxhpy/qcc_plus/tree/main/docs)

## License

MIT License - see [LICENSE](https://github.com/yxhpy/qcc_plus/blob/main/LICENSE) for details.
