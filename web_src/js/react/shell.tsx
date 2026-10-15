import {useEffect, useState, type FormEvent} from 'react';
import {createRoot} from 'react-dom/client';
import {commentPayload, fileUpdatePayload, mergePayload, moveIssuesPayload, repoSettingsPayload, webhookPayload, type BoardCard} from './actions.ts';
import {Aurora} from './shader.tsx';
import {gitsssReactPage, projectID, type GitsssReactPage} from './routes.ts';

type Item = {number?: number; title?: string; state?: string; name?: string; login?: string};
type Repo = {description?: string; private?: boolean; default_branch?: string; archived?: boolean; has_wiki?: boolean};
type Entry = {name?: string; path?: string; type?: string};
type FileBody = {content?: string; sha?: string; path?: string; encoding?: string};
type Run = {id?: number; name?: string; display_title?: string; status?: string; head_branch?: string};
type Column = {id: number; title: string; cards: BoardCard[]};
type Board = {id: number; title: string; columns: Column[]};
type Hook = {id?: number; config?: {url?: string}; active?: boolean};
type Collaborator = {login?: string; username?: string};

const shellStyle = `
.gitsss-react-shell { position: relative; overflow: hidden; margin: 16px auto 24px; max-width: 1120px; border: 1px solid var(--color-secondary, #d0d7de); border-radius: 6px; background: var(--color-box-body, #fff); color: var(--color-text, #1f2328); }
.gitsss-react-shader { position: absolute; inset: 0; width: 100%; height: 100%; pointer-events: none; opacity: 0.35; }
.gitsss-react-shell-body { position: relative; padding: 16px 20px 20px; }
.gitsss-react-shell h2 { margin: 0 0 12px; font-size: 20px; font-weight: 600; }
.gitsss-react-row, .gitsss-react-card { display: flex; gap: 12px; align-items: center; }
.gitsss-react-row { padding: 8px 0; border-bottom: 1px solid var(--color-secondary, #d0d7de); }
.gitsss-react-row button, .gitsss-react-shell button { border: 1px solid var(--color-secondary, #d0d7de); border-radius: 6px; background: var(--color-box-body, #fff); color: inherit; padding: 4px 8px; cursor: pointer; }
.gitsss-react-row a { color: var(--color-primary, #0969da); font-weight: 600; text-decoration: none; }
.gitsss-react-meta { color: var(--color-text-light, #656d76); font-size: 12px; }
.gitsss-react-columns { display: flex; gap: 12px; overflow-x: auto; }
.gitsss-react-column { min-width: 220px; padding: 8px; border: 1px solid var(--color-secondary, #d0d7de); border-radius: 6px; background: color-mix(in srgb, var(--color-box-body, #fff) 86%, #0969da); }
.gitsss-react-card { margin-top: 8px; padding: 8px; border-radius: 6px; background: var(--color-box-body, #fff); cursor: grab; }
.gitsss-react-shell textarea, .gitsss-react-shell input { width: 100%; margin: 8px 0; box-sizing: border-box; }
.gitsss-react-diff { white-space: pre-wrap; font-family: ui-monospace, monospace; font-size: 12px; }
.gitsss-react-classic { margin-top: 12px; font-size: 13px; }
`;

function apiPath(sub: string, owner: string, repo: string, suffix: string): string {
  const prefix = sub.endsWith('/') ? sub.slice(0, -1) : sub;
  return `${prefix}/api/v1/repos/${encodeURIComponent(owner)}/${encodeURIComponent(repo)}${suffix}`;
}

async function readJSON<T>(url: string): Promise<T> {
  const response = await fetch(url, {credentials: 'same-origin', headers: {Accept: 'application/json'}});
  if (!response.ok) throw new Error(String(response.status));
  return await response.json() as T;
}

async function sendJSON(url: string, method: string, body: unknown): Promise<void> {
  const response = await fetch(url, {
    method,
    credentials: 'same-origin',
    headers: {Accept: 'application/json', 'Content-Type': 'application/json'},
    body: JSON.stringify(body),
  });
  if (!response.ok) throw new Error(String(response.status));
}

async function readText(url: string): Promise<string> {
  const response = await fetch(url, {credentials: 'same-origin'});
  if (!response.ok) throw new Error(String(response.status));
  return await response.text();
}

function Rows({items, href}: {items: Item[]; href: (item: Item) => string}) {
  if (items.length === 0) return <p>这里还没有内容</p>;
  return (
    <div>
      {items.map((item) => (
        <div className="gitsss-react-row" key={`${item.number ?? ''}-${item.title ?? item.name ?? item.login ?? ''}`}>
          <a href={href(item)}>{item.title || item.name || item.login || `#${item.number ?? ''}`}</a>
          <span className="gitsss-react-meta">{item.state || ''}</span>
        </div>
      ))}
    </div>
  );
}

function Shell({page, owner, repo, sub, pathname}: {page: GitsssReactPage; owner: string; repo: string; sub: string; pathname: string}) {
  const [text, setText] = useState('正在加载');
  const [notice, setNotice] = useState('');
  const [repoInfo, setRepoInfo] = useState<Repo | null>(null);
  const [items, setItems] = useState<Item[]>([]);
  const [entries, setEntries] = useState<Entry[]>([]);
  const [file, setFile] = useState<FileBody | null>(null);
  const [draft, setDraft] = useState('');
  const [runs, setRuns] = useState<Run[]>([]);
  const [board, setBoard] = useState<Board | null>(null);
  const [hooks, setHooks] = useState<Hook[]>([]);
  const [collaborators, setCollaborators] = useState<Collaborator[]>([]);
  const [diff, setDiff] = useState('');
  const [selectedPull, setSelectedPull] = useState<number | null>(null);
  const [failed, setFailed] = useState(false);
  const branch = repoInfo?.default_branch || 'main';

  useEffect(() => {
    let ignore = false;
    const load = async () => {
      if (page === 'code') {
        const [body, info] = await Promise.all([
          readJSON<Entry[] | Entry>(apiPath(sub, owner, repo, '/contents/')),
          readJSON<Repo>(apiPath(sub, owner, repo, '')),
        ]);
        if (!ignore) {
          setEntries(Array.isArray(body) ? body : [body]);
          setRepoInfo(info);
        }
      } else if (page === 'issues') {
        if (!ignore) setItems(await readJSON<Item[]>(apiPath(sub, owner, repo, '/issues?type=issues&state=open&limit=30')));
      } else if (page === 'pulls') {
        if (!ignore) setItems(await readJSON<Item[]>(apiPath(sub, owner, repo, '/pulls?state=open&limit=30')));
      } else if (page === 'actions') {
        const body = await readJSON<{workflow_runs?: Run[]}>(apiPath(sub, owner, repo, '/actions/tasks?limit=20'));
        if (!ignore) setRuns(body.workflow_runs ?? []);
      } else if (page === 'wiki') {
        const pages = await readJSON<Array<{title?: string}>>(apiPath(sub, owner, repo, '/wiki/pages'));
        if (!ignore) setItems(pages.map((wiki) => ({title: wiki.title})));
      } else if (page === 'releases') {
        const releases = await readJSON<Array<{name?: string; tag_name?: string}>>(apiPath(sub, owner, repo, '/releases?limit=20'));
        if (!ignore) setItems(releases.map((release) => ({title: release.name || release.tag_name, name: release.tag_name})));
      } else if (page === 'settings') {
        const [info, people, hookList] = await Promise.all([
          readJSON<Repo>(apiPath(sub, owner, repo, '')),
          readJSON<Collaborator[]>(apiPath(sub, owner, repo, '/collaborators')),
          readJSON<Hook[]>(apiPath(sub, owner, repo, '/hooks')),
        ]);
        if (!ignore) {
          setRepoInfo(info);
          setCollaborators(people);
          setHooks(hookList);
        }
      } else if (page === 'projects') {
        const id = projectID(pathname);
        const prefix = sub.endsWith('/') ? sub.slice(0, -1) : sub;
        const body = await readJSON<Board>(`${prefix}/${encodeURIComponent(owner)}/${encodeURIComponent(repo)}/projects/${encodeURIComponent(id)}`);
        if (!ignore) setBoard(body);
      }
      if (!ignore) setText('');
    };
    load().catch(() => {
      if (!ignore) {
        setFailed(true);
        setText('');
      }
    });
    return () => {
      ignore = true;
    };
  }, [page, owner, repo, sub, pathname]);

  useEffect(() => {
    if (text !== '' || failed) return undefined;
    const legacy = document.querySelectorAll<HTMLElement>('.page-content.repository > .ui.container, .page-content.repository > .ui.container.fluid');
    for (const node of legacy) node.hidden = true;
    return () => {
      for (const node of legacy) node.hidden = false;
    };
  }, [text, failed]);

  async function openFile(entry: Entry) {
    if (entry.type === 'dir' || !entry.path) return;
    const body = await readJSON<FileBody>(apiPath(sub, owner, repo, `/contents/${entry.path}?ref=${encodeURIComponent(branch)}`));
    setFile(body);
    setDraft(body.encoding === 'base64' && body.content ? atob(body.content) : body.content || '');
  }

  async function saveFile() {
    if (!file?.path || !file.sha) return;
    await sendJSON(apiPath(sub, owner, repo, `/contents/${file.path}`), 'PUT', fileUpdatePayload(draft, file.sha, `edit ${file.path}`, branch));
    setNotice('文件已提交');
  }

  async function openDiff(number: number) {
    setSelectedPull(number);
    setDiff(await readText(apiPath(sub, owner, repo, `/pulls/${number}.diff`)));
  }

  async function commentOnPull() {
    if (selectedPull === null || draft.trim() === '') return;
    await sendJSON(apiPath(sub, owner, repo, `/issues/${selectedPull}/comments`), 'POST', commentPayload(draft));
    setDraft('');
    setNotice('评论已发送');
  }

  async function mergePull() {
    if (selectedPull === null) return;
    await sendJSON(apiPath(sub, owner, repo, `/pulls/${selectedPull}/merge`), 'POST', mergePayload());
    setNotice('合并请求已提交');
  }

  function dropCard(columnID: number, raw: string) {
    if (!board) return;
    const card = JSON.parse(raw) as BoardCard;
    const next = board.columns.map((column) => ({...column, cards: column.cards.filter((item) => item.id !== card.id)}));
    const target = next.find((column) => column.id === columnID);
    if (!target) return;
    target.cards = [...target.cards, card];
    setBoard({...board, columns: next});
    const prefix = sub.endsWith('/') ? sub.slice(0, -1) : sub;
    void sendJSON(`${prefix}/${encodeURIComponent(owner)}/${encodeURIComponent(repo)}/projects/${board.id}/${columnID}/move`, 'POST', moveIssuesPayload(target.cards))
      .then(() => setNotice('看板已更新'))
      .catch(() => setNotice('看板移动失败'));
  }

  async function saveSettings(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!repoInfo) return;
    const form = new FormData(event.currentTarget);
    await sendJSON(apiPath(sub, owner, repo, ''), 'PATCH', repoSettingsPayload({
      description: String(form.get('description') || ''),
      private: form.get('private') === 'on',
      hasWiki: form.get('has_wiki') === 'on',
      defaultBranch: String(form.get('default_branch') || branch),
    }));
    setNotice('仓库设置已保存');
  }

  async function addCollaborator(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const login = String(form.get('login') || '');
    await sendJSON(apiPath(sub, owner, repo, `/collaborators/${encodeURIComponent(login)}`), 'PUT', {permission: String(form.get('permission') || 'write')});
    setCollaborators([...collaborators, {login}]);
    setNotice('协作者已添加');
  }

  async function addWebhook(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    await sendJSON(apiPath(sub, owner, repo, '/hooks'), 'POST', webhookPayload(String(form.get('url') || '')));
    setNotice('Webhook 已创建');
  }

  const classic = `${pathname}?classic=1`;
  return (
    <section className="gitsss-react-shell">
      <style>{shellStyle}</style>
      <Aurora />
      <div className="gitsss-react-shell-body">
        <p className="gitsss-react-meta">{owner}/{repo}</p>
        <h2>{titleFor(page)}</h2>
        {failed ? <p>新界面暂时加载失败，下面仍可使用经典页面。</p> : null}
        {notice ? <p>{notice}</p> : null}
        {text !== '' && !failed ? <p>{text}</p> : null}
        {page === 'code' ? (
          <div>
            {entries.map((entry) => (
              <div className="gitsss-react-row" key={entry.path || entry.name}>
                <span>{entry.type === 'dir' ? '目录' : '文件'}</span>
                {entry.type === 'file' ? <button type="button" onClick={() => void openFile(entry).catch(() => setNotice('文件读取失败'))}>{entry.name}</button> : <span>{entry.name}</span>}
              </div>
            ))}
            {file ? (
              <form onSubmit={(event) => {
                event.preventDefault();
                void saveFile().catch(() => setNotice('提交失败'));
              }}>
                <textarea rows={12} value={draft} onChange={(event) => setDraft(event.target.value)} />
                <button type="submit">提交到 {branch}</button>
              </form>
            ) : null}
          </div>
        ) : null}
        {page === 'issues' ? <Rows items={items} href={(item) => `${sub}/${owner}/${repo}/issues/${item.number}`} /> : null}
        {page === 'pulls' ? (
          <div>
            {items.map((item) => (
              <div className="gitsss-react-row" key={item.number}>
                <a href={`${sub}/${owner}/${repo}/pulls/${item.number}`}>#{item.number} {item.title}</a>
                <button type="button" onClick={() => void openDiff(item.number || 0).catch(() => setNotice('diff 读取失败'))}>查看 diff</button>
              </div>
            ))}
            {selectedPull !== null ? (
              <div>
                <pre className="gitsss-react-diff">{diff}</pre>
                <textarea rows={4} value={draft} onChange={(event) => setDraft(event.target.value)} placeholder="评论" />
                <button type="button" onClick={() => void commentOnPull().catch(() => setNotice('评论失败'))}>发表评论</button>
                <button type="button" onClick={() => void mergePull().catch(() => setNotice('合并失败'))}>合并</button>
              </div>
            ) : null}
          </div>
        ) : null}
        {page === 'wiki' ? <Rows items={items} href={(item) => `${sub}/${owner}/${repo}/wiki/${encodeURIComponent(item.title || '')}`} /> : null}
        {page === 'releases' ? <Rows items={items} href={() => `${sub}/${owner}/${repo}/releases`} /> : null}
        {page === 'actions' ? (
          <div>
            {runs.length === 0 ? <p>没有 Actions 记录</p> : runs.map((run) => (
              <div className="gitsss-react-row" key={run.id}>
                <a href={`${sub}/${owner}/${repo}/actions`}>{run.display_title || run.name}</a>
                <span className="gitsss-react-meta">{run.status} {run.head_branch}</span>
              </div>
            ))}
          </div>
        ) : null}
        {page === 'projects' && board ? (
          <div className="gitsss-react-columns">
            {board.columns.map((column) => (
              <div className="gitsss-react-column" key={column.id} onDragOver={(event) => event.preventDefault()} onDrop={(event) => {
                event.preventDefault();
                dropCard(column.id, event.dataTransfer.getData('text/plain'));
              }}>
                <strong>{column.title}</strong>
                {column.cards.map((card) => (
                  <div className="gitsss-react-card" key={card.id} draggable onDragStart={(event) => event.dataTransfer.setData('text/plain', JSON.stringify(card))}>
                    #{card.number} {card.title}
                  </div>
                ))}
              </div>
            ))}
          </div>
        ) : null}
        {page === 'settings' && repoInfo ? (
          <div>
            <form onSubmit={(event) => void saveSettings(event).catch(() => setNotice('设置保存失败'))}>
              <label>描述<textarea name="description" defaultValue={repoInfo.description || ''} /></label>
              <label>默认分支<input name="default_branch" defaultValue={repoInfo.default_branch || ''} /></label>
              <label><input name="private" type="checkbox" defaultChecked={repoInfo.private} /> 私有</label>
              <label><input name="has_wiki" type="checkbox" defaultChecked={repoInfo.has_wiki} /> Wiki</label>
              <button type="submit">保存仓库</button>
            </form>
            <h3>协作者</h3>
            {collaborators.map((person) => <p key={person.login || person.username}>{person.login || person.username}</p>)}
            <form onSubmit={(event) => void addCollaborator(event).catch(() => setNotice('协作者添加失败'))}>
              <input name="login" placeholder="用户名" required />
              <select name="permission" defaultValue="write">
                <option value="read">read</option>
                <option value="write">write</option>
                <option value="admin">admin</option>
              </select>
              <button type="submit">添加权限</button>
            </form>
            <h3>Webhook</h3>
            {hooks.map((hook) => <p key={hook.id}>{hook.config?.url}</p>)}
            <form onSubmit={(event) => void addWebhook(event).catch(() => setNotice('Webhook 创建失败'))}>
              <input name="url" type="url" placeholder="https://example.test/hook" required />
              <button type="submit">创建 Webhook</button>
            </form>
          </div>
        ) : null}
        <p className="gitsss-react-classic"><a href={classic}>使用经典界面</a></p>
      </div>
    </section>
  );
}

function titleFor(page: GitsssReactPage): string {
  switch (page) {
    case 'code': return '代码';
    case 'issues': return 'Issue';
    case 'pulls': return 'Pull Request';
    case 'actions': return 'Actions';
    case 'projects': return '看板';
    case 'settings': return '仓库设置';
    case 'wiki': return 'Wiki';
    case 'releases': return 'Release';
    default: return '';
  }
}

const root = document.getElementById('gitsss-react-shell');
if (root) {
  const owner = root.dataset.owner || '';
  const repo = root.dataset.repo || '';
  const sub = root.dataset.sub || '';
  const path = window.location.pathname.startsWith(sub) ? window.location.pathname.slice(sub.length) : window.location.pathname;
  const page = gitsssReactPage(path, window.location.search);
  if (page && owner && repo) {
    root.hidden = false;
    createRoot(root).render(<Shell page={page} owner={owner} repo={repo} sub={sub} pathname={path} />);
  }
}
