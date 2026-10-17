import {initAvatarStylePicker} from './user-settings.ts';

test('avatar style tabs switch panels with click and keyboard', () => {
  document.body.innerHTML = `<div data-avatar-picker>
    <button role="tab" data-avatar-style="anime">Anime</button>
    <button role="tab" data-avatar-style="custom">Custom</button>
    <section data-avatar-panel="anime"></section>
    <section data-avatar-panel="custom"></section>
  </div>`;
  initAvatarStylePicker();
  const tabs = document.querySelectorAll<HTMLButtonElement>('[role="tab"]');
  const anime = document.querySelector<HTMLElement>('[data-avatar-panel="anime"]')!;
  const custom = document.querySelector<HTMLElement>('[data-avatar-panel="custom"]')!;
  expect(anime.hidden).toBe(false);
  expect(custom.hidden).toBe(true);
  tabs[1].click();
  expect(tabs[1].getAttribute('aria-selected')).toBe('true');
  expect(custom.hidden).toBe(false);
  expect(anime.hidden).toBe(true);
  tabs[1].dispatchEvent(new KeyboardEvent('keydown', {key: 'Home'}));
  expect(document.activeElement).toBe(tabs[0]);
  expect(anime.hidden).toBe(false);
  tabs[0].dispatchEvent(new KeyboardEvent('keydown', {key: 'ArrowLeft'}));
  expect(document.activeElement).toBe(tabs[1]);
});
