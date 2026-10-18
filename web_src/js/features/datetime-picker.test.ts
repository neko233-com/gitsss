import {validDateValue} from './datetime-picker.ts';

test('date controls preserve ISO validity, constraints and optional clearing', () => {
  const input = document.createElement('input');
  input.min = '2026-01-01'; input.max = '2026-12-31';
  expect(validDateValue(input, 'date', '2026-10-18')).toBe(true);
  expect(validDateValue(input, 'date', '2026-02-30')).toBe(false);
  expect(validDateValue(input, 'date', '2027-01-01')).toBe(false);
  expect(validDateValue(input, 'date', '')).toBe(true);
  input.required = true;
  expect(validDateValue(input, 'date', '')).toBe(false);
});

test('datetime values retain local wall clock and minute precision', () => {
  const input = document.createElement('input');
  expect(validDateValue(input, 'datetime-local', '2026-10-18T14:35')).toBe(true);
  expect(validDateValue(input, 'datetime-local', '2026-10-18T14:35Z')).toBe(false);
  expect(validDateValue(input, 'datetime-local', '2026-10-18T25:35')).toBe(false);
});
