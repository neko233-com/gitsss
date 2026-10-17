// Copyright 2026 The GitSSS Authors. SPDX-License-Identifier: MIT
const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const os = require('node:os');
const http = require('node:http');
const crypto = require('node:crypto');
const {settings, filesInWorkspace, run} = require('../../actions/builtin/actions/express233-upload/v1/dist/index.js');

test('preflight rejects invalid URLs, auth, paths, symlinks and empty uploads', async (t) => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'express-upload-'));
  t.after(() => fs.rm(root, {recursive: true, force: true}));
  const env = {INPUT_SERVER: 'http://example.invalid', 'INPUT_PROJECT-ID': '1', INPUT_VERSION: '1.0.0', INPUT_TOKEN: crypto.randomUUID()};
  assert.equal(settings(env).version, '1.0.0');
  for (const patch of [{INPUT_SERVER: 'file:///tmp/x'}, {INPUT_SERVER: 'http://user:pass@example.invalid'}, {INPUT_VERSION: '../x'}, {'INPUT_PROJECT-ID': '0'}, {INPUT_PUBLISH: 'maybe'}, {INPUT_USERNAME: 'mixed-auth'}]) {
    assert.throws(() => settings({...env, ...patch}));
  }
  await assert.rejects(filesInWorkspace(root, ['..']));
  await assert.rejects(filesInWorkspace(root, ['.']));
  await fs.mkdir(path.join(root, '.git'));
  await fs.writeFile(path.join(root, '.git', 'config'), 'private');
  await assert.rejects(filesInWorkspace(root, ['.git/config']));
  await fs.mkdir(path.join(root, 'output'));
  await fs.writeFile(path.join(root, 'output', 'a.bin'), 'hello');
  const files = await filesInWorkspace(root, ['output', 'output/a.bin']);
  assert.equal(files.length, 1);
  assert.equal(files[0].path, 'output/a.bin');
  assert.equal(files[0].sha256, crypto.createHash('sha256').update('hello').digest('hex'));
  if (process.platform !== 'win32') {
    await fs.symlink(os.tmpdir(), path.join(root, 'escape'));
    await assert.rejects(filesInWorkspace(root, ['escape']));
    await assert.rejects(filesInWorkspace(root, ['escape/anything']));
  }
});

test('streams multipart, retries only uploads, never follows redirects or overwrites versions', async (t) => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'express-upload-http-'));
  t.after(() => fs.rm(root, {recursive: true, force: true}));
  const payload = crypto.randomBytes(1024 * 1024 + 31), secret = crypto.randomUUID();
  await fs.mkdir(path.join(root, 'build'));
  await fs.writeFile(path.join(root, 'build', 'binary.dat'), payload);
  let mode = 'normal', creates = 0, uploads = 0, publishes = 0, redirects = 0;
  const server = http.createServer(async (req, res) => {
    assert.equal(req.headers.authorization, `Bearer ${secret}`);
    const chunks = await Array.fromAsync(req);
    const body = Buffer.concat(chunks);
    res.setHeader('Content-Type', 'application/json');
    if (req.url === '/redirect-target') { redirects++; res.end('{}'); return }
    if (req.url.endsWith('/versions')) {
      creates++;
      if (mode === 'duplicate') { res.writeHead(400).end(JSON.stringify({error: secret})); return }
      res.writeHead(201).end(JSON.stringify({version: '1.0.0', status: 'draft'})); return;
    }
    if (req.url.endsWith('/publish')) { publishes++; res.end('{"status":"published"}'); return }
    uploads++;
    if (mode === 'redirect') { res.writeHead(307, {Location: '/redirect-target'}).end('{}'); return }
    if (mode === 'unauthorized') { res.writeHead(401).end(JSON.stringify({error: secret})); return }
    if (uploads === 1) { res.writeHead(503).end('{}'); return }
    assert.match(req.headers['content-type'], /^multipart\/form-data; boundary=/);
    assert.equal(Number(req.headers['content-length']), body.length);
    assert(body.includes(Buffer.from('build/binary.dat')));
    assert(body.includes(payload));
    assert(body.includes(Buffer.from('name="tags"')));
    res.end('{}');
  });
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  t.after(() => new Promise((resolve) => server.close(resolve)));
  const env = {INPUT_SERVER: `http://127.0.0.1:${server.address().port}`, 'INPUT_PROJECT-ID': '1', INPUT_VERSION: '1.0.0', INPUT_TOKEN: secret,
    INPUT_PATH: 'build', INPUT_TAGS: 'linux\nrelease', INPUT_RETRIES: '1', GITHUB_WORKSPACE: root, RUNNER_TEMP: root,
    GITHUB_OUTPUT: path.join(root, 'outputs'), GITHUB_STEP_SUMMARY: path.join(root, 'summary')};
  /* eslint-disable no-console -- Capture output from the CommonJS action fixture. */
  const originalLog = console.log;
  console.log = () => {};
  t.after(() => { console.log = originalLog });
  /* eslint-enable no-console */
  const result = await run(env);
  assert.equal(creates, 1); assert.equal(uploads, 2); assert.equal(publishes, 0);
  const manifest = JSON.parse(await fs.readFile(result.manifest, 'utf8'));
  assert.equal(manifest.files[0].sha256, crypto.createHash('sha256').update(payload).digest('hex'));
  assert.match(await fs.readFile(env.GITHUB_OUTPUT, 'utf8'), /file-count=1/);
  await run({...env, INPUT_PUBLISH: 'true'});
  assert.equal(publishes, 1);
  for (const failure of ['redirect', 'unauthorized', 'duplicate']) {
    mode = failure;
    const previousUploads = uploads, previousCreates = creates;
    await assert.rejects(run(env), (error) => !error.message.includes(secret) && /HTTP/.test(error.message));
    assert.equal(creates, previousCreates + 1);
    assert.equal(uploads, previousUploads + (failure === 'duplicate' ? 0 : 1));
  }
  assert.equal(redirects, 0);
});
