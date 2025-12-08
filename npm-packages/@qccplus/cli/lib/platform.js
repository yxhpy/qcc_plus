/**
 * Platform detection and binary package resolution
 */

const PLATFORMS = {
  'darwin-arm64': '@qccplus/darwin-arm64',
  'darwin-x64': '@qccplus/darwin-x64',
  'linux-arm64': '@qccplus/linux-arm64',
  'linux-x64': '@qccplus/linux-x64',
  'win32-x64': '@qccplus/win32-x64',
};

const PLATFORM_NAMES = {
  darwin: 'macOS',
  linux: 'Linux',
  win32: 'Windows',
};

const ARCH_NAMES = {
  arm64: 'ARM64 (Apple Silicon)',
  x64: 'x64 (Intel/AMD)',
};

/**
 * Get the platform package name for the current system
 * @returns {string|null} Package name or null if unsupported
 */
export function getPlatformPackage() {
  const key = `${process.platform}-${process.arch}`;
  return PLATFORMS[key] || null;
}

/**
 * Get human-readable platform name
 * @returns {string}
 */
export function getPlatformName() {
  const os = PLATFORM_NAMES[process.platform] || process.platform;
  const arch = ARCH_NAMES[process.arch] || process.arch;
  return `${os} ${arch}`;
}

/**
 * Check if the current platform is supported
 * @returns {boolean}
 */
export function isPlatformSupported() {
  return getPlatformPackage() !== null;
}

/**
 * Get all supported platforms
 * @returns {string[]}
 */
export function getSupportedPlatforms() {
  return Object.keys(PLATFORMS);
}

/**
 * Get the binary name for the current platform
 * @returns {string}
 */
export function getBinaryName() {
  return process.platform === 'win32' ? 'qccplus.exe' : 'qccplus';
}

export default {
  getPlatformPackage,
  getPlatformName,
  isPlatformSupported,
  getSupportedPlatforms,
  getBinaryName,
};
