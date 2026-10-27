import {initCompSearchUserBox} from './SearchUserBox.ts';
import {GET} from '../../modules/fetch.ts';

vi.mock('../../modules/fetch.ts', () => ({GET: vi.fn()}));

describe('collaborator user search', {concurrent: false}, () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.mocked(GET).mockReset();
    document.body.innerHTML = '<div id="search-user-box" data-open-on-focus="true" data-empty-text="No users" data-error-text="Search failed"><input class="prompt"></div>';
  });

  afterEach(() => {
    vi.useRealTimers();
    document.body.replaceChildren();
  });

  it('opens candidates on focus, searches one character and selects with the keyboard', async () => {
    vi.mocked(GET).mockImplementation(async () => Response.json({data: [{login: 'fixture', full_name: 'Example', avatar_url: ''}]}));
    initCompSearchUserBox();
    const input = document.querySelector<HTMLInputElement>('#search-user-box input')!;
    input.focus();
    await vi.advanceTimersByTimeAsync(250);
    expect(GET).toHaveBeenCalledTimes(1);
    expect(document.querySelector('.results')!.textContent).toContain('Example');
    expect(input.getAttribute('aria-expanded')).toBe('true');
    input.value = 'f';
    input.dispatchEvent(new Event('input'));
    await vi.advanceTimersByTimeAsync(250);
    expect(GET).toHaveBeenCalledTimes(2);
    input.dispatchEvent(new KeyboardEvent('keydown', {key: 'ArrowDown', bubbles: true}));
    input.dispatchEvent(new KeyboardEvent('keydown', {key: 'Enter', bubbles: true}));
    expect(input.value).toBe('fixture');
    expect(input.getAttribute('aria-expanded')).toBe('false');
    input.dispatchEvent(new MouseEvent('click', {bubbles: true}));
    await vi.advanceTimersByTimeAsync(250);
    expect(GET).toHaveBeenCalledTimes(3);
    expect(input.getAttribute('aria-expanded')).toBe('true');
  });

  it('shows empty and failed searches', async () => {
    vi.mocked(GET).mockResolvedValueOnce(Response.json({data: []})).mockResolvedValueOnce(new Response('', {status: 500}));
    initCompSearchUserBox();
    const input = document.querySelector<HTMLInputElement>('#search-user-box input')!;
    input.focus();
    await vi.advanceTimersByTimeAsync(250);
    expect(document.querySelector('.results')!.textContent).toContain('No users');
    input.value = 'z';
    input.dispatchEvent(new Event('input'));
    await vi.advanceTimersByTimeAsync(250);
    expect(document.querySelector('.results')!.textContent).toContain('Search failed');
  });

  it('opens again after dismissal and blur', async () => {
    vi.mocked(GET).mockImplementation(async () => Response.json({data: [{login: 'fixture', full_name: 'Example', avatar_url: ''}]}));
    initCompSearchUserBox();
    const input = document.querySelector<HTMLInputElement>('#search-user-box input')!;
    input.focus();
    await vi.advanceTimersByTimeAsync(250);
    input.dispatchEvent(new KeyboardEvent('keydown', {key: 'Escape', bubbles: true}));
    input.click();
    await vi.advanceTimersByTimeAsync(250);
    expect(input.getAttribute('aria-expanded')).toBe('true');
    input.blur();
    await vi.advanceTimersByTimeAsync(200);
    input.focus();
    await vi.advanceTimersByTimeAsync(250);
    expect(input.getAttribute('aria-expanded')).toBe('true');
    expect(GET).toHaveBeenCalledTimes(3);
  });
});
