export function initOrgDirectory(): void {
  const search = document.querySelector<HTMLInputElement>('[data-org-directory-search]');
  const empty = document.querySelector<HTMLElement>('[data-org-directory-empty]');
  if (!search || !empty) return;
  const cards = Array.from(document.querySelectorAll<HTMLElement>('[data-org-directory-item]'));
  const normalize = (value: string) => value.normalize('NFKC').toLowerCase().trim();
  const update = () => {
    const query = normalize(search.value);
    let count = 0;
    for (const card of cards) {
      card.hidden = !normalize(card.getAttribute('data-search')!).includes(query);
      if (!card.hidden) count++;
    }
    empty.hidden = count !== 0;
  };
  search.addEventListener('input', update);
  search.addEventListener('keydown', (event) => {
    if (event.key === 'Escape') {
      search.value = '';
      update();
    }
  });
  update();
}
