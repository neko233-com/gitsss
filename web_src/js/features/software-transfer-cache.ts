// IndexedDB also works on intranet HTTP, where file-system handles are unavailable.
export type CachedTransfer = {scope: string; key: string; kind: 'upload' | 'download'; state: string; updated: number; offset: number; data: any};
const DB = 'gitsss-software-transfers-v1';
const retention = 7 * 86400000;
const result = <T>(request: IDBRequest<T>): Promise<T> => new Promise((resolve, reject) => {request.onsuccess = () => resolve(request.result); request.addEventListener('error', () => reject(request.error!))});
const done = (tx: IDBTransaction): Promise<void> => new Promise((resolve, reject) => {tx.oncomplete = () => resolve(); tx.addEventListener('abort', () => reject(tx.error || new Error('Local transfer cache unavailable')))});

export class TransferCache {
  private db!: IDBDatabase;
  private owner = `${Date.now()}-${Math.random()}`;
  readonly scope: string;
  constructor(scope: string) {this.scope = scope}
  async open() {
    const request = indexedDB.open(DB, 1);
    request.onupgradeneeded = () => {
      const db = request.result;
      db.createObjectStore('tasks', {keyPath: ['scope', 'key']});
      db.createObjectStore('sources', {keyPath: ['scope', 'key']});
      db.createObjectStore('chunks', {keyPath: ['scope', 'key', 'offset']});
      db.createObjectStore('leases', {keyPath: 'scope'});
    };
    this.db = await result(request);
    this.db.onversionchange = () => this.db.close();
  }
  async claim(): Promise<boolean> {
    const tx = this.db.transaction('leases', 'readwrite'); const finished = done(tx);
    const store = tx.objectStore('leases'); const lease = await result(store.get(this.scope));
    const owns = !lease || lease.owner === this.owner || lease.until < Date.now();
    if (owns) store.put({scope: this.scope, owner: this.owner, until: Date.now() + 15000});
    await finished; return owns;
  }
  async release() {
    const tx = this.db.transaction('leases', 'readwrite'); const finished = done(tx); const store = tx.objectStore('leases');
    const lease = await result(store.get(this.scope)); if (lease?.owner === this.owner) store.delete(this.scope);
    await finished;
  }
  async check() {
    if (!await this.claim()) throw new Error('Transfer is managed in another tab');
  }
  private range(key?: string) {return key === undefined ? IDBKeyRange.bound([this.scope], [this.scope, []]) : IDBKeyRange.bound([this.scope, key], [this.scope, key, []])}
  async list(): Promise<CachedTransfer[]> {
    const tx = this.db.transaction('tasks'); return result(tx.objectStore('tasks').getAll(this.range()));
  }
  async put(task: CachedTransfer, source?: File, signal?: AbortSignal) {
    await this.check();
    signal?.throwIfAborted();
    const tx = this.db.transaction(source ? ['tasks', 'sources'] : 'tasks', 'readwrite'); const finished = done(tx);
    const abort = () => {try {tx.abort()} catch {}};
    signal?.addEventListener('abort', abort, {once: true});
    tx.objectStore('tasks').put({...task, updated: Date.now()});
    if (source) tx.objectStore('sources').put({scope: this.scope, key: task.key, file: source});
    try {await finished} finally {signal?.removeEventListener('abort', abort)}
  }
  async source(key: string): Promise<File | undefined> {
    const tx = this.db.transaction('sources'); return (await result(tx.objectStore('sources').get([this.scope, key])))?.file;
  }
  async append(task: CachedTransfer, offset: number, blob: Blob) {
    await this.check();
    const tx = this.db.transaction(['tasks', 'chunks'], 'readwrite'); const finished = done(tx);
    tx.objectStore('chunks').put({scope: this.scope, key: task.key, offset, blob});
    tx.objectStore('tasks').put({...task, offset: offset + blob.size, updated: Date.now()});
    await finished;
  }
  async blob(key: string, expected: number): Promise<Blob> {
    const tx = this.db.transaction('chunks'); const entries = await result(tx.objectStore('chunks').getAll(this.range(key)));
    let offset = 0;
    for (const entry of entries) {if (entry.offset !== offset) throw new Error('Incomplete local download'); offset += entry.blob.size}
    if (offset !== expected) throw new Error('Incomplete local download');
    return new Blob(entries.map((entry) => entry.blob));
  }
  async remove(key: string, keepTask = false) {
    await this.check();
    const tx = this.db.transaction(['tasks', 'sources', 'chunks'], 'readwrite'); const finished = done(tx);
    if (!keepTask) tx.objectStore('tasks').delete([this.scope, key]);
    tx.objectStore('sources').delete([this.scope, key]); tx.objectStore('chunks').delete(this.range(key));
    await finished;
  }
  async prune() {for (const task of await this.list()) if (task.updated < Date.now() - retention) await this.remove(task.key);}
}
