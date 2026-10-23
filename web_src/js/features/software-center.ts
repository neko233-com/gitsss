import {GET, POST} from '../modules/fetch.ts';
import {confirmModal} from './comp/ConfirmModal.ts';
import {svg} from '../svg.ts';
import {SoftwareTransfers, formatTransferBytes as bytes} from './software-transfer.ts';
import {filesFromDrop, filesFromPicker} from './software-folders.ts';
import type {FolderImport} from './software-folders.ts';
import {softwareAction, softwareDate, softwareFileIcon, softwareFileType, softwareMoveAllowed, softwareSizeBand} from './software-presentation.ts';
import {reinitializeAreYouSure} from '../vendor/jquery.are-you-sure.ts';

type Node = {id: string; parent: string; name: string; directory: boolean; size: number; digest?: string; modified: number; platform?: string};
type Listing = {entries: Node[]; folders: Node[]; breadcrumbs: Node[]; used: number; reserved: number; quota: number; total: number; page: number};
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
  const root = panel; const base = root.getAttribute('data-base')!;
  const words: Record<string, string> = JSON.parse(root.getAttribute('data-labels')!);
  const query = new URLSearchParams(window.location.search);
  let parent = query.get('parent') || ''; let page = Number(query.get('page')) || 1;
  let pending: AbortController | undefined; let snapshot: Listing | undefined;
  const selected = new Map<string, Node>(); const expanded = new Set<string>(['']);
  const status = root.querySelector<HTMLElement>('[data-software-status]')!;
  const search = root.querySelector<HTMLInputElement>('[data-software-search]')!;
  search.value = query.get('q') || '';
  const tree = root.querySelector<HTMLElement>('[data-software-tree]')!;
  const rows = root.querySelector<HTMLElement>('[data-software-rows]')!;
  const dialog = root.querySelector<HTMLDialogElement>('[data-software-dialog]')!;
  const nameForm = dialog.querySelector<HTMLFormElement>('form')!;
  const nameInput = nameForm.elements.namedItem('name') as HTMLInputElement;
  const errorText = dialog.querySelector<HTMLElement>('[data-software-dialog-error]')!;
  const selectAll = root.querySelector<HTMLInputElement>('[data-software-select-all]')!;
  const moveDialog = root.querySelector<HTMLDialogElement>('[data-software-move-dialog]')!;
  const moveSearch = root.querySelector<HTMLInputElement>('[data-software-move-search]')!;
  const moveError = root.querySelector<HTMLElement>('[data-software-move-error]')!;
  let moveIDs: string[] = []; let destination = ''; let renameID = '';
  let timer: ReturnType<typeof setTimeout> | undefined;
  const transfers = new SoftwareTransfers(root, base, words, () => {load()});
  const message = (error: unknown) => error instanceof Error ? error.message === 'folder_limit' ? words.transfer_limit : error.message === 'folder_invalid' ? words.invalid : error.message : root.getAttribute('data-error')!;
  async function post<T>(operation: string, params: URLSearchParams): Promise<T> {
    const response = await POST(`${base}/${operation}`, {data: params}); const result = await response.json();
    if (!response.ok) throw new Error(result.message || root.getAttribute('data-error')!);
    return result;
  }
  const idParams = (ids: string[]) => {const params = new URLSearchParams(); for (const id of ids) params.append('ids', id); return params};
  const navigate = (id: string) => {parent = id; page = 1; search.value = ''; load(true)};
  const button = (text: string, action: () => void) => {
    const el = document.createElement('button'); el.type = 'button'; el.textContent = text; el.addEventListener('click', action); return el;
  };
  const openName = (node?: Node) => {
    renameID = node ? node.id : ''; nameInput.value = node ? node.name : ''; errorText.textContent = ''; nameInput.removeAttribute('aria-invalid');
    reinitializeAreYouSure(nameForm);
    dialog.querySelector('[data-software-dialog-title]')!.textContent = root.getAttribute(node ? 'data-rename' : 'data-new-folder')!;
    dialog.showModal(); nameInput.focus(); nameInput.select();
  };
  const selectionChanged = () => {
    root.querySelector<HTMLElement>('[data-software-selection]')!.hidden = selected.size === 0;
    root.querySelector('[data-software-selected]')!.textContent = `${words.selected}: ${selected.size} / 500`;
    const visible = snapshot ? snapshot.entries.filter((node) => selected.has(node.id)).length : 0;
    selectAll.checked = Boolean(snapshot?.entries.length && visible === snapshot.entries.length); selectAll.indeterminate = visible > 0 && !selectAll.checked;
    for (const checkbox of rows.querySelectorAll<HTMLInputElement>('input[type="checkbox"]')) checkbox.checked = selected.has(checkbox.value);
  };
  const remove = async (nodes: Node[]) => {
    if (!nodes.length) return;
    if (!await confirmModal({header: root.getAttribute('data-remove')!, content: `${words.delete_many_confirm}\n${nodes.slice(0, 10).map((node) => node.name).join('\n')}${nodes.length > 10 ? '\n…' : ''}`, confirmButtonColor: 'red'})) return;
    try {await post('batch/delete', idParams(nodes.map((node) => node.id))); for (const node of nodes) selected.delete(node.id); await load()} catch (error) {status.textContent = message(error)}
  };
  const move = async (ids: string[], target: string) => {
    if (!softwareMoveAllowed(ids, target, snapshot!.folders)) throw new Error(words.move_invalid);
    const params = idParams(ids); params.set('parent', target); await post('batch/move', params);
    for (const id of ids) selected.delete(id); await load(); status.textContent = words.move_success;
  };
  const renderDestination = () => {
    const folders = snapshot!.folders; const ancestors: Node[] = [];
    let current = folders.find((item) => item.id === destination);
    while (current) {ancestors.unshift(current); current = folders.find((item) => item.id === current!.parent)}
    const path = root.querySelector('[data-software-move-path]')!;
    path.replaceChildren(button(root.querySelector('[data-software-root]')!.textContent.trim(), () => {destination = ''; moveSearch.value = ''; renderDestination()}));
    for (const folder of ancestors) path.append(button(folder.name, () => {destination = folder.id; moveSearch.value = ''; renderDestination()}));
    const fragment = document.createDocumentFragment(); const filter = moveSearch.value.toLocaleLowerCase();
    const candidates = folders.filter((folder) => (filter ? folder.name.toLocaleLowerCase().includes(filter) : folder.parent === destination) && softwareMoveAllowed(moveIDs, folder.id, folders));
    if (destination) fragment.append(button(`↑ ${words.up}`, () => {destination = folders.find((item) => item.id === destination)!.parent; moveSearch.value = ''; renderDestination()}));
    for (const folder of candidates.slice(0, 200)) {
      const item = button(folder.name, () => {destination = folder.id; moveSearch.value = ''; renderDestination()});
      item.prepend(document.createRange().createContextualFragment(svg('octicon-file-directory-fill', 16))); fragment.append(item);
    }
    if (!candidates.length) {const empty = document.createElement('p'); empty.textContent = root.getAttribute('data-empty')!; fragment.append(empty)}
    if (candidates.length > 200) {const hint = document.createElement('p'); hint.textContent = words.showing_limit; fragment.append(hint)}
    root.querySelector('[data-software-move-folders]')!.replaceChildren(fragment);
  };
  const openMove = (ids: string[]) => {if (!snapshot || !ids.length) return; moveIDs = ids; destination = parent; moveSearch.value = ''; moveError.textContent = ''; renderDestination(); moveDialog.showModal(); moveSearch.focus()};
  const dragType = 'application/x-gitsss-software';
  let draggedIDs: string[] = [];
  const movable = (element: HTMLElement, node: Node) => {
    element.draggable = true;
    element.addEventListener('dragstart', (event) => {
      if ((event.target as HTMLElement).closest('input,.software-row-actions')) {event.preventDefault(); return}
      draggedIDs = selected.has(node.id) ? Array.from(selected.keys()) : [node.id];
      event.dataTransfer!.setData(dragType, JSON.stringify({base, ids: draggedIDs})); event.dataTransfer!.effectAllowed = 'move'; element.classList.add('software-dragging'); status.textContent = words.drag_move;
    });
    element.addEventListener('dragend', () => {draggedIDs = []; element.classList.remove('software-dragging'); root.querySelectorAll('.software-drop-active').forEach((el) => {el.classList.remove('software-drop-active')}); if (status.textContent === words.drag_move) status.textContent = '';});
  };
  const dropTarget = (element: HTMLElement, target: () => string) => {
    element.addEventListener('dragover', (event) => {
      const internal = event.dataTransfer!.types.includes(dragType);
      if (!internal && !event.dataTransfer!.types.includes('Files')) return;
      event.preventDefault(); event.stopPropagation();
      const allowed = !internal || !draggedIDs.length || softwareMoveAllowed(draggedIDs, target(), snapshot!.folders);
      event.dataTransfer!.dropEffect = allowed ? internal ? 'move' : 'copy' : 'none'; element.classList.toggle('software-drop-active', allowed);
    });
    element.addEventListener('dragleave', (event) => {if (!(event.relatedTarget instanceof globalThis.Node) || !element.contains(event.relatedTarget)) element.classList.remove('software-drop-active');});
    element.addEventListener('drop', (event) => {event.preventDefault(); event.stopPropagation(); element.classList.remove('software-drop-active'); handleDrop(event.dataTransfer!, target())});
  };
  const renderTree = () => {
    const children = folderChildren(snapshot!.folders); const fragment = document.createDocumentFragment();
    let visible = 0;
    const append = (id: string, depth: number) => {
      for (const folder of children.get(id) || []) {
        if (visible++ >= 500) return;
        const row = document.createElement('div'); row.className = 'software-tree-row'; row.style.setProperty('--depth', String(depth));
        const toggle = button('', () => {if (expanded.has(folder.id)) expanded.delete(folder.id); else expanded.add(folder.id); renderTree()});
        toggle.setAttribute('aria-expanded', String(expanded.has(folder.id))); toggle.setAttribute('aria-label', folder.name); toggle.disabled = !children.has(folder.id);
        toggle.innerHTML = svg(expanded.has(folder.id) ? 'octicon-chevron-down' : 'octicon-chevron-right', 14);
        const item = button(folder.name, () => navigate(folder.id)); item.title = folder.name;
        item.prepend(document.createRange().createContextualFragment(svg('octicon-file-directory-fill', 16)));
        if (folder.id === parent) item.setAttribute('aria-current', 'page');
        movable(item, folder); dropTarget(row, () => folder.id); row.append(toggle, item); fragment.append(row);
        if (expanded.has(folder.id)) append(folder.id, depth + 1);
      }
    };
    append('', 0);
    if (visible > 500) {const hint = document.createElement('p'); hint.textContent = words.tree_limit; fragment.append(hint)}
    tree.replaceChildren(fragment);
  };
  const render = () => {
    const data = snapshot!; page = data.page;
    const quotaInput = root.querySelector<HTMLInputElement>('[data-software-quota] input');
    if (quotaInput && !quotaInput.form!.classList.contains('dirty')) quotaInput.value = String(data.quota / 2 ** 30);
    for (const folder of data.breadcrumbs) expanded.add(folder.id);
    renderTree();
    root.querySelector('[data-software-usage]')!.textContent = `${bytes(data.used)} / ${bytes(data.quota)}${data.reserved ? ` (+${bytes(data.reserved)})` : ''}`;
    const breadcrumbs = root.querySelector('[data-software-breadcrumbs]')!;
    const home = button(root.querySelector('[data-software-root]')!.textContent.trim(), () => navigate('')); dropTarget(home, () => ''); breadcrumbs.replaceChildren(home);
    for (const folder of data.breadcrumbs.toReversed()) {
      const separator = document.createElement('span'); separator.textContent = '/'; const item = button(folder.name, () => navigate(folder.id)); dropTarget(item, () => folder.id); breadcrumbs.append(separator, item);
    }
    const fragment = document.createDocumentFragment();
    for (const node of data.entries) {
      if (selected.has(node.id)) selected.set(node.id, node);
      const row = document.createElement('tr'); row.setAttribute('data-software-node', node.id); movable(row, node); if (node.directory) {row.setAttribute('data-drop-folder', node.id); dropTarget(row, () => node.id)}
      const cell = document.createElement('td'); cell.className = 'software-select-cell';
      const checkbox = document.createElement('input'); checkbox.type = 'checkbox'; checkbox.value = node.id; checkbox.setAttribute('aria-label', `${words.select}: ${node.name}`);
      checkbox.addEventListener('change', () => {if (checkbox.checked) {if (selected.size >= 500) {checkbox.checked = false; status.textContent = words.transfer_limit; return} selected.set(node.id, node)} else selected.delete(node.id); selectionChanged()}); cell.append(checkbox);
      const name = document.createElement('td'); name.className = 'software-file-name';
      const content = document.createElement('div'); content.className = 'software-name-content';
      const handle = document.createElement('span'); handle.className = 'software-drag-handle'; handle.title = words.drag_move; handle.innerHTML = svg('octicon-grabber', 14); handle.setAttribute('aria-hidden', 'true');
      const kind = softwareFileIcon(node.name, node.directory); kind.title = node.directory ? root.getAttribute('data-folder')! : softwareFileType(node.name).extension.toUpperCase() || root.getAttribute('data-file')!;
      let link: HTMLElement;
      if (node.directory) {link = button(node.name, () => navigate(node.id))} else {
        const anchor = document.createElement('a'); anchor.href = `${base}/download/${node.id}`; anchor.textContent = node.name; anchor.setAttribute('download', node.name);
        anchor.addEventListener('click', (event) => {if (event.button === 0 && !event.ctrlKey && !event.metaKey && !event.shiftKey && !event.altKey) {event.preventDefault(); transfers.download([node.id])}}); link = anchor;
      }
      link.title = node.name; content.append(handle, kind, link); name.append(content);
      const platform = document.createElement('td');
      if (node.directory) {platform.textContent = '—'} else {
        const label = document.createElement('span'); label.className = 'ui small basic label';
        label.textContent = node.platform === 'all' ? words.platform_all : !node.platform || node.platform === 'unknown' ? words.platform_unknown : node.platform;
        platform.append(label);
      }
      const size = document.createElement('td'); const badge = document.createElement('span'); badge.className = 'software-size'; badge.setAttribute('data-band', node.directory ? 'folder' : softwareSizeBand(node.size)); badge.textContent = node.directory ? '—' : bytes(node.size); size.append(badge);
      const date = document.createElement('td'); const time = document.createElement('time'); time.textContent = softwareDate(node.modified); if (Number.isFinite(new Date(node.modified * 1000).getTime())) time.dateTime = new Date(node.modified * 1000).toISOString(); date.append(time);
      const actions = document.createElement('td'); actions.className = 'software-row-actions';
      const group = document.createElement('div'); group.className = 'software-actions';
      const download = softwareAction(words.download, 'octicon-download', () => {transfers.download([node.id])});
      const moveAction = softwareAction(words.move, 'octicon-arrow-switch', () => openMove([node.id]));
      const rename = softwareAction(root.getAttribute('data-rename')!, 'octicon-diff-renamed', () => openName(node));
      const deleteAction = softwareAction(root.getAttribute('data-remove')!, 'octicon-trash', () => {remove([node])}); deleteAction.classList.add('software-action-danger');
      for (const action of [download, moveAction, rename, deleteAction]) {action.title = `${action.textContent}: ${node.name}`; action.setAttribute('aria-label', action.title)}
      group.append(download, moveAction, rename, deleteAction); actions.append(group);
      row.append(cell, name, platform, size, date, actions); fragment.append(row);
    }
    rows.replaceChildren(fragment); selectionChanged();
    root.querySelector<HTMLElement>('[data-software-empty]')!.hidden = data.entries.length > 0;
    root.querySelector<HTMLButtonElement>('[data-software-prev]')!.disabled = page <= 1;
    root.querySelector<HTMLButtonElement>('[data-software-next]')!.disabled = page * 200 >= data.total;
    root.querySelector('[data-software-page]')!.textContent = `${page} / ${Math.max(1, Math.ceil(data.total / 200))} · ${data.total}`;
  };
  async function load(history = false) {
    pending?.abort(); const controller = new AbortController(); pending = controller; rows.setAttribute('aria-busy', 'true');
    const params = new URLSearchParams({parent, q: search.value, page: String(page)});
    try {
      const response = await GET(`${base}/list?${params}`, {signal: controller.signal}); const result = await response.json();
      if (!response.ok) throw new Error(result.message || root.getAttribute('data-error')!);
      if (controller.signal.aborted) return;
      snapshot = result; render();
      if (history) window.history.pushState({}, '', `?${params}`);
    } catch (error) {if (!controller.signal.aborted) status.textContent = message(error);} finally {if (pending === controller) rows.removeAttribute('aria-busy');}
  }
  root.querySelector('[data-software-root]')!.addEventListener('click', () => navigate(''));
  dropTarget(root.querySelector('[data-software-root]')!, () => ''); dropTarget(root, () => parent);
  root.querySelector('[data-software-mkdir]')!.addEventListener('click', () => openName());
  root.querySelector('[data-software-dialog-cancel]')!.addEventListener('click', () => dialog.close());
  dialog.addEventListener('close', () => {nameForm.reset(); reinitializeAreYouSure(nameForm)});
  root.querySelector('[data-software-prev]')!.addEventListener('click', () => {page--; load(true)});
  root.querySelector('[data-software-next]')!.addEventListener('click', () => {page++; load(true)});
  search.addEventListener('input', () => {clearTimeout(timer); pending?.abort(); timer = setTimeout(() => {page = 1; load(true)}, 200)});
  nameInput.addEventListener('input', () => {nameInput.removeAttribute('aria-invalid'); errorText.textContent = ''});
  nameForm.addEventListener('submit', async (event) => {
    event.preventDefault(); const submit = nameForm.querySelector<HTMLButtonElement>('[type="submit"]')!; submit.disabled = true;
    try {await post(renameID ? 'rename' : 'mkdir', new URLSearchParams({id: renameID, parent, name: nameInput.value.normalize('NFC')})); dialog.close(); await load()} catch (error) {errorText.textContent = message(error); nameInput.setAttribute('aria-invalid', 'true'); nameInput.focus()} finally {submit.disabled = false}
  });
  selectAll.addEventListener('change', () => {for (const node of snapshot!.entries) {if (selectAll.checked) {if (selected.size < 500) selected.set(node.id, node); else status.textContent = words.transfer_limit;} else selected.delete(node.id);} selectionChanged()});
  root.querySelector('[data-software-clear]')!.addEventListener('click', () => {selected.clear(); selectionChanged()});
  root.querySelector('[data-software-batch-download]')!.addEventListener('click', () => {transfers.download(Array.from(selected.keys()))});
  root.querySelector('[data-software-batch-delete]')!.addEventListener('click', () => {remove(Array.from(selected.values()))});
  root.querySelector('[data-software-batch-move]')!.addEventListener('click', () => openMove(Array.from(selected.keys())));
  moveSearch.addEventListener('input', renderDestination);
  root.querySelector('[data-software-move-cancel]')!.addEventListener('click', () => moveDialog.close());
  const moveConfirm = root.querySelector<HTMLButtonElement>('[data-software-move-confirm]')!;
  moveConfirm.addEventListener('click', async () => {moveConfirm.disabled = true; moveError.textContent = ''; try {await move(moveIDs, destination); moveDialog.close()} catch (error) {moveError.textContent = message(error)} finally {moveConfirm.disabled = false}});
  const quotaForm = root.querySelector<HTMLFormElement>('[data-software-quota]');
  quotaForm?.addEventListener('input', () => quotaForm.classList.add('dirty'));
  quotaForm?.addEventListener('submit', async (event) => {
    event.preventDefault(); const control = quotaForm.querySelector<HTMLButtonElement>('button')!; control.disabled = true;
    try {await post('quota', new URLSearchParams({gib: (quotaForm.elements.namedItem('gib') as HTMLInputElement).value})); quotaForm.classList.remove('dirty'); await load()} catch (error) {status.textContent = message(error)} finally {control.disabled = false}
  });
  async function importFiles(imported: FolderImport, target: string) {
    status.textContent = words.preparing;
    const paths = imported.directories.sort((a, b) => a.split('/').length - b.split('/').length || a.localeCompare(b));
    const folders: Record<string, string> = {'': target};
    for (let i = 0; i < paths.length; i += 500) {
      const params = new URLSearchParams({parent: target}); for (const path of paths.slice(i, i + 500)) params.append('paths', path);
      Object.assign(folders, await post<Record<string, string>>('batch/directories', params));
    }
    await transfers.enqueue(imported.files.map(({file, path}) => ({file, folder: folders[path.includes('/') ? path.slice(0, path.lastIndexOf('/')) : '']})));
    status.textContent = ''; await load();
  }
  async function handleDrop(data: DataTransfer, target: string) {
    const internal = data.getData(dragType);
    // Snapshot drag data before the browser invalidates its protected store at the first await.
    const entries = Array.from(data.items, (item) => item.webkitGetAsEntry()).filter((entry): entry is FileSystemEntry => entry !== null);
    const files = Array.from(data.files);
    try {if (internal) {const payload = JSON.parse(internal); if (payload.base !== base || !Array.isArray(payload.ids) || payload.ids.some((id: unknown) => typeof id !== 'string')) return; await move(payload.ids, target)} else if (entries.length || files.length) await importFiles(await filesFromDrop(entries, files), target);} catch (error) {status.textContent = message(error)}
  }
  const uploadInput = root.querySelector<HTMLInputElement>('[data-software-upload]')!;
  const folderInput = root.querySelector<HTMLInputElement>('[data-software-folder-upload]')!;
  for (const input of [uploadInput, folderInput]) input.addEventListener('change', async () => {
    const target = parent; const files = Array.from(input.files!); input.value = '';
    try {await importFiles(filesFromPicker(files), target)} catch (error) {status.textContent = message(error)}
  });
  root.querySelector('[data-software-empty-upload]')!.addEventListener('click', () => uploadInput.click());
  window.addEventListener('popstate', () => {const params = new URLSearchParams(window.location.search); parent = params.get('parent') || ''; page = Number(params.get('page')) || 1; search.value = params.get('q') || ''; load()});
  load();
}
