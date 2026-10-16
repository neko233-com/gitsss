import {POST} from '../modules/fetch.ts';
import {showErrorToast} from '../modules/toast.ts';

export function initHomeLinks(): void {
  const root = document.querySelector<HTMLElement>('[data-home-links]');
  if (!root || root.hasAttribute('data-home-links-ready')) return;
  root.setAttribute('data-home-links-ready', 'true');
  const pending = new WeakSet<HTMLFormElement>();
  root.addEventListener('click', (event) => {
    const target = event.target as HTMLElement;
    const add = target.closest<HTMLButtonElement>('[data-home-link-add]');
    if (add) {
      const form = add.form!;
      const rows = form.querySelector('[data-home-link-rows]')!;
      if (rows.children.length >= 32) return;
      const fragment = form.querySelector<HTMLTemplateElement>('[data-home-link-template]')!.content.cloneNode(true);
      rows.append(fragment);
      rows.lastElementChild!.querySelector<HTMLInputElement>('input')!.focus();
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
      const response = await POST(form.action, {data});
      if (!response.ok) throw new Error(root.getAttribute(response.status === 409 ? 'data-conflict' : response.status === 400 ? 'data-invalid' : 'data-error')!);
      const next = new DOMParser().parseFromString(await response.text(), 'text/html').querySelector<HTMLElement>('[data-home-links]');
      if (!next) throw new Error(root.getAttribute('data-error')!);
      // Refresh the saved scope only so another open editor keeps its unsaved draft.
      root.querySelector('.home-links-grid')!.replaceChildren(...Array.from(next.querySelector('.home-links-grid')!.childNodes));
      const scope = form.querySelector<HTMLInputElement>('[name="scope"]')!.value;
      const saved = next.querySelector<HTMLInputElement>(`[name="scope"][value="${CSS.escape(scope)}"]`)!.form!;
      form.replaceChildren(...Array.from(saved.childNodes));
      form.classList.remove('dirty');
    } catch (error) {
      showErrorToast(error instanceof Error ? error.message : root.getAttribute('data-error')!);
    } finally {
      for (const control of controls) control.disabled = false;
      pending.delete(form);
    }
  });
}
