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
  const direct = input('GO-VERSION');
  if (direct) return direct;

  const versionFile = input('GO-VERSION-FILE');
  if (!versionFile) return '';
  const filename = path.resolve(process.env.GITHUB_WORKSPACE || process.cwd(), versionFile);
  const content = fs.readFileSync(filename, 'utf8');
  const toolchain = content.match(/^\s*toolchain\s+go(\d+\.\d+(?:\.\d+)?)/m);
  if (toolchain) return toolchain[1];
  const directive = content.match(/^\s*go\s+(\d+\.\d+(?:\.\d+)?)/m);
  if (directive) return directive[1];
  return content.trim().split(/\r?\n/, 1)[0].replace(/^go\s+/, '').trim();
}

function requestedMinor(version) {
  if (!version || ['stable', 'latest', '*'].includes(version.toLowerCase())) return '1.26';
  const match = version.match(/(\d+)\.(\d+)/);
  return match ? `${match[1]}.${match[2]}` : '';
}

try {
  const requested = requestedVersion();
  if (requestedMinor(requested) !== '1.26') {
    throw new Error(`gitsss offline image provides Go 1.26; requested ${requested || 'an unknown version'}`);
  }

  const output = childProcess.execFileSync('/usr/local/go/bin/go', ['version'], {encoding: 'utf8'}).trim();
  const match = output.match(/\bgo(1\.26(?:\.\d+)?)\b/);
  if (!match) throw new Error(`embedded Go runtime is invalid: ${output}`);

  const home = process.env.HOME || '/data/git';
  appendCommand(process.env.GITHUB_PATH, '/usr/local/go/bin');
  appendCommand(process.env.GITHUB_ENV, 'GOROOT=/usr/local/go');
  appendCommand(process.env.GITHUB_ENV, `GOPATH=${path.join(home, 'go')}`);
  appendCommand(process.env.GITHUB_ENV, 'GOTOOLCHAIN=local');
  appendCommand(process.env.GITHUB_OUTPUT, `go-version=${match[1]}`);
  if (input('CACHE').toLowerCase() === 'true') {
    process.stdout.write('::notice::setup-go cache input is accepted; use bundled actions/cache for explicit dependency caching.\n');
  }
  process.stdout.write(`Using embedded Go ${match[1]}\n`);
} catch (error) {
  process.stdout.write(`::error::${String(error.message || error)}\n`);
  process.exitCode = 1;
}
