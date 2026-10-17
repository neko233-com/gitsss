import {GET} from '../modules/fetch.ts';

type Totals = {commits: number; additions: number; deletions: number; files: number; binary_files: number};
type Day = Totals & {date: string};
type Author = Totals & {id: string; name: string; days: Record<string, Day>};
type Report = Totals & {authors: Author[]; revision: string; from: string; to: string; timezone: string; truncated: boolean};
const fields = ['commits', 'additions', 'deletions', 'files', 'binary_files'] as const;
export type ContributionRow = Totals & {author: string; period: string};

export function contributionRows(authors: Author[], group: string, search: string): ContributionRow[] {
  const result: ContributionRow[] = [];
  const names = new Map<string, number>();
  for (const author of authors) names.set(author.name, (names.get(author.name) || 0) + 1);
  for (const author of authors) {
    if (!author.name.toLocaleLowerCase().includes(search.toLocaleLowerCase().trim())) continue;
    const buckets = new Map<string, ContributionRow>();
    for (const day of Object.values(author.days)) {
      let period = day.date;
      if (group === 'month') period = day.date.slice(0, 7);
      else if (group === 'week') {
        const date = new Date(`${day.date}T00:00:00Z`);
        date.setUTCDate(date.getUTCDate() - (date.getUTCDay() + 6) % 7);
        period = date.toISOString().slice(0, 10);
      }
      if (!buckets.has(period)) buckets.set(period, {author: names.get(author.name)! > 1 ? `${author.name} (${author.id.slice(0, 8)})` : author.name, period, commits: 0, additions: 0, deletions: 0, files: 0, binary_files: 0});
      for (const field of fields) buckets.get(period)![field] += day[field];
    }
    result.push(...buckets.values());
  }
  return result.sort((a, b) => b.period.localeCompare(a.period) || b.commits - a.commits || a.author.localeCompare(b.author));
}

export function contributionCSV(rows: ContributionRow[]): string {
  const cell = (value: string | number) => `"${String(value).replace(/^(?:\s*[=+\-@]|[\t\r])/u, "'$&").replaceAll('"', '""')}"`;
  return [['author', 'period', ...fields], ...rows.map((row) => [row.author, row.period, ...fields.map((field) => row[field])])].map((row) => row.map(cell).join(',')).join('\r\n');
}

export function initContributionReport(): void {
  const root = document.querySelector<HTMLElement>('[data-contribution-report]');
  if (!root) return;
  const form = root.querySelector<HTMLFormElement>('form')!;
  const from = form.elements.namedItem('from') as HTMLInputElement;
  const to = form.elements.namedItem('to') as HTMLInputElement;
  const group = form.elements.namedItem('group') as HTMLSelectElement;
  const search = form.elements.namedItem('search') as HTMLInputElement;
  const status = root.querySelector<HTMLElement>('[data-contribution-status]')!;
  const tbody = root.querySelector<HTMLElement>('[data-contribution-rows]')!;
  const exportButton = root.querySelector<HTMLButtonElement>('[data-contribution-export]')!;
  const params = new URLSearchParams(window.location.search);
  const date = new Date();
  const localDate = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
  to.value = params.get('to') || localDate(date);
  date.setDate(date.getDate() - 29);
  from.value = params.get('from') || localDate(date);
  group.value = params.get('group') || 'day';
  let report: Report | undefined;
  let pending: AbortController | undefined;
  let rows: ContributionRow[] = [];
  let page = 1;
  const prev = root.querySelector<HTMLButtonElement>('[data-contribution-prev]')!;
  const nextButton = root.querySelector<HTMLButtonElement>('[data-contribution-next]')!;
  const render = () => {
    if (!report) return;
    rows = contributionRows(report.authors, group.value, search.value);
    const fragment = document.createDocumentFragment();
    page = Math.min(page, Math.max(1, Math.ceil(rows.length / 200)));
    for (const row of rows.slice((page - 1) * 200, page * 200)) {
      const tr = document.createElement('tr');
      for (const value of [row.author, row.period, ...fields.map((field) => row[field].toLocaleString())]) {
        const td = document.createElement('td'); td.textContent = value; tr.append(td);
      }
      fragment.append(tr);
    }
    tbody.replaceChildren(fragment);
    prev.disabled = page <= 1; nextButton.disabled = page * 200 >= rows.length;
    root.querySelector('[data-contribution-page]')!.textContent = `${page} / ${Math.max(1, Math.ceil(rows.length / 200))} · ${rows.length}`;
    for (const field of fields) root.querySelector<HTMLElement>(`[data-contribution-total="${CSS.escape(field)}"]`)!.textContent = rows.reduce((total, row) => total + row[field], 0).toLocaleString();
    status.textContent = report.truncated ? root.getAttribute('data-truncated')! : `${report.from} – ${report.to} · ${report.timezone} · ${report.revision.slice(0, 12)}`;
    exportButton.disabled = false;
    const next = new URL(window.location.href);
    next.searchParams.set('from', from.value); next.searchParams.set('to', to.value); next.searchParams.set('group', group.value);
    window.history.replaceState({}, '', next);
  };
  const load = async () => {
    if (pending) pending.abort();
    const controller = new AbortController(); pending = controller;
    root.setAttribute('aria-busy', 'true'); status.textContent = '…';
    exportButton.disabled = true;
    const query = new URLSearchParams({from: from.value, to: to.value, timezone: new Intl.DateTimeFormat().resolvedOptions().timeZone});
    try {
      const response = await GET(`${root.getAttribute('data-url')}?${query}`, {signal: controller.signal});
      if (!response.ok) throw new Error(root.getAttribute('data-error')!);
      const next = await response.json();
      if (controller.signal.aborted) return;
      report = next; render();
    } catch {
      if (!controller.signal.aborted) status.textContent = root.getAttribute('data-error')!;
    } finally {
      if (pending === controller) root.removeAttribute('aria-busy');
    }
  };
  form.addEventListener('submit', (event) => {event.preventDefault(); load()});
  group.addEventListener('change', () => {page = 1; render()});
  search.addEventListener('input', () => {page = 1; render()});
  prev.addEventListener('click', () => {page--; render()});
  nextButton.addEventListener('click', () => {page++; render()});
  exportButton.addEventListener('click', () => {
    const url = URL.createObjectURL(new Blob(['\uFEFF', contributionCSV(rows)], {type: 'text/csv;charset=utf-8'}));
    const link = document.createElement('a'); link.href = url; link.download = `contributions-${report!.from}-${report!.to}.csv`; link.click(); URL.revokeObjectURL(url);
  });
  load();
}
