import {hideElem, showElem, toggleElem} from '../utils/dom.ts';
import {sanitizeRepoName} from './repo-common.ts';

const service = document.querySelector<HTMLInputElement>('#service_type');
const user = document.querySelector<HTMLInputElement>('#auth_username');
const pass = document.querySelector<HTMLInputElement>('#auth_password');
const token = document.querySelector<HTMLInputElement>('#auth_token');
const mirror = document.querySelector<HTMLInputElement>('#mirror');
const lfs = document.querySelector<HTMLInputElement>('#lfs');
const lfsSettings = document.querySelector<HTMLElement>('#lfs_settings')!;
const lfsEndpoint = document.querySelector<HTMLElement>('#lfs_endpoint')!;
const items = document.querySelectorAll<HTMLInputElement>('#migrate_items input[type=checkbox]');

export function initRepoMigration() {
  const chooser = document.querySelector<HTMLElement>('[data-repo-migration-chooser]');
  if (chooser) {
    initMigrationSourceChooser(chooser);
    return;
  }
  checkAuth();
  setLFSSettingsVisibility();
  initMigrationPreview();

  user?.addEventListener('input', () => {checkItems(false)});
  pass?.addEventListener('input', () => {checkItems(false)});
  token?.addEventListener('input', () => {checkItems(true)});
  mirror?.addEventListener('change', () => {checkItems(true)});
  document.querySelector('#lfs_settings_show')?.addEventListener('click', (e) => {
    e.preventDefault();
    e.stopPropagation();
    showElem(lfsEndpoint);
  });
  lfs?.addEventListener('change', setLFSSettingsVisibility);

  const elCloneAddr = document.querySelector<HTMLInputElement>('#clone_addr');
  const elRepoName = document.querySelector<HTMLInputElement>('#repo_name');
  if (elCloneAddr && elRepoName) {
    let repoNameChanged = elRepoName.value !== '';
    elRepoName.addEventListener('input', () => {repoNameChanged = true});
    elCloneAddr.addEventListener('input', () => {
      if (repoNameChanged) return;
      let repoNameFromUrl = elCloneAddr.value.split(/[?#]/)[0];
      const parts = /^(.*\/)?((.+?)\/?)$/.exec(repoNameFromUrl);
      if (!parts || parts.length < 4) {
        elRepoName.value = '';
        return;
      }
      repoNameFromUrl = parts[3].split(/[?#]/)[0];
      elRepoName.value = sanitizeRepoName(repoNameFromUrl);
      elRepoName.dispatchEvent(new Event('change', {bubbles: true}));
    });
  }
}

function initMigrationPreview() {
  const form = document.querySelector<HTMLFormElement>('.repo-migrate-workspace');
  if (!form) return;
  const owner = form.querySelector<HTMLInputElement>('[name=uid]')!;
  const name = form.querySelector<HTMLInputElement>('[name=repo_name]')!;
  const privateInput = form.querySelector<HTMLInputElement>('[name=private]')!;
  const path = form.querySelector('[data-migration-target]')!;
  const identity = form.querySelector('[data-migration-owner]')!;
  const visibility = form.querySelector('[data-migration-visibility]')!;
  const visibilityHelp = form.querySelector('[data-migration-visibility-help]')!;
  const mode = form.querySelector('[data-migration-mode]')!;
  const placeholder = path.textContent.split(' / ').slice(1).join(' / ');
  let previousOwner = '';
  const update = () => {
    const item = owner.closest('.dropdown')!.querySelector(`.menu > .item[data-value="${CSS.escape(owner.value)}"]`)!;
    if (previousOwner !== owner.value) {
      identity.replaceChildren(item.querySelector('.repo-owner-identity')!.cloneNode(true));
      previousOwner = owner.value;
    }
    path.textContent = `${item.querySelector('.repo-owner-name')!.textContent} / ${sanitizeRepoName(name.value) || placeholder}`;
    const attribute = privateInput.checked ? 'data-private' : 'data-public';
    visibility.textContent = visibility.getAttribute(attribute)!;
    visibilityHelp.textContent = visibilityHelp.getAttribute(attribute)!;
    mode.textContent = mode.getAttribute(mirror?.checked ? 'data-mirror' : 'data-import')!;
  };
  form.addEventListener('input', update);
  form.addEventListener('change', update);
  update();
}

function initMigrationSourceChooser(root: HTMLElement) {
  const input = root.querySelector<HTMLInputElement>('[data-migration-source-search]')!;
  const cards = Array.from(root.querySelectorAll<HTMLAnchorElement>('[data-migration-source]'));
  const count = root.querySelector('[data-migration-source-count]')!;
  const empty = root.querySelector<HTMLElement>('[data-migration-source-empty]')!;
  const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)');
  let activeCard: HTMLAnchorElement | null = null;
  let pendingFrame = 0;
  let pointerX = 0;
  let pointerY = 0;
  const resetPointer = () => {
    cancelAnimationFrame(pendingFrame);
    pendingFrame = 0;
    activeCard?.classList.remove('is-pointer-active');
    activeCard = null;
  };
  const filter = () => {
    resetPointer();
    const query = input.value.trim().toLocaleLowerCase();
    let visible = 0;
    for (const card of cards) {
      card.hidden = !card.getAttribute('data-migration-search')!.toLocaleLowerCase().includes(query);
      if (!card.hidden) visible++;
    }
    count.textContent = String(visible);
    empty.hidden = visible !== 0;
  };
  input.addEventListener('input', filter);
  input.addEventListener('keydown', (event) => {
    if (event.key === 'Escape') {
      input.value = '';
      filter();
    }
  });
  root.querySelector('[data-migration-source-clear]')!.addEventListener('click', () => {
    input.value = '';
    filter();
    input.focus();
  });
  root.addEventListener('pointermove', (event) => {
    if (reducedMotion.matches || event.pointerType !== 'mouse' || document.hidden) return;
    const card = (event.target as HTMLElement).closest<HTMLAnchorElement>('[data-migration-source]');
    if (card !== activeCard) {
      resetPointer();
      activeCard = card;
      activeCard?.classList.add('is-pointer-active');
    }
    if (!activeCard) return;
    const bounds = activeCard.getBoundingClientRect();
    pointerX = event.clientX - bounds.left;
    pointerY = event.clientY - bounds.top;
    if (!pendingFrame) pendingFrame = requestAnimationFrame(() => {
      if (activeCard) {
        activeCard.style.setProperty('--migration-x', `${pointerX}px`);
        activeCard.style.setProperty('--migration-y', `${pointerY}px`);
      }
      pendingFrame = 0;
    });
  }, {passive: true});
  root.addEventListener('pointerleave', resetPointer);
  reducedMotion.addEventListener('change', () => {
    resetPointer();
    for (const card of cards) for (const animation of card.getAnimations()) animation.cancel();
  });
  document.addEventListener('visibilitychange', () => { if (document.hidden) resetPointer(); });
  if (!reducedMotion.matches && !document.hidden) {
    for (const [index, card] of cards.entries()) {
      card.animate([{opacity: 0, transform: 'translateY(10px)'}, {opacity: 1, transform: 'translateY(0)'}], {
        duration: 200, delay: Math.min(index * 22, 160), easing: 'ease-out', fill: 'backwards',
      });
    }
  }
}

function checkAuth() {
  if (!service) return;
  const serviceType = Number(service.value);

  checkItems(serviceType !== 1);
}

function checkItems(tokenAuth: boolean) {
  let enableItems: boolean;
  if (tokenAuth) {
    enableItems = Boolean(token?.value.trim());
  } else {
    enableItems = Boolean(user?.value.trim() || pass?.value);
  }
  if (enableItems && Number(service?.value) > 1) {
    if (mirror?.checked) {
      for (const item of items) {
        item.disabled = item.name !== 'wiki';
      }
      return;
    }
    for (const item of items) item.disabled = false;
  } else {
    for (const item of items) item.disabled = true;
  }
}

function setLFSSettingsVisibility() {
  if (!lfs) return;
  const visible = lfs.checked;
  toggleElem(lfsSettings, visible);
  if (!visible) hideElem(lfsEndpoint);
}
