/**
 * Binary management - locate and execute the platform-specific binary
 */

import { createRequire } from 'module';
import { spawn, spawnSync } from 'child_process';
import { existsSync } from 'fs';
import { join, dirname } from 'path';
import { fileURLToPath } from 'url';
import { getPlatformPackage, getBinaryName, isPlatformSupported, getPlatformName } from './platform.js';

const __dirname = dirname(fileURLToPath(import.meta.url));
const require = createRequire(import.meta.url);

/**
 * Find the binary path for the current platform
 * @returns {string|null} Binary path or null if not found
 */
export function findBinaryPath() {
  const platformPkg = getPlatformPackage();

  if (!platformPkg) {
    return null;
  }

  const binaryName = getBinaryName();

  // Try to find the binary in the platform package
  try {
    const pkgPath = require.resolve(`${platformPkg}/package.json`);
    const pkgDir = dirname(pkgPath);
    const binaryPath = join(pkgDir, 'bin', binaryName);

    if (existsSync(binaryPath)) {
      return binaryPath;
    }
  } catch {
    // Package not installed
  }

  // Try local development path
  const localPath = join(__dirname, '..', '..', platformPkg.replace('@qccplus/', ''), 'bin', binaryName);
  if (existsSync(localPath)) {
    return localPath;
  }

  return null;
}

/**
 * Execute the binary with given arguments
 * @param {string[]} args - Arguments to pass to the binary
 * @param {Object} options - Execution options
 * @returns {Promise<number>} Exit code
 */
export async function executeBinary(args, options = {}) {
  const binaryPath = findBinaryPath();

  if (!binaryPath) {
    if (!isPlatformSupported()) {
      console.error(`Error: Platform ${getPlatformName()} is not supported.`);
      console.error('Supported platforms: darwin-arm64, darwin-x64, linux-arm64, linux-x64, win32-x64');
    } else {
      console.error('Error: Binary not found. Please reinstall the package:');
      console.error('  npm install -g @qccplus/cli');
    }
    return 1;
  }

  const env = { ...process.env, ...options.env };

  return new Promise((resolve) => {
    const child = spawn(binaryPath, args, {
      stdio: options.stdio || 'inherit',
      env,
      cwd: options.cwd || process.cwd(),
      detached: options.detached || false,
    });

    if (options.detached) {
      child.unref();
      resolve(0);
      return;
    }

    child.on('error', (err) => {
      console.error(`Error executing binary: ${err.message}`);
      resolve(1);
    });

    child.on('close', (code) => {
      resolve(code || 0);
    });
  });
}

/**
 * Execute the binary synchronously
 * @param {string[]} args - Arguments to pass to the binary
 * @param {Object} options - Execution options
 * @returns {Object} Result with status, stdout, stderr
 */
export function executeBinarySync(args, options = {}) {
  const binaryPath = findBinaryPath();

  if (!binaryPath) {
    return {
      status: 1,
      stdout: '',
      stderr: isPlatformSupported()
        ? 'Binary not found. Please reinstall the package.'
        : `Platform ${getPlatformName()} is not supported.`,
    };
  }

  const env = { ...process.env, ...options.env };

  const result = spawnSync(binaryPath, args, {
    encoding: 'utf-8',
    env,
    cwd: options.cwd || process.cwd(),
    stdio: options.stdio,
  });

  return {
    status: result.status || 0,
    stdout: result.stdout || '',
    stderr: result.stderr || '',
  };
}

/**
 * Get binary version
 * @returns {string|null} Version string or null
 */
export function getBinaryVersion() {
  const result = executeBinarySync(['version'], { stdio: 'pipe' });
  if (result.status === 0 && result.stdout) {
    // Parse version from output like "qcc_plus version: v1.9.1 (commit=...)"
    const match = result.stdout.match(/v?(\d+\.\d+\.\d+)/);
    return match ? match[1] : null;
  }
  return null;
}

export default {
  findBinaryPath,
  executeBinary,
  executeBinarySync,
  getBinaryVersion,
};
