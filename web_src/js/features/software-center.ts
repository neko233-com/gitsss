import {GET, POST} from '../modules/fetch.ts';
import {confirmModal} from './comp/ConfirmModal.ts';
import {svg} from '../svg.ts';

type Node = {id: string; parent: string; name: string; directory: boolean; size: number; digest?: string; modified: number};
type Listing = {entries: Node[]; folders: Node[]; breadcrumbs: Node[]; used: number; reserved: number; quota: number; total: number; page: number};
const bytes = (size: number) => {
  const unit = size >= 2 ** 30 ? 30 : size >= 2 ** 20 ? 20 : size >= 2 ** 10 ? 10 : 0;
  return `${(size / 2 ** unit).toLocaleString(undefined, {maximumFractionDigits: 1})} ${({0: 'B', 10: 'KiB', 20: 'MiB', 30: 'GiB'})[unit]}`;
};
export function folderChildren(folders: Node[]): Map<string, Node[]> {
  const children = new Map<string, Node[]>();
  for (const folder of folders) {
    if (!children.has(folder.parent)) children.set(folder.parent, []);
    children.get(folder.parent)!.push(folder);
  }
  return children;
}

export function initSoftwareCenter(): void {
  const panel = document.querySelector<HTMLElement>('[data-software-center]');
  if (!panel) return;
  const root = panel;
  const base = root.getAttribute('data-base')!;
  const query = new URLSearchParams(window.location.search);
  let parent = query.get('parent') || '';
  let page = Number(query.get('page')) || 1;
  let pending: AbortController | undefined;
  let snapshot: Listing | undefined;
  const expanded = new Set<string>(['']);
  const status = root.querySelector<HTMLElement>('[data-software-status]')!;
  const search = root.querySelector<HTMLInputElement>('[data-software-search]')!;
  search.value = query.get('q') || '';
  const tree = root.querySelector<HTMLElement>('[data-software-tree]')!;
  const rows = root.querySelector<HTMLElement>('[data-software-rows]')!;
  const dialog = root.querySelector<HTMLDialogElement>('dialog')!;
  const nameForm = dialog.querySelector<HTMLFormElement>('form')!;
  const nameInput = nameForm.elements.namedItem('name') as HTMLInputElement;
  const errorText = dialog.querySelector<HTMLElement>('[data-software-dialog-error]')!;
  let renameID = '';
  let timer: ReturnType<typeof setTimeout> | undefined;
  const navigate = (id: string) => {parent = id; page = 1; search.value = ''; load(true)};
  const button = (text: string, action: () => void) => {
    const el = document.createElement('button'); el.type = 'button'; el.textContent = text; el.addEventListener('click', action); return el;
  };
  const openName = (node?: Node) => {
    renameID = node ? node.id : '';
    nameInput.value = node ? node.name : ''; errorText.textContent = '';
    dialog.querySelector('[data-software-dialog-title]')!.textContent = root.getAttribute(node ? 'data-rename' : 'data-new-folder')!;
    dialog.showModal(); nameInput.focus(); nameInput.select();
  };
  const remove = async (node: Node) => {
    if (!await confirmModal({header: root.getAttribute('data-remove')!, content: `${root.getAttribute('data-delete')}\n${node.name}`, confirmButtonColor: 'red'})) return;
    try {
      const response = await POST(`${base}/delete`, {data: new URLSearchParams({id: node.id})});
      if (!response.ok) throw new Error((await response.json()).message || root.getAttribute('data-error')!);
      await load();
    } catch (error) {status.textContent = error instanceof Error ? error.message : root.getAttribute('data-error')!}
  };
  const renderTree = () => {
    const children = folderChildren(snapshot!.folders);
    const fragment = document.createDocumentFragment();
    const append = (id: string, depth: number) => {
      for (const folder of children.get(id) || []) {
        const row = document.createElement('div'); row.className = 'software-tree-row'; row.style.setProperty('--depth', String(depth));
        const toggle = button(expanded.has(folder.id) ? '−' : '+', () => {if (expanded.has(folder.id)) expanded.delete(folder.id); else expanded.add(folder.id); renderTree()});
        toggle.setAttribute('aria-expanded', String(expanded.has(folder.id))); toggle.setAttribute('aria-label', folder.name);
        toggle.disabled = !children.has(folder.id);
        toggle.innerHTML = svg(expanded.has(folder.id) ? 'octicon-chevron-down' : 'octicon-chevron-right', 14);
        const item = button(folder.name, () => navigate(folder.id)); item.title = folder.name;
        item.prepend(document.createRange().createContextualFragment(svg('octicon-file-directory-fill', 16)));
        if (folder.id === parent) item.setAttribute('aria-current', 'page');
        row.append(toggle, item); fragment.append(row);
        if (expanded.has(folder.id)) append(folder.id, depth + 1);
      }
    };
    append('', 0); tree.replaceChildren(fragment);
  };
  const render = () => {
    const data = snapshot!;
    page = data.page;
    const quotaInput = root.querySelector<HTMLInputElement>('[data-software-quota] input');
    if (quotaInput && !quotaInput.form!.classList.contains('dirty')) quotaInput.value = String(data.quota / 2 ** 30);
    for (const folder of data.breadcrumbs) expanded.add(folder.id);
    renderTree();
    root.querySelector('[data-software-usage]')!.textContent = `${bytes(data.used)} / ${bytes(data.quota)}${data.reserved ? ` (+${bytes(data.reserved)})` : ''}`;
    const breadcrumbs = root.querySelector('[data-software-breadcrumbs]')!;
    breadcrumbs.replaceChildren(button(root.querySelector('[data-software-root]')!.textContent.trim(), () => navigate('')));
    for (const folder of data.breadcrumbs.toReversed()) {
      const separator = document.createElement('span'); separator.textContent = '/'; breadcrumbs.append(separator, button(folder.name, () => navigate(folder.id)));
    }
    const fragment = document.createDocumentFragment();
    for (const node of data.entries) {
      const row = document.createElement('tr');
      const name = document.createElement('td');
      const kind = document.createElement('span'); kind.className = 'software-file-kind'; kind.textContent = root.dataset[node.directory ? 'folder' : 'file']!;
      kind.innerHTML = svg(node.directory ? 'octicon-file-directory-fill' : 'octicon-file', 16);
      kind.setAttribute('aria-label', root.dataset[node.directory ? 'folder' : 'file']!);
      let link: HTMLElement;
      if (node.directory) {link = button(node.name, () => navigate(node.id))} else {
        const anchor = document.createElement('a'); anchor.href = `${base}/download/${node.id}`; anchor.textContent = node.name; anchor.setAttribute('download', node.name); link = anchor;
      }
      name.append(kind, link);
      const size = document.createElement('td'); size.textContent = node.directory ? '—' : bytes(node.size);
      const date = document.createElement('td'); date.textContent = new Date(node.modified * 1000).toLocaleString();
      const actions = document.createElement('td'); actions.className = 'software-row-actions';
      actions.append(button(root.getAttribute('data-rename')!, () => openName(node)), button(root.getAttribute('data-remove')!, () => {remove(node)}));
      row.append(name, size, date, actions); fragment.append(row);
    }
    rows.replaceChildren(fragment);
    root.querySelector<HTMLElement>('[data-software-empty]')!.hidden = data.entries.length > 0;
    root.querySelector<HTMLButtonElement>('[data-software-prev]')!.disabled = page <= 1;
    root.querySelector<HTMLButtonElement>('[data-software-next]')!.disabled = page * 200 >= data.total;
    root.querySelector('[data-software-page]')!.textContent = `${page} / ${Math.max(1, Math.ceil(data.total / 200))} · ${data.total}`;
  };
  async function load(history = false) {
    if (pending) pending.abort();
    const controller = new AbortController(); pending = controller; rows.setAttribute('aria-busy', 'true');
    const params = new URLSearchParams({parent, q: search.value, page: String(page)});
    try {
      const response = await GET(`${base}/list?${params}`, {signal: controller.signal});
      if (!response.ok) throw new Error(root.getAttribute('data-error')!);
      const result = await response.json();
      if (controller.signal.aborted) return;
      snapshot = result; status.textContent = ''; render();
      if (history) window.history.pushState({}, '', `?${params}`);
    } catch (error) {if (!controller.signal.aborted) status.textContent = error instanceof Error ? error.message : root.getAttribute('data-error')!;} finally {if (pending === controller) rows.removeAttribute('aria-busy');}
  }
  root.querySelector('[data-software-root]')!.addEventListener('click', () => navigate(''));
  root.querySelector('[data-software-mkdir]')!.addEventListener('click', () => openName());
  root.querySelector('[data-software-dialog-cancel]')!.addEventListener('click', () => dialog.close());
  root.querySelector('[data-software-prev]')!.addEventListener('click', () => {page--; load(true)});
  root.querySelector('[data-software-next]')!.addEventListener('click', () => {page++; load(true)});
  search.addEventListener('input', () => {clearTimeout(timer); if (pending) pending.abort(); timer = setTimeout(() => {page = 1; load(true)}, 200)});
  nameForm.addEventListener('submit', async (event) => {
    event.preventDefault();
    const submit = nameForm.querySelector<HTMLButtonElement>('[type="submit"]')!; submit.disabled = true;
    try {
      const response = await POST(`${base}/${renameID ? 'rename' : 'mkdir'}`, {data: new URLSearchParams({id: renameID, parent, name: nameInput.value.normalize('NFC')})});
      if (!response.ok) throw new Error((await response.json()).message || root.getAttribute('data-error')!);
      dialog.close(); await load();
    } catch (error) {errorText.textContent = error instanceof Error ? error.message : root.getAttribute('data-error')!} finally {submit.disabled = false}
  });
  const quotaForm = root.querySelector<HTMLFormElement>('[data-software-quota]');
  quotaForm?.addEventListener('input', () => quotaForm.classList.add('dirty'));
  if (quotaForm) quotaForm.addEventListener('submit', async (event) => {
    event.preventDefault();
    const control = quotaForm.querySelector<HTMLButtonElement>('button')!; control.disabled = true;
    try {
      const gib = (quotaForm.elements.namedItem('gib') as HTMLInputElement).value;
      const response = await POST(`${base}/quota`, {data: new URLSearchParams({gib})});
      if (!response.ok) throw new Error((await response.json()).message || root.getAttribute('data-error')!);
      quotaForm.classList.remove('dirty'); await load();
    } catch (error) {status.textContent = error instanceof Error ? error.message : root.getAttribute('data-error')!} finally {control.disabled = false}
  });
  const uploadInput = root.querySelector<HTMLInputElement>('[data-software-upload]')!;
  const queue: Array<{file: File; folder: string}> = [];
  let active = 0;
  const runQueue = () => {
    while (active < 2 && queue.length) {
      const {file, folder} = queue.shift()!; active++;
      const task = document.createElement('div'); task.className = 'software-upload-task';
      const text = document.createElement('span'); text.textContent = file.name;
      const progress = document.createElement('progress'); progress.max = file.size || 1;
      const detail = document.createElement('span');
      const request = new XMLHttpRequest();
      const cancel = button(root.getAttribute('data-cancel')!, () => request.abort());
      task.append(text, progress, detail, cancel); root.querySelector('[data-software-uploads]')!.append(task);
      const started = performance.now();
      request.open('POST', `${base}/upload?${new URLSearchParams({parent: folder, name: file.name.normalize('NFC'), size: String(file.size)})}`);
      request.setRequestHeader('Content-Type', 'application/octet-stream');
      request.upload.addEventListener('progress', (event) => {progress.value = event.loaded; detail.textContent = `${bytes(event.loaded)} / ${bytes(file.size)} · ${bytes(event.loaded / Math.max((performance.now() - started) / 1000, .1))}/s`});
      request.addEventListener('load', () => {
        if (request.status === 201) {progress.value = progress.max; detail.textContent = '✓'} else {try {detail.textContent = JSON.parse(request.responseText).message || root.getAttribute('data-error')!} catch {detail.textContent = root.getAttribute('data-error')!}}
      });
      request.addEventListener('error', () => {detail.textContent = root.getAttribute('data-error')!});
      request.addEventListener('abort', () => {detail.textContent = root.getAttribute('data-cancel')!});
      request.addEventListener('loadend', () => {active--; cancel.remove(); load(); runQueue()});
      request.send(file);
    }
  };
  uploadInput.addEventListener('change', () => {
    for (const file of Array.from(uploadInput.files!)) queue.push({file, folder: parent});
    uploadInput.value = ''; runQueue();
  });
  window.addEventListener('popstate', () => {const params = new URLSearchParams(window.location.search); parent = params.get('parent') || ''; page = Number(params.get('page')) || 1; search.value = params.get('q') || ''; load()});
  load();
}
