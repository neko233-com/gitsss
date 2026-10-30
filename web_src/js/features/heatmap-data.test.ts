import {defaultHeatmapBounds, githubDarkHeatmapColors, githubLightHeatmapColors, heatmapData, isLegacyDefaultHeatmap, validHeatmapBounds} from './heatmap-data.ts';

test('GitHub palettes use four distinct levels and only untouched legacy defaults migrate', () => {
  expect(githubLightHeatmapColors).toEqual(['#aceebb', '#4ac26b', '#2da44e', '#116329']);
  expect(githubDarkHeatmapColors).toEqual(['#033a16', '#196c2e', '#2ea043', '#56d364']);
  const oldBounds = [1, 5, 10, 20, 50];
  const oldColors = ['#183b60', '#245b88', '#317eb4', '#55a9df', '#a5dfff'];
  expect(isLegacyDefaultHeatmap(oldBounds, oldColors)).toBe(true);
  expect(isLegacyDefaultHeatmap(oldBounds, ['#123456', ...oldColors.slice(1)])).toBe(false);
  expect(isLegacyDefaultHeatmap([1, 4, 10, 20, 50], oldColors)).toBe(false);
});

test('heatmap has exactly 365 local calendar days and excludes padding/future events', () => {
  const now = new Date(2026, 9, 18, 12);
  const result = heatmapData([
    {date: now, count: 3}, {date: new Date(2026, 9, 18, 1), count: 2},
    {date: new Date(2026, 9, 12), count: 10}, {date: new Date(2026, 8, 19), count: 20},
    {date: new Date(2026, 8, 18), count: 50}, {date: new Date(2025, 9, 18), count: 99},
    {date: new Date(2026, 9, 19), count: 100}, {date: now, count: -5}, {date: now, count: NaN},
  ], now);
  expect(result.weeks.flat().filter((day) => day.visible)).toHaveLength(365);
  expect(result.summary).toEqual({total: 85, recent7: 15, recent30: 35, active: 4, peak: 50});
  expect(result.changes).toEqual({total: -94, recent7: 5, recent30: -45, active: 0, peak: -49});
  expect(result.weeks.flat().find((day) => day.key === '2026-10-18')!.level).toBe(2);
  expect(result.weeks.flat().find((day) => day.key === '2026-09-18')!.level).toBe(4);
});

test('range boundaries are explicit, inclusive and reject unsafe preferences', () => {
  expect(validHeatmapBounds(defaultHeatmapBounds)).toBe(true);
  for (const values of [null, [], [1, 5, 5, 20, 50], [0, 5, 10, 20, 50], [1, 2.5, 5, 10, 20], [1, 5, 10, 20, Infinity]]) expect(validHeatmapBounds(values)).toBe(false);
  const now = new Date(2026, 2, 9);
  expect(heatmapData([{date: now, count: 50}], now).summary.recent7).toBe(50);
  expect(heatmapData([], now).summary).toEqual({total: 0, recent7: 0, recent30: 0, active: 0, peak: 0});
  expect(heatmapData([], now).changes).toEqual({total: 0, recent7: 0, recent30: 0, active: 0, peak: 0});
  expect(heatmapData([{date: now, count: 50}], now).changes).toEqual({total: 50, recent7: 50, recent30: 50, active: 1, peak: 50});
});
