import { copyFileSync, existsSync, mkdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const repoRoot = join(__dirname, '..', '..', '..');
const source = join(repoRoot, 'bin', process.platform === 'win32' ? 'zero-server.exe' : 'zero-server');

const triples = {
  'darwin-arm64': 'aarch64-apple-darwin',
  'darwin-x64': 'x86_64-apple-darwin',
  'linux-x64': 'x86_64-unknown-linux-gnu',
  'linux-arm64': 'aarch64-unknown-linux-gnu',
  'win32-x64': 'x86_64-pc-windows-msvc',
  'win32-arm64': 'aarch64-pc-windows-msvc',
};

const key = `${process.platform}-${process.arch}`;
const triple = triples[key];

if (!triple) {
  throw new Error(`Unsupported sidecar platform: ${key}`);
}

if (!existsSync(source)) {
  throw new Error(`Missing ${source}. Run make build before building the desktop app.`);
}

const outDir = join(__dirname, '..', 'src-tauri', 'binaries');
mkdirSync(outDir, { recursive: true });

const extension = process.platform === 'win32' ? '.exe' : '';
const target = join(outDir, `zero-server-${triple}${extension}`);
copyFileSync(source, target);
console.log(`Prepared Tauri sidecar: ${target}`);
