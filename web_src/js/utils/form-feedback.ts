export function attachSubmitFeedback(form: HTMLFormElement) {
  let submitting = false;
  const buttons = Array.from(form.querySelectorAll<HTMLButtonElement>('button[type=submit], button:not([type])'));
  const originalDisabled = new Map<HTMLButtonElement, boolean>();
  form.addEventListener('submit', (event) => {
    if (submitting) {
      event.preventDefault();
      return;
    }
    submitting = true;
    queueMicrotask(() => {
      if (event.defaultPrevented) {
        submitting = false;
        return;
      }
      form.setAttribute('aria-busy', 'true');
      for (const button of buttons) {
        originalDisabled.set(button, button.disabled);
        button.disabled = true;
        button.classList.add('loading');
      }
    });
  });
  window.addEventListener('pageshow', () => {
    if (!submitting) return;
    submitting = false;
    form.removeAttribute('aria-busy');
    for (const button of buttons) {
      button.disabled = originalDisabled.get(button)!;
      button.classList.remove('loading');
    }
  });
  for (const input of form.querySelectorAll<HTMLInputElement>('.field.error input, .field.error textarea')) {
    input.setAttribute('aria-invalid', 'true');
  }
}
