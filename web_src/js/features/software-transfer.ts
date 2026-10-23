import {GET, POST, DELETE} from '../modules/fetch.ts';
import {confirmModal} from './comp/ConfirmModal.ts';
import type {HashRequest, HashReply} from './software-hash-worker.ts';
import {TransferCache, TransferOwnershipError, type CachedTransfer} from './software-transfer-cache.ts';
import {SoftwareDownloads} from './software-downloads.ts';
import {softwareAction, softwareFileIcon} from './software-presentation.ts';
import type {SvgName} from '../svg.ts';

export type UploadSession = {id: string; name: string; parent: string; size: number; offset: number; chain: string; client_key: string; chunk_size: number; node?: {digest: string}};
type UploadTask = {key: string; name: string; folder: string; size: number; file?: File; session?: UploadSession; paused: boolean; cancelled: boolean; running: boolean; detached?: boolean; state: string; row?: HTMLElement; xhr?: XMLHttpRequest; bytes: number; speed?: number; cached?: boolean; cacheError?: boolean; cacheAbort?: AbortController};
export const formatTransferBytes = (size: number): string => {
  const power = size >= 2 ** 30 ? 30 : size >= 2 ** 20 ? 20 : size >= 2 ** 10 ? 10 : 0;
  return `${(size / 2 ** power).toLocaleString(undefined, {maximumFractionDigits: 1})} ${({0: 'B', 10: 'KiB', 20: 'MiB', 30: 'GiB'})[power]}`;
};

class HashClient {
  private worker = new Worker(new URL('software-hash-worker.ts', import.meta.url), {type: 'module'});
  private sequence = 0;
  private requests = new Map<number, {resolve: (reply: HashReply) => void; reject: (error: Error) => void}>();
  constructor() {
    this.worker.addEventListener('message', ({data}: MessageEvent<HashReply>) => {
      const pending = this.requests.get(data.id); if (!pending) return;
      this.requests.delete(data.id);
      if (data.error) pending.reject(new Error(data.error)); else pending.resolve(data);
    });
    this.worker.addEventListener('error', () => {for (const request of this.requests.values()) request.reject(new Error('Hash worker failed')); this.requests.clear()});
  }
  request(request: Omit<HashRequest, 'id'>): Promise<HashReply> {
    return new Promise((resolve, reject) => {
      const id = ++this.sequence; this.requests.set(id, {resolve, reject});
      this.worker.postMessage({...request, id}, request.data ? [request.data] : []);
    });
  }
  async hash(blob: Blob, accumulate = false) {return this.request({op: 'hash', data: await blob.arrayBuffer(), accumulate})}
  close() {this.worker.terminate(); for (const request of this.requests.values()) request.reject(new Error('Hash worker stopped')); this.requests.clear()}
}

const actionButton = (label: string, action: () => void, icon: SvgName = 'octicon-x'): HTMLButtonElement => softwareAction(label, icon, action);

export class SoftwareTransfers {
  private root: HTMLElement;
  private base: string;
  private words: Record<string, string>;
  private changed: () => void;
  private uploads: UploadTask[] = [];
  private downloads!: SoftwareDownloads;
  private cache: TransferCache;
  private cacheAvailable = false;
  private leaving = false;
  private active = 0;
  private sequence = 0;
  private queuePaused = false;
  private ready: Promise<void>;
  private uploading: HTMLElement;
  private summary: HTMLElement;
  constructor(root: HTMLElement, base: string, words: Record<string, string>, changed: () => void) {
    this.root = root; this.base = base; this.words = words; this.changed = changed;
    this.uploading = root.querySelector('[data-transfer-uploads]')!;
    this.cache = new TransferCache(`${root.getAttribute('data-user-id')}:${base}`);
    this.downloads = new SoftwareDownloads(root, base, words, this.cache, formatTransferBytes, () => this.recount());
    window.addEventListener('pagehide', async () => {this.leaving = true; for (const task of this.uploads) {task.paused = true; task.xhr?.abort()} this.downloads.suspend(); try {await this.cache.release()} catch {}});
    this.summary = root.querySelector('[data-transfer-summary]')!;
    window.addEventListener('beforeunload', (event) => {if (this.uploads.some((task) => !task.cached && task.file && task.state !== 'complete')) {event.preventDefault()}});
    root.querySelector('[data-transfer-pause]')!.addEventListener('click', () => {
      this.queuePaused = true; this.downloads.pauseAll();
      for (const task of this.uploads.filter((item) => item.state !== 'complete' && !item.cancelled)) {task.paused = true; task.xhr?.abort(); task.state = 'paused'; if (task.row) this.show(task, 'paused'); this.persistQuietly(task)}
      this.recount();
    });
    root.querySelector('[data-transfer-resume]')!.addEventListener('click', () => {
      this.queuePaused = false; this.downloads.resumeAll();
      for (const task of this.uploads.filter((item) => item.file && !item.cancelled && (item.state === 'paused' || item.state === 'failed'))) {task.paused = false; task.state = 'queued'}
      this.pump();
    });
    window.addEventListener('pageshow', (event) => {if (event.persisted) window.location.reload();});
    this.ready = this.restore();
  }
  private async json(url: string, init?: URLSearchParams, method: 'POST' | 'GET' | 'DELETE' = 'POST'): Promise<any> {
    const response = await (method === 'GET' ? GET(url) : method === 'DELETE' ? DELETE(url) : POST(url, {data: init}));
    if (!response.ok) {let message = this.root.getAttribute('data-error')!; try {message = (await response.json()).message || message} catch {} throw new Error(message)}
    return response.json();
  }
  private recount() {
    const queued = this.uploads.filter((task) => task.state === 'queued' || task.state === 'caching').length;
    const paused = this.uploads.filter((task) => task.state === 'paused').length;
    this.root.querySelector<HTMLButtonElement>('[data-transfer-pause]')!.disabled = !this.active && !queued && !this.downloads.busy;
    this.root.querySelector<HTMLButtonElement>('[data-transfer-resume]')!.disabled = !this.downloads.paused && (this.uploads.some((task) => task.running && task.paused) || this.uploads.every((task) => !(task.file && (task.paused || task.state === 'failed') && !task.cancelled)));
    const counts: [string, number][] = [[this.words.uploading, this.active], [this.words.queued, queued], [this.words.paused, paused + this.downloads.pausedCount], [this.words.downloading, this.downloads.busy], [this.words.ready, this.downloads.ready], [this.words.failed, this.uploads.filter((task) => task.state === 'failed').length + this.downloads.failed]];
    this.summary.textContent = [this.words.transfers, ...counts.filter(([, count]) => count > 0).map(([label, count]) => `${label}: ${count}`)].join(' · ');
    this.root.querySelector<HTMLElement>('[data-transfer-panel]')!.hidden = this.uploads.every((task) => !(task.state !== 'complete' && !task.cancelled)) && !this.downloads.pending;
  }
  async enqueue(files: Array<{file: File; folder: string}>) {
    await this.ready;
    this.queuePaused = false;
    const index = new Map(this.uploads.filter((task) => !task.cancelled && task.state !== 'complete').map((task) => [JSON.stringify([task.folder, task.name, task.size]), task]));
    for (const {file, folder} of files) {
      const existing = index.get(JSON.stringify([folder, file.name.normalize('NFC'), file.size]));
      if (existing) {if (!existing.running) {existing.file = file; existing.cached = false; existing.cacheError = false; existing.paused = false; existing.state = 'queued'} continue}
      this.uploads.push({key: `upload-${Date.now()}-${++this.sequence}-${Math.random()}`, name: file.name.normalize('NFC'), folder, size: file.size, file, paused: false, cancelled: false, running: false, state: 'queued', bytes: 0});
    }
    for (const task of this.uploads.filter((item) => item.state === 'queued' && item.file && !item.cached)) await this.stage(task);
    this.pump();
  }
  private async stage(task: UploadTask) {
    if (task.cancelled) return;
    try {
      if (this.cacheAvailable) {
        task.state = 'caching'; task.cacheAbort = new AbortController();
        if (this.uploads.indexOf(task) < 40) this.show(task, 'caching'); this.recount();
        await this.cache.put(this.record(task, task.paused ? 'paused' : 'queued'), task.file, task.cacheAbort.signal);
        if (task.cancelled) {await this.cache.remove(task.key); return}
        task.cached = true; task.state = task.paused ? 'paused' : 'queued'; await this.persist(task); if (task.paused && task.row) this.show(task, 'paused');
      }
    } catch (error) {if (error instanceof TransferOwnershipError) this.detach(task); else if (!task.cancelled) {task.cacheError = true; task.state = task.paused ? 'paused' : 'queued'}} finally {task.cacheAbort = undefined}
  }
  private record(task: UploadTask, state = task.state): CachedTransfer {return {scope: this.cache.scope, key: task.key, kind: 'upload', state, offset: task.bytes, updated: Date.now(), data: {name: task.name, folder: task.folder, size: task.size, session: task.session}}}
  private async persist(task: UploadTask) {if (task.cached && !this.leaving && !task.detached) await this.cache.put(this.record(task));}
  private detach(task: UploadTask) {task.detached = true; task.paused = true; task.xhr?.abort(); task.cacheAbort?.abort(); task.row?.remove(); this.uploads = this.uploads.filter((item) => item !== task)}
  private async persistQuietly(task: UploadTask) {try {await this.persist(task)} catch {}}
  private async resumeTask(task: UploadTask) {await this.persistQuietly(task); this.pump()}
  private pump() {
    while (!this.leaving && !this.queuePaused && this.active < 2) {
      const task = this.uploads.find((item) => item.state === 'queued' && (item.cached || item.cacheError || !this.cacheAvailable) && !item.running && !item.paused && !item.cancelled);
      if (!task) break;
      task.running = true; task.state = 'preparing'; this.active++;
      this.runQueued(task);
    }
    this.recount();
  }
  private async runQueued(task: UploadTask) {
    try {await this.run(task)} finally {task.running = false; this.active--; this.controls(task); this.recount(); this.changed(); this.pump()}
  }
  private row(task: UploadTask): HTMLElement {
    if (task.row) return task.row;
    const row = document.createElement('article'); row.className = 'software-transfer-row'; row.setAttribute('data-upload-task', task.key);
    const name = document.createElement('strong'); const text = document.createElement('span'); text.textContent = task.name; name.append(softwareFileIcon(task.name), text); name.title = task.name;
    const progress = document.createElement('progress'); progress.max = task.size || 1; progress.setAttribute('aria-label', task.name);
    const detail = document.createElement('span'); detail.className = 'software-transfer-detail'; detail.setAttribute('aria-live', 'off');
    const controls = document.createElement('div'); controls.className = 'software-transfer-controls';
    row.append(name, progress, detail, controls); this.uploading.append(row); task.row = row; return row;
  }
  private show(task: UploadTask, state: string, bytes = task.bytes, speed = 0, message = '') {
    if (task.detached) return;
    task.state = state; task.bytes = bytes;
    if (state === 'complete') {if (task.row?.contains(document.activeElement)) this.root.querySelector<HTMLInputElement>('[data-software-search]')?.focus(); task.row?.remove(); return}
    if (!task.row && !task.running && this.uploads.filter((entry) => entry.row?.isConnected).length >= 100) return;
    if (speed) task.speed = speed;
    const row = this.row(task); row.setAttribute('data-state', state);
    row.querySelector('progress')!.value = bytes;
    row.querySelector('.software-transfer-detail')!.textContent = `${this.words[state] || state} · ${Math.min(100, Math.floor(bytes / (task.size || 1) * 100))}% · ${formatTransferBytes(bytes)} / ${formatTransferBytes(task.size)}${speed ? ` · ${formatTransferBytes(speed)}/s` : ''}${message ? ` · ${message}` : ''}${task.cacheError || !this.cacheAvailable ? ` · ${this.words.cache_unavailable}` : ''}`;
    this.controls(task);
  }
  private controls(task: UploadTask) {
    if (!task.row) return;
    const controls = task.row.querySelector<HTMLElement>('.software-transfer-controls')!;
    // Keep focused controls in place while progress updates arrive.
    const mode = task.cancelled || task.state === 'complete' ? 'done' : task.state === 'caching' ? 'caching' : task.running && !task.paused ? 'running' : task.running ? 'stopping' : 'paused';
    if (controls.getAttribute('data-mode') === mode) return;
    controls.setAttribute('data-mode', mode); controls.replaceChildren();
    if (mode === 'done' || mode === 'stopping') return;
    if (mode === 'running') controls.append(actionButton(this.words.pause, () => {task.paused = true; task.xhr?.abort(); this.show(task, 'paused'); this.persistQuietly(task)}, 'octicon-pause'));
    else if (mode !== 'caching') controls.append(actionButton(task.file ? (task.state === 'failed' ? this.words.retry : this.words.resume) : this.words.choose_original, () => {
      if (task.file) {this.queuePaused = false; task.paused = false; task.state = 'queued'; this.resumeTask(task)} else {
        const input = document.createElement('input'); input.type = 'file';
        input.addEventListener('change', async () => {if (input.files?.[0]) {task.file = input.files[0]; task.cached = false; task.cacheError = false; this.queuePaused = false; task.paused = false; task.state = 'queued'; await this.stage(task); this.pump()}}, {once: true}); input.click();
      }
    }, task.file ? task.state === 'failed' ? 'octicon-history' : 'octicon-play' : 'octicon-file'));
    controls.append(actionButton(this.words.cancel_upload, () => {this.cancel(task)}));
  }
  private async cancel(task: UploadTask) {
    if (!await confirmModal({header: this.words.cancel_upload, content: `${this.words.cancel_upload}\n${task.name}`, confirmButtonColor: 'red'})) return;
    task.cancelled = true; task.paused = true; task.xhr?.abort(); task.cacheAbort?.abort();
    if (!task.running && !task.session) {try {if (this.cacheAvailable) await this.cache.remove(task.key); task.row?.remove(); this.uploads = this.uploads.filter((item) => item !== task)} catch (error) {task.cancelled = false; this.show(task, 'failed', task.bytes, 0, String(error))}}
    if (!task.running && task.session) {
      try {await this.json(`${this.base}/uploads/${task.session.id}`, undefined, 'DELETE'); if (task.cached) await this.cache.remove(task.key); task.row?.remove(); this.uploads = this.uploads.filter((item) => item !== task); this.changed()} catch (error) {task.cancelled = false; this.show(task, 'failed', task.bytes, 0, String(error))}
    }
    this.recount();
  }
  private ensureRunning(task: UploadTask) {if (task.paused || task.cancelled || task.detached) throw new DOMException('Paused', 'AbortError');}
  private async run(task: UploadTask) {
    const hash = new HashClient(); const file = task.file!;
    try {
      this.show(task, 'preparing');
      const head = await hash.hash(file.slice(0, 65536)); const tail = await hash.hash(file.slice(Math.max(0, file.size - 65536)));
      const key = (await hash.hash(new Blob([`${file.size}:${file.lastModified}:${head.value}:${tail.value}`]))).value;
      this.ensureRunning(task);
      if (task.session) {
        if (task.session.client_key !== key || task.session.size !== file.size) throw new Error(this.words.resume_mismatch);
        task.session = await this.json(`${this.base}/uploads/${task.session.id}`, undefined, 'GET');
      } else task.session = await this.json(`${this.base}/uploads`, new URLSearchParams({parent: task.folder, name: task.name, size: String(file.size), client_key: key}));
      if (task.cached && !task.session!.node) await this.cache.bindUpload(task.key, task.session!.id);
      await this.persist(task);
      const session = task.session!; this.ensureRunning(task);
      const size = session.chunk_size || 8 * 1024 * 1024;
      let chain = ''; let offset = 0;
      const prefix = session.node ? file.size : session.offset;
      const validatingStarted = performance.now();
      while (offset < prefix) {
        this.ensureRunning(task); if (task.cached) await this.cache.check(task.key); const end = Math.min(offset + size, prefix);
        const chunk = await hash.hash(file.slice(offset, end), true);
        chain = (await hash.request({op: 'chain', chain, digest: chunk.value, offset: end})).value;
        offset = end; this.show(task, 'validating', offset, offset / Math.max((performance.now() - validatingStarted) / 1000, .1));
      }
      if (!session.node && chain !== session.chain) throw new Error(this.words.resume_mismatch);
      const transferStarted = performance.now();
      if (session.node) {
        if ((await hash.request({op: 'finish'})).value !== session.node.digest) throw new Error(this.words.resume_mismatch);
      } else {
        while (offset < file.size) {
          this.ensureRunning(task); if (task.cached) await this.cache.check(task.key); const end = Math.min(offset + size, file.size);
          const chunk = await hash.hash(file.slice(offset, end), true);
          const expected = (await hash.request({op: 'chain', chain, digest: chunk.value, offset: end})).value;
          let acknowledged = false;
          for (let attempt = 0; attempt < 4 && !acknowledged; attempt++) {
            this.ensureRunning(task);
            if (task.cached) await this.cache.check(task.key);
            try {const next = await this.send(task, chunk.data!, offset, chunk.value); if (next.offset !== end || next.chain !== expected) throw new Error(this.words.resume_mismatch); task.session = next; acknowledged = true} catch (error) {
              this.ensureRunning(task);
              const current: UploadSession = await this.json(`${this.base}/uploads/${session.id}`, undefined, 'GET');
              if (current.offset === end && current.chain === expected) {task.session = current; acknowledged = true} else if (current.offset !== offset || attempt === 3) throw error;
              else await new Promise((resolve) => setTimeout(resolve, 500 * 2 ** attempt));
            }
          }
          chain = expected; offset = end; this.show(task, 'uploading', offset); await this.persist(task);
        }
        this.ensureRunning(task);
        const digest = (await hash.request({op: 'finish'})).value;
        if (task.cached) await this.cache.check(task.key);
        const node = await this.json(`${this.base}/uploads/${session.id}/complete`);
        if (node.digest !== digest) throw new Error(this.words.resume_mismatch);
      }
      if (!session.node) task.speed = (file.size - session.offset) / Math.max((performance.now() - transferStarted) / 1000, .01);
      this.show(task, 'complete', file.size); task.file = undefined;
      // A confirmed server completion must not become a failed upload if local cleanup fails.
      await this.persistQuietly(task);
      if (task.cached) {try {await this.cache.remove(task.key); task.cached = false} catch {}}
      this.uploads = this.uploads.filter((entry) => entry !== task);
    } catch (error) {
      if (error instanceof TransferOwnershipError) {
        if (this.cache.owns(task.key)) {try {await this.cache.remove(task.key)} catch {}}
        this.detach(task); return;
      }
      if (error instanceof Error && error.message === this.words.resume_mismatch) task.file = undefined;
      this.show(task, task.paused || task.cancelled ? 'paused' : 'failed', task.bytes, 0, task.paused || task.cancelled ? '' : error instanceof Error ? error.message : this.root.getAttribute('data-error')!);
    } finally {
      hash.close(); task.xhr = undefined; if (!task.cancelled) await this.persistQuietly(task);
      if (task.cancelled && !task.session) {if (task.cached) await this.cache.remove(task.key); task.row?.remove(); this.uploads = this.uploads.filter((item) => item !== task)}
      if (task.cancelled && task.session) {try {await this.json(`${this.base}/uploads/${task.session.id}`, undefined, 'DELETE'); if (task.cached) await this.cache.remove(task.key); task.row?.remove(); this.uploads = this.uploads.filter((item) => item !== task)} catch (error) {task.cancelled = false; this.show(task, 'failed', task.bytes, 0, String(error))}}
    }
  }
  private send(task: UploadTask, data: ArrayBuffer, offset: number, digest: string): Promise<UploadSession> {
    return new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest(); task.xhr = xhr;
      const started = performance.now(); let painted = 0;
      xhr.open('PATCH', `${this.base}/uploads/${task.session!.id}?${new URLSearchParams({offset: String(offset), size: String(data.byteLength)})}`);
      xhr.timeout = 120000; xhr.setRequestHeader('Content-Type', 'application/octet-stream'); xhr.setRequestHeader('X-Chunk-SHA256', digest);
      xhr.upload.addEventListener('progress', (event) => {if (performance.now() - painted > 150 || event.loaded === data.byteLength) {painted = performance.now(); this.show(task, 'uploading', offset + event.loaded, event.loaded / Math.max((painted - started) / 1000, .1))}});
      xhr.addEventListener('load', () => {let result; try {result = JSON.parse(xhr.responseText)} catch {} if (xhr.status === 200 && result?.id) resolve(result); else reject(new Error(result?.message || this.root.getAttribute('data-error')!));});
      xhr.addEventListener('error', () => reject(new Error(this.root.getAttribute('data-error')!))); xhr.addEventListener('timeout', () => reject(new Error(this.root.getAttribute('data-error')!))); xhr.addEventListener('abort', () => reject(new DOMException('Paused', 'AbortError'))); xhr.send(data);
    });
  }
  async download(ids: string[]) {
    await this.ready;
    const status = this.root.querySelector<HTMLElement>('[data-software-status]')!;
    try {if (!this.cacheAvailable) throw new Error(this.words.cache_required); await this.downloads.create(ids); status.textContent = this.words.download_started} catch (error) {status.textContent = error instanceof Error ? error.message : this.words.failed}
  }
  private async restoreRecords(records: CachedTransfer[], sessions?: UploadSession[]) {
    const duplicate = (record: CachedTransfer) => this.uploads.some((task) => task.key === record.key ||
      (record.data.session && task.session?.id === record.data.session.id) ||
      (task.folder === record.data.folder && task.name === record.data.name && task.size === record.data.size));
    for (const record of records.filter((entry) => entry.kind === 'upload' && !duplicate(entry))) {
      if (record.state === 'complete') {try {await this.cache.remove(record.key)} catch {} continue}
      const session = sessions?.find((entry) => entry.id === record.data.session?.id);
      const file = await this.cache.source(record.key);
      const paused = ['paused', 'failed'].includes(record.state) || !file;
      const task: UploadTask = {...record.data, key: record.key, file, session: session || record.data.session, paused, cancelled: false, running: false, cached: true, state: paused ? 'paused' : 'queued', bytes: session ? session.offset : record.offset};
      // Expired reservations are safely recreated with the same source fingerprint.
      if (sessions && !session) task.session = undefined;
      this.uploads.push(task); this.show(task, task.state, task.bytes);
    }
    await this.downloads.restore(records);
  }
  private async restore() {
    const status = this.root.querySelector<HTMLElement>('[data-software-status]')!;
    try {
      try {
        await this.cache.open();
        await this.cache.claim();
        this.cacheAvailable = true; await this.cache.prune();
      } catch {status.textContent = this.words.cache_unavailable}
      const records = this.cacheAvailable ? await this.cache.list() : [];
      const sessions: UploadSession[] = await this.json(`${this.base}/uploads`, undefined, 'GET');
      if (this.cacheAvailable) await this.restoreRecords(records, sessions);
      const all = this.cacheAvailable ? await this.cache.all() : [];
      for (const session of sessions.filter((entry) => this.uploads.every((task) => task.session?.id !== entry.id) && all.every((record) => record.kind !== 'upload' || !(record.data.session?.id === entry.id || record.data.folder === entry.parent && record.data.name === entry.name && record.data.size === entry.size)))) {
        const task: UploadTask = {key: session.id, name: session.name, folder: session.parent, size: session.size, session, paused: true, cancelled: false, running: false, state: 'paused', bytes: session.offset};
        this.uploads.push(task); this.show(task, 'paused', session.offset);
      }
      if (this.cacheAvailable) {
        let syncing = false;
        const heartbeat = setInterval(async () => {
          if (this.leaving) {clearInterval(heartbeat); return}
          if (syncing) return; syncing = true;
          try {
            await this.cache.claim();
            for (const task of this.uploads) if (task.cached && !this.cache.owns(task.key)) this.detach(task);
            this.downloads.detachUnowned();
            await this.restoreRecords(await this.cache.list()); this.pump();
          } catch {} finally {syncing = false}
        }, 3000);
      }
      this.recount(); this.pump();
    } catch (error) {status.textContent = String(error)}
  }
}
