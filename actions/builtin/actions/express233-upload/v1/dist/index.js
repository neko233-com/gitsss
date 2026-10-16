// Copyright 2026 The GitSSS Authors. SPDX-License-Identifier: MIT
'use strict';
const fs = require('node:fs');
const fsp = require('node:fs/promises');
const path = require('node:path');
const crypto = require('node:crypto');
const {setTimeout: delay} = require('node:timers/promises');

const input = (env, name) => (env[`INPUT_${name.toUpperCase().replaceAll(' ', '_')}`] || '').trim();
const lines = value => value.split(/\r?\n/).map(s => s.trim()).filter(Boolean);

function settings(env) {
  const server = new URL(input(env, 'server'));
  if (!['http:', 'https:'].includes(server.protocol) || server.username || server.password || server.search || server.hash) throw new Error('server must be a credential-free HTTP/HTTPS base URL');
  const project = input(env, 'project-id'), version = input(env, 'version');
  if (!/^[1-9]\d{0,15}$/.test(project)) throw new Error('project-id must be a positive integer');
  if (!/^[a-zA-Z0-9][a-zA-Z0-9._+-]{0,127}$/.test(version) || ['.', '..'].includes(version)) throw new Error('version must contain 1–128 letters, digits, dots, underscores, plus or hyphens');
  const publish = input(env, 'publish') || 'false';
  const timeout = Number(input(env, 'timeout-seconds') || '300');
  const retries = Number(input(env, 'retries') || '2');
  if (!['true', 'false'].includes(publish) || !Number.isInteger(timeout) || timeout < 10 || timeout > 3600 || !Number.isInteger(retries) || retries < 0 || retries > 5) throw new Error('invalid publish, timeout-seconds or retries input');
  const token = input(env, 'token'), username = input(env, 'username'), password = input(env, 'password');
  if (token ? username || password : !username || !password) throw new Error('provide token OR username and password through Actions secrets');
  if ([token, username, password].some(s => /[\r\n\x00]/.test(s))) throw new Error('authentication inputs contain invalid characters');
  const tags = lines(input(env, 'tags'));
  if (tags.length > 32 || tags.some(s => s.length > 120 || /[\r\n\x00]/.test(s))) throw new Error('invalid file tags');
  return {server: server.href.replace(/\/$/, ''), project, version, publish: publish === 'true', timeout: timeout * 1000, retries, tags,
    authorization: token ? `Bearer ${token}` : `Basic ${Buffer.from(`${username}:${password}`).toString('base64')}`};
}

async function filesInWorkspace(workspace, paths) {
  const root = await fsp.realpath(workspace), files = new Map();
  const within = p => p === root || !path.relative(root, p).startsWith(`..${path.sep}`) && path.relative(root, p) !== '..' && !path.isAbsolute(path.relative(root, p));
  async function visit(filename) {
    if (!within(filename)) throw new Error('upload paths must stay inside GITHUB_WORKSPACE');
    const relative = path.relative(root, filename).split(path.sep).join('/');
    if (relative.split('/').includes('.git') || /[\r\n\x00]/.test(relative)) throw new Error('unsupported upload path');
    // Check every parent too: an intermediate symlink must never escape the workspace.
    let parent = root;
    for (const part of path.relative(root, filename).split(path.sep).filter(Boolean)) {
      parent = path.join(parent, part);
      if ((await fsp.lstat(parent)).isSymbolicLink()) throw new Error('symlinks are not supported in uploads');
    }
    const stat = await fsp.lstat(filename);
    if (stat.isDirectory()) {
      for (const name of (await fsp.readdir(filename)).sort()) {
        if (name === '.git') continue;
        await visit(path.join(filename, name));
      }
    } else if (stat.isFile()) {
      if (files.size >= 10000 && !files.has(relative)) throw new Error('upload exceeds 10000 files');
      const hash = crypto.createHash('sha256');
      for await (const chunk of fs.createReadStream(filename)) hash.update(chunk);
      files.set(relative, {path: relative, size: stat.size, sha256: hash.digest('hex'), filename});
    } else throw new Error('only regular files and directories can be uploaded');
  }
  if (!paths.length) throw new Error('path must name at least one file or directory');
  for (const value of paths) {
    if (path.isAbsolute(value) || value.split(/[\\/]/).includes('..')) throw new Error('upload paths must be workspace-relative without parent segments');
    await visit(path.resolve(root, value));
  }
  if (!files.size) throw new Error('no files to upload');
  return [...files.values()].sort((a, b) => a.path.localeCompare(b.path));
}

async function boundedJSON(response) {
  let size = 0;
  const parts = [];
  for await (const chunk of response.body) {
    size += chunk.length;
    if (size > 1024 * 1024) throw new Error('receiver response exceeds 1 MiB');
    parts.push(chunk);
  }
  try { return JSON.parse(Buffer.concat(parts).toString('utf8')); }
  catch { throw new Error('receiver returned invalid JSON'); }
}

async function request(config, suffix, operation, bodyFactory, retryable = false) {
  for (let attempt = 0; ; attempt++) {
    let response;
    try {
      const body = bodyFactory();
      response = await fetch(`${config.server}/api/projects/${config.project}/versions${suffix}`, {
        method: 'POST', redirect: 'manual', signal: AbortSignal.timeout(config.timeout),
        headers: {Authorization: config.authorization, ...body.headers}, body: body.value, duplex: 'half',
      });
    } catch {
      if (retryable && attempt < config.retries) { await delay(250 * 2 ** attempt); continue; }
      throw new Error(`${operation}: connection failed or timed out; inspect the receiver before rerunning`);
    }
    if (response.status >= 200 && response.status < 300) return boundedJSON(response);
    await response.body.cancel();
    if (retryable && attempt < config.retries && (response.status === 429 || response.status >= 500)) { await delay(250 * 2 ** attempt); continue; }
    throw new Error(`${operation}: HTTP ${response.status}; receiver details omitted to protect secrets`);
  }
}

function multipart(file, tags) {
  const boundary = `gitsss-${crypto.randomBytes(18).toString('hex')}`;
  const field = (name, value) => `--${boundary}\r\nContent-Disposition: form-data; name="${name}"\r\n\r\n${value}\r\n`;
  const filename = path.basename(file.path).replaceAll('"', '%22').replaceAll('\\', '%5C');
  const head = Buffer.from(field('path', file.path) + tags.map(tag => field('tags', tag)).join('') + `--${boundary}\r\nContent-Disposition: form-data; name="file"; filename="${filename}"\r\nContent-Type: application/octet-stream\r\n\r\n`);
  const tail = Buffer.from(`\r\n--${boundary}--\r\n`);
  async function* stream() {
    const hash = crypto.createHash('sha256');
    let size = 0;
    yield head;
    for await (const chunk of fs.createReadStream(file.filename)) {
      size += chunk.length;
      hash.update(chunk);
      yield chunk;
    }
    if (size !== file.size || hash.digest('hex') !== file.sha256) throw new Error('input file changed during upload');
    yield tail;
  }
  return {headers: {'Content-Type': `multipart/form-data; boundary=${boundary}`, 'Content-Length': String(head.length + file.size + tail.length)}, value: stream()};
}

async function run(env = process.env) {
  for (const name of ['token', 'username', 'password']) {
    const secret = input(env, name);
    if (secret) console.log(`::add-mask::${secret.replaceAll('%', '%25').replaceAll('\r', '%0D').replaceAll('\n', '%0A')}`);
  }
  const config = settings(env);
  const files = await filesInWorkspace(env.GITHUB_WORKSPACE || process.cwd(), lines(input(env, 'path')));
  const jsonBody = value => () => ({headers: {'Content-Type': 'application/json'}, value: JSON.stringify(value)});
  // Creation and publication may have side effects, so neither operation is retried.
  const created = await request(config, '', 'create version', jsonBody({name: config.version}));
  if (created.version !== config.version || created.status !== 'draft') throw new Error('receiver did not create the requested draft version');
  const suffix = `/${encodeURIComponent(config.version)}`;
  for (const file of files) {
    await request(config, `${suffix}/files`, 'upload file', () => multipart(file, config.tags), true);
  }
  if (config.publish) {
    const published = await request(config, `${suffix}/publish`, 'publish version', jsonBody({}));
    if (published.status !== 'published') throw new Error('receiver did not confirm publication');
  }
  const manifest = path.join(env.RUNNER_TEMP || require('node:os').tmpdir(), `express233-upload-${crypto.randomUUID()}.json`);
  await fsp.mkdir(path.dirname(manifest), {recursive: true});
  await fsp.writeFile(manifest, JSON.stringify({version: config.version, repository: env.GITHUB_REPOSITORY || '', ref: env.GITHUB_REF || '', commit: env.GITHUB_SHA || '', files: files.map(({filename, ...record}) => record)}, null, 2) + '\n', {mode: 0o600});
  if (env.GITHUB_OUTPUT) await fsp.appendFile(env.GITHUB_OUTPUT, `version=${config.version}\nfile-count=${files.length}\nmanifest-path=${manifest}\n`);
  if (env.GITHUB_STEP_SUMMARY) await fsp.appendFile(env.GITHUB_STEP_SUMMARY, `### Express233\nVersion: \`${config.version}\` · ${files.length} input files · ${config.publish ? 'Published' : 'Draft'}\n`);
  console.log(`Express233: ${files.length} input files uploaded; version ${config.publish ? 'published' : 'left as draft'}.`);
  return {manifest, files};
}

module.exports = {settings, filesInWorkspace, run};
if (require.main === module) run().catch(error => {
  console.error(`::error::${error.message.replaceAll('%', '%25').replaceAll('\r', '%0D').replaceAll('\n', '%0A')}`);
  process.exitCode = 1;
});
