import {GET, POST, DELETE} from '../modules/fetch.ts';
import type {TransferCache, CachedTransfer} from './software-transfer-cache.ts';
import {softwareAction, softwareFileIcon} from './software-presentation.ts';
import type {SvgName} from '../svg.ts';

export type DownloadJob = {id: string; name: string; size: number; etag: string; ids: string[]; expires: number; save_offered?: boolean};
type Task = {key: string; job: DownloadJob; state: string; offset: number; running: boolean; paused: boolean; cancelled: boolean; row: HTMLElement; abort?: AbortController; error?: string; speed: number; offered: boolean};
const chunkSize = 8 * 1024 * 1024;
export class SoftwareDownloads {
  private tasks: Task[] = [];
  private active = 0;
  private leaving = false;
  private urls = new Set<string>();
  private root: HTMLElement;
  private base: string;
  private words: Record<string, string>;
  private cache: TransferCache;
  private format: (n: number) => string;
  private changed: () => void;
  constructor(root: HTMLElement, base: string, words: Record<string, string>, cache: TransferCache, format: (n: number) => string, changed: () => void) {
    this.root = root; this.base = base; this.words = words; this.cache = cache; this.format = format; this.changed = changed;
    window.addEventListener('pagehide', () => {for (const url of this.urls) URL.revokeObjectURL(url);});
  }
  get busy() {return this.tasks.filter((task) => task.running || task.state === 'queued').length}
  get pending() {return this.tasks.filter((task) => task.state !== 'saved' && !task.cancelled).length}
  get ready() {return this.tasks.filter((task) => task.state === 'ready').length}
  get failed() {return this.tasks.filter((task) => task.state === 'failed').length}
  get pausedCount() {return this.tasks.filter((task) => task.state === 'paused').length}
  get paused() {return this.tasks.some((task) => task.state === 'paused' || task.state === 'failed')}
  suspend() {this.leaving = true; for (const task of this.tasks) {task.paused = true; task.abort?.abort()}}
  private record(task: Task): CachedTransfer {return {scope: this.cache.scope, key: task.key, kind: 'download', state: task.state, offset: task.offset, updated: Date.now(), data: {...task.job, save_offered: task.offered}}}
  private async persist(task: Task) {if (!this.leaving) await this.cache.put(this.record(task));}
  private add(key: string, job: DownloadJob, state = 'queued', offset = 0) {
    const row = document.createElement('article'); row.className = 'software-transfer-row'; row.setAttribute('data-download-task', key);
    const name = document.createElement('strong'); const text = document.createElement('span'); text.textContent = job.name; name.append(softwareFileIcon(job.name), text); name.title = job.name;
    const progress = document.createElement('progress'); progress.max = job.size || 1; progress.setAttribute('aria-label', job.name);
    const detail = document.createElement('span'); detail.className = 'software-transfer-detail';
    const actions = document.createElement('div'); actions.className = 'software-transfer-controls'; row.append(name, progress, detail, actions);
    this.root.querySelector('[data-transfer-downloads]')!.prepend(row);
    const task: Task = {key, job, state, offset, running: false, paused: state === 'paused' || state === 'failed', cancelled: false, row, speed: 0, offered: Boolean(job.save_offered)};
    this.tasks.push(task); this.paint(task); return task;
  }
  async restore(records: CachedTransfer[]) {
    for (const record of records.filter((entry) => entry.kind === 'download')) {
      if (record.state === 'saved') {try {await this.cache.remove(record.key)} catch {} continue}
      this.add(record.key, record.data, ['paused', 'failed', 'ready'].includes(record.state) ? record.state : 'queued', record.offset);
    }
    this.pump();
  }
  async create(ids: string[]) {
    await this.cache.check();
    const params = new URLSearchParams(); for (const id of ids) params.append('ids', id);
    const response = await POST(`${this.base}/downloads`, {data: params});
    if (!response.ok) throw new Error(this.words.failed);
    const job: DownloadJob = await response.json(); const task = this.add(`download-${job.id}`, job);
    try {await this.persist(task)} catch (error) {await DELETE(`${this.base}/downloads/${job.id}`); task.row.remove(); this.tasks = this.tasks.filter((item) => item !== task); throw error}
    this.pump();
  }
  pauseAll() {for (const task of this.tasks.filter((entry) => entry.running || entry.state === 'queued')) this.pause(task);}
  resumeAll() {for (const task of this.tasks.filter((entry) => entry.paused || entry.state === 'failed')) {task.paused = false; task.state = 'queued'; this.persistQuietly(task); this.paint(task)} this.pump()}
  private pause(task: Task) {task.paused = true; task.abort?.abort(); task.state = 'paused'; this.persistQuietly(task); this.paint(task)}
  private paint(task: Task, bytes = task.offset) {
    const row = task.row; row.setAttribute('data-state', task.state); row.querySelector('progress')!.value = task.job.size === 0 && task.state === 'ready' ? 1 : bytes;
    row.querySelector('.software-transfer-detail')!.setAttribute('aria-live', task.state === 'failed' ? 'polite' : 'off');
    row.querySelector('.software-transfer-detail')!.textContent = `${this.words[task.state] || task.state} · ${this.format(bytes)} / ${this.format(task.job.size)}${task.speed ? ` · ${this.format(task.speed)}/s` : ''}${task.error ? ` · ${task.error}` : ''}`;
    const actions = row.querySelector<HTMLElement>('.software-transfer-controls')!;
    const focus = actions.contains(document.activeElement);
    const mode = `${task.state}:${task.running}:${task.offered}`; if (actions.getAttribute('data-mode') === mode) {this.changed(); return} actions.setAttribute('data-mode', mode); actions.replaceChildren();
    const button = (label: string, fn: () => void, icon: SvgName = 'octicon-x') => actions.append(softwareAction(label, icon, fn));
    if (task.state === 'ready') {button(this.words.save_file, () => {this.save(task)}, 'octicon-download'); if (task.offered) button(this.words.confirm_saved, () => {this.finishSave(task)}, 'octicon-check');} else if (task.running && !task.paused || task.state === 'queued') button(this.words.pause, () => this.pause(task), 'octicon-pause');
    else if (!task.running) button(task.state === 'failed' ? this.words.retry : this.words.resume, () => {task.paused = false; task.error = ''; task.state = 'queued'; this.resumeTask(task)}, task.state === 'failed' ? 'octicon-history' : 'octicon-play');
    if (!task.cancelled) button(task.state === 'saved' ? this.words.remove_record : this.words.cancel_transfer, () => {this.cancel(task)});
    if (focus) actions.querySelector<HTMLButtonElement>('button')?.focus();
    this.changed();
  }
  private fail(task: Task, error: unknown) {task.state = 'failed'; task.error = error instanceof Error ? error.message : this.words.failed; this.paint(task)}
  private pump() {
    while (!this.leaving && this.active < 2) {
      const task = this.tasks.find((entry) => entry.state === 'queued' && !entry.running && !entry.cancelled && !entry.paused); if (!task) break;
      task.running = true; this.active++; this.runQueued(task);
    }
    this.changed();
  }
  private async runQueued(task: Task) {try {await this.run(task)} finally {task.running = false; this.active--; this.paint(task); this.pump()}}
  private async persistQuietly(task: Task) {try {await this.persist(task)} catch {}}
  private async resumeTask(task: Task) {try {await this.persist(task); this.pump()} catch (error) {this.fail(task, error)}}
  private ensure(task: Task) {if (task.paused || task.cancelled) throw new DOMException('Paused', 'AbortError');}
  private async renew(task: Task) {
    if (task.job.expires * 1000 > Date.now()) return;
    const params = new URLSearchParams(); for (const id of task.job.ids) params.append('ids', id);
    const response = await POST(`${this.base}/downloads`, {data: params}); if (!response.ok) throw new Error(this.words.download_changed);
    const job: DownloadJob = await response.json();
    if (job.etag !== task.job.etag || job.size !== task.job.size) {await DELETE(`${this.base}/downloads/${job.id}`); throw new Error(this.words.download_changed)}
    await DELETE(`${this.base}/downloads/${task.job.id}`); task.job = job; await this.persist(task);
  }
  private async run(task: Task) {
    const started = performance.now(); const initialOffset = task.offset;
    try {
      await this.cache.check(); this.ensure(task); await this.renew(task); task.state = 'downloading'; await this.persist(task); this.paint(task);
      while (task.offset < task.job.size) {
        this.ensure(task); await this.cache.check(); const start = task.offset; const end = Math.min(start + chunkSize, task.job.size) - 1;
        let succeeded = false;
        for (let attempt = 0; attempt < 4 && !succeeded; attempt++) {
          this.ensure(task); task.abort = new AbortController();
          const timeout = setTimeout(() => task.abort!.abort(), 120000);
          try {
            const response = await GET(`${this.base}/downloads/${task.job.id}/file`, {signal: task.abort.signal, headers: {Range: `bytes=${start}-${end}`, 'If-Range': task.job.etag, 'X-GitSSS-Transfer': 'managed'}, cache: 'no-store', redirect: 'error'});
            if (response.status === 401 || response.status === 403) throw new Error(this.words.session_expired);
            if (response.status === 409 || response.status === 404) throw new Error(this.words.download_changed);
            if (response.status === 429 || response.status >= 500) throw new Error(this.words.failed);
            if (response.status !== 206 || response.headers.get('ETag') !== task.job.etag || response.headers.get('Content-Range') !== `bytes ${start}-${end}/${task.job.size}`) throw new Error(this.words.download_changed);
            const reader = response.body!.getReader(); const parts: Uint8Array<ArrayBuffer>[] = []; let length = 0; let painted = 0; const began = performance.now();
            for (;;) {const {value, done} = await reader.read(); this.ensure(task); if (done) break; length += value.byteLength; if (length > end - start + 1) {await reader.cancel(); throw new Error(this.words.download_changed)} parts.push(value); if (performance.now() - painted > 150) {painted = performance.now(); task.speed = length / Math.max((painted - began) / 1000, .01); this.paint(task, start + length)}}
            if (length !== end - start + 1) throw new Error(this.words.failed);
            this.ensure(task); await this.cache.append(this.record(task), start, new Blob(parts)); task.offset = end + 1; this.paint(task); succeeded = true;
          } catch (error) {
            this.ensure(task);
            if (error instanceof DOMException && error.name === 'QuotaExceededError') throw new Error(this.words.cache_full);
            if (attempt === 3 || error instanceof Error && [this.words.download_changed, this.words.session_expired].includes(error.message)) throw error;
            await new Promise((resolve) => setTimeout(resolve, 500 * 2 ** attempt));
          } finally {clearTimeout(timeout); task.abort?.abort(); task.abort = undefined}
        }
      }
      this.ensure(task); task.speed = (task.offset - initialOffset) / Math.max((performance.now() - started) / 1000, .01); task.state = 'ready'; await this.persist(task); this.paint(task);
    } catch (error) {if (!task.cancelled) {if (task.paused) task.state = 'paused'; else this.fail(task, error); await this.persistQuietly(task); this.paint(task)}} finally {if (task.cancelled) await this.remove(task);}
  }
  private async save(task: Task) {
    try {
      await this.cache.check();
      const blob = await this.cache.blob(task.key, task.job.size); const url = URL.createObjectURL(blob); this.urls.add(url);
      const link = document.createElement('a'); link.href = url; link.download = task.job.name; document.body.append(link); link.click(); link.remove();
      // Keep the bytes until the user confirms the browser's final local save.
      task.offered = true; await this.persist(task); this.paint(task);
      setTimeout(() => {URL.revokeObjectURL(url); this.urls.delete(url)}, 60000);
    } catch (error) {task.error = error instanceof Error ? error.message : this.words.failed; this.paint(task)}
  }
  private async finishSave(task: Task) {
    try {
      const response = await DELETE(`${this.base}/downloads/${task.job.id}`); if (!response.ok && response.status !== 404) throw new Error(this.words.failed);
      task.job.expires = 0; task.state = 'saved'; await this.persistQuietly(task);
      try {await this.cache.remove(task.key)} catch {}
      if (task.row.contains(document.activeElement)) this.root.querySelector<HTMLInputElement>('[data-software-search]')?.focus();
      task.row.remove(); this.tasks = this.tasks.filter((entry) => entry !== task); this.changed();
    } catch (error) {this.fail(task, error)}
  }
  private async cancel(task: Task) {task.cancelled = true; task.paused = true; task.abort?.abort(); if (!task.running) await this.remove(task);}
  private async remove(task: Task) {
    try {await this.cache.check(); const response = await DELETE(`${this.base}/downloads/${task.job.id}`); if (!response.ok && response.status !== 404) throw new Error(this.words.failed); await this.cache.remove(task.key); task.row.remove(); this.tasks = this.tasks.filter((entry) => entry !== task); this.changed()} catch (error) {task.cancelled = false; this.fail(task, error)}
  }
  async clearCompleted() {for (const task of this.tasks.filter((entry) => entry.state === 'saved')) await this.remove(task);}
}
