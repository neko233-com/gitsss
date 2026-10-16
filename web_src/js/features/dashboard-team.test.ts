import {initDashboardTeamFilter} from './dashboard-team.ts';
import {GET} from '../modules/fetch.ts';
import {replaceHeatmap} from './heatmap.ts';

vi.mock('../modules/fetch.ts', () => ({GET: vi.fn()}));
vi.mock('./heatmap.ts', () => ({replaceHeatmap: vi.fn()}));

test('team selection updates activity and repositories in place while preserving search', async () => {
  window.history.replaceState({}, '', '/org/acme/dashboard?repo-search-query=build');
  document.body.innerHTML = `<div data-dashboard-team-select>
    <span data-dashboard-team-label>All</span>
    <a href="/org/acme/dashboard" data-dashboard-team="0" data-label="All">All</a>
    <a href="/org/acme/dashboard/ci-cd" data-dashboard-team="5" data-label="Release team">Release team</a>
    </div><a data-org-team-nav="/org/acme/issues"></a>
    <div data-dashboard-feed-area data-error="Failed">Old activity</div>`;
  vi.mocked(GET).mockResolvedValue(new Response('<div data-dashboard-feed-area>Team activity</div><div id="user-heatmap" data-heatmap-url="/org/acme/dashboard/-/heatmap/ci-cd"></div>'));
  const changed = vi.fn();
  window.addEventListener('gitsss:dashboard-team', changed);
  initDashboardTeamFilter();
  document.querySelector<HTMLAnchorElement>('[data-dashboard-team="5"]')!.click();
  await vi.waitFor(() => expect(changed).toHaveBeenCalledOnce());
  expect(document.querySelector('[data-dashboard-feed-area]')!.textContent).toBe('Team activity');
  expect(document.querySelector('[data-dashboard-team-label]')!.textContent).toBe('Release team');
  await vi.waitFor(() => expect(replaceHeatmap).toHaveBeenCalledOnce());
  expect(vi.mocked(replaceHeatmap).mock.calls[0][0].getAttribute('data-heatmap-url')).toBe('/org/acme/dashboard/-/heatmap/ci-cd');
  expect(window.location.pathname).toBe('/org/acme/dashboard/ci-cd');
  expect(window.location.search).toContain('repo-search-query=build');
  expect(document.querySelector('a[data-org-team-nav]')!.getAttribute('href')).toBe('/org/acme/issues/ci-cd');
  window.removeEventListener('gitsss:dashboard-team', changed);
});
