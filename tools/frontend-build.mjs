import {createHash} from 'node:crypto';
import {globSync, readFileSync, writeFileSync} from 'node:fs';
import {join, resolve} from 'node:path';

const root = resolve(import.meta.dirname, '..');
const assets = join(root, 'public/assets');

function sourceState() {
  const files = [...globSync(['web_src/**/*.*', 'assets/**/*.*', 'public/assets/img/logo.*', 'public/assets/img/favicon.*'], {cwd: root}), 'package.json', 'pnpm-lock.yaml', 'vite.config.ts', 'vite.react.config.ts', 'tailwind.config.ts', 'tools/frontend-build.mjs'].sort();
  const hash = createHash('sha256');
  for (const file of files) hash.update(file.replaceAll('\\', '/')).update('\0').update(readFileSync(join(root, file))).update('\0');
  return {schema_version: 1, source_sha256: hash.digest('hex'), source_files: files.length};
}

export function writeFrontendBuildStamp() {
  writeFileSync(join(assets, 'frontend-build.json'), `${JSON.stringify(sourceState())}\n`);
}

export function verifyFrontendBuild() {
  const expected = sourceState();
  const stamp = JSON.parse(readFileSync(join(assets, 'frontend-build.json'), 'utf8'));
  if (stamp.source_sha256 !== expected.source_sha256) throw new Error('Frontend assets are stale; rebuild both Vite bundles before embedding.');
  const manifest = JSON.parse(readFileSync(join(assets, '.vite/manifest.json'), 'utf8'));
  for (const entry of Object.values(manifest)) {
    for (const file of [entry.file, ...(entry.css || []), ...(entry.assets || [])]) {
      const target = resolve(assets, file);
      if (!target.startsWith(`${assets}/`) && !target.startsWith(`${assets}\\`)) throw new Error('Frontend manifest contains an unsafe path');
      readFileSync(target);
    }
  }
  const index = manifest['web_src/js/index.ts'];
  const css = index.css.map((file) => readFileSync(join(assets, file), 'utf8')).join('\n');
  if (!css.includes('.workspace-sidebar') || !css.includes('.workspace-search')) throw new Error('Current workspace UI styles are absent from the index bundle');
  readFileSync(join(assets, 'js/gitsss-react.js'));
  console.info(`Frontend verified: ${stamp.source_sha256} (${Object.keys(manifest).length} entries)`);
  return stamp;
}

if (process.argv[1] && resolve(process.argv[1]) === import.meta.filename) {
  if (process.argv[2] === 'write') writeFrontendBuildStamp();
  else verifyFrontendBuild();
}
