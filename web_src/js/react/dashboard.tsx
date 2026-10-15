import {useEffect, useState} from 'react';
import {createRoot} from 'react-dom/client';

type Notice = {id: string; title: string; body: string};

function Announcements({endpoint}: {endpoint: string}) {
  const [notices, setNotices] = useState<Notice[] | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
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
    <section>
      <h2>企业公告</h2>
      {failed ? <p>公告暂时无法加载</p> : null}
      {notices === null && !failed ? <p>正在加载</p> : null}
      {notices !== null && notices.length === 0 ? <p>暂无公告</p> : null}
      {notices?.map((notice) => (
        <article key={notice.id}>
          <h3>{notice.title}</h3>
          <p>{notice.body}</p>
        </article>
      ))}
    </section>
  );
}

const root = document.getElementById('gitsss-react-dashboard');
if (root) {
  createRoot(root).render(<Announcements endpoint={root.dataset.endpoint || ''} />);
}
