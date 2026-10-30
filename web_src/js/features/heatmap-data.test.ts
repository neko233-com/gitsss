import {defaultHeatmapBounds, defaultHeatmapColors, heatmapData, legacyHeatmapColors, migrateHeatmapColors, validHeatmapBounds} from './heatmap-data.ts';

test('default colors become progressively brighter and preserve custom preferences', () => {
  const luminance = (hex: string) => {
    const [r, g, b] = hex.slice(1).match(/../g)!.map((c) => parseInt(c, 16) / 255).map((c) => c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4);
    return 0.2126 * r + 0.7152 * g + 0.0722 * b;
  };
  expect(defaultHeatmapColors.map(luminance)).toEqual(defaultHeatmapColors.map(luminance).toSorted((a, b) => a - b));
  expect(migrateHeatmapColors(legacyHeatmapColors)).toEqual(defaultHeatmapColors);
  const custom = ['#123456', ...legacyHeatmapColors.slice(1)];
  expect(migrateHeatmapColors(custom)).toEqual(custom);
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
  expect(result.weeks.flat().find((day) => day.key === '2026-09-18')!.level).toBe(5);
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
