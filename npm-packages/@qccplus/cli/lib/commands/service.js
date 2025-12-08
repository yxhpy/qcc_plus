/**
 * service command - System service management (install/uninstall/status)
 */

import { Command } from 'commander';
import chalk from 'chalk';
import { existsSync, writeFileSync, unlinkSync, readFileSync } from 'fs';
import { execSync, spawnSync } from 'child_process';
import { join } from 'path';
import { findBinaryPath } from '../binary.js';
import { paths, loadConfig, configToEnv } from '../config.js';

const SERVICE_NAME = 'qccplus';

export function serviceCommand() {
  const cmd = new Command('service');

  cmd
    .description('Manage QCC Plus as a system service')
    .addCommand(installCommand())
    .addCommand(uninstallCommand())
    .addCommand(serviceStatusCommand())
    .addCommand(enableCommand())
    .addCommand(disableCommand());

  return cmd;
}

function installCommand() {
  const cmd = new Command('install');

  cmd
    .description('Install QCC Plus as a system service')
    .option('--user', 'Install as user service (Linux only)')
    .action(async (options) => {
      if (process.platform === 'win32') {
        installWindows();
      } else if (process.platform === 'darwin') {
        installMacOS(options.user);
      } else if (process.platform === 'linux') {
        installLinux(options.user);
      } else {
        console.error(chalk.red(`Unsupported platform: ${process.platform}`));
        process.exit(1);
      }
    });

  return cmd;
}

function uninstallCommand() {
  const cmd = new Command('uninstall');

  cmd
    .description('Uninstall QCC Plus system service')
    .option('--user', 'Uninstall user service (Linux only)')
    .action(async (options) => {
      if (process.platform === 'win32') {
        uninstallWindows();
      } else if (process.platform === 'darwin') {
        uninstallMacOS(options.user);
      } else if (process.platform === 'linux') {
        uninstallLinux(options.user);
      } else {
        console.error(chalk.red(`Unsupported platform: ${process.platform}`));
        process.exit(1);
      }
    });

  return cmd;
}

function serviceStatusCommand() {
  const cmd = new Command('status');

  cmd
    .description('Show service status')
    .action(async () => {
      if (process.platform === 'win32') {
        statusWindows();
      } else if (process.platform === 'darwin') {
        statusMacOS();
      } else if (process.platform === 'linux') {
        statusLinux();
      } else {
        console.error(chalk.red(`Unsupported platform: ${process.platform}`));
        process.exit(1);
      }
    });

  return cmd;
}

function enableCommand() {
  const cmd = new Command('enable');

  cmd
    .description('Enable service to start on boot')
    .action(async () => {
      if (process.platform === 'linux') {
        try {
          execSync(`systemctl enable ${SERVICE_NAME}`, { stdio: 'inherit' });
          console.log(chalk.green('✓ Service enabled'));
        } catch {
          console.error(chalk.red('Failed to enable service'));
        }
      } else if (process.platform === 'darwin') {
        console.log(chalk.yellow('macOS: Service is automatically enabled on install'));
      } else {
        console.log(chalk.yellow('Use system tools to enable the service on boot'));
      }
    });

  return cmd;
}

function disableCommand() {
  const cmd = new Command('disable');

  cmd
    .description('Disable service from starting on boot')
    .action(async () => {
      if (process.platform === 'linux') {
        try {
          execSync(`systemctl disable ${SERVICE_NAME}`, { stdio: 'inherit' });
          console.log(chalk.green('✓ Service disabled'));
        } catch {
          console.error(chalk.red('Failed to disable service'));
        }
      } else if (process.platform === 'darwin') {
        console.log(chalk.yellow('macOS: Unload the service with: launchctl unload <plist>'));
      } else {
        console.log(chalk.yellow('Use system tools to disable the service'));
      }
    });

  return cmd;
}

// macOS implementation using launchd
function installMacOS(userMode = false) {
  const binaryPath = findBinaryPath();
  if (!binaryPath) {
    console.error(chalk.red('Binary not found. Please reinstall the package.'));
    process.exit(1);
  }

  const config = loadConfig();
  const env = configToEnv(config);

  const plistDir = userMode
    ? join(process.env.HOME, 'Library/LaunchAgents')
    : '/Library/LaunchDaemons';
  const plistPath = join(plistDir, `com.qccplus.proxy.plist`);

  const envDict = Object.entries(env)
    .map(([k, v]) => `      <key>${k}</key>\n      <string>${v}</string>`)
    .join('\n');

  const plist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.qccplus.proxy</string>
    <key>ProgramArguments</key>
    <array>
        <string>${binaryPath}</string>
        <string>proxy</string>
    </array>
    <key>EnvironmentVariables</key>
    <dict>
${envDict}
    </dict>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>${paths.LOG_FILE}</string>
    <key>StandardErrorPath</key>
    <string>${paths.LOG_FILE}</string>
    <key>WorkingDirectory</key>
    <string>${paths.CONFIG_DIR}</string>
</dict>
</plist>
`;

  try {
    writeFileSync(plistPath, plist);
    console.log(chalk.green(`✓ Service installed: ${plistPath}`));

    // Load the service
    const loadCmd = userMode ? 'launchctl load' : 'sudo launchctl load';
    execSync(`${loadCmd} ${plistPath}`, { stdio: 'inherit' });
    console.log(chalk.green('✓ Service loaded and started'));
  } catch (err) {
    console.error(chalk.red(`Failed to install service: ${err.message}`));
    if (!userMode) {
      console.log(chalk.yellow('Try running with sudo or use --user flag'));
    }
    process.exit(1);
  }
}

function uninstallMacOS(userMode = false) {
  const plistDir = userMode
    ? join(process.env.HOME, 'Library/LaunchAgents')
    : '/Library/LaunchDaemons';
  const plistPath = join(plistDir, `com.qccplus.proxy.plist`);

  if (!existsSync(plistPath)) {
    console.log(chalk.yellow('Service is not installed'));
    return;
  }

  try {
    const unloadCmd = userMode ? 'launchctl unload' : 'sudo launchctl unload';
    execSync(`${unloadCmd} ${plistPath}`, { stdio: 'inherit' });
    unlinkSync(plistPath);
    console.log(chalk.green('✓ Service uninstalled'));
  } catch (err) {
    console.error(chalk.red(`Failed to uninstall service: ${err.message}`));
    process.exit(1);
  }
}

function statusMacOS() {
  try {
    const result = spawnSync('launchctl', ['list', 'com.qccplus.proxy'], {
      encoding: 'utf-8',
    });

    if (result.status === 0) {
      console.log(chalk.green('● Service is running'));
      console.log(result.stdout);
    } else {
      console.log(chalk.yellow('○ Service is not running'));
    }
  } catch {
    console.log(chalk.yellow('○ Service status unknown'));
  }
}

// Linux implementation using systemd
function installLinux(userMode = false) {
  const binaryPath = findBinaryPath();
  if (!binaryPath) {
    console.error(chalk.red('Binary not found. Please reinstall the package.'));
    process.exit(1);
  }

  const config = loadConfig();
  const env = configToEnv(config);

  const envLines = Object.entries(env)
    .map(([k, v]) => `Environment="${k}=${v}"`)
    .join('\n');

  const serviceDir = userMode
    ? join(process.env.HOME, '.config/systemd/user')
    : '/etc/systemd/system';
  const servicePath = join(serviceDir, `${SERVICE_NAME}.service`);

  const serviceContent = `[Unit]
Description=QCC Plus - Claude Code CLI Proxy Server
After=network.target

[Service]
Type=simple
ExecStart=${binaryPath} proxy
WorkingDirectory=${paths.CONFIG_DIR}
${envLines}
Restart=always
RestartSec=5

[Install]
WantedBy=${userMode ? 'default.target' : 'multi-user.target'}
`;

  try {
    if (!userMode && process.getuid() !== 0) {
      console.error(chalk.red('Root privileges required. Run with sudo or use --user flag.'));
      process.exit(1);
    }

    writeFileSync(servicePath, serviceContent);
    console.log(chalk.green(`✓ Service file created: ${servicePath}`));

    const systemctl = userMode ? 'systemctl --user' : 'systemctl';
    execSync(`${systemctl} daemon-reload`, { stdio: 'inherit' });
    execSync(`${systemctl} enable ${SERVICE_NAME}`, { stdio: 'inherit' });
    execSync(`${systemctl} start ${SERVICE_NAME}`, { stdio: 'inherit' });

    console.log(chalk.green('✓ Service installed and started'));
  } catch (err) {
    console.error(chalk.red(`Failed to install service: ${err.message}`));
    process.exit(1);
  }
}

function uninstallLinux(userMode = false) {
  const serviceDir = userMode
    ? join(process.env.HOME, '.config/systemd/user')
    : '/etc/systemd/system';
  const servicePath = join(serviceDir, `${SERVICE_NAME}.service`);

  if (!existsSync(servicePath)) {
    console.log(chalk.yellow('Service is not installed'));
    return;
  }

  try {
    const systemctl = userMode ? 'systemctl --user' : 'systemctl';
    execSync(`${systemctl} stop ${SERVICE_NAME}`, { stdio: 'pipe' });
    execSync(`${systemctl} disable ${SERVICE_NAME}`, { stdio: 'pipe' });
    unlinkSync(servicePath);
    execSync(`${systemctl} daemon-reload`, { stdio: 'pipe' });

    console.log(chalk.green('✓ Service uninstalled'));
  } catch (err) {
    console.error(chalk.red(`Failed to uninstall service: ${err.message}`));
    process.exit(1);
  }
}

function statusLinux() {
  try {
    execSync(`systemctl status ${SERVICE_NAME}`, { stdio: 'inherit' });
  } catch {
    // systemctl returns non-zero if service is not running
  }
}

// Windows implementation (basic - uses sc.exe)
function installWindows() {
  const binaryPath = findBinaryPath();
  if (!binaryPath) {
    console.error(chalk.red('Binary not found. Please reinstall the package.'));
    process.exit(1);
  }

  console.log(chalk.yellow('Windows service installation requires administrator privileges.'));
  console.log(chalk.cyan('\nTo install as a Windows service, run as Administrator:'));
  console.log(chalk.gray(`  sc create ${SERVICE_NAME} binPath= "${binaryPath} proxy" start= auto`));
  console.log(chalk.gray(`  sc start ${SERVICE_NAME}`));
  console.log(chalk.cyan('\nAlternatively, use a tool like NSSM (Non-Sucking Service Manager):'));
  console.log(chalk.gray(`  nssm install ${SERVICE_NAME} "${binaryPath}" proxy`));
}

function uninstallWindows() {
  console.log(chalk.cyan('To uninstall the Windows service, run as Administrator:'));
  console.log(chalk.gray(`  sc stop ${SERVICE_NAME}`));
  console.log(chalk.gray(`  sc delete ${SERVICE_NAME}`));
}

function statusWindows() {
  try {
    const result = spawnSync('sc', ['query', SERVICE_NAME], {
      encoding: 'utf-8',
    });
    console.log(result.stdout || result.stderr);
  } catch {
    console.log(chalk.yellow('Could not query service status'));
  }
}

export default serviceCommand;
