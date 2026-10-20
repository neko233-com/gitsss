import {attachSubmitFeedback} from './form-feedback.ts';

test('submit feedback prevents duplicates, retains payload and restores disabled state', async () => {
  const form = document.createElement('form');
  form.innerHTML = '<input name="_csrf" value="unchanged"><input name="uid" value="7"><button type="submit">Save</button><button type="submit" disabled>Unavailable</button>';
  const buttons = form.querySelectorAll('button');
  attachSubmitFeedback(form);
  expect(form.dispatchEvent(new Event('submit', {cancelable: true}))).toBe(true);
  expect(form.dispatchEvent(new Event('submit', {cancelable: true}))).toBe(false);
  await Promise.resolve();
  expect(form.getAttribute('aria-busy')).toBe('true');
  expect(buttons[0].disabled).toBe(true);
  expect(new FormData(form).get('uid')).toBe('7');
  expect(new FormData(form).get('_csrf')).toBe('unchanged');
  window.dispatchEvent(new Event('pageshow'));
  expect(form.hasAttribute('aria-busy')).toBe(false);
  expect(buttons[0].disabled).toBe(false);
  expect(buttons[1].disabled).toBe(true);
});

test('a cancelled submit does not lock the form', async () => {
  const form = document.createElement('form');
  form.innerHTML = '<button type="submit">Save</button><div class="field error"><input name="repo_name"></div>';
  attachSubmitFeedback(form);
  form.addEventListener('submit', (event) => event.preventDefault());
  form.dispatchEvent(new Event('submit', {cancelable: true}));
  await Promise.resolve();
  expect(form.hasAttribute('aria-busy')).toBe(false);
  expect(form.querySelector('button')!.disabled).toBe(false);
  expect(form.querySelector('input')!.getAttribute('aria-invalid')).toBe('true');
});
