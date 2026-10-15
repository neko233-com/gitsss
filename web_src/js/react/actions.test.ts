import {commentPayload, fileUpdatePayload, mergePayload, moveIssuesPayload, repoSettingsPayload, webhookPayload} from './actions.ts';

test('interaction payloads stay explicit', () => {
  expect(moveIssuesPayload([{id: 9, number: 3, title: 'Card'}])).toEqual({issues: [{issueID: 9, sorting: 0}]});
  expect(fileUpdatePayload('hello', 'abc', 'edit readme', 'main').content).toEqual(btoa('hello'));
  expect(repoSettingsPayload({description: 'game', private: true, hasWiki: true, defaultBranch: 'main'})).toEqual({
    description: 'game', private: true, has_wiki: true, default_branch: 'main',
  });
  expect(webhookPayload('https://example.test/hook').config.url).toEqual('https://example.test/hook');
  expect(mergePayload()).toEqual({Do: 'merge'});
  expect(commentPayload('looks good').body).toEqual('looks good');
});
