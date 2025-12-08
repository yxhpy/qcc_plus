/**
 * Platform package - exports the binary path
 */

import { dirname, join } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));

export const binaryPath = join(__dirname, '..', 'bin', 'qccplus');
export const platform = 'linux';
export const arch = 'x64';

export default { binaryPath, platform, arch };
