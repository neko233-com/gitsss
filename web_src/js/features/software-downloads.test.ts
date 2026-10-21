import {GET, POST, DELETE} from '../modules/fetch.ts';
import {SoftwareDownloads} from './software-downloads.ts';
import type {TransferCache} from './software-transfer-cache.ts';

vi.mock('../modules/fetch.ts', () => ({GET: vi.fn(), POST: vi.fn(), DELETE: vi.fn()}));
const words = new Proxy({}, {get: (_, key) => String(key)}) as Record<string, string>;
const job = {id: 'ticket', ids: ['node'], name: 'sdk.bin', size: 3, etag: '"sha256-fixture"', expires: Math.floor(Date.now() / 1000) + 3600};
const reply = () => new Response(new Uint8Array([1, 2, 3]), {status: 206, headers: {ETag: job.etag, 'Content-Range': 'bytes 0-2/3'}});
const setup = () => {
  const root = document.createElement('div'); root.innerHTML = '<section data-transfer-downloads></section>'; document.body.append(root);
  const cache = {scope: 'user:organization', check: vi.fn(), put: vi.fn(), append: vi.fn(), remove: vi.fn()};
  const manager = new SoftwareDownloads(root, '/organization/-/software', words, cache as unknown as TransferCache, String, () => {});
  vi.mocked(POST).mockResolvedValue(Response.json(job, {status: 201}));
  vi.mocked(DELETE).mockImplementation(async () => new Response('{}'));
  return {root, cache, manager};
};
const button = (root: HTMLElement, text: string) => Array.from(root.querySelectorAll('button')).find((item) => item.textContent === text)!;

afterEach(() => {vi.resetAllMocks(); document.body.replaceChildren()});

test('managed download commits a checked range before becoming ready, without native download', {concurrent: false}, async () => {
  const {manager, root, cache} = setup(); vi.mocked(GET).mockImplementation(async () => reply());
  await manager.create(['node']);
  await vi.waitFor(() => expect(root.querySelector('article')!.getAttribute('data-state')).toBe('ready'));
  expect(cache.append).toHaveBeenCalledOnce(); expect(cache.append.mock.calls[0][1]).toBe(0);
  expect(cache.append.mock.calls[0][2].size).toBe(3);
  expect(root.querySelector('a')).toBeNull(); expect(button(root, 'save_file')).toBeTruthy();
  expect(GET).toHaveBeenCalledWith('/organization/-/software/downloads/ticket/file', expect.objectContaining({headers: {Range: 'bytes=0-2', 'If-Range': job.etag, 'X-GitSSS-Transfer': 'managed'}}));
  await manager.clearCompleted(); expect(cache.remove).not.toHaveBeenCalled();
});

test('changed content is refused and retry keeps the committed offset', {concurrent: false}, async () => {
  const {manager, root, cache} = setup();
  vi.mocked(GET).mockImplementation(async () => new Response('bad', {status: 206, headers: {ETag: '"changed"', 'Content-Range': 'bytes 0-2/3'}}));
  await manager.create(['node']);
  await vi.waitFor(() => expect(root.querySelector('article')!.getAttribute('data-state')).toBe('failed'));
  expect(cache.append).not.toHaveBeenCalled(); expect(root.textContent).toContain('download_changed');
  vi.mocked(GET).mockImplementation(async () => reply()); button(root, 'retry').click();
  await vi.waitFor(() => expect(root.querySelector('article')!.getAttribute('data-state')).toBe('ready'));
  expect(cache.append).toHaveBeenCalledOnce();
});

test('pause aborts in-flight reception, resume retries, cancellation clears only the task', {concurrent: false}, async () => {
  const {manager, root, cache} = setup();
  vi.mocked(GET).mockImplementation((_, opts) => new Promise((_, reject) => {opts!.signal!.addEventListener('abort', () => reject(new DOMException('Paused', 'AbortError')))}));
  await manager.create(['node']); await vi.waitFor(() => expect(GET).toHaveBeenCalledOnce());
  button(root, 'pause').click();
  await vi.waitFor(() => expect(button(root, 'resume')).toBeTruthy()); expect(cache.append).not.toHaveBeenCalled();
  vi.mocked(GET).mockImplementation(async () => reply()); button(root, 'resume').click();
  await vi.waitFor(() => expect(root.querySelector('article')!.getAttribute('data-state')).toBe('ready'));
  button(root, 'cancel_transfer').click(); await vi.waitFor(() => expect(root.querySelector('article')).toBeNull());
  expect(cache.remove).toHaveBeenCalledWith('download-ticket');
  expect(DELETE).toHaveBeenCalledWith('/organization/-/software/downloads/ticket');
});

test('restore preserves manual pause and resumes interrupted downloads from committed bytes', {concurrent: false}, async () => {
  const {manager, root} = setup(); vi.mocked(GET).mockImplementation(async () => reply());
  await manager.restore([{scope: 'user:organization', key: 'paused', kind: 'download', state: 'paused', offset: 0, updated: Date.now(), data: job}]);
  expect(GET).not.toHaveBeenCalled(); expect(button(root, 'resume')).toBeTruthy();
  await manager.restore([{scope: 'user:organization', key: 'active', kind: 'download', state: 'downloading', offset: 3, updated: Date.now(), data: job}]);
  await vi.waitFor(() => expect(root.querySelector('[data-download-task="active"]')!.getAttribute('data-state')).toBe('ready'));
  expect(GET).not.toHaveBeenCalled();
});

test('temporary overload retries the same range before committing once', {concurrent: false}, async () => {
  vi.useFakeTimers();
  try {
    const {manager, root, cache} = setup();
    vi.mocked(GET).mockResolvedValueOnce(new Response('busy', {status: 503})).mockImplementation(async () => reply());
    await manager.create(['node']); await vi.advanceTimersByTimeAsync(550);
    expect(root.querySelector('article')!.getAttribute('data-state')).toBe('ready');
    expect(GET).toHaveBeenCalledTimes(2); expect(cache.append).toHaveBeenCalledOnce();
  } finally {vi.useRealTimers()}
});

test('full browser storage retains the committed offset and allows retry', {concurrent: false}, async () => {
  const {manager, root, cache} = setup(); vi.mocked(GET).mockImplementation(async () => reply());
  cache.append.mockRejectedValueOnce(new DOMException('Full', 'QuotaExceededError'));
  await manager.create(['node']);
  await vi.waitFor(() => expect(root.querySelector('article')!.getAttribute('data-state')).toBe('failed'));
  expect(root.textContent).toContain('cache_full'); expect(GET).toHaveBeenCalledOnce();
  button(root, 'retry').click(); await vi.waitFor(() => expect(root.querySelector('article')!.getAttribute('data-state')).toBe('ready'));
  expect(cache.append.mock.calls.map((call) => call[1])).toEqual([0, 0]);
});

test('saved history disappears on restore while ready-to-save downloads remain available', {concurrent: false}, async () => {
  const {manager, root, cache} = setup();
  await manager.restore([{scope: 'user:organization', key: 'saved', kind: 'download', state: 'saved', offset: 3, updated: Date.now(), data: job}, {scope: 'user:organization', key: 'ready', kind: 'download', state: 'ready', offset: 3, updated: Date.now(), data: job}]);
  expect(cache.remove).toHaveBeenCalledWith('saved'); expect(root.querySelector('[data-download-task="saved"]')).toBeNull(); expect(manager.pending).toBe(1); expect(manager.ready).toBe(1); expect(button(root, 'save_file')).toBeTruthy();
});

test('confirming a local save automatically retires the download and releases cached bytes', {concurrent: false}, async () => {
  const {manager, root, cache} = setup();
  await manager.restore([{scope: 'user:organization', key: 'ready', kind: 'download', state: 'ready', offset: 3, updated: Date.now(), data: {...job, save_offered: true}}]);
  button(root, 'confirm_saved').click(); await vi.waitFor(() => expect(root.querySelector('article')).toBeNull());
  expect(cache.remove).toHaveBeenCalledWith('ready'); expect(manager.pending).toBe(0);
});
