import {readFileSync} from 'node:fs';
import path from 'node:path';
import {runInNewContext} from 'node:vm';

const source = readFileSync('actions/builtin/actions/setup-node/v7/dist/index.js', 'utf8');

test.each([
  {runtime: '/opt/runtime/node24/bin/node', expected: '/opt/runtime/node24/bin', paths: path.posix},
  {runtime: 'C:\\Runtime\\node24\\node.exe', expected: 'C:\\Runtime\\node24', paths: path.win32},
])('bundled setup-node preserves the actual runtime directory: $runtime', ({runtime, expected, paths}) => {
  const writes: Array<[string, string]> = [];
  const dependencies = {
    'node:child_process': {execFileSync: () => 'v24.19.0\n'},
    'node:fs': {
      realpathSync: (value: string) => value,
      appendFileSync: (file: string, value: string) => { writes.push([file, value]) },
    },
    'node:path': paths,
  };
  const process = {execPath: runtime, env: {GITHUB_PATH: 'path-output', GITHUB_OUTPUT: 'step-output', 'INPUT_NODE-VERSION': '24'}, stdout: {write: () => {}}, exitCode: 0};
  runInNewContext(source, {require: (name: keyof typeof dependencies) => dependencies[name], process});
  expect(process.exitCode).toBe(0);
  expect(writes).toContainEqual(['path-output', `${expected}\n`]);
});
