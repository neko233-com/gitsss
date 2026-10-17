import {filesFromDrop, filesFromPicker, normalizeFolderPath} from './software-folders.ts';
import {advanceUploadChain, hashBytes} from './software-hash.ts';

test('hashes are deterministic on intranet HTTP without Web Crypto', () => {
  expect(hashBytes(new TextEncoder().encode('abc'))).toBe('ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad');
  expect(advanceUploadChain('', hashBytes(new Uint8Array()), 0)).toHaveLength(64);
  expect(advanceUploadChain('', 'a', 8)).not.toBe(advanceUploadChain('', 'a', 9));
});

test('folder picker preserves all parents and rejects traversal and depth overflow', () => {
  const file = new File(['hello'], 'tool.zip'); Object.defineProperty(file, 'webkitRelativePath', {value: 'SDK/Go/tool.zip'});
  expect(filesFromPicker([file]).directories).toEqual(['SDK', 'SDK/Go']);
  expect(filesFromPicker([file]).files[0].path).toBe('SDK/Go/tool.zip');
  for (const path of ['../escape', '/absolute', 'a//b', 'a\\b', Array.from({length: 33}, () => 'x').join('/')]) expect(() => normalizeFolderPath(path)).toThrow();
});

test('drag directory reader drains every batch and keeps empty directories', async () => {
  let reads = 0;
  const children = Array.from({length: 101}, (_, index) => ({isFile: true, isDirectory: false, name: `${index}.txt`, file: (done: (file: File) => void) => done(new File(['x'], `${index}.txt`))}));
  const empty = {isFile: false, isDirectory: true, name: 'empty', createReader: () => ({readEntries: (done: (entries: FileSystemEntry[]) => void) => done([])})};
  const root = {isFile: false, isDirectory: true, name: 'folder', createReader: () => ({readEntries: (done: (entries: unknown[]) => void) => {done(reads++ === 0 ? children.slice(0, 100) : reads === 2 ? [children[100], empty] : [])}})};
  const imported = await filesFromDrop([root as unknown as FileSystemEntry], []);
  expect(reads).toBe(3); expect(imported.files).toHaveLength(101); expect(imported.files[100].path).toBe('folder/100.txt'); expect(imported.directories).toEqual(['folder', 'folder/empty']);
});
