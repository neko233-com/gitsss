import {hideElem, showElem} from '../utils/dom.ts';
import {localUserSettings} from '../modules/user-settings.ts';

export function initUserSettings() {
  if (!document.querySelector('.user.settings.profile')) return;
  initAvatarStylePicker();

  const usernameInput = document.querySelector<HTMLInputElement>('#username');
  if (!usernameInput) return;
  usernameInput.addEventListener('input', function () {
    const prompt = document.querySelector('#name-change-prompt')!;
    const promptRedirect = document.querySelector('#name-change-redirect-prompt')!;
    if (this.value.toLowerCase() !== this.getAttribute('data-name')!.toLowerCase()) {
      showElem(prompt);
      showElem(promptRedirect);
    } else {
      hideElem(prompt);
      hideElem(promptRedirect);
    }
  });
}

export function initAvatarStylePicker() {
  const picker = document.querySelector<HTMLElement>('[data-avatar-picker]');
  if (!picker) return;
  const tabs = Array.from(picker.querySelectorAll<HTMLButtonElement>('[data-avatar-style]'));
  const panels = Array.from(picker.querySelectorAll<HTMLElement>('[data-avatar-panel]'));
  if (tabs.length === 0) return;
  function activate(tab: HTMLButtonElement, focus = false) {
    localUserSettings.setString('avatar-style', tab.getAttribute('data-avatar-style')!);
    for (const item of tabs) {
      const selected = item === tab;
      item.setAttribute('aria-selected', String(selected));
      item.tabIndex = selected ? 0 : -1;
      item.classList.toggle('active', selected);
    }
    for (const panel of panels) panel.hidden = panel.getAttribute('data-avatar-panel') !== tab.getAttribute('data-avatar-style');
    if (focus) tab.focus();
  }
  for (const [index, tab] of tabs.entries()) {
    tab.addEventListener('click', () => activate(tab));
    tab.addEventListener('keydown', (event) => {
      let next: number;
      if (event.key === 'ArrowRight') next = (index + 1) % tabs.length;
      else if (event.key === 'ArrowLeft') next = (index - 1 + tabs.length) % tabs.length;
      else if (event.key === 'Home') next = 0;
      else if (event.key === 'End') next = tabs.length - 1;
      else return;
      event.preventDefault();
      activate(tabs[next], true);
    });
  }
  const selected = localUserSettings.getString('avatar-style');
  activate(tabs.find((tab) => tab.getAttribute('data-avatar-style') === selected) || tabs[0]);
}
