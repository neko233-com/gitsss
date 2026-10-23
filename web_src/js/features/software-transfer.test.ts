import {GET, POST} from '../modules/fetch.ts';
import {TransferCache} from './software-transfer-cache.ts';
import {SoftwareTransfers} from './software-transfer.ts';

vi.mock('../modules/fetch.ts', () => ({GET: vi.fn(), POST: vi.fn(), DELETE: vi.fn()}));
vi.mock('./software-transfer-cache.ts', async (original) => ({...await original<Record<string, unknown>>(), TransferCache: vi.fn()}));
const words = new Proxy({}, {get: (_, key) => String(key)}) as Record<string, string>;

describe('unfinished transfer panel', {concurrent: false}, () => {
  let root: HTMLElement;
  let cache: {scope: string; open: ReturnType<typeof vi.fn>; check: ReturnType<typeof vi.fn>; claim: ReturnType<typeof vi.fn>; owns: ReturnType<typeof vi.fn>; bindUpload: ReturnType<typeof vi.fn>; all: ReturnType<typeof vi.fn>; prune: ReturnType<typeof vi.fn>; list: ReturnType<typeof vi.fn>; source: ReturnType<typeof vi.fn>; put: ReturnType<typeof vi.fn>; remove: ReturnType<typeof vi.fn>};
  beforeEach(() => {
    vi.useFakeTimers(); vi.clearAllMocks();
    root = document.createElement('div'); root.setAttribute('data-user-id', '1'); root.setAttribute('data-error', 'failed');
    root.innerHTML = '<p data-software-status></p><input data-software-search><details data-transfer-panel hidden><summary data-transfer-summary></summary><button data-transfer-pause></button><button data-transfer-resume></button><section data-transfer-uploads></section><section data-transfer-downloads></section></details>'; document.body.append(root);
    cache = {scope: '1:org', open: vi.fn(), check: vi.fn(), claim: vi.fn().mockResolvedValue(true), owns: vi.fn().mockReturnValue(true), bindUpload: vi.fn(), all: vi.fn().mockResolvedValue([]), prune: vi.fn(), list: vi.fn().mockResolvedValue([]), source: vi.fn(), put: vi.fn(), remove: vi.fn()};
    // eslint-disable-next-line prefer-arrow-callback -- The cache mock is called with new.
    vi.mocked(TransferCache).mockImplementation(function () {return cache as unknown as TransferCache}); vi.mocked(GET).mockImplementation(async () => Response.json([]));
  });
  afterEach(() => {vi.clearAllTimers(); vi.useRealTimers(); vi.unstubAllGlobals(); document.body.replaceChildren()});

  test('completed legacy uploads are purged on restore and never painted', async () => {
    cache.list.mockResolvedValue([{key: 'old', kind: 'upload', state: 'complete', offset: 5, data: {name: 'done.exe', size: 5}}]);
    new SoftwareTransfers(root, '/org/-/software', words, () => {});
    await vi.waitFor(() => expect(cache.remove).toHaveBeenCalledWith('old'));
    expect(root.querySelectorAll('article')).toHaveLength(0); expect(root.querySelector<HTMLElement>('[data-transfer-panel]')!.hidden).toBe(true); expect(cache.source).not.toHaveBeenCalled();
  });

  test('paused uploads remain visible and keep recovery controls and icons', async () => {
    cache.list.mockResolvedValue([{key: 'paused', kind: 'upload', state: 'paused', offset: 3, data: {name: 'sdk.zip', size: 10, folder: ''}}]);
    new SoftwareTransfers(root, '/org/-/software', words, () => {});
    await vi.waitFor(() => expect(root.querySelector('[data-state="paused"]')).not.toBeNull());
    expect(root.querySelector<HTMLElement>('[data-transfer-panel]')!.hidden).toBe(false); expect(root.querySelector('.software-file-kind[data-kind="archive"] svg')).not.toBeNull();
    expect(root.textContent).toContain('choose_original'); expect(root.textContent).not.toContain('uploading: 0');
  });

  test('successful upload disappears automatically and removes its source cache', async () => {
    class HashWorker extends EventTarget {
      postMessage(data: {id: number; op: string; data?: ArrayBuffer}) {queueMicrotask(() => this.dispatchEvent(new MessageEvent('message', {data: {id: data.id, value: data.op === 'finish' ? 'digest' : 'key', data: data.data}})))}
      terminate() {}
    }
    vi.stubGlobal('Worker', HashWorker);
    vi.mocked(POST).mockImplementation(async (url) => Response.json(url.endsWith('/complete') ? {digest: 'digest'} : {id: 'session', size: 0, offset: 0, chain: '', client_key: 'key', chunk_size: 8388608}));
    const manager = new SoftwareTransfers(root, '/org/-/software', words, () => {});
    await manager.enqueue([{file: new File([], 'empty.txt'), folder: ''}]);
    await vi.waitFor(() => expect(cache.remove).toHaveBeenCalled());
    expect(root.querySelector('article')).toBeNull(); expect(root.querySelector<HTMLElement>('[data-transfer-panel]')!.hidden).toBe(true);
    expect(cache.remove.mock.calls[0]).toHaveLength(1);
  });

  test('another tab active upload is not offered as an uncached recovery task', async () => {
    cache.all.mockResolvedValue([{key: 'foreign', kind: 'upload', data: {folder: '', name: 'elsewhere.zip', size: 12}}]);
    vi.mocked(GET).mockImplementation(async () => Response.json([{id: 'session', parent: '', name: 'elsewhere.zip', size: 12, offset: 0}]));
    const manager = new SoftwareTransfers(root, '/org/-/software', words, () => {});
    await manager.enqueue([]);
    expect(root.querySelector('article')).toBeNull();
  });

  test('orphan queues are recovered on heartbeat without duplicating paused tasks', async () => {
    const manager = new SoftwareTransfers(root, '/org/-/software', words, () => {});
    await manager.enqueue([]);
    cache.list.mockResolvedValue([{key: 'orphan', kind: 'upload', state: 'paused', offset: 0, data: {folder: '', name: 'orphan.zip', size: 12}}]);
    await vi.advanceTimersByTimeAsync(3100);
    expect(root.querySelectorAll('article')).toHaveLength(1);
    await vi.advanceTimersByTimeAsync(3100);
    expect(root.querySelectorAll('article')).toHaveLength(1);
    expect(root.querySelector('article')!.getAttribute('data-state')).toBe('paused');
    cache.owns.mockReturnValue(false); cache.list.mockResolvedValue([]);
    await vi.advanceTimersByTimeAsync(3100);
    expect(root.querySelectorAll('article')).toHaveLength(0);
  });
});
