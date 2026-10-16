import {GET, POST} from '../modules/fetch.ts';
import {showErrorToast} from '../modules/toast.ts';
import {confirmModal} from './comp/ConfirmModal.ts';

export function filterBoardCards(root: HTMLElement, query: string, assignee: string): void {
  const terms = query.trim().toLocaleLowerCase().split(/\s+/u);
  for (const card of root.querySelectorAll<HTMLElement>('[data-board-task]')) {
    const text = (card.getAttribute('data-search') || '').toLocaleLowerCase();
    card.hidden = !(terms.every((term) => text.includes(term)) &&
      (!assignee || assignee === '0' || card.getAttribute('data-assignee') === assignee));
  }
  for (const column of root.querySelectorAll<HTMLElement>('[data-board-column]')) {
    const count = Array.from(column.querySelectorAll<HTMLElement>('[data-board-task]')).filter((card) => !card.hidden).length;
    column.querySelector<HTMLElement>('[data-board-count]')!.textContent = String(count);
    column.querySelector<HTMLElement>('[data-board-empty]')!.hidden = count > 0;
  }
}

export function initOrgKanban(): void {
  const root = document.querySelector<HTMLElement>('[data-org-kanban]');
  if (!root || root.hasAttribute('data-board-ready')) return;
  root.setAttribute('data-board-ready', 'true');
  const search = root.querySelector<HTMLInputElement>('[data-board-search]')!;
  const assignee = root.querySelector<HTMLSelectElement>('[data-board-assignee]')!;
  const filter = () => filterBoardCards(root, search.value, assignee.value);
  search.addEventListener('input', filter);
  assignee.addEventListener('change', filter);
  filter();

  const mentions = root.querySelector<HTMLDataListElement>('#board-mentions')!;
  const names = Array.from(mentions.options, (option) => option.getAttribute('data-name')!);
  for (const input of root.querySelectorAll<HTMLInputElement>('[data-board-title]')) {
    input.addEventListener('input', () => {
      const match = /(?:^|\s)@[^\s@]*$/u.exec(input.value);
      mentions.replaceChildren();
      if (!match) return;
      const prefix = input.value.slice(0, input.value.lastIndexOf('@'));
      for (const name of names) {
        const option = document.createElement('option');
        option.value = `${prefix}@${name}`;
        mentions.append(option);
      }
    });
  }

  const headers = {'X-Requested-With': 'XMLHttpRequest'};
  const boardURL = new URL(root.getAttribute('data-board-url')!, window.location.href);
  const pendingForms = new WeakSet<HTMLFormElement>();
  let searchVersion = 0;
  const loadSearch = async (url: URL, pushHistory: boolean) => {
    const version = ++searchVersion;
    const response = await GET(url.href);
    if (version !== searchVersion) return;
    if (!response.ok) throw new Error(root.getAttribute('data-error')!);
    const next = new DOMParser().parseFromString(await response.text(), 'text/html').querySelector('[data-org-kanban]');
    if (version !== searchVersion) return;
    if (!next) throw new Error(root.getAttribute('data-error')!);
    for (const column of root.querySelectorAll<HTMLElement>('[data-board-column]')) {
      const status = column.getAttribute('data-board-column')!;
      const cards = next.querySelector(`[data-board-column="${CSS.escape(status)}"] [data-board-cards]`)!;
      column.querySelector('[data-board-cards]')!.replaceChildren(...Array.from(cards.childNodes));
    }
    const limit = next.querySelector('[data-board-limit]')!;
    root.querySelector('[data-board-limit]')!.replaceChildren(...Array.from(limit.childNodes));
    assignee.value = next.querySelector<HTMLSelectElement>('[data-board-assignee]')!.value;
    if (pushHistory) window.history.pushState({}, '', url);
    filter();
  };
  root.addEventListener('submit', async (event) => {
    const form = event.target as HTMLFormElement;
    if (!form.matches('[data-board-form], [data-board-filter]')) return;
    event.preventDefault();
    if (pendingForms.has(form)) return;
    pendingForms.add(form);
    const data = new FormData(form);
    const controls = Array.from(form.querySelectorAll<HTMLInputElement | HTMLButtonElement | HTMLSelectElement | HTMLTextAreaElement>('input, button, select, textarea'));
    for (const control of controls) control.disabled = true;
    try {
      if (form.matches('[data-board-filter]')) {
        const url = new URL(boardURL);
        url.search = new URLSearchParams({q: search.value, assignee: assignee.value}).toString();
        await loadSearch(url, true);
      } else {
        const response = await POST(form.action, {data, headers});
        if (!response.ok) throw new Error(root.getAttribute(response.status === 400 ? 'data-invalid' : 'data-error')!);
        const card = new DOMParser().parseFromString(await response.text(), 'text/html').querySelector<HTMLElement>('[data-board-task]');
        if (!card) throw new Error(root.getAttribute('data-error')!);
        const status = card.getAttribute('data-status')!;
        const column = root.querySelector(`[data-board-column="${CSS.escape(status)}"] [data-board-cards]`);
        if (!column) throw new Error(root.getAttribute('data-error')!);
        const old = form.closest('[data-board-task]');
        if (old) old.remove();
        column.prepend(card);
        if (!old) form.reset();
        form.classList.remove('dirty');
        filter();
      }
    } catch (error) {
      showErrorToast(error instanceof Error ? error.message : root.getAttribute('data-error')!);
    } finally {
      for (const control of controls) control.disabled = false;
      pendingForms.delete(form);
      if (form.matches('.org-kanban-quick')) form.querySelector<HTMLInputElement>('[data-board-title]')!.focus();
      if (form.matches('[data-board-filter]')) search.focus();
    }
  });
  root.addEventListener('click', async (event) => {
    const target = event.target as HTMLElement;
    if (target.closest('[data-board-reset]')) {
      event.preventDefault();
      search.value = '';
      assignee.value = '0';
      try {
        await loadSearch(new URL(boardURL), true);
      } catch {
        showErrorToast(root.getAttribute('data-error')!);
      }
      return;
    }
    const button = target.closest<HTMLButtonElement>('[data-board-delete]');
    if (!button) return;
    event.preventDefault();
    if (!await confirmModal({content: button.getAttribute('data-confirm')!, confirmButtonColor: 'red'})) return;
    button.disabled = true;
    try {
      const response = await POST(button.form!.action, {data: new FormData(button.form!), headers});
      if (!response.ok) throw new Error(root.getAttribute('data-error')!);
      button.closest('[data-board-task]')!.remove();
      filter();
    } catch {
      showErrorToast(root.getAttribute('data-error')!);
      button.disabled = false;
    }
  });
  window.addEventListener('popstate', async () => {
    if (window.location.pathname !== boardURL.pathname) return;
    const url = new URL(window.location.href);
    search.value = url.searchParams.get('q') || '';
    assignee.value = url.searchParams.get('assignee') || '0';
    try {
      await loadSearch(url, false);
    } catch {
      showErrorToast(root.getAttribute('data-error')!);
    }
  });

  let dragged: HTMLElement | undefined;
  let saving = false;
  root.addEventListener('dragstart', (event) => {
    const target = event.target as HTMLElement;
    const card = target.closest<HTMLElement>('[data-board-task]');
    if (!card || saving || target.closest('form, input, textarea, select, details')) {
      event.preventDefault();
      return;
    }
    dragged = card;
    event.dataTransfer!.setData('text/plain', card.getAttribute('data-board-task')!);
    event.dataTransfer!.effectAllowed = 'move';
    card.classList.add('is-dragging');
  });
  root.addEventListener('dragend', () => {
    if (dragged) dragged.classList.remove('is-dragging');
    dragged = undefined;
    for (const column of root.querySelectorAll('[data-board-column]')) column.classList.remove('is-drop-target');
  });
  for (const column of root.querySelectorAll<HTMLElement>('[data-board-column]')) {
    column.addEventListener('dragover', (event) => {
      if (!dragged || saving) return;
      event.preventDefault();
      event.dataTransfer!.dropEffect = 'move';
      column.classList.add('is-drop-target');
    });
    column.addEventListener('dragleave', (event) => {
      if (!column.contains(event.relatedTarget as Node)) column.classList.remove('is-drop-target');
    });
    column.addEventListener('drop', async (event) => {
      event.preventDefault();
      column.classList.remove('is-drop-target');
      if (!dragged || saving || dragged.closest('[data-board-column]') === column) return;
      const card = dragged;
      const status = column.getAttribute('data-board-column')!;
      const data = new FormData();
      data.set('status', status);
      saving = true;
      try {
        const response = await POST(`${root.getAttribute('data-board-url')}/tasks/${card.getAttribute('data-board-task')}/move`, {data});
        if (!response.ok) throw new Error(root.getAttribute('data-error')!);
        column.querySelector('[data-board-cards]')!.prepend(card);
        card.querySelector<HTMLSelectElement>('select[name="status"]')!.value = status;
        filter();
      } catch {
        showErrorToast(root.getAttribute('data-error')!);
      } finally {
        saving = false;
        card.classList.remove('is-dragging');
      }
    });
  }
}
