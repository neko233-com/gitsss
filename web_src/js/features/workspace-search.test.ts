import {workspaceResultURL} from './workspace-search.ts';

test('global search only navigates same-host HTTP paths, preserving aliases', () => {
  const host = window.location.hostname;
  expect(workspaceResultURL('/engineering/demo')).toBe('/engineering/demo');
  expect(workspaceResultURL(`http://${host}:5030/engineering/demo/issues/1?q=a`)).toBe('/engineering/demo/issues/1?q=a');
  expect(workspaceResultURL('https://external.example.com/secret')).toBe('');
  // eslint-disable-next-line no-script-url
  expect(workspaceResultURL('javascript:alert(1)')).toBe('');
  expect(workspaceResultURL(`http://user:password@${host}/`)).toBe('');
});
