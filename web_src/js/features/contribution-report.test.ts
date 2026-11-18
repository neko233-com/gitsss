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
  expect(contributionRows(authors, 'month', '')[0]).toMatchObject({period: '2026-10', commits: 3, additions: 30, binary_files: 3, changed_lines: 36, net_lines: 24});
  expect(contributionRows(authors, 'month', 'missing')).toEqual([]);
});
test('groups by year, contributor identity, and visible name', () => {
  const sameName = [...authors, {id: 'b', name: '=Alice', ...totals, days: {'2026-10-05': {date: '2026-10-05', ...totals}}}];
  expect(contributionRows(authors, 'year', '')[0]).toMatchObject({period: '2026', commits: 3});
  const byAuthor = contributionRows(sameName, 'author', 'alice');
  expect(byAuthor).toHaveLength(2);
  expect(byAuthor[0]).toMatchObject({author: '=Alice (a)', period: 'all', commits: 3, changed_lines: 36});
  const byName = contributionRows(sameName, 'name', 'alice');
  expect(byName).toHaveLength(1);
  expect(byName[0]).toMatchObject({author: '=Alice', period: 'all', commits: 4, additions: 40, deletions: 8, changed_lines: 48, net_lines: 32});
});
test('CSV preserves quotes and prevents spreadsheet formula execution', () => {
  const csv = contributionCSV(contributionRows(authors, 'month', ''));
  expect(csv).toContain('"\'=Alice"');
  expect(csv).toContain('"30"');
  expect(csv).toContain('"changed_lines"');
  expect(csv).toContain('"net_lines"');
});
