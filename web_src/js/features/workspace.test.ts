import {initWorkspace} from './workspace.ts';

test('workspace drawer closes on viewport change and preserves filtering input', () => {
  const desktop = {matches: false, addEventListener: vi.fn()};
  vi.stubGlobal('matchMedia', vi.fn(() => desktop));
  document.body.innerHTML = '<button data-workspace-nav-open></button><aside class="workspace-sidebar"><button data-workspace-nav-close></button><input data-workspace-org-filter><a href="/org" data-workspace-org data-search="engineering">Engineering</a></aside>';
  initWorkspace();
  const open = document.querySelector<HTMLButtonElement>('[data-workspace-nav-open]')!;
  const root = document.querySelector<HTMLElement>('aside')!;
  const input = document.querySelector('input')!;
  expect(root.inert).toBe(true);
  open.click();
  expect(root.inert).toBe(false);
  expect(open.getAttribute('aria-expanded')).toBe('true');
  expect(document.activeElement).toBe(root.querySelector('button'));
  input.value = 'engineering';
  root.dispatchEvent(new KeyboardEvent('keydown', {key: 'Escape', bubbles: true}));
  expect(root.inert).toBe(true);
  expect(document.activeElement).toBe(open);
  desktop.matches = true;
  desktop.addEventListener.mock.calls[0][1]();
  expect(root.inert).toBe(false);
  expect(input.value).toBe('engineering');
  vi.unstubAllGlobals();
  document.body.classList.remove('workspace-nav-ready', 'workspace-nav-open');
});

test('organization search matches the stable name and localized note', () => {
  document.body.innerHTML = '<aside class="workspace-sidebar"><input data-workspace-org-filter><a data-workspace-org data-search="engineering 研发">Engineering</a><p data-workspace-org-empty hidden></p></aside>';
  initWorkspace();
  const input = document.querySelector<HTMLInputElement>('input')!;
  input.value = '研发'; input.dispatchEvent(new Event('input'));
  expect(document.querySelector<HTMLElement>('[data-workspace-org]')!.hidden).toBe(false);
  input.value = 'missing'; input.dispatchEvent(new Event('input'));
  expect(document.querySelector<HTMLElement>('[data-workspace-org]')!.hidden).toBe(true);
  expect(document.querySelector<HTMLElement>('[data-workspace-org-empty]')!.hidden).toBe(false);
});
test('slash focuses search without stealing typing from an editor', () => {
  document.body.innerHTML = '<input data-workspace-search><textarea></textarea>';
  initWorkspace();
  document.body.dispatchEvent(new KeyboardEvent('keydown', {key: '/', bubbles: true}));
  expect(document.activeElement).toBe(document.querySelector('input'));
  const editor = document.querySelector('textarea')!; editor.focus();
  editor.dispatchEvent(new KeyboardEvent('keydown', {key: '/', bubbles: true}));
  expect(document.activeElement).toBe(editor);
});
