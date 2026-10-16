import {initOrgDirectory} from './org-directory.ts';

describe('organization directory', () => {
  beforeEach(() => {
    document.body.innerHTML = '<input data-org-directory-search><div data-org-directory-item data-search="engineering 研发团队"></div><div data-org-directory-item data-search="design 设计团队"></div><p data-org-directory-empty hidden>No results</p>';
    initOrgDirectory();
  });

  test.each(['ENGINEERING', '研发', 'ｅｎｇｉｎｅｅｒｉｎｇ'])('searches identifier and display note: %s', (query) => {
    const input = document.querySelector<HTMLInputElement>('input')!;
    input.value = query;
    input.dispatchEvent(new Event('input'));
    expect(Array.from(document.querySelectorAll<HTMLElement>('[data-org-directory-item]'), (card) => card.hidden)).toEqual([false, true]);
  });

  test('shows empty results and Escape restores every organization', () => {
    const input = document.querySelector<HTMLInputElement>('input')!;
    const empty = document.querySelector<HTMLElement>('[data-org-directory-empty]')!;
    input.value = 'missing';
    input.dispatchEvent(new Event('input'));
    expect(empty.hidden).toBe(false);
    input.dispatchEvent(new KeyboardEvent('keydown', {key: 'Escape'}));
    expect(input.value).toBe('');
    expect(empty.hidden).toBe(true);
    expect(Array.from(document.querySelectorAll<HTMLElement>('[data-org-directory-item]')).every((card) => !card.hidden)).toBe(true);
  });
});
