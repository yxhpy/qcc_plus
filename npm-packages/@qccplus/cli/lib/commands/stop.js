/**
 * stop command - Stop the background proxy server
 */

import { Command } from 'commander';
import chalk from 'chalk';
import { existsSync, readFileSync, unlinkSync } from 'fs';
import { paths } from '../config.js';

export function stopCommand() {
  const cmd = new Command('stop');

  cmd
    .description('Stop background proxy server')
    .option('-f, --force', 'Force kill (SIGKILL)')
    .action(async (options) => {
      if (!existsSync(paths.PID_FILE)) {
        console.log(chalk.yellow('QCC Plus is not running (no PID file found)'));
        return;
      }

      const pid = parseInt(readFileSync(paths.PID_FILE, 'utf-8').trim(), 10);

      if (!isProcessRunning(pid)) {
        console.log(chalk.yellow('QCC Plus is not running (process not found)'));
        unlinkSync(paths.PID_FILE);
        return;
      }

      const signal = options.force ? 'SIGKILL' : 'SIGTERM';
      console.log(chalk.cyan(`Stopping QCC Plus (PID: ${pid})...`));

      try {
        process.kill(pid, signal);

        // Wait for process to exit
        let attempts = 0;
        while (isProcessRunning(pid) && attempts < 30) {
          await sleep(100);
          attempts++;
        }

        if (isProcessRunning(pid)) {
          if (!options.force) {
            console.log(chalk.yellow('Process did not stop gracefully, force killing...'));
            process.kill(pid, 'SIGKILL');
            await sleep(500);
          }

          if (isProcessRunning(pid)) {
            console.error(chalk.red('Failed to stop QCC Plus'));
            process.exit(1);
          }
        }

        unlinkSync(paths.PID_FILE);
        console.log(chalk.green('✓ QCC Plus stopped'));
      } catch (err) {
        if (err.code === 'ESRCH') {
          console.log(chalk.yellow('Process not found'));
          unlinkSync(paths.PID_FILE);
        } else {
          console.error(chalk.red(`Error stopping process: ${err.message}`));
          process.exit(1);
        }
      }
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

export default stopCommand;
