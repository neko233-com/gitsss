import {initHeatmap, replaceHeatmap} from './heatmap.ts';
import {GET} from '../modules/fetch.ts';

vi.mock('../modules/fetch.ts', () => ({GET: vi.fn()}));
vi.mock('../components/ActivityHeatmap.vue', async () => {
  const {h} = await import('vue');
  return {default: {
    props: ['values', 'locale'],
    setup: (props: {locale: {textTotalContributions: string}}) => () => h('span', {'data-heatmap-value': ''}, props.locale.textTotalContributions),
  }};
});

function markup(url = '/org/acme/dashboard/-/heatmap') {
  const element = document.createElement('div');
  element.id = 'user-heatmap';
  element.className = 'is-loading';
  element.setAttribute('data-heatmap-url', url);
  element.setAttribute('data-locale-total-contributions', 'Contributions: %s');
  element.setAttribute('data-locale-error', 'Localized load failure');
  return element;
}

function response(total = 3) {
  return Response.json({heatmapData: [[Math.floor(Date.now() / 1000), total]], totalContributions: total});
}

describe('heatmap lifecycle', {concurrent: false}, () => {
  beforeEach(() => {
    vi.mocked(GET).mockReset();
    document.documentElement.lang = 'en-US';
    document.body.replaceChildren(markup());
  });

  test('renders contributions and clears loading state', async () => {
    vi.mocked(GET).mockResolvedValue(response());
    await initHeatmap();
    expect(document.querySelector('[data-heatmap-value]')!.textContent).toBe('Contributions: 3');
    expect(document.querySelector('#user-heatmap')!.hasAttribute('aria-busy')).toBe(false);
    expect(document.querySelector('#user-heatmap')!.classList.contains('is-loading')).toBe(false);
  });

  test('team replacement unmounts the previous chart and uses the scoped URL', async () => {
    vi.mocked(GET).mockResolvedValueOnce(response()).mockResolvedValueOnce(response(8));
    await initHeatmap();
    const firstSignal = vi.mocked(GET).mock.calls[0][1]!.signal!;
    await replaceHeatmap(markup('/org/acme/dashboard/-/heatmap/team'));
    expect(firstSignal.aborted).toBe(true);
    expect(vi.mocked(GET).mock.calls[1][0]).toBe('/org/acme/dashboard/-/heatmap/team');
    expect(document.querySelector('[data-heatmap-value]')!.textContent).toBe('Contributions: 8');
  });

  test('a delayed old request cannot replace a newer team chart', async () => {
    const {promise, resolve} = Promise.withResolvers<Response>();
    vi.mocked(GET).mockReturnValueOnce(promise).mockResolvedValueOnce(response(8));
    const oldRequest = initHeatmap();
    await replaceHeatmap(markup('/org/acme/dashboard/-/heatmap/team'));
    resolve(response(99));
    await oldRequest;
    expect(document.querySelector('[data-heatmap-value]')!.textContent).toBe('Contributions: 8');
  });

  test('failure is localized and does not leave an endless loading indicator', async () => {
    const log = vi.spyOn(console, 'error').mockImplementation(() => {});
    vi.mocked(GET).mockResolvedValue(new Response('', {status: 503}));
    await initHeatmap();
    expect(document.querySelector('#user-heatmap')!.textContent).toBe('Localized load failure');
    expect(document.querySelector('#user-heatmap')!.hasAttribute('aria-busy')).toBe(false);
    expect(document.querySelector('#user-heatmap')!.classList.contains('is-loading')).toBe(false);
    log.mockRestore();
  });
});
