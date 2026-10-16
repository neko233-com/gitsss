import {initHomeLinks} from './home-links.ts';
import {POST} from '../modules/fetch.ts';
import {showErrorToast} from '../modules/toast.ts';

vi.mock('../modules/fetch.ts', () => ({POST: vi.fn()}));
vi.mock('../modules/toast.ts', () => ({showErrorToast: vi.fn()}));

function fixture() {
  return `<section data-home-links data-error="Failed" data-conflict="Reload" data-invalid="Invalid">
    <div class="home-links-grid"><a>Old</a></div>
    <form data-home-links-form action="/home-links"><input name="scope" value="personal"><input name="revision" value="old">
    <div data-home-link-rows></div><template data-home-link-template><div class="home-link-row"><input name="title"><input name="url"><button type="button" data-home-link-remove>Remove</button></div></template>
    <button type="button" data-home-link-add>Add</button><button>Save</button></form></section>`;
}

describe('homepage links', {concurrent: false}, () => {
  beforeEach(() => vi.clearAllMocks());

  test('links add, remove and save in place', async () => {
    document.body.innerHTML = fixture();
    initHomeLinks();
    initHomeLinks();
    document.querySelector<HTMLButtonElement>('[data-home-link-add]')!.click();
    expect(document.querySelectorAll('.home-link-row')).toHaveLength(1);
    expect(document.activeElement).toBe(document.querySelector('[name="title"]'));
    document.querySelector<HTMLInputElement>('[name="title"]')!.value = 'CI';
    document.querySelector<HTMLInputElement>('[name="url"]')!.value = 'http://ci.internal';
    vi.mocked(POST).mockResolvedValue(new Response(fixture().replace('<a>Old</a>', '<a>CI</a>').replace('value="old"', 'value="new"')));
    document.querySelector('form')!.dispatchEvent(new Event('submit', {bubbles: true, cancelable: true}));
    await vi.waitFor(() => expect(document.querySelector('.home-links-grid')!.textContent).toBe('CI'));
    expect(document.querySelector<HTMLInputElement>('[name="revision"]')!.value).toBe('new');
    document.querySelector<HTMLButtonElement>('[data-home-link-add]')!.click();
    document.querySelector<HTMLButtonElement>('[data-home-link-remove]')!.click();
    expect(document.querySelectorAll('.home-link-row')).toHaveLength(0);
  });

  test('failed save preserves draft and explains stale editor', async () => {
    document.body.innerHTML = fixture();
    initHomeLinks();
    document.querySelector<HTMLButtonElement>('[data-home-link-add]')!.click();
    document.querySelector<HTMLInputElement>('[name="title"]')!.value = 'Keep me';
    vi.mocked(POST).mockResolvedValue(new Response('', {status: 409}));
    document.querySelector('form')!.dispatchEvent(new Event('submit', {bubbles: true, cancelable: true}));
    await vi.waitFor(() => expect(showErrorToast).toHaveBeenCalledWith('Reload'));
    expect(document.querySelector<HTMLInputElement>('[name="title"]')!.value).toBe('Keep me');
    expect(document.querySelector<HTMLButtonElement>('[data-home-link-add]')!.disabled).toBe(false);
  });
});
