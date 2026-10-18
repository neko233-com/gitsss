import './relative-time.ts';

function createRelativeTime(datetime: string, attrs: Record<string, string> = {}): HTMLElement {
  const el = document.createElement('relative-time');
  el.setAttribute('lang', 'en');
  el.setAttribute('datetime', datetime);
  for (const [k, v] of Object.entries(attrs)) el.setAttribute(k, v);
  return el;
}

function getText(el: HTMLElement): string {
  return el.shadowRoot!.textContent ?? '';
}

test('renders "now" for current time', async () => {
  const el = createRelativeTime(new Date().toISOString());
  await Promise.resolve();
  expect(getText(el)).toBe('now');
});

test('renders minutes ago', async () => {
  const el = createRelativeTime(new Date(Date.now() - 3 * 60 * 1000).toISOString());
  await Promise.resolve();
  expect(getText(el)).toBe('3 minutes ago');
});

test('renders hours ago', async () => {
  const el = createRelativeTime(new Date(Date.now() - 3 * 60 * 60 * 1000).toISOString());
  await Promise.resolve();
  expect(getText(el)).toBe('3 hours ago');
});

test('renders yesterday', async () => {
  const el = createRelativeTime(new Date(Date.now() - 24 * 60 * 60 * 1000).toISOString());
  await Promise.resolve();
  expect(getText(el)).toBe('yesterday');
});

test('renders days ago', async () => {
  const el = createRelativeTime(new Date(Date.now() - 3 * 24 * 60 * 60 * 1000).toISOString());
  await Promise.resolve();
  expect(getText(el)).toBe('3 days ago');
});

test('renders future time', async () => {
  const el = createRelativeTime(new Date(Date.now() + 3 * 24 * 60 * 60 * 1000).toISOString());
  await Promise.resolve();
  expect(getText(el)).toBe('in 3 days');
});

test('switches to datetime format after default threshold', async () => {
  const el = createRelativeTime(new Date(Date.now() - 32 * 24 * 60 * 60 * 1000).toISOString(), {lang: 'en-US'});
  await Promise.resolve();
  expect(getText(el)).toMatch(/on [A-Z][a-z]{2} \d{1,2}/);
});

test('accepts unix seconds as integer string', async () => {
  const el = createRelativeTime(String(Math.floor(Date.now() / 1000) - 3 * 60));
  await Promise.resolve();
  expect(getText(el)).toBe('3 minutes ago');
});

test('ignores fractional unix seconds', async () => {
  const el = createRelativeTime('1700000000.5');
  el.shadowRoot!.textContent = 'fallback';
  await Promise.resolve();
  expect(getText(el)).toBe('fallback');
});

test('ignores negative unix seconds', async () => {
  const el = createRelativeTime('-86400');
  el.shadowRoot!.textContent = 'fallback';
  await Promise.resolve();
  expect(getText(el)).toBe('fallback');
});

test('ignores invalid datetime', async () => {
  const el = createRelativeTime('bogus');
  el.shadowRoot!.textContent = 'fallback';
  await Promise.resolve();
  expect(getText(el)).toBe('fallback');
});

test('ignores partial numeric datetime', async () => {
  const el = createRelativeTime('123abc');
  el.shadowRoot!.textContent = 'fallback';
  await Promise.resolve();
  expect(getText(el)).toBe('fallback');
});

test('handles empty datetime', async () => {
  const el = createRelativeTime('');
  el.shadowRoot!.textContent = 'fallback';
  await Promise.resolve();
  expect(getText(el)).toBe('fallback');
});

test('tense=past shows relative time beyond threshold', async () => {
  const el = createRelativeTime(new Date(Date.now() - 60 * 24 * 60 * 60 * 1000).toISOString(), {tense: 'past'});
  await Promise.resolve();
  expect(getText(el)).toMatch(/months? ago/);
});

test('tense=past clamps future to now', async () => {
  const el = createRelativeTime(new Date(Date.now() + 3000).toISOString(), {tense: 'past'});
  await Promise.resolve();
  expect(getText(el)).toBe('now');
});

test('format=duration renders duration', async () => {
  const el = createRelativeTime(new Date(Date.now() - 3 * 60 * 60 * 1000).toISOString(), {format: 'duration'});
  await Promise.resolve();
  expect(getText(el)).toMatch(/hours?/);
});

test('format=datetime renders formatted date', async () => {
  const el = createRelativeTime(new Date().toISOString(), {format: 'datetime', lang: 'en-US'});
  await Promise.resolve();
  expect(getText(el)).toMatch(/[A-Z][a-z]{2}, [A-Z][a-z]{2} \d{1,2}/);
});

test('sets data-tooltip-content', async () => {
  const el = createRelativeTime(new Date().toISOString());
  await Promise.resolve();
  expect(el.getAttribute('data-tooltip-content')).toBeTruthy();
  expect(el.getAttribute('aria-label')).toBe(el.getAttribute('data-tooltip-content'));
});

test('respects lang from parent element', async () => {
  const container = document.createElement('span');
  container.setAttribute('lang', 'de');
  const el = document.createElement('relative-time');
  el.setAttribute('datetime', new Date(Date.now() - 3 * 24 * 60 * 60 * 1000).toISOString());
  container.append(el);
  await Promise.resolve();
  expect(getText(el)).toBe('vor 3 Tagen');
});

test('falls back when navigator.language is invalid', async () => {
  vi.spyOn(navigator, 'language', 'get').mockReturnValue('undefined');
  try {
    const el = document.createElement('relative-time');
    el.setAttribute('datetime', new Date(Date.now() - 3 * 60 * 1000).toISOString());
    await Promise.resolve();
    expect(getText(el)).toBe('3 minutes ago');
  } finally {
    vi.restoreAllMocks();
  }
});

test('switches to datetime with P1D threshold', async () => {
  const el = createRelativeTime(new Date(Date.now() - 2 * 24 * 60 * 60 * 1000).toISOString(), {
    lang: 'en-US',
    threshold: 'P1D',
  });
  await Promise.resolve();
  expect(getText(el)).toMatch(/on [A-Z][a-z]{2} \d{1,2}/);
});

test('batches multiple attribute changes into single update', async () => {
  const el = document.createElement('relative-time');
  el.setAttribute('lang', 'en');
  el.setAttribute('datetime', new Date().toISOString());
  await Promise.resolve();
  expect(getText(el)).toBe('now');

  let updateCount = 0;
  const origUpdate = (el as any).update;
  (el as any).update = function () {
    updateCount++;
    return origUpdate.call(this);
  };
  el.setAttribute('second', '2-digit');
  el.setAttribute('hour', '2-digit');
  el.setAttribute('minute', '2-digit');
  await Promise.resolve();
  expect(updateCount).toBe(1);
});

test('activity combines exact local time, weekday and a calendar-day relative label', {concurrent: false}, async () => {
  vi.useFakeTimers();
  try {
    vi.setSystemTime(new Date(2026, 9, 18, 0, 1));
    const el = createRelativeTime(new Date(2026, 9, 16, 23, 59, 5).toISOString(), {format: 'activity', lang: 'zh-CN'});
    await Promise.resolve();
    expect(getText(el)).toBe('2026-10-16 23:59:05 周五 | 前天');
    expect(el.getAttribute('aria-label')).toContain(getText(el));
    el.setAttribute('lang', 'en-US');
    await Promise.resolve();
    expect(getText(el)).toBe('2026-10-16 23:59:05 Fri | 2 days ago');
    el.setAttribute('datetime', new Date(2026, 9, 19, 0, 0, 1).toISOString());
    await Promise.resolve();
    expect(getText(el)).toBe('2026-10-19 00:00:01 Mon | tomorrow');
  } finally {
    vi.useRealTimers();
  }
});

test('activity remains relative for older timestamps and updates across midnight', {concurrent: false}, async () => {
  vi.useFakeTimers();
  const el = createRelativeTime(new Date(2026, 9, 16, 12, 34, 56).toISOString(), {format: 'activity', lang: 'zh-CN'});
  try {
    vi.setSystemTime(new Date(2026, 9, 17, 23, 59, 40));
    document.body.append(el);
    await Promise.resolve();
    expect(getText(el)).toBe('2026-10-16 12:34:56 周五 | 昨天');
    vi.setSystemTime(new Date(2026, 9, 18, 0, 0, 40));
    (el as any).update();
    expect(getText(el)).toBe('2026-10-16 12:34:56 周五 | 前天');
    vi.setSystemTime(new Date(2027, 9, 18, 12));
    (el as any).update();
    expect(getText(el)).toBe('2026-10-16 12:34:56 周五 | 去年');
  } finally {
    el.remove();
    vi.useRealTimers();
  }
});

test('past seconds outside the now window remain visible', async () => {
  const el = createRelativeTime(new Date(Date.now() - 50_000).toISOString());
  await Promise.resolve();
  expect(getText(el)).toBe('50 seconds ago');
});
