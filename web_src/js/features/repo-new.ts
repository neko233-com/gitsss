import {hideElem, querySingleVisibleElem, showElem, toggleElem} from '../utils/dom.ts';
import {htmlEscape} from '../utils/html.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';
import {sanitizeRepoName} from './repo-common.ts';

const {appSubUrl} = window.config;

function initRepoNewTemplateSearch(form: HTMLFormElement) {
  const elSubmitButton = querySingleVisibleElem<HTMLInputElement>(form, '.ui.primary.button')!;
  const elCreateRepoErrorMessage = form.querySelector('#create-repo-error-message')!;
  const elRepoOwnerDropdown = form.querySelector('#repo_owner_dropdown')!;
  const elRepoTemplateDropdown = form.querySelector<HTMLInputElement>('#repo_template_search')!;
  const inputRepoTemplate = form.querySelector<HTMLInputElement>('#repo_template')!;
  const elTemplateUnits = form.querySelector('#template_units')!;
  const elNonTemplate = form.querySelector('#non_template')!;
  let selectedTemplateOnce = inputRepoTemplate.value !== '' && inputRepoTemplate.value !== '0';
  const checkTemplate = function () {
    const hasSelectedTemplate = inputRepoTemplate.value !== '' && inputRepoTemplate.value !== '0';
    if (hasSelectedTemplate && !selectedTemplateOnce) {
      form.querySelector<HTMLInputElement>('input[name="git_content"]')!.checked = true;
      selectedTemplateOnce = true;
    }
    toggleElem(elTemplateUnits, hasSelectedTemplate);
    toggleElem(elNonTemplate, !hasSelectedTemplate);
  };
  inputRepoTemplate.addEventListener('change', checkTemplate);
  checkTemplate();

  const $repoOwnerDropdown = fomanticQuery(elRepoOwnerDropdown);
  const $repoTemplateDropdown = fomanticQuery(elRepoTemplateDropdown);
  const onChangeOwner = function () {
    const ownerId = $repoOwnerDropdown.dropdown('get value');
    const $ownerItem = $repoOwnerDropdown.dropdown('get item', ownerId);
    hideElem(elCreateRepoErrorMessage);
    elSubmitButton.disabled = false;
    if ($ownerItem?.length) {
      const elOwnerItem = $ownerItem[0];
      elCreateRepoErrorMessage.textContent = elOwnerItem.getAttribute('data-create-repo-disallowed-prompt') ?? '';
      const hasError = Boolean(elCreateRepoErrorMessage.textContent);
      toggleElem(elCreateRepoErrorMessage, hasError);
      elSubmitButton.disabled = hasError;
    }
    $repoTemplateDropdown.dropdown('setting', {
      apiSettings: {
        url: `${appSubUrl}/repo/search?q={query}&template=true&priority_owner_id=${ownerId}`,
        onResponse(response: any) {
          const results = [];
          results.push({name: '', value: ''}); // empty item means not using template
          for (const tmplRepo of response.data) {
            results.push({
              name: htmlEscape(tmplRepo.repository.full_name),
              value: String(tmplRepo.repository.id),
            });
          }
          return {results};
        },
        cache: false,
      },
    });
  };
  $repoOwnerDropdown.dropdown('setting', 'onChange', onChangeOwner);
  onChangeOwner();
}

export function initRepoNew() {
  const pageContent = document.querySelector('.page-content.repository.new-repo');
  if (!pageContent) return;

  const form = document.querySelector<HTMLFormElement>('.new-repo-form')!;
  const inputGitIgnores = form.querySelector<HTMLInputElement>('input[name="gitignores"]')!;
  const inputLicense = form.querySelector<HTMLInputElement>('input[name="license"]')!;
  const inputAutoInit = form.querySelector<HTMLInputElement>('input[name="auto_init"]')!;
  const updateUiAutoInit = () => {
    if (inputGitIgnores.value || inputLicense.value) inputAutoInit.checked = true;
  };
  inputGitIgnores.addEventListener('change', updateUiAutoInit);
  inputLicense.addEventListener('change', updateUiAutoInit);

  const initialFiles = form.querySelector('[data-repo-initial-files]')!;
  const modeInputs = form.querySelectorAll<HTMLInputElement>('input[name="gitsss_initial_mode"]');
  const syncInitialMode = () => {
    for (const input of modeInputs) input.checked = (input.value === 'initialize') === inputAutoInit.checked;
    toggleElem(initialFiles, inputAutoInit.checked);
  };
  for (const input of modeInputs) {
    input.addEventListener('change', () => {
      inputAutoInit.checked = input.value === 'initialize';
      syncInitialMode();
      updatePreview();
    });
  }
  inputGitIgnores.addEventListener('change', syncInitialMode);
  inputLicense.addEventListener('change', syncInitialMode);
  syncInitialMode();

  const inputRepoName = form.querySelector<HTMLInputElement>('input[name="repo_name"]')!;
  const inputPrivate = form.querySelector<HTMLInputElement>('input[name="private"]')!;
  const visibilityHelp = form.querySelector<HTMLElement>('[data-repo-visibility-help]')!;
  const updateVisibilityHelp = () => {
    visibilityHelp.textContent = visibilityHelp.getAttribute(inputPrivate.checked ? 'data-private' : 'data-public')!;
  };
  inputPrivate.addEventListener('change', updateVisibilityHelp);
  const updateUiRepoName = () => {
    const helps = form.querySelectorAll(`.help[data-help-for-repo-name]`);
    hideElem(helps);
    let help = form.querySelector(`.help[data-help-for-repo-name="${CSS.escape(inputRepoName.value)}"]`);
    if (!help) help = form.querySelector(`.help[data-help-for-repo-name=""]`)!;
    showElem(help);
    const repoNamePreferPrivate: Record<string, boolean> = {'.profile': false, '.profile-private': true};
    const preferPrivate = repoNamePreferPrivate[inputRepoName.value];
    // inputPrivate might be disabled because site admin "force private"
    if (preferPrivate !== undefined && !inputPrivate.closest('.disabled, [disabled]')) {
      inputPrivate.checked = preferPrivate;
    }
    updateVisibilityHelp();
  };
  inputRepoName.addEventListener('input', updateUiRepoName);
  inputRepoName.addEventListener('change', () => {
    inputRepoName.value = sanitizeRepoName(inputRepoName.value);
    updateUiRepoName();
  });
  updateUiRepoName();

  initRepoNewTemplateSearch(form);

  const ownerValue = form.querySelector<HTMLInputElement>('input[name="uid"]')!;
  const templateValue = form.querySelector<HTMLInputElement>('#repo_template')!;
  const description = form.querySelector<HTMLTextAreaElement>('#description')!;
  const branch = form.querySelector<HTMLInputElement>('#default_branch')!;
  const previewPath = form.querySelector('[data-repo-create-path]')!;
  const previewOwner = form.querySelector('[data-repo-create-owner]')!;
  const previewDescription = form.querySelector('[data-repo-create-description]')!;
  const previewVisibility = form.querySelector('[data-repo-create-visibility]')!;
  const previewBranch = form.querySelector('[data-repo-create-branch]')!;
  const previewResult = form.querySelector('.repo-create-preview-result')!;
  const i18n = form.querySelector('[data-repo-create-i18n]')!;
  const importLink = pageContent.querySelector<HTMLAnchorElement>('[data-repo-create-import]');
  let previewOwnerId = '';
  function updatePreview() {
    const item = form.querySelector(`#repo_owner_dropdown .menu > .item[data-value="${CSS.escape(ownerValue.value)}"]`)!;
    const identity = item.querySelector('.repo-owner-identity')!;
    const owner = item.querySelector('.repo-owner-name')!.textContent;
    if (previewOwnerId !== ownerValue.value) {
      previewOwner.replaceChildren(identity.cloneNode(true));
      previewOwnerId = ownerValue.value;
      if (importLink) {
        const url = new URL(importLink.href);
        url.searchParams.set('org', ownerValue.value);
        importLink.href = url.href;
      }
    }
    previewPath.textContent = `${owner} / ${sanitizeRepoName(inputRepoName.value) || i18n.getAttribute('data-placeholder')!}`;
    previewDescription.textContent = description.value.trim() || i18n.getAttribute('data-no-description')!;
    previewVisibility.textContent = i18n.getAttribute(inputPrivate.checked ? 'data-private' : 'data-public')!;
    const hasTemplate = templateValue.value !== '' && templateValue.value !== '0';
    previewBranch.textContent = hasTemplate ? i18n.getAttribute('data-template-branch')! : branch.value || branch.placeholder;
    const result = hasTemplate ? 'template' : inputAutoInit.checked ? 'initialize' : 'empty';
    previewResult.textContent = previewResult.getAttribute(`data-${result}`)!;
  }
  form.addEventListener('input', updatePreview);
  form.addEventListener('change', updatePreview);
  updatePreview();
}
