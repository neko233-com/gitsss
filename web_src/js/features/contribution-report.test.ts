import {contributionCSV, contributionRows} from './contribution-report.ts';

const totals = {commits: 1, additions: 10, deletions: 2, files: 3, binary_files: 1};
const authors = [{id: 'a', name: '=Alice', ...totals, days: {
  '2026-10-04': {date: '2026-10-04', ...totals},
  '2026-10-05': {date: '2026-10-05', ...totals},
  '2026-10-06': {date: '2026-10-06', ...totals},
}}];
test('groups days into Monday weeks and months without fetching again', () => {
  expect(contributionRows(authors, 'day', '')).toHaveLength(3);
  const weeks = contributionRows(authors, 'week', 'alice');
  expect(weeks.map((row) => [row.period, row.commits])).toEqual([['2026-10-05', 2], ['2026-09-28', 1]]);
  expect(contributionRows(authors, 'month', '')[0]).toMatchObject({period: '2026-10', commits: 3, additions: 30, binary_files: 3});
  expect(contributionRows(authors, 'month', 'missing')).toEqual([]);
});
test('CSV preserves quotes and prevents spreadsheet formula execution', () => {
  const csv = contributionCSV(contributionRows(authors, 'month', ''));
  expect(csv).toContain('"\'=Alice"');
  expect(csv).toContain('"30"');
});
