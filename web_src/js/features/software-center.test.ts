import {folderChildren, softwareListParams, softwareOrganizationPage, softwareOrganizationPageCount} from './software-center.ts';

test('folder tree indexes children once and preserves parent relationships', () => {
  const base = {name: 'Folder', directory: true, size: 0, modified: 0};
  const result = folderChildren([{...base, id: 'a', parent: ''}, {...base, id: 'b', parent: 'a'}, {...base, id: 'c', parent: 'a'}]);
  expect(result.get('')!.map((node) => node.id)).toEqual(['a']);
  expect(result.get('a')!.map((node) => node.id)).toEqual(['b', 'c']);
  expect(result.has('b')).toBe(false);
});

test('software list query keeps the selected platform with the search and location', () => {
  const params = softwareListParams('folder-1', 'release', 3, 'Windows');
  expect(Array.from(params)).toEqual([['parent', 'folder-1'], ['q', 'release'], ['page', '3'], ['platform', 'Windows']]);
});

test('organization preview pages stay bounded and cover the whole plan', () => {
  const moves = Array.from({length: 123}, (_, index) => ({id: String(index), from: `old/${index}`, to: `new/${index}`, platform: 'Windows'}));
  expect(softwareOrganizationPageCount(moves.length)).toBe(3);
  expect(softwareOrganizationPage(moves, 0)).toHaveLength(50);
  expect(softwareOrganizationPage(moves, 1)[0].id).toBe('50');
  expect(softwareOrganizationPage(moves, 2)).toHaveLength(23);
  expect(softwareOrganizationPage(moves, -1)).toHaveLength(50);
});
