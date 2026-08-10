import {queryElems, toggleElem} from '../utils/dom.ts';

function initOrgTeamSettings() {
  // on the page "page-content organization new team"
  const pageContent = document.querySelector('.page-content.organization.new.team');
  if (!pageContent) return;
  queryElems(pageContent, 'input[name=permission]', (el) => el.addEventListener('change', () => {
    // Change team access mode
    const permission = pageContent.querySelector<HTMLInputElement>('input[name=permission]:checked');
    const val = permission ? permission.value : '';
    toggleElem(pageContent.querySelectorAll('.team-units'), val !== 'admin');
  }));
}

export function initOrgTeam() {
  if (!document.querySelector('.page-content.organization')) return;
  initOrgTeamSettings();
  initOrgTeamAssignment();
}

function initOrgTeamAssignment() {
  const palette = document.querySelector<HTMLElement>('[data-org-team-assignment]');
  const dropTarget = document.querySelector<HTMLElement>('[data-team-drop-target]');
  if (!palette || !dropTarget) return;
  const member = palette.querySelector<HTMLElement>('[data-team-member-name]');
  const teamForm = document.querySelector<HTMLFormElement>('form[action$="/action/add"]');
  if (!member || !teamForm) return;

  const submitMember = (username: string): void => {
    const input = teamForm.querySelector<HTMLInputElement>('input[name="uname"]');
    if (!input || !username) return;
    input.value = username;
    teamForm.classList.add('is-loading');
    teamForm.requestSubmit();
  };

  for (const candidate of palette.querySelectorAll<HTMLElement>('[data-team-member-name]')) {
    candidate.addEventListener('dragstart', (event) => {
      event.dataTransfer!.setData('text/plain', candidate.getAttribute('data-team-member-name')!);
      event.dataTransfer!.effectAllowed = 'copy';
      candidate.classList.add('is-dragging');
    });
    candidate.addEventListener('dragend', () => candidate.classList.remove('is-dragging'));
    candidate.addEventListener('keydown', (event) => {
      if (event.key !== 'Enter' && event.key !== ' ') return;
      event.preventDefault();
      submitMember(candidate.getAttribute('data-team-member-name')!);
    });
  }

  dropTarget.addEventListener('dragover', (event) => {
    event.preventDefault();
    dropTarget.classList.add('is-drag-over');
  });
  dropTarget.addEventListener('dragleave', () => dropTarget.classList.remove('is-drag-over'));
  dropTarget.addEventListener('drop', (event) => {
    event.preventDefault();
    dropTarget.classList.remove('is-drag-over');
    const username = event.dataTransfer!.getData('text/plain');
    if (!username) return;
    submitMember(username);
  });
}
