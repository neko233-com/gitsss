import {createApp} from 'vue';
import type {App} from 'vue';
import {translateMonth, translateDay} from '../utils.ts';
import {GET} from '../modules/fetch.ts';

type HeatmapResponse = {
  heatmapData: Array<[number, number]>; // [[1617235200, 2]] = [unix timestamp, count]
  totalContributions: number;
};

const views = new WeakMap<HTMLElement, App>();
const requests = new WeakMap<HTMLElement, AbortController>();

export async function replaceHeatmap(next: HTMLElement): Promise<void> {
  const current = document.querySelector<HTMLElement>('#user-heatmap');
  if (!current) return;
  requests.get(current)?.abort();
  views.get(current)?.unmount();
  views.delete(current);
  current.replaceWith(next);
  await initHeatmap();
}

export async function initHeatmap() {
  const el = document.querySelector<HTMLElement>('#user-heatmap');
  if (!el) return;
  requests.get(el)?.abort();
  const request = new AbortController();
  requests.set(el, request);
  el.setAttribute('aria-busy', 'true');
  try {
    const url = el.getAttribute('data-heatmap-url')!;
    const resp = await GET(url, {signal: request.signal});
    if (!resp.ok) throw new Error(`Failed to load heatmap data: ${resp.status} ${resp.statusText}`);
    const {heatmapData, totalContributions} = await resp.json() as HeatmapResponse;

    const heatmap: Record<string, number> = {};
    for (const [timestamp, contributions] of heatmapData) {
      // Convert to user timezone and sum contributions by date
      const dateStr = new Date(timestamp * 1000).toDateString();
      heatmap[dateStr] = (heatmap[dateStr] || 0) + contributions;
    }

    const values = Object.entries(heatmap).map(([dateStr, count]) => {
      return {date: new Date(dateStr), count};
    });

    const totalFormatted = totalContributions.toLocaleString();
    const textTotalContributions = el.getAttribute('data-locale-total-contributions')!.replace('%s', totalFormatted);

    // last heatmap tooltip localization attempt https://github.com/go-gitea/gitea/pull/24131/commits/a83761cbbae3c2e3b4bced71e680f44432073ac8
    const locale = {
      heatMapLocale: {
        months: new Array(12).fill(undefined).map((_, idx) => translateMonth(idx)),
        days: new Array(7).fill(undefined).map((_, idx) => translateDay(idx)),
        on: ' - ', // no correct locale support for it, because in many languages the sentence is not "something on someday"
        more: el.getAttribute('data-locale-more'),
        less: el.getAttribute('data-locale-less'),
      },
      tooltipUnit: el.getAttribute('data-locale-contributions') || 'contributions',
      textTotalContributions,
      noDataText: el.getAttribute('data-locale-no-contributions'),
    };

    const {default: ActivityHeatmap} = await import('../components/ActivityHeatmap.vue');
    if (request.signal.aborted) return;
    views.get(el)?.unmount();
    const View = createApp(ActivityHeatmap, {values, locale});
    View.mount(el);
    views.set(el, View);
    el.classList.remove('is-loading');
  } catch (err) {
    if (request.signal.aborted) return;
    console.error('Heatmap failed to load', err);
    views.get(el)?.unmount();
    views.delete(el);
    el.classList.remove('is-loading');
    el.textContent = el.getAttribute('data-locale-error') || 'Heatmap failed to load';
  } finally {
    if (requests.get(el) === request) el.removeAttribute('aria-busy');
  }
}
