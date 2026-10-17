import {initWorkspace} from './workspace.ts';

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
