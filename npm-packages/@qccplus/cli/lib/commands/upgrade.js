/**
 * upgrade command - Upgrade QCC Plus to the latest version
 */

import { Command } from 'commander';
import chalk from 'chalk';
import ora from 'ora';
import { createRequire } from 'module';
import { execSync, spawnSync } from 'child_process';
import semver from 'semver';
import { getBinaryVersion } from '../binary.js';

const require = createRequire(import.meta.url);

let packageJson;
try {
  packageJson = require('../../package.json');
} catch {
  packageJson = { version: 'unknown' };
}

export function upgradeCommand() {
  const cmd = new Command('upgrade');

  cmd
    .description('Upgrade QCC Plus to the latest version')
    .option('--check', 'Check for updates without installing')
    .option('--force', 'Force upgrade even if on latest version')
    .option('--pre', 'Include pre-release versions')
    .action(async (options) => {
      const spinner = ora('Checking for updates...').start();

      try {
        // Get current version
        const currentVersion = packageJson.version;

        // Get latest version from npm
        const latestVersion = await getLatestVersion('@qccplus/cli', options.pre);

        spinner.stop();

        if (!latestVersion) {
          console.error(chalk.red('Failed to check for updates'));
          process.exit(1);
        }

        console.log(chalk.bold('\nQCC Plus Version Check\n'));
        console.log(`  Current version: ${chalk.cyan(currentVersion)}`);
        console.log(`  Latest version:  ${chalk.green(latestVersion)}`);

        const binaryVersion = getBinaryVersion();
        if (binaryVersion) {
          console.log(`  Binary version:  ${chalk.gray(binaryVersion)}`);
        }

        const needsUpdate = semver.lt(currentVersion, latestVersion);

        if (!needsUpdate && !options.force) {
          console.log(chalk.green('\n✓ You are already on the latest version!'));
          return;
        }

        if (needsUpdate) {
          console.log(chalk.yellow(`\n  Update available: ${currentVersion} → ${latestVersion}`));
        }

        if (options.check) {
          if (needsUpdate) {
            console.log(chalk.cyan('\nRun "qccplus upgrade" to install the update'));
          }
          return;
        }

        // Perform upgrade
        console.log();
        const upgradeSpinner = ora('Upgrading...').start();

        try {
          // Detect package manager
          const pm = detectPackageManager();

          let upgradeCmd;
          if (pm === 'npm') {
            upgradeCmd = 'npm install -g @qccplus/cli@latest';
          } else if (pm === 'yarn') {
            upgradeCmd = 'yarn global add @qccplus/cli@latest';
          } else if (pm === 'pnpm') {
            upgradeCmd = 'pnpm add -g @qccplus/cli@latest';
          } else {
            upgradeCmd = 'npm install -g @qccplus/cli@latest';
          }

          execSync(upgradeCmd, { stdio: 'pipe' });

          upgradeSpinner.succeed('Upgrade complete!');

          // Show new version
          const newVersion = await getInstalledVersion();
          console.log(chalk.green(`\n✓ QCC Plus upgraded to ${newVersion}`));

          // Remind about service restart
          console.log(chalk.yellow('\nNote: If running as a service, restart it to apply the update:'));
          console.log(chalk.gray('  qccplus restart'));
          console.log(chalk.gray('  # or'));
          console.log(chalk.gray('  sudo systemctl restart qccplus'));
        } catch (err) {
          upgradeSpinner.fail('Upgrade failed');
          console.error(chalk.red(`\nError: ${err.message}`));
          console.log(chalk.cyan('\nTry upgrading manually:'));
          console.log(chalk.gray('  npm install -g @qccplus/cli@latest'));
          process.exit(1);
        }
      } catch (err) {
        spinner.fail('Failed to check for updates');
        console.error(chalk.red(`Error: ${err.message}`));
        process.exit(1);
      }
    });

  return cmd;
}

/**
 * Get the latest version from npm registry
 * @param {string} packageName - Package name
 * @param {boolean} includePre - Include pre-release versions
 * @returns {Promise<string|null>}
 */
async function getLatestVersion(packageName, includePre = false) {
  try {
    const result = spawnSync('npm', ['view', packageName, 'versions', '--json'], {
      encoding: 'utf-8',
      timeout: 30000,
    });

    if (result.status !== 0) {
      return null;
    }

    const versions = JSON.parse(result.stdout);

    if (!Array.isArray(versions)) {
      // Single version
      return versions;
    }

    // Filter and sort versions
    const validVersions = versions
      .filter(v => includePre || !semver.prerelease(v))
      .sort(semver.rcompare);

    return validVersions[0] || null;
  } catch {
    return null;
  }
}

/**
 * Get currently installed version
 * @returns {Promise<string>}
 */
async function getInstalledVersion() {
  try {
    const result = spawnSync('npm', ['list', '-g', '@qccplus/cli', '--json'], {
      encoding: 'utf-8',
    });

    const data = JSON.parse(result.stdout);
    return data.dependencies?.['@qccplus/cli']?.version || 'unknown';
  } catch {
    return 'unknown';
  }
}

/**
 * Detect which package manager was used to install globally
 * @returns {string} 'npm' | 'yarn' | 'pnpm'
 */
function detectPackageManager() {
  // Check for yarn
  try {
    const result = spawnSync('yarn', ['global', 'list', '--depth=0'], {
      encoding: 'utf-8',
      stdio: 'pipe',
    });
    if (result.stdout && result.stdout.includes('@qccplus/cli')) {
      return 'yarn';
    }
  } catch {}

  // Check for pnpm
  try {
    const result = spawnSync('pnpm', ['list', '-g', '--depth=0'], {
      encoding: 'utf-8',
      stdio: 'pipe',
    });
    if (result.stdout && result.stdout.includes('@qccplus/cli')) {
      return 'pnpm';
    }
  } catch {}

  // Default to npm
  return 'npm';
}

export default upgradeCommand;
