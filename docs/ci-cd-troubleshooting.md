# CI/CD Troubleshooting Guide

## Overview
This document covers common CI/CD deployment issues and their solutions for the qcc_plus project.

## Health Check Timeout (Exit Code 28)

### Symptoms
- GitHub Actions workflow fails at "Health check" step
- Error: `exit code 28` (curl timeout)
- Message: Health check failed after N attempts

### Root Causes

1. **Service Not Started**
   - Docker container failed to start
   - Application crashed during startup
   - Port binding conflict

2. **Network Issues**
   - Firewall blocking the port
   - Server not accessible from GitHub runner
   - DNS resolution issues

3. **Timing Issues**
   - Service needs more time to initialize
   - Race condition between deploy completion and health check

### Solution Applied (v1.0.1+)

The health check has been enhanced with:
- **10-second initial wait** for service stabilization
- **6 retry attempts** with 5-second delays (total: ~40 seconds)
- **Explicit timeouts**: 10s connect timeout, 20s max time
- **Detailed diagnostics** on failure

### Manual Verification

If the health check fails, SSH into the server and run:

```bash
# Check if container is running
docker ps -a | grep qcc_test-proxy

# Check container logs
docker logs qcc_test-proxy-1

# Check port binding
netstat -tlnp | grep 8001

# Manual curl test
curl -v http://localhost:8001/

# Check firewall (Ubuntu/Debian)
sudo ufw status
sudo ufw allow 8001/tcp
```

### Configuration

#### Test Environment
- Branch: `test`
- Port: `8001`
- Container: `qcc_test-proxy-1`
- Compose file: `docker-compose.test.yml`

#### Production Environment
- Deploy entry: `deploy-prod.yml` `workflow_dispatch`
- Ref: `prod` by default, also supports explicit branch/tag
- Port: `8000`
- Container: `qcc_prod-proxy-1`
- Compose file: `docker-compose.prod.yml`

## Deploy Script Issues

### npm ci Failures

**Symptom**: `npm ci` fails during frontend build

**Solution**: The deploy script includes automatic recovery:
1. Tries `npm ci` first
2. On failure, cleans `node_modules` and npm cache
3. Retries with clean `npm ci`

**Manual fix**:
```bash
cd /opt/qcc_plus/frontend
rm -rf node_modules
npm cache clean --force
npm ci
```

### Docker Compose Not Found

**Symptom**: `docker compose is not available on this host`

**Solution**: Install Docker Compose v2
```bash
# For Ubuntu/Debian
sudo apt-get update
sudo apt-get install docker-compose-plugin

# Verify
docker compose version
```

### Git Ref Checkout Issues

**Symptom**: production manual deploy fails with `git ref ... not found on origin as a branch or tag`

**Solution**: the deploy script now resets local changes, fetches tags, and checks out the requested branch/tag. Confirm the `workflow_dispatch` input really exists on `origin`:
```bash
cd /opt/qcc_plus
git fetch --force --prune origin --tags
git branch -r --list 'origin/prod'
git tag --list 'v1.2.0*'

# Re-run with an existing ref
./scripts/deploy-server.sh prod v1.2.0
```

## GitHub Actions Secrets

Ensure these secrets are configured in GitHub repository settings:

### Test Environment
- `TEST_HOST`: Server hostname or IP
- `TEST_SSH_USER`: SSH username
- `TEST_SSH_KEY`: SSH private key (PEM format)

### Production Environment
- `PROD_HOST`: Server hostname or IP
- `PROD_SSH_USER`: SSH username
- `PROD_SSH_KEY`: SSH private key (PEM format)

## Server Prerequisites

### Required Software
- Docker Engine 20.10+
- Docker Compose v2 (plugin)
- Git 2.0+
- curl (for health checks)
- Node.js 20+ and npm (for frontend build)

### Directory Setup
```bash
# Create application directory
sudo mkdir -p /opt/qcc_plus
sudo chown $USER:$USER /opt/qcc_plus

# Clone repository
cd /opt/qcc_plus
git clone https://github.com/yxhpy/qcc_plus.git .
git checkout test
```

### Firewall Configuration
```bash
# Ubuntu/Debian with ufw
sudo ufw allow 8000/tcp  # Production
sudo ufw allow 8001/tcp  # Test

# CentOS/RHEL with firewalld
sudo firewall-cmd --permanent --add-port=8000/tcp
sudo firewall-cmd --permanent --add-port=8001/tcp
sudo firewall-cmd --reload
```

## Monitoring and Logs

### View Deployment Logs
- GitHub Actions: Repository → Actions → Select workflow run
- Server logs: `docker logs qcc_test-proxy-1 -f`

### Real-time Monitoring
```bash
# Watch container status
watch -n 2 'docker ps -a | grep qcc_'

# Monitor resource usage
docker stats qcc_test-proxy-1

# Tail application logs
docker logs qcc_test-proxy-1 -f --tail 100
```

## Rollback

The deploy script includes automatic rollback on failure:
- Captures previous Docker image before deployment
- On error, reverts to previous image
- Restarts containers with old version

### Manual Rollback
```bash
cd /opt/qcc_plus
# Test environment rollback
git checkout test
git reset --hard HEAD~1
./scripts/deploy-server.sh test

# Production rollback example
./scripts/deploy-server.sh prod v1.2.0
```

## Contact and Support

- **GitHub Issues**: https://github.com/yxhpy/qcc_plus/issues
- **Documentation**: https://github.com/yxhpy/qcc_plus/tree/main/docs

## See Also
- [Multi-Tenant Architecture](multi-tenant-architecture.md)
- [Quick Start Guide](quick-start-multi-tenant.md)
- [Docker Hub Publish Guide](docker-hub-publish.md)
