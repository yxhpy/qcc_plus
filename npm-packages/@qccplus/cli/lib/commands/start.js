/**
 * start command - Start proxy server in background
 */

import { Command } from 'commander';
import chalk from 'chalk';
import { spawn } from 'child_process';
import { writeFileSync, existsSync, readFileSync } from 'fs';
import { loadConfig, configToEnv, paths } from '../config.js';
import { findBinaryPath } from '../binary.js';
import { openSync, closeSync } from 'fs';

export function startCommand() {
  const cmd = new Command('start');

  cmd
    .description('Start proxy server in background')
    .option('-p, --port <port>', 'Listen port (overrides config)')
    .option('--upstream <url>', 'Upstream base URL')
    .option('--api-key <key>', 'Upstream API key')
    .action(async (options) => {
      // Check if already running
      if (existsSync(paths.PID_FILE)) {
        const pid = parseInt(readFileSync(paths.PID_FILE, 'utf-8').trim(), 10);
        if (isProcessRunning(pid)) {
          console.log(chalk.yellow(`QCC Plus is already running (PID: ${pid})`));
          console.log(chalk.gray('Use "qccplus restart" to restart or "qccplus stop" to stop'));
          return;
        }
      }

      const binaryPath = findBinaryPath();
      if (!binaryPath) {
        console.error(chalk.red('Error: Binary not found. Please reinstall the package.'));
        process.exit(1);
      }

      const config = loadConfig();

      // Apply CLI overrides
      if (options.port) {
        config.listen_addr = `:${options.port}`;
      }
      if (options.upstream) {
        config.upstream.base_url = options.upstream;
      }
      if (options.apiKey) {
        config.upstream.api_key = options.apiKey;
      }

      const env = { ...process.env, ...configToEnv(config) };

      // Open log file
      const logFd = openSync(paths.LOG_FILE, 'a');

      console.log(chalk.cyan('Starting QCC Plus in background...'));

      const child = spawn(binaryPath, ['proxy'], {
        detached: true,
        stdio: ['ignore', logFd, logFd],
        env,
      });

      // Write PID file
      writeFileSync(paths.PID_FILE, String(child.pid));

      child.unref();
      closeSync(logFd);

      console.log(chalk.green(`✓ QCC Plus started (PID: ${child.pid})`));
      console.log(chalk.gray(`Listen: ${config.listen_addr}`));
      console.log(chalk.gray(`Logs: ${paths.LOG_FILE}`));
    });

  return cmd;
}

function isProcessRunning(pid) {
  try {
    process.kill(pid, 0);
    return true;
  } catch {
    return false;
  }
}

export default startCommand;
