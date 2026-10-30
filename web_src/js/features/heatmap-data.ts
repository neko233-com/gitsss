export type HeatmapValue = {date: Date; count: number};
export const defaultHeatmapBounds = [1, 5, 10, 20, 50];
export const legacyHeatmapColors = ['#dbeafe', '#93c5fd', '#3b82f6', '#1d4ed8', '#1e3a8a'];
export const defaultHeatmapColors = ['#183b60', '#245b88', '#317eb4', '#55a9df', '#a5dfff'];

export function migrateHeatmapColors(colors: string[]): string[] {
  return colors.every((color, index) => color.toLowerCase() === legacyHeatmapColors[index]) ? Array.from(defaultHeatmapColors) : colors;
}

export function validHeatmapBounds(values: unknown): values is number[] {
  return Array.isArray(values) && values.length === 5 && values[0] === 1 &&
    values.every((value, i) => Number.isSafeInteger(value) && value <= 1_000_000 && (i === 0 || value > values[i - 1]));
}

export function localDateKey(date: Date): string {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
}

export function heatmapData(values: HeatmapValue[], now: Date, bounds = defaultHeatmapBounds) {
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const start = new Date(today); start.setDate(start.getDate() - 364);
  const counts = new Map<string, number>();
  for (const {date, count} of values) {
    if (!Number.isFinite(date.getTime()) || !Number.isSafeInteger(count) || count < 0) continue;
    const key = localDateKey(date); counts.set(key, (counts.get(key) || 0) + count);
  }
  const cursor = new Date(start); cursor.setDate(cursor.getDate() - cursor.getDay());
  const weeks: Array<Array<{date: Date; key: string; count: number; level: number; visible: boolean}>> = [];
  const summary = {total: 0, recent7: 0, recent30: 0, active: 0, peak: 0};
  for (let w = 0; w < Math.ceil((365 + start.getDay()) / 7); w++) {
    const week = [];
    for (let i = 0; i < 7; i++) {
      const date = new Date(cursor); const key = localDateKey(date); const visible = date >= start && date <= today;
      const count = visible ? counts.get(key) || 0 : 0;
      const level = count ? bounds.filter((bound) => count >= bound).length : 0;
      week.push({date, key, count, level, visible});
      if (visible) {
        summary.total += count; summary.active += count > 0 ? 1 : 0; summary.peak = Math.max(summary.peak, count);
        const age = Math.round((Date.UTC(today.getFullYear(), today.getMonth(), today.getDate()) - Date.UTC(date.getFullYear(), date.getMonth(), date.getDate())) / 86400000);
        if (age < 7) summary.recent7 += count;
        if (age < 30) summary.recent30 += count;
      }
      cursor.setDate(cursor.getDate() + 1);
    }
    weeks.push(week);
  }
  const months = [{month: start.getMonth(), week: 0}];
  for (let w = 1; w < weeks.length - 1; w++) {
    if (weeks[w][0].date.getMonth() !== weeks[w - 1][0].date.getMonth()) months.push({month: weeks[w][0].date.getMonth(), week: w});
  }
  return {weeks, months, summary};
}
