/**
 * QCC Plus CLI - Main entry point
 */

import { Command } from 'commander';
import chalk from 'chalk';
import { createRequire } from 'module';
import { dirname, join } from 'path';
import { fileURLToPath } from 'url';

// Commands
import { proxyCommand } from './commands/proxy.js';
import { startCommand } from './commands/start.js';
import { stopCommand } from './commands/stop.js';
import { restartCommand } from './commands/restart.js';
import { statusCommand } from './commands/status.js';
import { logsCommand } from './commands/logs.js';
import { serviceCommand } from './commands/service.js';
import { configCommand } from './commands/config.js';
import { upgradeCommand } from './commands/upgrade.js';
import { versionCommand } from './commands/version.js';

const __dirname = dirname(fileURLToPath(import.meta.url));
const require = createRequire(import.meta.url);

// Load package.json for version
let packageJson;
try {
  packageJson = require('../package.json');
} catch {
  packageJson = { version: 'unknown' };
}

/**
 * Create the CLI program
 * @returns {Command}
 */
function createProgram() {
  const program = new Command();

  program
    .name('qccplus')
    .description('QCC Plus - Enterprise-grade Claude Code CLI proxy server')
    .version(packageJson.version, '-V, --version', 'Output the version number')
    .option('-c, --config <path>', 'Config file path')
    .option('-v, --verbose', 'Verbose output')
    .option('-q, --quiet', 'Quiet mode')
    .option('--no-color', 'Disable colored output');

  // Core commands
  program.addCommand(proxyCommand());
  program.addCommand(startCommand());
  program.addCommand(stopCommand());
  program.addCommand(restartCommand());
  program.addCommand(statusCommand());
  program.addCommand(logsCommand());

  // Service management
  program.addCommand(serviceCommand());

  // Configuration
  program.addCommand(configCommand());

  // Upgrade
  program.addCommand(upgradeCommand());

  // Version (detailed)
  program.addCommand(versionCommand());

  // Custom help
  program.addHelpText('after', `
${chalk.bold('Examples:')}
  ${chalk.gray('# Quick start')}
  $ qccplus config init
  $ qccplus config set upstream.api_key sk-ant-xxx
  $ qccplus start

  ${chalk.gray('# Run as system service')}
  $ sudo qccplus service install
  $ qccplus service status

  ${chalk.gray('# Upgrade to latest version')}
  $ qccplus upgrade

${chalk.bold('Documentation:')}
  https://github.com/yxhpy/qcc_plus
`);

  return program;
}

/**
 * Run the CLI
 * @param {string[]} args - Command line arguments
 */
export async function run(args) {
  const program = createProgram();

  try {
    await program.parseAsync(args, { from: 'user' });
  } catch (err) {
    if (err.code === 'commander.helpDisplayed' || err.code === 'commander.version') {
      process.exit(0);
    }
    console.error(chalk.red(`Error: ${err.message}`));
    process.exit(1);
  }
}

export { createProgram };
export default { run, createProgram };
