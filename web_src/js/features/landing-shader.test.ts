import {initLandingShader, initLandingWorkflow} from './landing-shader.ts';

const controls = vi.hoisted(() => ({fail: false, speed: vi.fn(), dispose: vi.fn()}));
vi.mock('../modules/paper-shader.ts', () => ({ShaderMount: class {
  constructor(target: HTMLElement) {
    if (controls.fail) throw new Error('WebGL unavailable');
    target.append(document.createElement('canvas'));
  }
  setSpeed = controls.speed;
  setUniforms = vi.fn();
  dispose = controls.dispose;
}}));

describe('landing animation lifecycle', {concurrent: false}, () => {
  afterEach(() => {
    window.dispatchEvent(new PageTransitionEvent('pagehide'));
    document.body.replaceChildren();
    vi.unstubAllGlobals();
    vi.clearAllMocks();
  });

  test.each([false, true])('landing motion remains controllable with fallback=%s', async (fallback) => {
    controls.fail = fallback;
    document.body.style.backgroundColor = 'rgb(255, 255, 255)';
    document.body.innerHTML = '<div class="landing-visual"><div data-landing-shader style="color:rgb(0, 100, 200)"></div><button data-landing-motion data-play="Play" data-pause="Pause" hidden></button></div>';
    vi.stubGlobal('matchMedia', () => ({matches: false, addEventListener: vi.fn(), removeEventListener: vi.fn()}));
    let visibility: (entries: {isIntersecting: boolean}[]) => void;
    vi.stubGlobal('IntersectionObserver', class {
      constructor(callback: typeof visibility) { visibility = callback }
      observe() {}
      disconnect() {}
    });
    await initLandingShader();
    const visual = document.querySelector<HTMLElement>('.landing-visual')!;
    const button = document.querySelector<HTMLButtonElement>('button')!;
    expect(visual.getAttribute('data-motion')).toBe('playing');
    expect(button.hidden).toBe(false);
    expect((document.querySelector<HTMLElement>('[data-landing-shader]')!).getAttribute('data-shader-status')).toBe(fallback ? 'fallback' : 'ready');
    button.click(); expect(visual.getAttribute('data-motion')).toBe('paused');
    button.click(); expect(visual.getAttribute('data-motion')).toBe('playing');
    visibility!([{isIntersecting: false}]); expect(visual.getAttribute('data-motion')).toBe('paused');
    visibility!([{isIntersecting: true}]); expect(visual.getAttribute('data-motion')).toBe('playing');
  });

  test('reduced motion uses a static SVG without allocating a WebGL canvas', async () => {
    document.body.innerHTML = '<div class="landing-visual"><div data-landing-shader style="color:rgb(0, 100, 200)"></div><button data-landing-motion hidden></button></div>';
    vi.stubGlobal('matchMedia', () => ({matches: true, addEventListener: vi.fn(), removeEventListener: vi.fn()}));
    vi.stubGlobal('IntersectionObserver', class { observe() {} disconnect() {} });
    await initLandingShader();
    expect(document.querySelector('canvas')).toBeNull();
    expect((document.querySelector<HTMLElement>('.landing-visual')!).getAttribute('data-motion')).toBe('paused');
    expect(document.querySelector<HTMLButtonElement>('button')!.hidden).toBe(true);
  });

  test('navigation during module loading does not allocate a late shader', async () => {
    controls.fail = false;
    document.body.innerHTML = '<div class="landing-visual"><div data-landing-shader></div><button data-landing-motion data-play="Play" data-pause="Pause"></button></div>';
    vi.stubGlobal('matchMedia', () => ({matches: false, addEventListener: vi.fn(), removeEventListener: vi.fn()}));
    vi.stubGlobal('IntersectionObserver', class { observe() {} disconnect() {} });
    const loading = initLandingShader();
    window.dispatchEvent(new PageTransitionEvent('pagehide'));
    await loading;
    expect(document.querySelector('canvas')).toBeNull();
  });
});

test('workflow selection changes the caption and documentation without navigation or losing state', () => {
  const visual = document.createElement('div');
  visual.innerHTML = '<button disabled data-landing-stage="code" data-caption="Code" data-docs="/help/code" aria-pressed="true"></button><button disabled data-landing-stage="delivery" data-caption="Actions & releases" data-docs="/help/actions" aria-pressed="false"></button><p data-landing-caption>Code</p><a data-landing-learn href="/help/code"></a>';
  initLandingWorkflow(visual);
  const buttons = visual.querySelectorAll('button');
  expect(buttons[0].disabled).toBe(false);
  buttons[1].click();
  expect(visual.getAttribute('data-mode')).toBe('delivery');
  expect(buttons[0].getAttribute('aria-pressed')).toBe('false');
  expect(buttons[1].getAttribute('aria-pressed')).toBe('true');
  expect(visual.querySelector('p')!.textContent).toBe('Actions & releases');
  expect(visual.querySelector('a')!.getAttribute('href')).toBe('/help/actions');
});
