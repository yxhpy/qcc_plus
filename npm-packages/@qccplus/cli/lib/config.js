/**
 * Configuration management
 * Handles reading/writing YAML config files in ~/.qccplus/
 */

import { readFileSync, writeFileSync, existsSync, mkdirSync } from 'fs';
import { join } from 'path';
import { homedir } from 'os';
import yaml from 'js-yaml';

const CONFIG_DIR = join(homedir(), '.qccplus');
const CONFIG_FILE = join(CONFIG_DIR, 'config.yaml');
const PID_FILE = join(CONFIG_DIR, 'qccplus.pid');
const LOG_FILE = join(CONFIG_DIR, 'qccplus.log');

/**
 * Default configuration
 * NOTE: MySQL is optional! When mysql.dsn is empty, the server runs in memory mode.
 * Memory mode is perfect for single-user/development use.
 */
const DEFAULT_CONFIG = {
  listen_addr: ':8000',
  upstream: {
    base_url: 'https://api.anthropic.com',
    api_key: '',
    name: 'default',
  },
  proxy: {
    retry_max: 3,
    fail_threshold: 3,
    health_interval_sec: 30,
  },
  // MySQL is OPTIONAL - leave empty for memory mode (default)
  // Only needed for: multi-instance deployment, data persistence across restarts
  mysql: {
    dsn: '',  // Empty = memory mode (no database required!)
  },
  admin: {
    api_key: 'admin',
    default_account: 'default',
    default_proxy_key: 'default-proxy-key',
  },
  tunnel: {
    enabled: false,
    subdomain: '',
    zone: '',
    api_token: '',
  },
};

/**
 * Ensure config directory exists
 */
export function ensureConfigDir() {
  if (!existsSync(CONFIG_DIR)) {
    mkdirSync(CONFIG_DIR, { recursive: true });
  }
}

/**
 * Load configuration from file
 * @returns {Object} Configuration object
 */
export function loadConfig() {
  ensureConfigDir();

  if (!existsSync(CONFIG_FILE)) {
    return { ...DEFAULT_CONFIG };
  }

  try {
    const content = readFileSync(CONFIG_FILE, 'utf-8');
    const config = yaml.load(content) || {};
    return deepMerge(DEFAULT_CONFIG, config);
  } catch (err) {
    console.error(`Warning: Failed to load config: ${err.message}`);
    return { ...DEFAULT_CONFIG };
  }
}

/**
 * Save configuration to file
 * @param {Object} config - Configuration object
 */
export function saveConfig(config) {
  ensureConfigDir();

  const content = yaml.dump(config, {
    indent: 2,
    lineWidth: 120,
    noRefs: true,
  });

  writeFileSync(CONFIG_FILE, content, 'utf-8');
}

/**
 * Get a config value by dot-notation path
 * @param {string} path - Config path (e.g., 'upstream.api_key')
 * @returns {*} Config value
 */
export function getConfigValue(path) {
  const config = loadConfig();
  return getByPath(config, path);
}

/**
 * Set a config value by dot-notation path
 * @param {string} path - Config path (e.g., 'upstream.api_key')
 * @param {*} value - Value to set
 */
export function setConfigValue(path, value) {
  const config = loadConfig();
  setByPath(config, path, value);
  saveConfig(config);
}

/**
 * Initialize config file with defaults
 * @param {boolean} force - Overwrite existing config
 * @returns {boolean} True if created, false if already exists
 */
export function initConfig(force = false) {
  ensureConfigDir();

  if (existsSync(CONFIG_FILE) && !force) {
    return false;
  }

  saveConfig(DEFAULT_CONFIG);
  return true;
}

/**
 * Convert config to environment variables for the binary
 * @param {Object} config - Configuration object
 * @returns {Object} Environment variables
 */
export function configToEnv(config) {
  const env = {};

  if (config.listen_addr) {
    env.LISTEN_ADDR = config.listen_addr;
  }

  if (config.upstream) {
    if (config.upstream.base_url) {
      env.UPSTREAM_BASE_URL = config.upstream.base_url;
    }
    if (config.upstream.api_key) {
      env.UPSTREAM_API_KEY = config.upstream.api_key;
    }
    if (config.upstream.name) {
      env.UPSTREAM_NAME = config.upstream.name;
    }
  }

  if (config.proxy) {
    if (config.proxy.retry_max) {
      env.PROXY_RETRY_MAX = String(config.proxy.retry_max);
    }
    if (config.proxy.fail_threshold) {
      env.PROXY_FAIL_THRESHOLD = String(config.proxy.fail_threshold);
    }
    if (config.proxy.health_interval_sec) {
      env.PROXY_HEALTH_INTERVAL_SEC = String(config.proxy.health_interval_sec);
    }
  }

  if (config.mysql?.dsn) {
    env.PROXY_MYSQL_DSN = config.mysql.dsn;
  }

  if (config.admin) {
    if (config.admin.api_key) {
      env.ADMIN_API_KEY = config.admin.api_key;
    }
    if (config.admin.default_account) {
      env.DEFAULT_ACCOUNT_NAME = config.admin.default_account;
    }
    if (config.admin.default_proxy_key) {
      env.DEFAULT_PROXY_API_KEY = config.admin.default_proxy_key;
    }
  }

  if (config.tunnel) {
    if (config.tunnel.enabled) {
      env.TUNNEL_ENABLED = 'true';
    }
    if (config.tunnel.subdomain) {
      env.TUNNEL_SUBDOMAIN = config.tunnel.subdomain;
    }
    if (config.tunnel.zone) {
      env.TUNNEL_ZONE = config.tunnel.zone;
    }
    if (config.tunnel.api_token) {
      env.CF_API_TOKEN = config.tunnel.api_token;
    }
  }

  return env;
}

// Helper functions

function getByPath(obj, path) {
  return path.split('.').reduce((acc, part) => acc?.[part], obj);
}

function setByPath(obj, path, value) {
  const parts = path.split('.');
  const last = parts.pop();
  const target = parts.reduce((acc, part) => {
    if (acc[part] === undefined) {
      acc[part] = {};
    }
    return acc[part];
  }, obj);
  target[last] = value;
}

function deepMerge(target, source) {
  const result = { ...target };
  for (const key in source) {
    if (source[key] && typeof source[key] === 'object' && !Array.isArray(source[key])) {
      result[key] = deepMerge(result[key] || {}, source[key]);
    } else {
      result[key] = source[key];
    }
  }
  return result;
}

// Export paths for other modules
export const paths = {
  CONFIG_DIR,
  CONFIG_FILE,
  PID_FILE,
  LOG_FILE,
};

// Export DEFAULT_CONFIG
export { DEFAULT_CONFIG };

export default {
  loadConfig,
  saveConfig,
  getConfigValue,
  setConfigValue,
  initConfig,
  configToEnv,
  ensureConfigDir,
  paths,
  DEFAULT_CONFIG,
};
