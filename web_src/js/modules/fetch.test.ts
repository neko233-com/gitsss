import {request} from './fetch.ts';

afterEach(() => vi.unstubAllGlobals());

test('request copies typed-array bodies to an ArrayBuffer', async () => {
  const fetchMock = vi.fn<typeof fetch>(async () => new Response());
  vi.stubGlobal('fetch', fetchMock);

  const data = new Uint8Array([0, 1, 2, 0]).subarray(1, 3);
  await request('/upload', {method: 'POST', data});

  const body = fetchMock.mock.calls[0][1]!.body!;
  expect(body).toBeInstanceOf(ArrayBuffer);
  expect(Array.from(new Uint8Array(body as ArrayBuffer))).toEqual([1, 2]);
});
