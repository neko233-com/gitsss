import {POST} from '../modules/fetch.ts';
import {showErrorToast} from '../modules/toast.ts';
import {svg, type SvgName} from '../svg.ts';

export function enhanceHomeLinkCards(root: HTMLElement): void {
  for (const link of root.querySelectorAll<HTMLAnchorElement>('.home-link')) {
    const holder = link.querySelector<HTMLElement>('.home-link-icon'); if (!holder || holder.getAttribute('data-ready')) continue; holder.setAttribute('data-ready', 'true');
    let url: URL; try {url = new URL(link.href)} catch {continue}
    const label = `${link.querySelector('strong')!.textContent} ${url.hostname} ${url.port}`.toLowerCase();
    const kind = /数据库|查询|\bdb\b|database|8075/.test(label) ? 'database' : /构建|打包|ci\/cd|jenkins|buildworld|8080/.test(label) ? 'build' : /文档|docs|wiki/.test(label) ? 'docs' : 'web';
    const icons: Record<string, SvgName> = {database: 'octicon-database', build: 'octicon-gear', docs: 'octicon-file-code', web: 'octicon-link'};
    link.setAttribute('data-kind', kind); const fallback = holder.querySelector<HTMLElement>('[data-home-link-fallback]')!; fallback.innerHTML = svg(icons[kind], 24);
    if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password || window.location.protocol === 'https:' && url.protocol === 'http:') continue;
    // Fetch only in the visitor's browser; never proxy private destinations through the server.
    const image = document.createElement('img'); image.alt = ''; image.width = 32; image.height = 32; image.loading = 'lazy'; image.decoding = 'async'; image.referrerPolicy = 'no-referrer';
    image.addEventListener('load', () => {if (image.naturalWidth) {image.setAttribute('data-loaded', 'true'); fallback.hidden = true}}, {once: true});
    image.addEventListener('error', () => image.remove(), {once: true}); image.src = `${url.origin}/favicon.ico`; holder.append(image);
  }
}

export function initHomeLinks(): void {
  const root = document.querySelector<HTMLElement>('[data-home-links]');
  if (!root || root.hasAttribute('data-home-links-ready')) return;
  root.setAttribute('data-home-links-ready', 'true');
  enhanceHomeLinkCards(root);
  const pending = new WeakSet<HTMLFormElement>();
  root.addEventListener('click', (event) => {
    const target = event.target as HTMLElement;
    const add = target.closest<HTMLButtonElement>('[data-home-link-add]');
    if (add) {
      const form = add.form!;
      const rows = form.querySelector('[data-home-link-rows]')!;
      const original = form.querySelectorAll('[name="page_index"]').length;
      const total = Number(form.getAttribute('data-scope-total') || 0);
      if (rows.children.length >= 64 || total - original + rows.children.length >= 10000) return;
      const fragment = form.querySelector<HTMLTemplateElement>('[data-home-link-template]')!.content.cloneNode(true);
      rows.append(fragment);
      rows.lastElementChild!.querySelector<HTMLInputElement>('input:not([type="hidden"])')!.focus();
    }
    const remove = target.closest<HTMLButtonElement>('[data-home-link-remove]');
    if (remove) remove.closest('.home-link-row')!.remove();
  });
  root.addEventListener('submit', async (event) => {
    const form = event.target as HTMLFormElement;
    if (!form.matches('[data-home-links-form]')) return;
    event.preventDefault();
    if (pending.has(form)) return;
    pending.add(form);
    const data = new FormData(form);
    const controls = Array.from(root.querySelectorAll<HTMLInputElement | HTMLButtonElement>('input, button'));
    for (const control of controls) control.disabled = true;
    try {
      const response = await POST(form.action, {data, headers: {'X-GitSSS-Partial': '1'}});
      if (!response.ok) throw new Error(root.getAttribute(response.status === 409 ? 'data-conflict' : response.status === 400 ? 'data-invalid' : 'data-error')!);
      const next = new DOMParser().parseFromString(await response.text(), 'text/html').querySelector<HTMLElement>('[data-home-links]');
      if (!next) throw new Error(root.getAttribute('data-error')!);
      // Refresh the saved scope only so another open editor keeps its unsaved draft.
      const results = root.querySelector('[data-home-links-results]');
      const nextResults = next.querySelector('[data-home-links-results]');
      if (results && nextResults) results.replaceChildren(...Array.from(nextResults.childNodes));
      else root.querySelector('.home-links-grid')!.replaceChildren(...Array.from(next.querySelector('.home-links-grid')!.childNodes));
      enhanceHomeLinkCards(root);
      const scope = form.querySelector<HTMLInputElement>('[name="scope"]')!.value;
      const saved = next.querySelector<HTMLInputElement>(`[name="scope"][value="${CSS.escape(scope)}"]`)!.form!;
      form.replaceChildren(...Array.from(saved.childNodes));
      form.setAttribute('data-scope-total', saved.getAttribute('data-scope-total') || '0');
      form.classList.remove('dirty');
    } catch (error) {
      showErrorToast(error instanceof Error ? error.message : root.getAttribute('data-error')!);
    } finally {
      for (const control of controls) control.disabled = false;
      pending.delete(form);
    }
  });
}
