import {GET} from '../modules/fetch.ts';
import {showErrorToast} from '../modules/toast.ts';

export function initDashboardTeamFilter(): void {
  const selector = document.querySelector<HTMLElement>('[data-dashboard-team-select]');
  const area = document.querySelector<HTMLElement>('[data-dashboard-feed-area]');
  if (!selector || !area) return;
  if (selector.hasAttribute('data-team-ready')) return;
  selector.setAttribute('data-team-ready', 'true');
  const links = Array.from(selector.querySelectorAll<HTMLAnchorElement>('[data-dashboard-team]'));
  const base = new URL(links.find((link) => link.getAttribute('data-dashboard-team') === '0')!.href);
  let pending: AbortController | undefined;

  const loadTeam = async (link: HTMLAnchorElement, pushHistory: boolean) => {
    if (pending) pending.abort();
    const controller = new AbortController();
    pending = controller;
    const url = new URL(link.href);
    url.search = window.location.search;
    url.searchParams.delete('page');
    url.searchParams.delete('repo-search-page');
    area.setAttribute('aria-busy', 'true');
    try {
      const response = await GET(url.href, {signal: controller.signal});
      if (!response.ok) throw new Error('dashboard request failed');
      const parsed = new DOMParser().parseFromString(await response.text(), 'text/html');
      const next = parsed.querySelector('[data-dashboard-feed-area]');
      if (!next) throw new Error('dashboard response missing');
      area.replaceChildren(...Array.from(next.childNodes));
      for (const item of links) {
        const selected = item === link;
        item.classList.toggle('selected', selected);
        item.classList.toggle('active', selected);
      }
      selector.querySelector('[data-dashboard-team-label]')!.textContent = link.getAttribute('data-label');
      const suffix = url.pathname.slice(base.pathname.length);
      for (const item of document.querySelectorAll<HTMLAnchorElement>('[data-org-team-nav]')) {
        item.href = item.getAttribute('data-org-team-nav')! + suffix;
      }
      const activity = document.querySelector<HTMLAnchorElement>('[data-org-dashboard-link]');
      if (activity) activity.href = url.pathname;
      if (pushHistory) window.history.pushState({}, '', url);
      window.dispatchEvent(new CustomEvent('gitsss:dashboard-team', {detail: {teamId: Number(link.getAttribute('data-dashboard-team'))}}));
    } catch {
      if (!controller.signal.aborted) showErrorToast(area.getAttribute('data-error')!);
    } finally {
      if (pending === controller) area.removeAttribute('aria-busy');
    }
  };
  for (const link of links) {
    link.addEventListener('click', async (event) => {
      if (event.ctrlKey || event.metaKey || event.shiftKey || event.altKey || event.button !== 0) return;
      event.preventDefault();
      await loadTeam(link, true);
    });
  }
  window.addEventListener('popstate', async () => {
    const link = links.find((item) => new URL(item.href).pathname === window.location.pathname);
    if (link) await loadTeam(link, false);
  });
}
