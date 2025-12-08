/**
 * restart command - Restart the proxy server
 */

import { Command } from 'commander';
import chalk from 'chalk';
import { existsSync, readFileSync, unlinkSync, writeFileSync } from 'fs';
import { spawn } from 'child_process';
import { openSync, closeSync } from 'fs';
import { loadConfig, configToEnv, paths } from '../config.js';
import { findBinaryPath } from '../binary.js';

export function restartCommand() {
  const cmd = new Command('restart');

  cmd
    .description('Restart proxy server')
    .action(async () => {
      const binaryPath = findBinaryPath();
      if (!binaryPath) {
        console.error(chalk.red('Error: Binary not found. Please reinstall the package.'));
        process.exit(1);
      }

      // Stop if running
      if (existsSync(paths.PID_FILE)) {
        const pid = parseInt(readFileSync(paths.PID_FILE, 'utf-8').trim(), 10);

        if (isProcessRunning(pid)) {
          console.log(chalk.cyan(`Stopping QCC Plus (PID: ${pid})...`));

          try {
            process.kill(pid, 'SIGTERM');

            let attempts = 0;
            while (isProcessRunning(pid) && attempts < 30) {
              await sleep(100);
              attempts++;
            }

            if (isProcessRunning(pid)) {
              process.kill(pid, 'SIGKILL');
              await sleep(500);
            }
          } catch (err) {
            if (err.code !== 'ESRCH') {
              console.error(chalk.red(`Error stopping: ${err.message}`));
            }
          }
        }

        try {
          unlinkSync(paths.PID_FILE);
        } catch {}
      }

      // Start
      const config = loadConfig();
      const env = { ...process.env, ...configToEnv(config) };
      const logFd = openSync(paths.LOG_FILE, 'a');

      console.log(chalk.cyan('Starting QCC Plus...'));

      const child = spawn(binaryPath, ['proxy'], {
        detached: true,
        stdio: ['ignore', logFd, logFd],
        env,
      });

      writeFileSync(paths.PID_FILE, String(child.pid));
      child.unref();
      closeSync(logFd);

      console.log(chalk.green(`✓ QCC Plus restarted (PID: ${child.pid})`));
      console.log(chalk.gray(`Listen: ${config.listen_addr}`));
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

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

export default restartCommand;
