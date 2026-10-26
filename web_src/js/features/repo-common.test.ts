import {initRepoCloneButtons, sanitizeRepoName, substituteRepoOpenWithUrl} from './repo-common.ts';
import {localUserSettings} from '../modules/user-settings.ts';

describe('clone protocol selection', () => {
  const httpUrl = 'http://gitsss.example/owner/repo.git';
  const sshUrl = 'git@gitsss.example:owner/repo.git';

  function renderCloneButtons(withHttp = true) {
    document.body.innerHTML = `<div class="clone-buttons-combo">
      ${withHttp ? `<button class="repo-clone-https" data-link="${httpUrl}">HTTPS</button>` : ''}
      <button class="repo-clone-ssh" data-link="${sshUrl}">SSH</button>
      <input class="repo-clone-url js-clone-url">
      <a class="js-clone-url-editor" data-href-template="vscode://clone?url={url}"></a>
    </div><span class="js-clone-url"></span>`;
    initRepoCloneButtons();
  }

  afterEach(() => {
    document.body.innerHTML = '';
    localStorage.clear();
  });

  test.each(['ssh', 'tea'])('prefers HTTP over the saved %s preference', (savedProtocol) => {
    localUserSettings.setString('repo-clone-protocol', savedProtocol);
    renderCloneButtons();
    expect(document.querySelector<HTMLInputElement>('.repo-clone-url')!.value).toBe(httpUrl);
    expect(document.querySelector('.repo-clone-https')!.textContent).toBe('HTTP');
    expect(document.querySelector('.repo-clone-https')!.classList.contains('active')).toBe(true);
    expect(document.querySelector('.js-clone-url-editor')!.getAttribute('href')).toBe(`vscode://clone?url=${encodeURIComponent(httpUrl)}`);
  });

  test('explicit SSH selection updates commands and resets on a new page', () => {
    renderCloneButtons();
    document.querySelector<HTMLButtonElement>('.repo-clone-ssh')!.click();
    expect(document.querySelector<HTMLInputElement>('.repo-clone-url')!.value).toBe(sshUrl);
    expect(document.querySelector('span.js-clone-url')!.textContent).toBe(sshUrl);
    expect(document.querySelector('.js-clone-url-editor')!.getAttribute('href')).toBe(`vscode://clone?url=${encodeURIComponent(sshUrl)}`);
    document.querySelector<HTMLButtonElement>('.repo-clone-https')!.click();
    expect(document.querySelector<HTMLInputElement>('.repo-clone-url')!.value).toBe(httpUrl);
    document.querySelector<HTMLButtonElement>('.repo-clone-ssh')!.click();
    renderCloneButtons();
    expect(document.querySelector<HTMLInputElement>('.repo-clone-url')!.value).toBe(httpUrl);
  });

  test('falls back to SSH when HTTP Git is disabled', () => {
    renderCloneButtons(false);
    expect(document.querySelector<HTMLInputElement>('.repo-clone-url')!.value).toBe(sshUrl);
    expect(document.querySelector('.repo-clone-ssh')!.classList.contains('active')).toBe(true);
  });
});

test('substituteRepoOpenWithUrl', () => {
  // For example: "x-github-client://openRepo/https://github.com/go-gitea/gitea"
  expect(substituteRepoOpenWithUrl('proto://a/{url}', 'https://gitea')).toEqual('proto://a/https://gitea');
  expect(substituteRepoOpenWithUrl('proto://a?link={url}', 'https://gitea')).toEqual('proto://a?link=https%3A%2F%2Fgitea');
});

test('sanitizeRepoName', () => {
  expect(sanitizeRepoName(' a b ')).toEqual('a-b');
  expect(sanitizeRepoName('a-b_c.git ')).toEqual('a-b_c');
  expect(sanitizeRepoName('/x.git/')).toEqual('-x.git-');
  expect(sanitizeRepoName('.profile')).toEqual('.profile');
  expect(sanitizeRepoName('.profile.')).toEqual('.profile');
  expect(sanitizeRepoName('.pro..file')).toEqual('.pro.file');

  expect(sanitizeRepoName('foo.rss.atom.git.wiki')).toEqual('foo');

  expect(sanitizeRepoName('.')).toEqual('');
  expect(sanitizeRepoName('..')).toEqual('');
  expect(sanitizeRepoName('-')).toEqual('');
});
