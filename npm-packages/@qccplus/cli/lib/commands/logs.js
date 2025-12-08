/**
 * logs command - View proxy server logs
 */

import { Command } from 'commander';
import chalk from 'chalk';
import { existsSync, readFileSync, createReadStream, statSync } from 'fs';
import { createInterface } from 'readline';
import { paths } from '../config.js';

export function logsCommand() {
  const cmd = new Command('logs');

  cmd
    .description('View proxy server logs')
    .option('-f, --follow', 'Follow log output (like tail -f)')
    .option('-n, --lines <number>', 'Number of lines to show', '50')
    .option('--clear', 'Clear log file')
    .action(async (options) => {
      if (!existsSync(paths.LOG_FILE)) {
        console.log(chalk.yellow('No log file found.'));
        console.log(chalk.gray(`Expected at: ${paths.LOG_FILE}`));
        return;
      }

      if (options.clear) {
        const { writeFileSync } = await import('fs');
        writeFileSync(paths.LOG_FILE, '');
        console.log(chalk.green('✓ Log file cleared'));
        return;
      }

      const lines = parseInt(options.lines, 10) || 50;

      if (options.follow) {
        await followLogs(lines);
      } else {
        showLastLines(lines);
      }
    });

  return cmd;
}

function showLastLines(n) {
  const content = readFileSync(paths.LOG_FILE, 'utf-8');
  const lines = content.split('\n').filter(Boolean);
  const lastLines = lines.slice(-n);

  if (lastLines.length === 0) {
    console.log(chalk.gray('(empty log file)'));
    return;
  }

  lastLines.forEach(line => console.log(colorize(line)));
}

async function followLogs(initialLines) {
  // Show initial lines
  showLastLines(initialLines);

  console.log(chalk.gray('\n--- Following logs (Ctrl+C to exit) ---\n'));

  // Get initial file size
  let lastSize = statSync(paths.LOG_FILE).size;
  let lastContent = readFileSync(paths.LOG_FILE, 'utf-8');

  // Poll for changes
  const interval = setInterval(() => {
    try {
      const currentSize = statSync(paths.LOG_FILE).size;

      if (currentSize > lastSize) {
        const content = readFileSync(paths.LOG_FILE, 'utf-8');
        const newContent = content.slice(lastContent.length);
        const newLines = newContent.split('\n').filter(Boolean);

        newLines.forEach(line => console.log(colorize(line)));

        lastSize = currentSize;
        lastContent = content;
      } else if (currentSize < lastSize) {
        // File was truncated
        console.log(chalk.gray('\n--- Log file was truncated ---\n'));
        lastSize = currentSize;
        lastContent = readFileSync(paths.LOG_FILE, 'utf-8');
      }
    } catch (err) {
      // File might have been deleted
    }
  }, 500);

  // Handle Ctrl+C
  process.on('SIGINT', () => {
    clearInterval(interval);
    console.log('\n');
    process.exit(0);
  });

  // Keep process running
  await new Promise(() => {});
}

function colorize(line) {
  // Colorize log levels
  if (line.includes('[ERROR]') || line.includes('error') || line.includes('Error')) {
    return chalk.red(line);
  }
  if (line.includes('[WARN]') || line.includes('warning') || line.includes('Warning')) {
    return chalk.yellow(line);
  }
  if (line.includes('[INFO]') || line.includes('info')) {
    return chalk.cyan(line);
  }
  if (line.includes('[DEBUG]') || line.includes('debug')) {
    return chalk.gray(line);
  }
  if (line.includes('Started') || line.includes('started') || line.includes('✓')) {
    return chalk.green(line);
  }
  return line;
}

export default logsCommand;
