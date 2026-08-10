export function initDashboardActions(): void {
  const root = document.querySelector<HTMLElement>('[data-dashboard-actions]');
  if (!root) return;
  const search = root.querySelector<HTMLInputElement>('[data-dashboard-actions-search]');
  const items = Array.from(root.querySelectorAll<HTMLElement>('[data-dashboard-action-item]'));
  const result = root.querySelector<HTMLElement>('[data-dashboard-actions-result]');
  if (!search || !result) return;

  const update = () => {
    const query = search.value.trim().toLowerCase();
    let visible = 0;
    for (const item of items) {
      const matches = !query || (item.getAttribute('data-search') || '').includes(query);
      item.hidden = !matches;
      if (matches) visible++;
    }
    result.textContent = `${visible}/${items.length}`;
  };

  search.addEventListener('input', update);
  update();
}
