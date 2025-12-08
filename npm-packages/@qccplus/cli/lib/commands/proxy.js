/**
 * proxy command - Run proxy server in foreground
 */

import { Command } from 'commander';
import chalk from 'chalk';
import { loadConfig, configToEnv } from '../config.js';
import { executeBinary } from '../binary.js';

export function proxyCommand() {
  const cmd = new Command('proxy');

  cmd
    .description('Run proxy server in foreground')
    .option('-p, --port <port>', 'Listen port (overrides config)')
    .option('--upstream <url>', 'Upstream base URL')
    .option('--api-key <key>', 'Upstream API key')
    .action(async (options) => {
      const config = loadConfig();

      // Apply CLI overrides
      if (options.port) {
        config.listen_addr = `:${options.port}`;
      }
      if (options.upstream) {
        config.upstream.base_url = options.upstream;
      }
      if (options.apiKey) {
        config.upstream.api_key = options.apiKey;
      }

      const env = configToEnv(config);

      console.log(chalk.cyan('Starting QCC Plus proxy server...'));
      console.log(chalk.gray(`Listen: ${config.listen_addr}`));
      console.log(chalk.gray(`Upstream: ${config.upstream.base_url}`));
      console.log('');

      const exitCode = await executeBinary(['proxy'], { env });
      process.exit(exitCode);
    });

  return cmd;
}

export default proxyCommand;
