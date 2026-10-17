export function initWorkspace(): void {
  const root = document.querySelector<HTMLElement>('.workspace-sidebar');
  if (root) {
    const input = root.querySelector<HTMLInputElement>('[data-workspace-org-filter]');
    const rows = Array.from(root.querySelectorAll<HTMLElement>('[data-workspace-org]'), (row) => ({row, text: row.getAttribute('data-search')!.normalize('NFKC').toLocaleLowerCase()}));
    input?.addEventListener('input', () => {
      const query = input.value.normalize('NFKC').trim().toLocaleLowerCase();
      let visible = 0;
      for (const {row, text} of rows) {
        row.hidden = !text.includes(query);
        if (!row.hidden) visible++;
      }
      const empty = root.querySelector<HTMLElement>('[data-workspace-org-empty]');
      if (empty) empty.hidden = visible > 0;
    });
    for (const link of root.querySelectorAll<HTMLAnchorElement>('.workspace-nav > a.active')) link.setAttribute('aria-current', 'page');
  }
  const search = document.querySelector<HTMLInputElement>('[data-workspace-search]');
  if (!search) return;
  document.addEventListener('keydown', (event) => {
    const target = event.target as HTMLElement;
    if (event.defaultPrevented || event.isComposing || target.closest('input, textarea, select, [contenteditable="true"], [role="textbox"]')) return;
    if (event.key === '/' && !event.altKey && !event.ctrlKey && !event.metaKey) {
      event.preventDefault();
      search.focus();
    }
  });
}
