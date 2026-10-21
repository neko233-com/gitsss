import {svg, type SvgName} from '../svg.ts';

export function softwareDate(seconds: number): string {
  const date = new Date(seconds * 1000);
  if (!Number.isFinite(date.getTime())) return '—';
  const pad = (value: number) => String(value).padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
}

export function softwareSizeBand(size: number): string {
  return size >= 10 * 2 ** 30 ? 'huge' : size >= 2 ** 30 ? 'large' : size >= 100 * 2 ** 20 ? 'medium' : size >= 10 * 2 ** 20 ? 'small' : 'tiny';
}

export function softwareFileType(name: string, directory = false): {icon: SvgName; kind: string; extension: string} {
  const extension = name.includes('.') ? name.split('.').pop()!.toLowerCase() : '';
  if (directory) return {icon: 'octicon-file-directory-fill', kind: 'folder', extension: ''};
  if (/^(exe|msi|dmg|pkg|app|apk|deb|rpm|appimage)$/.test(extension)) return {icon: 'octicon-device-desktop', kind: 'app', extension};
  if (/^(zip|7z|rar|tar|gz|tgz|bz2|xz|zst)$/.test(extension)) return {icon: 'octicon-file-zip', kind: 'archive', extension};
  if (/^(png|jpg|jpeg|gif|svg|webp|ico|avif)$/.test(extension)) return {icon: 'octicon-image', kind: 'image', extension};
  if (/^(json|js|ts|html|css|go|rs|cs|py|sh|ps1|yaml|yml|xml)$/.test(extension)) return {icon: 'octicon-file-code', kind: 'code', extension};
  return {icon: 'octicon-file', kind: 'file', extension};
}

export function softwareFileIcon(name: string, directory = false): HTMLElement {
  const type = softwareFileType(name, directory);
  const icon = document.createElement('span'); icon.className = 'software-file-kind'; icon.setAttribute('data-kind', type.kind); icon.setAttribute('aria-hidden', 'true'); icon.innerHTML = svg(type.icon, 20); return icon;
}

export function softwareAction(label: string, icon: SvgName, action: () => void): HTMLButtonElement {
  const button = document.createElement('button'); button.type = 'button'; button.className = 'ui basic compact button software-action'; button.innerHTML = svg(icon, 14);
  const text = document.createElement('span'); text.textContent = label; button.append(text); button.addEventListener('click', action); return button;
}

export function softwareMoveAllowed(ids: string[], target: string, folders: Array<{id: string; parent: string}>): boolean {
  const selected = new Set(ids); const parents = new Map(folders.map((folder) => [folder.id, folder.parent])); const visited = new Set<string>();
  for (let current = target; current; current = parents.get(current) || '') {
    if (selected.has(current) || visited.has(current) || !parents.has(current)) return false;
    visited.add(current);
  }
  return ids.length > 0 && ids.length <= 500;
}
