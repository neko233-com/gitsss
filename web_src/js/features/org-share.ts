// Copyright 2026 The GitSSS Authors. All rights reserved.
// SPDX-License-Identifier: MIT

import {GET, PATCH, POST} from '../modules/fetch.ts';
import {showErrorToast} from '../modules/toast.ts';

type Chunk = {start: number, end: number};
type UploadSession = {id: string, size: number};
type ShareFile = {file: File, path: string};

export function planChunks(size: number, chunk: number): Chunk[] {
  if (size <= 0 || chunk <= 0) return [];
  const spans: Chunk[] = [];
  for (let start = 0; start < size; start += chunk) {
    spans.push({start, end: Math.min(start + chunk, size)});
  }
  return spans;
}

export function contentRangeHeader(start: number, end: number, total: number): string {
  return `bytes ${start}-${end - 1}/${total}`;
}

function relativePath(file: File): string {
  const named = file as File & {webkitRelativePath?: string};
  const relative = named.webkitRelativePath || '';
  if (relative !== '') return relative.replaceAll('\\', '/');
  return file.name;
}

async function entryFiles(entry: FileSystemEntry, prefix: string): Promise<ShareFile[]> {
  if (entry.isFile) {
    const fileEntry = entry as FileSystemFileEntry;
    const file = await new Promise<File>((resolve, reject) => {
      fileEntry.file(resolve, reject);
    });
    return [{file, path: `${prefix}${file.name}`}];
  }
  const directory = entry as FileSystemDirectoryEntry;
  const reader = directory.createReader();
  const children: FileSystemEntry[] = [];
  for (;;) {
    const batch = await new Promise<FileSystemEntry[]>((resolve, reject) => {
      reader.readEntries(resolve, reject);
    });
    if (batch.length === 0) break;
    children.push(...batch);
  }
  const nested: ShareFile[] = [];
  for (const child of children) {
    const found = await entryFiles(child, `${prefix}${entry.name}/`);
    nested.push(...found);
  }
  return nested;
}

async function filesFromTransfer(transfer: DataTransfer): Promise<ShareFile[]> {
  const items = Array.from(transfer.items);
  const collected: ShareFile[] = [];
  let usedEntries = false;
  for (const item of items) {
    const asEntry = item as DataTransferItem & {webkitGetAsEntry?: () => FileSystemEntry | null};
    const entry = asEntry.webkitGetAsEntry ? asEntry.webkitGetAsEntry() : null;
    if (!entry) continue;
    usedEntries = true;
    const found = await entryFiles(entry, '');
    collected.push(...found);
  }
  if (usedEntries) return collected;
  return Array.from(transfer.files, (file) => ({file, path: relativePath(file)}));
}

async function uploadOne(api: string, parent: number, permanent: boolean, chunkBytes: number, parallel: number, item: ShareFile, progress: HTMLElement): Promise<void> {
  const begin = await POST(`${api}/uploads`, {
    data: {name: item.path, size: item.file.size, parent_id: parent, permanent, resume: true},
  });
  if (!begin.ok) throw new Error(await begin.text());
  const session = await begin.json() as UploadSession;
  const spans = planChunks(item.file.size, chunkBytes);
  let cursor = 0;
  const worker = async () => {
    for (;;) {
      const index = cursor;
      cursor += 1;
      const span = spans[index];
      if (!span) return;
      progress.textContent = `${item.path} ${index + 1}/${spans.length}`;
      const bytes = new Uint8Array(await item.file.slice(span.start, span.end).arrayBuffer());
      let response: Response | null = null;
      for (let attempt = 0; attempt < 3; attempt += 1) {
        response = await PATCH(`${api}/uploads/${session.id}`, {
          data: bytes,
          headers: {
            'content-range': contentRangeHeader(span.start, span.end, item.file.size),
            'content-type': 'application/octet-stream',
          },
        });
        if (response.ok) break;
      }
      if (!response || !response.ok) throw new Error(response ? await response.text() : 'upload failed');
    }
  };
  await Promise.all(Array.from({length: Math.max(1, parallel)}, () => worker()));
  const done = await POST(`${api}/uploads/${session.id}/complete`);
  if (!done.ok) throw new Error(await done.text());
}

async function saveChunks(url: string, size: number, name: string, chunkBytes: number, parallel: number): Promise<void> {
  const spans = planChunks(size, chunkBytes);
  const picker = (window as Window & {showSaveFilePicker?: (options: {suggestedName: string}) => Promise<FileSystemFileHandle>}).showSaveFilePicker;
  if (picker) {
    const handle = await picker({suggestedName: name});
    const writable = await handle.createWritable();
    let cursor = 0;
    const worker = async () => {
      for (;;) {
        const index = cursor;
        cursor += 1;
        const span = spans[index];
        if (!span) return;
        const response = await GET(url, {headers: {range: `bytes=${span.start}-${span.end - 1}`}});
        if (!response.ok) throw new Error(await response.text());
        await writable.write({type: 'write', position: span.start, data: await response.arrayBuffer()});
      }
    };
    await Promise.all(Array.from({length: Math.max(1, parallel)}, () => worker()));
    await writable.close();
    return;
  }
  const parts = new Array<Blob>(spans.length);
  let cursor = 0;
  const worker = async () => {
    for (;;) {
      const index = cursor;
      cursor += 1;
      const span = spans[index];
      if (!span) return;
      const response = await GET(url, {headers: {range: `bytes=${span.start}-${span.end - 1}`}});
      if (!response.ok) throw new Error(await response.text());
      parts[index] = new Blob([await response.arrayBuffer()]);
    }
  };
  await Promise.all(Array.from({length: Math.max(1, parallel)}, () => worker()));
  const blob = new Blob(parts);
  const anchor = document.createElement('a');
  anchor.href = URL.createObjectURL(blob);
  anchor.download = name;
  anchor.click();
  URL.revokeObjectURL(anchor.href);
}

export function initOrgShare(): void {
  const root = document.querySelector<HTMLElement>('[data-org-share]');
  if (!root) return;
  const api = root.getAttribute('data-api')!;
  const parent = Number(root.getAttribute('data-parent') || '0');
  const chunkBytes = Number(root.getAttribute('data-chunk-bytes') || '8388608');
  const parallel = Number(root.getAttribute('data-parallel') || '4');
  const progress = root.querySelector<HTMLElement>('[data-org-share-progress]')!;
  const permanentBox = root.querySelector<HTMLInputElement>('[data-org-share-permanent]')!;
  const drop = root.querySelector<HTMLElement>('[data-org-share-drop]')!;

  const send = async (items: ShareFile[]) => {
    try {
      for (const item of items) {
        await uploadOne(api, parent, permanentBox.checked, chunkBytes, parallel, item, progress);
      }
      window.location.reload();
    } catch (error) {
      progress.textContent = '';
      showErrorToast(error instanceof Error ? error.message : String(error));
    }
  };

  drop.addEventListener('dragover', (event) => {
    event.preventDefault();
  });
  drop.addEventListener('drop', async (event) => {
    event.preventDefault();
    const transfer = event.dataTransfer;
    if (!transfer) return;
    try {
      await send(await filesFromTransfer(transfer));
    } catch (error) {
      showErrorToast(error instanceof Error ? error.message : String(error));
    }
  });

  const bindInput = (selector: string) => {
    const input = root.querySelector<HTMLInputElement>(selector)!;
    input.addEventListener('change', async () => {
      const selected = input.files ? Array.from(input.files) : [];
      const files = selected.map((file) => ({file, path: relativePath(file)}));
      try {
        await send(files);
      } catch (error) {
        showErrorToast(error instanceof Error ? error.message : String(error));
      }
    });
  };
  bindInput('[data-org-share-files]');
  bindInput('[data-org-share-dir]');

  for (const button of root.querySelectorAll<HTMLButtonElement>('[data-org-share-download]')) {
    button.addEventListener('click', async () => {
      const url = button.getAttribute('data-url')!;
      const name = button.getAttribute('data-name')!;
      const size = Number(button.getAttribute('data-size') || '0');
      try {
        await saveChunks(url, size, name, chunkBytes, parallel);
      } catch (error) {
        showErrorToast(error instanceof Error ? error.message : String(error));
      }
    });
  }
}
