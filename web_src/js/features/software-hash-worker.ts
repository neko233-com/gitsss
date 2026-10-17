import {sha256} from '@noble/hashes/sha2.js';
import {bytesToHex} from '@noble/hashes/utils.js';
import {hashBytes, advanceUploadChain} from './software-hash.ts';

export type HashRequest = {id: number; op: 'hash' | 'chain' | 'finish'; data?: ArrayBuffer; accumulate?: boolean; chain?: string; digest?: string; offset?: number};
export type HashReply = {id: number; value: string; data?: ArrayBuffer; error?: string};

const scope = self as unknown as DedicatedWorkerGlobalScope;
const whole = sha256.create();
scope.addEventListener('message', (event: MessageEvent<HashRequest>) => {
  const request = event.data;
  try {
    let value: string;
    if (request.op === 'finish') value = bytesToHex(whole.digest());
    else if (request.op === 'chain') value = advanceUploadChain(request.chain!, request.digest!, request.offset!);
    else {
      const bytes = new Uint8Array(request.data!);
      if (request.accumulate) whole.update(bytes);
      value = hashBytes(bytes);
    }
    const reply: HashReply = {id: request.id, value, data: request.data};
    scope.postMessage(reply, request.data ? [request.data] : []);
  } catch (error) {scope.postMessage({id: request.id, error: error instanceof Error ? error.message : 'Hash failed'})}
});
