/**
 * version command - Show detailed version information
 */

import { Command } from 'commander';
import chalk from 'chalk';
import { createRequire } from 'module';
import { getBinaryVersion, findBinaryPath } from '../binary.js';
import { getPlatformName, getPlatformPackage, getSupportedPlatforms } from '../platform.js';

const require = createRequire(import.meta.url);

let packageJson;
try {
  packageJson = require('../../package.json');
} catch {
  packageJson = { version: 'unknown' };
}

export function versionCommand() {
  const cmd = new Command('version');

  cmd
    .description('Show detailed version information')
    .option('--json', 'Output as JSON')
    .action((options) => {
      const info = gatherVersionInfo();

      if (options.json) {
        console.log(JSON.stringify(info, null, 2));
        return;
      }

      console.log(chalk.bold('\nQCC Plus Version Information\n'));

      // CLI version
      console.log(chalk.cyan('  CLI Package'));
      console.log(`    Version:  ${chalk.green(info.cli.version)}`);
      console.log(`    Package:  ${info.cli.package}`);
      console.log();

      // Binary version
      console.log(chalk.cyan('  Binary'));
      if (info.binary.installed) {
        console.log(`    Version:  ${chalk.green(info.binary.version || 'unknown')}`);
        console.log(`    Path:     ${info.binary.path}`);
      } else {
        console.log(chalk.red('    Not installed'));
      }
      console.log();

      // Platform info
      console.log(chalk.cyan('  Platform'));
      console.log(`    OS:       ${info.platform.name}`);
      console.log(`    Package:  ${info.platform.package || chalk.red('Not supported')}`);
      console.log(`    Node.js:  ${info.platform.node}`);
      console.log();

      // Supported platforms
      console.log(chalk.cyan('  Supported Platforms'));
      for (const platform of info.supportedPlatforms) {
        const current = platform === `${process.platform}-${process.arch}`;
        const marker = current ? chalk.green(' ← current') : '';
        console.log(`    - ${platform}${marker}`);
      }
      console.log();

      // Links
      console.log(chalk.cyan('  Resources'));
      console.log(`    GitHub:     ${chalk.blue('https://github.com/yxhpy/qcc_plus')}`);
      console.log(`    npm:        ${chalk.blue('https://www.npmjs.com/package/@qccplus/cli')}`);
      console.log(`    Docker Hub: ${chalk.blue('https://hub.docker.com/r/yxhpy520/qcc_plus')}`);
      console.log();
    });

  return cmd;
}

function gatherVersionInfo() {
  const binaryPath = findBinaryPath();
  const binaryVersion = binaryPath ? getBinaryVersion() : null;

  return {
    cli: {
      version: packageJson.version,
      package: '@qccplus/cli',
    },
    binary: {
      installed: !!binaryPath,
      version: binaryVersion,
      path: binaryPath,
    },
    platform: {
      name: getPlatformName(),
      os: process.platform,
      arch: process.arch,
      package: getPlatformPackage(),
      node: process.version,
    },
    supportedPlatforms: getSupportedPlatforms(),
  };
}

export default versionCommand;
