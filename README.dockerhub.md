# QCC Plus

[![Version](https://img.shields.io/badge/version-1.9.4-blue.svg)](https://github.com/yxhpy/qcc_plus/releases/tag/v1.9.4)
[![GitHub](https://img.shields.io/badge/GitHub-yxhpy%2Fqcc__plus-181717?logo=github)](https://github.com/yxhpy/qcc_plus)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](https://github.com/yxhpy/qcc_plus/blob/main/LICENSE)

Multi-tenant Claude Code CLI proxy with admin UI, node failover, monitoring, usage analytics, notifications and Cloudflare Tunnel support.

## Core Features

- multi-tenant account isolation
- per-account node pools and proxy keys
- automatic failover and health checks
- React admin UI
- monitoring dashboard and shared monitor pages
- pricing, usage summary and request logs
- notification channels and subscriptions
- Cloudflare Tunnel integration

## Quick Start

### Docker Compose

```bash
curl -O https://raw.githubusercontent.com/yxhpy/qcc_plus/main/docker-compose.yml
docker compose up -d
```

Open: `http://localhost:8000/admin`

Default admin login:

- username: `admin`
- password: `admin123`

Notes:

- the server now defaults to SQLite storage
- regular default accounts are not auto-created
- create an account and node after first login before calling `/v1/messages`

### Single Container

```bash
docker run -d \
  --name qcc_plus \
  -p 8000:8000 \
  -e UPSTREAM_API_KEY=sk-ant-your-key \
  yxhpy520/qcc_plus:latest
```

## Important Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `LISTEN_ADDR` | Listen address | `:8000` |
| `UPSTREAM_BASE_URL` | Upstream base URL | `https://api.anthropic.com` |
| `UPSTREAM_API_KEY` | Upstream API key | - |
| `PROXY_SQLITE_PATH` | SQLite file path | `~/.qccplus/qccplus.db` |
| `PROXY_MYSQL_DSN` | MySQL DSN | - |
| `ADMIN_API_KEY` | Admin proxy key | `admin` |
| `PROXY_HEALTH_CHECK_MODE` | `cli/api/head` | `cli` |
| `PROXY_HEALTH_CHECK_ALL_INTERVAL` | Full health scan interval | `10m` |
| `TUNNEL_ENABLED` | Enable Tunnel | `false` |

## First API Call

1. login to `/admin`
2. create an account
3. add a node
4. call `/v1/messages` with that account's `x-api-key`

Example:

```bash
curl -c cookies.txt -X POST \
  -d "username=admin&password=admin123" \
  http://localhost:8000/login

curl -b cookies.txt -X POST \
  http://localhost:8000/admin/api/accounts \
  -H "Content-Type: application/json" \
  -d '{"name":"team-alpha","proxy_api_key":"alpha-key","is_admin":false}'

curl -b cookies.txt -X POST \
  http://localhost:8000/admin/api/nodes \
  -H "Content-Type: application/json" \
  -d '{"name":"node-1","base_url":"https://api.anthropic.com","api_key":"sk-ant-your-key","weight":1}'

curl http://localhost:8000/v1/messages \
  -H "x-api-key: alpha-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"claude-sonnet-4-5-20250929","messages":[{"role":"user","content":"Hello"}],"max_tokens":128}'
```

## More Docs

- https://github.com/yxhpy/qcc_plus
- https://github.com/yxhpy/qcc_plus/blob/main/README.md
- https://github.com/yxhpy/qcc_plus/blob/main/docs/README.md
