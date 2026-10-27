import {initRepoSettingsSearchTeamBox} from './repo-settings.ts';

describe('available repository teams', () => {
  it('filters names without hiding attached teams and restores them on clear', () => {
    document.body.innerHTML = `<div data-team-name="already attached"></div>
      <input id="available-team-filter"><div id="available-team-list">
        <div data-team-name="Server Developer"></div><div data-team-name="Client"></div>
      </div><p id="available-team-no-results" class="tw-hidden"></p>`;
    initRepoSettingsSearchTeamBox();
    const input = document.querySelector<HTMLInputElement>('#available-team-filter')!;
    const rows = document.querySelectorAll<HTMLElement>('#available-team-list [data-team-name]');
    input.value = 'SERVER';
    input.dispatchEvent(new Event('input'));
    expect(rows[0].classList.contains('tw-hidden')).toBe(false);
    expect(rows[1].classList.contains('tw-hidden')).toBe(true);
    expect(document.querySelector('[data-team-name="already attached"]')!.classList.contains('tw-hidden')).toBe(false);
    input.value = 'missing';
    input.dispatchEvent(new Event('input'));
    expect(document.querySelector('#available-team-no-results')!.classList.contains('tw-hidden')).toBe(false);
    input.value = '';
    input.dispatchEvent(new Event('input'));
    expect(Array.from(rows).every((row) => !row.classList.contains('tw-hidden'))).toBe(true);
  });
});
