import {softwareDate, softwareFileType, softwareMoveAllowed, softwareSizeBand} from './software-presentation.ts';

test('software dates use local fixed-width yyyy-MM-dd HH:mm:ss', () => {
  expect(softwareDate(new Date(2026, 9, 21, 4, 3, 2).getTime() / 1000)).toBe('2026-10-21 04:03:02');
  expect(softwareDate(NaN)).toBe('—');
});

test('size heat bands include exact binary thresholds', () => {
  expect([0, 10 * 2 ** 20 - 1, 10 * 2 ** 20, 100 * 2 ** 20 - 1, 100 * 2 ** 20, 2 ** 30, 10 * 2 ** 30].map(softwareSizeBand)).toEqual(['tiny', 'tiny', 'small', 'small', 'medium', 'large', 'huge']);
});

test('local file icons distinguish folders, installers, archives, images and code', () => {
  expect(['Setup.EXE', 'SDK.zip', 'cover.PNG', 'entry.ts', 'notes.pdf'].map((name) => softwareFileType(name).kind)).toEqual(['app', 'archive', 'image', 'code', 'file']);
  expect(softwareFileType('folder.exe', true).kind).toBe('folder');
});

test('drag destinations reject self, descendants, unknown targets and malformed trees', () => {
  const folders = [{id: 'a', parent: ''}, {id: 'b', parent: 'a'}, {id: 'c', parent: ''}];
  expect(softwareMoveAllowed(['a'], 'a', folders)).toBe(false);
  expect(softwareMoveAllowed(['a'], 'b', folders)).toBe(false);
  expect(softwareMoveAllowed(['b'], 'missing', folders)).toBe(false);
  expect(softwareMoveAllowed(['a', 'b'], 'c', folders)).toBe(true);
  expect(softwareMoveAllowed(['a'], '', folders)).toBe(true);
  expect(softwareMoveAllowed([], '', folders)).toBe(false);
  expect(softwareMoveAllowed(['file'], 'x', [{id: 'x', parent: 'y'}, {id: 'y', parent: 'x'}])).toBe(false);
});
