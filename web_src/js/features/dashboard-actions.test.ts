import {initDashboardActions} from './dashboard-actions.ts';

describe('dashboard Actions worker panel', () => {
  beforeEach(() => {
    document.body.innerHTML = `
      <section data-dashboard-actions>
        <input data-dashboard-actions-search>
        <span data-dashboard-actions-result></span>
        <a data-dashboard-action-item data-search="buildworld runner-a engineering"></a>
        <a data-dashboard-action-item data-search="package runner-b server"></a>
      </section>
    `;
  });

  test('filters active jobs and reports the visible count', () => {
    initDashboardActions();
    const search = document.querySelector<HTMLInputElement>('[data-dashboard-actions-search]')!;
    const items = Array.from(document.querySelectorAll<HTMLElement>('[data-dashboard-action-item]'));
    const result = document.querySelector<HTMLElement>('[data-dashboard-actions-result]')!;

    expect(result.textContent).toBe('2/2');
    search.value = 'server';
    search.dispatchEvent(new Event('input'));

    expect(items[0].hidden).toBe(true);
    expect(items[1].hidden).toBe(false);
    expect(result.textContent).toBe('1/2');
  });
});
