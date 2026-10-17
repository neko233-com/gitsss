import {folderChildren} from './software-center.ts';

test('folder tree indexes children once and preserves parent relationships', () => {
  const base = {name: 'Folder', directory: true, size: 0, modified: 0};
  const result = folderChildren([{...base, id: 'a', parent: ''}, {...base, id: 'b', parent: 'a'}, {...base, id: 'c', parent: 'a'}]);
  expect(result.get('')!.map((node) => node.id)).toEqual(['a']);
  expect(result.get('a')!.map((node) => node.id)).toEqual(['b', 'c']);
  expect(result.has('b')).toBe(false);
});
