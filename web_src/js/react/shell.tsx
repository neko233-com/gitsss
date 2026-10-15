import {useEffect, useState} from 'react';
import {createRoot} from 'react-dom/client';
import {Aurora} from './shader.tsx';
import {gitsssReactPage, projectID, type GitsssReactPage} from './routes.ts';

type Item = {number?: number; title?: string; state?: string; name?: string};
type Repo = {full_name?: string; description?: string; private?: boolean; default_branch?: string; archived?: boolean};
type Entry = {name?: string; path?: string; type?: string};
type Run = {id?: number; name?: string; display_title?: string; status?: string; head_branch?: string};
type Card = {id: number; number: number; title: string};
type Column = {id: number; title: string; cards: Card[]};
type Board = {id: number; title: string; columns: Column[]};

const shellStyle = `
.gitsss-react-shell { position: relative; overflow: hidden; margin: 16px auto 24px; max-width: 1120px; border: 1px solid var(--color-secondary, #d0d7de); border-radius: 6px; background: var(--color-box-body, #fff); color: var(--color-text, #1f2328); }
.gitsss-react-shader { position: absolute; inset: 0; width: 100%; height: 100%; pointer-events: none; opacity: 0.35; }
.gitsss-react-shell-body { position: relative; padding: 16px 20px 20px; }
.gitsss-react-shell h2 { margin: 0 0 12px; font-size: 20px; font-weight: 600; }
.gitsss-react-row { display: flex; gap: 12px; align-items: center; padding: 8px 0; border-bottom: 1px solid var(--color-secondary, #d0d7de); }
.gitsss-react-row a { color: var(--color-primary, #0969da); font-weight: 600; text-decoration: none; }
.gitsss-react-meta { color: var(--color-text-light, #656d76); font-size: 12px; }
.gitsss-react-columns { display: flex; gap: 12px; overflow-x: auto; }
.gitsss-react-column { min-width: 220px; padding: 8px; border: 1px solid var(--color-secondary, #d0d7de); border-radius: 6px; background: color-mix(in srgb, var(--color-box-body, #fff) 86%, #0969da); }
.gitsss-react-card { margin-top: 8px; padding: 8px; border-radius: 6px; background: var(--color-box-body, #fff); }
.gitsss-react-classic { margin-top: 12px; font-size: 13px; }
`;

function apiPath(sub: string, owner: string, repo: string, suffix: string): string {
  const prefix = sub.endsWith('/') ? sub.slice(0, -1) : sub;
  return `${prefix}/api/v1/repos/${encodeURIComponent(owner)}/${encodeURIComponent(repo)}${suffix}`;
}

async function readJSON<T>(url: string, headers?: HeadersInit): Promise<T> {
  const response = await fetch(url, {credentials: 'same-origin', headers: {Accept: 'application/json', ...headers}});
  if (!response.ok) throw new Error(String(response.status));
  return await response.json() as T;
}

function Rows({items, href}: {items: Item[]; href: (item: Item) => string}) {
  if (items.length === 0) return <p>这里还没有内容</p>;
  return (
    <div>
      {items.map((item) => (
        <div className="gitsss-react-row" key={`${item.number ?? ''}-${item.title ?? item.name ?? ''}`}>
          <a href={href(item)}>{item.title || item.name || `#${item.number ?? ''}`}</a>
          <span className="gitsss-react-meta">{item.state || ''}</span>
        </div>
      ))}
    </div>
  );
}

function Shell({page, owner, repo, sub, pathname}: {page: GitsssReactPage; owner: string; repo: string; sub: string; pathname: string}) {
  const [text, setText] = useState('正在加载');
  const [repoInfo, setRepoInfo] = useState<Repo | null>(null);
  const [items, setItems] = useState<Item[]>([]);
  const [entries, setEntries] = useState<Entry[]>([]);
  const [runs, setRuns] = useState<Run[]>([]);
  const [board, setBoard] = useState<Board | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    let ignore = false;
    const load = async () => {
      if (page === 'code') {
        const body = await readJSON<Entry[] | Entry>(apiPath(sub, owner, repo, '/contents/'));
        if (!ignore) setEntries(Array.isArray(body) ? body : [body]);
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
        if (!ignore) setRepoInfo(await readJSON<Repo>(apiPath(sub, owner, repo, '')));
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
    if (text !== '' || failed || page === 'settings') return undefined;
    const legacy = document.querySelectorAll<HTMLElement>('.page-content.repository > .ui.container, .page-content.repository > .ui.container.fluid');
    for (const node of legacy) node.hidden = true;
    return () => {
      for (const node of legacy) node.hidden = false;
    };
  }, [text, failed, page]);

  const classic = `${pathname}${pathname.includes('?') ? '&' : '?'}classic=1`;
  return (
    <section className="gitsss-react-shell">
      <style>{shellStyle}</style>
      <Aurora />
      <div className="gitsss-react-shell-body">
        <p className="gitsss-react-meta">{owner}/{repo}</p>
        <h2>{titleFor(page)}</h2>
        {failed ? <p>新界面暂时加载失败，下面仍可使用经典页面。</p> : null}
        {text !== '' && !failed ? <p>{text}</p> : null}
        {page === 'code' ? (
          <div>
            {entries.map((entry) => (
              <div className="gitsss-react-row" key={entry.path || entry.name}>
                <span>{entry.type === 'dir' ? '目录' : '文件'}</span>
                <a href={`${sub}/${owner}/${repo}/src/branch/${repoInfo?.default_branch || 'main'}/${entry.path || entry.name}`}>{entry.name}</a>
              </div>
            ))}
          </div>
        ) : null}
        {page === 'issues' ? <Rows items={items} href={(item) => `${sub}/${owner}/${repo}/issues/${item.number}`} /> : null}
        {page === 'pulls' ? <Rows items={items} href={(item) => `${sub}/${owner}/${repo}/pulls/${item.number}`} /> : null}
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
              <div className="gitsss-react-column" key={column.id}>
                <strong>{column.title}</strong>
                {column.cards.map((card) => (
                  <div className="gitsss-react-card" key={card.id}>#{card.number} {card.title}</div>
                ))}
              </div>
            ))}
          </div>
        ) : null}
        {page === 'settings' && repoInfo ? (
          <div>
            <p>{repoInfo.description || '没有描述'}</p>
            <p className="gitsss-react-meta">默认分支 {repoInfo.default_branch} · {repoInfo.private ? '私有' : '公开'}{repoInfo.archived ? ' · 已归档' : ''}</p>
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
