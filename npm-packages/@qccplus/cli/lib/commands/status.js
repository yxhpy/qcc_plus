/**
 * status command - Show proxy server status
 */

import { Command } from 'commander';
import chalk from 'chalk';
import { existsSync, readFileSync, statSync } from 'fs';
import { loadConfig, paths } from '../config.js';

export function statusCommand() {
  const cmd = new Command('status');

  cmd
    .description('Show proxy server status')
    .option('-j, --json', 'Output as JSON')
    .action(async (options) => {
      const status = getStatus();

      if (options.json) {
        console.log(JSON.stringify(status, null, 2));
        return;
      }

      console.log(chalk.bold('\nQCC Plus Status'));
      console.log('─'.repeat(40));

      if (status.running) {
        console.log(`Status:  ${chalk.green('● Running')}`);
        console.log(`PID:     ${status.pid}`);
        console.log(`Uptime:  ${formatUptime(status.uptime)}`);
      } else {
        console.log(`Status:  ${chalk.red('○ Stopped')}`);
      }

      console.log('');
      console.log(chalk.bold('Configuration'));
      console.log('─'.repeat(40));
      console.log(`Listen:   ${status.config.listen_addr}`);
      console.log(`Upstream: ${status.config.upstream?.base_url || 'not set'}`);
      console.log(`API Key:  ${status.config.upstream?.api_key ? maskKey(status.config.upstream.api_key) : chalk.yellow('not set')}`);

      console.log('');
      console.log(chalk.bold('Paths'));
      console.log('─'.repeat(40));
      console.log(`Config:  ${paths.CONFIG_FILE}`);
      console.log(`PID:     ${paths.PID_FILE}`);
      console.log(`Logs:    ${paths.LOG_FILE}`);
      console.log('');

      // Health check if running
      if (status.running) {
        const health = await checkHealth(status.config.listen_addr);
        if (health.ok) {
          console.log(chalk.bold('Health Check'));
          console.log('─'.repeat(40));
          console.log(`Endpoint: ${chalk.green('✓ OK')}`);
          if (health.version) {
            console.log(`Version:  ${health.version}`);
          }
          console.log('');
        } else {
          console.log(chalk.bold('Health Check'));
          console.log('─'.repeat(40));
          console.log(`Endpoint: ${chalk.yellow('⚠ Not responding')}`);
          console.log(chalk.gray('Service may still be starting...'));
          console.log('');
        }
      }
    });

  return cmd;
}

function getStatus() {
  const config = loadConfig();
  let running = false;
  let pid = null;
  let uptime = null;

  if (existsSync(paths.PID_FILE)) {
    pid = parseInt(readFileSync(paths.PID_FILE, 'utf-8').trim(), 10);
    running = isProcessRunning(pid);

    if (running) {
      try {
        const pidStat = statSync(paths.PID_FILE);
        uptime = Date.now() - pidStat.mtimeMs;
      } catch {}
    }
  }

  return {
    running,
    pid,
    uptime,
    config,
  };
}

function isProcessRunning(pid) {
  try {
    process.kill(pid, 0);
    return true;
  } catch {
    return false;
  }
}

function formatUptime(ms) {
  if (!ms) return 'unknown';

  const seconds = Math.floor(ms / 1000);
  const minutes = Math.floor(seconds / 60);
  const hours = Math.floor(minutes / 60);
  const days = Math.floor(hours / 24);

  if (days > 0) {
    return `${days}d ${hours % 24}h ${minutes % 60}m`;
  }
  if (hours > 0) {
    return `${hours}h ${minutes % 60}m ${seconds % 60}s`;
  }
  if (minutes > 0) {
    return `${minutes}m ${seconds % 60}s`;
  }
  return `${seconds}s`;
}

function maskKey(key) {
  if (!key || key.length < 8) return '****';
  return key.slice(0, 8) + '...' + key.slice(-4);
}

async function checkHealth(addr) {
  try {
    const port = addr.replace(/^:/, '').replace(/^.*:/, '');
    const url = `http://127.0.0.1:${port}/`;

    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 2000);

    const resp = await fetch(url, { signal: controller.signal });
    clearTimeout(timeout);

    if (resp.ok) {
      try {
        const data = await resp.json();
        return { ok: true, version: data.version };
      } catch {
        return { ok: true };
      }
    }
    return { ok: false };
  } catch {
    return { ok: false };
  }
}

export default statusCommand;
