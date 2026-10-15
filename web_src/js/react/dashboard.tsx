import {useEffect, useState} from 'react';
import {createRoot} from 'react-dom/client';
import {Aurora} from './shader.tsx';

type Notice = {id: string; title: string; body: string};

function Announcements({endpoint}: {endpoint: string}) {
  const [notices, setNotices] = useState<Notice[] | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    if (!endpoint) {
      setNotices([]);
      return undefined;
    }
    let ignore = false;
    fetch(endpoint, {credentials: 'same-origin', headers: {Accept: 'application/json'}})
      .then(async (response) => {
        if (!response.ok) throw new Error(String(response.status));
        return await response.json() as Notice[];
      })
      .then((body) => {
        if (!ignore) setNotices(body);
      })
      .catch(() => {
        if (!ignore) setFailed(true);
      });
    return () => {
      ignore = true;
    };
  }, [endpoint]);

  return (
    <section className="gitsss-react-home">
      <style>{`
        .gitsss-react-home { position: relative; overflow: hidden; margin-bottom: 16px; border: 1px solid var(--color-secondary, #d0d7de); border-radius: 6px; background: var(--color-box-body, #ffffff); color: var(--color-text, #1f2328); }
        .gitsss-react-shader { position: absolute; inset: 0; width: 100%; height: 100%; pointer-events: none; opacity: 0.42; }
        .gitsss-react-body { position: relative; padding: 16px 20px; }
        .gitsss-react-kicker { margin: 0 0 4px; font-size: 12px; font-weight: 600; letter-spacing: 0.04em; text-transform: uppercase; color: var(--color-text-light, #656d76); }
        .gitsss-react-home h2 { margin: 0 0 12px; font-size: 20px; font-weight: 600; }
        .gitsss-react-card { margin: 0 0 10px; padding: 10px 12px; border: 1px solid var(--color-secondary, #d0d7de); border-radius: 6px; background: color-mix(in srgb, var(--color-box-body, #fff) 88%, transparent); }
        .gitsss-react-card h3 { margin: 0 0 4px; font-size: 16px; }
        .gitsss-react-card p { margin: 0; white-space: pre-wrap; }
      `}</style>
      <Aurora />
      <div className="gitsss-react-body">
        <p className="gitsss-react-kicker">gitsss</p>
        <h2>企业公告</h2>
        {failed ? <p>公告暂时无法加载</p> : null}
        {notices === null && !failed ? <p>正在加载</p> : null}
        {notices !== null && notices.length === 0 ? <p>暂无公告</p> : null}
        {notices?.map((notice) => (
          <article className="gitsss-react-card" key={notice.id}>
            <h3>{notice.title}</h3>
            <p>{notice.body}</p>
          </article>
        ))}
      </div>
    </section>
  );
}

const root = document.getElementById('gitsss-react-dashboard');
if (root) {
  createRoot(root).render(<Announcements endpoint={root.dataset.endpoint || ''} />);
}
