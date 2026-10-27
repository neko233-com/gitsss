import {debounce} from 'throttle-debounce';
import {GET} from './fetch.ts';
import {errorName} from './errors.ts';
import {html, htmlRaw} from '../utils/html.ts';
import {urlQueryEscape} from '../utils/url.ts';

export type SearchResult = {
  title: string;
  description?: string;
  image?: string;
};

function buildResultHTML(result: SearchResult): string {
  const img = result.image ? html`<div class="image"><img src="${result.image}" alt=""></div>` : '';
  const desc = result.description ? html`<div class="description">${result.description}</div>` : '';
  return html`${htmlRaw(img)}<div class="content"><div class="title">${result.title}</div>${htmlRaw(desc)}</div>`;
}

function buildResultElement(result: SearchResult): HTMLElement {
  const item = document.createElement('div');
  item.className = 'result';
  item.innerHTML = buildResultHTML(result);
  return item;
}

// single delegated outside-click handler; each attachSearchBox registers a {container, hide} entry
const outsideClickBoxes = new Set<{container: HTMLElement; hide: () => void}>();
document.addEventListener('click', (event) => {
  for (const box of outsideClickBoxes) {
    if (!box.container.contains(event.target as Node)) box.hide();
  }
});

/** Attach an API-driven autocomplete to `container`. `parse` maps the raw JSON response into the rendered result list. The selected result's title is written to the input on selection. */
export function attachSearchBox<T = unknown>(container: HTMLElement, url: string, parse: (raw: T, query: string) => SearchResult[], {minCharacters = 2, openOnFocus = false, emptyText, errorText}: {minCharacters?: number; openOnFocus?: boolean; emptyText?: string; errorText?: string} = {}): void {
  const input = container.querySelector<HTMLInputElement>('input.prompt') ?? container.querySelector<HTMLInputElement>('input');
  if (!input) return;

  let resultsEl = container.querySelector<HTMLElement>(':scope > .results');
  if (!resultsEl) {
    resultsEl = document.createElement('div');
    resultsEl.className = 'results';
    container.append(resultsEl);
  }
  resultsEl.id ||= `${container.id}-results`;
  resultsEl.setAttribute('role', 'listbox');
  input.setAttribute('role', 'combobox');
  input.setAttribute('aria-autocomplete', 'list');
  input.setAttribute('aria-controls', resultsEl.id);
  input.setAttribute('aria-expanded', 'false');
  const itemResults = new Map<HTMLElement, SearchResult>();
  let fetchController: AbortController | null = null;
  let hideTimer: ReturnType<typeof setTimeout> | undefined;

  const hide = () => {
    fetchController?.abort();
    resultsEl.style.display = 'none';
    input.setAttribute('aria-expanded', 'false');
    input.removeAttribute('aria-activedescendant');
    resultsEl.replaceChildren();
    itemResults.clear();
  };

  const showMessage = (message?: string) => {
    if (!message) return hide();
    itemResults.clear();
    const el = document.createElement('div');
    el.className = 'search-message';
    el.setAttribute('role', 'status');
    el.textContent = message;
    resultsEl.replaceChildren(el);
    resultsEl.style.display = 'block';
    input.setAttribute('aria-expanded', 'true');
    input.removeAttribute('aria-activedescendant');
  };

  const render = (results: SearchResult[]) => {
    if (!results.length) return showMessage(emptyText);
    itemResults.clear();
    resultsEl.replaceChildren(...results.map((result, index) => {
      const item = buildResultElement(result);
      item.id = `${resultsEl.id}-${index}`;
      item.setAttribute('role', 'option');
      item.setAttribute('aria-selected', 'false');
      itemResults.set(item, result);
      return item;
    }));
    resultsEl.style.display = 'block';
    input.setAttribute('aria-expanded', 'true');
    input.removeAttribute('aria-activedescendant');
  };

  const select = (item: HTMLElement) => {
    input.value = itemResults.get(item)!.title;
    input.dispatchEvent(new Event('change', {bubbles: true}));
    hide();
  };

  const search = debounce(200, async (query: string) => {
    fetchController?.abort();
    if (query.length < minCharacters) return hide();
    const ctrl = (fetchController = new AbortController());
    try {
      const response = await GET(url.replaceAll('{query}', urlQueryEscape(query)), {signal: ctrl.signal});
      if (!response.ok) {
        if (!ctrl.signal.aborted && input.value === query) showMessage(errorText);
        return;
      }
      const results = parse(await response.json(), query);
      // only render if the fetch wasn't aborted (e.g. by hide()) and the input still matches
      if (!ctrl.signal.aborted && input.value === query) render(results);
    } catch (err) {
      if (errorName(err) !== 'AbortError' && !ctrl.signal.aborted && input.value === query) showMessage(errorText);
    }
  });
  // cancel + hide ensures a debounced fetch scheduled before any of these can't fire afterwards
  const dismiss = () => { search.cancel({upcomingOnly: true}); hide() };

  input.addEventListener('input', () => search(input.value));
  input.addEventListener('focus', () => {
    clearTimeout(hideTimer);
    if (openOnFocus) search(input.value);
    else if (itemResults.size) resultsEl.style.display = 'block';
  });
  input.addEventListener('click', () => {
    if (openOnFocus && resultsEl.style.display === 'none') search(input.value);
  });
  input.addEventListener('blur', () => { search.cancel({upcomingOnly: true}); hideTimer = setTimeout(hide, 150) }); // hide deferred so a result mousedown can land first
  input.addEventListener('keydown', (event) => {
    if (event.key === 'Escape') { dismiss(); return }
    const resultEls = Array.from(resultsEl.querySelectorAll<HTMLElement>('.result'));
    if (!resultEls.length) return;
    const index = resultEls.findIndex((item) => item.classList.contains('active'));
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault();
      resultEls[index]?.classList.remove('active');
      resultEls[index]?.setAttribute('aria-selected', 'false');
      const next = event.key === 'ArrowDown' ? (index + 1) % resultEls.length : index <= 0 ? resultEls.length - 1 : index - 1;
      resultEls[next].classList.add('active');
      resultEls[next].setAttribute('aria-selected', 'true');
      input.setAttribute('aria-activedescendant', resultEls[next].id);
      resultEls[next].scrollIntoView({block: 'nearest'});
    } else if (event.key === 'Enter' && index >= 0) {
      event.preventDefault();
      select(resultEls[index]);
    }
  });
  // mousedown fires before input blur so the selection registers before blur-hide kicks in
  resultsEl.addEventListener('mousedown', (event) => {
    const target = (event.target as HTMLElement).closest<HTMLElement>('.result');
    if (!target) return;
    event.preventDefault();
    select(target);
  });
  outsideClickBoxes.add({container, hide: dismiss});
}
