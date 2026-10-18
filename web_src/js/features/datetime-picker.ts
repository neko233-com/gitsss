import type {Instance} from 'flatpickr/dist/types/instance.js';

let pickerModules: Promise<any> | undefined;

export function validDateValue(input: HTMLInputElement, type: string, value = input.value): boolean {
  if (!value) return !input.required;
  if ((input.min && value < input.min) || (input.max && value > input.max)) return false;
  const validator = document.createElement('input');
  validator.type = type;
  validator.min = input.min; validator.max = input.max; validator.step = input.step;
  validator.required = input.required; validator.value = value;
  return validator.value === value && validator.checkValidity();
}

export async function enhanceDateInput(input: HTMLInputElement): Promise<void> {
  if (input.getAttribute('data-picker-type') || input.disabled || input.readOnly) return;
  const type = input.type;
  if (!['date', 'datetime-local'].includes(type)) return;
  input.setAttribute('data-picker-type', type);
  try {
    pickerModules ||= Promise.all([import('flatpickr'), import('flatpickr/dist/l10n/index.js'), import('flatpickr/dist/flatpickr.css')]);
    const [{default: flatpickr}, {default: locales}] = await pickerModules;
    if (!input.isConnected) return;
    const words = (key: string) => document.querySelector<HTMLElement>('#datetime-labels')!.getAttribute(`data-${key}`)!;
    const language = document.documentElement.lang.toLowerCase();
    const localeKey = language.startsWith('zh') ? (language.includes('tw') ? 'zh_tw' : 'zh') : language.split('-')[0];
    const hasTime = type === 'datetime-local';
    let raw = input.value;
    const validate = () => input.setCustomValidity(validDateValue(input, type) ? '' : words('invalid'));
    const instance: Instance = flatpickr(input, {
      locale: locales[localeKey] || 'default', enableTime: hasTime, time_24hr: true,
      dateFormat: hasTime ? 'Y-m-d\\TH:i' : 'Y-m-d', ariaDateFormat: 'Y F j',
      allowInput: true, disableMobile: true, animate: false, minuteIncrement: 1,
      minDate: input.min || undefined, maxDate: input.max || undefined,
      onDayCreate: (_dates: Date[], _value: string, _instance: Instance, day: HTMLElement) => day.setAttribute('role', 'button'),
      onChange: () => {raw = input.value; validate()},
      onClose: () => {
        if (raw && !validDateValue(input, type, raw)) input.value = raw;
        validate();
      },
    });
    input.addEventListener('input', () => {raw = input.value; validate()});
    input.addEventListener('change', validate);
    const footer = document.createElement('div'); footer.className = 'datetime-picker-footer';
    const today = document.createElement('button'); today.type = 'button'; today.textContent = words('today');
    today.addEventListener('click', () => {instance.setDate(new Date(), true); instance.close(); input.focus()});
    const clear = document.createElement('button'); clear.type = 'button'; clear.textContent = words('clear');
    clear.disabled = input.required;
    clear.addEventListener('click', () => {instance.clear(); raw = ''; validate(); instance.close(); input.focus()});
    footer.append(today, clear);
    instance.calendarContainer.append(footer);
    if (hasTime) {
      const timezone = document.createElement('small'); timezone.textContent = `${words('timezone')} · ${new Intl.DateTimeFormat().resolvedOptions().timeZone}`;
      footer.append(timezone);
    }
    input.form?.addEventListener('reset', () => setTimeout(() => {instance.setDate(input.defaultValue, false); raw = input.value; validate()}, 0));
    validate();
  } catch (error) {
    input.type = type; input.removeAttribute('data-picker-type');
    console.error('Date picker could not load', error);
  }
}

export function initDatePickers(): void {
  for (const input of document.querySelectorAll<HTMLInputElement>('input[type="date"], input[type="datetime-local"]')) enhanceDateInput(input);
  document.addEventListener('focusin', (event) => {
    const input = event.target as HTMLInputElement;
    if (input.matches('input[type="date"], input[type="datetime-local"]')) enhanceDateInput(input);
  });
}
