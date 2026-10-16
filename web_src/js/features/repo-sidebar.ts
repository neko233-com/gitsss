import {localUserSettings} from '../modules/user-settings.ts';
import {html} from '../utils/html.ts';

const sidebarStorageKey = 'gitsss-repo-sidebar-collapsed';

function applySidebarState(toggle: HTMLButtonElement, collapsed: boolean): void {
  document.body.classList.toggle('repo-sidebar-collapsed', collapsed);
  toggle.setAttribute('aria-expanded', String(!collapsed));
  const label = collapsed ? toggle.getAttribute('data-expand-title')! : toggle.getAttribute('data-collapse-title')!;
  toggle.setAttribute('title', label);
  toggle.setAttribute('aria-label', label);
  toggle.innerHTML = collapsed ? html`<svg class="svg octicon-chevron-right" width="16" height="16" aria-hidden="true"><path d="m6.22 3.97 4.03 4.03-4.03 4.03a.75.75 0 0 0 1.06 1.06l4.56-4.56a.75.75 0 0 0 0-1.06L7.28 2.91a.75.75 0 0 0-1.06 1.06Z"></path></svg>` : html`<svg class="svg octicon-chevron-left" width="16" height="16" aria-hidden="true"><path d="m9.78 3.97-4.03 4.03 4.03 4.03a.75.75 0 0 1-1.06 1.06l-4.56-4.56a.75.75 0 0 1 0-1.06l4.56-4.56a.75.75 0 0 1 1.06 1.06Z"></path></svg>`;
}

export function initRepoSidebar(): void {
  const sidebar = document.querySelector<HTMLElement>('.repo-sidebar');
  const toggle = sidebar ? sidebar.querySelector<HTMLButtonElement>('[data-repo-sidebar-toggle]') : null;
  if (!sidebar || !toggle) return;
  for (const link of sidebar.querySelectorAll<HTMLAnchorElement>('.repo-sidebar-link')) {
    const label = link.textContent.trim();
    link.setAttribute('aria-label', label);
    link.title = label;
    if (link.classList.contains('active')) link.setAttribute('aria-current', 'page');
  }
  toggle.setAttribute('data-collapse-title', toggle.getAttribute('title')!);
  let collapsed = false;
  try {
    collapsed = localUserSettings.getBoolean(sidebarStorageKey);
  } catch {
    // Storage may be unavailable in a hardened browser context.
  }
  applySidebarState(toggle, collapsed);
  toggle.addEventListener('click', () => {
    collapsed = !collapsed;
    applySidebarState(toggle, collapsed);
    try {
      localUserSettings.setBoolean(sidebarStorageKey, collapsed);
    } catch {
      // The sidebar remains usable without persistence.
    }
  });

  const mobileOpen = document.querySelector<HTMLButtonElement>('[data-repo-sidebar-mobile-open]');
  const mobileCloseButtons = document.querySelectorAll<HTMLButtonElement>('[data-repo-sidebar-mobile-close]');
  if (!mobileOpen || mobileCloseButtons.length === 0) return;

  const desktopQuery = window.matchMedia('(min-width: 1200px)');
  const setMobileState = (open: boolean): void => {
    const isDesktop = desktopQuery.matches;
    const active = open && !isDesktop;
    document.body.classList.toggle('repo-sidebar-mobile-open', active);
    sidebar.setAttribute('aria-hidden', String(!active && !isDesktop));
    mobileOpen.setAttribute('aria-expanded', String(active));
    mobileOpen.setAttribute('aria-label', active ? mobileOpen.getAttribute('data-close-label')! : mobileOpen.getAttribute('data-open-label')!);
  };

  const openLabel = mobileOpen.getAttribute('aria-label');
  mobileOpen.setAttribute('data-open-label', openLabel || mobileOpen.textContent.trim());
  const closeLabel = mobileCloseButtons[0].getAttribute('aria-label');
  mobileOpen.setAttribute('data-close-label', closeLabel || mobileOpen.textContent.trim());
  setMobileState(false);
  mobileOpen.addEventListener('click', () => setMobileState(true));
  for (const closeButton of mobileCloseButtons) closeButton.addEventListener('click', () => setMobileState(false));
  for (const link of sidebar.querySelectorAll<HTMLAnchorElement>('a')) link.addEventListener('click', () => setMobileState(false));
  document.addEventListener('keydown', (event) => {
    if (event.key === 'Escape') setMobileState(false);
  });
  desktopQuery.addEventListener('change', () => setMobileState(false));
}
