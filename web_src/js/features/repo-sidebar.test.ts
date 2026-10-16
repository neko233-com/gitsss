import {initRepoSidebar} from './repo-sidebar.ts';
import {localUserSettings} from '../modules/user-settings.ts';

const sidebarHTML = `
  <button type="button" data-repo-sidebar-mobile-open aria-label="Open repository navigation">Open</button>
  <button type="button" data-repo-sidebar-mobile-close aria-label="Close repository navigation"></button>
  <aside class="repo-sidebar">
    <div class="repo-sidebar-header">
      <button type="button" data-repo-sidebar-toggle data-expand-title="Expand" title="Collapse"></button>
      <button type="button" data-repo-sidebar-mobile-close aria-label="Close repository navigation"></button>
    </div>
    <nav><a class="repo-sidebar-link active" href="/code"><span class="repo-sidebar-label">Code</span></a></nav>
  </aside>
`;

describe('repository sidebar', () => {
  beforeEach(() => {
    document.body.innerHTML = sidebarHTML;
    localUserSettings.setBoolean('gitsss-repo-sidebar-collapsed', false);
    vi.stubGlobal('matchMedia', vi.fn(() => ({
      matches: false,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })));
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  test('opens and closes the mobile navigation', () => {
    initRepoSidebar();
    const open = document.querySelector<HTMLButtonElement>('[data-repo-sidebar-mobile-open]')!;
    const close = document.querySelector<HTMLButtonElement>('[data-repo-sidebar-mobile-close]')!;
    const sidebar = document.querySelector<HTMLElement>('.repo-sidebar')!;

    open.click();
    expect(document.body.classList.contains('repo-sidebar-mobile-open')).toBe(true);
    expect(sidebar.getAttribute('aria-hidden')).toBe('false');
    expect(open.getAttribute('aria-expanded')).toBe('true');

    close.click();
    expect(document.body.classList.contains('repo-sidebar-mobile-open')).toBe(false);
    expect(sidebar.getAttribute('aria-hidden')).toBe('true');
  });

  test('persists desktop collapsed state', () => {
    initRepoSidebar();
    const toggle = document.querySelector<HTMLButtonElement>('[data-repo-sidebar-toggle]')!;
    toggle.click();
    expect(document.body.classList.contains('repo-sidebar-collapsed')).toBe(true);
    expect(localUserSettings.getBoolean('gitsss-repo-sidebar-collapsed')).toBe(true);
    expect(toggle.getAttribute('aria-label')).toBe('Expand');
    const link = document.querySelector<HTMLAnchorElement>('.repo-sidebar-link')!;
    expect(link.getAttribute('aria-label')).toBe('Code');
    expect(link.getAttribute('aria-current')).toBe('page');
    expect(link.title).toBe('Code');
  });
});
