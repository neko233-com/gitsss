import Combobox from '@github/combobox-nav';
import {GET} from '../modules/fetch.ts';

type Result = {title: string; description: string; url: string};

export function workspaceResultURL(value: string): string {
  try {
    const url = new URL(value, window.location.href);
    if (!['http:', 'https:'].includes(url.protocol) || url.hostname !== window.location.hostname || url.username || url.password) return '';
    return `${url.pathname}${url.search}${url.hash}`;
  } catch {return ''}
}

export function initWorkspaceSearch(): void {
  const dialog = document.querySelector<HTMLDialogElement>('.workspace-search-dialog');
  const trigger = document.querySelector<HTMLInputElement>('[data-workspace-search]');
  if (!dialog || !trigger) return;
  document.body.append(dialog);
  trigger.setAttribute('aria-keyshortcuts', 'Control+k Meta+k');
  const input = dialog.querySelector<HTMLInputElement>('#workspace-search-input')!;
  const list = dialog.querySelector<HTMLElement>('[role="listbox"]')!;
  const status = dialog.querySelector<HTMLElement>('[role="status"]')!;
  const {appSubUrl} = window.config;
  const combo = new Combobox(input, list, {tabInsertsSuggestions: false, firstOptionSelectionMode: 'selected', scrollIntoViewOptions: {block: 'nearest'}});
  let controller: AbortController | undefined;
  let timer: ReturnType<typeof setTimeout>;
  let generation = 0;
  const normalize = (value: string) => value.normalize('NFKC').toLocaleLowerCase();
  function render(results: Result[]): void {
    combo.stop(); list.replaceChildren();
    for (const [index, result] of results.entries()) {
      const url = workspaceResultURL(result.url);
      if (!url) continue;
      const row = document.createElement('li'); row.id = `workspace-result-${index}`; row.role = 'option'; row.setAttribute('data-url', url);
      const title = document.createElement('strong'); title.textContent = result.title;
      const description = document.createElement('span'); description.textContent = result.description;
      row.append(title, description); list.append(row);
    }
    if (list.children.length) combo.start();
    status.textContent = list.children.length ? `${list.children.length} ${(dialog!).getAttribute('data-results')}` : (dialog!).getAttribute('data-empty')!;
  }
  async function search(): Promise<void> {
    controller?.abort(); const current = ++generation; const signal = (controller = new AbortController()).signal;
    const query = input.value.normalize('NFKC').trim(); const encoded = encodeURIComponent(query);
    const links = Array.from(document.querySelectorAll<HTMLAnchorElement>('.workspace-nav a, .workspace-sidebar [data-workspace-org]'), (link) => ({title: link.textContent.trim(), description: (dialog!).getAttribute('data-pages')!, url: link.href}));
    const staticResults = links.filter((row) => !query || normalize(row.title).includes(normalize(query))).slice(0, 8);
    render(staticResults);
    dialog!.querySelector<HTMLAnchorElement>('[data-search-all]')!.href = `${appSubUrl}/explore/repos?q=${encoded}`;
    dialog!.querySelector<HTMLAnchorElement>('[data-search-code]')!.href = `${appSubUrl}/explore/code?q=${encoded}`;
    if (!query) return;
    status.textContent = (dialog!).getAttribute('data-loading')!;
    let failed = false;
    const results = Array.from(staticResults);
    try {
      const response = await GET(`${appSubUrl}/workspace/search?q=${encoded}`, {signal, cache: 'no-store'});
      if (!response.ok) throw new Error('Search unavailable');
      const rows = await response.json();
      if (current !== generation || !dialog!.open || signal.aborted) return;
      if (!Array.isArray(rows)) throw new Error('Invalid search response');
      for (const row of rows) {
        if (typeof row.title !== 'string' || typeof row.url !== 'string' || !['repos', 'issues', 'orgs'].includes(row.kind)) continue;
        results.push({title: row.title, description: dialog!.getAttribute(`data-${row.kind}`)!, url: row.url});
      }
    } catch {
      if (current !== generation || signal.aborted || !dialog!.open) return;
      failed = true;
    }
    const seen = new Set<string>();
    render(results.filter((row) => {const url = workspaceResultURL(row.url); if (!url || seen.has(url)) return false; seen.add(url); return true}).slice(0, 30));
    if (failed) status.textContent = (dialog!).getAttribute('data-error')!;
  }
  function open(): void {
    if (document.querySelector('dialog[open]') && !dialog!.open) return;
    if (!dialog!.open) dialog!.showModal();
    input.focus(); input.select(); search();
  }
  trigger.addEventListener('click', open);
  document.addEventListener('keydown', (event) => {
    if ((event.ctrlKey || event.metaKey) && !event.altKey && !event.shiftKey && !event.isComposing && event.key.toLowerCase() === 'k') {
      event.preventDefault(); open();
    }
  }, {capture: true});
  input.addEventListener('input', () => {controller?.abort(); generation++; clearTimeout(timer); timer = setTimeout(() => search(), 200)});
  list.addEventListener('combobox-commit', (event) => {
    const url = ((event.target as HTMLElement).closest<HTMLElement>('[data-url]')!).getAttribute('data-url')!;
    window.location.assign(url);
  });
  dialog.querySelector('[data-search-close]')!.addEventListener('click', () => dialog.close());
  dialog.addEventListener('keydown', (event) => {if (event.key === 'Escape') {event.preventDefault(); dialog.close()}}, {capture: true});
  dialog.addEventListener('close', () => {controller?.abort(); generation++; clearTimeout(timer); combo.stop()});
}
