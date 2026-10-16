'use strict';

const childProcess = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');

function input(name) {
  return (process.env[`INPUT_${name.toUpperCase()}`] || '').trim();
}

function appendCommand(file, value) {
  if (file) fs.appendFileSync(file, `${value}\n`, 'utf8');
}

function requestedVersion() {
  const direct = input('NODE-VERSION');
  if (direct) return direct;

  const versionFile = input('NODE-VERSION-FILE');
  if (!versionFile) return '';
  const filename = path.resolve(process.env.GITHUB_WORKSPACE || process.cwd(), versionFile);
  const content = fs.readFileSync(filename, 'utf8').trim();
  if (path.basename(filename) === 'package.json') {
    const manifest = JSON.parse(content);
    return manifest.devEngines?.runtime?.version || manifest.engines?.node || '';
  }
  return content.split(/\r?\n/, 1)[0].trim();
}

function requestedMajor(version) {
  if (!version) return 24;
  if (['*', 'current', 'latest', 'node', 'stable', 'lts/*'].includes(version.toLowerCase())) return 24;
  const match = version.match(/\d+/);
  return match ? Number(match[0]) : Number.NaN;
}

try {
  const requested = requestedVersion();
  const major = requestedMajor(requested);
  if (major !== 24) {
    throw new Error(`gitsss offline image provides Node.js 24; requested ${requested || 'an unknown version'}`);
  }

  const version = childProcess.execFileSync('node', ['--version'], {encoding: 'utf8'}).trim().replace(/^v/, '');
  if (!version.startsWith('24.')) throw new Error(`embedded Node.js runtime is ${version}, expected 24.x`);

  appendCommand(process.env.GITHUB_PATH, path.dirname(fs.realpathSync(process.execPath)));
  appendCommand(process.env.GITHUB_OUTPUT, `node-version=${version}`);
  if (input('CACHE')) {
    process.stdout.write('::notice::setup-node cache input is accepted; use bundled actions/cache for explicit dependency caching.\n');
  }
  process.stdout.write(`Using embedded Node.js ${version}\n`);
} catch (error) {
  process.stdout.write(`::error::${String(error.message || error)}\n`);
  process.exitCode = 1;
}
