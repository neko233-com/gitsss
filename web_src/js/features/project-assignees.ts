import {POST} from '../modules/fetch.ts';
import {showErrorToast} from '../modules/toast.ts';

function assignedIDs(card: HTMLElement): Set<string> {
  return new Set((card.getAttribute('data-assignee-ids') ?? '').split(',').filter(Boolean));
}

async function assignIssue(card: HTMLElement, assigneeID: string): Promise<void> {
  const assigned = assignedIDs(card);
  if (assigneeID !== '0' && assigned.has(assigneeID)) return;

  const url = card.getAttribute('data-issue-assignee-url')!;
  const issueID = card.getAttribute('data-issue')!;
  const data = new URLSearchParams({
    action: assigneeID === '0' ? 'clear' : 'toggle',
    issue_ids: issueID,
    id: assigneeID === '0' ? '' : assigneeID,
  });

  card.classList.add('is-loading');
  try {
    const response = await POST(url, {data});
    if (!response.ok) throw new Error(`assignee request failed: ${response.status}`);
    window.location.reload();
  } catch (error) {
    console.error(error);
    showErrorToast(window.config.i18n.error_occurred);
  } finally {
    card.classList.remove('is-loading');
  }
}

export function initProjectAssignees(projectView: HTMLElement): void {
  const palette = projectView.querySelector<HTMLElement>('[data-project-assignee-palette]');
  if (!palette || palette.getAttribute('data-assignee-init') === 'true') return;
  palette.setAttribute('data-assignee-init', 'true');

  for (const chip of palette.querySelectorAll<HTMLElement>('[data-assignee-id]')) {
    chip.addEventListener('dragstart', (event) => {
      event.dataTransfer!.setData('text/plain', chip.getAttribute('data-assignee-id')!);
      event.dataTransfer!.effectAllowed = 'copy';
    });
  }

  for (const card of projectView.querySelectorAll<HTMLElement>('.issue-card[data-issue-assignee-url]')) {
    card.addEventListener('dragover', (event) => {
      event.preventDefault();
      card.classList.add('project-assignee-drop-target');
    });
    card.addEventListener('dragleave', () => card.classList.remove('project-assignee-drop-target'));
    card.addEventListener('drop', (event) => {
      event.preventDefault();
      card.classList.remove('project-assignee-drop-target');
      const assigneeID = event.dataTransfer!.getData('text/plain');
      if (assigneeID) assignIssue(card, assigneeID);
    });
  }
}
