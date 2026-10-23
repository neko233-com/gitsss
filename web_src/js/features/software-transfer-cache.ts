// IndexedDB also works on intranet HTTP, where file-system handles are unavailable.
export type CachedTransfer = {scope: string; key: string; kind: 'upload' | 'download'; state: string; updated: number; offset: number; data: any};
const DB = 'gitsss-software-transfers-v1';
const retention = 7 * 86400000;
const result = <T>(request: IDBRequest<T>): Promise<T> => new Promise((resolve, reject) => {request.onsuccess = () => resolve(request.result); request.addEventListener('error', () => reject(request.error!))});
const done = (tx: IDBTransaction): Promise<void> => new Promise((resolve, reject) => {tx.oncomplete = () => resolve(); tx.addEventListener('abort', () => reject(tx.error || new Error('Local transfer cache unavailable')))});

export class TransferOwnershipError extends Error {
  constructor() {super('Transfer is managed in another tab')}
}

export class TransferCache {
  private db!: IDBDatabase;
  private owner = `${Date.now()}-${Math.random()}`;
  private base: string;
  private scopes = new Set<string>();
  private keys = new Map<string, string>();
  private sessions = new Map<string, string>();
  scope: string;
  constructor(scope: string) {this.base = scope; this.scope = `${scope}\u0000tab:${this.owner}`}
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
  private async claimScope(scope: string): Promise<boolean> {
    const tx = this.db.transaction('leases', 'readwrite'); const finished = done(tx);
    const store = tx.objectStore('leases'); const lease = await result(store.get(scope));
    const owns = !lease || lease.owner === this.owner || lease.until < Date.now();
    if (owns) {store.put({scope, owner: this.owner, until: Date.now() + 15000}); this.scopes.add(scope)} else {this.scopes.delete(scope)}
    await finished; return owns;
  }
  async claim(): Promise<boolean> {
    for (const scope of this.scopes) if (!await this.claimScope(scope)) this.scopes.delete(scope);
    if (!await this.claimScope(this.scope)) {
      this.scope = `${this.base}\u0000tab:${this.owner}-${Date.now()}`;
      return this.claimScope(this.scope);
    }
    return true;
  }
  owns(key: string): boolean {return this.scopes.has(this.keys.get(key) || this.scope) && (!this.sessions.has(key) || this.scopes.has(this.sessions.get(key)!))}
  async bindUpload(key: string, id: string) {
    const scope = `${this.base}\u0000upload-session:${id}`;
    const previous = this.sessions.get(key);
    if (previous === scope) return;
    if (!await this.claimScope(scope)) throw new TransferOwnershipError();
    this.sessions.set(key, scope);
    if (previous) await this.releaseScope(previous);
  }
  async release() {
    const tx = this.db.transaction('leases', 'readwrite'); const finished = done(tx); const store = tx.objectStore('leases');
    for (const scope of this.scopes) {const lease = await result(store.get(scope)); if (lease?.owner === this.owner) store.delete(scope);}
    this.scopes.clear();
    await finished;
  }
  private async releaseScope(scope: string) {
    const tx = this.db.transaction('leases', 'readwrite'); const finished = done(tx); const store = tx.objectStore('leases');
    const lease = await result(store.get(scope)); if (lease?.owner === this.owner) store.delete(scope);
    await finished; this.scopes.delete(scope);
  }
  async check(key?: string) {
    if (!await this.claimScope(key === undefined ? this.scope : this.keys.get(key) || this.scope)) throw new TransferOwnershipError();
    if (key !== undefined && this.sessions.has(key) && !await this.claimScope(this.sessions.get(key)!)) throw new TransferOwnershipError();
  }
  private taskScope(key: string) {return this.keys.get(key) || this.scope}
  private range(key: string) {const scope = this.taskScope(key); return IDBKeyRange.bound([scope, key], [scope, key, []])}
  async all(): Promise<CachedTransfer[]> {
    const tx = this.db.transaction('tasks');
    return result(tx.objectStore('tasks').getAll(IDBKeyRange.bound([this.base], [`${this.base}\u0000tab:\uffff`, []])));
  }
  async list(): Promise<CachedTransfer[]> {
    const records = await this.all(); const available = new Set<string>();
    for (const scope of new Set(records.map((task) => task.scope))) if (await this.claimScope(scope)) available.add(scope);
    return records.filter((task) => {if (!available.has(task.scope)) return false; this.keys.set(task.key, task.scope); return true});
  }
  private async write(key: string, stores: string[], apply: (tx: IDBTransaction, scope: string) => void, signal?: AbortSignal) {
    await this.check(key); signal?.throwIfAborted();
    const scope = this.taskScope(key);
    const tx = this.db.transaction(['leases', ...stores], 'readwrite'); const finished = done(tx);
    const abort = () => {try {tx.abort()} catch {}}; signal?.addEventListener('abort', abort, {once: true});
    try {
      const leases = tx.objectStore('leases'); const lease = await result(leases.get(scope));
      if (!lease || lease.owner !== this.owner || lease.until < Date.now()) throw new TransferOwnershipError();
      leases.put({...lease, until: Date.now() + 15000});
      apply(tx, scope); await finished; this.keys.set(key, scope);
    } catch (error) {try {tx.abort()} catch {} try {await finished} catch {} throw error} finally {signal?.removeEventListener('abort', abort)}
  }
  async put(task: CachedTransfer, source?: File, signal?: AbortSignal) {
    await this.write(task.key, source ? ['tasks', 'sources'] : ['tasks'], (tx, scope) => {
      tx.objectStore('tasks').put({...task, scope, updated: Date.now()});
      if (source) tx.objectStore('sources').put({scope, key: task.key, file: source});
    }, signal);
  }
  async source(key: string): Promise<File | undefined> {
    const tx = this.db.transaction('sources'); return (await result(tx.objectStore('sources').get([this.taskScope(key), key])))?.file;
  }
  async append(task: CachedTransfer, offset: number, blob: Blob) {
    await this.write(task.key, ['tasks', 'chunks'], (tx, scope) => {
      tx.objectStore('chunks').put({scope, key: task.key, offset, blob});
      tx.objectStore('tasks').put({...task, scope, offset: offset + blob.size, updated: Date.now()});
    });
  }
  async blob(key: string, expected: number): Promise<Blob> {
    const tx = this.db.transaction('chunks'); const entries = await result(tx.objectStore('chunks').getAll(this.range(key)));
    let offset = 0;
    for (const entry of entries) {if (entry.offset !== offset) throw new Error('Incomplete local download'); offset += entry.blob.size}
    if (offset !== expected) throw new Error('Incomplete local download');
    return new Blob(entries.map((entry) => entry.blob));
  }
  async remove(key: string, keepTask = false) {
    await this.write(key, ['tasks', 'sources', 'chunks'], (tx, scope) => {
      if (!keepTask) tx.objectStore('tasks').delete([scope, key]);
      tx.objectStore('sources').delete([scope, key]); tx.objectStore('chunks').delete(this.range(key));
    });
    if (!keepTask) {
      const scope = this.taskScope(key); this.keys.delete(key);
      const session = this.sessions.get(key); this.sessions.delete(key);
      if (session) await this.releaseScope(session);
      if (scope !== this.scope && !Array.from(this.keys.values()).includes(scope)) await this.releaseScope(scope);
    }
  }
  async prune() {for (const task of await this.list()) if (task.updated < Date.now() - retention) await this.remove(task.key);}
}
