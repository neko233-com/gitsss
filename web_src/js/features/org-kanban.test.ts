import {filterBoardCards, initOrgKanban} from './org-kanban.ts';
import {POST} from '../modules/fetch.ts';

vi.mock('../modules/fetch.ts', () => ({POST: vi.fn(), GET: vi.fn()}));

describe('single organization board', {concurrent: false}, () => {
  beforeEach(() => {
    vi.clearAllMocks();
    document.body.innerHTML = `<div data-org-kanban data-board-url="/acme/-/kanban" data-error="Failed" data-invalid="Invalid">
      <input data-board-search><select data-board-assignee><option value="0">All</option><option value="2">Alice</option></select>
      <datalist id="board-mentions"><option data-name="alice" value="@alice"></option><option data-name="bob" value="@bob"></option></datalist>
      <section data-board-column="todo"><span data-board-count></span><div data-board-cards>
        <article data-board-task="1" data-assignee="2" data-search="Fix BuildWorld @alice"></article>
        <article data-board-task="2" data-assignee="4" data-search="Write docs @bob"></article>
      </div><form action="/acme/-/kanban/tasks" class="org-kanban-quick" data-board-form><input name="title" data-board-title><input name="status" value="todo" type="hidden"><button>Add</button></form><p data-board-empty></p></section>
      <section data-board-column="done"><span data-board-count></span><div data-board-cards></div><p data-board-empty></p></section>
    </div>`;
  });

  test('search and assignee filters update counts and empty states', () => {
    const root = document.querySelector<HTMLElement>('[data-org-kanban]')!;
    filterBoardCards(root, 'BUILDWORLD', '2');
    const cards = root.querySelectorAll<HTMLElement>('[data-board-task]');
    expect(cards[0].hidden).toBe(false);
    expect(cards[1].hidden).toBe(true);
    expect(root.querySelector('[data-board-count]')!.textContent).toBe('1');
    filterBoardCards(root, '@bob', '0');
    expect(cards[0].hidden).toBe(true);
    expect(cards[1].hidden).toBe(false);
  });

  test('typing @ offers organization member completions while preserving task text', () => {
    initOrgKanban();
    const input = document.querySelector<HTMLInputElement>('[data-board-title]')!;
    input.value = 'Review release @a';
    input.dispatchEvent(new Event('input'));
    const options = document.querySelector<HTMLDataListElement>('#board-mentions')!.options;
    expect(Array.from(options, (option) => option.value)).toEqual(['Review release @alice', 'Review release @bob']);
  });

  test('quick add inserts the server-rendered card without leaving the board', async () => {
    vi.mocked(POST).mockResolvedValue(new Response('<article data-board-task="3" data-status="todo" data-search="New task @alice" data-assignee="2">New task</article>'));
    initOrgKanban();
    const input = document.querySelector<HTMLInputElement>('[data-board-title]')!;
    input.value = 'New task @alice';
    const form = input.form!;
    const currentURL = window.location.href;
    form.dispatchEvent(new Event('submit', {bubbles: true, cancelable: true}));
    await vi.waitFor(() => expect(document.querySelector('[data-board-task="3"]')).not.toBeNull());
    expect(input.value).toBe('');
    expect(window.location.href).toBe(currentURL);
    expect(POST).toHaveBeenCalledOnce();
    expect(vi.mocked(POST).mock.calls[0][1]!.headers).toEqual({'X-Requested-With': 'XMLHttpRequest'});
  });
});
