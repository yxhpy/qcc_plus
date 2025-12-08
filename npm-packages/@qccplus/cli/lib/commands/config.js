/**
 * config command - Configuration management
 */

import { Command } from 'commander';
import chalk from 'chalk';
import {
  loadConfig,
  saveConfig,
  getConfigValue,
  setConfigValue,
  initConfig,
  paths,
  DEFAULT_CONFIG,
} from '../config.js';

export function configCommand() {
  const cmd = new Command('config');

  cmd
    .description('Manage QCC Plus configuration')
    .addCommand(initConfigCommand())
    .addCommand(getCommand())
    .addCommand(setCommand())
    .addCommand(listCommand())
    .addCommand(editCommand())
    .addCommand(pathCommand())
    .addCommand(resetCommand());

  return cmd;
}

function initConfigCommand() {
  const cmd = new Command('init');

  cmd
    .description('Initialize configuration file with defaults')
    .option('-f, --force', 'Overwrite existing configuration')
    .action((options) => {
      const created = initConfig(options.force);

      if (created) {
        console.log(chalk.green('✓ Configuration file created:'));
        console.log(chalk.gray(`  ${paths.CONFIG_FILE}`));
        console.log();
        console.log(chalk.cyan('Next steps:'));
        console.log(chalk.gray('  1. Set your upstream API key:'));
        console.log(chalk.gray('     qccplus config set upstream.api_key sk-ant-xxx'));
        console.log(chalk.gray('  2. Start the proxy:'));
        console.log(chalk.gray('     qccplus start'));
      } else {
        console.log(chalk.yellow('Configuration file already exists:'));
        console.log(chalk.gray(`  ${paths.CONFIG_FILE}`));
        console.log(chalk.gray('Use --force to overwrite'));
      }
    });

  return cmd;
}

function getCommand() {
  const cmd = new Command('get');

  cmd
    .description('Get a configuration value')
    .argument('<key>', 'Configuration key (e.g., upstream.api_key)')
    .action((key) => {
      const value = getConfigValue(key);

      if (value === undefined) {
        console.error(chalk.red(`Configuration key not found: ${key}`));
        process.exit(1);
      }

      if (typeof value === 'object') {
        console.log(JSON.stringify(value, null, 2));
      } else {
        console.log(value);
      }
    });

  return cmd;
}

function setCommand() {
  const cmd = new Command('set');

  cmd
    .description('Set a configuration value')
    .argument('<key>', 'Configuration key (e.g., upstream.api_key)')
    .argument('<value>', 'Value to set')
    .action((key, value) => {
      // Try to parse as JSON for complex values
      let parsedValue = value;
      try {
        parsedValue = JSON.parse(value);
      } catch {
        // Not JSON, use as string
        // Convert 'true'/'false' strings to booleans
        if (value === 'true') parsedValue = true;
        else if (value === 'false') parsedValue = false;
        // Convert numeric strings to numbers
        else if (/^\d+$/.test(value)) parsedValue = parseInt(value, 10);
        else if (/^\d+\.\d+$/.test(value)) parsedValue = parseFloat(value);
      }

      setConfigValue(key, parsedValue);
      console.log(chalk.green(`✓ ${key} = ${JSON.stringify(parsedValue)}`));
    });

  return cmd;
}

function listCommand() {
  const cmd = new Command('list');

  cmd
    .description('List all configuration values')
    .option('--json', 'Output as JSON')
    .option('--flat', 'Output as flat key=value pairs')
    .action((options) => {
      const config = loadConfig();

      if (options.json) {
        console.log(JSON.stringify(config, null, 2));
        return;
      }

      if (options.flat) {
        printFlat(config, '');
        return;
      }

      // Pretty print
      console.log(chalk.bold('\nQCC Plus Configuration\n'));
      printSection('Server', {
        'Listen Address': config.listen_addr,
      });

      printSection('Upstream', {
        'Base URL': config.upstream?.base_url,
        'API Key': maskApiKey(config.upstream?.api_key),
        'Name': config.upstream?.name,
      });

      printSection('Proxy', {
        'Retry Max': config.proxy?.retry_max,
        'Fail Threshold': config.proxy?.fail_threshold,
        'Health Interval': `${config.proxy?.health_interval_sec}s`,
      });

      printSection('MySQL', {
        'DSN': config.mysql?.dsn ? maskDSN(config.mysql.dsn) : chalk.gray('(not configured)'),
      });

      printSection('Admin', {
        'API Key': maskApiKey(config.admin?.api_key),
        'Default Account': config.admin?.default_account,
        'Default Proxy Key': maskApiKey(config.admin?.default_proxy_key),
      });

      printSection('Tunnel', {
        'Enabled': config.tunnel?.enabled ? chalk.green('Yes') : chalk.gray('No'),
        'Subdomain': config.tunnel?.subdomain || chalk.gray('(not set)'),
        'Zone': config.tunnel?.zone || chalk.gray('(not set)'),
      });

      console.log(chalk.gray(`\nConfig file: ${paths.CONFIG_FILE}\n`));
    });

  return cmd;
}

function editCommand() {
  const cmd = new Command('edit');

  cmd
    .description('Open configuration file in editor')
    .action(() => {
      const editor = process.env.EDITOR || process.env.VISUAL || 'vi';

      console.log(chalk.cyan(`Opening ${paths.CONFIG_FILE} with ${editor}...`));

      const { spawnSync } = require('child_process');
      spawnSync(editor, [paths.CONFIG_FILE], { stdio: 'inherit' });
    });

  return cmd;
}

function pathCommand() {
  const cmd = new Command('path');

  cmd
    .description('Show configuration file paths')
    .action(() => {
      console.log(chalk.bold('\nConfiguration Paths\n'));
      console.log(`Config Directory: ${chalk.cyan(paths.CONFIG_DIR)}`);
      console.log(`Config File:      ${chalk.cyan(paths.CONFIG_FILE)}`);
      console.log(`PID File:         ${chalk.cyan(paths.PID_FILE)}`);
      console.log(`Log File:         ${chalk.cyan(paths.LOG_FILE)}`);
      console.log();
    });

  return cmd;
}

function resetCommand() {
  const cmd = new Command('reset');

  cmd
    .description('Reset configuration to defaults')
    .option('-y, --yes', 'Skip confirmation')
    .action((options) => {
      if (!options.yes) {
        console.log(chalk.yellow('This will reset all configuration to defaults.'));
        console.log(chalk.gray('Use --yes to confirm'));
        return;
      }

      saveConfig(DEFAULT_CONFIG);
      console.log(chalk.green('✓ Configuration reset to defaults'));
    });

  return cmd;
}

// Helper functions

function printSection(title, items) {
  console.log(chalk.bold.cyan(`  ${title}`));
  for (const [key, value] of Object.entries(items)) {
    console.log(`    ${key}: ${value}`);
  }
  console.log();
}

function printFlat(obj, prefix) {
  for (const [key, value] of Object.entries(obj)) {
    const path = prefix ? `${prefix}.${key}` : key;
    if (value && typeof value === 'object' && !Array.isArray(value)) {
      printFlat(value, path);
    } else {
      console.log(`${path}=${JSON.stringify(value)}`);
    }
  }
}

function maskApiKey(key) {
  if (!key) return chalk.gray('(not set)');
  if (key.length <= 8) return '****';
  return key.slice(0, 4) + '****' + key.slice(-4);
}

function maskDSN(dsn) {
  // Mask password in DSN like user:pass@tcp(host)/db
  return dsn.replace(/:([^:@]+)@/, ':****@');
}

export default configCommand;
