function initWorkspaceNavigation(root: HTMLElement): void {
  const open = document.querySelector<HTMLButtonElement>('[data-workspace-nav-open]');
  const close = root.querySelector<HTMLButtonElement>('[data-workspace-nav-close]');
  if (!open || !close) return;
  const desktop = window.matchMedia('(min-width: 1051px)');
  const setOpen = (active: boolean, restoreFocus = false): void => {
    active = active && !desktop.matches;
    document.body.classList.toggle('workspace-nav-open', active);
    root.inert = !desktop.matches && !active;
    open.setAttribute('aria-expanded', String(active));
    if (active) close.focus();
    else if (restoreFocus && !desktop.matches) open.focus();
  };
  document.body.classList.add('workspace-nav-ready');
  setOpen(false);
  open.addEventListener('click', () => setOpen(true));
  for (const button of document.querySelectorAll('[data-workspace-nav-close]')) button.addEventListener('click', () => setOpen(false, true));
  root.addEventListener('keydown', (event) => {
    if (desktop.matches || !document.body.classList.contains('workspace-nav-open')) return;
    if (event.key === 'Escape') { event.preventDefault(); setOpen(false, true) }
    if (event.key !== 'Tab') return;
    const controls = Array.from(root.querySelectorAll<HTMLElement>('a[href], button, input')).filter((e) => e.getClientRects().length && !e.matches(':disabled, [tabindex="-1"]'));
    const first = controls[0], last = controls.at(-1);
    if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last!.focus() } else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
  });
  desktop.addEventListener('change', () => {
    const focusedInside = root.contains(document.activeElement);
    setOpen(false, focusedInside);
  });
}

export function initWorkspace(): void {
  const root = document.querySelector<HTMLElement>('.workspace-sidebar');
  if (root) {
    initWorkspaceNavigation(root);
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
