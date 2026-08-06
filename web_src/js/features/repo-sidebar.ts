const sidebarStorageKey = 'gitsss-repo-sidebar-collapsed';

function applySidebarState(toggle: HTMLButtonElement, collapsed: boolean): void {
  document.body.classList.toggle('repo-sidebar-collapsed', collapsed);
  toggle.setAttribute('aria-expanded', String(!collapsed));
  toggle.setAttribute('title', collapsed ? toggle.getAttribute('data-expand-title')! : toggle.getAttribute('data-collapse-title')!);
  toggle.innerHTML = collapsed ? '<svg class="svg octicon-chevron-right" width="16" height="16" aria-hidden="true"><path d="m6.22 3.97 4.03 4.03-4.03 4.03a.75.75 0 0 0 1.06 1.06l4.56-4.56a.75.75 0 0 0 0-1.06L7.28 2.91a.75.75 0 0 0-1.06 1.06Z"></path></svg>' : '<svg class="svg octicon-chevron-left" width="16" height="16" aria-hidden="true"><path d="m9.78 3.97-4.03 4.03 4.03 4.03a.75.75 0 0 1-1.06 1.06l-4.56-4.56a.75.75 0 0 1 0-1.06l4.56-4.56a.75.75 0 0 1 1.06 1.06Z"></path></svg>';
}

export function initRepoSidebar(): void {
  const sidebar = document.querySelector<HTMLElement>('.repo-sidebar');
  const toggle = sidebar ? sidebar.querySelector<HTMLButtonElement>('[data-repo-sidebar-toggle]') : null;
  if (!sidebar || !toggle) return;
  toggle.setAttribute('data-collapse-title', toggle.getAttribute('title')!);
  let collapsed = false;
  try {
    collapsed = localStorage.getItem(sidebarStorageKey) === 'true';
  } catch {
    // Storage may be unavailable in a hardened browser context.
  }
  applySidebarState(toggle, collapsed);
  toggle.addEventListener('click', () => {
    collapsed = !collapsed;
    applySidebarState(toggle, collapsed);
    try {
      localStorage.setItem(sidebarStorageKey, String(collapsed));
    } catch {
      // The sidebar remains usable without persistence.
    }
  });
}
