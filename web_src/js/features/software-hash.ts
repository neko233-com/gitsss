import {sha256} from '@noble/hashes/sha2.js';
import {bytesToHex} from '@noble/hashes/utils.js';

export const hashBytes = (bytes: Uint8Array): string => bytesToHex(sha256(bytes));
export const advanceUploadChain = (chain: string, digest: string, offset: number): string => hashBytes(new TextEncoder().encode(`${chain}:${digest}:${offset}`));
