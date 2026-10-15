import {gitsssReactPage, projectID} from './routes.ts';

test('gitsssReactPage maps repository sections', () => {
  expect(gitsssReactPage('/alice/repo', '')).toEqual('code');
  expect(gitsssReactPage('/alice/repo/src/branch/main', '')).toEqual('code');
  expect(gitsssReactPage('/alice/repo/issues', '')).toEqual('issues');
  expect(gitsssReactPage('/alice/repo/pulls/2', '')).toEqual('pulls');
  expect(gitsssReactPage('/alice/repo/actions', '')).toEqual('actions');
  expect(gitsssReactPage('/alice/repo/projects/4', '')).toEqual('projects');
  expect(gitsssReactPage('/alice/repo/projects', '')).toEqual('');
  expect(gitsssReactPage('/alice/repo/settings', '')).toEqual('settings');
  expect(gitsssReactPage('/alice/repo/wiki/Home', '')).toEqual('wiki');
  expect(gitsssReactPage('/alice/repo/releases', '')).toEqual('releases');
  expect(gitsssReactPage('/alice/repo/issues', '?classic=1')).toEqual('');
  expect(projectID('/alice/repo/projects/4')).toEqual('4');
});
