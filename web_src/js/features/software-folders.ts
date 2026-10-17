export type FolderFile = {file: File; path: string};
export type FolderImport = {files: FolderFile[]; directories: string[]};
const maxEntries = 20000;
export function normalizeFolderPath(path: string): string {
  const parts = path.normalize('NFC').split('/');
  if (parts.length > 32 || parts.some((part) => !part || part === '.' || part === '..' || /[\\\x00-\x1f]/.test(part) || new TextEncoder().encode(part).length > 255)) throw new Error('folder_invalid');
  return parts.join('/');
}
export function filesFromPicker(files: File[]): FolderImport {
  const directories = new Set<string>();
  const entries = files.map((file) => {
    const path = normalizeFolderPath(file.webkitRelativePath || file.name);
    const parts = path.split('/');
    for (let i = 1; i < parts.length; i++) directories.add(parts.slice(0, i).join('/'));
    return {file, path};
  });
  if (entries.length + directories.size > maxEntries) throw new Error('folder_limit');
  return {files: entries, directories: Array.from(directories)};
}
export async function filesFromDrop(entries: FileSystemEntry[], fallback: File[]): Promise<FolderImport> {
  if (!entries.length) return filesFromPicker(fallback);
  const files: FolderFile[] = []; const directories: string[] = [];
  async function walk(entry: FileSystemEntry, prefix: string) {
    const path = normalizeFolderPath(`${prefix}${entry.name}`);
    if (files.length + directories.length >= maxEntries) throw new Error('folder_limit');
    if (entry.isFile) {
      const file = await new Promise<File>((resolve, reject) => (entry as FileSystemFileEntry).file(resolve, reject));
      files.push({file, path});
    } else {
      directories.push(path);
      const reader = (entry as FileSystemDirectoryEntry).createReader();
      for (;;) {
        const batch = await new Promise<FileSystemEntry[]>((resolve, reject) => reader.readEntries(resolve, reject));
        if (!batch.length) break;
        for (const child of batch) await walk(child, `${path}/`);
      }
    }
  }
  for (const entry of entries) await walk(entry, '');
  return {files, directories};
}
